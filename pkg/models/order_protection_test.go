package models

import (
	"testing"
	"time"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestDefaultProtectionPolicy_PhysicalGood(t *testing.T) {
	p := DefaultProtectionPolicy(pb.Listing_Metadata_PHYSICAL_GOOD)
	if p.AutoCompleteAfterShipDays != 14 {
		t.Errorf("AutoCompleteAfterShipDays = %d, want 14", p.AutoCompleteAfterShipDays)
	}
	if p.MaxShipDays != 7 {
		t.Errorf("MaxShipDays = %d, want 7", p.MaxShipDays)
	}
	if p.ExtendProtectionDays != 14 {
		t.Errorf("ExtendProtectionDays = %d, want 14", p.ExtendProtectionDays)
	}
}

func TestDefaultProtectionPolicy_DigitalGood(t *testing.T) {
	p := DefaultProtectionPolicy(pb.Listing_Metadata_DIGITAL_GOOD)
	if p.AutoCompleteAfterShipDays != 3 {
		t.Errorf("AutoCompleteAfterShipDays = %d, want 3", p.AutoCompleteAfterShipDays)
	}
	if p.MaxShipDays != 3 {
		t.Errorf("MaxShipDays = %d, want 3", p.MaxShipDays)
	}
	if p.ExtendProtectionDays != 0 {
		t.Errorf("ExtendProtectionDays = %d, want 0", p.ExtendProtectionDays)
	}
}

func TestDefaultProtectionPolicy_Service(t *testing.T) {
	p := DefaultProtectionPolicy(pb.Listing_Metadata_SERVICE)
	if p.AutoCompleteAfterShipDays != 7 {
		t.Errorf("AutoCompleteAfterShipDays = %d, want 7", p.AutoCompleteAfterShipDays)
	}
	if p.MaxShipDays != 3 {
		t.Errorf("MaxShipDays = %d, want 3", p.MaxShipDays)
	}
	if p.ExtendProtectionDays != 0 {
		t.Errorf("ExtendProtectionDays = %d, want 0", p.ExtendProtectionDays)
	}
}

func TestDefaultProtectionPolicy_UnknownFallsBackToPhysical(t *testing.T) {
	p := DefaultProtectionPolicy(pb.Listing_Metadata_ContractType(99))
	expected := DefaultProtectionPolicy(pb.Listing_Metadata_PHYSICAL_GOOD)
	if p.AutoCompleteAfterShipDays != expected.AutoCompleteAfterShipDays ||
		p.MaxShipDays != expected.MaxShipDays ||
		p.AfterSaleWindowDays != expected.AfterSaleWindowDays ||
		p.ExtendProtectionDays != expected.ExtendProtectionDays ||
		p.DisputeNegotiationDays != expected.DisputeNegotiationDays ||
		p.DisputeResolutionDays != expected.DisputeResolutionDays {
		t.Errorf("unknown contract type should fall back to PHYSICAL_GOOD defaults")
	}
}

func TestDefaultProtectionPolicy_SharedValues(t *testing.T) {
	for _, ct := range []pb.Listing_Metadata_ContractType{
		pb.Listing_Metadata_PHYSICAL_GOOD,
		pb.Listing_Metadata_DIGITAL_GOOD,
		pb.Listing_Metadata_SERVICE,
	} {
		p := DefaultProtectionPolicy(ct)
		if p.AfterSaleWindowDays != 7 {
			t.Errorf("ct=%v: AfterSaleWindowDays = %d, want 7", ct, p.AfterSaleWindowDays)
		}
		if p.DisputeNegotiationDays != 7 {
			t.Errorf("ct=%v: DisputeNegotiationDays = %d, want 7", ct, p.DisputeNegotiationDays)
		}
		if p.DisputeResolutionDays != 7 {
			t.Errorf("ct=%v: DisputeResolutionDays = %d, want 7", ct, p.DisputeResolutionDays)
		}
	}
}

func TestDefaultProtectionPolicy_ReminderBeforeDays(t *testing.T) {
	tests := []struct {
		name     string
		ct       pb.Listing_Metadata_ContractType
		wantDays []int
	}{
		{"physical", pb.Listing_Metadata_PHYSICAL_GOOD, []int{3, 1}},
		{"digital", pb.Listing_Metadata_DIGITAL_GOOD, []int{1}},
		{"service", pb.Listing_Metadata_SERVICE, []int{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := DefaultProtectionPolicy(tt.ct)
			if len(p.ReminderBeforeDays) != len(tt.wantDays) {
				t.Fatalf("ReminderBeforeDays len = %d, want %d", len(p.ReminderBeforeDays), len(tt.wantDays))
			}
			for i, d := range p.ReminderBeforeDays {
				if d != tt.wantDays[i] {
					t.Errorf("ReminderBeforeDays[%d] = %d, want %d", i, d, tt.wantDays[i])
				}
			}
		})
	}
}

func TestOrderProtectionPolicy_DurationHelpers(t *testing.T) {
	p := DefaultProtectionPolicy(pb.Listing_Metadata_PHYSICAL_GOOD)
	if got := p.AutoCompleteDuration(); got != 14*24*time.Hour {
		t.Errorf("AutoCompleteDuration = %v, want %v", got, 14*24*time.Hour)
	}
	if got := p.MaxShipDuration(); got != 7*24*time.Hour {
		t.Errorf("MaxShipDuration = %v, want %v", got, 7*24*time.Hour)
	}
	if got := p.AfterSaleWindowDuration(); got != 7*24*time.Hour {
		t.Errorf("AfterSaleWindowDuration = %v, want %v", got, 7*24*time.Hour)
	}
}

// ── ComputeProtection tests ──────────────────────────────────────────────

func TestComputeProtection_NilForInapplicableStates(t *testing.T) {
	for _, state := range []OrderState{
		OrderState_PENDING,
		OrderState_AWAITING_PAYMENT,
		OrderState_AWAITING_PAYMENT_VERIFICATION,
		OrderState_CANCELED,
		OrderState_DECLINED,
		OrderState_REFUNDED,
		OrderState_RESOLVED,
		OrderState_PROCESSING_ERROR,
	} {
		o := &Order{State: state}
		if got := o.ComputeProtection(time.Now()); got != nil {
			t.Errorf("state=%s: expected nil, got %+v", state, got)
		}
	}
}

func TestComputeProtection_AwaitingShipment(t *testing.T) {
	o := &Order{State: OrderState_AWAITING_SHIPMENT}
	info := o.ComputeProtection(time.Now())
	if info == nil {
		t.Fatal("expected non-nil protection info")
	}
	if info.Stage != ProtectionStageEscrowed {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageEscrowed)
	}
	if info.DaysRemaining != 0 {
		t.Errorf("daysRemaining = %d, want 0", info.DaysRemaining)
	}
	if info.Extendable {
		t.Error("extendable should be false for ESCROWED")
	}
	if info.AfterSaleWindowDays != 7 {
		t.Errorf("afterSaleWindowDays = %d, want 7", info.AfterSaleWindowDays)
	}
}

