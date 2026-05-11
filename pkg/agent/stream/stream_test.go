package stream

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBuffered_BasicFlow(t *testing.T) {
	s := NewBuffered(context.Background(), 4)

	go func() {
		s.Send(Chunk{Delta: "Hello"})
		s.Send(Chunk{Delta: " World"})
		s.Finish()
	}()

	chunks, err := Collect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Delta != "Hello" || chunks[1].Delta != " World" {
		t.Fatalf("unexpected content: %v", chunks)
	}
}

func TestBuffered_ErrorPropagation(t *testing.T) {
	s := NewBuffered(context.Background(), 4)
	expectedErr := errors.New("llm timeout")

	go func() {
		s.Send(Chunk{Delta: "partial"})
		s.SendError(expectedErr)
	}()

	chunks, err := Collect(s)
	if err == nil || !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk before error")
	}
}

func TestBuffered_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewBuffered(ctx, 4)

	go func() {
		s.Send(Chunk{Delta: "first"})
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	var count int
	for s.Next() != nil {
		count++
	}
	err := s.Err()
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestBuffered_CloseIdempotent(t *testing.T) {
	s := NewBuffered(context.Background(), 4)
	s.Close()
	s.Close() // should not panic
	if c := s.Next(); c != nil {
		t.Fatal("expected nil after close")
	}
}

func TestBuffered_ToolCallChunk(t *testing.T) {
	s := NewBuffered(context.Background(), 4)

	go func() {
		s.Send(Chunk{
			ToolCalls: []ToolCall{
				{ID: "tc_1", Name: "search_listings", Arguments: `{"query":"vintage"}`},
			},
		})
		s.Finish()
	}()

	chunks, err := Collect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	tc := chunks[0].ToolCalls
	if len(tc) != 1 || tc[0].Name != "search_listings" {
		t.Fatalf("unexpected tool call: %v", tc)
	}
}

func TestBuffered_SendAfterFinish(t *testing.T) {
	s := NewBuffered(context.Background(), 4)
	s.Finish()
	s.Send(Chunk{Delta: "late"}) // should not panic or block
}

func TestBuffered_ConcurrentSendFinish(t *testing.T) {
	for i := 0; i < 100; i++ {
		s := NewBuffered(context.Background(), 1)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				s.Send(Chunk{Delta: "x"})
			}
		}()
		go func() {
			defer wg.Done()
			s.Finish()
		}()
		wg.Wait()
	}
}
