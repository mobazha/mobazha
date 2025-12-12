package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	"github.com/ipfs/boxo/bootstrap"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/coreiface/options"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	opb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"gorm.io/gorm"

	nameSysOpts "github.com/ipfs/boxo/namesys"
)

const (
	// republishInterval is the amount of time to go between republishes.
	republishInterval = time.Hour * 36

	// nameValidTime is the amount of time an IPNS record is considered valid
	// after publish.
	nameValidTime = time.Hour * 24 * 30
)

// Publish will publish the current public data directory to IPNS.
// It will interrupt the publish if a shutdown happens during.
//
// This cannot be called with the database lock held.
func (n *MobazhaNode) Publish(done chan<- struct{}) {
	go func() {
		<-n.initialBootstrapChan
		n.publishChan <- pubCloser{done}
	}()
}

func (n *MobazhaNode) publish(ctx context.Context, done chan<- struct{}) {
	atomic.AddInt32(&n.publishActive, 1)

	publishID := rand.Intn(math.MaxInt32)
	n.eventBus.Emit(&events.PublishStarted{
		ID: publishID,
	})

	var publishErr error

	defer func() {
		atomic.AddInt32(&n.publishActive, -1)
		if publishErr == coreiface.ErrNothingToPublish {
			return
		}
		if publishErr != nil && publishErr != context.Canceled {
			n.eventBus.Emit(&events.PublishingError{
				Err: publishErr,
			})
		} else if publishErr == nil {
			n.eventBus.Emit(&events.PublishFinished{
				ID: publishID,
			})
			logger.LogInfoWithIDf(log, n.nodeID, "Publishing complete")
		}
	}()

	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-cctx.Done():
		case <-n.shutdown:
			cancel()
		}
		if done != nil {
			close(done)
		}
	}()

	api, err := coreapi.NewCoreAPI(n.ipfsNode)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error building core API: %s", err.Error())
		publishErr = err
		return
	}

	currentRoot, err := n.ipnsRecordValue(ctx)

	// First uppin old root hash
	if err == nil {
		rp, _, err := api.ResolvePath(context.Background(), path.FromCid(currentRoot))
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Error resolving path: %s", err.Error())
			publishErr = err
			return
		}

		if err := api.Pin().Rm(context.Background(), rp, options.Pin.RmRecursive(true)); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Error unpinning root: %s", err.Error())
		}
	}

	// Add the directory to IPFS
	stat, err := os.Lstat(n.repo.DB().PublicDataPath())
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error calling Lstat: %s", err.Error())
		publishErr = err
		return
	}

	f, err := files.NewSerialFile(n.repo.DB().PublicDataPath(), false, stat)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error serializing file: %s", err.Error())
		publishErr = err
		return
	}

	opts := []options.UnixfsAddOption{
		options.Unixfs.Pin(true),
	}
	pth, err := api.Unixfs().Add(cctx, files.ToDir(f), opts...)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error adding root: %s", err.Error())
		publishErr = err
		return
	}

	// If the state has not changed since last publish then just return.
	// The IPNS republisher is responsible for keeping our current IPNS
	// record alive.
	if pth.RootCid() == currentRoot {
		return
	}

	record, err := n.ipnsRecord(cctx)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error getting ipns record: %s", err.Error())
		publishErr = err
		return
	}

	if len(n.ipnsResolver) > 0 {
		err = net.SetIPNSRecordToResolver(cctx, n.ipnsResolver, n.Identity(), record)
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Error setting ipns record to resolver: %s", err.Error())
			publishErr = err
		}
	}

	// Publish
	go func() {
		logger.LogInfoWithIDf(log, n.nodeID, "Publishing to IPNS...")
		eol := time.Now().Add(nameValidTime)
		ipfsNode := n.SharedManager().GetIPFSNode()
		if err := ipfsNode.Namesys.Publish(cctx, ipfsNode.PrivateKey, pth, nameSysOpts.PublishWithEOL(eol)); err != nil {
			if err != context.Canceled {
				logger.LogErrorWithIDf(log, n.nodeID, "Error namesys publish: %s", err.Error())
			}
			publishErr = err
			return
		}
	}()

	// Publish to pubsub all records topic.
	go func() {
		logger.LogInfoWithIDf(log, n.nodeID, "Going to publish to pubsub:")

		if err := n.publishIPNSRecordToPubsub(context.Background()); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Error publishing IPNS record to pubsub: %s", err)
		}
	}()

	err = n.repo.DB().Update(func(tx database.Tx) error {
		return tx.Save(&models.Event{Name: "last_publish", Time: time.Now()})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error saving last publish time to the db: %s", err.Error())
	}

	// Send the new graph to our connected followers.
	graph, err := n.fetchGraph(cctx)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error fetching graph: %s", err.Error())
		publishErr = err
		return
	}

	storeMsg := &pb.StoreMessage{}
	for _, cid := range graph {
		storeMsg.Cids = append(storeMsg.Cids, cid.Bytes())
	}

	any := &anypb.Any{}
	if err := any.MarshalFrom(storeMsg); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error marshalling store message: %s", err.Error())
		publishErr = err
		return
	}

	snfServers := repo.DefaultMainnetSNFServers
	if n.testnet {
		snfServers = repo.DefaultTestnetSNFServers
	}
	svrMap := map[peer.ID]bool{}
	for _, snf := range snfServers {
		if peer, err := peer.Decode(snf); err == nil {
			svrMap[peer] = true
		}
	}

	msg := newMessageWithID()
	msg.MessageType = pb.Message_STORE
	msg.Payload = any
	for _, peer := range n.followerTracker.ConnectedFollowers() {
		if _, ok := svrMap[peer]; !ok {
			go n.networkService.SendMessage(context.Background(), peer, msg)
		}
	}

	for peer := range svrMap {
		go n.networkService.SendMessage(context.Background(), peer, msg)
	}
}

