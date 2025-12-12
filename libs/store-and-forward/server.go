package storeandforward

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log"
	ctxio "github.com/jbenet/go-context/io"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	inet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	msgio "github.com/libp2p/go-msgio"
	"github.com/mobazha/mobazha3.0/libs/store-and-forward/pb"
	"github.com/multiformats/go-base32"
)

const (
	protectionTag         = "store-and-forward"
	registrationKeyPrefix = "/snf/registeredPeer/"
	messageKeyPrefix      = "/snf/message/"
)

var log = logging.Logger("snf")

// Server is a store and forward server which can be used for asynchronous
// communication between peers on the network.
type Server struct {
	host             host.Host
	ctx              context.Context
	ds               datastore.Datastore
	replicationPeers map[peer.ID]inet.Stream
	serverProtocol   protocol.ID
	clientProtocol   protocol.ID
	mtx              sync.RWMutex
}

// NewServer returns a new store and forward server.
func NewServer(ctx context.Context, h host.Host, opts ...Option) (*Server, error) {
	var cfg Options
	if err := cfg.Apply(append([]Option{Defaults}, opts...)...); err != nil {
		return nil, err
	}

	if len(cfg.ServerProtocols) == 0 {
		return nil, errors.New("server protocol option is required")
	}

	if len(cfg.ClientProtocols) == 0 {
		return nil, errors.New("client protocol option is required")
	}

	repPeersMap := make(map[peer.ID]inet.Stream)
	for _, p := range cfg.ReplicationPeers {
		h.ConnManager().Protect(p, protectionTag)
		repPeersMap[p] = nil
	}

	s := &Server{
		host:             h,
		ctx:              ctx,
		ds:               cfg.Datastore,
		serverProtocol:   cfg.ServerProtocols[0],
		clientProtocol:   cfg.ClientProtocols[0],
		replicationPeers: repPeersMap,
		mtx:              sync.RWMutex{},
	}

	for _, protocol := range cfg.ServerProtocols {
		h.SetStreamHandler(protocol, s.handleNewStream)
	}

	return s, nil
}

func (svr *Server) handleNewStream(s inet.Stream) {
	go svr.streamHandler(s)
}

// streamSession tracks the authenticated client identity for a stream (for proxy mode support)
type streamSession struct {
	authenticatedID peer.ID // The actual authenticated client ID (may differ in proxy mode)
}

func (svr *Server) streamHandler(s inet.Stream) {
	defer s.Close()
	contextReader := ctxio.NewReader(svr.ctx, s)
	reader := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
	writer := msgio.NewVarintWriter(s)
	remotePeer := s.Conn().RemotePeer()

	// Session tracking for proxy mode
	session := &streamSession{
		authenticatedID: remotePeer, // Default to connection peer
	}

	defer func() {
		svr.mtx.Lock()
		if _, ok := svr.replicationPeers[remotePeer]; ok {
			svr.replicationPeers[remotePeer] = nil
		}
		svr.mtx.Unlock()
	}()

	for {
		select {
		case <-svr.ctx.Done():
			return
		default:
		}

		pmes := new(pb.Message)
		msgBytes, err := reader.ReadMsg()
		if err != nil {
			reader.ReleaseMsg(msgBytes)
			s.Reset()
			if err == io.EOF {
				log.Debugf("peer %s closed stream", remotePeer)
			}
			return
		}
		if err := proto.Unmarshal(msgBytes, pmes); err != nil {
			reader.ReleaseMsg(msgBytes)
			s.Reset()
			return
		}
		reader.ReleaseMsg(msgBytes)

		switch pmes.Type {
		case pb.Message_REGISTER:
			var clientID peer.ID
			clientID, err = svr.handleRegisterWithSession(writer, pmes, remotePeer)
			if err == nil && clientID != "" {
				session.authenticatedID = clientID
			}
		case pb.Message_UNREGISTER:
			err = svr.handleUnregister(writer, pmes, session.authenticatedID)
		case pb.Message_GET_MESSAGES:
			var clientID peer.ID
			clientID, err = svr.handleGetMessagesWithSession(writer, pmes, remotePeer)
			if err == nil && clientID != "" {
				session.authenticatedID = clientID
			}
		case pb.Message_MESSAGE_ACK:
			err = svr.handleAckMessage(writer, pmes, session.authenticatedID)
		case pb.Message_PROVE_REGISTRATION:
			err = svr.handleProveRegistrationMessage(writer, pmes, remotePeer)
		case pb.Message_STORE_MESSAGE:
			err = svr.handleStoreMessage(writer, pmes, remotePeer)
		case pb.Message_GET_MESSAGE:
			if _, ok := svr.replicationPeers[remotePeer]; !ok {
				err = writeStatusMessage(writer, pb.Message_UNAUTHORIZED)
				break
			}
			err = svr.handleGetMessage(writer, pmes, remotePeer)
		case pb.Message_REPLICATE:
			if _, ok := svr.replicationPeers[remotePeer]; !ok {
				err = writeStatusMessage(writer, pb.Message_UNAUTHORIZED)
				break
			}
			err = svr.handleReplicateMessage(writer, pmes, remotePeer)
		case pb.Message_MESSAGE:
			if _, ok := svr.replicationPeers[remotePeer]; !ok {
				err = writeStatusMessage(writer, pb.Message_UNAUTHORIZED)
				break
			}
			err = svr.handleMessageMessage(writer, pmes, remotePeer)
		}
		if err != nil {
			log.Errorf("Peer %s: Error handling %s message: %s", remotePeer, pmes.Type, err)
		}
	}
}

