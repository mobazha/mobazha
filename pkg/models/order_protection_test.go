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
