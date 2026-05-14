package api

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// originRequestMeta holds the original HTTP request metadata captured at the
// Huma middleware layer. nodeBridgeRequest restores these onto synthetic
// requests so legacy handlers see the same headers, Host, RemoteAddr,
// and TLS state as the original inbound request.
//
// OriginalHeaders stores ALL HTTP headers from the inbound request
// (via humachi.Unwrap). This eliminates the need for per-header
// allowlists and ensures webhook signatures, custom headers, etc.
// are transparently forwarded through the bridge.
type originRequestMeta struct {
	OriginalHeaders http.Header // all inbound HTTP headers
	Host            string
	RemoteAddr      string
	IsTLS           bool
}

type originRequestMetaKey struct{}

// withOriginMeta stores the original request metadata into context.
func withOriginMeta(ctx context.Context, meta *originRequestMeta) context.Context {
	return context.WithValue(ctx, originRequestMetaKey{}, meta)
}

// getOriginMeta retrieves the stored original request metadata from context.
func getOriginMeta(ctx context.Context) *originRequestMeta {
	if m, ok := ctx.Value(originRequestMetaKey{}).(*originRequestMeta); ok {
		return m
	}
	return nil
}

// extractCookieFromMeta parses a named cookie value from the Cookie header
// stored in originRequestMeta.OriginalHeaders.
func extractCookieFromMeta(meta *originRequestMeta, name string) string {
	if meta == nil {
		return ""
	}
	raw := meta.OriginalHeaders.Get("Cookie")
	if raw == "" {
		return ""
	}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		if part[:eq] == name {
			return part[eq+1:]
		}
	}
	return ""
}

// nodeBridgeRequest builds a synthetic *http.Request that carries the
// huma-managed context (which includes nodeContextKey and AuthIdentity)
// so legacy handlers can read them via getNodeService(r)
// and GetAuthIdentity(r.Context()).
//
// It restores ALL original HTTP headers plus Host, RemoteAddr and TLS
// state from the originRequestMeta captured by nodeHumaOriginMiddleware.
func nodeBridgeRequest(ctx context.Context, method, rawURL string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, rawURL, body)
	req = req.WithContext(ctx)

	if meta := getOriginMeta(ctx); meta != nil {
		for k, vals := range meta.OriginalHeaders {
			req.Header[k] = vals
		}
		// Remove hop-by-hop and body-framing headers that refer to the
		// original inbound request. The synthetic body may differ in size,
		// and httptest.NewRequest will set Content-Length from the reader.
		req.Header.Del("Content-Length")
		req.Header.Del("Transfer-Encoding")
		req.Header.Del("Trailer")
		req.Header.Del("Connection")

		if meta.Host != "" {
			req.Host = meta.Host
		}
		if meta.RemoteAddr != "" {
			req.RemoteAddr = meta.RemoteAddr
		}
		if meta.IsTLS {
			req.TLS = &tls.ConnectionState{}
		}
	}
	return req
}

// nodeBridgeRequestWithVars is like nodeBridgeRequest but also injects
// chi URL parameters so legacy handlers using chi.URLParam(r, key) work.
func nodeBridgeRequestWithVars(ctx context.Context, method, rawURL string, body io.Reader, vars map[string]string) *http.Request {
	req := nodeBridgeRequest(ctx, method, rawURL, body)
	if len(vars) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range vars {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	return req
}

// nodeBridgeSuccessData calls a legacy handler via httptest.NewRecorder,
// unwraps the response envelope and returns the inner "data" payload.
// On non-2xx status it returns a huma error preserving the original code
// and message.
func nodeBridgeSuccessData(rr *httptest.ResponseRecorder) (any, error) {
	if rr.Code < http.StatusOK || rr.Code >= http.StatusMultipleChoices {
		return nil, nodeBridgeToHumaError(rr)
	}
	if rr.Body.Len() == 0 {
		return nil, nil
	}
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrap); err != nil {
		return nil, huma.Error500InternalServerError("invalid node response")
	}
	if len(wrap.Data) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(wrap.Data, &out); err != nil {
		return nil, huma.Error500InternalServerError("invalid node response data")
	}
	return out, nil
}

// nodeBridgeNoContent calls a legacy handler that returns 204 No Content.
func nodeBridgeNoContent(rr *httptest.ResponseRecorder) error {
	if rr.Code >= http.StatusOK && rr.Code < http.StatusMultipleChoices {
		return nil
	}
	return nodeBridgeToHumaError(rr)
}

// nodeBridgeRawSuccess preserves the full JSON response from a legacy handler
// (including envelope fields like "data" and "meta") rather than unwrapping only "data".
func nodeBridgeRawSuccess(rr *httptest.ResponseRecorder) (any, error) {
	if rr.Code < http.StatusOK || rr.Code >= http.StatusMultipleChoices {
		return nil, nodeBridgeToHumaError(rr)
	}
	if rr.Body.Len() == 0 {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		return nil, huma.Error500InternalServerError("invalid node response")
	}
	return out, nil
}

// nodeBridgeFlexJSON unwraps a {"data":...} envelope when present; otherwise decodes
// the raw JSON body. Use for legacy handlers that do not emit the standard envelope.
func nodeBridgeFlexJSON(rr *httptest.ResponseRecorder) (any, error) {
	if rr.Code < http.StatusOK || rr.Code >= http.StatusMultipleChoices {
		return nil, nodeBridgeToHumaError(rr)
	}
	if rr.Body.Len() == 0 {
		return nil, nil
	}
	raw := rr.Body.Bytes()
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Data) > 0 {
		var out any
		if err := json.Unmarshal(wrap.Data, &out); err != nil {
			return nil, huma.Error500InternalServerError("invalid node response data")
		}
		return out, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, huma.Error500InternalServerError("invalid node response")
	}
	return out, nil
}

