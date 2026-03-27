package api

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

// bridgeMockChatService implements a minimal contracts.MatrixChatService
// that exposes a controllable event channel for bridge testing.
type bridgeMockChatService struct {
	ch  chan contracts.MatrixChatEvent
	err error
}

func (m *bridgeMockChatService) Subscribe(_ context.Context) (<-chan contracts.MatrixChatEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ch, nil
}

func (m *bridgeMockChatService) Start(context.Context) error              { return nil }
func (m *bridgeMockChatService) Stop() error                              { return nil }
func (m *bridgeMockChatService) IsReady() bool                            { return true }
func (m *bridgeMockChatService) GetRooms(context.Context) ([]contracts.MatrixRoom, error) {
	return nil, nil
}
func (m *bridgeMockChatService) GetRoom(context.Context, string) (*contracts.MatrixRoom, error) {
	return nil, nil
}
func (m *bridgeMockChatService) GetInvitedRooms(context.Context) ([]contracts.MatrixRoom, error) {
	return nil, nil
}
func (m *bridgeMockChatService) CreateDirectRoom(context.Context, string) (string, error) {
	return "", nil
}
func (m *bridgeMockChatService) CreateGroupRoom(context.Context, string, []string, map[string]string) (string, error) {
	return "", nil
}
func (m *bridgeMockChatService) JoinRoom(context.Context, string) error  { return nil }
func (m *bridgeMockChatService) LeaveRoom(context.Context, string) error { return nil }
func (m *bridgeMockChatService) InviteToRoom(context.Context, string, string) error {
	return nil
}
func (m *bridgeMockChatService) KickUser(context.Context, string, string, string) error { return nil }
func (m *bridgeMockChatService) SetRoomName(context.Context, string, string) error {
	return nil
}
func (m *bridgeMockChatService) SetRoomTopic(context.Context, string, string) error {
	return nil
}
func (m *bridgeMockChatService) SetRoomAvatar(context.Context, string, io.Reader, string) error {
	return nil
}
func (m *bridgeMockChatService) SendMessage(context.Context, string, string) (string, error) {
	return "", nil
}
func (m *bridgeMockChatService) SendMedia(_ context.Context, _ string, _ io.Reader, _ string, _ int64, _ string) (string, error) {
	return "", nil
}
func (m *bridgeMockChatService) GetMessages(context.Context, string, int, string, string) ([]contracts.MatrixMessage, string, error) {
	return nil, "", nil
}
func (m *bridgeMockChatService) EditMessage(context.Context, string, string, string) error {
	return nil
}
func (m *bridgeMockChatService) RedactMessage(context.Context, string, string) error { return nil }
func (m *bridgeMockChatService) SendReaction(context.Context, string, string, string) (string, error) {
	return "", nil
}
func (m *bridgeMockChatService) BlockUser(context.Context, string) error       { return nil }
func (m *bridgeMockChatService) UnblockUser(context.Context, string) error     { return nil }
func (m *bridgeMockChatService) GetBlockedUsers(context.Context) ([]string, error) {
	return nil, nil
}
func (m *bridgeMockChatService) SendTyping(context.Context, string, bool) error      { return nil }
func (m *bridgeMockChatService) MarkAsRead(context.Context, string, string) error    { return nil }
func (m *bridgeMockChatService) SetDisplayName(context.Context, string) error        { return nil }
func (m *bridgeMockChatService) SetAvatar(_ context.Context, _ io.Reader, _ string) error {
	return nil
}
func (m *bridgeMockChatService) DownloadMedia(_ context.Context, _, _ string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, nil
}
func (m *bridgeMockChatService) GetChatSettings(context.Context) (*contracts.ChatSettings, error) {
	return &contracts.ChatSettings{InvitePolicy: contracts.InvitePolicyAutoMobazha}, nil
}
func (m *bridgeMockChatService) SetChatSettings(context.Context, *contracts.ChatSettings) error {
	return nil
}
func (m *bridgeMockChatService) StartVerification(context.Context, string) (string, error) {
	return "", nil
}
func (m *bridgeMockChatService) AcceptVerification(context.Context, string) error  { return nil }
func (m *bridgeMockChatService) StartSAS(context.Context, string) error            { return nil }
func (m *bridgeMockChatService) ConfirmSAS(context.Context, string) error          { return nil }
func (m *bridgeMockChatService) CancelVerification(context.Context, string) error  { return nil }
func (m *bridgeMockChatService) GetStatus(context.Context) contracts.MatrixChatStatus {
	return contracts.MatrixChatStatus{Connected: true}
}

