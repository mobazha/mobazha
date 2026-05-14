package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setupHandlerTestGateway builds a Gateway whose auth state mirrors the
// post-EnsureStandaloneAuth bootstrap window: hash + username loaded
// (isConfigured()==true), hashFile pointing at dataDir.
func setupHandlerTestGateway(t *testing.T, dataDir string) *Gateway {
	t.Helper()
	hash, err := HashPassword("bootstrap-password")
	if err != nil {
		t.Fatal(err)
	}
	return &Gateway{
		config: &GatewayConfig{},
		auth: authState{
			username:     adminUsername,
			passwordHash: hash,
			hashFile:     filepath.Join(dataDir, adminHashFile),
			plainFile:    filepath.Join(dataDir, adminPasswordFile),
		},
	}
}

func postSetup(t *testing.T, g *Gateway, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(initialSetupRequest{Password: password})
	req := httptest.NewRequest(http.MethodPost, "/v1/system/setup", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	g.handlePOSTSetup(rr, req)
	return rr
}

// TestPOSTSetup_AllowsUnauthInBootstrapWindow exercises the legitimate
// first-run path: plaintext file still on disk, so unauthenticated wizard
// requests are allowed.
func TestPOSTSetup_AllowsUnauthInBootstrapWindow(t *testing.T) {
	dir := t.TempDir()
	g := setupHandlerTestGateway(t, dir)

	// Simulate EnsureStandaloneAuth having dropped the plaintext file.
	if err := os.WriteFile(AdminPasswordPlaintextPath(dir), []byte("bootstrap-password"), 0600); err != nil {
		t.Fatal(err)
	}

	rr := postSetup(t, g, "user-chosen-password")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 in bootstrap window, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// TestPOSTSetup_RequiresAuthAfterPlaintextRemoved confirms the existing
// post-bootstrap behavior: once the plaintext file is gone, anonymous
// callers must authenticate.
func TestPOSTSetup_RequiresAuthAfterPlaintextRemoved(t *testing.T) {
	dir := t.TempDir()
	g := setupHandlerTestGateway(t, dir)
	// No plaintext file — wizard already consumed it.

	rr := postSetup(t, g, "attacker-chosen-password")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 once plaintext removed, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// TestPOSTSetup_FailsClosedOnStatError is the regression test for the
// security bypass: any non-ErrNotExist os.Stat error (EACCES, EIO, …)
// must NOT be treated as "plaintext present". The handler must refuse
// to rotate the password rather than silently allow unauthenticated
// access.
func TestPOSTSetup_FailsClosedOnStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-denied stat is not portable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("test requires unprivileged user (chmod 0 bypassed by root)")
	}

	dataDir := t.TempDir()
	// Place the plaintext file inside a sub-directory we can lock down.
	// AdminPasswordPlaintextPath joins dataDir with adminPasswordFile;
	// to force a stat error on that path we strip read+execute on
	// dataDir itself.
	if err := os.WriteFile(AdminPasswordPlaintextPath(dataDir), []byte("bootstrap"), 0600); err != nil {
		t.Fatal(err)
	}

	g := setupHandlerTestGateway(t, dataDir)

	if err := os.Chmod(dataDir, 0o000); err != nil {
		t.Fatalf("chmod 0: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dataDir, 0o700)
	})

	rr := postSetup(t, g, "attacker-chosen-password")
	switch rr.Code {
	case http.StatusOK:
		t.Fatalf("FAIL-OPEN: stat error allowed unauth setup (200 OK)")
	case http.StatusInternalServerError, http.StatusUnauthorized:
		// Acceptable fail-closed responses.
	default:
		t.Fatalf("expected fail-closed (500 or 401), got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// TestPOSTSetup_BlockedAfterSetupComplete is a defence-in-depth check:
// once the setup_complete flag exists, every POST is rejected regardless
// of plaintext-file state.
func TestPOSTSetup_BlockedAfterSetupComplete(t *testing.T) {
	dir := t.TempDir()
	g := setupHandlerTestGateway(t, dir)

	if err := os.WriteFile(SetupCompleteFilePath(dir), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}

	rr := postSetup(t, g, "anything")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 after setup_complete, got %d; body: %s", rr.Code, rr.Body.String())
	}
}
