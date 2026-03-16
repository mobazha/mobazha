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
	if p != expected {
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
