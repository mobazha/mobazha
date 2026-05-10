// Package testutil provides test-only helpers for order/dispute/payment
// message construction. It is deliberately separated from
// internal/orders/utils because that package depends on pkg/contracts.Signer,
// which transitively imports pkg/models — pulling test fixtures from
// pkg/models would create an import cycle (pkg/models → utils → contracts
// → pkg/models). Keeping the helpers here means consumers can fixture
// order messages without dragging in the contracts layer.
package testutil

import (
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// MustWrapOrderMessage wraps an order/dispute/payment protobuf message in
// the canonical OrderMessage envelope used over the wire. The OrderID and
// Signature fields are populated with placeholder values suitable for tests
// that exercise marshalling and routing logic but do not verify signatures.
func MustWrapOrderMessage(message proto.Message) *npb.OrderMessage {
	a := &anypb.Any{}
	if err := a.MarshalFrom(message); err != nil {
		panic(err)
	}
	var messageType npb.OrderMessage_MessageType
	switch message.(type) {
	case *pb.OrderOpen:
		messageType = npb.OrderMessage_ORDER_OPEN
	case *pb.OrderDecline:
		messageType = npb.OrderMessage_ORDER_DECLINE
	case *pb.OrderCancel:
		messageType = npb.OrderMessage_ORDER_CANCEL
	case *pb.OrderConfirmation:
		messageType = npb.OrderMessage_ORDER_CONFIRMATION
	case *pb.RatingSignatures:
		messageType = npb.OrderMessage_RATING_SIGNATURES
	case *pb.OrderShipment:
		messageType = npb.OrderMessage_ORDER_SHIPMENT
	case *pb.OrderComplete:
		messageType = npb.OrderMessage_ORDER_COMPLETE
	case *pb.DisputeOpen:
		messageType = npb.OrderMessage_DISPUTE_OPEN
	case *pb.DisputeUpdate:
		messageType = npb.OrderMessage_DISPUTE_UPDATE
	case *pb.DisputeClose:
		messageType = npb.OrderMessage_DISPUTE_CLOSE
	case *pb.Refund:
		messageType = npb.OrderMessage_REFUND
	case *pb.PaymentSent:
		messageType = npb.OrderMessage_PAYMENT_SENT
	case *pb.PaymentFinalized:
		messageType = npb.OrderMessage_PAYMENT_FINALIZED
	}
	return &npb.OrderMessage{
		OrderID:     "abc",
		Message:     a,
		MessageType: messageType,
		Signature:   []byte("1234"),
	}
}
