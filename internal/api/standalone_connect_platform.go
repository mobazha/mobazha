package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	netutil "github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/pkg/response"
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

type connectPlatformRequest struct {
	Token string `json:"token"`
}

type connectPlatformResponse struct {
	CasdoorAvailable bool   `json:"casdoorAvailable"`
	OwnerUserID      string `json:"ownerUserId"`
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
	dataDir := g.setupDataDir()
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

	if err := os.WriteFile(OwnerUserIDFilePath(dataDir), []byte(claims.Id), 0600); err != nil {
		log.Errorf("Failed to persist owner user ID: %v", err)
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
			"Failed to save platform configuration")
		return
	}

	jv.UpdateOwnerUserID(claims.Id)
	log.Infof("Platform connected: ownerUserID=%s bound to standalone store", claims.Id)

	// Claim store on SaaS side so TenantMiddleware can resolve peerID
	// from the store_registry. Non-fatal — the local binding is complete
	// regardless of whether SaaS-side claim succeeds.
	if g.config.SaaSAPIURL != "" && g.config.StandaloneAPIKey != "" && g.config.LocalPeerID != "" {
		if err := g.callSaaSClaim(r.Context(), g.config.LocalPeerID, claims.Id); err != nil {
			log.Warningf("SaaS store claim failed (non-fatal): %v", err)
		} else {
			log.Infof("SaaS store claimed: peerID=%s ownerUserID=%s", g.config.LocalPeerID, claims.Id)
		}
	}

	response.Success(w, connectPlatformResponse{
		CasdoorAvailable: true,
		OwnerUserID:      claims.Id,
	})
}
