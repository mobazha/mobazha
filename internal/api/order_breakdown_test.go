package api

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestBuildOrderSettlementBreakdown_ConfirmedActionLines(t *testing.T) {
	order := &models.Order{
		ID: models.OrderID("order-managed-confirm"),
		SettlementActions: []models.SettlementActionSnapshot{{
			ActionID:         "act-confirm",
			Action:           "confirm",
			SettlementAction: "confirm",
			State:            "confirmed",
			TxHash:           "0xrelease",
			SettlementCoin:   "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7",
			GrossAmount:      "292929",
			ObservedLines: []models.SettlementPayoutLine{{
				Type:    "seller",
				Amount:  "141414",
				Address: "0xb9a2226c9da66db8210edfc51ede121e977e2e39",
				Coin:    "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7",
			}, {
				Type:    "platform",
				Amount:  "151515",
				Address: "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
				Coin:    "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7",
			}},
			UpdatedAt: time.Now(),
		}},
	}
	if err := order.SetPaymentSent(&pb.PaymentSent{
		Amount:         "292929",
		Coin:           "crypto:eip155:1:erc20:0xdAC17F958D2ee523a2206206994597C13D831ec7",
		PlatformAmount: "151515",
		PlatformAddr:   "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method:     pb.PaymentSent_CANCELABLE,
			PayMode:    "address_monitored",
			EscrowType: "managed",
		},
	}); err != nil {
		t.Fatalf("SetPaymentSent: %v", err)
	}

	breakdown := buildOrderSettlementBreakdown(order)
	if breakdown == nil {
		t.Fatal("expected settlement breakdown")
	}
	if breakdown.Source != "settlement_action" {
		t.Fatalf("Source = %q, want settlement_action", breakdown.Source)
	}
	if breakdown.EscrowedAmount != "292929" {
		t.Fatalf("EscrowedAmount = %q, want 292929", breakdown.EscrowedAmount)
	}
	if breakdown.SellerAmount != "141414" {
		t.Fatalf("SellerAmount = %q, want 141414", breakdown.SellerAmount)
	}
	if breakdown.PlatformAmount != "151515" {
		t.Fatalf("PlatformAmount = %q, want 151515", breakdown.PlatformAmount)
	}
	if breakdown.PlatformAddress != "0x10d44982e0e50bcbf4c1df72f8c43497baf74668" {
		t.Fatalf("PlatformAddress = %q", breakdown.PlatformAddress)
	}
	if breakdown.SellerAddress != "0xb9a2226c9da66db8210edfc51ede121e977e2e39" {
		t.Fatalf("SellerAddress = %q", breakdown.SellerAddress)
	}
	if breakdown.TxHash != "0xrelease" {
		t.Fatalf("TxHash = %q, want 0xrelease", breakdown.TxHash)
	}
	if len(breakdown.Lines) != 2 {
		t.Fatalf("len(Lines) = %d, want 2", len(breakdown.Lines))
	}
	if breakdown.Lines[0].Type != "seller" || breakdown.Lines[0].Amount != "141414" {
		t.Fatalf("seller line = %#v", breakdown.Lines[0])
	}
	if breakdown.Lines[1].Type != "platform" || breakdown.Lines[1].Amount != "151515" {
		t.Fatalf("platform line = %#v", breakdown.Lines[1])
	}
}

func TestLatestConfirmedSettlementLines_PicksNewestConfirmedByUpdatedAt(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	actions := []models.SettlementActionSnapshot{
		{
			ActionID:         "act-confirm",
			Action:           "confirm",
			SettlementAction: "confirm",
			State:            "confirmed",
			UpdatedAt:        base,
			ObservedLines: []models.SettlementPayoutLine{{
				Type: "seller", Amount: "100", Address: "seller-old",
			}},
		},
		{
			ActionID:         "act-dispute",
			Action:           "dispute_release",
			SettlementAction: "dispute_release",
			State:            "confirmed",
			UpdatedAt:        base.Add(2 * time.Hour),
			ObservedLines: []models.SettlementPayoutLine{
				{Type: "buyer", Amount: "40", Address: "buyer-refund"},
				{Type: "seller", Amount: "50", Address: "seller-partial"},
			},
		},
	}

	action, lines := latestConfirmedSettlementLines(actions)
	if action == nil {
		t.Fatal("expected selected action")
	}
	if action.ActionID != "act-dispute" {
		t.Fatalf("ActionID = %q, want act-dispute", action.ActionID)
	}
	if len(lines) != 2 || lines[0].Amount != "40" || lines[1].Amount != "50" {
		t.Fatalf("lines = %#v", lines)
	}
}

