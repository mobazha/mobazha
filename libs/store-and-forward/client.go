package storeandforward

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	ctxio "github.com/jbenet/go-context/io"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	inet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-msgio"
	"github.com/mobazha/mobazha3.0/libs/store-and-forward/pb"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Message represents an encrypted message.
type Message struct {
	MessageID        []byte
	EncryptedMessage []byte
}

// Subscription is a subscription chan for listening for
// relayed messages.
type Subscription struct {
	Out   chan Message
	Close func()
}

// Client implements the functionality needed to use the store
// and forward protocol. It authenticates and registers with
// the store and forward servers.
type Client struct {
	servers             map[peer.ID]bool
	cachedRegistrations map[peer.ID]time.Time
	subs                map[int32]*Subscription
	recentlyRelayed     map[string]bool
	host                host.Host
	ctx                 context.Context
	bootstrapChan       chan struct{}
	bootstrapOnce       sync.Once
	mtx                 sync.RWMutex
	sk                  crypto.PrivKey
	serverProtocol      protocol.ID
	clientProtocol      protocol.ID
}

// NewClient returns a new Client and connects, authenticates, and registers with the servers.
func NewClient(ctx context.Context, sk crypto.PrivKey, servers []peer.ID, h host.Host, opts ...Option) (*Client, error) {
	var cfg Options
	if err := cfg.Apply(append([]Option{Defaults}, opts...)...); err != nil {
		return nil, err
	}

	serverMap := make(map[peer.ID]bool)
	for _, peer := range servers {
		serverMap[peer] = true
	}

	if len(cfg.ServerProtocols) == 0 {
		return nil, errors.New("server protocol option is required")
	}

	if len(cfg.ClientProtocols) == 0 {
		return nil, errors.New("client protocol option is required")
	}

	c := &Client{
		servers:             serverMap,
		subs:                make(map[int32]*Subscription),
		cachedRegistrations: make(map[peer.ID]time.Time),
		recentlyRelayed:     make(map[string]bool),
		host:                h,
		ctx:                 ctx,
		sk:                  sk,
		bootstrapChan:       cfg.BootstrapDone,
		mtx:                 sync.RWMutex{},
		serverProtocol:      cfg.ServerProtocols[0],
		clientProtocol:      cfg.ClientProtocols[0],
	}

	if !h.ID().MatchesPrivateKey(sk) {
		return nil, errors.New("private key does not match host peer ID")
	}

	if cfg.RegistrationDuration < time.Hour {
		return nil, errors.New("expiration duration must be at least one hour")
	}

	for _, protocol := range cfg.ClientProtocols {
		h.SetStreamHandler(protocol, c.handleNewStream)
	}

	go c.registerWithServers(cfg.RegistrationDuration)

	return c, nil
}

// SubscribeMessages returns a subscription which fires whenever the client
// receives a relayed message.
func (cli *Client) SubscribeMessages() *Subscription {
	cli.mtx.Lock()
	defer cli.mtx.Unlock()

	n := rand.Int31()
	sub := &Subscription{
		Out: make(chan Message),
		Close: func() {
			cli.mtx.Lock()
			delete(cli.subs, n)
			cli.mtx.Unlock()

		},
	}
	cli.subs[n] = sub
	return sub
}

// GetMessages queries the servers for messages and returns them.
func (cli *Client) GetMessages(ctx context.Context) ([]Message, error) {
	resp, err := cli.GetMessagesAsync(ctx)
	if err != nil {
		return nil, err
	}
	var messages []Message
	for m := range resp {
		messages = append(messages, m)
	}
	return messages, nil
}

// GetMessagesAsync behaves like GetMessages but returns the responses over a channel.
func (cli *Client) GetMessagesAsync(ctx context.Context) (<-chan Message, error) {
	var (
		downloaded = make(map[string]bool)
		mtx        = sync.Mutex{}
		resp       = make(chan Message)
		wg         sync.WaitGroup
	)
	for p, registered := range cli.servers {
		if registered {
			wg.Add(1)
			go func(p peer.ID) {
				log.Debugf("Requesting messages from server %s", p)
				defer wg.Done()

				s, err := cli.host.NewStream(inet.WithAllowLimitedConn(ctx, "identify"), p, cli.serverProtocol)
				if err != nil {
					log.Errorf("Error opening stream with server %s, error: %v", p, err)
					return
				}
				defer s.Close()

				contextReader := ctxio.NewReader(ctx, s)
				r := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
				w := msgio.NewVarintWriter(s)

				err = writeMsgWithTimeout(w, &pb.Message{
					Type: pb.Message_GET_MESSAGES,
				})
				if err != nil {
					log.Errorf("Error sending GET_MESSAGES to server %s", p)
					return
				}

				for {
					pmes := new(pb.Message)
					if err := readMsgWithTimeout(r, pmes); err != nil {
						log.Errorf("Error reading MESSAGE from server %s", p)
						return
					}
					if pmes.Type != pb.Message_MESSAGE || pmes.GetEncryptedMessage() == nil {
						log.Errorf("Server %s sending malformed MESSAGE, %s", p, pmes.Type)
						return
					}
					enc := pmes.GetEncryptedMessage()

					if !enc.More {
						log.Debugf("Downloaded messages from server %s", p)
						return
					}

					messageIDStr := hex.EncodeToString(enc.MessageID)
					mtx.Lock()
					_, ok := downloaded[messageIDStr]
					if !ok {
						downloaded[messageIDStr] = true
						resp <- Message{
							MessageID:        enc.MessageID,
							EncryptedMessage: enc.Message,
						}
					}
					mtx.Unlock()
				}
			}(p)
		}
	}
	go func() {
		wg.Wait()
		close(resp)
	}()

	return resp, nil
}

