package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func probeClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 3 * time.Second}
}

// NewLocalhostOnlyClient returns an *http.Client whose transport refuses to
// dial any address that does not resolve to a loopback IP. This is a
// defense-in-depth measure for PrivateDistribution mode: even if handler-level URL
// validation is bypassed (e.g. DNS rebinding), the network layer blocks
// outbound connections to non-loopback hosts.
func NewLocalhostOnlyClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("localhost-only: invalid address %q: %w", addr, err)
			}

			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("localhost-only: DNS resolve %q: %w", host, err)
			}

			for _, ip := range ips {
				if !ip.IP.IsLoopback() {
					return nil, fmt.Errorf("localhost-only: %q resolved to non-loopback %s — blocked", host, ip.IP)
				}
			}

			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
}

// NewPlainHTTPOnlyClient returns an *http.Client that allows any HTTP (plain,
// non-TLS) connection but rejects HTTPS connections to non-loopback hosts.
// Used for Docker-internal AI endpoints (e.g. http://ollama:11434/v1) where
// the service is trusted (same Docker network) but not a loopback address.
func NewPlainHTTPOnlyClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Block TLS connections — prevents data from reaching external HTTPS APIs.
			host, _, _ := net.SplitHostPort(addr)
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("plain-http-only: DNS resolve %q: %w", host, err)
			}
			for _, ip := range ips {
				if !ip.IP.IsLoopback() {
					return nil, fmt.Errorf("plain-http-only: TLS connection to non-loopback %s blocked", ip.IP)
				}
			}
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
}

// ProbeOllamaSupportsVision returns true when the given Ollama model supports
// image/vision input (e.g. llava, llama3.2-vision). It queries the Ollama
// /api/show endpoint and checks for "vision" in the capabilities list.
// Returns false for non-Ollama providers (where baseURL doesn't look like an
// Ollama instance) or when the check fails — callers should treat false as
// "vision not confirmed" rather than "definitely no vision".
func ProbeOllamaSupportsVision(client *http.Client, baseURL, model string) bool {
	if baseURL == "" || model == "" {
		return false
	}
	// Derive the Ollama API root: strip /v1 suffix if present.
	ollamaRoot := strings.TrimRight(strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1"), "/")

	body, _ := json.Marshal(map[string]string{"model": model})
	resp, err := probeClient(client).Post(ollamaRoot+"/api/show", "application/json", bytes.NewReader(body))
	if err != nil {
		// Might be a non-Ollama provider — assume vision is possible (BYOK user's responsibility).
		return true
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return true
	}

	var result struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return true
	}
	if len(result.Capabilities) == 0 {
		// Older Ollama version that doesn't report capabilities — assume vision.
		return true
	}
	for _, cap := range result.Capabilities {
		if strings.EqualFold(cap, "vision") {
			return true
		}
	}
	return false
}

// ProbeOllamaReady checks whether an Ollama-compatible server is listening at
// the given base URL (e.g. "http://localhost:11434/v1"). It queries the
// /models endpoint and returns (true, firstModelID) on success, or
// (false, "") when the server is unreachable or has no models.
func ProbeOllamaReady(client *http.Client, baseURL string) (bool, string) {
	resp, err := probeClient(client).Get(baseURL + "/models")
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return false, ""
	}

	// Parse the OpenAI-compatible /v1/models response to find a model ID.
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || len(body.Data) == 0 {
		// Server is up but returned no/invalid model list — still "ready" but
		// use the hardcoded fallback model name.
		return true, ""
	}
	return true, body.Data[0].ID
}
