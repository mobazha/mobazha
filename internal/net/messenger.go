package net

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	inet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	storeandforward "github.com/mobazha/mobazha3.0/libs/store-and-forward"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"gorm.io/gorm"
)

const (
	// RetryInterval is the interval at which retry sending messages
	// that haven't yet been ACKed.
	RetryInterval = time.Minute * 1

	// RequeryInterval is the interval at which re-query the store
	// and forward servers. We don't want to poll to frequently as
	// we are also subscribed to push messages from them.
	RequeryInterval = time.Minute * 30

	// SendTimeout is how long to wait while trying to send an online
	// message before giving up and sending it to the store and forward
	// servers.
	SendTimeout = time.Second * 5
)

// Messenger manages the reliable sending of outgoing messages.
// New messages are saved to the database and continually retried
// until the recipient receives it.
type Messenger struct {
	ns             *NetworkService
	db             database.Database
	sk             crypto.PrivKey
	testnet        bool
	snfClient      *storeandforward.Client
	getProfileFunc func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error)
	done           chan struct{}
	bootstrapDone  chan struct{}
	mtx            sync.RWMutex
	wg             sync.WaitGroup
}

// MessengerConfig holds the data needed to construct a new Messenger.
type MessengerConfig struct {
	Context        context.Context
	Service        *NetworkService
	DB             database.Database
	Privkey        crypto.PrivKey
	SNFServers     []peer.ID
	Testnet        bool
	GetProfileFunc func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error)
}

// NewMessenger returns a Messenger and starts the retry service.
func NewMessenger(cfg *MessengerConfig) (*Messenger, error) {
	snfServerProtocol := ProtocolStoreAndForwardMainnet_Server
	snfClientProtocol := ProtocolStoreAndForwardMainnet_Client
	if cfg.Testnet {
		snfServerProtocol = ProtocolStoreAndForwardTestnet_Server
		snfClientProtocol = ProtocolStoreAndForwardTestnet_Client
	}
	bootstrapDone := make(chan struct{})
	clientOpts := []storeandforward.Option{
		storeandforward.ServerProtocols(protocol.ID(snfServerProtocol)),
		storeandforward.ClientProtocols(protocol.ID(snfClientProtocol)),
		storeandforward.BootstrapDone(bootstrapDone),
	}
	client, err := storeandforward.NewClient(cfg.Context, cfg.Privkey, cfg.SNFServers, cfg.Service.host, clientOpts...)
	if err != nil {
		return nil, err
	}
	m := &Messenger{
		ns:             cfg.Service,
		db:             cfg.DB,
		sk:             cfg.Privkey,
		testnet:        cfg.Testnet,
		snfClient:      client,
		getProfileFunc: cfg.GetProfileFunc,
		done:           make(chan struct{}),
		bootstrapDone:  bootstrapDone,
		mtx:            sync.RWMutex{},
		wg:             sync.WaitGroup{},
	}
	return m, nil
}

// Stop shuts down the Messenger and blocks until all message
// attempts are finished.
func (m *Messenger) Stop() {
	close(m.done)
	m.wg.Wait()
}

// ReliablySendMessage persists the message to the database before sending, then continually retries
// the send until it finally goes through.
func (m *Messenger) ReliablySendMessage(tx database.Tx, peer peer.ID, message *pb.Message, done chan<- struct{}) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.wg.Add(1)

	ser, err := proto.Marshal(message)
	if err != nil {
		m.wg.Done()
		return err
	}

	if len(ser) > inet.MessageSizeMax {
		return errors.New("message exceeds max message size")
	}

	// Before we do anything save the message to the database. This way
	// we can retry sending the message until we know for sure that it
	// has been delivered.
	err = tx.Save(&models.OutgoingMessage{
		ID:                message.MessageID,
		Recipient:         peer.String(),
		SerializedMessage: ser,
		MessageType:       message.MessageType.String(),
		Timestamp:         time.Now(),
		LastAttempt:       time.Now(),
	})
	if err != nil {
		m.wg.Done()
		return err
	}

	// Send the message on commit.
	tx.RegisterCommitHook(func() {
		go m.trySendMessage(peer, message, done)
	})

	return nil
}