// handleRegister saves a user registration in the db. Duplicate registrations are allowed
// and a prior registration is overridden.
// handleRegisterWithSession handles registration and returns the authenticated client ID
func (svr *Server) handleRegisterWithSession(w msgio.Writer, pmes *pb.Message, from peer.ID) (peer.ID, error) {
	log.Debugf("handleRegister: peer %s", from)
	regMsg := pmes.GetRegistration()
	if regMsg == nil {
		return "", writeStatusMessage(w, pb.Message_MALFORMED_MESSAGE)
	}
	if peer.ID(regMsg.Server) != svr.host.ID() {
		return "", writeStatusMessage(w, pb.Message_PEERID_INVALID)
	}

	var (
		pubKey   crypto.PubKey
		err      error
		clientID peer.ID // The actual client identity (may differ from connection peer in proxy mode)
	)

	// If pubkey is provided in the message, use it to derive the client identity
	// This supports "proxy mode" where multiple clients connect through a shared transport
	if regMsg.GetPubkey() != nil {
		pubKey, err = crypto.UnmarshalPublicKey(regMsg.GetPubkey())
		if err != nil {
			return "", writeStatusMessage(w, pb.Message_PUBKEY_INVALID)
		}
		clientID, err = peer.IDFromPublicKey(pubKey)
		if err != nil {
			return "", writeStatusMessage(w, pb.Message_PUBKEY_INVALID)
		}
		log.Debugf("handleRegister: proxy mode, client %s via transport %s", clientID, from)
	} else {
		// Traditional mode: use connection peer ID
		pubKey, err = from.ExtractPublicKey()
		if err != nil {
			return "", writeStatusMessage(w, pb.Message_PUBKEY_INVALID)
		}
		clientID = from
	}

	// Verify signature with the provided/derived public key
	m := proto.Clone(regMsg)
	regCpy := m.(*pb.Message_Registration)
	regCpy.Signature = nil
	sigSer, err := proto.Marshal(regCpy)
	if err != nil {
		return "", err
	}
	valid, err := pubKey.Verify(sigSer, regMsg.Signature)
	if err != nil {
		return "", err
	}
	if !valid {
		return "", writeStatusMessage(w, pb.Message_SIGNATURE_INVALID)
	}

	ser, err := proto.Marshal(regMsg)
	if err != nil {
		return "", err
	}

	// Store registration using the actual client identity
	err = svr.ds.Put(svr.ctx, registrationKey(clientID), ser)
	if err != nil {
		return "", err
	}

	if err := writeStatusMessage(w, pb.Message_SUCCESS); err != nil {
		return "", err
	}
	return clientID, nil
}

// handleUnregister unregisters a peer from this server.
func (svr *Server) handleUnregister(w msgio.Writer, _ *pb.Message, from peer.ID) error {
	log.Debugf("handleUnregister: peer %s", from)
	err := svr.ds.Delete(svr.ctx, registrationKey(from))
	if err != nil {
		return err
	}
	return writeStatusMessage(w, pb.Message_SUCCESS)
}

