package core

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// GetRooms returns all joined rooms with summary metadata.
func (s *mautrixChatService) GetRooms(ctx context.Context) ([]contracts.MatrixRoom, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.touchActivity()

	resp, err := s.client.JoinedRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get joined rooms: %w", err)
	}

	rooms := make([]contracts.MatrixRoom, 0, len(resp.JoinedRooms))
	for _, roomID := range resp.JoinedRooms {
		room, err := s.buildRoomSummary(ctx, roomID)
		if err != nil {
			continue
		}
		rooms = append(rooms, *room)
	}

	sort.Slice(rooms, func(i, j int) bool {
		ti := rooms[i].LastMessage
		tj := rooms[j].LastMessage
		if ti == nil && tj == nil {
			return false
		}
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}
		return ti.Timestamp.After(tj.Timestamp)
	})

	return rooms, nil
}

// GetRoom returns detailed info for a single room.
func (s *mautrixChatService) GetRoom(ctx context.Context, roomID string) (*contracts.MatrixRoom, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.touchActivity()
	return s.buildRoomSummary(ctx, id.RoomID(roomID))
}

// CreateDirectRoom creates or retrieves a 1:1 DM room with the given Matrix user.
func (s *mautrixChatService) CreateDirectRoom(ctx context.Context, userID string) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	s.touchActivity()

	targetUserID := id.UserID(userID)

	resp, err := s.client.CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Preset:   "trusted_private_chat",
		IsDirect: true,
		Invite:   []id.UserID{targetUserID},
		InitialState: []*event.Event{
			{
				Type: event.StateEncryption,
				Content: event.Content{
					Parsed: &event.EncryptionEventContent{
						Algorithm: id.AlgorithmMegolmV1,
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create direct room: %w", err)
	}

	s.sendPeerIDStateEvent(ctx, resp.RoomID, s.matrixUserID.String(), s.config.PeerID.String())

	return resp.RoomID.String(), nil
}

// CreateGroupRoom creates a new group chat room.
func (s *mautrixChatService) CreateGroupRoom(ctx context.Context, name string, memberIDs []string, metadata map[string]string) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	s.touchActivity()

	invites := make([]id.UserID, len(memberIDs))
	for i, mid := range memberIDs {
		invites[i] = id.UserID(mid)
	}

	resp, err := s.client.CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Name:   name,
		Preset: "private_chat",
		Invite: invites,
		InitialState: []*event.Event{
			{
				Type: event.StateEncryption,
				Content: event.Content{
					Parsed: &event.EncryptionEventContent{
						Algorithm: id.AlgorithmMegolmV1,
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create group room: %w", err)
	}

	s.sendPeerIDStateEvent(ctx, resp.RoomID, s.matrixUserID.String(), s.config.PeerID.String())

	if len(metadata) > 0 {
		s.sendRoomMetadataStateEvent(ctx, resp.RoomID, metadata)
	}

	return resp.RoomID.String(), nil
}

// JoinRoom joins an existing room by ID or alias.
func (s *mautrixChatService) JoinRoom(ctx context.Context, roomID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	_, err := s.client.JoinRoomByID(ctx, id.RoomID(roomID))
	return err
}

// LeaveRoom leaves a room.
func (s *mautrixChatService) LeaveRoom(ctx context.Context, roomID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	_, err := s.client.LeaveRoom(ctx, id.RoomID(roomID))
	return err
}

// InviteToRoom invites a Matrix user to a room.
func (s *mautrixChatService) InviteToRoom(ctx context.Context, roomID, userID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	_, err := s.client.InviteUser(ctx, id.RoomID(roomID), &mautrix.ReqInviteUser{
		UserID: id.UserID(userID),
	})
	return err
}

// KickUser removes a user from a room with an optional reason.
func (s *mautrixChatService) KickUser(ctx context.Context, roomID, userID, reason string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	_, err := s.client.SendStateEvent(ctx, id.RoomID(roomID), event.StateMember, userID, &event.MemberEventContent{
		Membership: event.MembershipLeave,
		Reason:     reason,
	})
	return err
}

// SetRoomName changes the display name of a room.
func (s *mautrixChatService) SetRoomName(ctx context.Context, roomID, name string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	_, err := s.client.SendStateEvent(ctx, id.RoomID(roomID), event.StateRoomName, "", &event.RoomNameEventContent{
		Name: name,
	})
	return err
}

// SetRoomTopic changes the topic of a room.
func (s *mautrixChatService) SetRoomTopic(ctx context.Context, roomID, topic string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	_, err := s.client.SendStateEvent(ctx, id.RoomID(roomID), event.StateTopic, "", &event.TopicEventContent{
		Topic: topic,
	})
	return err
}

// SetRoomAvatar uploads an image and sets it as the room avatar.
func (s *mautrixChatService) SetRoomAvatar(ctx context.Context, roomID string, reader io.Reader, contentType string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	limited := io.LimitReader(reader, maxMediaUploadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("failed to read room avatar data: %w", err)
	}
	if int64(len(data)) > maxMediaUploadSize {
		return errMediaTooLarge
	}

	resp, err := s.client.UploadBytes(ctx, data, contentType)
	if err != nil {
		return fmt.Errorf("failed to upload room avatar: %w", err)
	}
	_, err = s.client.SendStateEvent(ctx, id.RoomID(roomID), event.StateRoomAvatar, "", &event.RoomAvatarEventContent{
		URL: resp.ContentURI.CUString(),
	})
	return err
}

// buildRoomSummary fetches room state and builds a MatrixRoom summary.
func (s *mautrixChatService) buildRoomSummary(ctx context.Context, roomID id.RoomID) (*contracts.MatrixRoom, error) {
	room := &contracts.MatrixRoom{
		RoomID: roomID.String(),
	}

	members, err := s.client.JoinedMembers(ctx, roomID)
	if err == nil {
		memberList := make([]contracts.MatrixMember, 0, len(members.Joined))
		for uid, member := range members.Joined {
			memberList = append(memberList, contracts.MatrixMember{
				UserID:      uid.String(),
				DisplayName: member.DisplayName,
				AvatarURL:   member.AvatarURL,
				Membership:  "join",
			})
		}
		room.Members = memberList
		room.IsDirect = len(members.Joined) == 2
	}

	stateMap, err := s.client.State(ctx, roomID)
	if err == nil {
		if nameEvts, ok := stateMap[event.StateRoomName]; ok {
			if evt, ok := nameEvts[""]; ok {
				if content, ok := evt.Content.Parsed.(*event.RoomNameEventContent); ok {
					room.Name = content.Name
				}
			}
		}
		if topicEvts, ok := stateMap[event.StateTopic]; ok {
			if evt, ok := topicEvts[""]; ok {
				if content, ok := evt.Content.Parsed.(*event.TopicEventContent); ok {
					room.Topic = content.Topic
				}
			}
		}
		if avatarEvts, ok := stateMap[event.StateRoomAvatar]; ok {
			if evt, ok := avatarEvts[""]; ok {
				if content, ok := evt.Content.Parsed.(*event.RoomAvatarEventContent); ok {
					room.AvatarURL = string(content.URL)
				}
			}
		}
		if _, ok := stateMap[event.StateEncryption]; ok {
			room.Encrypted = true
		}

		peerIDMap := make(map[string]string)
		if peerEvts, ok := stateMap[peerIDEventType]; ok {
			for stateKey, evt := range peerEvts {
				if pid, ok := evt.Content.Raw["peer_id"].(string); ok {
					peerIDMap[stateKey] = pid
				}
			}
		}
		for i := range room.Members {
			if pid, ok := peerIDMap[room.Members[i].UserID]; ok {
				room.Members[i].PeerID = pid
			}
		}

		if metaEvts, ok := stateMap[roomMetadataEventType]; ok {
			if evt, ok := metaEvts[""]; ok && evt.Content.Raw != nil {
				meta := make(map[string]string)
				for k, v := range evt.Content.Raw {
					if s, ok := v.(string); ok {
						meta[k] = s
					}
				}
				if len(meta) > 0 {
					room.Metadata = meta
				}
			}
		}

		room.RoomType = classifyRoomFromState(stateMap, len(room.Members))
	}

	if room.Name == "" && room.IsDirect {
		for _, m := range room.Members {
			if m.UserID != s.matrixUserID.String() {
				room.Name = m.DisplayName
				if room.Name == "" {
					room.Name = m.UserID
				}
				break
			}
		}
	}

	if lastMsg := s.fetchLastMessage(ctx, roomID); lastMsg != nil {
		room.LastMessage = lastMsg
	}

	return room, nil
}

// fetchLastMessage retrieves the most recent message event from a room
// for display in the room list. Silently returns nil on any error.
func (s *mautrixChatService) fetchLastMessage(ctx context.Context, roomID id.RoomID) *contracts.MatrixMessage {
	resp, err := s.client.Messages(ctx, roomID, "", "", mautrix.DirectionBackward, nil, 5)
	if err != nil {
		return nil
	}
	for _, evt := range resp.Chunk {
		if evt.Type == event.EventEncrypted && s.client.Crypto != nil {
			if err := evt.Content.ParseRaw(evt.Type); err != nil {
				continue
			}
			decrypted, err := s.client.Crypto.Decrypt(ctx, evt)
			if err != nil {
				continue
			}
			evt = decrypted
		}
		if evt.Type != event.EventMessage {
			continue
		}
		msg := s.eventToMessage(evt)
		return &msg
	}
	return nil
}

var (
	peerIDEventType       = event.NewEventType("mobazha.peer.id")
	roomMetadataEventType = event.NewEventType("mobazha.room.metadata")
)

// sendPeerIDStateEvent stores the original (case-sensitive) PeerID as a room
// state event so it can be recovered from room members later.
func (s *mautrixChatService) sendPeerIDStateEvent(ctx context.Context, roomID id.RoomID, matrixUID, peerID string) {
	_, err := s.client.SendStateEvent(ctx, roomID, peerIDEventType, matrixUID, map[string]string{
		"peer_id": peerID,
	})
	if err != nil {
		log.Warningf("Failed to send mobazha.peer.id state event for %s in %s: %v", matrixUID, roomID, err)
	}
}

// sendRoomMetadataStateEvent stores room metadata (type, orderId, storeId, etc.)
// as a single mobazha.room.metadata state event. classifyRoomFromState reads
// the "type" field from this same event for room classification.
func (s *mautrixChatService) sendRoomMetadataStateEvent(ctx context.Context, roomID id.RoomID, metadata map[string]string) {
	_, err := s.client.SendStateEvent(ctx, roomID, roomMetadataEventType, "", metadata)
	if err != nil {
		log.Warningf("Failed to send mobazha.room.metadata state event in %s: %v", roomID, err)
	}
}

// classifyRoomFromState determines the room type from an already-fetched state map.
func classifyRoomFromState(stateMap mautrix.RoomStateMap, memberCount int) string {
	if stateKeyMap, ok := stateMap[roomMetadataEventType]; ok {
		if evt, ok := stateKeyMap[""]; ok {
			if rt, ok := evt.Content.Raw["type"].(string); ok {
				switch rt {
				case "store", "order", "moderator", "community":
					return rt
				}
			}
		}
	}

	if memberCount == 2 {
		return "direct"
	}
	return "group"
}
