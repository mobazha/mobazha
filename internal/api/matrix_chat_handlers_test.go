package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/pkg/contracts"
)

type mockMatrixChatService struct {
	rooms       []contracts.MatrixRoom
	messages    []contracts.MatrixMessage
	nextToken   string
	createdRoom string
	lastCall    string
	lastArgs    map[string]interface{}
	err         error
}

func (m *mockMatrixChatService) Start(_ context.Context) error { return nil }
func (m *mockMatrixChatService) Stop() error                   { return nil }
func (m *mockMatrixChatService) IsReady() bool                 { return true }

func (m *mockMatrixChatService) GetRooms(_ context.Context) ([]contracts.MatrixRoom, error) {
	m.lastCall = "GetRooms"
	return m.rooms, m.err
}

func (m *mockMatrixChatService) GetRoom(_ context.Context, roomID string) (*contracts.MatrixRoom, error) {
	m.lastCall = "GetRoom"
	for i := range m.rooms {
		if m.rooms[i].RoomID == roomID {
			return &m.rooms[i], nil
		}
	}
	return &contracts.MatrixRoom{RoomID: roomID}, m.err
}

func (m *mockMatrixChatService) GetInvitedRooms(_ context.Context) ([]contracts.MatrixRoom, error) {
	m.lastCall = "GetInvitedRooms"
	return nil, m.err
}

func (m *mockMatrixChatService) CreateDirectRoom(_ context.Context, target contracts.MatrixDirectRoomTarget) (string, error) {
	m.lastCall = "CreateDirectRoom"
	m.lastArgs = map[string]interface{}{
		"targetUserID": target.TargetUserID,
		"targetPeerID": target.TargetPeerID,
	}
	return m.createdRoom, m.err
}

func (m *mockMatrixChatService) CreateGroupRoom(_ context.Context, name string, memberIDs []string, metadata map[string]string) (string, error) {
	m.lastCall = "CreateGroupRoom"
	m.lastArgs = map[string]interface{}{"name": name, "memberIDs": memberIDs, "metadata": metadata}
	return m.createdRoom, m.err
}

func (m *mockMatrixChatService) JoinRoom(_ context.Context, roomID string) error {
	m.lastCall = "JoinRoom"
	m.lastArgs = map[string]interface{}{"roomID": roomID}
	return m.err
}

func (m *mockMatrixChatService) LeaveRoom(_ context.Context, roomID string) error {
	m.lastCall = "LeaveRoom"
	m.lastArgs = map[string]interface{}{"roomID": roomID}
	return m.err
}

func (m *mockMatrixChatService) InviteToRoom(_ context.Context, roomID, userID string) error {
	m.lastCall = "InviteToRoom"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "userID": userID}
	return m.err
}

func (m *mockMatrixChatService) KickUser(_ context.Context, roomID, userID, reason string) error {
	m.lastCall = "KickUser"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "userID": userID, "reason": reason}
	return m.err
}

func (m *mockMatrixChatService) SetRoomName(_ context.Context, roomID, name string) error {
	m.lastCall = "SetRoomName"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "name": name}
	return m.err
}

func (m *mockMatrixChatService) SetRoomTopic(_ context.Context, roomID, topic string) error {
	m.lastCall = "SetRoomTopic"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "topic": topic}
	return m.err
}

func (m *mockMatrixChatService) SetRoomAvatar(_ context.Context, roomID string, reader io.Reader, contentType string) error {
	m.lastCall = "SetRoomAvatar"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "contentType": contentType}
	return m.err
}

func (m *mockMatrixChatService) SendMessage(_ context.Context, roomID, content string) (string, error) {
	m.lastCall = "SendMessage"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "content": content}
	return "$evt123", m.err
}

func (m *mockMatrixChatService) SendMedia(_ context.Context, roomID string, _ io.Reader, filename string, _ int64, contentType string) (string, error) {
	m.lastCall = "SendMedia"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "filename": filename, "contentType": contentType}
	return "$media456", m.err
}

func (m *mockMatrixChatService) GetMessages(_ context.Context, roomID string, limit int, token string, dir string) ([]contracts.MatrixMessage, string, error) {
	m.lastCall = "GetMessages"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "limit": limit, "token": token, "dir": dir}
	return m.messages, m.nextToken, m.err
}

func (m *mockMatrixChatService) EditMessage(_ context.Context, roomID, eventID, newContent string) error {
	m.lastCall = "EditMessage"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "eventID": eventID, "newContent": newContent}
	return m.err
}

