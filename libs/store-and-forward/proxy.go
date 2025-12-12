package storeandforward

import (
	"context"
	"encoding/hex"
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
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SNFProxy manages SNF communications for multiple local nodes through a shared transport.
// This reduces the number of connections to SNF servers when hosting multiple nodes.
type SNFProxy struct {
	// transportHost is the shared libp2p host used for network communication
	transportHost host.Host

	// servers is the list of SNF server peer IDs
	servers []peer.ID

	// localClients maps peer ID to their local client state
	localClients map[peer.ID]*LocalClient

	// serverProtocol is the SNF server protocol ID
	serverProtocol protocol.ID

	// clientProtocol is the SNF client protocol ID
	clientProtocol protocol.ID

	// ctx is the context for the proxy
	ctx context.Context

	// registrationDuration is how long registrations are valid
	registrationDuration time.Duration

	// bootstrapChan signals when at least one server is connected
	bootstrapChan chan struct{}
	bootstrapOnce sync.Once

	mtx sync.RWMutex
}

// LocalClient represents a lightweight SNF client for a single node.
// It communicates through the SNFProxy rather than directly with servers.
type LocalClient struct {
	// peerID is this node's peer ID
	peerID peer.ID

	// privateKey is used for signing registrations
	privateKey crypto.PrivKey

	// proxy is the parent SNFProxy
	proxy *SNFProxy

	// registeredServers tracks which servers this client is registered with
	registeredServers map[peer.ID]time.Time

	// messageChan receives messages for this node
	messageChan chan Message

	// subs holds message subscriptions
	subs map[int32]*Subscription

	// cachedRegistrations caches peer registrations for sending messages
	cachedRegistrations map[peer.ID]time.Time

	// recentlyRelayed tracks recently relayed messages to avoid duplicates
	recentlyRelayed map[string]bool

	// recentlyRelayedTime tracks when messages were relayed for cleanup
	recentlyRelayedTime map[string]time.Time

	mtx sync.RWMutex
}

// ProxyConfig holds configuration for creating an SNFProxy
type ProxyConfig struct {
	// TransportHost is the shared libp2p host for network communication
	TransportHost host.Host

	// Servers is the list of SNF server peer IDs
	Servers []peer.ID

	// ServerProtocol is the SNF server protocol ID
	ServerProtocol protocol.ID

	// ClientProtocol is the SNF client protocol ID
	ClientProtocol protocol.ID

	// RegistrationDuration is how long registrations are valid
	RegistrationDuration time.Duration

	// BootstrapDone is signaled when at least one server is connected
	BootstrapDone chan struct{}
}

// NewSNFProxy creates a new SNF proxy instance
func NewSNFProxy(ctx context.Context, cfg *ProxyConfig) (*SNFProxy, error) {
	if cfg.RegistrationDuration < time.Hour {
		cfg.RegistrationDuration = time.Hour * 24 * 365 * 10 // 10 years default
	}

	proxy := &SNFProxy{
		transportHost:        cfg.TransportHost,
		servers:              cfg.Servers,
		localClients:         make(map[peer.ID]*LocalClient),
		serverProtocol:       cfg.ServerProtocol,
		clientProtocol:       cfg.ClientProtocol,
		ctx:                  ctx,
		registrationDuration: cfg.RegistrationDuration,
		bootstrapChan:        cfg.BootstrapDone,
	}

	// Set up stream handler for incoming messages
	cfg.TransportHost.SetStreamHandler(cfg.ClientProtocol, proxy.handleIncomingStream)

	return proxy, nil
}

// RegisterNode registers a new local node with the proxy
func (p *SNFProxy) RegisterNode(peerID peer.ID, privateKey crypto.PrivKey) (*LocalClient, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	// Check if already registered
	if client, ok := p.localClients[peerID]; ok {
		return client, nil
	}

	client := &LocalClient{
		peerID:              peerID,
		privateKey:          privateKey,
		proxy:               p,
		registeredServers:   make(map[peer.ID]time.Time),
		messageChan:         make(chan Message, 100),
		subs:                make(map[int32]*Subscription),
		cachedRegistrations: make(map[peer.ID]time.Time),
		recentlyRelayed:     make(map[string]bool),
		recentlyRelayedTime: make(map[string]time.Time),
	}

	p.localClients[peerID] = client

	// Start cleanup goroutine for this client
	go client.cleanupRecentlyRelayed()

	// Start registration with all servers
	go client.registerWithAllServers()

	return client, nil
}

// UnregisterNode removes a local node from the proxy
func (p *SNFProxy) UnregisterNode(peerID peer.ID) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if client, ok := p.localClients[peerID]; ok {
		close(client.messageChan)
		delete(p.localClients, peerID)
	}
}

