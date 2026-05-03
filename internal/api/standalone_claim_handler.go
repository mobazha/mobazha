//go:build !private_distribution

package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/response"
)

type claimStoreRequest struct {
	AdminPassword string `json:"admin_password"`
}

// handlePOSTClaimStore allows a Telegram Mini App user to claim this standalone
// store by proving they know the admin password. On success, the user's Casdoor
// ID is registered as the store owner via the SaaS platform.
//
// POST /v1/system/claim-store
//
// Authentication: Bearer JWT (valid signature required, but NOT admin).
// The caller must also provide the admin password in the request body.
//
// This endpoint is NOT behind the standard auth() middleware because the
// caller is not yet the store owner — they're trying to become one.
func (g *Gateway) handlePOSTClaimStore(w http.ResponseWriter, r *http.Request) {
	jv := g.getJWTValidator()
	if jv == nil {
		response.Error(w, http.StatusServiceUnavailable, response.CodeInternalError,
			"JWT validation not configured on this node")
		return
	}

	if g.config.SaaSAPIURL == "" || g.config.StandaloneAPIKey == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeInternalError,
			"SaaS integration not configured")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
			"Bearer token required")
		return
	}
	tokenStr := authHeader[7:]

	claims, err := jv.ValidateToken(tokenStr)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
			"Invalid or expired token")
		return
	}
	if claims.Id == "" {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized,
			"Token missing user ID")
		return
	}

	var req claimStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"Invalid request body")
		return
	}
	if req.AdminPassword == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"admin_password is required")
		return
	}

	h := sha256.Sum256([]byte(req.AdminPassword))
	providedHash := hex.EncodeToString(h[:])
	g.auth.mu.RLock()
	username := g.auth.username
	g.auth.mu.RUnlock()
	if !g.auth.check(username, providedHash) {
		response.Error(w, http.StatusForbidden, response.CodeForbidden,
			"Incorrect password")
		return
	}

	localPeerID := g.config.LocalPeerID
	if localPeerID == "" {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
			"Node peer ID not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := g.callSaaSClaim(ctx, localPeerID, claims.Id); err != nil {
		log.Errorf("SaaS claim failed: %v", err)
		if strings.Contains(err.Error(), "already claimed") {
			response.Error(w, http.StatusForbidden, response.CodeForbidden,
				"Store already claimed by another user")
			return
		}
		response.Error(w, http.StatusBadGateway, response.CodeInternalError,
			"Failed to register claim with platform")
		return
	}

	jv.UpdateOwnerUserID(claims.Id)

	response.Success(w, map[string]interface{}{
		"success": true,
		"message": "Store claimed successfully. Refresh to switch to owner mode.",
	})
}

// callSaaSClaim calls the hosting platform to atomically claim the store.
func (g *Gateway) callSaaSClaim(ctx context.Context, peerID, casdoorUserID string) error {
	payload, err := json.Marshal(map[string]string{
		"casdoor_user_id": casdoorUserID,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/platform/v1/stores/%s/claim", g.config.SaaSAPIURL, peerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Standalone-Store-Key", g.config.StandaloneAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call SaaS: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("already claimed: %s", string(body))
	}
	return fmt.Errorf("SaaS claim failed (%d): %s", resp.StatusCode, string(body))
}
