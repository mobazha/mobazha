package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const maxMediaUploadSize = 50 << 20 // 50 MiB

var (
	errMediaTooLarge = errors.New("media exceeds maximum upload size of 50 MiB")

	roomKeyRequestTracker     = make(map[string]time.Time)
	roomKeyRequestTrackerLock sync.Mutex
	roomKeyRequestCooldown    = 5 * time.Minute
)

type undecryptableSession struct {
	senderKey id.SenderKey
	sender    id.UserID
	sessionID id.SessionID
}

// SendMessage sends a text message to a room. Returns the event ID.
func (s *mautrixChatService) SendMessage(ctx context.Context, roomID, content string) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	s.touchActivity()

	rid := id.RoomID(roomID)
	log.Infof("SendMessage: room=%s user=%s device=%s crypto=%v", roomID, s.client.UserID, s.client.DeviceID, s.client.Crypto != nil)

	resp, err := s.client.SendMessageEvent(ctx, rid, event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    content,
	})
	if err != nil {
		log.Warningf("SendMessage: failed for room %s: %v", roomID, err)
		return "", fmt.Errorf("failed to send message: %w", err)
	}
	log.Infof("SendMessage: sent eventID=%s to room %s", resp.EventID, roomID)
	return resp.EventID.String(), nil
}

// SendMedia uploads and sends a media message. The Matrix msgType is inferred
// from contentType: image/* → m.image, video/* → m.video, audio/* → m.audio,
// everything else → m.file. For images, magic-byte detection refines the MIME.
func (s *mautrixChatService) SendMedia(ctx context.Context, roomID string, reader io.Reader, filename string, size int64, contentType string) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	s.touchActivity()

	limited := io.LimitReader(reader, maxMediaUploadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("failed to read media data: %w", err)
	}
	if int64(len(data)) > maxMediaUploadSize {
		return "", errMediaTooLarge
	}

	msgType := mediaMsgType(contentType)
	if msgType == event.MsgImage {
		contentType = detectImageContentType(data)
	}

	uploadResp, err := s.client.UploadBytes(ctx, data, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload media: %w", err)
	}

	resp, err := s.client.SendMessageEvent(ctx, id.RoomID(roomID), event.EventMessage, &event.MessageEventContent{
		MsgType: msgType,
		Body:    filename,
		URL:     uploadResp.ContentURI.CUString(),
		Info: &event.FileInfo{
			MimeType: contentType,
			Size:     int(size),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send media message: %w", err)
	}
	return resp.EventID.String(), nil
}

func mediaMsgType(contentType string) event.MessageType {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return event.MsgImage
	case strings.HasPrefix(contentType, "video/"):
		return event.MsgVideo
	case strings.HasPrefix(contentType, "audio/"):
		return event.MsgAudio
	default:
		return event.MsgFile
	}
}

func detectImageContentType(data []byte) string {
	if len(data) > 8 {
		switch {
		case data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
			return "image/png"
		case data[0] == 'G' && data[1] == 'I' && data[2] == 'F':
			return "image/gif"
		case data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F':
			return "image/webp"
		}
	}
	return "image/jpeg"
}

// GetMessages returns paginated messages for a room.
// Returns messages, a pagination token for the next page, and an error.
func (s *mautrixChatService) GetMessages(ctx context.Context, roomID string, limit int, token string, dir string) ([]contracts.MatrixMessage, string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, "", err
	}
	s.touchActivity()
	if limit <= 0 {
		limit = 50
	}

	direction := mautrix.DirectionBackward
	if dir == "f" {
		direction = mautrix.DirectionForward
	}

	resp, err := s.client.Messages(ctx, id.RoomID(roomID), token, "", direction, nil, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get messages: %w", err)
	}

	messages := make([]contracts.MatrixMessage, 0, len(resp.Chunk))
	var missingSessions []undecryptableSession
	log.Infof("GetMessages: room=%s user=%s device=%s events=%d crypto=%v", roomID, s.client.UserID, s.client.DeviceID, len(resp.Chunk), s.client.Crypto != nil)
	for _, evt := range resp.Chunk {
		if evt.Type == event.EventEncrypted && s.client.Crypto != nil {
			if err := evt.Content.ParseRaw(evt.Type); err != nil {
				log.Warningf("GetMessages: parse encrypted event %s failed: %v", evt.ID, err)
				messages = append(messages, contracts.MatrixMessage{
					EventID:   evt.ID.String(),
					RoomID:    evt.RoomID.String(),
					Sender:    evt.Sender.String(),
					Content:   "Unable to decrypt this message",
					MsgType:   "m.text",
					Timestamp: time.UnixMilli(evt.Timestamp),
					Metadata:  map[string]string{"decryptionFailed": "true"},
				})
				continue
			}
			enc := evt.Content.AsEncrypted()
			log.Infof("GetMessages: encrypted event %s sender=%s sessionID=%s senderKey=%s", evt.ID, evt.Sender, enc.SessionID, enc.SenderKey)
			decrypted, err := s.client.Crypto.Decrypt(ctx, evt)
			if err != nil {
				log.Warningf("GetMessages: decrypt event %s failed: %v (sessionID=%s)", evt.ID, err, enc.SessionID)
				missingSessions = append(missingSessions, undecryptableSession{
					senderKey: id.SenderKey(enc.SenderKey),
					sender:    evt.Sender,
					sessionID: enc.SessionID,
				})
				messages = append(messages, contracts.MatrixMessage{
					EventID:   evt.ID.String(),
					RoomID:    evt.RoomID.String(),
					Sender:    evt.Sender.String(),
					Content:   "Unable to decrypt this message",
					MsgType:   "m.text",
					Timestamp: time.UnixMilli(evt.Timestamp),
					Metadata:  map[string]string{"decryptionFailed": "true"},
				})
				continue
			}
			log.Infof("GetMessages: decrypted event %s → type=%s", evt.ID, decrypted.Type.Type)
			evt = decrypted
		}
		if evt.Type != event.EventMessage {
			continue
		}
		msg := s.eventToMessage(evt)
		messages = append(messages, msg)
	}

	if len(missingSessions) > 0 && s.cryptoHelper != nil {
		go s.requestMissingRoomKeys(ctx, id.RoomID(roomID), missingSessions)
	}

	return messages, resp.End, nil
}

