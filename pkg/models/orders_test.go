package models

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestOrder_Role(t *testing.T) {
	var order Order

	order.SetRole(RoleVendor)

	ret := order.Role()
	if ret != RoleVendor {
		t.Errorf("Expected RoleVendor, got %s", ret)
	}
}

func TestOrder_Timestamp(t *testing.T) {
	var order Order

	now := time.Now().UTC()
	pbt := timestamppb.New(now)
	err := pbt.CheckValid()
	if err != nil {
		t.Fatal(err)
	}
	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
		Timestamp: pbt,
	}))
	if err != nil {
		t.Fatal(err)
	}

	check, err := order.Timestamp()
	if err != nil {
		t.Fatal(err)
	}
	if now != check {
		t.Fatal("Returned incorrect timestamp")
	}
}

func TestOrder_ReturnRole(t *testing.T) {
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					VendorID: &pb.ID{
						PeerID: "QmbN1x5opuJ8FwNyQDaasCRRiTami1WcbNV5oguwHX83g9",
					},
				},
			},
		},
		BuyerID: &pb.ID{
			PeerID: "QmPFZPt6FJMZFQABX44RnxmZGh2XGW8ev7KKEMpL8YMxd4",
		},
	}
	paymentSent := &pb.PaymentSent{
		Moderator: "QmW4cc8jh8vNDza49YVCmFX56tb7QtEGfvcihXEWAKwdcf",
	}

	var order Order
	if err := order.PutMessage(utils.MustWrapOrderMessage(orderOpen)); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(utils.MustWrapOrderMessage(paymentSent)); err != nil {
		t.Fatal(err)
	}

	buyerID, err := order.Buyer()
	if err != nil {
		t.Fatal(err)
	}

	if buyerID.String() != orderOpen.BuyerID.PeerID {
		t.Errorf("Incorrect peerID. Expected %s, got %s", orderOpen.BuyerID.PeerID, buyerID)
	}

	vendorID, err := order.Vendor()
	if err != nil {
		t.Fatal(err)
	}

	if vendorID.String() != orderOpen.Listings[0].Listing.VendorID.PeerID {
		t.Errorf("Incorrect peerID. Expected %s, got %s", orderOpen.Listings[0].Listing.VendorID.PeerID, vendorID)
	}

	moderatorID, err := order.Moderator()
	if err != nil {
		t.Fatal(err)
	}

	if moderatorID.String() != paymentSent.Moderator {
		t.Errorf("Incorrect peerID. Expected %s, got %s", paymentSent.Moderator, moderatorID.String())
	}

	paymentSent.Moderator = ""
	if err := order.PutMessage(utils.MustWrapOrderMessage(paymentSent)); err != nil {
		t.Fatal(err)
	}

	_, err = order.Moderator()
	if err == nil {
		t.Error("Expected error from Moderator() method")
	}

}

