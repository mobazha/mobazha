package core

import (
	"context"
	"errors"
	"fmt"
	"os"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database/netdb"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	"gorm.io/gorm"
)

type UpdateAndSaveProfileFunc func(tx database.Tx) error
type GetMyProfileFunc func() (*models.Profile, error)

type FollowAppService struct {
	db       database.Database
	messenger contracts.Messenger
	eventBus  events.Bus
	nodeID    string
	netDB     *netdb.NetDB

	coTenantPublicData   contracts.CoTenantPublicDataFn
	updateAndSaveProfile UpdateAndSaveProfileFunc
	getMyProfile         GetMyProfileFunc
}

type FollowAppServiceConfig struct {
	DB       database.Database
	Messenger contracts.Messenger
	EventBus  events.Bus
	NodeID    string
	NetDB     *netdb.NetDB

	CoTenantPublicData   contracts.CoTenantPublicDataFn
	UpdateAndSaveProfile UpdateAndSaveProfileFunc
	GetMyProfile         GetMyProfileFunc
}

func NewFollowAppService(cfg FollowAppServiceConfig) *FollowAppService {
	return &FollowAppService{
		db:                   cfg.DB,
		messenger:            cfg.Messenger,
		eventBus:             cfg.EventBus,
		nodeID:               cfg.NodeID,
		netDB:                cfg.NetDB,
		coTenantPublicData:   cfg.CoTenantPublicData,
		updateAndSaveProfile: cfg.UpdateAndSaveProfile,
		getMyProfile:         cfg.GetMyProfile,
	}
}

