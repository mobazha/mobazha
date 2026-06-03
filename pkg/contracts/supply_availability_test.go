package contracts

import (
	"context"
	"testing"
)

type mockSupplyProvider struct{}

func (mockSupplyProvider) Kind() SupplyKind { return SupplyKindSkuQuantity }

func (mockSupplyProvider) GetAvailability(context.Context, AvailabilityRequest) (*AvailabilityResult, error) {
	return &AvailabilityResult{
		SupplyKind:        SupplyKindSkuQuantity,
		Status:            SupplyAvailabilityAvailable,
		Available:         true,
		AvailableQuantity: 1,
	}, nil
}

func (mockSupplyProvider) Reserve(context.Context, ReserveSupplyRequest) (*SupplyReservation, error) {
	return &SupplyReservation{
		SupplyKind: SupplyKindSkuQuantity,
		Status:     SupplyReservationReserved,
		Quantity:   1,
	}, nil
}

func (mockSupplyProvider) Commit(context.Context, CommitSupplyRequest) error { return nil }
func (mockSupplyProvider) Release(context.Context, ReleaseSupplyRequest) error {
	return nil
}

type mockSupplyAvailabilityService struct{}

func (mockSupplyAvailabilityService) Quote(context.Context, SupplyQuoteRequest) (*SupplyQuoteResult, error) {
	return &SupplyQuoteResult{CanSell: true}, nil
}

func (mockSupplyAvailabilityService) ReserveOrder(context.Context, ReserveOrderSupplyRequest) (*ReserveOrderSupplyResult, error) {
	return &ReserveOrderSupplyResult{}, nil
}

func (mockSupplyAvailabilityService) CommitOrder(context.Context, string, string) error {
	return nil
}

func (mockSupplyAvailabilityService) ReleaseOrder(context.Context, string, string, string) error {
	return nil
}

func TestSupplyKindIsValid(t *testing.T) {
	valid := []SupplyKind{
		SupplyKindSkuQuantity,
		SupplyKindLicenseKeyPool,
		SupplyKindUnlimitedDigital,
		SupplyKindExternalSupply,
	}
	for _, k := range valid {
		if !k.IsValid() {
			t.Fatalf("expected %q to be valid", k)
		}
	}
	if SupplyKind("unknown_kind").IsValid() {
		t.Fatal("unexpected provider kind should not be valid")
	}
}

func TestSupplyAvailabilityContractCompliance(t *testing.T) {
	var _ SupplyProvider = mockSupplyProvider{}
	var _ SupplyAvailabilityService = mockSupplyAvailabilityService{}
}

func TestPartitionReservableSupplyLines(t *testing.T) {
	lines := []SupplyLine{
		{LineID: "sku-1", SupplyKind: SupplyKindSkuQuantity, ListingSlug: "local-shirt"},
		{LineID: "ext-1", SupplyKind: SupplyKindExternalSupply, ListingSlug: "supplier-shirt"},
		{LineID: "key-1", SupplyKind: SupplyKindLicenseKeyPool, ListingSlug: "software"},
	}
	reservable, manualAction := PartitionReservableSupplyLines(lines)
	if len(reservable) != 2 {
		t.Fatalf("reservable len = %d, want 2", len(reservable))
	}
	if len(manualAction) != 1 {
		t.Fatalf("manualAction len = %d, want 1", len(manualAction))
	}
	if manualAction[0].SupplyKind != SupplyKindExternalSupply {
		t.Fatalf("manualAction kind = %q, want external_supply", manualAction[0].SupplyKind)
	}
}

func TestSupplyAvailabilityConstants(t *testing.T) {
	if SupplyAvailabilityManualActionRequired != "manual_action_required" {
		t.Fatalf("unexpected manual action status: %q", SupplyAvailabilityManualActionRequired)
	}
	if SupplyReservationNoop != "noop" {
		t.Fatalf("unexpected no-op reservation status: %q", SupplyReservationNoop)
	}
}
