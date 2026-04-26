package mcp

import (
	"context"
	"net/url"
	"testing"
)

type chatBridgeCall struct {
	method string
	path   string
	body   interface{}
}

type chatBridgeResp struct {
	code int
	body []byte
	err  error
}

type chatBridgeStub struct {
	calls []chatBridgeCall
	resps []chatBridgeResp
}

func (s *chatBridgeStub) Call(
	_ context.Context,
	method, path string,
	_ url.Values,
	body interface{},
) (int, []byte, error) {
	s.calls = append(s.calls, chatBridgeCall{
		method: method,
		path:   path,
		body:   body,
	})

	if len(s.resps) == 0 {
		return 500, []byte(`{"error":{"code":"NO_STUB","message":"missing stub response"}}`), nil
	}
	resp := s.resps[0]
	s.resps = s.resps[1:]
	return resp.code, resp.body, resp.err
}

func (s *chatBridgeStub) CallMultipart(_ context.Context, _, _ string, _, _ string, _ []byte) (int, []byte, error) {
	return 500, []byte(`{"error":"not implemented"}`), nil
}

func TestExtractRoomID_DataEnvelope(t *testing.T) {
	body := []byte(`{"data":{"roomId":"!room123:matrix.local"}}`)
	roomID := extractRoomID(body)
	if roomID != "!room123:matrix.local" {
		t.Fatalf("expected roomID !room123:matrix.local, got %q", roomID)
	}
}

func TestResolveDMRoomID_FallbackPayload(t *testing.T) {
	bridge := &chatBridgeStub{
		resps: []chatBridgeResp{
			{
				code: 400,
				body: []byte(`{"error":{"code":"BAD_REQUEST","message":"targetPeerID unsupported"}}`),
			},
			{
				code: 200,
				body: []byte(`{"data":{"roomID":"!fallback:matrix.local"}}`),
			},
		},
	}

	roomID, toolErr, err := resolveDMRoomID(context.Background(), bridge, "12D3KooWPeer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if toolErr != nil {
		t.Fatalf("unexpected tool error: %+v", toolErr)
	}
	if roomID != "!fallback:matrix.local" {
		t.Fatalf("expected !fallback:matrix.local, got %q", roomID)
	}

	if len(bridge.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(bridge.calls))
	}
	if bridge.calls[0].path != "/v1/chat/rooms" || bridge.calls[1].path != "/v1/chat/rooms" {
		t.Fatalf("expected calls to /v1/chat/rooms, got %+v", bridge.calls)
	}

	firstBody, ok := bridge.calls[0].body.(map[string]interface{})
	if !ok {
		t.Fatalf("first payload type = %T, expected map[string]interface{}", bridge.calls[0].body)
	}
	secondBody, ok := bridge.calls[1].body.(map[string]interface{})
	if !ok {
		t.Fatalf("second payload type = %T, expected map[string]interface{}", bridge.calls[1].body)
	}

	if _, ok := firstBody["targetPeerID"]; !ok {
		t.Fatalf("first payload should use targetPeerID, got %+v", firstBody)
	}
	if _, ok := secondBody["peerID"]; !ok {
		t.Fatalf("second payload should fallback to peerID, got %+v", secondBody)
	}
}

func TestResolveDMRoomID_HTTPError(t *testing.T) {
	bridge := &chatBridgeStub{
		resps: []chatBridgeResp{
			{
				code: 405,
				body: []byte(`{"error":{"code":"METHOD_NOT_ALLOWED","message":"not allowed"}}`),
			},
			{
				code: 405,
				body: []byte(`{"error":{"code":"METHOD_NOT_ALLOWED","message":"not allowed"}}`),
			},
		},
	}

	roomID, toolErr, err := resolveDMRoomID(context.Background(), bridge, "12D3KooWPeer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if roomID != "" {
		t.Fatalf("expected empty roomID, got %q", roomID)
	}
	if toolErr == nil || !toolErr.IsError {
		t.Fatalf("expected tool error for 405 response, got %+v", toolErr)
	}
}
