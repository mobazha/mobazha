package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

type inMemoryAuditLogger struct {
	mu      sync.Mutex
	entries []MCPAuditEntry
}

func (l *inMemoryAuditLogger) Log(entry MCPAuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *inMemoryAuditLogger) getEntries() []MCPAuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]MCPAuditEntry, len(l.entries))
	copy(cp, l.entries)
	return cp
}

func TestWithAudit_LogsSuccess(t *testing.T) {
	logger := &inMemoryAuditLogger{}
	identityFn := StaticIdentityFunc("user-1", "QmPeer1")

	innerHandler := func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		return gomcp.NewToolResultText("ok"), nil
	}

	wrapped := WithAudit(logger, "stdio", identityFn, innerHandler, "test_tool")

	req := gomcp.CallToolRequest{}
	result, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("expected success result")
	}

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", e.UserID)
	}
	if e.PeerID != "QmPeer1" {
		t.Errorf("expected QmPeer1, got %s", e.PeerID)
	}
	if e.ToolName != "test_tool" {
		t.Errorf("expected test_tool, got %s", e.ToolName)
	}
	if !e.Success {
		t.Error("expected success=true")
	}
	if e.Transport != "stdio" {
		t.Errorf("expected stdio transport, got %s", e.Transport)
	}
	if e.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestWithAudit_LogsError(t *testing.T) {
	logger := &inMemoryAuditLogger{}
	identityFn := StaticIdentityFunc("user-2", "QmPeer2")

	innerHandler := func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		return nil, fmt.Errorf("db connection failed")
	}

	wrapped := WithAudit(logger, "sse", identityFn, innerHandler, "failing_tool")

	_, err := wrapped(context.Background(), gomcp.CallToolRequest{})
	if err == nil {
		t.Fatal("expected error")
	}

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Success {
		t.Error("expected success=false for error")
	}
	if e.ErrorMsg != "db connection failed" {
		t.Errorf("expected 'db connection failed', got %s", e.ErrorMsg)
	}
}

func TestWithAudit_NilLogger(t *testing.T) {
	called := false
	innerHandler := func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		called = true
		return gomcp.NewToolResultText("ok"), nil
	}

	wrapped := WithAudit(nil, "stdio", nil, innerHandler, "test_tool")
	wrapped(context.Background(), gomcp.CallToolRequest{})

	if !called {
		t.Error("inner handler should have been called")
	}
}

func TestWithAudit_RecordsDuration(t *testing.T) {
	logger := &inMemoryAuditLogger{}
	identityFn := StaticIdentityFunc("u", "p")

	innerHandler := func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		time.Sleep(10 * time.Millisecond)
		return gomcp.NewToolResultText("ok"), nil
	}

	wrapped := WithAudit(logger, "stdio", identityFn, innerHandler, "slow_tool")
	wrapped(context.Background(), gomcp.CallToolRequest{})

	entries := logger.getEntries()
	if entries[0].Duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", entries[0].Duration)
	}
}

func TestRedactArguments(t *testing.T) {
	args := map[string]any{
		"name":     "Test Store",
		"token":    "secret-jwt-token",
		"password": "hunter2",
		"limit":    10,
	}

	result := redactArguments(args)

	if result == "{}" {
		t.Fatal("expected non-empty result")
	}

	if !strings.Contains(result, `"[REDACTED]"`) {
		t.Errorf("expected REDACTED in output: %s", result)
	}
	if !strings.Contains(result, `"Test Store"`) {
		t.Errorf("expected non-sensitive values preserved: %s", result)
	}
	if strings.Contains(result, "secret-jwt-token") {
		t.Errorf("token should be redacted: %s", result)
	}
	if strings.Contains(result, "hunter2") {
		t.Errorf("password should be redacted: %s", result)
	}
}

func TestRedactArguments_EmptyArgs(t *testing.T) {
	result := redactArguments(nil)
	if result != "{}" {
		t.Errorf("expected '{}', got %s", result)
	}

	result = redactArguments(map[string]any{})
	if result != "{}" {
		t.Errorf("expected '{}', got %s", result)
	}
}

func TestAuditMiddleware_LogsToolName(t *testing.T) {
	logger := &inMemoryAuditLogger{}
	identityFn := StaticIdentityFunc("user-mid", "QmPeerMid")

	mw := AuditMiddleware(logger, "stdio", identityFn)

	innerHandler := func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		return gomcp.NewToolResultText("ok"), nil
	}
	wrapped := mw(innerHandler)

	req := gomcp.CallToolRequest{}
	req.Params.Name = "listings_list_mine"
	result, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("expected success result")
	}

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.ToolName != "listings_list_mine" {
		t.Errorf("expected listings_list_mine, got %s", e.ToolName)
	}
	if e.UserID != "user-mid" {
		t.Errorf("expected user-mid, got %s", e.UserID)
	}
	if !e.Success {
		t.Error("expected success=true")
	}
}

func TestStaticIdentityFunc(t *testing.T) {
	fn := StaticIdentityFunc("user-abc", "QmPeer123")

	userID, peerID := fn(gomcp.CallToolRequest{})
	if userID != "user-abc" {
		t.Errorf("expected user-abc, got %s", userID)
	}
	if peerID != "QmPeer123" {
		t.Errorf("expected QmPeer123, got %s", peerID)
	}
}