func TestOrder_Transactions(t *testing.T) {
	var (
		order Order
		id0   = "xyz"
		id1   = "abc"
	)

	err := order.PutTransaction(iwallet.Transaction{
		ID: iwallet.TransactionID(id0),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutTransaction(iwallet.Transaction{
		ID: iwallet.TransactionID(id1),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutTransaction(iwallet.Transaction{
		ID: "abc",
	})
	if err != ErrDuplicateTransaction {
		t.Errorf("Failed to return duplicate transaction error")
	}

	txs, err := order.GetTransactions()
	if err != nil {
		t.Fatal(err)
	}

	for txs[0].ID.String() != id0 {
		t.Errorf("Incorrect txid returned. Expected %s, got %s", id0, txs[0].ID)
	}
	for txs[1].ID.String() != id1 {
		t.Errorf("Incorrect txid returned. Expected %s, got %s", id1, txs[1].ID)
	}
}

func TestOrder_PutAndGet(t *testing.T) {
	messages := []proto.Message{
		&pb.OrderOpen{},
		&pb.OrderDecline{},
		&pb.OrderCancel{},
		&pb.OrderConfirmation{},
		&pb.RatingSignatures{},
		&pb.OrderFulfillment{},
		&pb.OrderComplete{},
		&pb.DisputeOpen{},
		&pb.DisputeClose{},
		&pb.DisputeUpdate{},
		&pb.Refund{},
		&pb.PaymentFinalized{},
	}

	var order Order
	for i, message := range messages {
		if err := order.PutMessage(utils.MustWrapOrderMessage(message)); err != nil {
			t.Errorf("Error putting message %d", i)
		}
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if orderOpen == nil {
		t.Error("Message is nil")
	}
	if order.OrderOpenSignature == "" {
		t.Error("signature is empty")
	}
	orderDecline, err := order.OrderDeclineMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if orderDecline == nil {
		t.Error("Message is nil")
	}
	if order.OrderDeclineSignature == "" {
		t.Error("signature is empty")
	}
	orderCancel, err := order.OrderCancelMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if orderCancel == nil {
		t.Error("Message is nil")
	}
	if order.OrderCancelSignature == "" {
		t.Error("signature is empty")
	}
	orderConfirmation, err := order.OrderConfirmationMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if orderConfirmation == nil {
		t.Error("Message is nil")
	}
	if order.OrderConfirmationSignature == "" {
		t.Error("signature is empty")
	}
	ratingSignatures, err := order.RatingSignaturesMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if ratingSignatures == nil {
		t.Error("Message is nil")
	}
	if order.RatingSignaturesSignature == "" {
		t.Error("signature is empty")
	}
	orderFulfillment, err := order.OrderFulfillmentMessages()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if orderFulfillment == nil {
		t.Error("Message is nil")
	}
	orderComplete, err := order.OrderCompleteMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if orderComplete == nil {
		t.Error("Message is nil")
	}
	if order.OrderCompleteSignature == "" {
		t.Error("signature is empty")
	}
	disputeOpen, err := order.DisputeOpenMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if disputeOpen == nil {
		t.Error("Message is nil")
	}
	if order.DisputeOpenSignature == "" {
		t.Error("signature is empty")
	}
	disputeClosed, err := order.DisputeClosedMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if disputeClosed == nil {
		t.Error("Message is nil")
	}
	if order.DisputeClosedSignature == "" {
		t.Error("signature is empty")
	}
	disputeUpdate, err := order.DisputeUpdateMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if disputeUpdate == nil {
		t.Error("Message is nil")
	}
	if order.DisputeUpdateSignature == "" {
		t.Error("signature is empty")
	}
	refunds, err := order.Refunds()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if refunds == nil {
		t.Error("Message is nil")
	}
	paymentFinalized, err := order.PaymentFinalizedMessage()
	if err != nil {
		t.Errorf("Get failed: %s", err)
	}
	if paymentFinalized == nil {
		t.Error("Message is nil")
	}
	if order.PaymentFinalizedSignature == "" {
		t.Error("signature is empty")
	}

	order = Order{}
	orderOpen, err = order.OrderOpenMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	orderDecline, err = order.OrderDeclineMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	orderCancel, err = order.OrderCancelMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	orderConfirmation, err = order.OrderConfirmationMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	ratingSignatures, err = order.RatingSignaturesMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	orderFulfillment, err = order.OrderFulfillmentMessages()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	orderComplete, err = order.OrderCompleteMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	disputeOpen, err = order.DisputeOpenMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	disputeClosed, err = order.DisputeClosedMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	disputeUpdate, err = order.DisputeUpdateMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	refunds, err = order.Refunds()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
	paymentFinalized, err = order.PaymentFinalizedMessage()
	if err != ErrMessageDoesNotExist {
		t.Errorf("Get failed to return correct error: %s", err)
	}
}

func TestOrder_Payments(t *testing.T) {
	var (
		order Order
		id0   = "xyz"
	)

	err := order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
		TransactionID: id0,
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
		TransactionID: id0,
	}))
	if err != ErrDuplicateTransaction {
		t.Errorf("expected ErrDuplicateTransaction, got %v", err)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		t.Fatal(err)
	}

	if paymentSent.TransactionID != id0 {
		t.Errorf("Incorrect txid returned. Expected %s, got %s", id0, paymentSent.TransactionID)
	}
}

func TestOrder_Fulfillments(t *testing.T) {
	var (
		order Order
		idx1  = uint32(1)
		idx2  = uint32(2)
	)

	err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
		Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
			{
				ItemIndex: idx1,
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
		Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
			{
				ItemIndex: idx2,
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
		Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
			{
				ItemIndex: idx1,
			},
		},
	}))
	if err != ErrDuplicateTransaction {
		t.Errorf("Failed to return duplicate transaction error")
	}

	fulfillments, err := order.OrderFulfillmentMessages()
	if err != nil {
		t.Fatal(err)
	}

	for fulfillments[0].Fulfillments[0].ItemIndex != idx1 {
		t.Errorf("Incorrect index returned. Expected %d, got %d", idx1, fulfillments[0].Fulfillments[0].ItemIndex)
	}
	for fulfillments[1].Fulfillments[0].ItemIndex != idx2 {
		t.Errorf("Incorrect index returned. Expected %d, got %d", idx2, fulfillments[1].Fulfillments[0].ItemIndex)
	}
}