// GetLocalClient returns the LocalClient for a given peer ID
func (p *SNFProxy) GetLocalClient(peerID peer.ID) (*LocalClient, bool) {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	client, ok := p.localClients[peerID]
	return client, ok
}

// handleIncomingStream handles incoming messages from SNF servers
func (p *SNFProxy) handleIncomingStream(s inet.Stream) {
	go p.streamHandler(s)
}

func (p *SNFProxy) streamHandler(s inet.Stream) {
	defer s.Close()
	contextReader := ctxio.NewReader(p.ctx, s)
	reader := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
	remotePeer := s.Conn().RemotePeer()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		pmes := new(pb.Message)
		msgBytes, err := reader.ReadMsg()
		if err != nil {
			reader.ReleaseMsg(msgBytes)
			s.Reset()
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
			log.Debugf("Server %s sending non-MESSAGE type: %s", remotePeer, pmes.Type)
			continue
		}

		enc := pmes.GetEncryptedMessage()
		messageIDStr := hex.EncodeToString(enc.MessageID)

		// Route message directly to target client using peer ID (O(1) lookup)
		if len(enc.PeerID) == 0 {
			log.Debugf("Received message without PeerID from server %s, ignoring", remotePeer)
			continue
		}

		targetPeerID := peer.ID(enc.PeerID)
		p.mtx.RLock()
		client, ok := p.localClients[targetPeerID]
		p.mtx.RUnlock()

		if ok {
			p.deliverToClient(client, messageIDStr, enc)
		}
		// If client not found, message is not for any of our nodes - ignore
	}
}

// deliverToClient delivers a message to a specific LocalClient with deduplication
func (p *SNFProxy) deliverToClient(client *LocalClient, messageIDStr string, enc *pb.Message_EncryptedMessage) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	_, alreadyRelayed := client.recentlyRelayed[messageIDStr]
	if alreadyRelayed {
		return
	}

	client.recentlyRelayed[messageIDStr] = true
	client.recentlyRelayedTime[messageIDStr] = time.Now()

	// Notify subscribers
	for _, sub := range client.subs {
		select {
		case sub.Out <- Message{
			MessageID:        enc.MessageID,
			EncryptedMessage: enc.Message,
		}:
		default:
			// Channel full, message will be retrieved via GetMessages later
			log.Warnf("LocalClient %s: subscription channel full, message %s deferred to polling", client.peerID.ShortString(), messageIDStr)
		}
	}
}

// openStreamToServer opens a stream to the specified SNF server
func (p *SNFProxy) openStreamToServer(ctx context.Context, server peer.ID) (inet.Stream, error) {
	return p.transportHost.NewStream(inet.WithAllowLimitedConn(ctx, "snf"), server, p.serverProtocol)
}

// ============== LocalClient Methods ==============

// cleanupRecentlyRelayed periodically cleans up old entries from recentlyRelayed map
// This uses a single goroutine per client instead of one goroutine per message
func (c *LocalClient) cleanupRecentlyRelayed() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.proxy.ctx.Done():
			return
		case <-ticker.C:
			c.mtx.Lock()
			now := time.Now()
			for msgID, relayedAt := range c.recentlyRelayedTime {
				if now.Sub(relayedAt) > 5*time.Minute {
					delete(c.recentlyRelayed, msgID)
					delete(c.recentlyRelayedTime, msgID)
				}
			}
			c.mtx.Unlock()
		}
	}
}

// registerWithAllServers registers this client with all SNF servers
func (c *LocalClient) registerWithAllServers() {
	for _, server := range c.proxy.servers {
		c.registerWithServer(server)
	}

	// Start periodic re-registration
	go c.registrationLoop()
}

func (c *LocalClient) registrationLoop() {
	reregisterTicker := time.NewTicker(c.proxy.registrationDuration - time.Minute*10)
	retryTicker := time.NewTicker(time.Minute)

	for {
		select {
		case <-reregisterTicker.C:
			for _, server := range c.proxy.servers {
				go c.registerWithServer(server)
			}
		case <-retryTicker.C:
			c.mtx.RLock()
			for _, server := range c.proxy.servers {
				if _, registered := c.registeredServers[server]; !registered {
					go c.registerWithServer(server)
				}
			}
			c.mtx.RUnlock()
		case <-c.proxy.ctx.Done():
			return
		}
	}
}