func TestComputeProtection_AwaitingShipmentCancelableReleased(t *testing.T) {
	o := &Order{State: OrderState_AWAITING_SHIPMENT}
	if err := o.SetPaymentSent(&pb.PaymentSent{
		Coin:           "crypto:eip155:1:native",
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
	}); err != nil {
		t.Fatalf("SetPaymentSent: %v", err)
	}
	raw, err := protojson.Marshal(&pb.OrderConfirmation{TransactionID: "0xrelease"})
	if err != nil {
		t.Fatalf("marshal OrderConfirmation: %v", err)
	}
	o.SerializedOrderConfirmation = raw

	if info := o.ComputeProtection(time.Now()); info != nil {
		t.Fatalf("expected no buyer-protection hold after cancelable settlement release, got %+v", info)
	}
}

func TestComputeProtection_Shipped_WithTimestamp(t *testing.T) {
	shippedAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State:          OrderState_SHIPPED,
		OrderLifecycle: OrderLifecycle{ShippedAt: &shippedAt},
	}

	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	info := o.ComputeProtection(now)
	if info == nil {
		t.Fatal("expected non-nil protection info")
	}
	if info.Stage != ProtectionStageProtectionPeriod {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageProtectionPeriod)
	}
	if info.DaysRemaining != 10 {
		t.Errorf("daysRemaining = %d, want 10 (14 - 4 days elapsed)", info.DaysRemaining)
	}
	if info.AutoCompleteAt == nil {
		t.Fatal("autoCompleteAt should not be nil")
	}
	expectedDeadline := shippedAt.Add(14 * 24 * time.Hour)
	if !info.AutoCompleteAt.Equal(expectedDeadline) {
		t.Errorf("autoCompleteAt = %v, want %v", info.AutoCompleteAt, expectedDeadline)
	}
	if !info.Extendable {
		t.Error("physical goods should be extendable")
	}
}