func TestOrder_Refunds(t *testing.T) {
	var (
		order    Order
		id0      = "xyz"
		id1      = "abc"
		release0 = &pb.Refund_ReleaseInfo{ReleaseInfo: &pb.EscrowRelease{
			EscrowSignatures: []*pb.Signature{
				{
					From:      []byte{0x00},
					Signature: []byte{0x01},
					Index:     0,
				},
			},
			ToAddress: "abc",
			ToAmount:  "0",
		}}
		release1 = &pb.Refund_ReleaseInfo{ReleaseInfo: &pb.EscrowRelease{
			EscrowSignatures: []*pb.Signature{
				{
					From:      []byte{0x00},
					Signature: []byte{0x02},
					Index:     0,
				},
			},
			ToAddress: "abc",
			ToAmount:  "1",
		}}
	)

	err := order.PutMessage(utils.MustWrapOrderMessage(&pb.Refund{
		RefundInfo: &pb.Refund_TransactionID{
			TransactionID: id0,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.Refund{
		RefundInfo: &pb.Refund_TransactionID{
			TransactionID: id1,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.Refund{
		RefundInfo: &pb.Refund_TransactionID{
			TransactionID: id1,
		},
	}))
	if err != ErrDuplicateTransaction {
		t.Errorf("Failed to return duplicate transaction error")
	}

	refunds, err := order.Refunds()
	if err != nil {
		t.Fatal(err)
	}

	for refunds[0].GetTransactionID() != id0 {
		t.Errorf("Incorrect txid returned. Expected %s, got %s", id0, refunds[0].GetTransactionID())
	}
	for refunds[1].GetTransactionID() != id1 {
		t.Errorf("Incorrect txid returned. Expected %s, got %s", id1, refunds[1].GetTransactionID())
	}

	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.Refund{
		RefundInfo: release0,
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.Refund{
		RefundInfo: release1,
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = order.PutMessage(utils.MustWrapOrderMessage(&pb.Refund{
		RefundInfo: release1,
	}))
	if err != ErrDuplicateTransaction {
		t.Errorf("Failed to return duplicate transaction error")
	}

	refunds, err = order.Refunds()
	if err != nil {
		t.Fatal(err)
	}

	marshaler := protojson.MarshalOptions{}

	releaseInfo0 := marshaler.Format(&pb.Refund{
		RefundInfo: release0,
	})

	releaseInfo1 := marshaler.Format(&pb.Refund{
		RefundInfo: release1,
	})

	saved0 := marshaler.Format(refunds[2])

	saved1 := marshaler.Format(refunds[3])

	if releaseInfo0 != saved0 {
		t.Error("Incorrect release info returned")
	}
	if releaseInfo1 != saved1 {
		t.Error("Incorrect release info returned")
	}
}

func TestOrder_ParkedMessages(t *testing.T) {
	m1 := &anypb.Any{}
	if err := m1.MarshalFrom(&pb.OrderOpen{AlternateContactInfo: "abc"}); err != nil {
		t.Fatal(err)
	}
	m2 := &anypb.Any{}
	if err := m2.MarshalFrom(&pb.OrderOpen{AlternateContactInfo: "123"}); err != nil {
		t.Fatal(err)
	}
	var (
		order Order
		msg1  = &npb.OrderMessage{
			MessageType: npb.OrderMessage_ORDER_OPEN,
			Message:     m1,
		}

		msg2 = &npb.OrderMessage{
			MessageType: npb.OrderMessage_ORDER_DECLINE,
			Message:     m2,
		}
	)

	msgs, err := order.GetParkedMessages()
	if err != nil {
		t.Fatal(err)
	}

	if len(msgs.Messages) != 0 {
		t.Error("Messages should be nil")
	}

	// Set SenderPeerID (normally set by SignOrderMessage)
	msg1.SenderPeerID = "12D3KooWTestPeer"
	msg2.SenderPeerID = "12D3KooWTestPeer"
	if err := order.ParkMessage(msg1); err != nil {
		t.Fatal(err)
	}
	if err := order.ParkMessage(msg2); err != nil {
		t.Fatal(err)
	}

	msgs, err = order.GetParkedMessages()
	if err != nil {
		t.Fatal(err)
	}

	if msgs.Messages[0].MessageType != msg1.MessageType {
		t.Errorf("Expected %s message type got %s", msg1.MessageType.String(), msgs.Messages[0].MessageType.String())
	}
	if msgs.Messages[1].MessageType != msg2.MessageType {
		t.Errorf("Expected %s message type got %s", msg2.MessageType.String(), msgs.Messages[1].MessageType.String())
	}

	if err := order.DeleteParkedMessage(npb.OrderMessage_ORDER_DECLINE); err != nil {
		t.Fatal(err)
	}

	msgs, err = order.GetParkedMessages()
	if err != nil {
		t.Fatal(err)
	}

	if len(msgs.Messages) != 1 {
		t.Errorf("Failed to delete message")
	}
}

func TestOrder_CanCancel(t *testing.T) {
	tests := []struct {
		setup     func(order *Order) error
		ourRole   OrderRole
		canCancel bool
	}{
		{
			// Success
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: true,
		},
		{
			// Is vendor
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canCancel: false,
		},
		{
			// Order is nil
			setup: func(order *Order) error {
				return nil
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil decline
			setup: func(order *Order) error {
				order.SerializedOrderDecline = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil cancel
			setup: func(order *Order) error {
				order.SerializedOrderCancel = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil confirmation
			setup: func(order *Order) error {
				order.SerializedOrderConfirmation = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil fulfillment
			setup: func(order *Order) error {
				order.SerializedOrderFulfillments = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil complete
			setup: func(order *Order) error {
				order.SerializedOrderComplete = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil dispute open
			setup: func(order *Order) error {
				order.SerializedDisputeOpen = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil dispute close
			setup: func(order *Order) error {
				order.SerializedDisputeClosed = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil dispute update
			setup: func(order *Order) error {
				order.SerializedDisputeUpdate = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil refund
			setup: func(order *Order) error {
				order.SerializedRefunds = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
		{
			// Non nil payment finalized
			setup: func(order *Order) error {
				order.SerializedPaymentFinalized = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleBuyer,
			canCancel: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}
		order.SetRole(test.ourRole)

		canCancel := order.CanCancel()
		if canCancel != test.canCancel {
			t.Errorf("Test %d: Got incorrect result. Expected %t, got %t", i, test.canCancel, canCancel)
		}
	}
}

func TestOrder_ErroredMessages(t *testing.T) {
	var (
		order Order
		msg1  = &npb.OrderMessage{
			MessageType: npb.OrderMessage_ORDER_OPEN,
		}

		msg2 = &npb.OrderMessage{
			MessageType: npb.OrderMessage_ORDER_DECLINE,
		}
	)

	msgs, err := order.GetErroredMessages()
	if err != nil {
		t.Fatal(err)
	}

	if len(msgs.Messages) != 0 {
		t.Error("Messages should be nil")
	}

	if err := order.PutErrorMessage(msg1); err != nil {
		t.Fatal(err)
	}
	if err := order.PutErrorMessage(msg2); err != nil {
		t.Fatal(err)
	}

	msgs, err = order.GetErroredMessages()
	if err != nil {
		t.Fatal(err)
	}

	if msgs.Messages[0].MessageType != msg1.MessageType {
		t.Errorf("Expected %s message type got %s", msg1.MessageType.String(), msgs.Messages[0].MessageType.String())
	}
	if msgs.Messages[1].MessageType != msg2.MessageType {
		t.Errorf("Expected %s message type got %s", msg2.MessageType.String(), msgs.Messages[1].MessageType.String())
	}
}

func TestOrder_CanDecline(t *testing.T) {
	tests := []struct {
		setup      func(order *Order) error
		ourRole    OrderRole
		canDecline bool
	}{
		{
			// Success
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canDecline: true,
		},
		{
			// Is buyer
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleBuyer,
			canDecline: false,
		},
		{
			// Order is nil
			setup: func(order *Order) error {
				return nil
			},
			ourRole:    RoleVendor,
			canDecline: false,
		},
		{
			// Non nil decline
			setup: func(order *Order) error {
				order.SerializedOrderDecline = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canDecline: false,
		},
		{
			// Non nil cancel
			setup: func(order *Order) error {
				order.SerializedOrderCancel = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
		{
			// Non nil confirmation
			setup: func(order *Order) error {
				order.SerializedOrderConfirmation = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
		{
			// Non nil fulfillment
			setup: func(order *Order) error {
				order.SerializedOrderFulfillments = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
		{
			// Non nil complete
			setup: func(order *Order) error {
				order.SerializedOrderComplete = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
		{
			// Non nil dispute open
			setup: func(order *Order) error {
				order.SerializedDisputeOpen = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
		{
			// Non nil dispute close
			setup: func(order *Order) error {
				order.SerializedDisputeClosed = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
		{
			// Non nil dispute update
			setup: func(order *Order) error {
				order.SerializedDisputeUpdate = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
		{
			// Non nil refund
			setup: func(order *Order) error {
				order.SerializedRefunds = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
		{
			// Non nil payment finalized
			setup: func(order *Order) error {
				order.SerializedPaymentFinalized = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canDecline: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}
		order.SetRole(test.ourRole)

		canDecline := order.CanDecline()
		if canDecline != test.canDecline {
			t.Errorf("Test %d: Got incorrect result. Expected %t, got %t", i, test.canDecline, canDecline)
		}
	}
}

func TestOrder_CanDispute(t *testing.T) {
	tests := []struct {
		setup      func(order *Order) error
		ourRole    OrderRole
		canDispute bool
	}{
		{
			// Success vendor
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}

				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
				if err != nil {
					return err
				}

				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
					Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
						{
							ItemIndex: 0,
						},
					},
				}))
				if err != nil {
					return err
				}
				return err
			},
			ourRole:    RoleVendor,
			canDispute: true,
		},
		{
			// Success buyer
			setup: func(order *Order) error {
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
			},
			ourRole:    RoleBuyer,
			canDispute: true,
		},
		{
			// OrderOpen is nil
			setup: func(order *Order) error {
				return nil
			},
			ourRole:    RoleVendor,
			canDispute: false,
		},
		{
			// Not Buyer or vendor
			setup: func(order *Order) error {
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
			},
			ourRole:    RoleModerator,
			canDispute: false,
		},
		{
			// Not fulfilled
			setup: func(order *Order) error {
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
			},
			ourRole:    RoleVendor,
			canDispute: false,
		},
		{
			// Order is complete
			setup: func(order *Order) error {
				order.SerializedOrderComplete = []byte{0x00}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
			},
			ourRole:    RoleBuyer,
			canDispute: false,
		},
		{
			// Order is finalized
			setup: func(order *Order) error {
				order.SerializedPaymentFinalized = []byte{0x00}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
			},
			ourRole:    RoleBuyer,
			canDispute: false,
		},
		{
			// Under active dispute
			setup: func(order *Order) error {
				order.SerializedDisputeOpen = []byte{0x00}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
			},
			ourRole:    RoleBuyer,
			canDispute: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}
		order.SetRole(test.ourRole)

		canDispute := order.CanDispute()
		if canDispute != test.canDispute {
			t.Errorf("Test %d: Got incorrect result. Expected %t, got %t", i, test.canDispute, canDispute)
		}
	}
}

func TestOrder_CanRefund(t *testing.T) {
	tests := []struct {
		setup     func(order *Order) error
		ourRole   OrderRole
		canRefund bool
	}{
		{
			// Success
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}

				return order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
			},
			ourRole:   RoleVendor,
			canRefund: true,
		},
		{
			// Nil buyerID
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:   RoleVendor,
			canRefund: false,
		},
		{
			// Is buyer
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}

				return order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
			},
			ourRole:   RoleBuyer,
			canRefund: false,
		},
		{
			// Order is nil
			setup: func(order *Order) error {
				return nil
			},
			ourRole:   RoleVendor,
			canRefund: false,
		},
		{
			// Cancelable - vendor can refund from 1-of-2 address
			setup: func(order *Order) error {
				order.SerializedOrderDecline = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}

				return order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_CANCELABLE,
				}))
			},
			ourRole:   RoleVendor,
			canRefund: true,
		},
		{
			// Non nil cancel
			setup: func(order *Order) error {
				order.SerializedOrderCancel = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}

				return order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
			},
			ourRole:   RoleVendor,
			canRefund: false,
		},
		{
			// Non nil complete
			setup: func(order *Order) error {
				order.SerializedOrderComplete = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}

				return order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
			},
			ourRole:   RoleVendor,
			canRefund: false,
		},
		{
			// Non nil payment finalized
			setup: func(order *Order) error {
				order.SerializedPaymentFinalized = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}

				return order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
			},
			ourRole:   RoleVendor,
			canRefund: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}
		order.SetRole(test.ourRole)

		canRefund := order.CanRefund()
		if canRefund != test.canRefund {
			t.Errorf("Test %d: Got incorrect result. Expected %t, got %t", i, test.canRefund, canRefund)
		}
	}
}

func TestOrder_CanFulfill(t *testing.T) {
	tests := []struct {
		setup      func(order *Order) error
		ourRole    OrderRole
		canFulfill bool
	}{
		{
			// Success
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderConfirmation{}))
				return err
			},
			ourRole:    RoleVendor,
			canFulfill: true,
		},
		{
			// Unfunded
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
					Amount: iwallet.NewAmount(1).String(),
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderConfirmation{}))
				return err
			},
			ourRole:    RoleVendor,
			canFulfill: false,
		},
		{
			// Already fulfilled
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
					Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
						{
							ItemIndex: 0,
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderConfirmation{}))
				return err
			},
			ourRole:    RoleVendor,
			canFulfill: false,
		},
		{
			// Is buyer
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
				return err
			},
			ourRole:    RoleBuyer,
			canFulfill: false,
		},
		{
			// Order is nil
			setup: func(order *Order) error {
				return nil
			},
			ourRole:    RoleVendor,
			canFulfill: false,
		},
		{
			// Nil order confirmation
			setup: func(order *Order) error {
				order.SerializedOrderDecline = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_CANCELABLE,
				}))
				return err
			},
			ourRole:    RoleVendor,
			canFulfill: false,
		},
		{
			// Non nil cancel
			setup: func(order *Order) error {
				order.SerializedOrderCancel = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderConfirmation{}))
				return err
			},
			ourRole:    RoleVendor,
			canFulfill: false,
		},
		{
			// Non nil complete
			setup: func(order *Order) error {
				order.SerializedOrderComplete = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderConfirmation{}))
				return err
			},
			ourRole:    RoleVendor,
			canFulfill: false,
		},
		{
			// Non nil payment finalized
			setup: func(order *Order) error {
				order.SerializedPaymentFinalized = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderConfirmation{}))
				return err
			},
			ourRole:    RoleVendor,
			canFulfill: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}
		order.SetRole(test.ourRole)

		canFulfill := order.CanFulfill()
		if canFulfill != test.canFulfill {
			t.Errorf("Test %d: Got incorrect result. Expected %t, got %t", i, test.canFulfill, canFulfill)
		}
	}
}

func TestOrder_CanConfirm(t *testing.T) {
	tests := []struct {
		setup      func(order *Order) error
		ourRole    OrderRole
		canConfirm bool
	}{
		{
			// Success - OrderOpen + PaymentSent must both exist for vendor to confirm
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Method: pb.PaymentSent_DIRECT,
				}))
			},
			ourRole:    RoleVendor,
			canConfirm: true,
		},
		{
			// Is buyer
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleBuyer,
			canConfirm: false,
		},
		{
			// Order is nil
			setup: func(order *Order) error {
				return nil
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil decline
			setup: func(order *Order) error {
				order.SerializedOrderDecline = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil cancel
			setup: func(order *Order) error {
				order.SerializedOrderCancel = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil confirmation
			setup: func(order *Order) error {
				order.SerializedOrderConfirmation = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil fulfillment
			setup: func(order *Order) error {
				order.SerializedOrderFulfillments = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil complete
			setup: func(order *Order) error {
				order.SerializedOrderComplete = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil dispute open
			setup: func(order *Order) error {
				order.SerializedDisputeOpen = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil dispute close
			setup: func(order *Order) error {
				order.SerializedDisputeClosed = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil dispute update
			setup: func(order *Order) error {
				order.SerializedDisputeUpdate = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil refund
			setup: func(order *Order) error {
				order.SerializedRefunds = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
		{
			// Non nil payment finalized
			setup: func(order *Order) error {
				order.SerializedPaymentFinalized = []byte{0x00}
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:    RoleVendor,
			canConfirm: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}
		order.SetRole(test.ourRole)

		canConfirm := order.CanConfirm()
		if canConfirm != test.canConfirm {
			t.Errorf("Test %d: Got incorrect result. Expected %t, got %t", i, test.canConfirm, canConfirm)
		}
	}
}

func TestOrder_CanComplete(t *testing.T) {
	tests := []struct {
		setup       func(order *Order) error
		ourRole     OrderRole
		canComplete bool
	}{
		{
			// Success
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
					Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
						{
							ItemIndex: 0,
						},
						{
							ItemIndex: 1,
						},
					},
				}))
			},
			ourRole:     RoleBuyer,
			canComplete: true,
		},
		{
			// Is Vendor
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				return err
			},
			ourRole:     RoleVendor,
			canComplete: false,
		},
		{
			// Order is nil
			setup: func(order *Order) error {
				return nil
			},
			ourRole:     RoleBuyer,
			canComplete: false,
		},
		{
			// Not fulfilled
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "",
						},
					},
				}))
				return err
			},
			ourRole:     RoleBuyer,
			canComplete: false,
		},
		{
			// Completed
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				order.SerializedOrderComplete = []byte{0x00}
				if err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
					Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
						{
							ItemIndex: 0,
						},
						{
							ItemIndex: 1,
						},
					},
				}))
			},
			ourRole:     RoleBuyer,
			canComplete: false,
		},
		{
			// Payment Finalized
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				order.SerializedPaymentFinalized = []byte{0x00}
				if err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
					Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
						{
							ItemIndex: 0,
						},
						{
							ItemIndex: 1,
						},
					},
				}))
			},
			ourRole:     RoleBuyer,
			canComplete: false,
		},
		{
			// Dispute Open
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				order.SerializedDisputeOpen = []byte{0x00}
				if err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
					Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
						{
							ItemIndex: 0,
						},
						{
							ItemIndex: 1,
						},
					},
				}))
			},
			ourRole:     RoleBuyer,
			canComplete: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}

		order.SetRole(test.ourRole)

		canComplete := order.CanComplete()
		if canComplete != test.canComplete {
			t.Errorf("Test %d: Got incorrect result. Expected %t, got %t", i, test.canComplete, canComplete)
		}
	}
}