func (s *FollowAppService) FollowNode(peerID peer.ID, done chan<- struct{}) error {
	err := s.db.Update(func(tx database.Tx) error {
		following, err := tx.GetFollowing()
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		for _, p := range following {
			if p == peerID.String() {
				return fmt.Errorf("%w: already following peer", coreiface.ErrBadRequest)
			}
		}

		var seq models.FollowSequence
		if err := tx.Read().Where("peer_id = ?", peerID.String()).First(&seq).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		seq.Num++
		seq.PeerID = peerID.String()
		if err := tx.Save(&seq); err != nil {
			return err
		}

		following = append(following, peerID.String())

		if err := tx.SetFollowing(following); err != nil {
			return err
		}

		if s.updateAndSaveProfile != nil {
			if err := s.updateAndSaveProfile(tx); err != nil {
				return err
			}
		}

		msg := newMessageWithID()
		msg.MessageType = pb.Message_FOLLOW
		msg.Sequence = uint32(seq.Num)

		logger.LogDebugWithIDf(log, s.nodeID, "Sending FOLLOW message to %s. MessageID: %s", peerID, msg.MessageID)
		if err := s.messenger.ReliablySendMessage(tx, peerID, msg, done); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}

	s.syncFollowingToNetDB()
	return nil
}

func (s *FollowAppService) UnfollowNode(peerID peer.ID, done chan<- struct{}) error {
	err := s.db.Update(func(tx database.Tx) error {
		following, err := tx.GetFollowing()
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		exists := false
		for i, pid := range following {
			if pid == peerID.String() {
				exists = true
				following = append(following[:i], following[i+1:]...)
				break
			}
		}
		if !exists {
			return fmt.Errorf("%w: not following peer", coreiface.ErrBadRequest)
		}

		var seq models.FollowSequence
		if err := tx.Read().Where("peer_id = ?", peerID.String()).First(&seq).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		seq.PeerID = peerID.String()
		seq.Num++
		if err := tx.Save(&seq); err != nil {
			return err
		}

		if err := tx.SetFollowing(following); err != nil {
			return err
		}

		if s.updateAndSaveProfile != nil {
			if err := s.updateAndSaveProfile(tx); err != nil {
				return err
			}
		}

		msg := newMessageWithID()
		msg.MessageType = pb.Message_UNFOLLOW
		msg.Sequence = uint32(seq.Num)

		logger.LogDebugWithIDf(log, s.nodeID, "Sending UNFOLLOW message to %s. MessageID: %s", peerID, msg.MessageID)
		if err := s.messenger.ReliablySendMessage(tx, peerID, msg, done); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}

	s.syncFollowingToNetDB()
	return nil
}

func (s *FollowAppService) FollowsMe(peerID peer.ID) (bool, error) {
	var seq models.FollowSequence
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("peer_id = ?", peerID.String()).First(&seq).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	return err == nil, nil
}

func (s *FollowAppService) GetMyFollowers() (models.Followers, error) {
	var (
		followers models.Followers
		err       error
	)
	err = s.db.View(func(tx database.Tx) error {
		followers, err = tx.GetFollowers()
		return err
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return followers, nil
}

func (s *FollowAppService) GetMyFollowing() (models.Following, error) {
	var (
		following models.Following
		err       error
	)
	err = s.db.View(func(tx database.Tx) error {
		following, err = tx.GetFollowing()
		return err
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return following, nil
}

func (s *FollowAppService) GetFollowers(_ context.Context, peerID peer.ID, reqCtx *request.Context, _ bool) (models.Followers, error) {
	if s.coTenantPublicData != nil {
		if pd, err := s.coTenantPublicData(peerID); err == nil {
			if followers, err := pd.GetFollowers(); err == nil {
				return followers, nil
			}
		}
	}

	if s.netDB != nil {
		return s.netDB.GetFollowers(peerID.String(), reqCtx)
	}

	return nil, fmt.Errorf("follower data not available for remote peer %s", peerID)
}

func (s *FollowAppService) GetFollowing(_ context.Context, peerID peer.ID, reqCtx *request.Context, _ bool) (models.Following, error) {
	if s.coTenantPublicData != nil {
		if pd, err := s.coTenantPublicData(peerID); err == nil {
			if following, err := pd.GetFollowing(); err == nil {
				return following, nil
			}
		}
	}

	if s.netDB != nil {
		return s.netDB.GetFollowing(peerID.String(), reqCtx)
	}

	return nil, fmt.Errorf("following data not available for remote peer %s", peerID)
}

func (s *FollowAppService) HandleFollowMessage(from peer.ID, message *pb.Message) error {
	defer s.sendAckMessage(message.MessageID, from)

	if s.isDuplicate(message) {
		return nil
	}

	var ErrAlreadyFollowing = errors.New("peer already following us")

	err := s.db.Update(func(tx database.Tx) error {
		followers, err := tx.GetFollowers()
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		for _, follower := range followers {
			if follower == from.String() {
				return ErrAlreadyFollowing
			}
		}
		followers = append(followers, from.String())

		err = tx.SetFollowers(followers)

		if s.updateAndSaveProfile != nil {
			s.updateAndSaveProfile(tx)
		}

		return err
	})

	if err != nil && err != ErrAlreadyFollowing {
		return err
	} else if err == ErrAlreadyFollowing {
		logger.LogDebugWithIDf(log, s.nodeID, "Received FOLLOW message from peer %s which already follows us", from)
		return nil
	}

	s.syncFollowersToNetDB()

	logger.LogInfoWithIDf(log, s.nodeID, "Received FOLLOW message from %s", from)
	s.eventBus.Emit(&events.Follow{
		PeerID: from.String(),
	})
	return nil
}

func (s *FollowAppService) HandleUnFollowMessage(from peer.ID, message *pb.Message) error {
	defer s.sendAckMessage(message.MessageID, from)

	if s.isDuplicate(message) {
		return nil
	}

	logger.LogInfoWithIDf(log, s.nodeID, "Received UNFOLLOW message from %s", from)

	var ErrNotFollowing = errors.New("peer not following us")

	err := s.db.Update(func(tx database.Tx) error {
		followers, err := tx.GetFollowers()
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		exists := false
		for i, pid := range followers {
			if pid == from.String() {
				exists = true
				followers = append(followers[:i], followers[i+1:]...)
				break
			}
		}
		if !exists {
			return ErrNotFollowing
		}

		err = tx.SetFollowers(followers)

		if s.updateAndSaveProfile != nil {
			s.updateAndSaveProfile(tx)
		}

		return err
	})
	if err != nil && err != ErrNotFollowing {
		return err
	} else if err == ErrNotFollowing {
		logger.LogDebugWithIDf(log, s.nodeID, "Received UNFOLLOW message from peer %s that was not following us", from)
		return nil
	}

	s.syncFollowersToNetDB()

	s.eventBus.Emit(&events.Unfollow{
		PeerID: from.String(),
	})
	return nil
}

func (s *FollowAppService) syncFollowingToNetDB() {
	go func() {
		if s.netDB != nil {
			if following, err := s.GetMyFollowing(); err == nil {
				s.netDB.SetOwnFollowing(following)
			}
			if s.getMyProfile != nil {
				if profile, err := s.getMyProfile(); err == nil {
					s.netDB.SetOwnProfile(profile)
				}
			}
		}
	}()
}

func (s *FollowAppService) syncFollowersToNetDB() {
	go func() {
		if s.netDB != nil {
			if followers, err := s.GetMyFollowers(); err == nil {
				s.netDB.SetOwnFollowers(followers)
			}
			if s.getMyProfile != nil {
				if profile, err := s.getMyProfile(); err == nil {
					s.netDB.SetOwnProfile(profile)
				}
			}
		}
	}()
}

func (s *FollowAppService) isDuplicate(message *pb.Message) bool {
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", message.MessageID).First(&models.IncomingMessage{}).Error
	})
	return err == nil
}

func (s *FollowAppService) sendAckMessage(messageID string, to peer.ID) {
	err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.IncomingMessage{ID: messageID})
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "Error saving incoming message ID to database: %s", err)
	}
	s.messenger.SendACK(messageID, to)
}
