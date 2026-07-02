package electrum

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/logging"
	"github.com/mobazha/mobazha3.0/pkg/redact"
)

var log = logging.MustGetLogger("electrum")

const defaultReconnectDelay = 5 * time.Second

// Client represents an Electrum protocol client
type Client struct {
	servers        []string
	currentServer  int
	conn           net.Conn
	writer         *bufio.Writer
	mu             sync.Mutex
	idGen          RequestIDGenerator
	pending        map[uint64]chan *Response
	subscriptions  map[string]func(params []interface{})
	subMu          sync.RWMutex
	connected      bool
	connecting     bool // true during initial connection, prevents auto-reconnect
	shutdown       chan struct{}
	heartbeatStop  chan struct{} // channel to stop heartbeat goroutine
	readLoopDone   chan struct{} // closed when the current readLoop goroutine exits
	reconnectDelay time.Duration
	timeout        time.Duration
	useTLS         bool
	tlsConfig      *tls.Config
	chain          string
}

// ClientConfig holds configuration for the Electrum client
type ClientConfig struct {
	Servers        []string      // List of servers to connect to
	Timeout        time.Duration // Connection and read timeout
	ReconnectDelay time.Duration // Delay between reconnection attempts
	UseTLS         bool          // Whether to use TLS
	TLSConfig      *tls.Config   // Optional custom TLS config (e.g. for cert pinning)
	Chain          string        // Chain identifier (BTC, LTC, etc.)
	Testnet        bool          // Whether to use testnet servers
}

// DefaultClientConfig returns a default client configuration
// Set testnet=true for testnet servers, false for mainnet
func DefaultClientConfig(chain string, testnet bool) *ClientConfig {
	return &ClientConfig{
		Servers:        GetDefaultServers(chain, testnet),
		Timeout:        30 * time.Second,
		ReconnectDelay: 5 * time.Second,
		UseTLS:         true,
		Chain:          chain,
		Testnet:        testnet,
	}
}

// NewClient creates a new Electrum client
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = DefaultClientConfig("BTC", false)
	}

	reconnect := config.ReconnectDelay
	if reconnect <= 0 {
		reconnect = defaultReconnectDelay
	}

	return &Client{
		servers:        config.Servers,
		currentServer:  0,
		pending:        make(map[uint64]chan *Response),
		subscriptions:  make(map[string]func(params []interface{})),
		shutdown:       make(chan struct{}),
		reconnectDelay: reconnect,
		timeout:        config.Timeout,
		useTLS:         config.UseTLS,
		tlsConfig:      config.TLSConfig,
		chain:          config.Chain,
	}
}

// Connect establishes connection to an Electrum server
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.connecting = true
	c.mu.Unlock()

	// Release lock during connection attempts
	err := c.connectWithoutLock(ctx)

	c.mu.Lock()
	c.connecting = false
	c.mu.Unlock()

	return err
}

