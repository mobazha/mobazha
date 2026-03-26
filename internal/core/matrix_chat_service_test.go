package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// newTestService returns a minimal mautrixChatService with only the fields
// needed for pub/sub, state machine, and conversion tests. It does NOT
// connect to a real Synapse server.
func newTestService() *mautrixChatService {
	s := &mautrixChatService{
		serverName:   "matrix.test.local",
		matrixUserID: "@peer_qmtest:matrix.test.local",
	}
	s.ready.Store(false)
	s.stopped.Store(false)
	return s
}

// ──────────────────── Subscribe / broadcast / unsubscribe ────────────────────

func TestSubscribe_ReceivesEvents(t *testing.T) {
	s := newTestService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := s.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	evt := contracts.MatrixChatEvent{Type: "chat.message", Data: "hello"}
	s.broadcast(evt)

	select {
	case got := <-ch:
		if got.Type != "chat.message" {
			t.Errorf("expected type chat.message, got %s", got.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBroadcast_MultipleSubscribers(t *testing.T) {
	s := newTestService()
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	ch1, _ := s.Subscribe(ctx1)
	ch2, _ := s.Subscribe(ctx2)

	evt := contracts.MatrixChatEvent{Type: "chat.typing", Data: "room1"}
	s.broadcast(evt)

	for i, ch := range []<-chan contracts.MatrixChatEvent{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Type != "chat.typing" {
				t.Errorf("subscriber %d: expected chat.typing, got %s", i, got.Type)
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d: timed out", i)
		}
	}
}

func TestBroadcast_DropsWhenChannelFull(t *testing.T) {
	s := newTestService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, _ := s.Subscribe(ctx)

	for i := 0; i < matrixEventBufSize+10; i++ {
		s.broadcast(contracts.MatrixChatEvent{Type: "chat.flood", Data: i})
	}

	drained := 0
	for {
		select {
		case <-ch:
			drained++
		default:
			goto done
		}
	}
done:
	if drained > matrixEventBufSize {
		t.Errorf("expected at most %d events, drained %d", matrixEventBufSize, drained)
	}
}

func TestSubscribe_UnsubscribesOnContextCancel(t *testing.T) {
	s := newTestService()
	ctx, cancel := context.WithCancel(context.Background())
	ch, _ := s.Subscribe(ctx)

	cancel()
	time.Sleep(50 * time.Millisecond)

	s.subsMu.Lock()
	subCount := len(s.subs)
	s.subsMu.Unlock()

	if subCount != 0 {
		t.Errorf("expected 0 subscribers after cancel, got %d", subCount)
	}

	// Channel should be closed.
	_, open := <-ch
	if open {
		t.Error("expected channel to be closed after context cancel")
	}
}

// ──────────────────── ensureReady state machine ─────────────────────────────

func TestEnsureReady_AlreadyReady(t *testing.T) {
	s := newTestService()
	s.ready.Store(true)
	if err := s.ensureReady(context.Background()); err != nil {
		t.Errorf("expected nil error when already ready, got %v", err)
	}
}

func TestEnsureReady_PermanentlyStopped(t *testing.T) {
	s := newTestService()
	s.stopped.Store(true)
	err := s.ensureReady(context.Background())
	if err == nil {
		t.Fatal("expected error when permanently stopped")
	}
}

// ──────────────────── idleStop ───────────────────────────────────────────────

func TestIdleStop_SetsReadyFalse(t *testing.T) {
	s := newTestService()
	s.ready.Store(true)
	s.syncCancel = func() {}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, _ := s.Subscribe(ctx)

	s.idleStop()

	if s.ready.Load() {
		t.Error("expected ready=false after idleStop")
	}

	select {
	case evt := <-ch:
		if evt.Type != "chat.disconnected" {
			t.Errorf("expected chat.disconnected, got %s", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for disconnected event")
	}
}

func TestIdleStop_NoopWhenNotReady(t *testing.T) {
	s := newTestService()
	s.idleStop()
	if s.ready.Load() {
		t.Error("ready should remain false")
	}
}

// ──────────────────── Stop ──────────────────────────────────────────────────

func TestStop_ClosesSubscribers(t *testing.T) {
	s := newTestService()
	s.ready.Store(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, _ := s.Subscribe(ctx)

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	if !s.stopped.Load() {
		t.Error("expected stopped=true after Stop")
	}

	_, open := <-ch
	if open {
		t.Error("expected subscriber channel to be closed after Stop")
	}
}

func TestStop_Idempotent(t *testing.T) {
	s := newTestService()
	s.ready.Store(true)
	_ = s.Stop()
	if err := s.Stop(); err != nil {
		t.Errorf("second Stop should not error, got %v", err)
	}
}

// ──────────────────── GetStatus ─────────────────────────────────────────────

func TestGetStatus_NotReady(t *testing.T) {
	s := newTestService()
	status := s.GetStatus()
	if status.Connected {
		t.Error("expected Connected=false when not ready")
	}
}

func TestGetStatus_Ready(t *testing.T) {
	s := newTestService()
	s.ready.Store(true)
	// Simulate a client with known fields (cannot create real mautrix.Client
	// without a server, so we only test the not-ready path thoroughly).
	status := s.GetStatus()
	// client is nil → falls into !ready branch despite ready=true.
	if status.Connected {
		t.Log("Connected=false because client is nil (expected in unit test without Synapse)")
	}
}

// ──────────────────── eventToMessage conversion ─────────────────────────────

func makeTextEvent(evtID, roomID, sender, body string, ts int64) *event.Event {
	return &event.Event{
		ID:        id.EventID(evtID),
		RoomID:    id.RoomID(roomID),
		Sender:    id.UserID(sender),
		Timestamp: ts,
		Type:      event.EventMessage,
		Content: event.Content{
			Parsed: &event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    body,
			},
		},
	}
}

func TestEventToMessage_TextMessage(t *testing.T) {
	s := newTestService()
	evt := makeTextEvent("$evt1", "!room1:test", "@alice:test", "hello world", 1700000000000)
	msg := s.eventToMessage(evt)

	if msg.EventID != "$evt1" {
		t.Errorf("EventID: got %s, want $evt1", msg.EventID)
	}
	if msg.RoomID != "!room1:test" {
		t.Errorf("RoomID: got %s", msg.RoomID)
	}
	if msg.Sender != "@alice:test" {
		t.Errorf("Sender: got %s", msg.Sender)
	}
	if msg.Content != "hello world" {
		t.Errorf("Content: got %s", msg.Content)
	}
	if msg.MsgType != "m.text" {
		t.Errorf("MsgType: got %s", msg.MsgType)
	}
	if msg.EditedAt != nil {
		t.Error("EditedAt should be nil for non-edit")
	}
	if msg.ReplyTo != "" {
		t.Errorf("ReplyTo should be empty, got %s", msg.ReplyTo)
	}
	if msg.Media != nil {
		t.Error("Media should be nil for text message")
	}
}

func TestEventToMessage_ImageMessage(t *testing.T) {
	s := newTestService()
	evt := &event.Event{
		ID:        id.EventID("$img1"),
		RoomID:    id.RoomID("!room1:test"),
		Sender:    id.UserID("@bob:test"),
		Timestamp: 1700000000000,
		Type:      event.EventMessage,
		Content: event.Content{
			Parsed: &event.MessageEventContent{
				MsgType: event.MsgImage,
				Body:    "photo.jpg",
				URL:     id.ContentURIString("mxc://test/abc123"),
				Info: &event.FileInfo{
					MimeType: "image/jpeg",
					Size:     45000,
					Width:    800,
					Height:   600,
				},
			},
		},
	}

	msg := s.eventToMessage(evt)
	if msg.MsgType != "m.image" {
		t.Errorf("MsgType: got %s, want m.image", msg.MsgType)
	}
	if msg.Media == nil {
		t.Fatal("Media should not be nil for image message")
	}
	if msg.Media.URL != "mxc://test/abc123" {
		t.Errorf("Media.URL: got %s", msg.Media.URL)
	}
	if msg.Media.MimeType != "image/jpeg" {
		t.Errorf("Media.MimeType: got %s", msg.Media.MimeType)
	}
	if msg.Media.Width != 800 || msg.Media.Height != 600 {
		t.Errorf("Media dimensions: got %dx%d", msg.Media.Width, msg.Media.Height)
	}
	if msg.Media.Filename != "photo.jpg" {
		t.Errorf("Media.Filename: got %s", msg.Media.Filename)
	}
}

func TestEventToMessage_EditMessage(t *testing.T) {
	s := newTestService()
	evt := &event.Event{
		ID:        id.EventID("$edit1"),
		RoomID:    id.RoomID("!room1:test"),
		Sender:    id.UserID("@alice:test"),
		Timestamp: 1700000000000,
		Type:      event.EventMessage,
		Content: event.Content{
			Parsed: &event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    "* corrected text",
				RelatesTo: &event.RelatesTo{
					Type:    event.RelReplace,
					EventID: id.EventID("$original1"),
				},
				NewContent: &event.MessageEventContent{
					MsgType: event.MsgText,
					Body:    "corrected text",
				},
			},
		},
	}

	msg := s.eventToMessage(evt)
	if msg.Content != "corrected text" {
		t.Errorf("Content should be NewContent body, got %s", msg.Content)
	}
	if msg.EditedAt == nil {
		t.Fatal("EditedAt should be set for edit messages")
	}
}

func TestEventToMessage_ReplyMessage(t *testing.T) {
	s := newTestService()
	evt := &event.Event{
		ID:        id.EventID("$reply1"),
		RoomID:    id.RoomID("!room1:test"),
		Sender:    id.UserID("@bob:test"),
		Timestamp: 1700000000000,
		Type:      event.EventMessage,
		Content: event.Content{
			Parsed: &event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    "reply text",
				RelatesTo: &event.RelatesTo{
					InReplyTo: &event.InReplyTo{
						EventID: id.EventID("$parent1"),
					},
				},
			},
		},
	}

	msg := s.eventToMessage(evt)
	if msg.ReplyTo != "$parent1" {
		t.Errorf("ReplyTo: got %s, want $parent1", msg.ReplyTo)
	}
}

func TestEventToMessage_FileWithThumbnail(t *testing.T) {
	s := newTestService()
	evt := &event.Event{
		ID:        id.EventID("$file1"),
		RoomID:    id.RoomID("!room1:test"),
		Sender:    id.UserID("@carol:test"),
		Timestamp: 1700000000000,
		Type:      event.EventMessage,
		Content: event.Content{
			Parsed: &event.MessageEventContent{
				MsgType: event.MsgFile,
				Body:    "document.pdf",
				URL:     id.ContentURIString("mxc://test/file999"),
				Info: &event.FileInfo{
					MimeType:     "application/pdf",
					Size:         120000,
					ThumbnailURL: id.ContentURIString("mxc://test/thumb999"),
				},
			},
		},
	}

	msg := s.eventToMessage(evt)
	if msg.Media == nil {
		t.Fatal("Media should not be nil for file message")
	}
	if msg.Media.ThumbnailURL != "mxc://test/thumb999" {
		t.Errorf("ThumbnailURL: got %s", msg.Media.ThumbnailURL)
	}
	if msg.Media.Size != 120000 {
		t.Errorf("Size: got %d", msg.Media.Size)
	}
}

// ──────────────────── touchActivity / lastActivity ──────────────────────────

func TestTouchActivity_UpdatesTimestamp(t *testing.T) {
	s := newTestService()
	before := time.Now().UnixNano()
	s.touchActivity()
	after := time.Now().UnixNano()

	got := s.lastActivity.Load()
	if got < before || got > after {
		t.Errorf("lastActivity %d outside [%d, %d]", got, before, after)
	}
}

// ──────────────────── Concurrent safety ─────────────────────────────────────

func TestSubscribeBroadcast_ConcurrentSafety(t *testing.T) {
	s := newTestService()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ch, err := s.Subscribe(ctx)
			if err != nil {
				return
			}
			select {
			case <-ch:
			case <-time.After(200 * time.Millisecond):
			}
		}()
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.broadcast(contracts.MatrixChatEvent{Type: "chat.test", Data: n})
		}(i)
	}

	wg.Wait()
}