// nodeBridgeSSEOrFlexJSON handles legacy handlers that may return SSE (text/event-stream)
// or JSON. For SSE success responses, returns a small placeholder map so huma/OpenAPI can
// still type the operation.
func nodeBridgeSSEOrFlexJSON(rr *httptest.ResponseRecorder) (any, error) {
	if rr.Code < http.StatusOK || rr.Code >= http.StatusMultipleChoices {
		return nil, nodeBridgeToHumaError(rr)
	}
	if strings.Contains(rr.Header().Get("Content-Type"), "text/event-stream") {
		return map[string]any{"stream": true}, nil
	}
	return nodeBridgeFlexJSON(rr)
}

func nodeBridgeToHumaError(rr *httptest.ResponseRecorder) error {
	var wrap struct {
		Error response.APIError `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrap); err != nil || wrap.Error.Message == "" {
		return newNodeEnvelopeError(rr.Code, http.StatusText(rr.Code))
	}
	return &envelopeError{
		status: rr.Code,
		body:   response.ErrorEnvelope{Error: wrap.Error},
	}
}

// nodeBridgeRequestWithOptionalAuth is like nodeBridgeRequest but additionally
// performs best-effort node auth validation and injects an AuthIdentity if
// credentials are valid. Used for public endpoints that optionally accept auth
// alongside capability tokens (e.g. system setup, buyer digital assets).
//
// Security: applies rate-limit, AllowedIPs, and Cookie gate checks (mirroring
// nodeHumaAuthMiddleware) to prevent brute-force attacks on the admin password
// even though the operation itself is public (no Security declaration).
// Note: on first setup (no cookie configured yet), g.config.Cookie == "" so the
// Cookie gate is automatically skipped.
//
// Authorization header is already restored from originRequestMeta by
// nodeBridgeRequest — this method adds security checks + inline credential
// checks. Invalid credentials never fail the public route here; callers decide
// whether absence of AuthIdentity is acceptable.
func (g *Gateway) nodeBridgeRequestWithOptionalAuth(ctx context.Context, method, rawURL string, body io.Reader) *http.Request {
	req := nodeBridgeRequest(ctx, method, rawURL, body)
	if GetAuthIdentity(req.Context()) != nil {
		return req
	}

	meta := getOriginMeta(ctx)
	peerIP := "unknown"
	if meta != nil && meta.RemoteAddr != "" {
		peerIP = meta.RemoteAddr
		if host, _, err := net.SplitHostPort(peerIP); err == nil {
			peerIP = host
		}
	}

	// No up-front isBlocked check — see AuthenticationMiddleware.
	// Up-front blocking here would silently downgrade an authenticated
	// request to anonymous, which surfaces as confusing 401/404s downstream.
	if len(g.config.AllowedIPs) > 0 && !g.config.AllowedIPs[peerIP] {
		return req
	}
	if g.config.Cookie != "" {
		cookieVal := extractCookieFromMeta(meta, AuthCookieName)
		if subtle.ConstantTimeCompare([]byte(cookieVal), []byte(g.config.Cookie)) != 1 {
			return req
		}
	}

	authHeader := req.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		bearerVal := strings.TrimSpace(authHeader[7:])
		if bearerVal != "" {
			if identity, ok := g.tryAPITokenAuth(bearerVal); ok {
				return req.WithContext(WithAuthIdentity(req.Context(), identity))
			}
			if identity, ok := g.tryJWTAuthWith(g.getJWTValidator(), req); ok {
				return req.WithContext(WithAuthIdentity(req.Context(), identity))
			}
			if identity, ok := g.tryJWTSubjectWith(g.getJWTValidator(), req); ok {
				return req.WithContext(WithAuthIdentity(req.Context(), identity))
			}
		}
	}

	if g.auth.isConfigured() {
		username, password, ok := req.BasicAuth()
		if !ok {
			return req
		}
		matched, _ := g.auth.checkPassword(username, password)
		if !matched {
			if g.authLimiter != nil {
				g.authLimiter.recordFailure(peerIP)
			}
			return req
		}
		if g.authLimiter != nil {
			g.authLimiter.resetIP(peerIP)
		}
		req = req.WithContext(WithAuthIdentity(req.Context(), &AuthIdentity{
			UserID:  username,
			IsAdmin: true,
		}))
	}
	// SECURITY: do NOT inject a synthetic "anonymous IsAdmin" identity when
	// neither Basic Auth nor a JWT validator is configured. Earlier code did
	// so to make bare dev nodes more curl-friendly, but it silently granted
	// full admin scope (Scopes == nil → HasScope always true) to any caller
	// on every endpoint that uses this bridge. On production deployments
	// with misconfigured auth, that becomes a critical privilege escalation
	// for owner-only paths reachable via the bridge. Callers that need a
	// genuinely public path must not rely on AuthIdentity being present.
	return req
}

// nodeDataOutput is a generic output wrapper for bridged handlers
// whose response body is an opaque JSON object/array.
type nodeDataOutput struct {
	Body any `doc:"Success envelope inner data."`
}

// nodeNoContentOutput is used for 204 responses.
type nodeNoContentOutput struct{}

// nodeMultipartInput carries a raw body (typically multipart/form-data) and
// the original Content-Type header for bridging to legacy handlers.
type nodeMultipartInput struct {
	ContentType string `header:"Content-Type" required:"true"`
	RawBody     []byte
}

// nodeMultipartWithRoomInput extends nodeMultipartInput with a room ID path param.
type nodeMultipartWithRoomInput struct {
	RoomID      string `path:"roomID" doc:"Matrix room ID."`
	ContentType string `header:"Content-Type" required:"true"`
	RawBody     []byte
}