func (c *Client) connectWithoutLock(ctx context.Context) error {

	// Try servers in parallel, use first one that completes handshake
	type connectResult struct {
		serverIdx int
		conn      net.Conn
		err       error
	}

	resultCh := make(chan connectResult, len(c.servers))

	// Use shorter timeout for individual connection attempts
	dialTimeout := c.timeout
	if dialTimeout > 5*time.Second {
		dialTimeout = 5 * time.Second
	}

	// Start parallel connection attempts
	for i := 0; i < len(c.servers); i++ {
		serverIdx := (c.currentServer + i) % len(c.servers)
		go func(idx int, server string) {
			// Create per-connection timeout context
			dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
			defer cancel()

			conn, err := c.dialServer(dialCtx, server)
			resultCh <- connectResult{serverIdx: idx, conn: conn, err: err}
		}(serverIdx, c.servers[serverIdx])
	}

	// Collect results. Per-endpoint failures are accumulated and emitted as a
	// single aggregated warning at the end — logging one warning per failed
	// server creates noise that dwarfs the eventual success line and floods
	// logs when the first few hosts in a public endpoint list are stale.
	var failures []endpointFailure
	received := 0

	for received < len(c.servers) {
		select {
		case result := <-resultCh:
			received++

			if result.err != nil {
				failures = append(failures, endpointFailure{
					server: redact.ServerAddr(c.servers[result.serverIdx]),
					err:    result.err,
				})
				continue
			}

			// Got a connection, try handshake
			if c.tryHandshake(ctx, result.conn, result.serverIdx) {
				if len(failures) > 0 {
					// At least one endpoint was unhealthy, but we recovered.
					// Emit a single info-level summary so operators can spot
					// flaky endpoints without flooding warning logs.
					log.Infof("[%s] Connected to Electrum after %d endpoint failure(s): %s",
						c.chain, len(failures), formatEndpointFailures(failures))
				}
				// Close any remaining connections that come in
				remaining := len(c.servers) - received
				go func(toClose int) {
					for i := 0; i < toClose; i++ {
						r := <-resultCh
						if r.conn != nil {
							r.conn.Close()
						}
					}
				}(remaining)
				return nil
			}

			// Handshake failed, close connection
			result.conn.Close()
			failures = append(failures, endpointFailure{
				server: redact.ServerAddr(c.servers[result.serverIdx]),
				err:    fmt.Errorf("handshake failed"),
			})

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// All endpoints failed — emit one aggregated warning instead of N.
	log.Warningf("[%s] Failed to connect to all %d Electrum endpoints: %s",
		c.chain, len(failures), formatEndpointFailures(failures))
	if len(failures) == 0 {
		return fmt.Errorf("no Electrum endpoints configured")
	}
	return fmt.Errorf("failed to connect to any Electrum server (%d endpoints): %w",
		len(failures), failures[len(failures)-1].err)
}

// endpointFailure carries one server's connect/handshake failure for the
// aggregated log emitted by connectWithoutLock.
type endpointFailure struct {
	server string
	err    error
}

// formatEndpointFailures renders a compact "server=err; server=err" summary,
// truncating after a few entries to keep log lines bounded.
func formatEndpointFailures(failures []endpointFailure) string {
	const maxShown = 3
	parts := make([]string, 0, len(failures))
	for i, f := range failures {
		if i >= maxShown {
			parts = append(parts, fmt.Sprintf("(+%d more)", len(failures)-maxShown))
			break
		}
		parts = append(parts, fmt.Sprintf("%s=%v", f.server, f.err))
	}
	return strings.Join(parts, "; ")
}

// tryHandshake attempts to complete the Electrum handshake on a connection
func (c *Client) tryHandshake(ctx context.Context, conn net.Conn, serverIdx int) bool {
	server := c.servers[serverIdx]

	// Wait for previous readLoop to exit before starting a new one.
	// This prevents two readLoop goroutines from co-existing and
	// potentially racing on the shared bufio.Reader.
	// Respect the caller's context to avoid blocking indefinitely if the
	// previous readLoop is stuck waiting for a read deadline (up to 30s).
	c.mu.Lock()
	prevDone := c.readLoopDone
	c.mu.Unlock()
	if prevDone != nil {
		select {
		case <-prevDone:
		case <-ctx.Done():
			return false
		}
	}

	// Setup connection
	c.mu.Lock()
	reader := bufio.NewReader(conn)
	c.conn = conn
	c.writer = bufio.NewWriter(conn)
	c.currentServer = serverIdx
	c.connected = true
	c.heartbeatStop = make(chan struct{})
	done := make(chan struct{})
	c.readLoopDone = done
	c.mu.Unlock()

	// Start reading responses — each readLoop goroutine owns its own conn and
	// reader, eliminating data races between concurrent readLoop/reconnect.
	go c.readLoop(conn, reader, done)

	// Perform handshake with timeout
	handshakeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := c.serverVersion(handshakeCtx); err != nil {
		log.Warningf("[%s] Handshake failed with %s: %v", c.chain, redact.ServerAddr(server), err)
		c.mu.Lock()
		c.closeConnection()
		c.mu.Unlock()
		return false
	}

	// Start heartbeat to keep connection alive.
	// Pass heartbeatStop as parameter so the goroutine owns its own reference,
	// avoiding a data race with closeConnection() which sets c.heartbeatStop = nil.
	c.mu.Lock()
	hbStop := c.heartbeatStop
	c.mu.Unlock()
	go c.heartbeatLoop(hbStop)

	log.Infof("[%s] Connected to Electrum server: %s", c.chain, redact.ServerAddr(server))
	return true
}

// heartbeatLoop sends periodic ping requests to keep the connection alive.
// It takes its own stop channel as a parameter to avoid a data race on
// c.heartbeatStop with closeConnection() which sets the field to nil.
func (c *Client) heartbeatLoop(stop <-chan struct{}) {
	// Ping interval should be less than read timeout (30s)
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdown:
			return
		case <-stop:
			return
		case <-ticker.C:
			c.mu.Lock()
			connected := c.connected
			c.mu.Unlock()

			if !connected {
				return
			}

			// Send ping with short timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := c.Ping(ctx); err != nil {
				log.Debugf("[%s] Heartbeat ping failed: %v", c.chain, err)
			}
			cancel()
		}
	}
}

