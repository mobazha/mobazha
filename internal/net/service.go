package net

import (
	"context"
	"io"
	"sync"

	ctxio "github.com/jbenet/go-context/io"
	host "github.com/libp2p/go-libp2p/core/host"
	inet "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	protocol "github.com/libp2p/go-libp2p/core/protocol"
	msgio "github.com/libp2p/go-msgio"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"google.golang.org/protobuf/proto"
)

var log = logging.MustGetLogger("NET")

// LocalDeliverer is implemented by the hosting process to deliver messages
// between nodes that live in the same process. It is intentionally limited to
// in-process delivery; standalone/NAT peers continue through the regular
// libp2p/SNF messenger path.
type LocalDeliverer interface {
	DeliverToLocal(target peer.ID, from peer.ID, msg *pb.Message) (handled bool, err error)
}

// Compile-time check: *NetworkService implements contracts.NetworkService.
var _ contracts.NetworkService = (*NetworkService)(nil)

type NetworkService struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	nodeID string

	host host.Host

	messageSenders map[peer.ID]*messageSender

	msMtx sync.RWMutex

	handlers   map[pb.Message_MessageType]func(peerID peer.ID, msg *pb.Message) error
	handlerMtx sync.RWMutex

	banManager *BanManager

	protocolID protocol.ID

	// Optional same-process delivery before regular libp2p.
	localDeliverer LocalDeliverer
}

func NewNetworkService(nodeID string, host host.Host, banManager *BanManager, useTestnet bool) *NetworkService {
	ctx, cancel := context.WithCancel(context.Background())
	protocolID := ProtocolAppMainnetTwo
	if useTestnet {
		protocolID = ProtocolAppTestnetTwo
	}
	ns := &NetworkService{
		ctx:            ctx,
		ctxCancel:      cancel,
		nodeID:         nodeID,
		host:           host,
		messageSenders: make(map[peer.ID]*messageSender),
		msMtx:          sync.RWMutex{},
		handlers:       make(map[pb.Message_MessageType]func(peerID peer.ID, message *pb.Message) error),
		handlerMtx:     sync.RWMutex{},
		banManager:     banManager,
		protocolID:     protocol.ID(protocolID),
	}

	disConnected := func(_ inet.Network, conn inet.Conn) {
		ns.msMtx.Lock()
		defer ns.msMtx.Unlock()
		delete(ns.messageSenders, conn.RemotePeer())
	}
	notifier := &inet.NotifyBundle{
		DisconnectedF: disConnected,
	}
	host.Network().Notify(notifier)
	host.SetStreamHandler(ns.protocolID, ns.HandleNewStream)
	return ns
}

func (ns *NetworkService) Close() {
	ns.ctxCancel()
}

func (ns *NetworkService) RegisterHandler(messageType pb.Message_MessageType, handler func(peerID peer.ID, message *pb.Message) error) {
	ns.handlerMtx.Lock()
	defer ns.handlerMtx.Unlock()
	ns.handlers[messageType] = handler
}

// SetLocalDeliverer configures an in-process delivery adapter.
func (ns *NetworkService) SetLocalDeliverer(ld LocalDeliverer) {
	ns.localDeliverer = ld
}

// HandleNewStream receives new incoming streams from other peers.
// A stream is not a connection. You may already have an open connection
// with this peer over which you have been using other protocols. A stream
// is an abstraction which allows you to multiplex multiple streams of data
// over the same connection. Each stream does not technically need to be
// a different protocol. You could, for example, have multiple streams open
// to the same peer using the MobazhaProtocol. This would allow for each
// stream operating in parallel with each other *as if* each one were a
// different connection.
func (ns *NetworkService) HandleNewStream(s inet.Stream) {
	go ns.handleNewMessage(s)
}

func (ns *NetworkService) handleNewMessage(s inet.Stream) {
	defer s.Close()
	contextReader := ctxio.NewReader(ns.ctx, s)
	reader := msgio.NewVarintReaderSize(contextReader, inet.MessageSizeMax)
	remotePeer := s.Conn().RemotePeer()

	if ns.banManager.IsBanned(remotePeer) {
		logger.LogInfoWithIDf(log, ns.nodeID, "Received new stream request from banned peer %s. Closing.", remotePeer)
		return
	}

	for {
		select {
		case <-ns.ctx.Done():
			return
		default:
		}

		pmes := new(pb.Message)
		msgBytes, err := reader.ReadMsg()
		if err != nil {
			reader.ReleaseMsg(msgBytes)
			s.Reset()
			if err == io.EOF {
				logger.LogInfoWithIDf(log, ns.nodeID, "Peer %s closed stream", remotePeer)
			}
			return
		}
		if err := proto.Unmarshal(msgBytes, pmes); err != nil {
			reader.ReleaseMsg(msgBytes)
			s.Reset()
			return
		}
		reader.ReleaseMsg(msgBytes)
		// Check again
		if ns.banManager.IsBanned(remotePeer) {
			logger.LogInfoWithIDf(log, ns.nodeID, "Received message from banned peer %s. Closing.", remotePeer)
			return
		}

		ns.handlerMtx.RLock()
		handler, ok := ns.handlers[pmes.MessageType]
		if !ok {
			logger.LogInfoWithIDf(log, ns.nodeID, "Received message type %s with unregistered handler", pmes.MessageType.String())
			ns.handlerMtx.RUnlock()
			continue
		}
		ns.handlerMtx.RUnlock()
		if err := handler(remotePeer, pmes); err != nil {
			logger.LogInfoWithIDf(log, ns.nodeID, "Error processing %s message from %s: %s", pmes.MessageType.String(), remotePeer, err)
		}
	}
}

func (ns *NetworkService) SendMessage(ctx context.Context, peerID peer.ID, message *pb.Message) error {
	if ns.localDeliverer != nil {
		from := ns.host.ID()
		handled, err := ns.localDeliverer.DeliverToLocal(peerID, from, message)
		if handled {
			if err != nil {
				return err
			}
			logger.LogDebugWithIDf(log, ns.nodeID, "Message %s delivered locally to %s", message.MessageID, peerID)
			return nil
		}
		logger.LogDebugWithIDf(log, ns.nodeID, "Local delivery unavailable for %s, falling back to network", peerID)
	}

	ms, err := ns.messageSenderForPeer(ctx, peerID)
	if err != nil {
		return err
	}
	return ms.sendMessage(ctx, message)
}

// DeliverLocalMessage 提供同进程本地直达的消息分发路径，避免经过网络栈。
// 要求调用方仅在“同进程本机节点”场景使用。
func (ns *NetworkService) DeliverLocalMessage(from peer.ID, pmes *pb.Message) error {
	// 尺寸限制与安全检查，保持与网络路径行为一致
	if pmes == nil {
		return nil
	}
	if ns.banManager != nil && ns.banManager.IsBanned(from) {
		logger.LogInfoWithIDf(log, ns.nodeID, "Received local message from banned peer %s. Dropping.", from)
		return nil
	}

	ns.handlerMtx.RLock()
	handler, ok := ns.handlers[pmes.MessageType]
	ns.handlerMtx.RUnlock()
	if !ok {
		logger.LogInfoWithIDf(log, ns.nodeID, "Received local message type %s with unregistered handler", pmes.MessageType.String())
		return nil
	}
	return handler(from, pmes)
}
