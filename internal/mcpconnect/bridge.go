package mcpconnect

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// BridgeOpts configures the stdio-to-MCP bridge.
// The bridge uses SSE-style GET streaming + POST for JSON-RPC messages,
// which is compatible with both legacy SSE servers and Streamable HTTP
// servers (mcp-go's StreamableHTTPServer serves GET as SSE stream).
type BridgeOpts struct {
	SSEURL string // MCP endpoint URL (supports both SSE and Streamable HTTP GET)
	Token  string

	Stdin  io.Reader // defaults to os.Stdin
	Stdout io.Writer // defaults to os.Stdout
	Stderr io.Writer // defaults to os.Stderr

	MaxRetries    int
	RetryInterval time.Duration
}

func (o *BridgeOpts) defaults() {
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.MaxRetries == 0 {
		o.MaxRetries = 30
	}
	if o.RetryInterval == 0 {
		o.RetryInterval = 2 * time.Second
	}
}

// RunBridge starts a bidirectional bridge between stdio and MCP SSE.
//
// Flow:
//  1. Connect to the SSE endpoint and read the session endpoint URL
//  2. Forward SSE events to stdout (one JSON-RPC message per line)
//  3. Read JSON-RPC messages from stdin and POST them to the session endpoint
func RunBridge(ctx context.Context, opts BridgeOpts) error {
	opts.defaults()

	sessionEndpoint, sseReader, err := connectSSEWithRetry(ctx, opts)
	if err != nil {
		return fmt.Errorf("SSE connection failed: %w", err)
	}

	fmt.Fprintf(opts.Stderr, "bridge: connected to %s\n", opts.SSEURL)
	fmt.Fprintf(opts.Stderr, "bridge: session endpoint: %s\n", sessionEndpoint)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		if err := forwardSSEToStdout(ctx, sseReader, opts.Stdout); err != nil {
			fmt.Fprintf(opts.Stderr, "bridge: SSE read error: %v\n", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		if err := forwardStdinToHTTP(ctx, opts.Stdin, sessionEndpoint, opts.Token); err != nil {
			fmt.Fprintf(opts.Stderr, "bridge: stdin read error: %v\n", err)
		}
	}()

	wg.Wait()
	return nil
}

// connectSSEWithRetry connects to the SSE endpoint with wait-for-node retry logic.
func connectSSEWithRetry(ctx context.Context, opts BridgeOpts) (string, io.ReadCloser, error) {
	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(opts.Stderr, "bridge: retrying connection (attempt %d/%d)...\n", attempt, opts.MaxRetries)
			select {
			case <-ctx.Done():
				return "", nil, ctx.Err()
			case <-time.After(opts.RetryInterval):
			}
		}

		sessionURL, body, err := connectSSE(ctx, opts.SSEURL, opts.Token)
		if err == nil {
			return sessionURL, body, nil
		}
		lastErr = err
	}
	return "", nil, fmt.Errorf("failed after %d attempts: %w", opts.MaxRetries, lastErr)
}

// connectSSE establishes an SSE connection and reads the endpoint event
// to discover the session POST URL.
func connectSSE(ctx context.Context, sseURL, token string) (string, io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return "", nil, fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	sessionURL := ""
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: endpoint") {
			if scanner.Scan() {
				dataLine := scanner.Text()
				if strings.HasPrefix(dataLine, "data: ") {
					endpoint := strings.TrimPrefix(dataLine, "data: ")
					sessionURL = resolveSessionURL(sseURL, endpoint)
					break
				}
			}
		}
	}

	if sessionURL == "" {
		resp.Body.Close()
		return "", nil, fmt.Errorf("no endpoint event received from SSE")
	}

	return sessionURL, resp.Body, nil
}

// resolveSessionURL turns a relative session path into an absolute URL
// based on the SSE URL's scheme and host.
func resolveSessionURL(sseURL, endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}

	idx := strings.Index(sseURL, "//")
	if idx < 0 {
		return endpoint
	}
	slashIdx := strings.Index(sseURL[idx+2:], "/")
	if slashIdx < 0 {
		return sseURL + endpoint
	}
	base := sseURL[:idx+2+slashIdx]
	return base + endpoint
}

// forwardSSEToStdout reads SSE events and writes JSON-RPC messages to stdout.
func forwardSSEToStdout(ctx context.Context, body io.ReadCloser, stdout io.Writer) error {
	defer body.Close()
	scanner := bufio.NewScanner(body)

	const maxBufSize = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxBufSize)

	currentEvent := ""
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") && currentEvent == "message" {
			data := strings.TrimPrefix(line, "data: ")
			if json.Valid([]byte(data)) {
				fmt.Fprintln(stdout, data)
			}
		}

		if line == "" {
			currentEvent = ""
		}
	}
	return scanner.Err()
}

// forwardStdinToHTTP reads JSON-RPC messages from stdin and POSTs them
// to the session endpoint.
func forwardStdinToHTTP(ctx context.Context, stdin io.Reader, sessionURL, token string) error {
	scanner := bufio.NewScanner(stdin)
	const maxBufSize = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxBufSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if !json.Valid([]byte(line)) {
			continue
		}

		req, err := http.NewRequestWithContext(ctx, "POST", sessionURL, strings.NewReader(line))
		if err != nil {
			return fmt.Errorf("creating POST request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("posting to session: %w", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return scanner.Err()
}