// requestMissingRoomKeys sends m.room_key_request to-device messages for
// sessions that could not be decrypted. Requests are deduplicated per
// (roomID, sessionID) with a cooldown to avoid spamming the sender.
func (s *mautrixChatService) requestMissingRoomKeys(parentCtx context.Context, roomID id.RoomID, missing []undecryptableSession) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("requestMissingRoomKeys: PANIC recovered: %v", r)
		}
	}()
	log.Infof("requestMissingRoomKeys: goroutine started for room %s with %d sessions", roomID, len(missing))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mach := s.cryptoHelper.Machine()
	if mach == nil {
		log.Errorf("requestMissingRoomKeys: OlmMachine is nil, aborting")
		return
	}
	seen := make(map[id.SessionID]bool)

	for _, m := range missing {
		if seen[m.sessionID] {
			continue
		}
		seen[m.sessionID] = true

		trackKey := string(roomID) + "/" + string(m.sessionID)
		roomKeyRequestTrackerLock.Lock()
		if last, ok := roomKeyRequestTracker[trackKey]; ok && time.Since(last) < roomKeyRequestCooldown {
			roomKeyRequestTrackerLock.Unlock()
			continue
		}
		roomKeyRequestTracker[trackKey] = time.Now()
		roomKeyRequestTrackerLock.Unlock()

		devices, err := mach.CryptoStore.GetDevices(ctx, m.sender)
		if err != nil || len(devices) == 0 {
			if err != nil {
				log.Warningf("requestMissingRoomKeys: GetDevices for %s failed: %v", m.sender, err)
			}
			_, err = mach.FetchKeys(ctx, []id.UserID{m.sender}, true)
			if err != nil {
				log.Warningf("requestMissingRoomKeys: FetchKeys for %s failed: %v", m.sender, err)
				continue
			}
			devices, _ = mach.CryptoStore.GetDevices(ctx, m.sender)
		}

		targets := map[id.UserID][]id.DeviceID{m.sender: {}}
		for devID := range devices {
			targets[m.sender] = append(targets[m.sender], devID)
		}
		if len(targets[m.sender]) == 0 {
			targets[m.sender] = []id.DeviceID{"*"}
		}

		err = mach.SendRoomKeyRequest(ctx, roomID, m.senderKey, m.sessionID, "", targets)
		if err != nil {
			log.Warningf("requestMissingRoomKeys: SendRoomKeyRequest for session %s in room %s failed: %v", m.sessionID, roomID, err)
		} else {
			log.Infof("requestMissingRoomKeys: requested key for session %s in room %s from %s (%d devices)",
				m.sessionID, roomID, m.sender, len(targets[m.sender]))
		}
	}
}

// EditMessage edits a previously sent message.
func (s *mautrixChatService) EditMessage(ctx context.Context, roomID, eventID, newContent string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()

	_, err := s.client.SendMessageEvent(ctx, id.RoomID(roomID), event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    "* " + newContent,
		RelatesTo: &event.RelatesTo{
			Type:    event.RelReplace,
			EventID: id.EventID(eventID),
		},
		NewContent: &event.MessageEventContent{
			MsgType: event.MsgText,
			Body:    newContent,
		},
	})
	return err
}

// RedactMessage redacts (deletes) a message.
func (s *mautrixChatService) RedactMessage(ctx context.Context, roomID, eventID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	_, err := s.client.RedactEvent(ctx, id.RoomID(roomID), id.EventID(eventID))
	return err
}

// SendReaction sends an emoji reaction to a message. Returns the reaction event ID.
func (s *mautrixChatService) SendReaction(ctx context.Context, roomID, eventID, key string) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	s.touchActivity()

	resp, err := s.client.SendMessageEvent(ctx, id.RoomID(roomID), event.EventReaction, &event.ReactionEventContent{
		RelatesTo: event.RelatesTo{
			Type:    event.RelAnnotation,
			EventID: id.EventID(eventID),
			Key:     key,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send reaction: %w", err)
	}
	return resp.EventID.String(), nil
}

// SendTyping sends a typing notification to a room.
func (s *mautrixChatService) SendTyping(ctx context.Context, roomID string, typing bool) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	timeout := int64(0)
	if typing {
		timeout = 30000
	}
	_, err := s.client.UserTyping(ctx, id.RoomID(roomID), typing, time.Duration(timeout)*time.Millisecond)
	return err
}

// MarkAsRead marks the given event in a room as read.
func (s *mautrixChatService) MarkAsRead(ctx context.Context, roomID, eventID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()
	return s.client.MarkRead(ctx, id.RoomID(roomID), id.EventID(eventID))
}