func TestOrder_IsFunded(t *testing.T) {
	tests := []struct {
		setup    func(order *Order) error
		isFunded bool
	}{
		// Funded
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Amount:    "1000",
					ToAddress: "aaaaaa",
				}))
				if err != nil {
					return err
				}

				return order.PutTransaction(iwallet.Transaction{
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress("aaaaaa", iwallet.CtMock),
							Amount:  iwallet.NewAmount("1000"),
						},
					},
				})
			},
			isFunded: true,
		},
		// Multiple payments
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Amount:    "1000",
					ToAddress: "aaaaaa",
				}))
				if err != nil {
					return err
				}
				err = order.PutTransaction(iwallet.Transaction{
					ID: "123",
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress("aaaaaa", iwallet.CtMock),
							Amount:  iwallet.NewAmount("100"),
						},
					},
				})
				if err != nil {
					return err
				}

				return order.PutTransaction(iwallet.Transaction{
					ID: "456",
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress("aaaaaa", iwallet.CtMock),
							Amount:  iwallet.NewAmount("900"),
						},
					},
				})
			},
			isFunded: true,
		},
		// Short
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Amount:    "1000",
					ToAddress: "aaaaaa",
				}))
				if err != nil {
					return err
				}

				return order.PutTransaction(iwallet.Transaction{
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress("aaaaaa", iwallet.CtMock),
							Amount:  iwallet.NewAmount("100"),
						},
					},
				})
			},
			isFunded: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}

		isFunded, err := order.IsFunded()
		if err != nil {
			t.Errorf("Test %d: Is funded error: %s", i, err)
		}
		if isFunded != test.isFunded {
			t.Errorf("Got incorrect result. Expected %t, got %t", test.isFunded, isFunded)
		}
	}
}