func TestComputeProtection_Shipped_WithoutTimestamp(t *testing.T) {
	o := &Order{
		State:          OrderState_SHIPPED,
		OrderLifecycle: OrderLifecycle{ShippedAt: nil},
	}
	info := o.ComputeProtection(time.Now())
	if info == nil {
		t.Fatal("expected non-nil")
	}
	if info.DaysRemaining != 14 {
		t.Errorf("daysRemaining = %d, want 14 (fallback to full policy)", info.DaysRemaining)
	}
	if info.AutoCompleteAt != nil {
		t.Error("autoCompleteAt should be nil when shippedAt is missing")
	}
}

func TestComputeProtection_Shipped_DeadlinePassed(t *testing.T) {
	shippedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	o := &Order{
		State:          OrderState_SHIPPED,
		OrderLifecycle: OrderLifecycle{ShippedAt: &shippedAt},
	}
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	info := o.ComputeProtection(now)
	if info.DaysRemaining != 0 {
		t.Errorf("daysRemaining = %d, want 0 (deadline passed)", info.DaysRemaining)
	}
}

func TestComputeProtection_Completed_InAfterSaleWindow(t *testing.T) {
	completedAt := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State:          OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{CompletedAt: &completedAt},
	}
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	info := o.ComputeProtection(now)
	if info == nil {
		t.Fatal("expected non-nil")
	}
	if info.Stage != ProtectionStageAfterSaleWindow {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageAfterSaleWindow)
	}
	if info.DaysRemaining != 3 {
		t.Errorf("daysRemaining = %d, want 3 (7 - 4 elapsed)", info.DaysRemaining)
	}
}

func TestComputeProtection_Completed_PastAfterSaleWindow(t *testing.T) {
	completedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	o := &Order{
		State:          OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{CompletedAt: &completedAt},
	}
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	info := o.ComputeProtection(now)
	if info.Stage != ProtectionStageCompleted {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageCompleted)
	}
	if info.DaysRemaining != 0 {
		t.Errorf("daysRemaining = %d, want 0", info.DaysRemaining)
	}
}

func TestComputeProtection_Completed_NoTimestamp(t *testing.T) {
	o := &Order{
		State:          OrderState_COMPLETED,
		OrderLifecycle: OrderLifecycle{CompletedAt: nil},
	}
	info := o.ComputeProtection(time.Now())
	if info.Stage != ProtectionStageCompleted {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageCompleted)
	}
}

func TestComputeProtection_Disputed(t *testing.T) {
	o := &Order{State: OrderState_DISPUTED}
	info := o.ComputeProtection(time.Now())
	if info == nil {
		t.Fatal("expected non-nil")
	}
	if info.Stage != ProtectionStageDisputed {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageDisputed)
	}
}

func TestComputeProtection_Decided(t *testing.T) {
	o := &Order{State: OrderState_DECIDED}
	info := o.ComputeProtection(time.Now())
	if info == nil {
		t.Fatal("expected non-nil")
	}
	if info.Stage != ProtectionStageDisputed {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageDisputed)
	}
}

func TestComputeProtection_PaymentFinalized(t *testing.T) {
	completedAt := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State:          OrderState_PAYMENT_FINALIZED,
		OrderLifecycle: OrderLifecycle{CompletedAt: &completedAt},
	}
	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	info := o.ComputeProtection(now)
	if info.Stage != ProtectionStageAfterSaleWindow {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageAfterSaleWindow)
	}
}

// ── Extended Protection tests ──────────────────────────────────────────