func (c *LocalClient) registerWithServer(server peer.ID) {
	ctx, cancel := context.WithTimeout(c.proxy.ctx, 30*time.Second)
	defer cancel()

	s, err := c.proxy.openStreamToServer(ctx, server)
	if err != nil {
		log.Debugf("LocalClient %s: failed to connect to server %s: %v", c.peerID.ShortString(), server.ShortString(), err)
		return
	}
	defer s.Close()

	contextReader := ctxio.NewReader(ctx, s)
	r := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
	w := msgio.NewVarintWriter(s)

	// Create registration with this client's peer ID and signature
	ts := timestamppb.New(time.Now().Add(c.proxy.registrationDuration))

	// Include public key for proxy mode identity verification
	pubKeyBytes, err := crypto.MarshalPublicKey(c.privateKey.GetPublic())
	if err != nil {
		log.Errorf("LocalClient %s: failed to marshal public key: %v", c.peerID.ShortString(), err)
		return
	}

	reg := &pb.Message_Registration{
		Expiry: ts,
		Server: []byte(server),
		Pubkey: pubKeyBytes, // Include pubkey BEFORE signing
	}

	ser, err := proto.Marshal(reg)
	if err != nil {
		log.Errorf("LocalClient %s: registration marshal error: %v", c.peerID.ShortString(), err)
		return
	}

	sig, err := c.privateKey.Sign(ser)
	if err != nil {
		log.Errorf("LocalClient %s: registration sign error: %v", c.peerID.ShortString(), err)
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
		log.Errorf("LocalClient %s: registration send error: %v", c.peerID.ShortString(), err)
		return
	}

	resp := new(pb.Message)
	if err := readMsgWithTimeout(r, resp); err != nil {
		log.Errorf("LocalClient %s: registration response read error: %v", c.peerID.ShortString(), err)
		return
	}

	if resp.Type != pb.Message_STATUS {
		log.Errorf("LocalClient %s: server %s sent invalid response type", c.peerID.ShortString(), server.ShortString())
		return
	}

	if resp.Code != pb.Message_SUCCESS {
		log.Errorf("LocalClient %s: server %s rejected registration: %s", c.peerID.ShortString(), server.ShortString(), resp.Code)
		return
	}

	c.mtx.Lock()
	c.registeredServers[server] = time.Now().Add(c.proxy.registrationDuration)
	c.mtx.Unlock()

	// Signal bootstrap done
	if c.proxy.bootstrapChan != nil {
		c.proxy.bootstrapOnce.Do(func() {
			close(c.proxy.bootstrapChan)
		})
	}

	log.Debugf("LocalClient %s: registered with server %s", c.peerID.ShortString(), server.ShortString())
}

// SubscribeMessages returns a subscription for receiving relayed messages
func (c *LocalClient) SubscribeMessages() *Subscription {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	n := int32(time.Now().UnixNano())
	sub := &Subscription{
		Out: make(chan Message),
		Close: func() {
			c.mtx.Lock()
			delete(c.subs, n)
			c.mtx.Unlock()
		},
	}
	c.subs[n] = sub
	return sub
}

// GetMessages retrieves messages from all registered SNF servers
func (c *LocalClient) GetMessages(ctx context.Context) ([]Message, error) {
	var (
		downloaded = make(map[string]bool)
		mtx        sync.Mutex
		messages   []Message
		wg         sync.WaitGroup
	)

	c.mtx.RLock()
	servers := make([]peer.ID, 0, len(c.registeredServers))
	for server := range c.registeredServers {
		servers = append(servers, server)
	}
	c.mtx.RUnlock()

	for _, server := range servers {
		wg.Add(1)
		go func(srv peer.ID) {
			defer wg.Done()

			s, err := c.proxy.openStreamToServer(ctx, srv)
			if err != nil {
				log.Debugf("LocalClient %s: failed to connect to server %s for GetMessages: %v", c.peerID.ShortString(), srv.ShortString(), err)
				return
			}
			defer s.Close()

			contextReader := ctxio.NewReader(ctx, s)
			r := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
			w := msgio.NewVarintWriter(s)

			// Create identity proof for proxy mode
			reg, err := c.createIdentityProof(srv)
			if err != nil {
				log.Errorf("LocalClient %s: failed to create identity proof: %v", c.peerID.ShortString(), err)
				return
			}

			err = writeMsgWithTimeout(w, &pb.Message{
				Type: pb.Message_GET_MESSAGES,
				Payload: &pb.Message_Registration_{
					Registration: reg,
				},
			})
			if err != nil {
				return
			}

			for {
				pmes := new(pb.Message)
				if err := readMsgWithTimeout(r, pmes); err != nil {
					return
				}
				if pmes.Type != pb.Message_MESSAGE || pmes.GetEncryptedMessage() == nil {
					return
				}
				enc := pmes.GetEncryptedMessage()

				if !enc.More {
					return
				}

				messageIDStr := hex.EncodeToString(enc.MessageID)
				mtx.Lock()
				if !downloaded[messageIDStr] {
					downloaded[messageIDStr] = true
					messages = append(messages, Message{
						MessageID:        enc.MessageID,
						EncryptedMessage: enc.Message,
					})
				}
				mtx.Unlock()
			}
		}(server)
	}

	wg.Wait()
	return messages, nil
}

