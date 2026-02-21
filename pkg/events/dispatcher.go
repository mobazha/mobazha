package events

import (
	"context"
	"sync"

	"github.com/op/go-logging"
)

var dispatcherLog = logging.MustGetLogger("EVTD")

const defaultSinkBuffer = 256

// Dispatcher subscribes to the EventBus once and fans out events to registered EventSinks.
// Each sink has its own buffered channel and a configurable number of worker goroutines,
// providing error isolation and backpressure.
type Dispatcher struct {
	bus     Bus
	sinks  []EventSink
	sub    Subscription
	done   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup

	// per-sink worker channels
	workers map[string]chan dispatchItem
}

type dispatchItem struct {
	meta  EventMeta
	event interface{}
}

// NewDispatcher creates a dispatcher for the given sinks. Call Start() to begin processing.
func NewDispatcher(bus Bus, sinks ...EventSink) *Dispatcher {
	return &Dispatcher{
		bus:     bus,
		sinks:  sinks,
		done:   make(chan struct{}),
		workers: make(map[string]chan dispatchItem),
	}
}

// Start subscribes to all registered events and begins dispatching.
func (d *Dispatcher) Start() error {
	sub, err := d.bus.Subscribe(AllSamples())
	if err != nil {
		return err
	}
	d.sub = sub

	for _, sink := range d.sinks {
		ch := make(chan dispatchItem, defaultSinkBuffer)
		d.workers[sink.Name()] = ch
		workers := sinkWorkerCount(sink)
		for i := 0; i < workers; i++ {
			d.wg.Add(1)
			go d.sinkWorker(sink, ch)
		}
	}

	d.wg.Add(1)
	go d.loop()
	return nil
}

// Stop gracefully shuts down the dispatcher and waits for all workers to finish.
// ManagedEscrow to call multiple times.
func (d *Dispatcher) Stop() {
	d.once.Do(func() {
		close(d.done)
		if d.sub != nil {
			d.sub.Close()
		}
		for _, ch := range d.workers {
			close(ch)
		}
	})
	d.wg.Wait()
}

func (d *Dispatcher) loop() {
	defer d.wg.Done()
	for {
		select {
		case <-d.done:
			return
		case evt, ok := <-d.sub.Out():
			if !ok {
				return
			}
			meta := LookupEvent(evt)
			if meta == nil {
				continue
			}
			item := dispatchItem{meta: *meta, event: evt}
			for _, sink := range d.sinks {
				if !sink.Accept(*meta) {
					continue
				}
				ch, exists := d.workers[sink.Name()]
				if !exists {
					continue
				}
				select {
				case ch <- item:
				default:
					dispatcherLog.Warningf("sink %s backpressure: dropping event %s", sink.Name(), meta.Name)
				}
			}
		}
	}
}

func (d *Dispatcher) sinkWorker(sink EventSink, ch <-chan dispatchItem) {
	defer d.wg.Done()
	for item := range ch {
		if err := sink.Handle(context.Background(), item.meta, item.event); err != nil {
			dispatcherLog.Errorf("sink %s error handling %s: %v", sink.Name(), item.meta.Name, err)
		}
	}
}

// sinkWorkerCount returns the number of goroutines for a sink.
// If the sink implements ConcurrentSink, its Concurrency() value is used.
// Otherwise defaults to 2.
func sinkWorkerCount(sink EventSink) int {
	if cs, ok := sink.(ConcurrentSink); ok {
		if n := cs.Concurrency(); n > 0 {
			return n
		}
	}
	return 2
}