func (m *mockMatrixChatService) RedactMessage(_ context.Context, roomID, eventID string) error {
	m.lastCall = "RedactMessage"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "eventID": eventID}
	return m.err
}

func (m *mockMatrixChatService) SendReaction(_ context.Context, roomID, eventID, key string) (string, error) {
	m.lastCall = "SendReaction"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "eventID": eventID, "key": key}
	return "$reaction001", m.err
}

func (m *mockMatrixChatService) BlockUser(_ context.Context, userID string) error {
	m.lastCall = "BlockUser"
	m.lastArgs = map[string]interface{}{"userID": userID}
	return m.err
}

func (m *mockMatrixChatService) UnblockUser(_ context.Context, userID string) error {
	m.lastCall = "UnblockUser"
	m.lastArgs = map[string]interface{}{"userID": userID}
	return m.err
}

func (m *mockMatrixChatService) GetBlockedUsers(_ context.Context) ([]string, error) {
	m.lastCall = "GetBlockedUsers"
	return []string{"@blocked:test"}, m.err
}

func (m *mockMatrixChatService) SendTyping(_ context.Context, roomID string, typing bool) error {
	m.lastCall = "SendTyping"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "typing": typing}
	return m.err
}

func (m *mockMatrixChatService) MarkAsRead(_ context.Context, roomID, eventID string) error {
	m.lastCall = "MarkAsRead"
	m.lastArgs = map[string]interface{}{"roomID": roomID, "eventID": eventID}
	return m.err
}

func (m *mockMatrixChatService) Subscribe(_ context.Context) (<-chan contracts.MatrixChatEvent, error) {
	ch := make(chan contracts.MatrixChatEvent)
	return ch, m.err
}

func (m *mockMatrixChatService) SetDisplayName(_ context.Context, _ string) error {
	m.lastCall = "SetDisplayName"
	return m.err
}

func (m *mockMatrixChatService) SetAvatar(_ context.Context, _ io.Reader, _ string) error {
	return m.err
}

func (m *mockMatrixChatService) DownloadMedia(_ context.Context, _, _ string) (io.ReadCloser, string, int64, error) {
	m.lastCall = "DownloadMedia"
	body := io.NopCloser(strings.NewReader("fake-media-bytes"))
	return body, "image/png", 16, m.err
}

func (m *mockMatrixChatService) GetChatSettings(_ context.Context) (*contracts.ChatSettings, error) {
	return &contracts.ChatSettings{InvitePolicy: contracts.InvitePolicyAutoMobazha}, m.err
}

func (m *mockMatrixChatService) SetChatSettings(_ context.Context, _ *contracts.ChatSettings) error {
	return m.err
}

func (m *mockMatrixChatService) StartVerification(_ context.Context, userID string) (string, error) {
	m.lastCall = "StartVerification"
	m.lastArgs = map[string]interface{}{"userID": userID}
	return "txn_123", m.err
}

func (m *mockMatrixChatService) AcceptVerification(_ context.Context, txnID string) error {
	m.lastCall = "AcceptVerification"
	m.lastArgs = map[string]interface{}{"txnID": txnID}
	return m.err
}

func (m *mockMatrixChatService) StartSAS(_ context.Context, txnID string) error {
	m.lastCall = "StartSAS"
	m.lastArgs = map[string]interface{}{"txnID": txnID}
	return m.err
}

func (m *mockMatrixChatService) ConfirmSAS(_ context.Context, txnID string) error {
	m.lastCall = "ConfirmSAS"
	m.lastArgs = map[string]interface{}{"txnID": txnID}
	return m.err
}

func (m *mockMatrixChatService) CancelVerification(_ context.Context, txnID string) error {
	m.lastCall = "CancelVerification"
	m.lastArgs = map[string]interface{}{"txnID": txnID}
	return m.err
}

func (m *mockMatrixChatService) GetStatus(context.Context) contracts.MatrixChatStatus {
	return contracts.MatrixChatStatus{
		Connected:             true,
		SyncRunning:           true,
		UserID:                "@peer_abc:matrix.mobazha.org",
		VerificationAvailable: true,
	}
}

// mockNodeWithMatrixChat wraps mockNode and overrides MatrixChat() to return a real mock.
type mockNodeWithMatrixChat struct {
	mockNode
	chatSvc *mockMatrixChatService
}

