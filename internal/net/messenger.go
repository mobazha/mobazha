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
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/repo"
	storeandforward "github.com/mobazha/mobazha3.0/libs/store-and-forward"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
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

// Compile-time check: *Messenger implements contracts.Messenger.
var _ contracts.Messenger = (*Messenger)(nil)

// Messenger manages the reliable sending of outgoing messages.
// New messages are saved to the database and continually retried
// until the recipient receives it.
type Messenger struct {
	NodeID         string
	ns             *NetworkService
	db             database.Database
	sk             crypto.PrivKey
	testnet        bool
	snfClient      storeandforward.SNFClientInterface
	getProfileFunc func(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error)
	done           chan struct{}
	bootstrapDone  chan struct{}
	mtx            sync.RWMutex
	wg             sync.WaitGroup
	stopMu         sync.RWMutex // guards wg.Add vs wg.Wait during shutdown

	// fallbackSNFServers holds the configured SNF servers to use as fallback
	// when the target peer's profile SNF servers can't be loaded.
	// This includes both user-configured and default servers.
	fallbackSNFServers []peer.ID

	// recentlyProcessed tracks recently processed message IDs to prevent duplicate processing
	// when messages arrive via both subscription and polling paths
	recentlyProcessed   map[string]time.Time
	recentlyProcessedMu sync.Mutex

	// sendTails chains direct/offline delivery attempts per recipient. Database
	// commit hooks enqueue in commit order, so related order messages cannot be
	// observed out of order merely because their send goroutines were scheduled
	// differently (for example ORDER_SHIPMENT before ORDER_CONFIRMATION).
	sendQueueMu sync.Mutex
	sendTails   map[peer.ID]chan struct{}

	retryRunning    atomic.Bool // prevents overlapping retryAllMessages
	downloadRunning atomic.Bool // prevents overlapping downloadMessages
}

// MessengerConfig holds the data needed to construct a new Messenger.
type MessengerConfig struct {
	NodeID         string
	Context        context.Context
	Service        *NetworkService
	DB             database.Database
	Privkey        crypto.PrivKey
	SNFServers     []peer.ID
	Testnet        bool
	GetProfileFunc func(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error)

	// SNFClient is an optional pre-configured SNF client (e.g., LocalClient from SNFProxy).
	// If provided, SNFServers will be ignored and this client will be used instead.
	SNFClient storeandforward.SNFClientInterface

	// BootstrapDone is an optional channel that will be closed when SNF bootstrap is complete.
	// Only used when SNFClient is provided.
	BootstrapDone chan struct{}
}

// NewMessenger returns a Messenger and starts the retry service.
func NewMessenger(cfg *MessengerConfig) (*Messenger, error) {
	var snfClient storeandforward.SNFClientInterface
	var bootstrapDone chan struct{}

	if cfg.SNFClient != nil {
		// Use the provided SNF client (e.g., LocalClient from SNFProxy)
		snfClient = cfg.SNFClient
		bootstrapDone = cfg.BootstrapDone
		if bootstrapDone == nil {
			// Create a closed channel if not provided
			bootstrapDone = make(chan struct{})
			close(bootstrapDone)
		}
	} else {
		// Create a new traditional SNF client
		snfServerProtocol := ProtocolStoreAndForwardMainnet_Server
		snfClientProtocol := ProtocolStoreAndForwardMainnet_Client
		if cfg.Testnet {
			snfServerProtocol = ProtocolStoreAndForwardTestnet_Server
			snfClientProtocol = ProtocolStoreAndForwardTestnet_Client
		}
		bootstrapDone = make(chan struct{})
		clientOpts := []storeandforward.Option{
			storeandforward.ServerProtocols(protocol.ID(snfServerProtocol)),
			storeandforward.ClientProtocols(protocol.ID(snfClientProtocol)),
			storeandforward.BootstrapDone(bootstrapDone),
		}
		client, err := storeandforward.NewClient(cfg.Context, cfg.Privkey, cfg.SNFServers, cfg.Service.host, clientOpts...)
		if err != nil {
			return nil, err
		}
		snfClient = client
	}

	// Build fallback SNF servers: use the configured list if available,
	// otherwise fall back to hardcoded defaults.
	fallbackServers := cfg.SNFServers
	if len(fallbackServers) == 0 {
		defaultServers := repo.DefaultMainnetSNFServers
		if cfg.Testnet {
			defaultServers = repo.DefaultTestnetSNFServers
		}
		for _, s := range defaultServers {
			pid, err := peer.Decode(s)
			if err == nil {
				fallbackServers = append(fallbackServers, pid)
			}
		}
	}

	m := &Messenger{
		NodeID:             cfg.NodeID,
		ns:                 cfg.Service,
		db:                 cfg.DB,
		sk:                 cfg.Privkey,
		testnet:            cfg.Testnet,
		snfClient:          snfClient,
		getProfileFunc:     cfg.GetProfileFunc,
		fallbackSNFServers: fallbackServers,
		done:               make(chan struct{}),
		bootstrapDone:      bootstrapDone,
		recentlyProcessed:  make(map[string]time.Time),
		sendTails:          make(map[peer.ID]chan struct{}),
	}
	return m, nil
}