func TestComputeProtection_Shipped_Extended_WithTimestamp(t *testing.T) {
	shippedAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	extendedAt := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State: OrderState_SHIPPED,
		OrderLifecycle: OrderLifecycle{
			ShippedAt:            &shippedAt,
			ProtectionExtendedAt: &extendedAt,
		},
	}

	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	info := o.ComputeProtection(now)
	if info == nil {
		t.Fatal("expected non-nil protection info")
	}
	if info.Stage != ProtectionStageProtectionPeriod {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageProtectionPeriod)
	}
	// 14 base + 14 extended = 28 days total from shippedAt
	// 28 - 9 elapsed = 19 days remaining
	if info.DaysRemaining != 19 {
		t.Errorf("daysRemaining = %d, want 19 (28 - 9 days elapsed)", info.DaysRemaining)
	}
	expectedDeadline := shippedAt.Add(28 * 24 * time.Hour)
	if info.AutoCompleteAt == nil || !info.AutoCompleteAt.Equal(expectedDeadline) {
		t.Errorf("autoCompleteAt = %v, want %v", info.AutoCompleteAt, expectedDeadline)
	}
	if info.Extendable {
		t.Error("should not be extendable after extension")
	}
	if !info.Extended {
		t.Error("extended should be true")
	}
}

func TestComputeProtection_Shipped_Extended_WithoutTimestamp(t *testing.T) {
	extendedAt := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State: OrderState_SHIPPED,
		OrderLifecycle: OrderLifecycle{
			ShippedAt:            nil,
			ProtectionExtendedAt: &extendedAt,
		},
	}
	info := o.ComputeProtection(time.Now())
	if info == nil {
		t.Fatal("expected non-nil")
	}
	// 14 + 14 = 28 total days when no timestamp
	if info.DaysRemaining != 28 {
		t.Errorf("daysRemaining = %d, want 28 (14 base + 14 extended)", info.DaysRemaining)
	}
	if info.Extendable {
		t.Error("should not be extendable after extension")
	}
	if !info.Extended {
		t.Error("extended should be true")
	}
}

func TestComputeProtection_Shipped_NotExtended_Extendable(t *testing.T) {
	shippedAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State:          OrderState_SHIPPED,
		OrderLifecycle: OrderLifecycle{ShippedAt: &shippedAt},
	}
	info := o.ComputeProtection(time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC))
	if !info.Extendable {
		t.Error("physical goods should be extendable when not yet extended")
	}
	if info.Extended {
		t.Error("extended should be false when not extended")
	}
}

// ── ResolvePolicyForOrder tests (DG-1.11) ────────────────────────────────

// makeContractOrder builds an Order whose ContractType() returns the given
// type by serialising a minimal OrderOpen into SerializedOrderOpen via the
// package-shared protojson marshaler.
func makeContractOrder(t *testing.T, ct pb.Listing_Metadata_ContractType, override uint32) *Order {
	t.Helper()
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{Listing: &pb.Listing{Metadata: &pb.Listing_Metadata{ContractType: ct}}},
		},
	}
	serialized, err := marshaler.Marshal(orderOpen)
	if err != nil {
		t.Fatalf("marshal OrderOpen: %v", err)
	}
	return &Order{
		SerializedOrderOpen: serialized,
		OrderLifecycle:      OrderLifecycle{AutoCompleteAfterShipDaysOverride: override},
	}
}

func TestResolvePolicyForOrder_NilOrder(t *testing.T) {
	p := ResolvePolicyForOrder(nil)
	expected := DefaultProtectionPolicy(pb.Listing_Metadata_PHYSICAL_GOOD)
	if p.AutoCompleteAfterShipDays != expected.AutoCompleteAfterShipDays {
		t.Errorf("nil order: AutoCompleteAfterShipDays = %d, want %d",
			p.AutoCompleteAfterShipDays, expected.AutoCompleteAfterShipDays)
	}
}

func TestResolvePolicyForOrder_DigitalGoodNoOverride(t *testing.T) {
	o := makeContractOrder(t, pb.Listing_Metadata_DIGITAL_GOOD, 0)
	p := ResolvePolicyForOrder(o)
	if p.AutoCompleteAfterShipDays != 3 {
		t.Errorf("AutoCompleteAfterShipDays = %d, want 3 (DIGITAL_GOOD default)", p.AutoCompleteAfterShipDays)
	}
}

