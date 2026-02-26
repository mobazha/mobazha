package notifier

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectChats_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottest-token/getUpdates" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"result": []map[string]interface{}{
				{
					"update_id": 1,
					"message": map[string]interface{}{
						"chat": map[string]interface{}{
							"id":    -1001234567890,
							"title": "Test Group",
							"type":  "supergroup",
						},
					},
				},
				{
					"update_id": 2,
					"message": map[string]interface{}{
						"chat": map[string]interface{}{
							"id":    -1001234567890,
							"title": "Test Group",
							"type":  "supergroup",
						},
					},
				},
				{
					"update_id": 3,
					"message": map[string]interface{}{
						"chat": map[string]interface{}{
							"id":   99999,
							"title": "",
							"type":  "private",
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	sender := NewTelegramSender(srv.Client())
	chats, err := sender.DetectChats("test-token", srv.URL)
	if err != nil {
		t.Fatalf("DetectChats error: %v", err)
	}

	if len(chats) != 2 {
		t.Fatalf("expected 2 distinct chats, got %d", len(chats))
	}

	if chats[0].ID != "-1001234567890" {
		t.Errorf("expected first chat ID -1001234567890, got %s", chats[0].ID)
	}
	if chats[0].Title != "Test Group" {
		t.Errorf("expected title 'Test Group', got %s", chats[0].Title)
	}

	if chats[1].ID != "99999" {
		t.Errorf("expected second chat ID 99999, got %s", chats[1].ID)
	}
	if chats[1].Title != "private" {
		t.Errorf("expected title fallback to 'private', got %s", chats[1].Title)
	}
}

func TestDetectChats_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":          false,
			"description": "Unauthorized",
		})
	}))
	defer srv.Close()

	sender := NewTelegramSender(srv.Client())
	_, err := sender.DetectChats("bad-token", srv.URL)
	if err == nil {
		t.Fatal("expected error for bad token")
	}
}

func TestDetectChats_EmptyUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"result": []interface{}{},
		})
	}))
	defer srv.Close()

	sender := NewTelegramSender(srv.Client())
	chats, err := sender.DetectChats("test-token", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chats) != 0 {
		t.Errorf("expected 0 chats, got %d", len(chats))
	}
}

func TestDetectChats_DefaultBaseURL(t *testing.T) {
	sender := NewTelegramSender(&http.Client{})
	// Passing empty baseURL should use the default; we can't really test against
	// the real API, but we verify the function doesn't panic with empty base_url.
	_, _ = sender.DetectChats("fake-token", "")
}