// handleProveRegistrationMessage returns the peer's registration info if it exists.
func (svr *Server) handleProveRegistrationMessage(w msgio.Writer, pmes *pb.Message, from peer.ID) error {
	log.Debugf("handleProveRegistration: peer %s", from)
	ids := pmes.GetIds()
	if ids == nil {
		return writeStatusMessage(w, pb.Message_MALFORMED_MESSAGE)
	}
	record, err := svr.ds.Get(svr.ctx, registrationKey(peer.ID(ids.PeerID)))
	if err != nil && err == datastore.ErrNotFound {
		return writeStatusMessage(w, pb.Message_NOT_REGISTERED)
	}

	reg := new(pb.Message_Registration)
	err = proto.Unmarshal(record, reg)
	if err != nil {
		return err
	}
	expiry, err := ptypes.Timestamp(reg.Expiry)
	if err != nil {
		return err
	}
	if expiry.Before(time.Now()) {
		err := svr.ds.Delete(svr.ctx, registrationKey(peer.ID(ids.PeerID)))
		if err != nil {
			return err
		}
		return writeStatusMessage(w, pb.Message_NOT_REGISTERED)
	}
	return writeMsgWithTimeout(w, &pb.Message{
		Type: pb.Message_RESPONSE,
		Payload: &pb.Message_Registration_{
			Registration: reg,
		},
	})
}

// handleReplicateMessage checks the db for the message and if it doesn't exist it requests it.
// This method may only be used by a replication peer.
func (svr *Server) handleReplicateMessage(w msgio.Writer, pmes *pb.Message, from peer.ID) error {
	log.Debugf("handleReplicate: peer %s", from)
	ids := pmes.GetIds()
	if ids == nil {
		return writeStatusMessage(w, pb.Message_MALFORMED_MESSAGE)
	}

	has, err := svr.ds.Has(svr.ctx, messageKey(peer.ID(ids.PeerID), ids.MessageID))
	if err != nil {
		return err
	}
	if !has {
		return writeMsgWithTimeout(w, &pb.Message{
			Type: pb.Message_GET_MESSAGE,
			Payload: &pb.Message_Ids{
				Ids: &pb.Message_IDs{
					MessageID: ids.MessageID,
					PeerID:    ids.PeerID,
				},
			},
		})
	}
	return nil
}

// handleMessageMessage saves the message straight into the db.
// This method may only be used by a replication peer.
func (svr *Server) handleMessageMessage(w msgio.Writer, pmes *pb.Message, from peer.ID) error {
	log.Debugf("handleMessage: peer %s", from)
	enc := pmes.GetEncryptedMessage()
	if enc == nil {
		return writeStatusMessage(w, pb.Message_MALFORMED_MESSAGE)
	}
	return svr.ds.Put(svr.ctx, messageKey(peer.ID(enc.PeerID), enc.MessageID), enc.Message)
}

// handleGetMessage loads a specific message from the db and returns it. This method
// may only be used by a replication peer.
func (svr *Server) handleGetMessage(w msgio.Writer, pmes *pb.Message, from peer.ID) error {
	log.Debugf("handleGetMessage: peer %s", from)
	ids := pmes.GetIds()
	if ids == nil {
		return writeStatusMessage(w, pb.Message_MALFORMED_MESSAGE)
	}

	message, err := svr.ds.Get(svr.ctx, messageKey(peer.ID(ids.PeerID), ids.MessageID))
	if err != nil {
		return err
	}
	return writeMsgWithTimeout(w, &pb.Message{
		Type: pb.Message_MESSAGE,
		Payload: &pb.Message_EncryptedMessage_{
			EncryptedMessage: &pb.Message_EncryptedMessage{
				MessageID: ids.MessageID,
				Message:   message,
				PeerID:    ids.PeerID,
			},
		},
	})
}

