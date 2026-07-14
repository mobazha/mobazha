package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	netutil "github.com/mobazha/mobazha/internal/net"
	"github.com/mobazha/mobazha/pkg/response"
)

const (
	casdoorCertFile = "casdoor_certificate"
	ownerUserIDFile = "owner_user_id"
)

// CertFilePath returns the path to the persisted Casdoor certificate.
func CertFilePath(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, casdoorCertFile)
}

// OwnerUserIDFilePath returns the path to the persisted owner user ID.
func OwnerUserIDFilePath(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, ownerUserIDFile)
}

// LoadPersistedPlatformConfig reads certificate and ownerUserID from the data
// directory. Returns empty strings (not errors) when files don't exist.
func LoadPersistedPlatformConfig(dataDir string) (certPEM, ownerUID string) {
	if dataDir == "" {
		return "", ""
	}
	if b, err := os.ReadFile(CertFilePath(dataDir)); err == nil {
		certPEM = string(b)
	}
	if b, err := os.ReadFile(OwnerUserIDFilePath(dataDir)); err == nil {
		ownerUID = string(b)
	}
	return
}

func (g *Gateway) platformDataDir() string {
	if g != nil && g.config != nil && g.config.DataDir != "" {
		return g.config.DataDir
	}
	return g.setupDataDir()
}

type connectPlatformRequest struct {
	Token string `json:"token"`
}

type connectPlatformResponse struct {
	CasdoorAvailable bool   `json:"casdoorAvailable"`
	OwnerUserID      string `json:"ownerUserId"`
}

type disconnectPlatformResponse struct {
	Disconnected bool `json:"disconnected"`
}

type refreshPlatformCredentialResponse struct {
	Refreshed bool `json:"refreshed"`
}

// handlePOSTRefreshPlatformCredential performs a fresh Peer-signed store
// registration and hot-reloads the resulting platform credential. It does not
// authenticate or associate a Casdoor account.
//
// POST /v1/system/refresh-platform-credential
// Auth: local administrator
func (g *Gateway) handlePOSTRefreshPlatformCredential(w http.ResponseWriter, r *http.Request) {
	if g == nil || g.config == nil || g.config.SaaSAPIURL == "" || g.config.LocalPeerID == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Platform credential recovery is not configured")
		return
	}

	apiKey, err := g.refreshStoreCredential(r.Context())
	if err != nil || apiKey == "" {
		log.Warningf("refresh platform credential failed: %v", err)
		response.Error(w, http.StatusBadGateway, response.CodeServiceUnavail,
			"Unable to refresh the store platform credential")
		return
	}
	g.SetStandaloneAPIKey(apiKey)
	response.Success(w, refreshPlatformCredentialResponse{Refreshed: true})
}

// handleDELETEConnectPlatform removes the optional account association from
// Hosting first and then from the local administrator boundary. Store Peer
// identity and commerce data remain unchanged.
//
// DELETE /v1/system/connect-platform
// Auth: local administrator
func (g *Gateway) handleDELETEConnectPlatform(w http.ResponseWriter, r *http.Request) {
	if g == nil || g.config == nil || g.config.SaaSAPIURL == "" ||
		g.config.LocalPeerID == "" || g.StandaloneAPIKey() == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Platform account disconnect is not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	status, err := g.callSaaSOwnerDisconnect(ctx, g.config.LocalPeerID)
	if err != nil {
		log.Warningf("disconnect platform owner failed: %v", err)
		if status == http.StatusUnauthorized {
			response.Error(w, http.StatusUnauthorized, response.CodeStoreCredentialInvalid,
				"Store platform credential is invalid or revoked")
			return
		}
		response.Error(w, http.StatusBadGateway, response.CodeServiceUnavail,
			"Unable to disconnect the platform account")
		return
	}

	ownerPath := OwnerUserIDFilePath(g.platformDataDir())
	if ownerPath != "" {
		if err := os.Remove(ownerPath); err != nil && !os.IsNotExist(err) {
			log.Errorf("Failed to remove persisted owner user ID: %v", err)
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
				"Platform account was disconnected remotely but local cleanup failed; retry")
			return
		}
	}
	if validator := g.getJWTValidator(); validator != nil {
		validator.UpdateOwnerUserID("")
	}
	response.Success(w, disconnectPlatformResponse{Disconnected: true})
}