// ProcessACK deletes the message from the database after it has been
// ACKed so we no longer try sending.
func (m *Messenger) ProcessACK(tx database.Tx, ack *pb.AckMessage) error {
	log.Debugf("Received ACK for message ID %s", ack.AckedMessageID)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return tx.Delete("id", ack.AckedMessageID, nil, &models.OutgoingMessage{})
}

// SendACK sends an ACK for the message with the given ID to the provided
// peer. The ACK send is only attempted just once and unlike other messages
// is not persisted to the database. It is expect that the message handler
// will send an ACK for every duplicate message it receives. This implies
// that the sender will continue sending messages until he receives an
// ACK and the recipient will continue ACKing them until he stops receiving
// duplicate messages.
func (m *Messenger) SendACK(messageID string, peer peer.ID) {
	log.Debugf("Sending ACK for message ID: %s", messageID)

	m.wg.Add(1)

	ack := &pb.AckMessage{
		AckedMessageID: messageID,
	}

	payload := &anypb.Any{}
	if err := payload.MarshalFrom(ack); err != nil {
		log.Errorf("Error marshalling ack message: %s", err)
		return
	}

	mid := make([]byte, 20)
	rand.Read(mid)

	msg := &pb.Message{
		MessageID:   hex.EncodeToString(mid),
		MessageType: pb.Message_ACK,
		Payload:     payload,
	}
	go m.trySendMessage(peer, msg, nil)
}

// Start will start a recurring process which will attempt
// to resend any messages than have not yet been ACKed.
func (m *Messenger) Start() {
	// Run once at startup
	go m.retryAllMessages()

	go func() {
		<-m.bootstrapDone
		m.downloadMessages()
	}()

	sub := m.snfClient.SubscribeMessages()

	// Then every RetryInterval
	retryTicker := time.NewTicker(RetryInterval)
	requeryTicker := time.NewTicker(RequeryInterval)
	for {
		select {
		case <-m.done:
			retryTicker.Stop()
			requeryTicker.Stop()
			return
		case <-retryTicker.C:
			go m.retryAllMessages()
		case <-requeryTicker.C:
			go m.downloadMessages()
		case msg := <-sub.Out:
			p, pmes, err := m.decryptMessage(msg.EncryptedMessage)
			if err != nil {
				log.Warningf("Decryption failed for message %x", msg.MessageID)
			}
			m.ns.handlerMtx.RLock()
			handler, ok := m.ns.handlers[pmes.MessageType]
			m.ns.handlerMtx.RUnlock()
			if ok {
				if err := handler(p, pmes); err != nil {
					log.Errorf("Error processing %s message from %s: %s", pmes.MessageType.String(), p, err)
				}
			} else {
				log.Warningf("No handler for decrypted message %s", pmes.MessageID)
				continue
			}

			if err := m.snfClient.AckMessage(context.Background(), msg.MessageID); err != nil {
				log.Errorf("Error acking message with snf servers: %s", err)
			}
		}
	}
}