// SendMessage stores the message with the provided server.
func (cli *Client) SendMessage(ctx context.Context, to, server peer.ID, pubkey crypto.PubKey, encryptedMessage, metadata []byte) error {
	log.Debugf("Sending message to server %s", server)
	s, err := cli.host.NewStream(inet.WithAllowLimitedConn(ctx, "identify"), server, cli.serverProtocol)
	if err != nil {
		return err
	}
	defer s.Close()

	w := msgio.NewVarintWriter(s)
	contextReader := ctxio.NewReader(ctx, s)
	r := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)

	cli.mtx.RLock()
	expiry, ok := cli.cachedRegistrations[to]
	cli.mtx.RUnlock()

	if !ok || expiry.Before(time.Now()) {
		err = writeMsgWithTimeout(w, &pb.Message{
			Type: pb.Message_PROVE_REGISTRATION,
			Payload: &pb.Message_Ids{
				Ids: &pb.Message_IDs{
					PeerID: []byte(to),
				},
			},
		})
		if err != nil {
			return err
		}
		resp := new(pb.Message)
		if err = readMsgWithTimeout(r, resp); err != nil {
			return err
		}
		reg := resp.GetRegistration()
		if reg == nil {
			return fmt.Errorf("server %s returned invalid registration", server)
		}

		expiry, err := ptypes.Timestamp(reg.Expiry)
		if err != nil {
			return err
		}
		if expiry.Before(time.Now()) {
			return fmt.Errorf("server %s returned invalid registration", server)
		}

		if peer.ID(reg.Server) != server {
			return fmt.Errorf("server %s returned invalid registration", server)
		}

		if pubkey == nil {
			pubkey, err = to.ExtractPublicKey()
			if err != nil {
				return err
			}
		}

		m := proto.Clone(reg)
		regCpy := m.(*pb.Message_Registration)
		regCpy.Signature = nil
		sigSer, err := proto.Marshal(regCpy)
		if err != nil {
			return err
		}
		valid, err := pubkey.Verify(sigSer, reg.Signature)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("server %s returned invalid registration", server)
		}

		cli.mtx.Lock()
		cli.cachedRegistrations[to] = expiry
		cli.mtx.Unlock()
	}

	err = writeMsgWithTimeout(w, &pb.Message{
		Type: pb.Message_STORE_MESSAGE,
		Payload: &pb.Message_EncryptedMessage_{
			EncryptedMessage: &pb.Message_EncryptedMessage{
				Message:  encryptedMessage,
				PeerID:   []byte(to),
				Metadata: metadata,
			},
		},
	})
	if err != nil {
		return err
	}
	resp := new(pb.Message)
	if err = readMsgWithTimeout(r, resp); err != nil {
		return err
	}
	if resp.Code != pb.Message_SUCCESS {
		return fmt.Errorf("store failed with code: %s", resp.Code)
	}
	log.Debugf("Message send to server %s successful", server)
	return nil
}

// AckMessage sends the ack message to the servers. Upon receipt of the message
// the servers will delete the message so the client must make sure it has it
// fully committed first.
func (cli *Client) AckMessage(ctx context.Context, messageID []byte) error {
	var wg sync.WaitGroup
	for p, registered := range cli.servers {
		if registered {
			wg.Add(1)
			go func(p peer.ID) {
				log.Debugf("Sending ack to server %s", p)
				defer wg.Done()

				s, err := cli.host.NewStream(inet.WithAllowLimitedConn(ctx, "identify"), p, cli.serverProtocol)
				if err != nil {
					log.Errorf("Error opening stream with server %s", p)
					return
				}
				defer s.Close()

				contextReader := ctxio.NewReader(cli.ctx, s)
				r := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
				w := msgio.NewVarintWriter(s)

				err = writeMsgWithTimeout(w, &pb.Message{
					Type: pb.Message_MESSAGE_ACK,
					Payload: &pb.Message_Ack_{
						Ack: &pb.Message_Ack{
							MessageID: messageID,
						},
					},
				})
				if err != nil {
					log.Errorf("Error sending MESSAGE_ACK to server %s", p)
					return
				}

				resp := new(pb.Message)
				if err = readMsgWithTimeout(r, resp); err != nil {
					log.Errorf("Error reading MESSAGE_ACK response from server %s, error: %v", p, err)
					return
				}
				if resp.Code != pb.Message_SUCCESS {
					log.Errorf("message ack to server %s failed with code: %s", p, resp.Code)
				}
			}(p)
		}
	}
	wg.Wait()
	return nil
}

