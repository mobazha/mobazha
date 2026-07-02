package electrum

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fakeElectrumServer — minimal TCP server that speaks Electrum JSON-RPC.
// Handles server.version (handshake) and server.ping (heartbeat).
// ---------------------------------------------------------------------------

type fakeElectrumServer struct {
	t        testing.TB
	listener net.Listener
	addr     string

	mu    sync.Mutex
	conns []net.Conn
}

func newFakeServer(t testing.TB) *fakeElectrumServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fakeElectrumServer: listen failed: %v", err)
	}
	s := &fakeElectrumServer{t: t, listener: l, addr: l.Addr().String()}
	go s.acceptLoop()
	return s
}

func (s *fakeElectrumServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		s.mu.Lock()
		s.conns = append(s.conns, conn)
		s.mu.Unlock()
		go s.handleConn(conn)
	}
}

func (s *fakeElectrumServer) handleConn(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		resp := Response{ID: req.ID, JSONRPC: "2.0"}
		switch req.Method {
		case MethodServerVersion:
			resp.Result, _ = json.Marshal([]string{"FakeElectrum/1.0", "1.4"})
		case MethodServerPing:
			resp.Result = json.RawMessage("null")
		default:
			resp.Error = &RPCError{Code: -32601, Message: "method not found"}
		}
		data, _ := json.Marshal(resp)
		data = append(data, '\n')
		conn.Write(data)
	}
}

func (s *fakeElectrumServer) close() {
	s.listener.Close()
	s.mu.Lock()
	for _, c := range s.conns {
		c.Close()
	}
	s.conns = nil
	s.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestClient(server string) *Client {
	return &Client{
		servers:        []string{server},
		pending:        make(map[uint64]chan *Response),
		subscriptions:  make(map[string]func(params []interface{})),
		shutdown:       make(chan struct{}),
		reconnectDelay: 100 * time.Millisecond,
		timeout:        5 * time.Second,
		useTLS:         false,
		chain:          "TEST",
	}
}

func TestReconnectWithoutEndpointsReturns(t *testing.T) {
	client := &Client{
		shutdown: make(chan struct{}),
		chain:    "TEST",
	}
	done := make(chan struct{})
	go func() {
		client.reconnect()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reconnect must not spin forever without configured endpoints")
	}
}

// ---------------------------------------------------------------------------
// Test 1: readLoop uses its own reader parameter, not Client shared state.
//
// Core correctness: readLoop receives a dedicated bufio.Reader and dispatches
// server messages through it, completely independent of any Client field.
// ---------------------------------------------------------------------------

func TestReadLoop_OwnReader_NotAffectedByFieldReplacement(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer l.Close()

	var srvConn net.Conn
	accepted := make(chan struct{})
	go func() {
		srvConn, _ = l.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	<-accepted

	client := &Client{
		pending:  make(map[uint64]chan *Response),
		shutdown: make(chan struct{}),
		chain:    "TEST",
	}
	t.Cleanup(func() { _ = client.Close() })
	client.conn = clientConn
	client.connected = true

	ownReader := bufio.NewReader(clientConn)
	done := make(chan struct{})

	respCh := make(chan *Response, 1)
	client.mu.Lock()
	client.pending[42] = respCh
	client.mu.Unlock()

	go client.readLoop(clientConn, ownReader, done)

	// Server sends response for request ID 42.
	srvConn.Write([]byte(`{"jsonrpc":"2.0","id":42,"result":"hello"}` + "\n"))

	select {
	case r := <-respCh:
		if r == nil {
			t.Fatal("received nil response")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readLoop did not dispatch response — may be using wrong reader")
	}

	srvConn.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("readLoop did not exit after server conn closed")
	}
}

// ---------------------------------------------------------------------------
// Test 2: tryHandshake waits for previous readLoop to exit before starting
// a new one, and proceeds once the old readLoop signals done.
// ---------------------------------------------------------------------------

func TestTryHandshake_PreviousReadLoopRunning_WaitsForExit(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.close()

	client := newTestClient(srv.addr)

	// Pretend a previous readLoop is still running.
	prevDone := make(chan struct{})
	client.mu.Lock()
	client.readLoopDone = prevDone
	client.mu.Unlock()

	conn, err := net.Dial("tcp", srv.addr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result bool
	handshakeDone := make(chan struct{})
	go func() {
		result = client.tryHandshake(ctx, conn, 0)
		close(handshakeDone)
	}()

	// Should still be blocking after 300 ms.
	select {
	case <-handshakeDone:
		t.Fatal("tryHandshake should block while previous readLoop is running")
	case <-time.After(300 * time.Millisecond):
	}

	// Release — tryHandshake should complete.
	close(prevDone)

	select {
	case <-handshakeDone:
		if !result {
			t.Fatal("tryHandshake should succeed after previous readLoop exits")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tryHandshake did not complete after prevDone closed")
	}

	client.Close()
}

// ---------------------------------------------------------------------------
// Test 3: tryHandshake respects context cancellation instead of blocking
// forever when the previous readLoop is stuck.
// ---------------------------------------------------------------------------

func TestTryHandshake_ContextCancelled_ReturnsFalse(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.close()

	client := newTestClient(srv.addr)

	// Never-closing done simulates a stuck readLoop.
	client.mu.Lock()
	client.readLoopDone = make(chan struct{})
	client.mu.Unlock()

	conn, err := net.Dial("tcp", srv.addr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	result := client.tryHandshake(ctx, conn, 0)

	if result {
		t.Error("tryHandshake should return false when context is cancelled")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v, should have returned near 200 ms", d)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Concurrent Ping — race detector validation.
//
// 5 goroutines Ping concurrently while readLoop reads from the same conn.
// Before the fix, concurrent access to c.reader would be detected as a race.
// After: readLoop owns its reader, call() writes under lock. No race.
//
// Run with:  go test -race -run TestClient_ConcurrentPing_NoRace
// ---------------------------------------------------------------------------

func TestClient_ConcurrentPing_NoRace(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.close()

	client := newTestClient(srv.addr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	var wg sync.WaitGroup
	var pingOK, pingErr int64

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				pCtx, pCancel := context.WithTimeout(ctx, time.Second)
				if err := client.Ping(pCtx); err != nil {
					atomic.AddInt64(&pingErr, 1)
				} else {
					atomic.AddInt64(&pingOK, 1)
				}
				pCancel()
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	ok := atomic.LoadInt64(&pingOK)
	t.Logf("Ping results: ok=%d err=%d", ok, atomic.LoadInt64(&pingErr))

	if ok == 0 {
		t.Error("expected at least some successful Pings")
	}
}