// Stop shuts down the Messenger and blocks until all message
// attempts are finished.
func (m *Messenger) Stop() {
	m.stopMu.Lock()
	close(m.done)
	m.stopMu.Unlock()
	m.wg.Wait()
}

// trySendAdd increments the WaitGroup if the Messenger is not
// shutting down. Returns false when Stop has been called; the
// caller MUST call wg.Done when true is returned.
func (m *Messenger) trySendAdd() bool {
	m.stopMu.RLock()
	defer m.stopMu.RUnlock()
	select {
	case <-m.done:
		return false
	default:
		m.wg.Add(1)
		return true
	}
}

// markMessageProcessed marks a message as recently processed and returns true if it was already processed.
// This prevents duplicate processing when messages arrive via both subscription and polling paths.
func (m *Messenger) markMessageProcessed(messageID string) bool {
	m.recentlyProcessedMu.Lock()
	defer m.recentlyProcessedMu.Unlock()

	// Clean up old entries (older than 5 minutes)
	cutoff := time.Now().Add(-5 * time.Minute)
	for id, t := range m.recentlyProcessed {
		if t.Before(cutoff) {
			delete(m.recentlyProcessed, id)
		}
	}

	if _, exists := m.recentlyProcessed[messageID]; exists {
		return true // Already processed
	}

	m.recentlyProcessed[messageID] = time.Now()
	return false
}

func (m *Messenger) unmarkMessageProcessed(messageID string) {
	m.recentlyProcessedMu.Lock()
	defer m.recentlyProcessedMu.Unlock()
	delete(m.recentlyProcessed, messageID)
}

