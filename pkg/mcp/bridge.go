package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// Bridge abstracts HTTP calls to the Mobazha API.
// Tool handlers call Bridge.Call() and relay the response to the AI client.
type Bridge interface {
	// Call makes an authenticated HTTP request to the Mobazha API.
	// path should start with "/" (e.g., "/v1/listings/index").
	// Returns (statusCode, responseBody, error). error is only for transport failures.
	Call(ctx context.Context, method, path string, query url.Values, body interface{}) (int, []byte, error)

	// CallMultipart sends a multipart/form-data request with a file field.
	// Used for endpoints that accept file uploads (e.g., /v1/listings/import/json).
	CallMultipart(ctx context.Context, method, path string, fieldName, fileName string, fileData []byte) (int, []byte, error)
}

// BridgeFactory creates a Bridge scoped to a specific MCP request.
// In stdio mode, it returns the same pre-configured Bridge for every call.
// In SSE mode, it creates a request-scoped Bridge from the HTTP headers.
type BridgeFactory func(req gomcp.CallToolRequest) Bridge

// StaticBridgeFactory wraps a single Bridge for stdio mode (single user).
func StaticBridgeFactory(bridge Bridge) BridgeFactory {
	return func(_ gomcp.CallToolRequest) Bridge { return bridge }
}

// SSEBridgeFactory creates per-request Bridges by extracting credentials
// from the CallToolRequest headers. Each tool call gets its own Bridge
// with the caller's token and peerID.
func SSEBridgeFactory(gatewayURL string, httpClient *http.Client) BridgeFactory {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return func(req gomcp.CallToolRequest) Bridge {
		token := extractBearerToken(req.Header)
		peerID := ""
		if req.Header != nil {
			peerID = req.Header.Get("X-Store-PeerID")
		}
		return NewHTTPBridge(gatewayURL, token, peerID, httpClient)
	}
}

func extractBearerToken(h http.Header) string {
	if h == nil {
		return ""
	}
	auth := h.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return auth
}

// HTTPBridge implements Bridge using HTTP calls to the Mobazha Gateway.
type HTTPBridge struct {
	gatewayURL string
	token      string
	peerID     string
	client     *http.Client
}

// NewHTTPBridge creates a Bridge that calls the Mobazha Gateway via HTTP.
func NewHTTPBridge(gatewayURL, token, peerID string, client *http.Client) *HTTPBridge {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPBridge{
		gatewayURL: strings.TrimRight(gatewayURL, "/"),
		token:      token,
		peerID:     peerID,
		client:     client,
	}
}

func (b *HTTPBridge) Call(ctx context.Context, method, path string, query url.Values, body interface{}) (int, []byte, error) {
	u := b.gatewayURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+b.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if b.peerID != "" {
		req.Header.Set("X-Store-PeerID", b.peerID)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

func (b *HTTPBridge) CallMultipart(ctx context.Context, method, path string, fieldName, fileName string, fileData []byte) (int, []byte, error) {
	u := b.gatewayURL + path

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return 0, nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return 0, nil, fmt.Errorf("write file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return 0, nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, &buf)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+b.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if b.peerID != "" {
		req.Header.Set("X-Store-PeerID", b.peerID)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}

	return resp.StatusCode, respBody, nil
}