// trySendMessage tries to send the message directly to the peer using a
// network connection. If that fails, it sends the message over the offline
// messaging system.
func (m *Messenger) trySendMessage(peerID peer.ID, message *pb.Message, done chan<- struct{}) {
	defer func() {
		if done != nil {
			close(done)
		}
		m.wg.Done()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), SendTimeout)
	defer cancel()

	if err := m.ns.SendMessage(ctx, peerID, message); err != nil && m.snfClient != nil {
		log.Debugf("Failed to connect to peer %s, error: %v. Sending offline message", peerID, err)
		// We failed to deliver directly to the peer. Let's send
		// using the offline system.
		var record models.StoreAndForwardServers
		dberr := m.db.View(func(tx database.Tx) error {
			if err := tx.Read().Where("peer_id=?", peerID.String()).Find(&record).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return nil
		})
		servers, iberr := record.Servers()
		if dberr != nil || iberr != nil {
			log.Errorf("Error loading peers snf server addresses %s", err)
			return
		}

		if (len(servers) == 0 || record.LastUpdated.Add(time.Hour*48).Before(time.Now())) && m.getProfileFunc != nil {
			snfServers := []string{}

			profile, err := m.getProfileFunc(context.Background(), peerID, true)
			if err == nil {
				snfServers = profile.StoreAndForwardServers
			} else {
				log.Errorf("Error sending offline message: Can't load profile for peer %s, error: %v", peerID, err)
				log.Info("Use default snfServers instead for message sending")
				snfServers = repo.DefaultMainnetSNFServers
				if m.testnet {
					snfServers = repo.DefaultTestnetSNFServers
				}
			}
			if len(snfServers) == 0 {
				log.Errorf("Error sending offline message: No inbox peers for peer %s", peerID)
				return
			}

			servers = []peer.ID{}
			for _, peerStr := range snfServers {
				pid, err := peer.Decode(peerStr)
				if err == nil {
					servers = append(servers, pid)
				}
			}
		}

		cipherText, err := m.prepEncryptedMessage(peerID, message)
		if err != nil {
			log.Errorf("Error preparing offline message to %s: %s", peerID, err)
			return
		}

		successes := uint32(0)
		var wg sync.WaitGroup
		wg.Add(len(servers))
		for _, server := range servers {
			go func(svr peer.ID) {
				defer wg.Done()
				err := m.snfClient.SendMessage(context.Background(), peerID, svr, nil, cipherText, []byte(message.MessageType.String()))
				if err != nil {
					log.Warningf("Error pushing offline message %s to server %s: %s", message.MessageID, svr, err)
					return
				}
				atomic.AddUint32(&successes, 1)
			}(server)
		}
		wg.Wait()
		log.Debugf("Message %s sent to %d of %d servers", message.MessageID, successes, len(servers))
		return
	}
	log.Debugf("Message %s direct send successful", message.MessageID)
}

// retryAllMessages loads all un-ACKed messages from the database and
// tries to send them again using an exponential backoff.
func (m *Messenger) retryAllMessages() {
	// Increment the waitgroup to make sure we don't shutdown before
	// this process finishes.
	m.wg.Add(1)
	defer m.wg.Done()

	var messages []models.OutgoingMessage
	err := m.db.View(func(tx database.Tx) error {
		m.mtx.RLock()
		defer m.mtx.RUnlock()

		return tx.Read().Find(&messages).Error
	})
	if err != nil {
		log.Errorf("Error loading outgoing messages from the database: %s", err)
		return
	}

	for _, message := range messages {
		pmes := new(pb.Message)
		if err := proto.Unmarshal(message.SerializedMessage, pmes); err != nil {
			log.Error("Error unmarshalling outgoing message: %s", err)
			continue
		}
		pid, err := peer.Decode(message.Recipient)
		if err != nil {
			log.Error("Error parsing peer ID in outgoing message: %s", err)
			continue
		}
		if shouldWeRetry(message.Timestamp, message.LastAttempt) {
			m.wg.Add(1)
			go m.trySendMessage(pid, pmes, nil)

			err = m.db.Update(func(tx database.Tx) error {
				return tx.Update("last_attempt", time.Now(), nil, &message)
			})
			if err != nil {
				log.Error("Error updating last attempt for outgoing message: %s", err)
			}
		}
	}
}

// downloadMessages will attempt to download messages from the snf client and
// decrypt and process them.
func (m *Messenger) downloadMessages() {
	if m.snfClient != nil {
		encryptedMessages, err := m.snfClient.GetMessages(context.Background())
		if err != nil {
			log.Error("Error downloading messages from snf client: %s", err)
			return
		}
		log.Debugf("Downloaded %d encrypted messages from store-and-forward servers", len(encryptedMessages))
		type messageWithPeer struct {
			m *pb.Message
			p peer.ID
		}
		messages := make([]messageWithPeer, 0, len(encryptedMessages))
		for _, enc := range encryptedMessages {
			p, m, err := m.decryptMessage(enc.EncryptedMessage)
			if err != nil {
				log.Warningf("Decryption failed for message %x", enc.MessageID)
				continue
			}
			messages = append(messages, messageWithPeer{m: m, p: p})
		}
		// Sort the messages by sequence so we process the lowest sequences
		// first.
		sort.SliceStable(messages, func(i, j int) bool {
			return messages[i].m.Sequence < messages[j].m.Sequence
		})
		for _, mwp := range messages {
			m.ns.handlerMtx.RLock()
			handler, ok := m.ns.handlers[mwp.m.MessageType]
			m.ns.handlerMtx.RUnlock()
			if ok {
				if err := handler(mwp.p, mwp.m); err != nil {
					log.Errorf("Error processing %s message from %s: %s", mwp.m.MessageType.String(), mwp.p, err)
				}
			} else {
				log.Warningf("No handler for decrypted message %s", mwp.m.MessageID)
			}
		}
		for _, enc := range encryptedMessages {
			if err := m.snfClient.AckMessage(context.Background(), enc.MessageID); err != nil {
				log.Errorf("Error acking message with snf servers: %s", err)
			}
		}
	}
}

