package core

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

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

// SendImage uploads and sends an image message. Returns the event ID.
func (s *mautrixChatService) SendImage(ctx context.Context, roomID string, reader io.Reader, filename string, size int64) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	s.touchActivity()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	contentType := "image/jpeg"
	if len(data) > 8 {
		if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
			contentType = "image/png"
		} else if data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
			contentType = "image/gif"
		} else if data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' {
			contentType = "image/webp"
		}
	}

	uploadResp, err := s.client.UploadBytes(ctx, data, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	resp, err := s.client.SendMessageEvent(ctx, id.RoomID(roomID), event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    filename,
		URL:     uploadResp.ContentURI.CUString(),
		Info: &event.FileInfo{
			MimeType: contentType,
			Size:     int(size),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send image message: %w", err)
	}
	return resp.EventID.String(), nil
}

// SendFile uploads and sends a file message. Returns the event ID.
func (s *mautrixChatService) SendFile(ctx context.Context, roomID string, reader io.Reader, filename string, size int64) (string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	s.touchActivity()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read file data: %w", err)
	}

	uploadResp, err := s.client.UploadBytes(ctx, data, "application/octet-stream")
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	resp, err := s.client.SendMessageEvent(ctx, id.RoomID(roomID), event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgFile,
		Body:    filename,
		URL:     uploadResp.ContentURI.CUString(),
		Info: &event.FileInfo{
			MimeType: "application/octet-stream",
			Size:     int(size),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send file message: %w", err)
	}
	return resp.EventID.String(), nil
}

// GetMessages returns paginated messages for a room.
// Returns messages, a pagination token for the next page, and an error.
func (s *mautrixChatService) GetMessages(ctx context.Context, roomID string, limit int, before string) ([]contracts.MatrixMessage, string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, "", err
	}
	s.touchActivity()
	if limit <= 0 {
		limit = 50
	}

	resp, err := s.client.Messages(ctx, id.RoomID(roomID), before, "", mautrix.DirectionBackward, nil, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get messages: %w", err)
	}

	messages := make([]contracts.MatrixMessage, 0, len(resp.Chunk))
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

	return messages, resp.End, nil
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
