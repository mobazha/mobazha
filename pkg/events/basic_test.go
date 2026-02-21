package events

import (
	"testing"
	"time"
)

func TestSubscribeAndEmit(t *testing.T) {
	type TestNotif1 struct{}
	type TestNotif2 struct{}

	bus := NewBus()

	sub1, err := bus.Subscribe(&TestNotif1{})
	if err != nil {
		t.Fatal(err)
	}

	sub2, err := bus.Subscribe(&TestNotif2{})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		bus.Emit(&TestNotif1{})
		bus.Emit(&TestNotif2{})
	}()

	notif1 := <-sub1.Out()
	_, ok := notif1.(*TestNotif1)
	if !ok {
		t.Error("Notification is wrong type")
	}

	notif2 := <-sub2.Out()
	_, ok = notif2.(*TestNotif2)
	if !ok {
		t.Error("Notification is wrong type")
	}

	if err := sub1.Close(); err != nil {
		t.Error(err)
	}

	if err := sub2.Close(); err != nil {
		t.Error(err)
	}
}

func TestEmit_AllEventsDelivered(t *testing.T) {
	type Evt struct{ Seq int }

	bus := NewBus()
	sub1, err := bus.Subscribe(&Evt{}, BufSize(64))
	if err != nil {
		t.Fatal(err)
	}
	defer sub1.Close()

	const total = 50
	go func() {
		for i := 0; i < total; i++ {
			bus.Emit(&Evt{Seq: i})
		}
	}()

	for i := 0; i < total; i++ {
		select {
		case evt := <-sub1.Out():
			if evt.(*Evt).Seq != i {
				t.Fatalf("expected Seq=%d, got %d", i, evt.(*Evt).Seq)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
}

func TestEmit_MultipleSubscribers_AllReceive(t *testing.T) {
	type Evt struct{ Seq int }

	bus := NewBus()

	sub1, err := bus.Subscribe(&Evt{}, BufSize(64))
	if err != nil {
		t.Fatal(err)
	}
	defer sub1.Close()

	sub2, err := bus.Subscribe(&Evt{}, BufSize(64))
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Close()

	const total = 10
	go func() {
		for i := 0; i < total; i++ {
			bus.Emit(&Evt{Seq: i})
		}
	}()

	for i := 0; i < total; i++ {
		select {
		case <-sub1.Out():
		case <-time.After(2 * time.Second):
			t.Fatalf("sub1 timed out at event %d", i)
		}
		select {
		case <-sub2.Out():
		case <-time.After(2 * time.Second):
			t.Fatalf("sub2 timed out at event %d", i)
		}
	}
}