func (m *mockNodeWithMatrixChat) MatrixChat() contracts.MatrixChatService {
	return m.chatSvc
}

func newTestRouterWithChatMock(chatSvc *mockMatrixChatService) (chi.Router, *Gateway) {
	node := &mockNodeWithMatrixChat{chatSvc: chatSvc}
	g := &Gateway{}
	r := chi.NewMux()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	r.Get("/v1/chat/status", g.handleGETMatrixChatStatus)
	r.Get("/v1/chat/rooms", g.handleGETMatrixChatRooms)
	r.Post("/v1/chat/rooms", g.handlePOSTMatrixChatRoom)
	r.Post("/v1/chat/rooms/{roomID}/join", g.handlePOSTMatrixChatRoomJoin)
	r.Post("/v1/chat/rooms/{roomID}/leave", g.handlePOSTMatrixChatRoomLeave)
	r.Get("/v1/chat/rooms/{roomID}/messages", g.handleGETMatrixChatRoomMessages)
	r.Post("/v1/chat/rooms/{roomID}/messages", g.handlePOSTMatrixChatRoomMessage)
	r.Put("/v1/chat/rooms/{roomID}/messages/{eventID}", g.handlePUTMatrixChatRoomMessage)
	r.Delete("/v1/chat/rooms/{roomID}/messages/{eventID}", g.handleDELETEMatrixChatRoomMessage)
	r.Post("/v1/chat/rooms/{roomID}/messages/{eventID}/reactions", g.handlePOSTMatrixChatRoomReaction)
	r.Post("/v1/chat/rooms/{roomID}/typing", g.handlePOSTMatrixChatRoomTyping)
	r.Post("/v1/chat/rooms/{roomID}/read", g.handlePOSTMatrixChatRoomRead)
	r.Get("/v1/chat/rooms/{roomID}/members", g.handleGETMatrixChatRoomMembers)
	r.Post("/v1/chat/rooms/{roomID}/invite", g.handlePOSTMatrixChatRoomInvite)
	r.Post("/v1/chat/rooms/{roomID}/kick", g.handlePOSTMatrixChatRoomKick)
	r.Get("/v1/chat/rooms/{roomID}/settings", g.handleGETMatrixChatRoomSettings)
	r.Put("/v1/chat/rooms/{roomID}/settings", g.handlePUTMatrixChatRoomSettings)
	r.Post("/v1/chat/rooms/{roomID}/avatar", g.handlePOSTMatrixChatRoomAvatar)
	r.Get("/v1/chat/media/{serverName}/{mediaID}", g.handleGETMatrixChatMediaDownload)
	r.Post("/v1/chat/users/{userID}/block", g.handlePOSTMatrixChatUserBlock)
	r.Delete("/v1/chat/users/{userID}/block", g.handleDELETEMatrixChatUserBlock)
	r.Get("/v1/chat/blocked-users", g.handleGETMatrixChatBlockedUsers)
	r.Get("/v1/chat/settings", g.handleGETMatrixChatSettings)
	r.Put("/v1/chat/settings", g.handlePUTMatrixChatSettings)
	r.Post("/v1/chat/verification/request", g.handlePOSTMatrixChatVerificationRequest)
	r.Post("/v1/chat/verification/{txnId}/accept", g.handlePOSTMatrixChatVerificationAccept)
	r.Post("/v1/chat/verification/{txnId}/start-sas", g.handlePOSTMatrixChatVerificationStartSAS)
	r.Post("/v1/chat/verification/{txnId}/confirm", g.handlePOSTMatrixChatVerificationConfirm)
	r.Post("/v1/chat/verification/{txnId}/cancel", g.handlePOSTMatrixChatVerificationCancel)

	return r, g
}

func TestMatrixChat_GetStatus(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing data envelope")
	}
	if data["connected"] != true {
		t.Errorf("expected connected=true, got %v", data["connected"])
	}
	if data["syncRunning"] != true {
		t.Errorf("expected syncRunning=true, got %v", data["syncRunning"])
	}
	if data["verificationAvailable"] != true {
		t.Errorf("expected verificationAvailable=true, got %v", data["verificationAvailable"])
	}
}