// SendMessage sends a message through an SNF server
func (c *LocalClient) SendMessage(ctx context.Context, to, server peer.ID, pubkey crypto.PubKey, encryptedMessage, metadata []byte) error {
	s, err := c.proxy.openStreamToServer(ctx, server)
	if err != nil {
		return err
	}
	defer s.Close()

	w := msgio.NewVarintWriter(s)
	contextReader := ctxio.NewReader(ctx, s)
	r := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)

	// Check cached registration
	c.mtx.RLock()
	expiry, ok := c.cachedRegistrations[to]
	c.mtx.RUnlock()

	if !ok || expiry.Before(time.Now()) {
		// Prove registration
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
			return ErrNotRegistered
		}

		expiry, err := ptypes.Timestamp(reg.Expiry)
		if err != nil {
			return err
		}
		if expiry.Before(time.Now()) {
			return ErrNotRegistered
		}

		// Verify signature
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
			return ErrInvalidSignature
		}

		c.mtx.Lock()
		c.cachedRegistrations[to] = expiry
		c.mtx.Unlock()
	}

	// Send message
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
		return ErrStoreFailed
	}

	return nil
}

// AckMessage acknowledges receipt of a message
func (c *LocalClient) AckMessage(ctx context.Context, messageID []byte) error {
	var wg sync.WaitGroup

	c.mtx.RLock()
	servers := make([]peer.ID, 0, len(c.registeredServers))
	for server := range c.registeredServers {
		servers = append(servers, server)
	}
	c.mtx.RUnlock()

	for _, server := range servers {
		wg.Add(1)
		go func(srv peer.ID) {
			defer wg.Done()

			s, err := c.proxy.openStreamToServer(ctx, srv)
			if err != nil {
				return
			}
			defer s.Close()

			contextReader := ctxio.NewReader(ctx, s)
			r := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
			w := msgio.NewVarintWriter(s)

			// In proxy mode, we need to establish identity first
			// Send a REGISTER message to authenticate this session
			reg, err := c.createIdentityProof(srv)
			if err != nil {
				log.Debugf("LocalClient %s: failed to create identity proof for ACK: %v", c.peerID.ShortString(), err)
				return
			}

			err = writeMsgWithTimeout(w, &pb.Message{
				Type: pb.Message_REGISTER,
				Payload: &pb.Message_Registration_{
					Registration: reg,
				},
			})
			if err != nil {
				return
			}

			// Read and ignore the registration response
			resp := new(pb.Message)
			if err := readMsgWithTimeout(r, resp); err != nil {
				return
			}

			// Now send the ACK (session is authenticated)
			err = writeMsgWithTimeout(w, &pb.Message{
				Type: pb.Message_MESSAGE_ACK,
				Payload: &pb.Message_Ack_{
					Ack: &pb.Message_Ack{
						MessageID: messageID,
					},
				},
			})
			if err != nil {
				return
			}

			resp = new(pb.Message)
			if err := readMsgWithTimeout(r, resp); err != nil {
				log.Debugf("LocalClient %s: failed to read ACK response from server %s: %v", c.peerID.ShortString(), srv.ShortString(), err)
			}
		}(server)
	}

	wg.Wait()
	return nil
}

// GetRegisteredServers returns the list of servers this client is registered with
func (c *LocalClient) GetRegisteredServers() []peer.ID {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	servers := make([]peer.ID, 0, len(c.registeredServers))
	for server := range c.registeredServers {
		servers = append(servers, server)
	}
	return servers
}

// createIdentityProof creates a signed registration message to prove identity in proxy mode
func (c *LocalClient) createIdentityProof(server peer.ID) (*pb.Message_Registration, error) {
	ts := timestamppb.New(time.Now().Add(time.Minute * 5)) // Short-lived proof
	reg := &pb.Message_Registration{
		Expiry: ts,
		Server: []byte(server),
	}

	// Include public key for identity verification
	pubKeyBytes, err := crypto.MarshalPublicKey(c.privateKey.GetPublic())
	if err != nil {
		return nil, err
	}
	reg.Pubkey = pubKeyBytes

	// Sign the registration
	ser, err := proto.Marshal(reg)
	if err != nil {
		return nil, err
	}
	sig, err := c.privateKey.Sign(ser)
	if err != nil {
		return nil, err
	}
	reg.Signature = sig

	return reg, nil
}

// Error definitions
var (
	ErrNotRegistered    = &SNFError{"peer not registered with server"}
	ErrInvalidSignature = &SNFError{"invalid registration signature"}
	ErrStoreFailed      = &SNFError{"failed to store message"}
)

type SNFError struct {
	msg string
}

func (e *SNFError) Error() string {
	return e.msg
}
