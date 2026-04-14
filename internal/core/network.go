package core

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	opb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"gorm.io/gorm"
)

const (
	// republishInterval is the amount of time to go between republishes.
	// Used to batch-publish rating/follower changes that don't trigger
	// an immediate publish.
	republishInterval = time.Hour * 36
)

// Publish computes a content hash of public data records in the database and
// sends STORE messages to followers and SNF servers for replication notification.
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

	// Load last published root CID from database.
	var currentRoot cid.Cid
	_ = n.db.View(func(tx database.Tx) error {
		var event models.Event
		if err := tx.Read().Where("name = ?", "last_publish").First(&event).Error; err != nil {
			return err
		}
		if event.Value != "" {
			currentRoot, _ = cid.Decode(event.Value)
		}
		return nil
	})

	// Compute public data content hash directly from DB for change detection.
	newRoot, err := n.db.ComputePublicDataHash()
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error computing public data hash: %s", err.Error())
		publishErr = err
		return
	}

	// No-change detection: if data hasn't changed, skip replication.
	if newRoot == currentRoot {
		return
	}

	// Persist new root CID and publish timestamp.
	err = n.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Event{
			Name:  "last_publish",
			Time:  time.Now(),
			Value: newRoot.String(),
		})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Error saving last publish to the db: %s", err.Error())
	}

	// Send STORE message with root CID to followers/SNF servers for notification.
	storeMsg := &pb.StoreMessage{}
	storeMsg.Cids = append(storeMsg.Cids, newRoot.Bytes())

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

	go func() {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer sendCancel()

		for _, p := range n.followerTracker.ConnectedFollowers() {
			if _, ok := svrMap[p]; !ok {
				wg.Add(1)
				sem <- struct{}{}
				go func(target peer.ID) {
					defer wg.Done()
					defer func() { <-sem }()
					n.networkService.SendMessage(sendCtx, target, msg)
				}(p)
			}
		}

		for p := range svrMap {
			wg.Add(1)
			sem <- struct{}{}
			go func(target peer.ID) {
				defer wg.Done()
				defer func() { <-sem }()
				n.networkService.SendMessage(sendCtx, target, msg)
			}(p)
		}
		wg.Wait()
	}()
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

	// 有界并发发送消息（后台执行，不阻塞 PublishFile 返回）
	go func() {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer sendCancel()

		for _, p := range n.followerTracker.ConnectedFollowers() {
			if _, ok := svrMap[p]; !ok {
				wg.Add(1)
				sem <- struct{}{}
				go func(target peer.ID) {
					defer wg.Done()
					defer func() { <-sem }()
					n.networkService.SendMessage(sendCtx, target, msg)
				}(p)
			}
		}

		for p := range svrMap {
			wg.Add(1)
			sem <- struct{}{}
			go func(target peer.ID) {
				defer wg.Done()
				defer func() { <-sem }()
				n.networkService.SendMessage(sendCtx, target, msg)
			}(p)
		}
		wg.Wait()
	}()
}

// sendAckMessage saves the incoming message ID in the database so we can
// check for duplicate messages later. Then it sends the ACK message to
// the remote peer.
func (n *MobazhaNode) sendAckMessage(messageID string, to peer.ID) {
	err := n.db.Update(func(tx database.Tx) error {
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

	err := n.db.Update(func(tx database.Tx) error {
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

// handleOrderMessage is the handler for the ORDER message. It delegates to
// OrderAppService.HandleIncomingOrderMessage which orchestrates per-order
// locking, pre-processing I/O, deterministic ProcessMessage, and
// post-processing within a single DB transaction.
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

	event, order, err := n.orderService.HandleIncomingOrderMessage(n.nodeCtx, orderMsg)
	if err != nil {
		return err
	}

	// Emit ratings event for NetDBSyncService
	if orderMsg.MessageType == pb.OrderMessage_ORDER_COMPLETE && n.eventBus != nil {
		complete := new(opb.OrderComplete)
		if err := orderMsg.Message.UnmarshalTo(complete); err == nil {
			if order.Role() == models.RoleVendor && len(complete.Ratings) > 0 {
				n.eventBus.Emit(&events.RatingsChanged{Ratings: complete.Ratings})
			}
		}
	}

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
		err = n.db.View(func(tx database.Tx) error {
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
	for _, b := range store.Cids {
		c, err := cid.Cast(b)
		if err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "store handler cid cast error: %s", err)
			continue
		}
		cids = append(cids, c)
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
	err := n.db.View(func(tx database.Tx) error {
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
			err = n.db.View(func(tx database.Tx) error {
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

// bootstrapDHT starts the DHT bootstrap process.
// For lightweight nodes (p2pInfra == nil), skip bootstrap and signal ready immediately.
func (n *MobazhaNode) bootstrapDHT() error {
	if n.p2pInfra == nil || n.p2pInfra.DHT == nil {
		close(n.initialBootstrapChan)
		return nil
	}
	if err := n.p2pInfra.DHT.Bootstrap(n.p2pInfra.Ctx); err != nil {
		return err
	}
	close(n.initialBootstrapChan)
	return nil
}

type pubCloser struct {
	done chan<- struct{}
}

// publishHandler manages the publish loop. It periodically re-publishes
// (every 36 hours) to batch-publish rating/follower changes that don't
// otherwise trigger an immediate publish. If a new publish is requested
// while one is active, the active publish is canceled.
func (n *MobazhaNode) publishHandler() {
	var lastPublish time.Time
	err := n.db.View(func(tx database.Tx) error {
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