// ReliablySendMessage persists the message to the database before sending, then continually retries
// the send until it finally goes through.
func (m *Messenger) ReliablySendMessage(tx database.Tx, peer peer.ID, message *pb.Message, done chan<- struct{}) error {
	ser, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	if len(ser) > inet.MessageSizeMax {
		return errors.New("message exceeds max message size")
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if !m.trySendAdd() {
		return errors.New("messenger is shutting down")
	}
	sent := false
	defer func() {
		if !sent {
			m.wg.Done()
		}
	}()

	err = tx.Save(&models.OutgoingMessage{
		ID:                message.MessageID,
		Recipient:         peer.String(),
		SerializedMessage: ser,
		MessageType:       message.MessageType.String(),
		Timestamp:         time.Now(),
		LastAttempt:       time.Now(),
	})
	if err != nil {
		return err
	}

	sent = true
	tx.RegisterCommitHook(func() {
		m.enqueueSend(peer, message, done)
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

	if !m.trySendAdd() {
		return
	}
	sent := false
	defer func() {
		if !sent {
			m.wg.Done()
		}
	}()

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
	sent = true
	m.enqueueSend(peer, msg, nil)
}

// enqueueSend preserves FIFO delivery-attempt order for each recipient while
// retaining asynchronous network I/O. The caller must already have reserved a
// WaitGroup slot with trySendAdd; trySendMessage releases that slot.
func (m *Messenger) enqueueSend(peerID peer.ID, message *pb.Message, done chan<- struct{}) {
	m.sendQueueMu.Lock()
	previous := m.sendTails[peerID]
	current := make(chan struct{})
	m.sendTails[peerID] = current
	m.sendQueueMu.Unlock()

	go func() {
		defer func() {
			close(current)
			m.sendQueueMu.Lock()
			if m.sendTails[peerID] == current {
				delete(m.sendTails, peerID)
			}
			m.sendQueueMu.Unlock()
		}()

		if previous != nil {
			<-previous
		}

		m.trySendMessage(peerID, message, done)
	}()
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
			go func() {
				if !m.retryRunning.CompareAndSwap(false, true) {
					return
				}
				defer m.retryRunning.Store(false)
				m.retryAllMessages()
			}()
		case <-requeryTicker.C:
			go func() {
				if !m.downloadRunning.CompareAndSwap(false, true) {
					return
				}
				defer m.downloadRunning.Store(false)
				m.downloadMessages()
			}()
		case msg := <-sub.Out:
			p, pmes, err := m.decryptMessage(msg.EncryptedMessage)
			if err != nil {
				logger.LogInfoWithIDf(log, m.NodeID, "Decryption failed for message %x: %v", msg.MessageID, err)
				continue // Skip messages we can't decrypt (not for us)
			}

			// Check for duplicate processing (message might arrive via both subscription and polling)
			if m.markMessageProcessed(pmes.MessageID) {
				logger.LogDebugWithIDf(log, m.NodeID, "Skipping duplicate message %s from subscription", pmes.MessageID)
				// Still ACK the message to remove it from SNF servers
				if err := m.snfClient.AckMessage(context.Background(), msg.MessageID); err != nil {
					logger.LogInfoWithIDf(log, m.NodeID, "Error acking duplicate message with snf servers: %s", err)
				}
				continue
			}

			logger.LogDebugWithIDf(log, m.NodeID, "Decrypted message %x from peer %s, type=%s", msg.MessageID, p.String(), pmes.MessageType.String())
			m.ns.handlerMtx.RLock()
			handler, ok := m.ns.handlers[pmes.MessageType]
			m.ns.handlerMtx.RUnlock()
			if ok {
				if err := handler(p, pmes); err != nil {
					logger.LogInfoWithIDf(log, m.NodeID, "Error processing %s message from %s: %s", pmes.MessageType.String(), p, err)
					m.unmarkMessageProcessed(pmes.MessageID)
					continue
				}
			} else {
				logger.LogInfoWithIDf(log, m.NodeID, "No handler for decrypted message %s", pmes.MessageID)
				m.unmarkMessageProcessed(pmes.MessageID)
				continue
			}

			if err := m.snfClient.AckMessage(context.Background(), msg.MessageID); err != nil {
				logger.LogInfoWithIDf(log, m.NodeID, "Error acking message with snf servers: %s", err)
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
		logger.LogInfoWithIDf(log, m.NodeID, "Failed to connect to peer %s, error: %v. Sending offline message", peerID, err)
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
			logger.LogInfoWithIDf(log, m.NodeID, "Error loading peers snf server addresses %s", err)
			return
		}

		if (len(servers) == 0 || record.LastUpdated.Add(time.Hour*48).Before(time.Now())) && m.getProfileFunc != nil {
			profile, err := m.getProfileFunc(context.Background(), peerID, nil, true)
			if err == nil && len(profile.StoreAndForwardServers) > 0 {
				servers = []peer.ID{}
				for _, peerStr := range profile.StoreAndForwardServers {
					pid, err := peer.Decode(peerStr)
					if err == nil {
						servers = append(servers, pid)
					}
				}
			} else {
				logger.LogInfoWithIDf(log, m.NodeID, "Error sending offline message: Can't load profile for peer %s or no snf servers, error: %v", peerID, err)
				logger.LogInfoWithIDf(log, m.NodeID, "Use configured fallback snfServers instead for message sending")
				// Use the configured fallback servers (which include both user-configured and default servers)
				// instead of only the hardcoded defaults, so custom SNF servers (e.g., in E2E testing) are used.
				servers = m.fallbackSNFServers
			}

			if len(servers) == 0 {
				logger.LogInfoWithIDf(log, m.NodeID, "Error sending offline message: No inbox peers for peer %s", peerID)
				return
			}
		}

		cipherText, err := m.prepEncryptedMessage(peerID, message)
		if err != nil {
			logger.LogInfoWithIDf(log, m.NodeID, "Error preparing offline message to %s: %s", peerID, err)
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
					logger.LogInfoWithIDf(log, m.NodeID, "Error pushing offline message %s to server %s: %s", message.MessageID, svr, err)
					return
				}
				atomic.AddUint32(&successes, 1)
			}(server)
		}
		wg.Wait()
		logger.LogInfoWithIDf(log, m.NodeID, "Message %s sent to %d of %d servers", message.MessageID, successes, len(servers))
		return
	}
	logger.LogInfoWithIDf(log, m.NodeID, "Message %s direct send successful", message.MessageID)
}

