package models

import (
	"testing"
	"time"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func TestDefaultProtectionPolicy_PhysicalGood(t *testing.T) {
	p := DefaultProtectionPolicy(pb.Listing_Metadata_PHYSICAL_GOOD)
	if p.AutoCompleteAfterShipDays != 14 {
		t.Errorf("AutoCompleteAfterShipDays = %d, want 14", p.AutoCompleteAfterShipDays)
	}
	if p.MaxFulfillDays != 7 {
		t.Errorf("MaxFulfillDays = %d, want 7", p.MaxFulfillDays)
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
	if p.MaxFulfillDays != 3 {
		t.Errorf("MaxFulfillDays = %d, want 3", p.MaxFulfillDays)
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
	if p.MaxFulfillDays != 3 {
		t.Errorf("MaxFulfillDays = %d, want 3", p.MaxFulfillDays)
	}
	if p.ExtendProtectionDays != 0 {
		t.Errorf("ExtendProtectionDays = %d, want 0", p.ExtendProtectionDays)
	}
}

func TestDefaultProtectionPolicy_UnknownFallsBackToPhysical(t *testing.T) {
	p := DefaultProtectionPolicy(pb.Listing_Metadata_ContractType(99))
	expected := DefaultProtectionPolicy(pb.Listing_Metadata_PHYSICAL_GOOD)
	if p.AutoCompleteAfterShipDays != expected.AutoCompleteAfterShipDays ||
		p.MaxFulfillDays != expected.MaxFulfillDays ||
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
	if got := p.MaxFulfillDuration(); got != 7*24*time.Hour {
		t.Errorf("MaxFulfillDuration = %v, want %v", got, 7*24*time.Hour)
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

func TestComputeProtection_AwaitingFulfillment(t *testing.T) {
	o := &Order{State: OrderState_AWAITING_FULFILLMENT}
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

func TestComputeProtection_Fulfilled_WithTimestamp(t *testing.T) {
	fulfilledAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State:       OrderState_FULFILLED,
		FulfilledAt: &fulfilledAt,
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
	expectedDeadline := fulfilledAt.Add(14 * 24 * time.Hour)
	if !info.AutoCompleteAt.Equal(expectedDeadline) {
		t.Errorf("autoCompleteAt = %v, want %v", info.AutoCompleteAt, expectedDeadline)
	}
	if !info.Extendable {
		t.Error("physical goods should be extendable")
	}
}

func TestComputeProtection_Fulfilled_WithoutTimestamp(t *testing.T) {
	o := &Order{
		State:       OrderState_FULFILLED,
		FulfilledAt: nil,
	}
	info := o.ComputeProtection(time.Now())
	if info == nil {
		t.Fatal("expected non-nil")
	}
	if info.DaysRemaining != 14 {
		t.Errorf("daysRemaining = %d, want 14 (fallback to full policy)", info.DaysRemaining)
	}
	if info.AutoCompleteAt != nil {
		t.Error("autoCompleteAt should be nil when fulfilledAt is missing")
	}
}

func TestComputeProtection_Fulfilled_DeadlinePassed(t *testing.T) {
	fulfilledAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	o := &Order{
		State:       OrderState_FULFILLED,
		FulfilledAt: &fulfilledAt,
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
		State:       OrderState_COMPLETED,
		CompletedAt: &completedAt,
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
		State:       OrderState_COMPLETED,
		CompletedAt: &completedAt,
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
		State:       OrderState_COMPLETED,
		CompletedAt: nil,
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
		State:       OrderState_PAYMENT_FINALIZED,
		CompletedAt: &completedAt,
	}
	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	info := o.ComputeProtection(now)
	if info.Stage != ProtectionStageAfterSaleWindow {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageAfterSaleWindow)
	}
}

// ── Extended Protection tests ──────────────────────────────────────────

func TestComputeProtection_Fulfilled_Extended_WithTimestamp(t *testing.T) {
	fulfilledAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	extendedAt := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State:                OrderState_FULFILLED,
		FulfilledAt:          &fulfilledAt,
		ProtectionExtendedAt: &extendedAt,
	}

	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	info := o.ComputeProtection(now)
	if info == nil {
		t.Fatal("expected non-nil protection info")
	}
	if info.Stage != ProtectionStageProtectionPeriod {
		t.Errorf("stage = %s, want %s", info.Stage, ProtectionStageProtectionPeriod)
	}
	// 14 base + 14 extended = 28 days total from fulfilledAt
	// 28 - 9 elapsed = 19 days remaining
	if info.DaysRemaining != 19 {
		t.Errorf("daysRemaining = %d, want 19 (28 - 9 days elapsed)", info.DaysRemaining)
	}
	expectedDeadline := fulfilledAt.Add(28 * 24 * time.Hour)
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

func TestComputeProtection_Fulfilled_Extended_WithoutTimestamp(t *testing.T) {
	extendedAt := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State:                OrderState_FULFILLED,
		FulfilledAt:          nil,
		ProtectionExtendedAt: &extendedAt,
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

func TestComputeProtection_Fulfilled_NotExtended_Extendable(t *testing.T) {
	fulfilledAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	o := &Order{
		State:       OrderState_FULFILLED,
		FulfilledAt: &fulfilledAt,
	}
	info := o.ComputeProtection(time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC))
	if !info.Extendable {
		t.Error("physical goods should be extendable when not yet extended")
	}
	if info.Extended {
		t.Error("extended should be false when not extended")
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
