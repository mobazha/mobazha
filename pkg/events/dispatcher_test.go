package events

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockSink struct {
	name     string
	accepted func(EventMeta) bool
	handled  []interface{}
	mu       sync.Mutex
	count    atomic.Int32
}

func (s *mockSink) Name() string { return s.name }

func (s *mockSink) Accept(meta EventMeta) bool {
	if s.accepted != nil {
		return s.accepted(meta)
	}
	return true
}

func (s *mockSink) Handle(_ context.Context, _ EventMeta, event interface{}) error {
	s.mu.Lock()
	s.handled = append(s.handled, event)
	s.mu.Unlock()
	s.count.Add(1)
	return nil
}

func (s *mockSink) eventCount() int {
	return int(s.count.Load())
}

func TestDispatcher_FanOut(t *testing.T) {
	bus := NewBus()
	sink1 := &mockSink{name: "s1"}
	sink2 := &mockSink{name: "s2"}

	d := NewDispatcher(bus, sink1, sink2)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	bus.Emit(&NewOrder{})

	waitFor(t, func() bool { return sink1.eventCount() >= 1 && sink2.eventCount() >= 1 }, 2*time.Second)

	if sink1.eventCount() != 1 {
		t.Errorf("sink1 expected 1 event, got %d", sink1.eventCount())
	}
	if sink2.eventCount() != 1 {
		t.Errorf("sink2 expected 1 event, got %d", sink2.eventCount())
	}
}

func TestDispatcher_AcceptFilter(t *testing.T) {
	bus := NewBus()
	orderOnly := &mockSink{
		name: "orderOnly",
		accepted: func(meta EventMeta) bool {
			return meta.Category == "order"
		},
	}
	chatOnly := &mockSink{
		name: "chatOnly",
		accepted: func(meta EventMeta) bool {
			return meta.Category == "chat"
		},
	}

	d := NewDispatcher(bus, orderOnly, chatOnly)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	bus.Emit(&NewOrder{})
	bus.Emit(&ChatMessage{})

	waitFor(t, func() bool { return orderOnly.eventCount() >= 1 && chatOnly.eventCount() >= 1 }, 2*time.Second)

	if orderOnly.eventCount() != 1 {
		t.Errorf("orderOnly expected 1, got %d", orderOnly.eventCount())
	}
	if chatOnly.eventCount() != 1 {
		t.Errorf("chatOnly expected 1, got %d", chatOnly.eventCount())
	}
}

func TestDispatcher_UnregisteredEventIgnored(t *testing.T) {
	bus := NewBus()
	sink := &mockSink{name: "all"}

	d := NewDispatcher(bus, sink)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	bus.Emit(&NewOrder{})
	waitFor(t, func() bool { return sink.eventCount() >= 1 }, 2*time.Second)

	if sink.eventCount() != 1 {
		t.Errorf("expected 1, got %d", sink.eventCount())
	}
}

func TestDispatcher_MultipleEvents(t *testing.T) {
	bus := NewBus()
	sink := &mockSink{name: "multi"}

	d := NewDispatcher(bus, sink)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	defer d.Stop()

	bus.Emit(&NewOrder{})
	bus.Emit(&OrderFunded{})
	bus.Emit(&DisputeOpen{})

	waitFor(t, func() bool { return sink.eventCount() >= 3 }, 2*time.Second)
	if sink.eventCount() != 3 {
		t.Errorf("expected 3, got %d", sink.eventCount())
	}
}

type concurrentMockSink struct {
	mockSink
	concurrency int
}

func (s *concurrentMockSink) Concurrency() int { return s.concurrency }

func TestSinkWorkerCount_Default(t *testing.T) {
	sink := &mockSink{name: "plain"}
	if n := sinkWorkerCount(sink); n != 2 {
		t.Errorf("expected default 2, got %d", n)
	}
}

func TestSinkWorkerCount_ConcurrentSink(t *testing.T) {
	sink := &concurrentMockSink{mockSink: mockSink{name: "conc"}, concurrency: 8}
	if n := sinkWorkerCount(sink); n != 8 {
		t.Errorf("expected 8, got %d", n)
	}
}

func TestSinkWorkerCount_ConcurrentSinkZero(t *testing.T) {
	sink := &concurrentMockSink{mockSink: mockSink{name: "zero"}, concurrency: 0}
	if n := sinkWorkerCount(sink); n != 2 {
		t.Errorf("expected fallback 2 for zero concurrency, got %d", n)
	}
}

func waitFor(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