// PublishFile will publish the given file to SNF servers and followers for storage.
// It will interrupt the publish if a shutdown happens during.
func (n *MobazhaNode) PublishFile(ctx context.Context, cid cid.Cid, done chan<- struct{}) {
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-cctx.Done():
		case <-n.shutdown:
			cancel()
		}
		if done != nil {
			close(done)
		}
	}()

	storeMsg := &pb.StoreMessage{}
	storeMsg.Cids = append(storeMsg.Cids, cid.Bytes())

	any := &anypb.Any{}
	if err := any.MarshalFrom(storeMsg); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error marshalling store message: %s", err.Error())
		return
	}

	snfServers := repo.DefaultMainnetSNFServers
	if n.testnet {
		snfServers = repo.DefaultTestnetSNFServers
	}
	svrMap := map[peer.ID]bool{}
	for _, snf := range snfServers {
		if peer, err := peer.Decode(snf); err == nil {
			svrMap[peer] = true
		}
	}

	msg := newMessageWithID()
	msg.MessageType = pb.Message_STORE
	msg.Payload = any
	for _, peer := range n.followerTracker.ConnectedFollowers() {
		if _, ok := svrMap[peer]; !ok {
			go n.networkService.SendMessage(context.Background(), peer, msg)
		}
	}

	for peer := range svrMap {
		go n.networkService.SendMessage(context.Background(), peer, msg)
	}
}

