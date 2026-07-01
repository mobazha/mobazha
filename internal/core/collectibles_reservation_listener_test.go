package core

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/events"
)

func TestCollectibleReservationReleaseEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   interface{}
		orderID string
	}{
		{name: "cancel", event: &events.OrderCancel{OrderID: "order-cancel"}, orderID: "order-cancel"},
		{name: "decline", event: &events.OrderDeclined{OrderID: "order-decline"}, orderID: "order-decline"},
		{name: "expired", event: &events.OrderExpired{OrderID: "order-expired", Reason: "timeout"}, orderID: "order-expired"},
		{name: "auto cancelled", event: &events.OrderAutoCancelled{OrderID: "order-auto", Reason: "shipment_overdue"}, orderID: "order-auto"},
		{name: "ignored", event: &events.OrderFunded{OrderID: "order-funded"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			orderID, _ := collectibleReservationReleaseEvent(test.event)
			if orderID != test.orderID {
				t.Fatalf("orderID = %q, want %q", orderID, test.orderID)
			}
		})
	}
}
