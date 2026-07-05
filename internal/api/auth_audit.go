package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

const authAuditSchemaVersion = 1

// AuthAuditEventType is a stable, queryable authentication security event name.
type AuthAuditEventType string

const (
	AuthAuditLoginDenied       AuthAuditEventType = "auth.login.denied"
	AuthAuditSessionCreated    AuthAuditEventType = "auth.session.created"
	AuthAuditSessionRestored   AuthAuditEventType = "auth.session.restored"
	AuthAuditSessionRejected   AuthAuditEventType = "auth.session.rejected"
	AuthAuditSessionCSRFDenied AuthAuditEventType = "auth.session.csrf_denied"
	AuthAuditSessionRevoked    AuthAuditEventType = "auth.session.revoked"
	AuthAuditPasswordRotated   AuthAuditEventType = "auth.password.rotated"
)

// AuthAuditEvent contains the deliberately narrow authentication audit schema.
// Credential material, cookies, session tokens, and CSRF tokens have no fields
// in this type and must never be added as generic attributes.
type AuthAuditEvent struct {
	SchemaVersion    int                `json:"schema_version"`
	Type             AuthAuditEventType `json:"type"`
	Outcome          string             `json:"outcome"`
	Reason           string             `json:"reason,omitempty"`
	ActorID          string             `json:"actor_id,omitempty"`
	AuthMethod       string             `json:"auth_method,omitempty"`
	ClientIP         string             `json:"client_ip,omitempty"`
	RequestMethod    string             `json:"request_method,omitempty"`
	RequestPath      string             `json:"request_path,omitempty"`
	SessionExpiresAt *time.Time         `json:"session_expires_at,omitempty"`
	RevokedSessions  int                `json:"revoked_sessions,omitempty"`
	OccurredAt       time.Time          `json:"occurred_at"`
}

// AuthAuditSink records authentication audit events. Implementations must be
// concurrency-safe, non-blocking, and must not enrich events with credentials.
type AuthAuditSink interface {
	RecordAuthAudit(context.Context, AuthAuditEvent)
}

type logAuthAuditSink struct{}

func authAuditAttributes(event AuthAuditEvent) []any {
	attrs := []any{
		"audit.category", "authentication",
		"audit.schema_version", event.SchemaVersion,
		"audit.event", string(event.Type),
		"audit.outcome", event.Outcome,
		"event.occurred_at", event.OccurredAt.UTC().Format(time.RFC3339Nano),
	}
	if event.Reason != "" {
		attrs = append(attrs, "audit.reason", event.Reason)
	}
	if event.ActorID != "" {
		attrs = append(attrs, "actor.id", event.ActorID)
	}
	if event.AuthMethod != "" {
		attrs = append(attrs, "auth.method", event.AuthMethod)
	}
	if event.ClientIP != "" {
		attrs = append(attrs, "client.ip", event.ClientIP)
	}
	if event.RequestMethod != "" {
		attrs = append(attrs, "http.method", event.RequestMethod)
	}
	if event.RequestPath != "" {
		attrs = append(attrs, "http.path", event.RequestPath)
	}
	if event.SessionExpiresAt != nil {
		attrs = append(attrs, "session.expires_at", event.SessionExpiresAt.UTC().Format(time.RFC3339Nano))
	}
	if event.RevokedSessions > 0 {
		attrs = append(attrs, "session.revoked_count", event.RevokedSessions)
	}
	return attrs
}

func (logAuthAuditSink) RecordAuthAudit(_ context.Context, event AuthAuditEvent) {
	attrs := authAuditAttributes(event)
	log.With(attrs...).Info("authentication audit event")
}

func (g *Gateway) recordAuthAudit(ctx context.Context, event AuthAuditEvent) {
	if g.authAuditSink == nil {
		return
	}
	if event.SchemaVersion == 0 {
		event.SchemaVersion = authAuditSchemaVersion
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	g.authAuditSink.RecordAuthAudit(ctx, event)
}

func authMethodFromHeader(header string) string {
	switch {
	case strings.HasPrefix(header, "Basic "):
		return "basic"
	case strings.HasPrefix(header, "Bearer "):
		return "jwt"
	default:
		return ""
	}
}

func authMethodFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if method := authMethodFromHeader(r.Header.Get("Authorization")); method != "" {
		return method
	}
	if _, err := r.Cookie(AdminSessionCookieName); err == nil {
		return "session"
	}
	return ""
}

func requestPeerIP(r *http.Request) string {
	if r == nil || r.RemoteAddr == "" {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func authAuditActorID(ctx context.Context) string {
	identity := GetAuthIdentity(ctx)
	if identity == nil {
		return ""
	}
	return identity.UserID
}

func isAdminSessionLoginOperation(op *huma.Operation) bool {
	return op != nil && op.OperationID == "auth-admin-session-post"
}

func (g *Gateway) recordHumaLoginDenied(
	ctx huma.Context,
	op *huma.Operation,
	clientIP string,
	authMethod string,
	reason string,
) {
	if !isAdminSessionLoginOperation(op) {
		return
	}
	g.recordAuthAudit(ctx.Context(), AuthAuditEvent{
		Type:          AuthAuditLoginDenied,
		Outcome:       "denied",
		Reason:        reason,
		AuthMethod:    authMethod,
		ClientIP:      clientIP,
		RequestMethod: op.Method,
		RequestPath:   op.Path,
	})
}