// sendAckMessage saves the incoming message ID in the database so we can
// check for duplicate messages later. Then it sends the ACK message to
// the remote peer.
func (n *MobazhaNode) sendAckMessage(messageID string, to peer.ID) {
	err := n.repo.DB().Update(func(tx database.Tx) error {
		return tx.Save(&models.IncomingMessage{ID: messageID})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error saving incoming message ID to database: %s", err)
	}
	n.messenger.SendACK(messageID, to)
}

// handleAckMessage is the handler for the ACK message. It sends it off to the messenger
// for processing. If this is an order message it also sends it to the order processor
// to be recorded there as well.
func (n *MobazhaNode) handleAckMessage(from peer.ID, message *pb.Message) error {
	if message.MessageType != pb.Message_ACK {
		return errors.New("message is not type ACK")
	}
	ack := new(pb.AckMessage)
	if err := message.Payload.UnmarshalTo(ack); err != nil {
		return err
	}

	err := n.repo.DB().Update(func(tx database.Tx) error {
		var outgoingMessage models.OutgoingMessage
		if err := tx.Read().Where("id = ?", ack.AckedMessageID).First(&outgoingMessage).Error; err != nil {
			return err
		}
		if outgoingMessage.MessageType == pb.Message_ORDER.String() || outgoingMessage.MessageType == pb.Message_DISPUTE.String() {
			if err := n.orderProcessor.ProcessACK(tx, &outgoingMessage); err != nil {
				return err
			}
		}
		if err := n.messenger.ProcessACK(tx, ack); err != nil {
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	n.eventBus.Emit(&events.MessageACK{MessageID: ack.AckedMessageID})
	return nil
}

// handleOrderMessage is the handler for the ORDER message. It sends it off to the order
// order processor for processing.
func (n *MobazhaNode) handleOrderMessage(from peer.ID, message *pb.Message) error {
	defer n.sendAckMessage(message.MessageID, from)

	if n.isDuplicate(message) {
		return nil
	}

	if message.MessageType != pb.Message_ORDER {
		return errors.New("message is not type ORDER")
	}
	orderMsg := new(pb.OrderMessage)
	if err := message.Payload.UnmarshalTo(orderMsg); err != nil {
		return err
	}

	var event interface{}
	var order models.Order
	err := n.repo.DB().Update(func(tx database.Tx) error {
		tx.Read().Where("id = ?", orderMsg.OrderID).First(&order)

		var err error
		event, err = n.orderProcessor.ProcessMessage(tx, orderMsg)
		return err
	})
	if err != nil {
		return err
	}

	// Store ratings in NetDB
	go func() {
		if orderMsg.MessageType == pb.OrderMessage_ORDER_COMPLETE && n.netDB != nil {
			complete := new(opb.OrderComplete)
			if err := orderMsg.Message.UnmarshalTo(complete); err == nil {
				if order.Role() == models.RoleVendor && len(complete.Ratings) > 0 {
					if ratingIndex, err := n.GetMyRatings(); err == nil {
						n.netDB.SetOwnRatingIndex(ratingIndex)
					}
				}
			}
		}
	}()

	if event != nil {
		n.eventBus.Emit(event)
	}
	return nil
}

func (n *MobazhaNode) isSelfDefaultSNFServer() bool {
	snfServers := repo.DefaultMainnetSNFServers
	if n.testnet {
		snfServers = repo.DefaultTestnetSNFServers
	}
	svrMap := map[peer.ID]bool{}
	for _, snf := range snfServers {
		if peer, err := peer.Decode(snf); err == nil {
			svrMap[peer] = true
		}
	}

	return svrMap[n.Identity()]
}

// handleStoreMessage is the handler for the STORE message. It will download and
// pin any objects sent to it from its followers.
func (n *MobazhaNode) handleStoreMessage(from peer.ID, message *pb.Message) error {
	if message.MessageType != pb.Message_STORE {
		return errors.New("message is not type STORE")
	}

	if !n.isSelfDefaultSNFServer() {
		var (
			following models.Following
			err       error
		)
		err = n.repo.DB().View(func(tx database.Tx) error {
			following, err = tx.GetFollowing()
			return err
		})
		if err != nil {
			return err
		}
		if !following.IsFollowing(from) {
			return errors.New("STORE message from peer that is not followed")
		}
	}

	store := new(pb.StoreMessage)
	if err := message.Payload.UnmarshalTo(store); err != nil {
		return err
	}

	var cids []cid.Cid
	var failedCids []cid.Cid
	for _, b := range store.Cids {
		cid, err := cid.Cast(b)
		if err != nil {
			failedCids = append(failedCids, cid)
			logger.LogErrorWithIDf(log, n.nodeID, "store handler cid cast error, %s, %s", cid.String(), err)
			continue
		}
		cids = append(cids, cid)
		if err := n.pin(context.Background(), path.FromCid(cid)); err != nil {
			failedCids = append(failedCids, cid)
			logger.LogErrorWithIDf(log, n.nodeID, "store handler error pinning file, %s, %s", cid.String(), err)
			continue
		}
	}
	if len(failedCids) > 0 {
		return fmt.Errorf("store handler error for %d cids", len(failedCids))
	}
	n.eventBus.Emit(&events.MessageStore{
		Peer: from,
		Cids: cids,
	})
	logger.LogInfoWithIDf(log, n.nodeID, "Received STORE message from %s", from)
	return nil
}

// isDuplicate checks if the message ID exists in the incoming messages database.
func (n *MobazhaNode) isDuplicate(message *pb.Message) bool {
	err := n.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", message.MessageID).First(&models.IncomingMessage{}).Error
	})
	return err == nil
}

// syncMessages listens for new connections to peers and checks to see if we have
// any outgoing messages for them. If so we send the messages over the direct
// connection.
func (n *MobazhaNode) syncMessages() {
	connectedSub, err := n.eventBus.Subscribe(&events.PeerConnected{})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error subscribing to PeerConnected event: %s", err)
	}
	for {
		select {
		case event := <-connectedSub.Out():
			notif, ok := event.(*events.PeerConnected)
			if !ok {
				logger.LogErrorWithIDf(log, n.nodeID, "syncMessages type assertion failed on PeerConnected")
				return
			}
			var messages []models.OutgoingMessage
			err = n.repo.DB().View(func(tx database.Tx) error {
				return tx.Read().Where("recipient = ?", notif.Peer.String()).Find(&messages).Error
			})
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				logger.LogErrorWithIDf(log, n.nodeID, "syncMessages outgoing messages lookup error: %s", err)
				return
			}
			for _, om := range messages {
				// If a message is less than a second old it is likely that this connection
				// was established for the purpose of sending this message. In this case let's
				// skip this message so as to avoid sending an unnecessary duplicate.
				if time.Since(om.Timestamp) < time.Second {
					continue
				}
				var message pb.Message
				if err := proto.Unmarshal(om.SerializedMessage, &message); err != nil {
					logger.LogErrorWithIDf(log, n.nodeID, "syncMessages unmarshal error: %s", err)
					continue
				}
				recipient, err := peer.Decode(om.Recipient)
				if err != nil {
					logger.LogErrorWithIDf(log, n.nodeID, "syncMessages peer decode error: %s", err)
					continue
				}
				go n.networkService.SendMessage(context.Background(), recipient, &message)
			}
		case <-n.shutdown:
			return
		}
	}
}