// handleGetMessages loads all the messages for the given peer from the database and sends
// them in separate MESSAGE messages.
// handleGetMessagesWithSession handles GET_MESSAGES and returns the authenticated client ID
func (svr *Server) handleGetMessagesWithSession(w msgio.Writer, pmes *pb.Message, connPeer peer.ID) (peer.ID, error) {
	log.Debugf("handleGetMessages: connection peer %s", connPeer)

	// Determine the actual client identity
	// In proxy mode, the request may include a Registration with pubkey to identify the client
	clientID := connPeer
	if reg := pmes.GetRegistration(); reg != nil && reg.GetPubkey() != nil {
		// Proxy mode: verify the client identity from the registration
		pubKey, err := crypto.UnmarshalPublicKey(reg.GetPubkey())
		if err != nil {
			return "", writeStatusMessage(w, pb.Message_PUBKEY_INVALID)
		}
		clientID, err = peer.IDFromPublicKey(pubKey)
		if err != nil {
			return "", writeStatusMessage(w, pb.Message_PUBKEY_INVALID)
		}

		// Verify signature to prove ownership of the private key
		m := proto.Clone(reg)
		regCpy := m.(*pb.Message_Registration)
		regCpy.Signature = nil
		sigSer, err := proto.Marshal(regCpy)
		if err != nil {
			return "", err
		}
		valid, err := pubKey.Verify(sigSer, reg.Signature)
		if err != nil || !valid {
			return "", writeStatusMessage(w, pb.Message_SIGNATURE_INVALID)
		}
		log.Debugf("handleGetMessages: proxy mode, client %s via transport %s", clientID, connPeer)
	}

	record, err := svr.ds.Get(svr.ctx, registrationKey(clientID))
	if err != nil && err == datastore.ErrNotFound {
		return "", writeStatusMessage(w, pb.Message_NOT_REGISTERED)
	}

	reg := new(pb.Message_Registration)
	err = proto.Unmarshal(record, reg)
	if err != nil {
		return "", err
	}
	expiry, err := ptypes.Timestamp(reg.Expiry)
	if err != nil {
		return "", err
	}
	if expiry.Before(time.Now()) {
		err := svr.ds.Delete(svr.ctx, registrationKey(clientID))
		if err != nil {
			return "", err
		}
		return "", writeStatusMessage(w, pb.Message_NOT_REGISTERED)
	}

	q := query.Query{
		Prefix: messageKeyPrefix + clientID.String(),
	}
	results, err := svr.ds.Query(svr.ctx, q)
	if err != nil {
		return "", err
	}

	for {
		result, more := results.NextSync()
		if !more {
			err := writeMsgWithTimeout(w, &pb.Message{
				Type: pb.Message_MESSAGE,
				Payload: &pb.Message_EncryptedMessage_{
					EncryptedMessage: &pb.Message_EncryptedMessage{
						More: more,
					},
				},
			})
			if err != nil {
				return "", err
			}
			return clientID, nil
		}

		s := strings.Split(result.Key, "/")

		messageID, err := base32.RawStdEncoding.DecodeString(s[4])
		if err != nil {
			return "", err
		}

		err = writeMsgWithTimeout(w, &pb.Message{
			Type: pb.Message_MESSAGE,
			Payload: &pb.Message_EncryptedMessage_{
				EncryptedMessage: &pb.Message_EncryptedMessage{
					MessageID: messageID,
					Message:   result.Value,
					More:      more,
				},
			},
		})
		if err != nil {
			return "", err
		}
	}
}

// handleAckMessage deletes the message with the provided ID from the database. The client
// should take care to make sure it is fully committed on the client side before acking
// the message.
func (svr *Server) handleAckMessage(w msgio.Writer, pmes *pb.Message, peer peer.ID) error {
	log.Debugf("handleAck: peer %s", peer)
	record, err := svr.ds.Get(svr.ctx, registrationKey(peer))
	if err != nil && err == datastore.ErrNotFound {
		return writeStatusMessage(w, pb.Message_NOT_REGISTERED)
	}

	reg := new(pb.Message_Registration)
	err = proto.Unmarshal(record, reg)
	if err != nil {
		return err
	}
	expiry, err := ptypes.Timestamp(reg.Expiry)
	if err != nil {
		return err
	}
	if expiry.Before(time.Now()) {
		err := svr.ds.Delete(svr.ctx, registrationKey(peer))
		if err != nil {
			return err
		}
		return writeStatusMessage(w, pb.Message_NOT_REGISTERED)
	}

	ack := pmes.GetAck()
	if ack == nil {
		return writeStatusMessage(w, pb.Message_MALFORMED_MESSAGE)
	}

	if err := svr.ds.Delete(svr.ctx, messageKey(peer, ack.MessageID)); err != nil {
		return err
	}

	return writeStatusMessage(w, pb.Message_SUCCESS)
}