func (cli *Client) handleNewStream(s inet.Stream) {
	go cli.streamHandler(s)
}

func (cli *Client) streamHandler(s inet.Stream) {
	defer s.Close()
	contextReader := ctxio.NewReader(cli.ctx, s)
	reader := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
	remotePeer := s.Conn().RemotePeer()

	for {
		select {
		case <-cli.ctx.Done():
			return
		default:
		}

		pmes := new(pb.Message)
		msgBytes, err := reader.ReadMsg()
		if err != nil {
			reader.ReleaseMsg(msgBytes)
			s.Reset()
			if err == io.EOF {
				log.Debugf("server %s closed stream", remotePeer)
			}
			return
		}

		err = proto.Unmarshal(msgBytes, pmes)
		if err != nil {
			reader.ReleaseMsg(msgBytes)
			s.Reset()
			return
		}
		reader.ReleaseMsg(msgBytes)

		if pmes.Type != pb.Message_MESSAGE || pmes.GetEncryptedMessage() == nil {
			log.Errorf("Server %s sending malformed MESSAGE, %s", remotePeer, pmes.Type)
			continue
		}
		enc := pmes.GetEncryptedMessage()
		messageIDStr := hex.EncodeToString(enc.MessageID)

		cli.mtx.Lock()
		_, ok := cli.recentlyRelayed[messageIDStr]
		if !ok {
			cli.recentlyRelayed[messageIDStr] = true
			time.AfterFunc(time.Minute*5, func() {
				cli.mtx.Lock()
				delete(cli.recentlyRelayed, messageIDStr)
				cli.mtx.Unlock()
			})

			for _, sub := range cli.subs {
				sub.Out <- Message{
					MessageID:        enc.MessageID,
					EncryptedMessage: enc.Message,
				}
			}
		}
		cli.mtx.Unlock()
	}
}

func (cli *Client) registerWithServers(expiration time.Duration) {
	for p := range cli.servers {
		cli.registerSingle(p, expiration)
	}

	go func() {
		newRegistrationTicker := time.NewTicker(expiration - time.Minute*10)
		boostrapTicker := time.NewTicker(time.Minute)

		for {
			select {
			case <-newRegistrationTicker.C:
				for p := range cli.servers {
					go cli.registerSingle(p, expiration)
				}
			case <-boostrapTicker.C:
				cli.mtx.RLock()
				for p, registered := range cli.servers {
					if !registered {
						go cli.registerSingle(p, expiration)
					}
				}
				cli.mtx.RUnlock()
			case <-cli.ctx.Done():
				return
			}
		}
	}()
}

func (cli *Client) registerSingle(server peer.ID, expiration time.Duration) {
	s, err := cli.host.NewStream(inet.WithAllowLimitedConn(cli.ctx, "identify"), server, cli.serverProtocol)
	if err != nil {
		log.Errorf("Server %s authentication fail. Error: %s", server, err)
		return
	}
	defer s.Close()

	contextReader := ctxio.NewReader(cli.ctx, s)
	r := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
	w := msgio.NewVarintWriter(s)

	ts := timestamppb.New(time.Now().Add(expiration))
	reg := &pb.Message_Registration{
		Expiry: ts,
		Server: []byte(server),
	}

	ser, err := proto.Marshal(reg)
	if err != nil {
		log.Errorf("Server %s registration error. Error: %s", server, err)
		return
	}

	sig, err := cli.sk.Sign(ser)
	if err != nil {
		log.Errorf("Server %s registration error. Error: %s", server, err)
		return
	}
	reg.Signature = sig

	err = writeMsgWithTimeout(w, &pb.Message{
		Type: pb.Message_REGISTER,
		Payload: &pb.Message_Registration_{
			Registration: reg,
		},
	})
	if err != nil {
		log.Errorf("Server %s registration error. Error: %s", server, err)
		return
	}

	resp := new(pb.Message)
	if err := readMsgWithTimeout(r, resp); err != nil {
		log.Errorf("Server %s registration error. Error: %s", server, err)
		return
	}

	if resp.Type != pb.Message_STATUS {
		log.Errorf("Server %s sent us an invalid challenge response", server)
		return
	}

	if resp.Code != pb.Message_SUCCESS {
		log.Errorf("Server %s rejected our authentication. Code %s", server, resp.Code)
		return
	}

	cli.mtx.Lock()
	cli.servers[server] = true
	cli.mtx.Unlock()

	if cli.bootstrapChan != nil {
		cli.bootstrapOnce.Do(func() {
			close(cli.bootstrapChan)
		})
	}

	log.Debugf("Registered with server %s", server)
}
