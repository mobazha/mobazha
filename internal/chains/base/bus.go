package base

import (
	"errors"
	"io"
	"reflect"
	"sync"
)

// SubscriptionOpt represents a subscriber option. Use the options exposed by the implementation of choice.
type SubscriptionOpt = func(interface{}) error

// CancelFunc closes a subscriber.
type CancelFunc = func()

// Subscription represents a subscription to one or multiple event types.
type Subscription interface {
	io.Closer

	// Out returns the channel from which to consume events.
	Out() <-chan interface{}
}

// Bus is an interface for a type-based event delivery system.
type Bus interface {
	// Subscribe creates a new Subscription.
	//
	// eventType can be either a pointer to a single event type, or a slice of pointers to
	// subscribe to multiple event types at once, under a single subscription (and channel).
	//
	// Failing to drain the channel may cause publishers to block.
	//
	// Simple example
	//
	//  sub, err := eventbus.Subscribe(new(EventType))
	//  defer sub.Close()
	//  for e := range sub.Out() {
	//    event := e.(EventType) // guaranteed safe
	//    [...]
	//  }
	//
	// Multi-type example
	//
	//  sub, err := eventbus.Subscribe([]interface{}{new(EventA), new(EventB)})
	//  defer sub.Close()
	//  for e := range sub.Out() {
	//    select e.(type):
	//      case EventA:
	//        [...]
	//      case EventB:
	//        [...]
	//    }
	//  }
	Subscribe(eventType interface{}, opts ...SubscriptionOpt) (Subscription, error)

	// Emit emits an event onto the eventbus. If any channel subscribed to the topic is blocked,
	// calls to Emit will block.
	//
	// Calling this function with wrong event type will cause a panic.
	Emit(evt interface{})
}

type (
	ChainStartedEvent              struct{}
	BlockReceivedEvent             struct{}
	ScanCompleteEvent              struct{}
	UpdateUnconfirmedCompleteEvent struct{}
	WatchAddressAddedEvent         struct{}
	AddAddressSubscriptionEvent    struct{}
)

type subSettings struct {
	buffer int
}

var subSettingsDefault = subSettings{
	buffer: 16,
}

func BufSize(n int) func(interface{}) error {
	return func(s interface{}) error {
		s.(*subSettings).buffer = n
		return nil
	}
}

// basicBus is a type-based event delivery system
type basicBus struct {
	lk   sync.Mutex
	subs map[reflect.Type][]*sub
}

var _ Bus = (*basicBus)(nil)

func (b *basicBus) Emit(event interface{}) {
	typ := reflect.TypeOf(event)
	sinks, ok := b.subs[typ]
	if !ok {
		return
	}
	for _, sub := range sinks {
		sub.ch <- event
	}
}

func (b *basicBus) dropSubscriber(typ reflect.Type, s *sub) {
	subs, ok := b.subs[typ]
	if !ok {
		return
	}
	for i, sub := range subs {
		if sub == s {
			subs = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

// NewBus returns a basic event bus.
func NewBus() Bus {
	return &basicBus{
		lk:   sync.Mutex{},
		subs: make(map[reflect.Type][]*sub),
	}
}

type sub struct {
	ch   chan interface{}
	typs []reflect.Type
	drop func(typ reflect.Type, s *sub)
}

func (s *sub) Out() <-chan interface{} {
	return s.ch
}

func (s *sub) Close() error {
	go func() {
		// drain the event channel, will return when closed and drained.
		// this is necessary to unblock publishes to this channel.
		for range s.ch {
		}
	}()

	for _, typ := range s.typs {
		s.drop(typ, s)
	}
	close(s.ch)
	return nil
}

var _ Subscription = (*sub)(nil)

// Subscribe creates new subscription. Failing to drain the channel will cause
// publishers to get blocked.
func (b *basicBus) Subscribe(evtTypes interface{}, opts ...SubscriptionOpt) (_ Subscription, err error) {
	b.lk.Lock()
	defer b.lk.Unlock()

	settings := subSettingsDefault
	for _, opt := range opts {
		if err := opt(&settings); err != nil {
			return nil, err
		}
	}

	types, ok := evtTypes.([]interface{})
	if !ok {
		types = []interface{}{evtTypes}
	}

	out := &sub{
		ch:   make(chan interface{}, settings.buffer),
		drop: b.dropSubscriber,
	}

	for _, etyp := range types {
		if reflect.TypeOf(etyp).Kind() != reflect.Ptr {
			return nil, errors.New("subscribe called with non-pointer type")
		}
	}

	for _, etyp := range types {
		typ := reflect.TypeOf(etyp)
		cur, ok := b.subs[typ]
		if !ok {
			cur = []*sub{}
			b.subs[typ] = cur
		}

		cur = append(cur, out)
		b.subs[typ] = cur
		out.typs = append(out.typs, typ)
	}

	return out, nil
}
