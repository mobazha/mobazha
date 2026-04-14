package net

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	inet "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	protocol "github.com/libp2p/go-libp2p/core/protocol"
)

const (
	httpProxyRequestTimeout  = 30 * time.Second
	httpProxyMaxRequestBytes = 10 << 20 // 10 MB
)

// HTTPProxyHandler handles incoming libp2p stream requests and proxies them
// to the local HTTP API. Only streams from trusted peers (SaaS default node)
// are accepted; all others are rejected immediately.
//
// Trust chain:
//  1. trustedPeers whitelist (this handler)
//  2. Local AuthenticationMiddleware validates JWT on write operations
type HTTPProxyHandler struct {
	trustedPeers map[peer.ID]bool
	localAPIAddr string // derived from GatewayAddr, e.g. "http://127.0.0.1:5102"
	client       *http.Client
}

// NewHTTPProxyHandler creates a handler that proxies libp2p streams to
// the local API server. Only peers in trustedPeers are allowed to connect.
func NewHTTPProxyHandler(trustedPeers []peer.ID, localAPIAddr string) *HTTPProxyHandler {
	trusted := make(map[peer.ID]bool, len(trustedPeers))
	for _, p := range trustedPeers {
		trusted[p] = true
	}
	return &HTTPProxyHandler{
		trustedPeers: trusted,
		localAPIAddr: localAPIAddr,
		client: &http.Client{
			Timeout: httpProxyRequestTimeout,
		},
	}
}

// RegisterOnHost registers the HTTP proxy stream handler on the given libp2p host.
func (h *HTTPProxyHandler) RegisterOnHost(lhost host.Host, protocolID protocol.ID) {
	lhost.SetStreamHandler(protocolID, h.handleStream)
}

// RegisterHTTPProxyOnHost creates and registers the handler on the given libp2p host.
func RegisterHTTPProxyOnHost(lhost host.Host, protocolID protocol.ID, trustedPeers []peer.ID, localAPIAddr string) *HTTPProxyHandler {
	handler := NewHTTPProxyHandler(trustedPeers, localAPIAddr)
	handler.RegisterOnHost(lhost, protocolID)
	return handler
}

func (h *HTTPProxyHandler) handleStream(stream inet.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()
	if !h.trustedPeers[remotePeer] {
		log.Warningf("http-proxy: rejected stream from untrusted peer %s", remotePeer)
		return
	}

	h.proxyRequest(stream, stream, remotePeer)
}

// proxyRequest reads an HTTP request from reader, proxies it to the local API,
// and writes the response back to writer. This is the core logic extracted for
// testability — the stream handler delegates here.
func (h *HTTPProxyHandler) proxyRequest(reader io.Reader, writer io.Writer, remotePeer peer.ID) {
	bufReader := bufio.NewReader(io.LimitReader(reader, httpProxyMaxRequestBytes))
	req, err := http.ReadRequest(bufReader)
	if err != nil {
		log.Warningf("http-proxy: failed to parse request from %s: %v", remotePeer, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), httpProxyRequestTimeout)
	defer cancel()

	targetURL := h.localAPIAddr + req.URL.RequestURI()
	proxyReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, req.Body)
	if err != nil {
		log.Warningf("http-proxy: failed to create proxy request: %v", err)
		writeHTTPError(writer, http.StatusInternalServerError, "proxy request failed")
		return
	}

	for key, values := range req.Header {
		for _, v := range values {
			proxyReq.Header.Add(key, v)
		}
	}
	proxyReq.Header.Set("X-Forwarded-Via", "libp2p")
	proxyReq.Header.Set("X-Forwarded-Peer", remotePeer.String())

	resp, err := h.client.Do(proxyReq)
	if err != nil {
		log.Warningf("http-proxy: local API request failed: %v", err)
		writeHTTPError(writer, http.StatusBadGateway, "local API unreachable")
		return
	}
	defer resp.Body.Close()

	if err := resp.Write(writer); err != nil {
		log.Warningf("http-proxy: failed to write response: %v", err)
	}
}

func writeHTTPError(w io.Writer, statusCode int, message string) {
	body := fmt.Sprintf(`{"error":{"code":"PROXY_ERROR","message":"%s"}}`, message)
	resp := &http.Response{
		StatusCode:    statusCode,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	resp.Header.Set("Content-Type", "application/json")
	resp.Write(w)
}
