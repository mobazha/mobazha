// Package redact provides helpers for masking sensitive data in logs,
// audit trails, and diagnostic exports. It is a toolkit—not a framework—so
// callers invoke helpers explicitly rather than relying on automatic interception.
package redact

import (
	"encoding/json"
	"net"
	"strings"
)

// Token masks a bearer/order/API token for safe logging.
// It preserves the first 8 characters for correlation while preventing
// full token reuse from log output.
func Token(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:8] + "..."
}

// Secret masks a secret key (API secret, webhook secret, etc.) by
// preserving the first 4 and last 3 characters with masked middle.
func Secret(secret string) string {
	if len(secret) <= 6 {
		if secret != "" {
			return "****"
		}
		return ""
	}
	return secret[:4] + "****" + secret[len(secret)-3:]
}

// ServerAddr masks infrastructure server addresses for safe logging.
// IP addresses are fully replaced; named hosts keep the hostname but
// strip the port.
func ServerAddr(addr string) string {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	if net.ParseIP(host) != nil {
		return "<ip>:***"
	}
	return host + ":***"
}

// sensitiveKeys is the shared registry of key names that hold secret values.
// Keys are stored lowercase for case-insensitive matching.
var sensitiveKeys = map[string]bool{
	"password":          true,
	"token":             true,
	"apikey":            true,
	"api_key":           true,
	"mnemonic":          true,
	"privatekey":        true,
	"private_key":       true,
	"secret":            true,
	"secret_key":        true,
	"secretkey":         true,
	"admin_password":    true,
	"standalone_api_key": true,
}

// IsSensitiveKey reports whether the given key name (field name, env var, etc.)
// refers to a secret value. Matching is case-insensitive.
func IsSensitiveKey(key string) bool {
	return sensitiveKeys[strings.ToLower(key)]
}

// RedactMap returns a shallow copy of m with sensitive keys replaced by
// "[REDACTED]". Non-sensitive values are copied as-is.
func RedactMap(m map[string]any) map[string]any {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if IsSensitiveKey(k) {
			out[k] = "[REDACTED]"
		} else {
			out[k] = v
		}
	}
	return out
}

// RedactMapJSON is a convenience wrapper that returns a JSON string
// of the redacted map, suitable for structured logging.
func RedactMapJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	data, _ := json.Marshal(RedactMap(m))
	return string(data)
}

// SanitizeEnvBlock redacts values of sensitive keys in a multi-line
// KEY=VALUE text block (e.g. .env file content).
func SanitizeEnvBlock(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && IsSensitiveKey(strings.TrimSpace(parts[0])) {
			lines = append(lines, parts[0]+"=<REDACTED>")
		} else {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