// dialServer attempts to establish a TCP connection to the server
func (c *Client) dialServer(ctx context.Context, server string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: c.timeout}

	if c.useTLS {
		host := server
		if idx := strings.LastIndex(server, ":"); idx > 0 {
			host = server[:idx]
		}

		tlsConfig := c.tlsConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{
				ServerName: host,
				MinVersion: tls.VersionTLS12,
			}
		} else {
			tlsConfig = tlsConfig.Clone()
			if tlsConfig.ServerName == "" {
				tlsConfig.ServerName = host
			}
		}

		conn, err := tls.DialWithDialer(dialer, "tcp", server, tlsConfig)
		if err != nil {
			return nil, err
		}
		// Verify connection is valid
		if conn == nil {
			return nil, fmt.Errorf("nil connection returned")
		}
		return conn, nil
	}

	return dialer.DialContext(ctx, "tcp", server)
}

func (c *Client) closeConnection() {
	// Stop heartbeat goroutine
	if c.heartbeatStop != nil {
		select {
		case <-c.heartbeatStop:
			// Already closed
		default:
			close(c.heartbeatStop)
		}
		c.heartbeatStop = nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.shutdown)
	c.closeConnection()

	// Cancel all pending requests
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = make(map[uint64]chan *Response)

	return nil
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// readLoop reads messages from the Electrum server connection.
//
// Each goroutine owns its own conn and reader to avoid sharing mutable state.
// When reconnect() establishes a new connection, the old readLoop continues
// using its own (now-closed) conn/reader until ReadBytes returns an error,
// then exits cleanly. The done channel is closed on exit so that the next
// tryHandshake can wait for this goroutine to fully terminate.
func (c *Client) readLoop(conn net.Conn, reader *bufio.Reader, done chan struct{}) {
	defer close(done) // signal that this readLoop has fully exited

	for {
		select {
		case <-c.shutdown:
			return
		default:
		}

		// Use reasonable read timeout - will be reset on each successful read.
		// We set the deadline on our own conn reference (not c.conn) so that
		// reconnect replacing c.conn does not affect us.
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		line, err := reader.ReadBytes('\n')
		if err != nil {
			// Check if shutdown was requested
			select {
			case <-c.shutdown:
				return
			default:
			}

			if err == io.EOF || errors.Is(err, net.ErrClosed) {
				log.Debugf("[%s] Connection closed", c.chain)
			} else if !strings.Contains(err.Error(), "use of closed") {
				log.Debugf("[%s] Read error: %v", c.chain, err)
			}
			c.handleDisconnect()
			return
		}

		c.handleMessage(line)
	}
}

func (c *Client) handleMessage(data []byte) {
	// Try to parse as response first
	var resp Response
	if err := json.Unmarshal(data, &resp); err == nil && resp.ID > 0 {
		c.mu.Lock()
		if ch, ok := c.pending[resp.ID]; ok {
			ch <- &resp
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()
		return
	}

	// Try to parse as notification
	var notif Notification
	if err := json.Unmarshal(data, &notif); err == nil && notif.Method != "" {
		c.handleNotification(&notif)
	}
}

func (c *Client) handleNotification(notif *Notification) {
	c.subMu.RLock()
	defer c.subMu.RUnlock()

	// For scripthash.subscribe, the first param is the scripthash
	if notif.Method == MethodScripthashSubscribe && len(notif.Params) > 0 {
		if scripthash, ok := notif.Params[0].(string); ok {
			if callback, ok := c.subscriptions[scripthash]; ok {
				go callback(notif.Params)
			}
		}
	}
}

func (c *Client) handleDisconnect() {
	c.mu.Lock()
	c.connected = false
	// Don't reconnect if we're still in initial connection phase
	if c.connecting {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	// Only reconnect if not shutting down
	select {
	case <-c.shutdown:
		return
	default:
		go c.reconnect()
	}
}

func (c *Client) reconnect() {
	const maxDelay = 2 * time.Minute
	if len(c.servers) == 0 {
		log.Warningf("[%s] Automatic reconnect disabled: no Electrum endpoints configured", c.chain)
		return
	}
	delay := c.reconnectDelay
	if delay <= 0 {
		delay = defaultReconnectDelay
	}

	for attempt := 1; ; attempt++ {
		select {
		case <-c.shutdown:
			return
		case <-time.After(delay):
		}

		c.mu.Lock()
		if c.connected {
			c.mu.Unlock()
			return
		}
		c.connecting = true
		c.closeConnection()
		c.mu.Unlock()

		log.Infof("[%s] Reconnect attempt %d (backoff %v)...", c.chain, attempt, delay)
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		err := c.connectWithoutLock(ctx)
		cancel()

		c.mu.Lock()
		c.connecting = false
		c.mu.Unlock()

		if err == nil {
			log.Infof("[%s] Reconnected successfully after %d attempt(s)", c.chain, attempt)
			c.resubscribeAll()
			return
		}

		log.Warningf("[%s] Reconnect attempt %d failed: %v", c.chain, attempt, err)

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

func (c *Client) resubscribeAll() {
	c.subMu.RLock()
	scripthashes := make([]string, 0, len(c.subscriptions))
	for sh := range c.subscriptions {
		scripthashes = append(scripthashes, sh)
	}
	c.subMu.RUnlock()

	ctx := context.Background()
	for _, sh := range scripthashes {
		if _, err := c.call(ctx, MethodScripthashSubscribe, sh); err != nil {
			log.Warningf("[%s] Failed to resubscribe to %s: %v", c.chain, sh, err)
		}
	}
}

func (c *Client) call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	id := c.idGen.Next()
	req := NewRequest(id, method, params...)

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	respCh := make(chan *Response, 1)

	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, errors.New("not connected")
	}
	c.pending[id] = respCh

	_, err = c.writer.Write(data)
	if err != nil {
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("write error: %w", err)
	}
	err = c.writer.Flush()
	if err != nil {
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("flush error: %w", err)
	}
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case resp, ok := <-respCh:
		if !ok {
			return nil, errors.New("connection closed")
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

// serverVersion performs the version handshake
func (c *Client) serverVersion(ctx context.Context) (*ServerVersion, error) {
	result, err := c.call(ctx, MethodServerVersion, "mobazha", "1.4")
	if err != nil {
		return nil, err
	}

	var versions []string
	if err := json.Unmarshal(result, &versions); err != nil {
		return nil, err
	}

	if len(versions) < 2 {
		return nil, errors.New("invalid server version response")
	}

	return &ServerVersion{
		ServerSoftware: versions[0],
		ProtocolMax:    versions[1],
	}, nil
}

// Ping sends a ping to the server
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.call(ctx, MethodServerPing)
	return err
}

// GetBalance returns the balance for a scripthash
func (c *Client) GetBalance(ctx context.Context, scripthash string) (*ScriptHashBalance, error) {
	result, err := c.call(ctx, MethodScripthashBalance, scripthash)
	if err != nil {
		return nil, err
	}

	var balance ScriptHashBalance
	if err := json.Unmarshal(result, &balance); err != nil {
		return nil, err
	}

	return &balance, nil
}

// GetHistory returns the transaction history for a scripthash
func (c *Client) GetHistory(ctx context.Context, scripthash string) ([]ScriptHashHistory, error) {
	result, err := c.call(ctx, MethodScripthashHistory, scripthash)
	if err != nil {
		return nil, err
	}

	var history []ScriptHashHistory
	if err := json.Unmarshal(result, &history); err != nil {
		return nil, err
	}

	return history, nil
}

// GetMempool returns unconfirmed transactions for a scripthash
func (c *Client) GetMempool(ctx context.Context, scripthash string) ([]ScriptHashHistory, error) {
	result, err := c.call(ctx, MethodScripthashMempool, scripthash)
	if err != nil {
		return nil, err
	}

	var mempool []ScriptHashHistory
	if err := json.Unmarshal(result, &mempool); err != nil {
		return nil, err
	}

	return mempool, nil
}

// ListUnspent returns unspent outputs for a scripthash
func (c *Client) ListUnspent(ctx context.Context, scripthash string) ([]ScriptHashUnspent, error) {
	result, err := c.call(ctx, MethodScripthashListunspent, scripthash)
	if err != nil {
		return nil, err
	}

	var unspent []ScriptHashUnspent
	if err := json.Unmarshal(result, &unspent); err != nil {
		return nil, err
	}

	return unspent, nil
}

// Subscribe subscribes to notifications for a scripthash
func (c *Client) Subscribe(ctx context.Context, scripthash string, callback func(params []interface{})) error {
	// First register the callback
	c.subMu.Lock()
	c.subscriptions[scripthash] = callback
	c.subMu.Unlock()

	// Then send the subscribe request
	_, err := c.call(ctx, MethodScripthashSubscribe, scripthash)
	if err != nil {
		c.subMu.Lock()
		delete(c.subscriptions, scripthash)
		c.subMu.Unlock()
		return err
	}

	return nil
}

// Unsubscribe unsubscribes from notifications for a scripthash
func (c *Client) Unsubscribe(ctx context.Context, scripthash string) error {
	c.subMu.Lock()
	delete(c.subscriptions, scripthash)
	c.subMu.Unlock()

	_, err := c.call(ctx, MethodScripthashUnsubscribe, scripthash)
	return err
}

// GetTransaction returns a transaction by its hash
func (c *Client) GetTransaction(ctx context.Context, txhash string, verbose bool) (*TransactionInfo, error) {
	result, err := c.call(ctx, MethodTransactionGet, txhash, verbose)
	if err != nil {
		return nil, err
	}

	if !verbose {
		var hex string
		if err := json.Unmarshal(result, &hex); err != nil {
			return nil, err
		}
		return &TransactionInfo{Hex: hex}, nil
	}

	var tx TransactionInfo
	if err := json.Unmarshal(result, &tx); err != nil {
		return nil, err
	}

	return &tx, nil
}

// BroadcastTransaction broadcasts a raw transaction
func (c *Client) BroadcastTransaction(ctx context.Context, rawTx string) (string, error) {
	result, err := c.call(ctx, MethodTransactionBroadcast, rawTx)
	if err != nil {
		return "", err
	}

	var txhash string
	if err := json.Unmarshal(result, &txhash); err != nil {
		return "", err
	}

	return txhash, nil
}

// EstimateFee estimates the fee for a transaction
func (c *Client) EstimateFee(ctx context.Context, numBlocks int) (float64, error) {
	result, err := c.call(ctx, MethodBlockchainEstimatefee, numBlocks)
	if err != nil {
		return 0, err
	}

	var fee float64
	if err := json.Unmarshal(result, &fee); err != nil {
		return 0, err
	}

	return fee, nil
}

// GetRelayFee returns the minimum relay fee
func (c *Client) GetRelayFee(ctx context.Context) (float64, error) {
	result, err := c.call(ctx, MethodBlockchainRelayfee)
	if err != nil {
		return 0, err
	}

	var fee float64
	if err := json.Unmarshal(result, &fee); err != nil {
		return 0, err
	}

	return fee, nil
}