func TestOrder_IsFulfilled(t *testing.T) {
	tests := []struct {
		setup       func(order *Order) error
		isFulfilled bool
	}{
		// Fulfilled
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
						{
							ListingHash: "123",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Amount:    "1000",
					ToAddress: "aaaaaa",
				}))
				if err != nil {
					return err
				}

				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
					Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
						{
							ItemIndex: 0,
						},
						{
							ItemIndex: 1,
						},
					},
				}))
			},
			isFulfilled: true,
		},
		// Only one fulfilled.
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
						{
							ListingHash: "123",
						},
					},
				}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Amount:    "1000",
					ToAddress: "aaaaaa",
				}))
				if err != nil {
					return err
				}

				return order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderFulfillment{
					Fulfillments: []*pb.OrderFulfillment_FulfilledItem{
						{
							ItemIndex: 0,
						},
					},
				}))
			},
			isFulfilled: false,
		},
		// No fulfillments
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
					Items: []*pb.OrderOpen_Item{
						{
							ListingHash: "abc",
						},
						{
							ListingHash: "123",
						},
					},
				}))
				if err != nil {
					return err
				}
				return order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Amount:    "1000",
					ToAddress: "aaaaaa",
				}))
			},
			isFulfilled: false,
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}

		isFulfilled, err := order.IsFulfilled()
		if err != nil {
			t.Errorf("Test %d: Is fufilled error: %s", i, err)
		}
		if isFulfilled != test.isFulfilled {
			t.Errorf("Got incorrect result. Expected %t, got %t", test.isFulfilled, isFulfilled)
		}
	}
}

