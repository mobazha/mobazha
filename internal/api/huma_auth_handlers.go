package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type adminSessionStatus struct {
	Authenticated bool       `json:"authenticated"`
	CSRFToken     string     `json:"csrfToken,omitempty"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
}

type adminSessionOutput struct {
	SetCookie    string `header:"Set-Cookie" required:"false"`
	CacheControl string `header:"Cache-Control" required:"false"`
	Body         adminSessionStatus
}

// registerNodeHumaAuthPublicOperations registers public auth ops
// (node version fingerprint — unauthenticated health check).
func (g *Gateway) registerNodeHumaAuthPublicOperations(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-version-get",
		Method:      http.MethodGet,
		Path:        "/v1/admin/version",
		Summary:     "Node binary version fingerprint (public)",
		Tags:        []string{"system"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/admin/version", nil)
		rr := httptest.NewRecorder()
		g.handleAdminVersion(rr, req)
		data, err := nodeBridgeFlexJSON(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerNodeHumaAuthAdminOperations registers authenticated admin auth ops:
// tokens, identity, scopes.
func (g *Gateway) registerNodeHumaAuthAdminOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	type tokenIDPath struct {
		TokenID string `path:"tokenID"`
	}

	huma.Register(api, huma.Operation{
		OperationID: "auth-admin-session-post",
		Method:      http.MethodPost,
		Path:        "/v1/auth/admin-session",
		Summary:     "Create a short-lived standalone administrator session",
		Tags:        []string{"auth"},
		Security:    adminLoginAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*adminSessionOutput, error) {
		identity := GetAuthIdentity(ctx)
		if identity == nil || !identity.IsAdmin {
			return nil, huma.NewError(http.StatusUnauthorized, "Administrator authentication required")
		}
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/auth/admin-session", nil)
		token, session, err := g.ensureAdminSessionStore().issue(identity.UserID)
		if err != nil {
			g.recordAuthAudit(ctx, AuthAuditEvent{
				Type: AuthAuditSessionCreated, Outcome: "error", Reason: "issue_failed",
				ActorID: identity.UserID, AuthMethod: authMethodFromHeader(req.Header.Get("Authorization")),
				ClientIP: requestPeerIP(req), RequestMethod: req.Method, RequestPath: req.URL.Path,
			})
			log.Errorf("Failed to issue administrator session: %v", err)
			return nil, huma.NewError(http.StatusInternalServerError, "Failed to create administrator session")
		}
		expiresAt := session.ExpiresAt.UTC()
		g.recordAuthAudit(ctx, AuthAuditEvent{
			Type: AuthAuditSessionCreated, Outcome: "success",
			ActorID: identity.UserID, AuthMethod: authMethodFromHeader(req.Header.Get("Authorization")),
			ClientIP: requestPeerIP(req), RequestMethod: req.Method, RequestPath: req.URL.Path,
			SessionExpiresAt: &expiresAt,
		})
		return &adminSessionOutput{
			SetCookie:    g.adminSessionCookie(token, session.ExpiresAt, req),
			CacheControl: "no-store",
			Body: adminSessionStatus{
				Authenticated: true,
				CSRFToken:     session.CSRFToken,
				ExpiresAt:     &expiresAt,
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-admin-session-get",
		Method:      http.MethodGet,
		Path:        "/v1/auth/admin-session",
		Summary:     "Inspect the current standalone administrator session",
		Tags:        []string{"auth"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*adminSessionOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/auth/admin-session", nil)
		_, session, ok := g.adminSessionFromRequest(req)
		status := adminSessionStatus{Authenticated: true}
		if ok {
			expiresAt := session.ExpiresAt.UTC()
			status.CSRFToken = session.CSRFToken
			status.ExpiresAt = &expiresAt
			g.recordAuthAudit(ctx, AuthAuditEvent{
				Type: AuthAuditSessionRestored, Outcome: "success",
				ActorID: session.UserID, AuthMethod: "session", ClientIP: requestPeerIP(req),
				RequestMethod: req.Method, RequestPath: req.URL.Path, SessionExpiresAt: &expiresAt,
			})
		}
		return &adminSessionOutput{CacheControl: "no-store", Body: status}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-admin-session-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/auth/admin-session",
		Summary:     "Revoke the current standalone administrator session",
		Tags:        []string{"auth"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*adminSessionOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodDelete, "/v1/auth/admin-session", nil)
		token, _, _ := g.adminSessionFromRequest(req)
		revoked := g.ensureAdminSessionStore().revoke(token)
		identity := GetAuthIdentity(ctx)
		userID := ""
		if identity != nil && identity.UserID != "" {
			userID = identity.UserID
		}
		revokedCount := 0
		if revoked {
			revokedCount = 1
		}
		g.recordAuthAudit(ctx, AuthAuditEvent{
			Type: AuthAuditSessionRevoked, Outcome: "success",
			ActorID: userID, AuthMethod: authMethodFromRequest(req), ClientIP: requestPeerIP(req),
			RequestMethod: req.Method, RequestPath: req.URL.Path, RevokedSessions: revokedCount,
		})
		return &adminSessionOutput{
			SetCookie:    g.expiredAdminSessionCookie(req),
			CacheControl: "no-store",
			Body:         adminSessionStatus{Authenticated: false},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-password-post",
		Method:      http.MethodPost,
		Path:        "/v1/admin/password",
		Summary:     "Rotate standalone admin password",
		Tags:        []string{"auth"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/admin/password", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleChangePassword(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-tokens-post",
		Method:      http.MethodPost,
		Path:        "/v1/auth/tokens",
		Summary:     "Mint local API token (standalone)",
		Tags:        []string{"auth"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/auth/tokens", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAuthToken(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-tokens-get",
		Method:      http.MethodGet,
		Path:        "/v1/auth/tokens",
		Summary:     "List local API tokens (standalone)",
		Tags:        []string{"auth"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/auth/tokens", nil)
		rr := httptest.NewRecorder()
		g.handleGETAuthTokens(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-tokens-token-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/auth/tokens/{tokenID}",
		Summary:     "Revoke local API token by ID",
		Tags:        []string{"auth"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *tokenIDPath) (*nodeNoContentOutput, error) {
		raw := "/v1/auth/tokens/" + url.PathEscape(in.TokenID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, raw, nil, map[string]string{"tokenID": in.TokenID})
		rr := httptest.NewRecorder()
		g.handleDELETEAuthToken(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-identity-get",
		Method:      http.MethodGet,
		Path:        "/v1/auth/identity",
		Summary:     "Inspect resolved principal and scopes",
		Tags:        []string{"auth"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/auth/identity", nil)
		rr := httptest.NewRecorder()
		g.handleGETAuthIdentity(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-scopes-get",
		Method:      http.MethodGet,
		Path:        "/v1/auth/scopes",
		Summary:     "Enumerate token scope catalog",
		Tags:        []string{"auth"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/auth/scopes", nil)
		rr := httptest.NewRecorder()
		g.handleGETAuthScopes(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