func TestMatrixChat_GetRooms_ReturnsJSON(t *testing.T) {
	chatSvc := &mockMatrixChatService{
		rooms: []contracts.MatrixRoom{
			{RoomID: "!room1:test", Name: "Test Room", IsDirect: true, Encrypted: true},
			{RoomID: "!room2:test", Name: "Group", IsDirect: false, Members: []contracts.MatrixMember{
				{UserID: "@peer_abc:test", DisplayName: "Alice", PeerID: "QmAlice123"},
			}},
		},
	}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/rooms", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []contracts.MatrixRoom `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(resp.Data))
	}
	if resp.Data[0].RoomID != "!room1:test" {
		t.Errorf("room[0].roomId = %q, want !room1:test", resp.Data[0].RoomID)
	}
	if !resp.Data[0].IsDirect {
		t.Errorf("room[0].isDirect should be true")
	}
	if resp.Data[1].Members[0].PeerID != "QmAlice123" {
		t.Errorf("room[1].members[0].peerID = %q, want QmAlice123", resp.Data[1].Members[0].PeerID)
	}
}

func TestMatrixChat_CreateDirectRoom(t *testing.T) {
	chatSvc := &mockMatrixChatService{createdRoom: "!dm123:test"}
	router, _ := newTestRouterWithChatMock(chatSvc)
	const targetPeerID = "12D3KooWBkkycUCusJiLHXogEfiHghmMy3kDgtSovn58zy9uwikB"

	body := `{"targetPeerID":"` + targetPeerID + `","isDM":true}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["roomId"] != "!dm123:test" {
		t.Errorf("roomId = %v, want !dm123:test", resp.Data["roomId"])
	}
	if chatSvc.lastCall != "CreateDirectRoom" {
		t.Errorf("expected CreateDirectRoom, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["targetPeerID"] != targetPeerID {
		t.Errorf("expected targetPeerID %s, got %v", targetPeerID, chatSvc.lastArgs["targetPeerID"])
	}
}

func TestMatrixChat_CreateDirectRoom_AllowTargetUserIDForExternalUser(t *testing.T) {
	chatSvc := &mockMatrixChatService{createdRoom: "!dm123:test"}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"targetUserID":"@alice:matrix.org","isDM":true}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "CreateDirectRoom" {
		t.Fatalf("expected CreateDirectRoom, got %s", chatSvc.lastCall)
	}
	if got, _ := chatSvc.lastArgs["targetPeerID"].(string); got != "" {
		t.Fatalf("expected empty targetPeerID, got %q", got)
	}
	if got, _ := chatSvc.lastArgs["targetUserID"].(string); got != "@alice:matrix.org" {
		t.Fatalf("expected targetUserID @alice:matrix.org, got %q", got)
	}
}

func TestMatrixChat_CreateDirectRoom_InvalidTargetPeerID(t *testing.T) {
	chatSvc := &mockMatrixChatService{createdRoom: "!dm123:test"}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"targetPeerID":"invalid-peer-id","isDM":true}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "" {
		t.Fatalf("expected service not to be called, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_CreateDirectRoom_RequireSingleTarget(t *testing.T) {
	chatSvc := &mockMatrixChatService{createdRoom: "!dm123:test"}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"isDM":true}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "" {
		t.Fatalf("expected service not to be called, got %s", chatSvc.lastCall)
	}

	body = `{"targetUserID":"@alice:matrix.org","targetPeerID":"12D3KooWBkkycUCusJiLHXogEfiHghmMy3kDgtSovn58zy9uwikB","isDM":true}`
	req = httptest.NewRequest("POST", "/v1/chat/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "" {
		t.Fatalf("expected service not to be called, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_CreateDirectRoom_InvalidTargetUserID(t *testing.T) {
	chatSvc := &mockMatrixChatService{createdRoom: "!dm123:test"}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"targetUserID":"alice-no-prefix","isDM":true}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "" {
		t.Fatalf("expected service not to be called, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_CreateGroupRoom(t *testing.T) {
	chatSvc := &mockMatrixChatService{createdRoom: "!grp456:test"}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"name":"Test Group","memberIDs":["@peer_a:test","@peer_b:test"],"isDM":false}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "CreateGroupRoom" {
		t.Errorf("expected CreateGroupRoom, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["name"] != "Test Group" {
		t.Errorf("name = %v", chatSvc.lastArgs["name"])
	}
}

func TestMatrixChat_SendMessage_RequiresBody(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"body":""}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_SendMessage_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"body":"Hello, world!"}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["eventId"] != "$evt123" {
		t.Errorf("eventId = %v, want $evt123", resp.Data["eventId"])
	}
	if chatSvc.lastArgs["content"] != "Hello, world!" {
		t.Errorf("content mismatch: %v", chatSvc.lastArgs["content"])
	}
}

func TestMatrixChat_EditMessage(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"body":"Updated content"}`
	req := httptest.NewRequest("PUT", "/v1/chat/rooms/!room1:test/messages/$evt123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "EditMessage" {
		t.Errorf("expected EditMessage, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["newContent"] != "Updated content" {
		t.Errorf("newContent mismatch: %v", chatSvc.lastArgs["newContent"])
	}
}

func TestMatrixChat_RedactMessage(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("DELETE", "/v1/chat/rooms/!room1:test/messages/$evt123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "RedactMessage" {
		t.Errorf("expected RedactMessage, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_GetMessages_Pagination(t *testing.T) {
	now := time.Now()
	chatSvc := &mockMatrixChatService{
		messages: []contracts.MatrixMessage{
			{EventID: "$msg1", RoomID: "!room1:test", Sender: "@peer_abc:test", Content: "Hi", MsgType: "m.text", Timestamp: now},
		},
		nextToken: "tok_abc",
	}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/rooms/!room1:test/messages?limit=20&before=tok_prev", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Messages []contracts.MatrixMessage `json:"messages"`
			End      string                    `json:"end"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.End != "tok_abc" {
		t.Errorf("end = %v, want tok_abc", resp.Data.End)
	}
	if len(resp.Data.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Data.Messages))
	}
	if chatSvc.lastArgs["limit"] != 20 {
		t.Errorf("limit = %v, want 20", chatSvc.lastArgs["limit"])
	}
	if chatSvc.lastArgs["token"] != "tok_prev" {
		t.Errorf("token = %v, want tok_prev", chatSvc.lastArgs["token"])
	}
	if chatSvc.lastArgs["dir"] != "b" {
		t.Errorf("dir = %v, want b", chatSvc.lastArgs["dir"])
	}
}

func TestMatrixChat_GetMessages_AfterParam(t *testing.T) {
	chatSvc := &mockMatrixChatService{
		messages:  []contracts.MatrixMessage{},
		nextToken: "tok_next",
	}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/rooms/!room1:test/messages?after=tok_start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastArgs["token"] != "tok_start" {
		t.Errorf("token = %v, want tok_start", chatSvc.lastArgs["token"])
	}
	if chatSvc.lastArgs["dir"] != "f" {
		t.Errorf("dir = %v, want f", chatSvc.lastArgs["dir"])
	}
}

func TestMatrixChat_GetMessages_SinceDeprecated(t *testing.T) {
	chatSvc := &mockMatrixChatService{
		messages:  []contracts.MatrixMessage{},
		nextToken: "",
	}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/rooms/!room1:test/messages?since=tok_old", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastArgs["token"] != "tok_old" {
		t.Errorf("token = %v, want tok_old", chatSvc.lastArgs["token"])
	}
	if chatSvc.lastArgs["dir"] != "b" {
		t.Errorf("dir = %v, want b (since is backward-compat alias for before)", chatSvc.lastArgs["dir"])
	}
	if w.Header().Get("X-Deprecated") == "" {
		t.Error("expected X-Deprecated header for since param")
	}
}

func TestMatrixChat_MarkAsRead_RequiresEventId(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/read", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_MarkAsRead_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"eventId":"$evt999"}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/read", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastArgs["eventID"] != "$evt999" {
		t.Errorf("eventID = %v, want $evt999", chatSvc.lastArgs["eventID"])
	}
}

func TestMatrixChat_InviteToRoom_RequiresUserID(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/invite", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_JoinRoom(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/join", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "JoinRoom" {
		t.Errorf("expected JoinRoom, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_LeaveRoom(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/leave", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "LeaveRoom" {
		t.Errorf("expected LeaveRoom, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_MediaDownload(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/media/matrix.mobazha.org/abc123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type = %s, want image/png", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "private, max-age=86400, immutable" {
		t.Errorf("Cache-Control = %q, want 'private, max-age=86400, immutable'", w.Header().Get("Cache-Control"))
	}
	if chatSvc.lastCall != "DownloadMedia" {
		t.Errorf("expected DownloadMedia, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_MediaDownload_SSRF_RejectsIP(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	for _, server := range []string{"127.0.0.1", "192.168.1.1", "10.0.0.1", "::1"} {
		req := httptest.NewRequest("GET", "/v1/chat/media/"+server+"/abc123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Errorf("server=%q: expected 400, got %d", server, w.Code)
		}
	}
}

func TestMatrixChat_MediaDownload_SSRF_RejectsLocalhost(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	for _, server := range []string{"localhost", "sub.localhost"} {
		req := httptest.NewRequest("GET", "/v1/chat/media/"+server+"/abc123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Errorf("server=%q: expected 400, got %d", server, w.Code)
		}
	}
}

func TestMatrixChat_MediaDownload_SSRF_RejectsSingleLabel(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/media/intranet/abc123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400 for single-label hostname, got %d", w.Code)
	}
}

func TestMatrixChat_RoomSettings_Get(t *testing.T) {
	chatSvc := &mockMatrixChatService{
		rooms: []contracts.MatrixRoom{
			{RoomID: "!room1:test", Name: "My Room", Topic: "Hello", Encrypted: true, RoomType: "direct"},
		},
	}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/rooms/!room1:test/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["name"] != "My Room" {
		t.Errorf("name = %v, want My Room", resp.Data["name"])
	}
	if resp.Data["encrypted"] != true {
		t.Errorf("encrypted should be true")
	}
}

func TestMatrixChat_RoomSettings_Update(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"name":"New Name"}`
	req := httptest.NewRequest("PUT", "/v1/chat/rooms/!room1:test/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "SetRoomName" {
		t.Errorf("expected SetRoomName, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["name"] != "New Name" {
		t.Errorf("name = %v, want New Name", chatSvc.lastArgs["name"])
	}
}

func TestMatrixChat_RoomSettings_UpdateTopic(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"topic":"New Topic"}`
	req := httptest.NewRequest("PUT", "/v1/chat/rooms/!room1:test/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "SetRoomTopic" {
		t.Errorf("expected SetRoomTopic, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["topic"] != "New Topic" {
		t.Errorf("topic = %v, want New Topic", chatSvc.lastArgs["topic"])
	}
}

func TestMatrixChat_RoomSettings_ClearTopic(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"topic":""}`
	req := httptest.NewRequest("PUT", "/v1/chat/rooms/!room1:test/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "SetRoomTopic" {
		t.Errorf("expected SetRoomTopic, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["topic"] != "" {
		t.Errorf("topic = %v, want empty string", chatSvc.lastArgs["topic"])
	}
}

func TestMatrixChat_RoomAvatar(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := &strings.Reader{}
	_ = body
	var buf strings.Builder
	boundary := "----TestBoundary"
	buf.WriteString("------TestBoundary\r\n")
	buf.WriteString("Content-Disposition: form-data; name=\"avatar\"; filename=\"avatar.png\"\r\n")
	buf.WriteString("Content-Type: image/png\r\n\r\n")
	buf.WriteString("fake-png-data")
	buf.WriteString("\r\n------TestBoundary--\r\n")

	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/avatar", strings.NewReader(buf.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "SetRoomAvatar" {
		t.Errorf("expected SetRoomAvatar, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_Typing(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"typing":true}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/typing", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "SendTyping" {
		t.Errorf("expected SendTyping, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_GetRoomMembers(t *testing.T) {
	chatSvc := &mockMatrixChatService{
		rooms: []contracts.MatrixRoom{
			{
				RoomID: "!room1:test",
				Members: []contracts.MatrixMember{
					{UserID: "@peer_abc:test", DisplayName: "Alice", PeerID: "QmAlice123", Membership: "join"},
					{UserID: "@peer_def:test", DisplayName: "Bob", PeerID: "QmBob456", Membership: "join"},
				},
			},
		},
	}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/rooms/!room1:test/members", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []contracts.MatrixMember `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 members, got %d", len(resp.Data))
	}
	if resp.Data[0].PeerID != "QmAlice123" {
		t.Errorf("members[0].peerID = %q, want QmAlice123", resp.Data[0].PeerID)
	}
}

// Verify JSON field casing matches frontend expectations (camelCase).
func TestMatrixChat_JSONFieldCasing(t *testing.T) {
	now := time.Now()
	room := contracts.MatrixRoom{
		RoomID:   "!r1:test",
		Name:     "Test",
		IsDirect: true,
		Members: []contracts.MatrixMember{
			{UserID: "@peer_abc:test", DisplayName: "Alice", AvatarURL: "mxc://test/abc", PeerID: "QmAbc123", Membership: "join"},
		},
		LastMessage: &contracts.MatrixMessage{
			EventID:   "$evt1",
			RoomID:    "!r1:test",
			Sender:    "@peer_abc:test",
			Content:   "Hello",
			MsgType:   "m.text",
			Timestamp: now,
		},
		UnreadCount: 3,
		Encrypted:   true,
	}

	b, err := json.Marshal(room)
	if err != nil {
		t.Fatal(err)
	}

	raw := string(b)
	expectedFields := []string{
		`"roomId"`, `"isDirect"`, `"unreadCount"`, `"encrypted"`,
		`"userId"`, `"displayName"`, `"avatarUrl"`, `"peerID"`, `"membership"`,
		`"msgType"`, `"sender"`, `"content"`,
		`"id"`, // MatrixMessage.EventID uses json tag "id"
	}
	for _, field := range expectedFields {
		if !strings.Contains(raw, field) {
			t.Errorf("JSON missing camelCase field %s in:\n%s", field, raw)
		}
	}

	forbiddenFields := []string{
		`"RoomID"`, `"IsDirect"`, `"UnreadCount"`, `"UserID"`,
		`"DisplayName"`, `"AvatarURL"`, `"PeerID"`, `"MsgType"`, `"EventID"`,
	}
	for _, field := range forbiddenFields {
		if strings.Contains(raw, field) {
			t.Errorf("JSON contains PascalCase field %s — frontend expects camelCase", field)
		}
	}
}

// Ensure service-unavailable returns 503 when MatrixChat() returns nil.
func TestMatrixChat_ServiceUnavailable(t *testing.T) {
	node := &mockNode{}
	g := &Gateway{}
	r := chi.NewMux()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/v1/chat/rooms", g.handleGETMatrixChatRooms)

	req := httptest.NewRequest("GET", "/v1/chat/rooms", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503 when MatrixChat() is nil, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_SendReaction_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"key":"👍"}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/messages/$evt123/reactions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "SendReaction" {
		t.Errorf("expected SendReaction, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["key"] != "👍" {
		t.Errorf("key = %v, want 👍", chatSvc.lastArgs["key"])
	}
	if chatSvc.lastArgs["eventID"] != "$evt123" {
		t.Errorf("eventID = %v, want $evt123", chatSvc.lastArgs["eventID"])
	}
	var resp struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["eventId"] != "$reaction001" {
		t.Errorf("eventId = %v, want $reaction001", resp.Data["eventId"])
	}
}

func TestMatrixChat_SendReaction_EmptyKey(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"key":""}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/messages/$evt123/reactions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for empty key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_SendReaction_ServiceError(t *testing.T) {
	chatSvc := &mockMatrixChatService{err: fmt.Errorf("reaction failed")}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"key":"❤️"}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/messages/$evt123/reactions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500 on service error, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_EditMessage_EmptyBody(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"body":""}`
	req := httptest.NewRequest("PUT", "/v1/chat/rooms/!room1:test/messages/$evt123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_GetStatus_ServiceUnavailable(t *testing.T) {
	node := &mockNode{}
	g := &Gateway{}
	r := chi.NewMux()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/v1/chat/status", g.handleGETMatrixChatStatus)

	req := httptest.NewRequest("GET", "/v1/chat/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status should return 200 even when service is nil, got %d", w.Code)
	}
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["connected"] != false {
		t.Errorf("expected connected=false when service nil, got %v", resp.Data["connected"])
	}
	if resp.Data["verificationAvailable"] != false {
		t.Errorf("expected verificationAvailable=false when service nil, got %v", resp.Data["verificationAvailable"])
	}
}

// ===================== Block/Unblock Tests (M-002) =====================

func TestMatrixChat_BlockUser_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/users/@bad:test/block", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "BlockUser" {
		t.Errorf("expected BlockUser, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["userID"] != "@bad:test" {
		t.Errorf("userID = %v, want @bad:test", chatSvc.lastArgs["userID"])
	}
}

func TestMatrixChat_UnblockUser_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("DELETE", "/v1/chat/users/@bad:test/block", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "UnblockUser" {
		t.Errorf("expected UnblockUser, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_GetBlockedUsers_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/blocked-users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "GetBlockedUsers" {
		t.Errorf("expected GetBlockedUsers, got %s", chatSvc.lastCall)
	}
	var resp struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0] != "@blocked:test" {
		t.Errorf("unexpected blocked users: %v", resp.Data)
	}
}

// ===================== Kick Tests =====================

func TestMatrixChat_KickUser_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"userID":"@spam:test","reason":"spam"}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/kick", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "KickUser" {
		t.Errorf("expected KickUser, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["roomID"] != "!room1:test" {
		t.Errorf("unexpected roomID: %v", chatSvc.lastArgs["roomID"])
	}
	if chatSvc.lastArgs["userID"] != "@spam:test" {
		t.Errorf("unexpected userID: %v", chatSvc.lastArgs["userID"])
	}
	if chatSvc.lastArgs["reason"] != "spam" {
		t.Errorf("unexpected reason: %v", chatSvc.lastArgs["reason"])
	}
}

func TestMatrixChat_KickUser_MissingUserID(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"reason":"spam"}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/kick", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_KickUser_ServiceError(t *testing.T) {
	chatSvc := &mockMatrixChatService{err: fmt.Errorf("no permission")}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"userID":"@spam:test"}`
	req := httptest.NewRequest("POST", "/v1/chat/rooms/!room1:test/kick", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ===================== Chat Settings Tests (F-002) =====================

func TestMatrixChat_GetSettings(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("GET", "/v1/chat/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["invitePolicy"] != "auto_mobazha" {
		t.Errorf("expected invitePolicy=auto_mobazha, got %v", resp.Data["invitePolicy"])
	}
}

func TestMatrixChat_PutSettings_ValidPolicy(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"invitePolicy":"auto_all"}`
	req := httptest.NewRequest("PUT", "/v1/chat/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_PutSettings_InvalidJSON(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("PUT", "/v1/chat/settings", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for bad JSON, got %d", w.Code)
	}
}

func TestMatrixChat_PutSettings_ServiceError(t *testing.T) {
	chatSvc := &mockMatrixChatService{err: fmt.Errorf("invalid invite policy: bad_value")}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"invitePolicy":"bad_value"}`
	req := httptest.NewRequest("PUT", "/v1/chat/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid policy, got %d: %s", w.Code, w.Body.String())
	}
}

// ===================== Verification Handler Tests (F-001) =====================

func TestMatrixChat_VerificationRequest_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"userId":"@alice:test"}`
	req := httptest.NewRequest("POST", "/v1/chat/verification/request", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "StartVerification" {
		t.Errorf("expected StartVerification, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["userID"] != "@alice:test" {
		t.Errorf("expected userID @alice:test, got %v", chatSvc.lastArgs["userID"])
	}
	var resp struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["transactionId"] != "txn_123" {
		t.Errorf("expected transactionId=txn_123, got %s", resp.Data["transactionId"])
	}
}

func TestMatrixChat_VerificationRequest_EmptyUserID(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"userId":""}`
	req := httptest.NewRequest("POST", "/v1/chat/verification/request", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for empty userId, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_VerificationRequest_MissingBody(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/verification/request", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for missing userId, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_VerificationAccept_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/verification/txn_abc/accept", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "AcceptVerification" {
		t.Errorf("expected AcceptVerification, got %s", chatSvc.lastCall)
	}
	if chatSvc.lastArgs["txnID"] != "txn_abc" {
		t.Errorf("expected txnID=txn_abc, got %v", chatSvc.lastArgs["txnID"])
	}
}

func TestMatrixChat_VerificationStartSAS_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/verification/txn_abc/start-sas", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "StartSAS" {
		t.Errorf("expected StartSAS, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_VerificationConfirm_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/verification/txn_abc/confirm", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "ConfirmSAS" {
		t.Errorf("expected ConfirmSAS, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_VerificationCancel_Success(t *testing.T) {
	chatSvc := &mockMatrixChatService{}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/verification/txn_abc/cancel", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if chatSvc.lastCall != "CancelVerification" {
		t.Errorf("expected CancelVerification, got %s", chatSvc.lastCall)
	}
}

func TestMatrixChat_VerificationAccept_ServiceError(t *testing.T) {
	chatSvc := &mockMatrixChatService{err: fmt.Errorf("verification not found")}
	router, _ := newTestRouterWithChatMock(chatSvc)

	req := httptest.NewRequest("POST", "/v1/chat/verification/txn_bad/accept", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500 on service error, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixChat_VerificationRequest_ServiceError(t *testing.T) {
	chatSvc := &mockMatrixChatService{err: fmt.Errorf("user not found")}
	router, _ := newTestRouterWithChatMock(chatSvc)

	body := `{"userId":"@unknown:test"}`
	req := httptest.NewRequest("POST", "/v1/chat/verification/request", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500 on service error, got %d: %s", w.Code, w.Body.String())
	}
}
