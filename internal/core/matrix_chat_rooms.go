package core

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/encryption"
	"github.com/mobazha/mobazha/pkg/models"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// GetRooms returns all joined rooms with summary metadata.
func (s *mautrixChatService) GetRooms(ctx context.Context) ([]contracts.MatrixRoom, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.awaitFirstSync(ctx)
	s.touchActivity()

	unreadCounts := s.fetchUnreadCounts(ctx)

	resp, err := s.client.JoinedRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get joined rooms: %w", err)
	}

	rooms := make([]contracts.MatrixRoom, 0, len(resp.JoinedRooms))
	for _, roomID := range resp.JoinedRooms {
		room, lastMessageReliable, err := s.buildRoomSummary(ctx, roomID)
		if err != nil {
			continue
		}
		if count, ok := unreadCounts[roomID]; ok {
			applyRoomUnreadCount(room, count, lastMessageReliable)
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

func applyRoomUnreadCount(room *contracts.MatrixRoom, count int, lastMessageReliable bool) {
	if room == nil {
		return
	}
	if room.LastMessage == nil && lastMessageReliable {
		room.UnreadCount = 0
		return
	}
	room.UnreadCount = count
}

// GetRoom returns detailed info for a single room.
func (s *mautrixChatService) GetRoom(ctx context.Context, roomID string) (*contracts.MatrixRoom, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.touchActivity()
	room, _, err := s.buildRoomSummary(ctx, id.RoomID(roomID))
	return room, err
}

// GetInvitedRooms returns rooms where the user has a pending invite.
// Uses a lightweight one-shot sync with timeout=0 to retrieve the current invite list.
func (s *mautrixChatService) GetInvitedRooms(ctx context.Context) ([]contracts.MatrixRoom, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.touchActivity()

	inviteFilter := `{"room":{"timeline":{"limit":0},"state":{"lazy_load_members":true}},"presence":{"limit":0}}`
	resp, err := s.client.FullSyncRequest(ctx, mautrix.ReqSync{
		Timeout:     0,
		FilterID:    inviteFilter,
		SetPresence: event.PresenceOffline,
	})
	if err != nil {
		return nil, fmt.Errorf("sync for invites: %w", err)
	}

	rooms := make([]contracts.MatrixRoom, 0, len(resp.Rooms.Invite))
	for roomID, inviteRoom := range resp.Rooms.Invite {
		if inviteRoom == nil {
			continue
		}
		room := contracts.MatrixRoom{
			RoomID: roomID.String(),
		}
		for _, evt := range inviteRoom.State.Events {
			if evt == nil {
				continue
			}
			_ = evt.Content.ParseRaw(evt.Type)
			switch evt.Type {
			case event.StateRoomName:
				if c, ok := evt.Content.Parsed.(*event.RoomNameEventContent); ok {
					room.Name = c.Name
				}
			case event.StateEncryption:
				room.Encrypted = true
			case event.StateMember:
				if c, ok := evt.Content.Parsed.(*event.MemberEventContent); ok {
					room.Members = append(room.Members, contracts.MatrixMember{
						UserID:      evt.GetStateKey(),
						DisplayName: c.Displayname,
						AvatarURL:   string(c.AvatarURL),
						Membership:  string(c.Membership),
					})
				}
			}
		}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

// CreateDirectRoom creates or retrieves a 1:1 DM room for the provided target.
func (s *mautrixChatService) CreateDirectRoom(ctx context.Context, target contracts.MatrixDirectRoomTarget) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	s.touchActivity()

	targetUserID, targetPeerID, err := s.resolveDirectRoomTarget(target)
	if err != nil {
		return "", err
	}

	s.directRoomCreateMu.Lock()
	defer s.directRoomCreateMu.Unlock()

	existingRoomID, findErr := s.findExistingDirectRoom(ctx, targetUserID, targetPeerID)
	if findErr != nil {
		log.Warningf("Failed to search existing direct room for %s: %v", targetUserID, findErr)
	} else if existingRoomID != "" {
		return existingRoomID, nil
	}

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
	if _, err := s.client.InviteUser(ctx, resp.RoomID, &mautrix.ReqInviteUser{UserID: targetUserID}); err != nil &&
		!isAlreadyInvitedOrJoinedError(err) {
		return "", fmt.Errorf("failed to invite direct room target: %w", err)
	}

	s.publishPeerIDState(ctx, resp.RoomID, s.selfPeerIDAssignments())
	metadata := map[string]string{"type": "direct"}
	if targetPeerID != "" {
		metadata[directRoomTargetPeerIDMetaKey] = targetPeerID
	}
	s.sendRoomMetadataStateEvent(ctx, resp.RoomID, metadata)

	return resp.RoomID.String(), nil
}

func (s *mautrixChatService) findExistingDirectRoom(
	ctx context.Context,
	targetUserID id.UserID,
	targetPeerID string,
) (string, error) {
	rooms, err := s.GetRooms(ctx)
	if err != nil {
		return "", err
	}

	selfUserID := s.matrixUserID.String()
	targetUserIDStr := targetUserID.String()
	for _, room := range rooms {
		if !room.IsDirect && room.RoomType != "direct" {
			continue
		}
		if len(room.Members) != 2 {
			continue
		}

		hasSelf := false
		hasTarget := false
		targetMemberPeerID := ""
		for _, member := range room.Members {
			if member.UserID == selfUserID {
				hasSelf = true
			}
			if member.UserID == targetUserIDStr {
				hasTarget = true
				targetMemberPeerID = strings.TrimSpace(member.PeerID)
			}
		}
		if !hasSelf || !hasTarget {
			continue
		}
		if targetPeerID != "" && targetMemberPeerID != "" && targetMemberPeerID != targetPeerID {
			continue
		}
		return room.RoomID, nil
	}
	return "", nil
}

func (s *mautrixChatService) resolveDirectRoomTarget(
	target contracts.MatrixDirectRoomTarget,
) (id.UserID, string, error) {
	targetUserID := strings.TrimSpace(target.TargetUserID)
	targetPeerID := strings.TrimSpace(target.TargetPeerID)

	if targetUserID == "" && targetPeerID == "" {
		return "", "", fmt.Errorf("targetUserID or targetPeerID is required")
	}
	if targetUserID != "" && targetPeerID != "" {
		return "", "", fmt.Errorf("only one direct target is allowed")
	}

	if targetPeerID != "" {
		targetUserID = encryption.MatrixUserIDFromPeerID(targetPeerID, s.serverName)
		if resolved := s.lookupCanonicalPeerIDsByMatrixUserID([]string{targetUserID}); len(resolved) > 0 {
			if canonicalPeerID, ok := resolved[targetUserID]; ok && canonicalPeerID != "" {
				targetPeerID = canonicalPeerID
			}
		}
	}
	if targetUserID == s.matrixUserID.String() {
		return "", "", fmt.Errorf("target user cannot be self")
	}

	return id.UserID(targetUserID), targetPeerID, nil
}

func isAlreadyInvitedOrJoinedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already invited") ||
		strings.Contains(msg, "already in the room") ||
		strings.Contains(msg, "already joined")
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

	s.publishPeerIDState(ctx, resp.RoomID, s.selfPeerIDAssignments())

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
	targetRoomID := id.RoomID(roomID)
	_, err := s.client.JoinRoomByID(ctx, targetRoomID)
	if err != nil {
		return err
	}
	s.publishPeerIDState(ctx, targetRoomID, s.selfPeerIDAssignments())
	return nil
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

// buildRoomSummary fetches room state and builds a MatrixRoom summary. The
// boolean reports whether an empty LastMessage means the room was read reliably
// and has no displayable messages, instead of a transient fetch/decrypt failure.
func (s *mautrixChatService) buildRoomSummary(ctx context.Context, roomID id.RoomID) (*contracts.MatrixRoom, bool, error) {
	room := &contracts.MatrixRoom{
		RoomID: roomID.String(),
	}

	memberMap := make(map[string]contracts.MatrixMember)

	members, err := s.client.JoinedMembers(ctx, roomID)
	if err == nil {
		for uid, member := range members.Joined {
			memberMap[uid.String()] = contracts.MatrixMember{
				UserID:      uid.String(),
				DisplayName: member.DisplayName,
				AvatarURL:   member.AvatarURL,
				Membership:  "join",
			}
		}
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
		if memberEvts, ok := stateMap[event.StateMember]; ok {
			for stateKey, evt := range memberEvts {
				if evt == nil {
					continue
				}
				_ = evt.Content.ParseRaw(evt.Type)
				content, ok := evt.Content.Parsed.(*event.MemberEventContent)
				if !ok {
					continue
				}
				if !isActiveMemberMembership(content.Membership) {
					delete(memberMap, stateKey)
					continue
				}
				memberMap[stateKey] = contracts.MatrixMember{
					UserID:      stateKey,
					DisplayName: content.Displayname,
					AvatarURL:   string(content.AvatarURL),
					Membership:  string(content.Membership),
				}
			}
		}
		room.Members = membersFromMap(memberMap)

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
		s.fillDirectRoomMemberPeerIDFromMetadata(room.Members, room.Metadata)
		s.fillMissingMemberPeerIDs(room.Members)

		room.RoomType = classifyRoomFromState(stateMap, len(room.Members))
		if room.RoomType == "direct" {
			room.IsDirect = true
		}
		if !room.IsDirect {
			room.IsDirect = len(room.Members) == 2
		}
	} else {
		room.Members = membersFromMap(memberMap)
		if room.IsDirect || len(room.Members) == 2 {
			room.IsDirect = true
			room.RoomType = "direct"
		}
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

	lastMsg, lastMessageReliable := s.fetchLastMessage(ctx, roomID)
	if lastMsg != nil {
		room.LastMessage = lastMsg
	}

	return room, lastMessageReliable, nil
}

func isActiveMemberMembership(membership event.Membership) bool {
	switch membership {
	case event.MembershipJoin, event.MembershipInvite, event.MembershipKnock:
		return true
	default:
		return false
	}
}

func membersFromMap(memberMap map[string]contracts.MatrixMember) []contracts.MatrixMember {
	if len(memberMap) == 0 {
		return nil
	}
	ids := make([]string, 0, len(memberMap))
	for userID := range memberMap {
		ids = append(ids, userID)
	}
	sort.Strings(ids)

	members := make([]contracts.MatrixMember, 0, len(ids))
	for _, userID := range ids {
		members = append(members, memberMap[userID])
	}
	return members
}

func (s *mautrixChatService) fillMissingMemberPeerIDs(members []contracts.MatrixMember) {
	missingUserIDs := make([]string, 0)
	for _, member := range members {
		if member.PeerID == "" && member.UserID != "" {
			missingUserIDs = append(missingUserIDs, member.UserID)
		}
	}
	if len(missingUserIDs) == 0 {
		return
	}

	resolvedPeerIDs := s.lookupCanonicalPeerIDsByMatrixUserID(missingUserIDs)
	if len(resolvedPeerIDs) == 0 {
		return
	}

	for i := range members {
		if members[i].PeerID != "" {
			continue
		}
		if peerID, ok := resolvedPeerIDs[members[i].UserID]; ok {
			members[i].PeerID = peerID
		}
	}
}

func (s *mautrixChatService) lookupCanonicalPeerIDsByMatrixUserID(userIDs []string) map[string]string {
	resolved := make(map[string]string)
	if s.config.DB == nil || len(userIDs) == 0 {
		return resolved
	}

	uniqueUserIDs := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		uniqueUserIDs = append(uniqueUserIDs, userID)
	}
	if len(uniqueUserIDs) == 0 {
		return resolved
	}

	var records []models.MatrixCredentials
	if err := s.config.DB.View(func(tx database.Tx) error {
		return tx.Read().Where("matrix_user_id IN ?", uniqueUserIDs).Find(&records).Error
	}); err != nil {
		log.Warningf("Failed to resolve Matrix user IDs to peer IDs: %v", err)
		return resolved
	}

	for _, record := range records {
		if record.MatrixUserID == "" || record.PeerID == "" {
			continue
		}
		resolved[record.MatrixUserID] = record.PeerID
	}

	return resolved
}

// fetchUnreadCounts returns the cached per-room notification counts collected
// from the background sync loop. The counts are updated on every /sync
// response by handleSyncResponse, so no extra HTTP call is needed.
func (s *mautrixChatService) fetchUnreadCounts(_ context.Context) map[id.RoomID]int {
	s.unreadCountsMu.RLock()
	defer s.unreadCountsMu.RUnlock()
	if s.unreadCounts == nil {
		return nil
	}
	out := make(map[id.RoomID]int, len(s.unreadCounts))
	for k, v := range s.unreadCounts {
		out[k] = v
	}
	return out
}

// fetchLastMessage retrieves the most recent message event from a room for
// display in the room list. The boolean reports whether the lookup was reliable
// enough to treat a nil message as "no displayable messages yet".
func (s *mautrixChatService) fetchLastMessage(ctx context.Context, roomID id.RoomID) (*contracts.MatrixMessage, bool) {
	resp, err := s.client.Messages(ctx, roomID, "", "", mautrix.DirectionBackward, nil, 50)
	if err != nil {
		return nil, false
	}
	reliable := true
	for _, evt := range resp.Chunk {
		if evt.Type == event.EventEncrypted && s.client.Crypto != nil {
			if err := evt.Content.ParseRaw(evt.Type); err != nil {
				reliable = false
				continue
			}
			decrypted, err := s.client.Crypto.Decrypt(ctx, evt)
			if err != nil {
				reliable = false
				continue
			}
			evt = decrypted
		} else if evt.Type == event.EventEncrypted {
			reliable = false
			continue
		}
		if evt.Type != event.EventMessage {
			continue
		}
		msg := s.eventToMessage(evt)
		return &msg, true
	}
	return nil, reliable
}

var (
	// Custom room metadata/identity events are state events and must use
	// StateEventType class for RoomStateMap lookups to work reliably.
	peerIDEventType       = event.Type{Type: "mobazha.peer.id", Class: event.StateEventType}
	roomMetadataEventType = event.Type{Type: "mobazha.room.metadata", Class: event.StateEventType}
)

const (
	directRoomTargetPeerIDMetaKey = "direct_target_peer_id"
)

func (s *mautrixChatService) selfPeerIDAssignments() map[string]string {
	if s.matrixUserID == "" || s.config.PeerID == "" {
		return nil
	}
	return map[string]string{s.matrixUserID.String(): s.config.PeerID.String()}
}

func (s *mautrixChatService) publishPeerIDState(ctx context.Context, roomID id.RoomID, assignments map[string]string) {
	for matrixUserID, peerID := range assignments {
		if matrixUserID == "" || peerID == "" {
			continue
		}
		s.sendPeerIDStateEvent(ctx, roomID, matrixUserID, peerID)
	}
}

func (s *mautrixChatService) fillDirectRoomMemberPeerIDFromMetadata(
	members []contracts.MatrixMember,
	metadata map[string]string,
) {
	if len(members) == 0 || len(metadata) == 0 {
		return
	}
	if roomType := metadata["type"]; roomType != "" && roomType != "direct" {
		return
	}

	targetPeerID := metadata[directRoomTargetPeerIDMetaKey]
	if targetPeerID == "" {
		return
	}
	if selfPeerID := s.config.PeerID.String(); selfPeerID != "" && targetPeerID == selfPeerID {
		// Metadata stores the creation target's peerID. On the invitee side that
		// value equals self and must not be assigned to the counterparty member.
		return
	}

	selfUserID := s.matrixUserID.String()
	for i := range members {
		if members[i].UserID == selfUserID {
			continue
		}
		if members[i].PeerID == "" {
			members[i].PeerID = targetPeerID
			return
		}
	}

	// Fallback when self userID is unavailable in summary (e.g. malformed room state):
	// assign the target peerID to the first member still missing a peerID.
	for i := range members {
		if members[i].PeerID == "" {
			members[i].PeerID = targetPeerID
			return
		}
	}
}

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
				case "direct", "store", "order", "moderator", "community":
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