// bootstrapIPFS bootstraps the IPFS node.
func (n *MobazhaNode) bootstrapIPFS() error {
	if err := n.ipfsNode.Bootstrap(bootstrap.DefaultBootstrapConfig); err != nil {
		return err
	}
	close(n.initialBootstrapChan)
	return nil
}

type pubCloser struct {
	done chan<- struct{}
}

// publishHandler is a loop that runs and handles IPNS record publishes and republishes. It shoots to
// republish 36 hours from the last publish so as to not slam the network on startup every time.
// If a current publish is active it will be canceled and the new publish will supersede it.
//
// The only reason we have this republish functionality at all is to publish ratings and followers/follows
// that do not otherwise trigger an automatic publish. So we essentially batch and publish these
// changes every 36 hours if the user does not trigger a publish in the interim.
func (n *MobazhaNode) publishHandler() {
	var lastPublish time.Time
	err := n.repo.DB().View(func(tx database.Tx) error {
		var event models.Event
		if err := tx.Read().Where("name = ?", "last_publish").First(&event).Error; err != nil {
			return err
		}
		lastPublish = event.Time
		return nil
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.LogErrorWithIDf(log, n.nodeID, "Error loading last republish time: %s", err.Error())
	}

	tick := time.After(republishInterval - time.Since(lastPublish))
	publishCtx, publishCancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-tick:
				lastPublish = time.Now()
				tick = time.After(republishInterval - time.Since(lastPublish))
				err = n.repo.DB().Update(func(tx database.Tx) error {
					return tx.Save(&models.Event{Name: "last_publish", Time: lastPublish})
				})
				if err != nil {
					logger.LogErrorWithIDf(log, n.nodeID, "Error saving last publish time to the db: %s", err.Error())
				}
				go n.Publish(nil)
			case p := <-n.publishChan:
				publishCancel()
				publishCtx, publishCancel = context.WithCancel(context.Background())
				lastPublish = time.Now()
				tick = time.After(republishInterval - time.Since(lastPublish))
				go n.publish(publishCtx, p.done)
			case <-n.shutdown:
				publishCancel()
				return
			}
		}
	}()
}
