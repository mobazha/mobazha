package events

import "io"

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