// retryAllMessages loads all un-ACKed messages from the database and
// tries to send them again using an exponential backoff.
func (m *Messenger) retryAllMessages() {
	if !m.trySendAdd() {
		return
	}
	defer m.wg.Done()

	var messages []models.OutgoingMessage
	err := m.db.View(func(tx database.Tx) error {
		m.mtx.RLock()
		defer m.mtx.RUnlock()

		// Preserve each recipient's persisted send order across process restarts.
		// Recipient grouping also makes enqueueSend rebuild one FIFO chain at a
		// time instead of depending on an unspecified database row order.
		return tx.Read().Order("recipient ASC, timestamp ASC, id ASC").Find(&messages).Error
	})
	if err != nil {
		logger.LogInfoWithIDf(log, m.NodeID, "Error loading outgoing messages from the database: %s", err)
		return
	}

	for _, message := range messages {
		pmes := new(pb.Message)
		if err := proto.Unmarshal(message.SerializedMessage, pmes); err != nil {
			logger.LogInfoWithIDf(log, m.NodeID, "Error unmarshalling outgoing message: %s", err)
			continue
		}
		pid, err := peer.Decode(message.Recipient)
		if err != nil {
			logger.LogInfoWithIDf(log, m.NodeID, "Error parsing peer ID in outgoing message: %s", err)
			continue
		}
		if shouldWeRetry(message.Timestamp, message.LastAttempt) {
			if !m.trySendAdd() {
				return
			}
			m.enqueueSend(pid, pmes, nil)

			err = m.db.Update(func(tx database.Tx) error {
				return tx.Update("last_attempt", time.Now(), nil, &message)
			})
			if err != nil {
				logger.LogInfoWithIDf(log, m.NodeID, "Error updating last attempt for outgoing message: %s", err)
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
			logger.LogInfoWithIDf(log, m.NodeID, "Error downloading messages from snf client: %s", err)
			return
		}
		if len(encryptedMessages) > 0 {
			logger.LogInfoWithIDf(log, m.NodeID, "Downloaded %d encrypted messages from store-and-forward servers", len(encryptedMessages))
		}

		type messageWithPeer struct {
			m      *pb.Message
			p      peer.ID
			encIDs [][]byte // Track all enc.MessageIDs for this application message
		}

		// First pass: decrypt and deduplicate by application-layer message ID
		// This prevents processing the same logical message multiple times
		// (sender retries create multiple encrypted copies with the same app message ID)
		uniqueMessages := make(map[string]*messageWithPeer)
		messages := make([]*messageWithPeer, 0, len(encryptedMessages))
		decryptionFailures := 0

		for _, enc := range encryptedMessages {
			p, msg, err := m.decryptMessage(enc.EncryptedMessage)
			if err != nil {
				decryptionFailures++
				continue
			}

			// Deduplicate by application message ID
			if existing, ok := uniqueMessages[msg.MessageID]; ok {
				// Same application message, just track the enc.MessageID for ACK
				existing.encIDs = append(existing.encIDs, enc.MessageID)
			} else {
				mwp := &messageWithPeer{
					m:      msg,
					p:      p,
					encIDs: [][]byte{enc.MessageID},
				}
				uniqueMessages[msg.MessageID] = mwp
				// Keep first-seen storage order. Iterating the deduplication map
				// here used to randomize all messages whose legacy sequence is 0.
				messages = append(messages, mwp)
			}
		}

		if decryptionFailures > 0 {
			logger.LogDebugWithIDf(log, m.NodeID, "Decryption failed for %d messages (not for this node)", decryptionFailures)
		}

		// Stable sorting retains first-seen storage order for legacy/business
		// messages whose sequence is zero while preserving explicit FOLLOW /
		// UNFOLLOW sequence semantics.
		sort.SliceStable(messages, func(i, j int) bool {
			return messages[i].m.Sequence < messages[j].m.Sequence
		})

		// Log deduplication stats if significant
		totalEncrypted := len(encryptedMessages) - decryptionFailures
		if totalEncrypted > len(messages) && totalEncrypted > 1 {
			logger.LogDebugWithIDf(log, m.NodeID, "Deduplicated %d encrypted messages to %d unique application messages",
				totalEncrypted, len(messages))
		}

		processedCount := 0
		skippedCount := 0
		for _, mwp := range messages {
			// Check for duplicate processing (message might have been processed via subscription)
			if m.markMessageProcessed(mwp.m.MessageID) {
				skippedCount++
				// Still ACK all the encrypted copies
				for _, encID := range mwp.encIDs {
					if err := m.snfClient.AckMessage(context.Background(), encID); err != nil {
						logger.LogDebugWithIDf(log, m.NodeID, "Error acking duplicate message: %s", err)
					}
				}
				continue
			}

			m.ns.handlerMtx.RLock()
			handler, ok := m.ns.handlers[mwp.m.MessageType]
			m.ns.handlerMtx.RUnlock()
			if ok {
				if err := handler(mwp.p, mwp.m); err != nil {
					logger.LogInfoWithIDf(log, m.NodeID, "Error processing %s message from %s: %s", mwp.m.MessageType.String(), mwp.p, err)
					m.unmarkMessageProcessed(mwp.m.MessageID)
					continue
				} else {
					processedCount++
				}
			} else {
				logger.LogInfoWithIDf(log, m.NodeID, "No handler for decrypted message %s", mwp.m.MessageID)
				m.unmarkMessageProcessed(mwp.m.MessageID)
				continue
			}

			// ACK all encrypted copies of this message
			for _, encID := range mwp.encIDs {
				if err := m.snfClient.AckMessage(context.Background(), encID); err != nil {
					logger.LogDebugWithIDf(log, m.NodeID, "Error acking message: %s", err)
				}
			}
		}

		if skippedCount > 0 {
			logger.LogDebugWithIDf(log, m.NodeID, "Skipped %d already-processed messages from download", skippedCount)
		}
		if processedCount > 0 {
			logger.LogInfoWithIDf(log, m.NodeID, "Successfully processed %d new messages", processedCount)
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
		logger.LogErrorWithIDf(log, m.NodeID, "decryptMessage: failed to unmarshal sender pubkey: %v, pubkey len=%d", err, len(env.SenderPubkey))
		return peer.ID(""), nil, err
	}

	// Derive sender peer ID for logging
	senderPeerID, _ := peer.IDFromPublicKey(senderPubkey)
	logger.LogDebugWithIDf(log, m.NodeID, "decryptMessage: sender pubkey type=%d, derived peer ID=%s", senderPubkey.Type(), senderPeerID.String())

	sig := env.Signature
	env.Signature = nil
	ser, err := proto.Marshal(env)
	if err != nil {
		return peer.ID(""), nil, err
	}

	valid, err := senderPubkey.Verify(ser, sig)
	if err != nil {
		logger.LogErrorWithIDf(log, m.NodeID, "decryptMessage: envelope signature verify error: %v", err)
		return peer.ID(""), nil, err
	}
	if !valid {
		logger.LogErrorWithIDf(log, m.NodeID, "decryptMessage: INVALID envelope signature from sender %s", senderPeerID.String())
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
