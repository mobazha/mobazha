package events

import "testing"

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