func TestOrder_FundingTotal(t *testing.T) {
	tests := []struct {
		setup func(order *Order) error
		total iwallet.Amount
	}{
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					ToAddress: "aaaaaa",
				}))
				if err != nil {
					return err
				}

				return order.PutTransaction(iwallet.Transaction{
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress("aaaaaa", iwallet.CtMock),
							Amount:  iwallet.NewAmount("1000"),
						},
					},
				})
			},
			total: iwallet.NewAmount(1000),
		},
		// Multiple payments
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Amount:    "1000",
					ToAddress: "aaaaaa",
				}))
				if err != nil {
					return err
				}
				err = order.PutTransaction(iwallet.Transaction{
					ID: "123",
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress("aaaaaa", iwallet.CtMock),
							Amount:  iwallet.NewAmount("100"),
						},
					},
				})
				if err != nil {
					return err
				}

				return order.PutTransaction(iwallet.Transaction{
					ID: "456",
					To: []iwallet.SpendInfo{
						{
							Address: iwallet.NewAddress("aaaaaa", iwallet.CtMock),
							Amount:  iwallet.NewAmount("900"),
						},
					},
				})
			},
			total: iwallet.NewAmount(1000),
		},
		// No payments
		{
			setup: func(order *Order) error {
				err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{}))
				if err != nil {
					return err
				}
				err = order.PutMessage(utils.MustWrapOrderMessage(&pb.PaymentSent{
					Amount:    "1000",
					ToAddress: "aaaaaa",
				}))
				if err != nil {
					return err
				}
				return nil
			},
			total: iwallet.NewAmount(0),
		},
	}

	for i, test := range tests {
		var order Order
		if err := test.setup(&order); err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
		}

		total, err := order.FundingTotal()
		if err != nil {
			t.Errorf("Test %d: Is funded error: %s", i, err)
		}
		if total.Cmp(test.total) != 0 {
			t.Errorf("Got incorrect result. Expected %s, got %s", test.total, total)
		}
	}
}
