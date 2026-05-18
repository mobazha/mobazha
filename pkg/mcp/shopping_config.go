package mcp

import (
	"os"
	"strconv"
	"strings"
)

// LoadShoppingConfigFromEnv builds the Phase 0 shopping config from environment
// variables. The demo tools stay disabled unless a demo store peer ID is set.
func LoadShoppingConfigFromEnv() *ShoppingConfig {
	peerID := strings.TrimSpace(firstNonEmptyEnv("DEMO_STORE_PEER_ID", "MCP_DEMO_STORE_PEER_ID"))
	if peerID == "" {
		return nil
	}

	cfg := &ShoppingConfig{
		DemoStorePeerID: peerID,
		AllowedSlugs: splitCSVEnv(
			firstNonEmptyEnv("DEMO_ALLOWED_SLUGS", "MCP_DEMO_ALLOWED_SLUGS"),
		),
		DemoOrderToken: strings.TrimSpace(
			firstNonEmptyEnv("DEMO_ORDER_TOKEN", "MCP_DEMO_ORDER_TOKEN"),
		),
	}

	if raw := strings.TrimSpace(firstNonEmptyEnv("DEMO_MAX_ORDER_AMOUNT", "MCP_DEMO_MAX_ORDER_AMOUNT")); raw != "" {
		if amount, err := strconv.ParseFloat(raw, 64); err == nil && amount > 0 {
			cfg.MaxOrderAmount = amount
		}
	}

	return cfg
}

// LoadQuoteTokenSecretFromEnv returns the optional HMAC secret used to sign
// quote tokens. When empty, the signer falls back to a per-process random key.
func LoadQuoteTokenSecretFromEnv() []byte {
	secret := strings.TrimSpace(firstNonEmptyEnv("QUOTE_TOKEN_SECRET", "MCP_QUOTE_TOKEN_SECRET"))
	if secret == "" {
		return nil
	}
	return []byte(secret)
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func splitCSVEnv(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