func TestLatestConfirmedSettlementLines_IgnoresNonConfirmedAndUnknownActions(t *testing.T) {
	now := time.Now().UTC()
	actions := []models.SettlementActionSnapshot{
		{
			ActionID:  "act-relay",
			Action:    "relay_submit",
			State:     "confirmed",
			UpdatedAt: now.Add(time.Hour),
			ObservedLines: []models.SettlementPayoutLine{{
				Type: "seller", Amount: "999", Address: "ignored",
			}},
		},
		{
			ActionID:         "act-submitted",
			Action:           "dispute_release",
			SettlementAction: "dispute_release",
			State:            "submitted",
			UpdatedAt:        now.Add(2 * time.Hour),
			ObservedLines: []models.SettlementPayoutLine{{
				Type: "seller", Amount: "888", Address: "ignored",
			}},
		},
		{
			ActionID:         "act-confirm",
			Action:           "confirm",
			SettlementAction: "confirm",
			State:            "confirmed",
			UpdatedAt:        now,
			PlannedLines: []models.SettlementPayoutLine{{
				Type: "seller", Amount: "141414", Address: "seller",
			}},
		},
	}

	action, lines := latestConfirmedSettlementLines(actions)
	if action == nil || action.ActionID != "act-confirm" {
		t.Fatalf("action = %#v, want act-confirm", action)
	}
	if len(lines) != 1 || lines[0].Amount != "141414" {
		t.Fatalf("lines = %#v", lines)
	}
}

func TestBuildOrderSettlementBreakdown_PrefersNewestConfirmedOverProtobufDispute(t *testing.T) {
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	order := &models.Order{
		ID: models.OrderID("order-dispute-breakdown"),
		SettlementActions: []models.SettlementActionSnapshot{
			{
				ActionID:         "act-confirm",
				Action:           "confirm",
				SettlementAction: "confirm",
				State:            "confirmed",
				TxHash:           "0xconfirm",
				GrossAmount:      "1000",
				UpdatedAt:        base,
				ObservedLines: []models.SettlementPayoutLine{{
					Type: "seller", Amount: "900", Address: "0xseller",
				}, {
					Type: "platform", Amount: "100", Address: "0xfee",
				}},
			},
			{
				ActionID:         "act-dispute-release",
				Action:           "dispute_release",
				SettlementAction: "dispute_release",
				State:            "confirmed",
				TxHash:           "0xdispute",
				GrossAmount:      "1000",
				UpdatedAt:        base.Add(3 * time.Hour),
				ObservedLines: []models.SettlementPayoutLine{
					{Type: "buyer", Amount: "600", Address: "0xbuyer"},
					{Type: "seller", Amount: "300", Address: "0xseller"},
					{Type: "platform", Amount: "100", Address: "0xfee"},
				},
			},
			{
				ActionID:         "act-retry",
				Action:           "dispute_release",
				SettlementAction: "dispute_release",
				State:            "submitted",
				UpdatedAt:        base.Add(4 * time.Hour),
				ObservedLines: []models.SettlementPayoutLine{{
					Type: "seller", Amount: "999", Address: "0xwrong",
				}},
			},
		},
	}
	if err := order.SetPaymentSent(&pb.PaymentSent{
		Amount: "1000",
		Coin:   "USDT",
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method:     pb.PaymentSent_MODERATED,
			PayMode:    "address_monitored",
			EscrowType: "managed",
		},
	}); err != nil {
		t.Fatalf("SetPaymentSent: %v", err)
	}
	pj := protojson.MarshalOptions{}
	order.SerializedDisputeClosed = []byte(pj.Format(&pb.DisputeClose{
		ReleaseInfo: &pb.DisputeClose_ModeratedEscrowRelease{
			BuyerAmount:   "100",
			VendorAmount:  "800",
			BuyerAddress:  "0xstale-buyer",
			VendorAddress: "0xstale-seller",
		},
	}))

	breakdown := buildOrderSettlementBreakdown(order)
	if breakdown == nil {
		t.Fatal("expected settlement breakdown")
	}
	if breakdown.Source != "settlement_action" {
		t.Fatalf("Source = %q, want settlement_action", breakdown.Source)
	}
	if breakdown.TxHash != "0xdispute" {
		t.Fatalf("TxHash = %q, want 0xdispute", breakdown.TxHash)
	}
	if breakdown.BuyerAmount != "600" {
		t.Fatalf("BuyerAmount = %q, want 600", breakdown.BuyerAmount)
	}
	if breakdown.SellerAmount != "300" {
		t.Fatalf("SellerAmount = %q, want 300", breakdown.SellerAmount)
	}
	if breakdown.PlatformAmount != "100" {
		t.Fatalf("PlatformAmount = %q, want 100", breakdown.PlatformAmount)
	}
}