// prepEncryptedMessage signs the message, wraps it in an envelop, and encrypts it.
func (m *Messenger) prepEncryptedMessage(to peer.ID, message *pb.Message) ([]byte, error) {
	theirPubkey, err := to.ExtractPublicKey()
	if err != nil {
		return nil, err
	}

	ourPubkeyBytes, err := crypto.MarshalPublicKey(m.sk.GetPublic())
	if err != nil {
		return nil, err
	}

	env := pb.Envelope{
		Message:      message,
		SenderPubkey: ourPubkeyBytes,
	}

	ser, err := proto.Marshal(&env)
	if err != nil {
		return nil, err
	}

	sig, err := m.sk.Sign(ser)
	if err != nil {
		return nil, err
	}

	env.Signature = sig

	return Encrypt(theirPubkey, &env)
}

// decryptMessage will attempt to decrypt, validate, and unmarshal the message.
func (m *Messenger) decryptMessage(cipherText []byte) (peer.ID, *pb.Message, error) {
	env := new(pb.Envelope)
	if err := Decrypt(m.sk, cipherText, env); err != nil {
		return peer.ID(""), nil, err
	}

	senderPubkey, err := crypto.UnmarshalPublicKey(env.SenderPubkey)
	if err != nil {
		return peer.ID(""), nil, err
	}

	sig := env.Signature
	env.Signature = nil
	ser, err := proto.Marshal(env)
	if err != nil {
		return peer.ID(""), nil, err
	}

	valid, err := senderPubkey.Verify(ser, sig)
	if err != nil {
		return peer.ID(""), nil, err
	}
	if !valid {
		return peer.ID(""), nil, errors.New("invalid signature")
	}

	pid, err := peer.IDFromPublicKey(senderPubkey)
	return pid, env.Message, err
}

// shouldWeRetry calculates an exponential backoff for message retries based
// on how old the message is and how long since our last attempt.
func shouldWeRetry(messageTimestamp time.Time, lastTry time.Time) bool {
	timeSinceMessage := time.Since(messageTimestamp)
	timeSinceLastTry := time.Since(lastTry)

	switch t := timeSinceMessage; {
	// Less than 15 minute old message, retry every minute.
	case t < time.Minute*15 && timeSinceLastTry > time.Minute*1:
		return true
	// Less than 1 hour old message, retry every five minutes.
	case t < time.Hour && timeSinceLastTry > time.Minute*5:
		return true
	// Less than 1 day old message, retry every ten minutes.
	case t < time.Hour*24 && timeSinceLastTry > time.Minute*10:
		return true
	// Less than 1 week old message, retry every fifteen minutes.
	case t < time.Hour*24*7 && timeSinceLastTry > time.Minute*15:
		return true
	// Less than 1 month old message, retry every thirty minutes.
	case t < time.Hour*24*30 && timeSinceLastTry > time.Minute*30:
		return true
	// Less than 3 month old message, retry every hour.
	case t < time.Hour*24*30*3 && timeSinceLastTry > time.Hour:
		return true
	// Less than 6 month old message, retry every three hours.
	case t < time.Hour*24*30*6 && timeSinceLastTry > time.Hour*3:
		return true
	// Less than 1 year old message, retry every day.
	case t < time.Hour*24*30*12 && timeSinceLastTry > time.Hour*24:
		return true
	// Older than 1 year old message, retry every week.
	case t >= time.Hour*24*30*12 && timeSinceLastTry > time.Hour*24*7:
		return true
	}

	return false
}
