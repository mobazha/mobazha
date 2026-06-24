package stream

import (
	"context"
	"sync"
)

// Chunk represents an incremental output fragment from the LLM.
type Chunk struct {
	Delta      string     `json:"delta,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolEvent  *ToolEvent `json:"tool_event,omitempty"`
	FinishFlag string     `json:"finish_flag,omitempty"`
	Error      error      `json:"-"`
}

// ToolCall represents a single tool invocation requested by the model.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolEvent is a redacted progress event for UI transparency. It deliberately
// excludes tool arguments and results.
type ToolEvent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Stream is a pull-based iterator over incremental chunks.
// Callers use Next() in a loop; a nil return signals completion.
// The associated context controls cancellation.
type Stream interface {
	// Next blocks until the next chunk is available.
	// Returns nil when the stream is exhausted or context cancelled.
	Next() *Chunk
	// Err returns the first non-EOF error encountered, if any.
	Err() error
	// Close releases resources. ManagedEscrow to call multiple times.
	Close()
}

// Buffered is a concrete Stream backed by a channel.
// Producers call Send/SendError/Finish; consumers call Next.
//
// The data channel (ch) is never closed. Completion is signaled via a
// separate done channel, eliminating the send-on-closed-channel race
// between concurrent Send and Finish calls.
type Buffered struct {
	ch     chan Chunk
	done   chan struct{} // closed by Finish — unblocks Send and signals Next
	ctx    context.Context
	cancel context.CancelFunc

	once   sync.Once
	mu     sync.Mutex
	err    error
	closed bool
}

// NewBuffered creates a stream with the given buffer size and parent context.
func NewBuffered(ctx context.Context, bufSize int) *Buffered {
	if bufSize < 1 {
		bufSize = 16
	}
	childCtx, cancel := context.WithCancel(ctx)
	return &Buffered{
		ch:     make(chan Chunk, bufSize),
		done:   make(chan struct{}),
		ctx:    childCtx,
		cancel: cancel,
	}
}

// Send enqueues a chunk. Blocks if buffer is full. No-op after Finish.
func (s *Buffered) Send(c Chunk) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	select {
	case s.ch <- c:
	case <-s.done:
	case <-s.ctx.Done():
	}
}

// SendError enqueues an error chunk and finishes the stream.
func (s *Buffered) SendError(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
	s.Send(Chunk{Error: err})
	s.Finish()
}

// Finish signals no more data. ManagedEscrow to call multiple times.
func (s *Buffered) Finish() {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.done)
	})
}

// Next implements Stream. Returns nil when done.
func (s *Buffered) Next() *Chunk {
	select {
	case c := <-s.ch:
		if c.Error != nil {
			s.mu.Lock()
			if s.err == nil {
				s.err = c.Error
			}
			s.mu.Unlock()
		}
		return &c
	case <-s.done:
		// Producer finished — drain any remaining buffered chunks.
		select {
		case c := <-s.ch:
			if c.Error != nil {
				s.mu.Lock()
				if s.err == nil {
					s.err = c.Error
				}
				s.mu.Unlock()
			}
			return &c
		default:
			return nil
		}
	case <-s.ctx.Done():
		s.mu.Lock()
		if s.err == nil {
			s.err = s.ctx.Err()
		}
		s.mu.Unlock()
		return nil
	}
}

// Err implements Stream.
func (s *Buffered) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close implements Stream.
func (s *Buffered) Close() {
	s.Finish()
	s.cancel()
}

// Collect drains the stream and returns all chunks + first error.
func Collect(s Stream) ([]Chunk, error) {
	var chunks []Chunk
	for {
		c := s.Next()
		if c == nil {
			break
		}
		chunks = append(chunks, *c)
	}
	return chunks, s.Err()
}