// newTestGatewayWithHub creates a minimal Gateway with a single hub for testing.
func newTestGatewayWithHub(nodeID string) (*Gateway, *hub) {
	g := &Gateway{
		hubs: make(map[string]*hub),
	}
	h := newHub(nodeID)
	g.hubs[nodeID] = h
	go h.run()
	return g, h
}

func TestStartMatrixChatEventBridge_ForwardsEvents(t *testing.T) {
	const nodeID = "test-node-1"
	g, h := newTestGatewayWithHub(nodeID)
	defer close(h.stop)

	mockSvc := &bridgeMockChatService{
		ch: make(chan contracts.MatrixChatEvent, 10),
	}

	// Register a test "connection" to receive broadcasts.
	recv := make(chan []byte, 10)
	testConn := &connection{send: recv, h: h}
	h.register <- testConn
	time.Sleep(20 * time.Millisecond) // let hub process the register

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.StartMatrixChatEventBridge(ctx, nodeID, mockSvc)
	}()

	mockSvc.ch <- contracts.MatrixChatEvent{
		Type: "chat.message",
		Data: map[string]string{"roomId": "!r1:test", "content": "hi"},
	}

	select {
	case raw := <-recv:
		var msg wsMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("failed to unmarshal broadcast: %v", err)
		}
		if msg.Type != "chat.message" {
			t.Errorf("expected type chat.message, got %s", msg.Type)
		}
		var data map[string]string
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal data: %v", err)
		}
		if data["roomId"] != "!r1:test" {
			t.Errorf("roomId: got %s", data["roomId"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WS broadcast")
	}

	cancel()
	wg.Wait()
}

func TestStartMatrixChatEventBridge_TypingEvent(t *testing.T) {
	const nodeID = "test-node-2"
	g, h := newTestGatewayWithHub(nodeID)
	defer close(h.stop)

	mockSvc := &bridgeMockChatService{
		ch: make(chan contracts.MatrixChatEvent, 10),
	}

	recv := make(chan []byte, 10)
	testConn := &connection{send: recv, h: h}
	h.register <- testConn
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.StartMatrixChatEventBridge(ctx, nodeID, mockSvc)
	}()

	mockSvc.ch <- contracts.MatrixChatEvent{
		Type: "chat.typing",
		Data: map[string]interface{}{
			"roomId":  "!r2:test",
			"userIds": []string{"@alice:test"},
		},
	}

	select {
	case raw := <-recv:
		var msg wsMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Type != "chat.typing" {
			t.Errorf("type: got %s", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	cancel()
	wg.Wait()
}

func TestStartMatrixChatEventBridge_NilService(t *testing.T) {
	g := &Gateway{hubs: make(map[string]*hub)}
	// Should return immediately without panic.
	g.StartMatrixChatEventBridge(context.Background(), "node-x", nil)
}

func TestStartMatrixChatEventBridge_ContextCancel(t *testing.T) {
	const nodeID = "test-node-3"
	g, h := newTestGatewayWithHub(nodeID)
	defer close(h.stop)

	mockSvc := &bridgeMockChatService{
		ch: make(chan contracts.MatrixChatEvent, 10),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		g.StartMatrixChatEventBridge(ctx, nodeID, mockSvc)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not exit after context cancel")
	}
}

func TestStartMatrixChatEventBridge_ChannelClose(t *testing.T) {
	const nodeID = "test-node-4"
	g, h := newTestGatewayWithHub(nodeID)
	defer close(h.stop)

	evtCh := make(chan contracts.MatrixChatEvent, 10)
	mockSvc := &bridgeMockChatService{ch: evtCh}

	done := make(chan struct{})
	go func() {
		g.StartMatrixChatEventBridge(context.Background(), nodeID, mockSvc)
		close(done)
	}()

	close(evtCh)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not exit after channel close")
	}
}
