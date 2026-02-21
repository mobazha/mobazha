package events

import (
	"errors"
	"reflect"
	"sync"
)

// basicBus is a type-based event delivery system.
// Emit blocks if any subscriber's channel is full. The default buffer size (64)
// is generous enough that blocking should not occur when consumers drain promptly.
// Real backpressure isolation lives in the Dispatcher layer (per-sink non-blocking fan-out).
type basicBus struct {
	lk   sync.Mutex
	subs map[reflect.Type][]*sub
}

var _ Bus = (*basicBus)(nil)

func (b *basicBus) Emit(event interface{}) {
	b.lk.Lock()
	defer b.lk.Unlock()

	typ := reflect.TypeOf(event)
	sinks, ok := b.subs[typ]
	if !ok {
		return
	}

notify:
	for _, sub := range sinks {
		if sub.match != nil {
			val := reflect.Indirect(reflect.ValueOf(event))
			for field, value := range sub.match {
				if val.FieldByName(field).String() != value {
					continue notify
				}
			}
		}
		sub.ch <- event
	}
}

func (b *basicBus) dropSubscriber(typ reflect.Type, s *sub) {
	b.lk.Lock()
	defer b.lk.Unlock()

	subs, ok := b.subs[typ]
	if !ok {
		return
	}
	for i, sub := range subs {
		if sub == s {
			subs = append(subs[:i], subs[i+1:]...)
			b.subs[typ] = subs
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
	ch    chan interface{}
	typs  []reflect.Type
	drop  func(typ reflect.Type, s *sub)
	match map[string]string
}

func (s *sub) Out() <-chan interface{} {
	return s.ch
}

func (s *sub) Close() error {
	go func() {
		// Drain the channel so any blocked Emit unblocks, allowing drop() to
		// acquire the lock and remove this subscriber.
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

// Subscribe creates a new subscription.
// Failing to drain the channel may cause publishers to block.
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
		out.match = settings.matchFieldValues
	}

	return out, nil
}
