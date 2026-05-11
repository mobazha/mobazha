package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// testGateway creates a minimal Gateway with auth configured for testing.
func testGateway(t *testing.T, password string) *Gateway {
	t.Helper()
	dir := t.TempDir()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return &Gateway{
		config: &GatewayConfig{},
		auth: authState{
			username:     "admin",
			passwordHash: hash,
			hashFile:     filepath.Join(dir, adminHashFile),
			plainFile:    filepath.Join(dir, adminPasswordFile),
		},
	}
}

func TestAuthMiddleware_RejectsNoAuth(t *testing.T) {
	g := testGateway(t, "testpass")

	handler := g.AuthenticationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if rr.Header().Get("WWW-Authenticate") != "" {
		t.Error("API routes should not send WWW-Authenticate (triggers browser native dialog)")
	}
}

func TestAuthMiddleware_RejectsWrongPassword(t *testing.T) {
	g := testGateway(t, "testpass")

	handler := g.AuthenticationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
	req.SetBasicAuth("admin", "wrongpass")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_AcceptsCorrectCredentials(t *testing.T) {
	g := testGateway(t, "testpass")

	handler := g.AuthenticationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
	req.SetBasicAuth("admin", "testpass")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_SkipsWhenNoCredentials(t *testing.T) {
	g := &Gateway{
		config: &GatewayConfig{},
		auth: authState{
			username:     "",
			passwordHash: "",
		},
	}

	handler := g.AuthenticationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 when no auth configured, got %d", rr.Code)
	}
}

func TestChangePassword_RejectsGET(t *testing.T) {
	g := testGateway(t, "testpass")

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/password", nil)
	rr := httptest.NewRecorder()
	g.handleChangePassword(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestChangePassword_Success(t *testing.T) {
	g := testGateway(t, "oldpassword")

	body, _ := json.Marshal(changePasswordRequest{
		CurrentPassword: "oldpassword",
		NewPassword:     "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	g.handleChangePassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(g.auth.passwordHash), []byte("newpassword123")); err != nil {
		t.Errorf("password hash in memory doesn't match new password: %v", err)
	}

	persistedHash, err := os.ReadFile(g.auth.hashFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword(persistedHash, []byte("newpassword123")); err != nil {
		t.Errorf("persisted hash doesn't match new password: %v", err)
	}
}

func TestChangePassword_RejectsWrongCurrent(t *testing.T) {
	g := testGateway(t, "testpass")

	body, _ := json.Marshal(changePasswordRequest{
		CurrentPassword: "wrongpass",
		NewPassword:     "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	g.handleChangePassword(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestChangePassword_RejectsShortPassword(t *testing.T) {
	g := testGateway(t, "testpass")

	body, _ := json.Marshal(changePasswordRequest{
		CurrentPassword: "testpass",
		NewPassword:     "short",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	g.handleChangePassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestChangePassword_RejectsTooLongPassword(t *testing.T) {
	g := testGateway(t, "testpass")

	longPw := strings.Repeat("a", 129)
	body, _ := json.Marshal(changePasswordRequest{
		CurrentPassword: "testpass",
		NewPassword:     longPw,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	g.handleChangePassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestChangePassword_RejectsWhenNoAuth(t *testing.T) {
	g := &Gateway{
		config: &GatewayConfig{},
		auth:   authState{},
	}

	body, _ := json.Marshal(changePasswordRequest{
		CurrentPassword: "anything",
		NewPassword:     "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	g.handleChangePassword(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

func TestEnsureStandaloneAuth_GeneratesOnFirstRun(t *testing.T) {
	dir := t.TempDir()

	username, hash, err := EnsureStandaloneAuth(dir)
	if err != nil {
		t.Fatal(err)
	}

	if username != adminUsername {
		t.Errorf("expected username %q, got %q", adminUsername, username)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Errorf("expected bcrypt hash, got %q", hash)
	}

	hashPath := filepath.Join(dir, adminHashFile)
	if _, err := os.Stat(hashPath); os.IsNotExist(err) {
		t.Error("hash file should exist")
	}

	plainPath := filepath.Join(dir, adminPasswordFile)
	if _, err := os.Stat(plainPath); os.IsNotExist(err) {
		t.Error("plaintext file should exist for first-run retrieval")
	}
}

func TestEnsureStandaloneAuth_ReusesExistingHash(t *testing.T) {
	dir := t.TempDir()

	_, hash1, err := EnsureStandaloneAuth(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Remove plaintext to simulate post-setup state
	_ = os.Remove(AdminPasswordPlaintextPath(dir))

	_, hash2, err := EnsureStandaloneAuth(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hash2 != hash1 {
		t.Error("hash should be the same across inits")
	}
}

func TestEnsureStandaloneAuth_HashesExistingPlaintext(t *testing.T) {
	dir := t.TempDir()
	plainPath := filepath.Join(dir, adminPasswordFile)
	if err := os.WriteFile(plainPath, []byte("docker-env-password"), 0600); err != nil {
		t.Fatal(err)
	}

	username, hash, err := EnsureStandaloneAuth(dir)
	if err != nil {
		t.Fatal(err)
	}
	if username != adminUsername {
		t.Errorf("expected %q, got %q", adminUsername, username)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("docker-env-password")); err != nil {
		t.Errorf("bcrypt hash should verify docker-env-password: %v", err)
	}

	if _, err := os.Stat(plainPath); !os.IsNotExist(err) {
		t.Error("plaintext file should be removed after hashing")
	}
}

func TestGenerateAdminPassword_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pw, err := GenerateAdminPassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(pw) != adminPasswordLength {
			t.Errorf("expected length %d, got %d", adminPasswordLength, len(pw))
		}
		if seen[pw] {
			t.Errorf("duplicate password generated: %s", pw)
		}
		seen[pw] = true
	}
}

func TestLoadCredentials_HashFileTakesPriority(t *testing.T) {
	dir := t.TempDir()
	hashPath := filepath.Join(dir, adminHashFile)

	// Write a hash file that differs from config value.
	fileHash := "aaaa_hash_from_file"
	if err := os.WriteFile(hashPath, []byte(fileHash), 0600); err != nil {
		t.Fatal(err)
	}

	user, hash := LoadCredentials(dir, "alice", "bbbb_hash_from_config")
	if user != "alice" {
		t.Errorf("expected config username 'alice', got %q", user)
	}
	if hash != fileHash {
		t.Errorf("hash file should take priority: got %q, want %q", hash, fileHash)
	}
}

func TestLoadCredentials_FallsBackToConfig(t *testing.T) {
	dir := t.TempDir() // no hash file

	user, hash := LoadCredentials(dir, "alice", "config_hash")
	if user != "alice" || hash != "config_hash" {
		t.Errorf("expected config values, got (%q, %q)", user, hash)
	}
}

func TestLoadCredentials_EmptyDir(t *testing.T) {
	user, hash := LoadCredentials("", "alice", "config_hash")
	if user != "alice" || hash != "config_hash" {
		t.Errorf("expected config values with empty dir, got (%q, %q)", user, hash)
	}
}

func TestLoadCredentials_HashFileWithDefaultUsername(t *testing.T) {
	dir := t.TempDir()
	hashPath := filepath.Join(dir, adminHashFile)
	if err := os.WriteFile(hashPath, []byte("file_hash"), 0600); err != nil {
		t.Fatal(err)
	}

	user, hash := LoadCredentials(dir, "", "")
	if user != adminUsername {
		t.Errorf("expected default username %q, got %q", adminUsername, user)
	}
	if hash != "file_hash" {
		t.Errorf("expected hash from file, got %q", hash)
	}
}

func TestAuthMiddleware_CookieValidation(t *testing.T) {
	g := &Gateway{
		config: &GatewayConfig{
			Cookie: "secret-cookie-value",
		},
		auth: authState{},
	}

	handler := g.AuthenticationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Wrong cookie
	req := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: "wrong"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("wrong cookie: expected 403, got %d", rr.Code)
	}

	// Correct cookie
	req2 := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil)
	req2.AddCookie(&http.Cookie{Name: AuthCookieName, Value: "secret-cookie-value"})
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("correct cookie: expected 200, got %d", rr2.Code)
	}
}