// handlePOSTConnectPlatform binds the admin's Casdoor identity to this
// standalone store, enabling admin social login (Telegram / Discord / etc.).
//
// POST /v1/system/connect-platform
// Auth: Basic Auth (admin only)
// Body: { "token": "<Casdoor JWT from SaaS Bridge popup>" }
//
// The Casdoor certificate is normally auto-fetched on startup. If it hasn't
// been fetched yet (SaaS unreachable at boot), this endpoint fetches it as
// a fallback before validating the token.
func (g *Gateway) handlePOSTConnectPlatform(w http.ResponseWriter, r *http.Request) {
	dataDir := g.platformDataDir()
	if dataDir == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Not available in this mode")
		return
	}

	var req connectPlatformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"Invalid request body")
		return
	}
	if req.Token == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"token is required")
		return
	}

	// Ensure jwtValidator is available. If auto-fetch at startup succeeded,
	// it's already initialized. Otherwise fetch now as fallback.
	g.mu.RLock()
	hasValidator := g.jwtValidator != nil
	g.mu.RUnlock()

	if !hasValidator {
		saasURL := g.config.SaaSAPIURL
		if saasURL == "" {
			response.Error(w, http.StatusServiceUnavailable, response.CodeInternalError,
				"SaaS API URL not configured")
			return
		}

		certPEM, err := netutil.FetchCasdoorCertificate(r.Context(), saasURL)
		if err != nil {
			log.Errorf("Failed to fetch Casdoor certificate from %s: %v", saasURL, err)
			response.Error(w, http.StatusBadGateway, response.CodeInternalError,
				"Failed to fetch platform certificate")
			return
		}

		if err := os.WriteFile(CertFilePath(dataDir), []byte(certPEM), 0600); err != nil {
			log.Errorf("Failed to persist Casdoor certificate: %v", err)
		}

		localPeerID := g.config.LocalPeerID
		if err := g.EnableJWTAuth(certPEM, localPeerID, ""); err != nil {
			log.Errorf("Failed to enable JWT auth: %v", err)
			response.Error(w, http.StatusBadGateway, response.CodeInternalError,
				"Platform certificate is invalid")
			return
		}
		log.Infof("Casdoor certificate fetched on-demand via connect-platform")
	}

	g.mu.RLock()
	jv := g.jwtValidator
	g.mu.RUnlock()

	claims, err := jv.ValidateToken(req.Token)
	if err != nil {
		log.Warningf("connect-platform: token validation failed: %v", err)
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
			"Token validation failed — ensure you logged in on the SaaS platform")
		return
	}
	if claims.Id == "" {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
			"Token missing user ID")
		return
	}

	// Hosting is the platform account authority. Do not report a successful
	// connection or persist local admin ownership unless the Peer claim has
	// been confirmed there.
	if g.config.SaaSAPIURL == "" || g.StandaloneAPIKey() == "" || g.config.LocalPeerID == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Store platform credential is not configured")
		return
	}
	if err := g.callSaaSClaim(r.Context(), g.config.LocalPeerID, claims.Id, req.Token); err != nil {
		log.Warningf("SaaS store claim failed: %v", err)
		response.Error(w, http.StatusBadGateway, response.CodeInternalError,
			"Platform did not confirm the store account association")
		return
	}

	if err := os.WriteFile(OwnerUserIDFilePath(dataDir), []byte(claims.Id), 0600); err != nil {
		log.Errorf("Failed to persist owner user ID: %v", err)
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
			"Failed to save platform configuration")
		return
	}

	jv.UpdateOwnerUserID(claims.Id)
	log.Infof("Platform connected: ownerUserID=%s bound to standalone store", claims.Id)

	log.Infof("SaaS store claimed: peerID=%s ownerUserID=%s", g.config.LocalPeerID, claims.Id)

	response.Success(w, connectPlatformResponse{
		CasdoorAvailable: true,
		OwnerUserID:      claims.Id,
	})
}
