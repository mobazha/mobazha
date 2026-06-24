package api

import (
	"net"
	"net/http"
	"strings"
)

// publicRequestOrigin returns the public origin (scheme + host) for the
// incoming request. It is intended for generating URLs that leave the node,
// such as provider webhooks or AI image fetch URLs.
func publicRequestOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := "https"
	if proto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = strings.ToLower(proto)
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	if fwd := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); fwd != "" {
		host = fwd
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

// allowLoopbackGatewayForRequest returns true only for direct local-dev
// requests. Forwarded requests must not enable the loopback media fetch bypass,
// because forwarded host/proto headers can be spoofed when a proxy is misconfigured.
func allowLoopbackGatewayForRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if hasForwardedHeader(r) {
		return false
	}
	if !isLoopbackHost(hostnameOnly(r.Host)) {
		return false
	}
	return isLoopbackHost(hostnameOnly(r.RemoteAddr))
}

func hasForwardedHeader(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("Forwarded")) != "" ||
		strings.TrimSpace(r.Header.Get("X-Forwarded-For")) != "" ||
		strings.TrimSpace(r.Header.Get("X-Forwarded-Host")) != "" ||
		strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")) != ""
}

func firstHeaderValue(value string) string {
	return strings.TrimSpace(strings.Split(value, ",")[0])
}

func hostnameOnly(value string) string {
	value = firstHeaderValue(value)
	if value == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(value, "[]")
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