// handleStoreMessage stores the given message in the db with a random messageID.
// If the peer is not registered with this server we return an error.
// Further, we check to see if the recipient is connected to us and if so relay
// the message to them.
func (svr *Server) handleStoreMessage(w msgio.Writer, pmes *pb.Message, from peer.ID) error {
	log.Debugf("handleStore: peer %s", from)
	encMsg := pmes.GetEncryptedMessage()
	if encMsg == nil {
		return writeStatusMessage(w, pb.Message_MALFORMED_MESSAGE)
	}
	if encMsg.GetPeerID() == nil {
		return writeStatusMessage(w, pb.Message_PEERID_INVALID)
	}

	to := peer.ID(encMsg.GetPeerID())

	record, err := svr.ds.Get(svr.ctx, registrationKey(to))
	if err != nil && err == datastore.ErrNotFound {
		return writeStatusMessage(w, pb.Message_NOT_REGISTERED)
	}

	reg := new(pb.Message_Registration)
	err = proto.Unmarshal(record, reg)
	if err != nil {
		return err
	}
	expiry, err := ptypes.Timestamp(reg.Expiry)
	if err != nil {
		return err
	}
	if expiry.Before(time.Now()) {
		err := svr.ds.Delete(svr.ctx, registrationKey(to))
		if err != nil {
			return err
		}
		return writeStatusMessage(w, pb.Message_NOT_REGISTERED)
	}

	id := sha256.Sum256(append([]byte(from), encMsg.Message...))

	if err := svr.ds.Put(svr.ctx, messageKey(to, id[:]), encMsg.Message); err != nil {
		return err
	}

	go func() {
		connectedness := svr.host.Network().Connectedness(to)
		if connectedness == inet.Connected {
			stream, err := svr.host.NewStream(inet.WithAllowLimitedConn(svr.ctx, "identify"), to, svr.clientProtocol)
			if err != nil {
				log.Errorf("Error relaying message to connected peer %s: %s", to, err)
				return
			}
			defer stream.Close()

			writer := msgio.NewVarintWriter(stream)
			err = writeMsgWithTimeout(writer, &pb.Message{
				Type: pb.Message_MESSAGE,
				Payload: &pb.Message_EncryptedMessage_{
					EncryptedMessage: &pb.Message_EncryptedMessage{
						MessageID: id[:],
						Message:   encMsg.Message,
						PeerID:    []byte(to), // Include target peer ID for routing in proxy mode
					},
				},
			})
			if err != nil {
				log.Errorf("Error relaying message to connected peer %s: %s", to, err)
			}
		}
	}()

	svr.mtx.RLock()
	for p, s := range svr.replicationPeers {
		go func(p peer.ID, s inet.Stream) {
			if s == nil {
				s, err = svr.host.NewStream(inet.WithAllowLimitedConn(svr.ctx, "identify"), p, svr.serverProtocol)
				if err != nil {
					log.Errorf("Error replicating message to peer %s: %s", p, err)
					return
				}
				svr.mtx.Lock()
				svr.replicationPeers[p] = s
				svr.mtx.Unlock()
				svr.handleNewStream(s)
			}

			writer := msgio.NewVarintWriter(s)
			err = writeMsgWithTimeout(writer, &pb.Message{
				Type: pb.Message_REPLICATE,
				Payload: &pb.Message_Ids{
					Ids: &pb.Message_IDs{
						MessageID: id[:],
						PeerID:    []byte(to),
					},
				},
			})
			if err != nil {
				log.Errorf("Error writing REPLICATE message to peer %s: %s", p, err)
			}
		}(p, s)
	}
	svr.mtx.RUnlock()
	return writeStatusMessage(w, pb.Message_SUCCESS)
}

func writeStatusMessage(w msgio.Writer, code pb.Message_Status) error {
	return writeMsgWithTimeout(w, &pb.Message{
		Type: pb.Message_STATUS,
		Code: code,
	})
}

func registrationKey(p peer.ID) datastore.Key {
	return datastore.NewKey(registrationKeyPrefix + p.String())
}

func messageKey(p peer.ID, messageID []byte) datastore.Key {
	id := base32.RawStdEncoding.EncodeToString(messageID)
	return datastore.NewKey(messageKeyPrefix + p.String() + "/" + id)
}