func TestResolvePolicyForOrder_DigitalGoodExtendOverride(t *testing.T) {
	// Override is honoured only when it EXTENDS the buyer-protection window.
	for _, override := range []uint32{4, 5, 7} {
		o := makeContractOrder(t, pb.Listing_Metadata_DIGITAL_GOOD, override)
		p := ResolvePolicyForOrder(o)
		if p.AutoCompleteAfterShipDays != int(override) {
			t.Errorf("override=%d: AutoCompleteAfterShipDays = %d, want %d",
				override, p.AutoCompleteAfterShipDays, override)
		}
		// Other fields must remain at the DIGITAL_GOOD defaults — override
		// should only touch AutoCompleteAfterShipDays.
		if p.MaxShipDays != 3 {
			t.Errorf("override=%d: MaxShipDays = %d, want 3", override, p.MaxShipDays)
		}
		if p.AfterSaleWindowDays != 7 {
			t.Errorf("override=%d: AfterSaleWindowDays = %d, want 7", override, p.AfterSaleWindowDays)
		}
	}
}

func TestResolvePolicyForOrder_DigitalGoodShortenOverrideClampedToDefault(t *testing.T) {
	// Trust safety: override cannot SHORTEN the buyer-protection window.
	// A value below the ContractType default is silently clamped — buyers
	// always get at least the default 3-day window.
	for _, override := range []uint32{1, 2, 3} {
		o := makeContractOrder(t, pb.Listing_Metadata_DIGITAL_GOOD, override)
		p := ResolvePolicyForOrder(o)
		if p.AutoCompleteAfterShipDays != 3 {
			t.Errorf("override=%d: AutoCompleteAfterShipDays = %d, want 3 (clamped to default)",
				override, p.AutoCompleteAfterShipDays)
		}
	}
}

func TestResolvePolicyForOrder_PhysicalGoodIgnoresOverride(t *testing.T) {
	// Override snapshot should never apply to PHYSICAL_GOOD orders even if
	// (somehow) populated — Phase 1.1 ships DIGITAL_GOOD only.
	o := makeContractOrder(t, pb.Listing_Metadata_PHYSICAL_GOOD, 5)
	p := ResolvePolicyForOrder(o)
	if p.AutoCompleteAfterShipDays != 14 {
		t.Errorf("PHYSICAL_GOOD with override: AutoCompleteAfterShipDays = %d, want 14",
			p.AutoCompleteAfterShipDays)
	}
}

func TestComputeProtection_DigitalGoodOverrideExtendsWindow(t *testing.T) {
	shippedAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	o := makeContractOrder(t, pb.Listing_Metadata_DIGITAL_GOOD, 5) // 5d window (extended from default 3)
	o.State = OrderState_SHIPPED
	o.ShippedAt = &shippedAt

	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC) // 24h after shipment
	info := o.ComputeProtection(now)
	if info == nil {
		t.Fatal("expected non-nil protection info")
	}
	if info.AutoCompleteAt == nil {
		t.Fatal("expected non-nil AutoCompleteAt")
	}
	expectedDeadline := shippedAt.Add(5 * 24 * time.Hour) // 5 days, not 3
	if !info.AutoCompleteAt.Equal(expectedDeadline) {
		t.Errorf("AutoCompleteAt = %v, want %v", info.AutoCompleteAt, expectedDeadline)
	}
	if info.DaysRemaining != 4 {
		t.Errorf("DaysRemaining = %d, want 4 (5d - 1d elapsed)", info.DaysRemaining)
	}
}

func TestDaysUntil(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		deadline time.Time
		want     int
	}{
		{
			"exactly 3 days",
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC),
			3,
		},
		{
			"3 days and 1 hour rounds up to 4",
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 4, 1, 0, 0, 0, time.UTC),
			4,
		},
		{
			"1 hour remaining rounds up to 1",
			time.Date(2026, 3, 1, 23, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
			1,
		},
		{
			"deadline passed",
			time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			0,
		},
		{
			"deadline is now",
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := daysUntil(tt.now, tt.deadline)
			if got != tt.want {
				t.Errorf("daysUntil() = %d, want %d", got, tt.want)
			}
		})
	}
}
