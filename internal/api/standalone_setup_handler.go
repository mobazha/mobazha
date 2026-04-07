package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/response"
)

type setupStatusResponse struct {
	SetupComplete    bool           `json:"setupComplete"`
	CompletedSteps   completedSteps `json:"completedSteps"`
	CasdoorAvailable bool           `json:"casdoorAvailable"`
	OwnerUserID      string         `json:"ownerUserId,omitempty"`
}

type completedSteps struct {
	Password    bool `json:"password"`
	Profile     bool `json:"profile"`
	Preferences bool `json:"preferences"`
	Payment     bool `json:"payment"`
}

type initialSetupRequest struct {
	Password string `json:"password"`
}

// setupDataDir derives the data directory from g.auth.hashFile.
// Returns empty string if hashFile is not configured.
func (g *Gateway) setupDataDir() string {
	g.auth.mu.RLock()
	hashFile := g.auth.hashFile
	g.auth.mu.RUnlock()
	if hashFile == "" {
		return ""
	}
	return filepath.Dir(hashFile)
}

// isSetupComplete checks whether the setup_complete flag file exists.
func (g *Gateway) isSetupComplete() bool {
	dataDir := g.setupDataDir()
	if dataDir == "" {
		return true
	}
	_, err := os.Stat(SetupCompleteFilePath(dataDir))
	return err == nil
}

// handleGETSetup returns the current setup status for standalone first-run wizard.
//
// GET /v1/system/setup (public, no auth required)
func (g *Gateway) handleGETSetup(w http.ResponseWriter, r *http.Request) {
	dataDir := g.setupDataDir()
	if dataDir == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Setup not available in this mode")
		return
	}

	passwordDone := g.isSetupComplete()

	var profileDone, prefsDone, paymentDone bool

	node := getNodeService(r)

	profileSvc := node.Profile()
	if profileSvc != nil {
		if profile, err := profileSvc.GetMyProfile(); err == nil && profile != nil {
			profileDone = strings.TrimSpace(profile.Name) != ""
		}
	}

	prefsSvc := node.Preferences()
	if prefsSvc != nil {
		if prefs, err := prefsSvc.GetPreferences(); err == nil && prefs != nil {
			prefsDone = prefs.LocalCurrency != ""
		}
	}

	walletSvc := node.Wallet()
	if walletSvc != nil {
		if accounts, err := walletSvc.GetReceivingAccounts(); err == nil {
			paymentDone = len(accounts) > 0
		}
	}

	setupComplete := passwordDone && profileDone

	var ownerUID string
	if jv := g.getJWTValidator(); jv != nil {
		ownerUID = jv.OwnerUserID()
	}

	response.Success(w, setupStatusResponse{
		SetupComplete:    setupComplete,
		CasdoorAvailable: g.getJWTValidator() != nil,
		OwnerUserID:      ownerUID,
		CompletedSteps: completedSteps{
			Password:    passwordDone,
			Profile:     profileDone,
			Preferences: prefsDone,
			Payment:     paymentDone,
		},
	})
}

// handlePOSTSetup handles the one-time initial password setup for standalone nodes.
// This endpoint is permanently disabled after the first successful call.
//
// POST /v1/system/setup (public, no auth — one-time only)
func (g *Gateway) handlePOSTSetup(w http.ResponseWriter, r *http.Request) {
	dataDir := g.setupDataDir()
	if dataDir == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Setup not available in this mode")
		return
	}

	if g.isSetupComplete() {
		response.Error(w, http.StatusForbidden, response.CodeForbidden,
			"Initial setup already completed")
		return
	}

	var req initialSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"Invalid request body")
		return
	}

	if len(req.Password) < 8 || len(req.Password) > 128 {
		response.Error(w, http.StatusBadRequest, response.CodeValidation,
			"Password must be between 8 and 128 characters")
		return
	}

	h := sha256.Sum256([]byte(req.Password))
	hashHex := hex.EncodeToString(h[:])

	hashPath := HashFilePath(dataDir)
	if err := os.WriteFile(hashPath, []byte(hashHex), 0600); err != nil {
		log.Errorf("Failed to write password hash: %v", err)
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
			"Failed to save password")
		return
	}

	g.auth.mu.Lock()
	g.auth.username = adminUsername
	g.auth.passwordHash = hashHex
	g.auth.mu.Unlock()

	plainPath := PlainFilePath(dataDir)
	if err := os.Remove(plainPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warningf("Failed to remove plaintext password file: %v", err)
	}

	flagPath := SetupCompleteFilePath(dataDir)
	if err := os.WriteFile(flagPath, []byte("1"), 0600); err != nil {
		log.Errorf("Failed to write setup_complete flag: %v", err)
	}

	response.Success(w, map[string]string{
		"username": adminUsername,
	})
}

// handleSetup dispatches GET/POST for /v1/system/setup.
func (g *Gateway) handleSetup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.handleGETSetup(w, r)
	case http.MethodPost:
		g.handlePOSTSetup(w, r)
	default:
		response.Error(w, http.StatusMethodNotAllowed, response.CodeBadRequest,
			"Method not allowed")
	}
}

// CheckSetupWarning logs a warning if standalone setup has not been completed.
// Call this from the startup sequence after a delay.
func CheckSetupWarning(dataDir string) {
	if dataDir == "" {
		return
	}
	flagPath := SetupCompleteFilePath(dataDir)
	if _, err := os.Stat(flagPath); errors.Is(err, os.ErrNotExist) {
		log.Warning("⚠️  Standalone setup not completed. Visit the admin panel to set your password and configure the store.")
	}
}

