package payment

import (
	"math/big"
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestCancelableAutoConfirmReady_UTXOScriptRequiresFundingFacts(t *testing.T) {
	order := &models.Order{}
	ps := &pb.PaymentSent{
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method:     pb.PaymentSent_CANCELABLE,
			PayMode:    "address_monitored",
			EscrowType: string(EscrowTypeUTXOScript),
		},
	}
	if CancelableAutoConfirmReady(order, ps) {
		t.Fatal("expected false without funding facts")
	}
	ps.FundingFacts = []*pb.PaymentSent_FundingFact{{
		Id:     "f1",
		TxHash: "abc",
		Amount: "1000",
		Status: models.PaymentObservationStatusConfirmed,
	}}
	if !CancelableAutoConfirmReady(order, ps) {
		t.Fatal("expected true with confirmed funding fact")
	}
}

func TestCancelablePaymentReadyEvent_RequiresVerifiedPayment(t *testing.T) {
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "_default"},
		ID:          "order-1",
		MyRole:      string(models.RoleVendor),
	}
	ps := &pb.PaymentSent{
		TransactionID: "tx-1",
		Coin:          "MCK",
		Amount:        "1000",
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method:     pb.PaymentSent_CANCELABLE,
			PayMode:    "address_monitored",
			EscrowType: string(EscrowTypeManaged),
		},
	}
	if got := CancelablePaymentReadyEvent(order, ps, big.NewInt(1000)); got != nil {
		t.Fatal("expected nil before payment verified")
	}
	order.MarkPaymentVerified()
	if got := CancelablePaymentReadyEvent(order, ps, big.NewInt(1000)); got == nil {
		t.Fatal("expected event after payment verified")
	}
}

func TestCancelablePaymentReadyEvent_RequiresFundingFactsForUTXO(t *testing.T) {
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "_default"},
		ID:          "order-1",
		MyRole:      string(models.RoleVendor),
	}
	order.MarkPaymentVerified()
	ps := &pb.PaymentSent{
		TransactionID: "tx-1",
		Coin:          "MCK",
		Amount:        "1000",
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method:     pb.PaymentSent_CANCELABLE,
			PayMode:    "address_monitored",
			EscrowType: string(EscrowTypeUTXOScript),
		},
	}

	if got := CancelablePaymentReadyEvent(order, ps, big.NewInt(1000)); got != nil {
		t.Fatal("expected nil without funding facts")
	}

	ps.FundingFacts = []*pb.PaymentSent_FundingFact{{
		Id:     "f1",
		TxHash: "tx-1",
		Amount: "1000",
		Status: models.PaymentObservationStatusConfirmed,
	}}
	got := CancelablePaymentReadyEvent(order, ps, big.NewInt(1000))
	if got == nil {
		t.Fatal("expected event with funding facts")
	}
	if got.OrderID != "order-1" || got.Amount != "1000" || got.TransactionID != "tx-1" {
		t.Fatalf("unexpected event: %+v", got)
	}
}

func TestCancelablePaymentReadyEvent_PreservesLargeWeiAmount(t *testing.T) {
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: "_default"},
		ID:          "order-wei",
		MyRole:      string(models.RoleVendor),
	}
	order.MarkPaymentVerified()
	ps := &pb.PaymentSent{
		TransactionID: "tx-wei",
		Coin:          "crypto:eip155:1:native",
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method:     pb.PaymentSent_CANCELABLE,
			PayMode:    "address_monitored",
			EscrowType: string(EscrowTypeManaged),
		},
	}
	large := new(big.Int)
	large.SetString("18446744073709551616", 10) // > uint64 max
	got := CancelablePaymentReadyEvent(order, ps, large)
	if got == nil {
		t.Fatal("expected event for large wei amount")
	}
	if got.Amount != large.String() {
		t.Fatalf("amount = %q, want %q", got.Amount, large.String())
	}
}
