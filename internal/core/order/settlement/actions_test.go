//go:build !private_distribution

package settlement

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestEscrowUsesBackendSubmittedRelease(t *testing.T) {
	cases := []struct {
		escrowType string
		want       bool
	}{
		{string(payment.EscrowTypeUTXOScript), true},
		{string(payment.EscrowTypeManagedEscrow), true},
		{string(payment.EscrowTypeSolanaEscrow), true},
		{"none", false},
	}
	for _, tc := range cases {
		spec := payment.SettlementSpec{EscrowType: payment.EscrowType(tc.escrowType)}
		if got := EscrowUsesBackendSubmittedRelease(spec); got != tc.want {
			t.Fatalf("EscrowUsesBackendSubmittedRelease(%q) = %v, want %v", tc.escrowType, got, tc.want)
		}
	}
}

func TestActionTxHash_SkipsNewerInFlightToOlderConfirmed(t *testing.T) {
	order := &models.Order{
		ID: models.OrderID("order-1"),
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "complete", State: "failed", UpdatedAt: time.Date(2026, 6, 6, 12, 5, 0, 0, time.UTC)},
			{Action: "complete", TxHash: "0xconfirmed", State: "confirmed", UpdatedAt: time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)},
		},
	}
	if got := ActionTxHash(order, "complete"); got != "0xconfirmed" {
		t.Fatalf("ActionTxHash = %q, want 0xconfirmed", got)
	}
	txid, submitted, err := EvaluateRelease(order, "", "complete")
	if err != nil {
		t.Fatalf("EvaluateRelease: %v", err)
	}
	if !submitted || string(txid) != "0xconfirmed" {
		t.Fatalf("ready release = (%q, %v), want (0xconfirmed, true)", txid, submitted)
	}
}

func TestEvaluateRelease_PendingAndReady(t *testing.T) {
	order := &models.Order{
		ID: models.OrderID("order-1"),
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "complete", State: "submitted"},
		},
	}
	_, _, err := EvaluateRelease(order, "", "complete")
	if err == nil {
		t.Fatal("expected pending release error")
	}

	order.SettlementActions = []models.SettlementActionSnapshot{
		{Action: "complete", TxHash: "abc123"},
	}
	txid, submitted, err := EvaluateRelease(order, "", "complete")
	if err != nil {
		t.Fatalf("EvaluateRelease ready: %v", err)
	}
	if !submitted || string(txid) != "abc123" {
		t.Fatalf("ready release = (%q, %v), want (abc123, true)", txid, submitted)
	}
}

func TestStaleSyncAction(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	staleAt := now.Add(-3 * time.Minute)
	if !StaleSyncAction(SyncActionID("o1", "complete"), "submitting", "", staleAt, now) {
		t.Fatal("expected stale sync action")
	}
	if StaleSyncAction("relay-action-1", "submitting", "", staleAt, now) {
		t.Fatal("non-sync action should not be stale")
	}
	if StaleSyncAction(SyncActionID("o1", "complete"), "submitting", string(iwallet.TransactionID("0x1")), staleAt, now) {
		t.Fatal("action with tx hash should not be stale")
	}
}
