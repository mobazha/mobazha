package digital

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/stretchr/testify/require"
)

func TestDigitalAssetService_LicenseKeyPoolSupplyLinesForOrderItems(t *testing.T) {
	assetSvc, _ := newTestAssetService(t)
	_, err := assetSvc.CreateLinkAsset("listing-link", "", "https://example.com/download")
	require.NoError(t, err)
	_, err = assetSvc.ImportLicenseKeys("listing-lic", "", "app-lic", []string{"KEY-1", "KEY-2"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	lines, err := assetSvc.LicenseKeyPoolSupplyLinesForOrderItems([]OrderLineItem{
		{ListingSlug: "listing-link", Quantity: 1},
		{ListingSlug: "listing-lic", Quantity: 2},
	})
	require.NoError(t, err)
	require.Len(t, lines, 1)
	require.Equal(t, "listing-lic", lines[0].ListingSlug)
	require.Equal(t, "", lines[0].VariantSKU)
	require.Equal(t, 2, lines[0].Quantity)
	require.Equal(t, contracts.SupplyKindLicenseKeyPool, lines[0].SupplyKind)
	require.Contains(t, lines[0].LineID, "license_key_pool")
}

func TestDigitalAssetService_LicenseKeySupplyResolutionUsesLicenseAssetVariant(t *testing.T) {
	assetSvc, _ := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-universal", "", "app-universal", []string{"KEY-1"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	lines, err := assetSvc.LicenseKeyPoolSupplyLinesForOrderItems([]OrderLineItem{
		{ListingSlug: "listing-universal", VariantSKU: "buyer-selected-blue", Quantity: 1},
	})
	require.NoError(t, err)
	require.Len(t, lines, 1)
	require.Equal(t, "listing-universal", lines[0].ListingSlug)
	require.Equal(t, "", lines[0].VariantSKU, "provider must query the universal license key pool")
}

func TestDigitalAssetService_LicenseKeySupplyResolutionTreatsZeroQuantityAsOne(t *testing.T) {
	assetSvc, _ := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-default-qty", "", "app-default", []string{"KEY-1"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	lines, err := assetSvc.LicenseKeyPoolSupplyLinesForOrderItems([]OrderLineItem{
		{ListingSlug: "listing-default-qty"},
	})
	require.NoError(t, err)
	require.Len(t, lines, 1)
	require.Equal(t, 1, lines[0].Quantity)
}

func TestDigitalAssetService_LicenseKeySupplyResolutionPrefersKeyPoolForMixedAssets(t *testing.T) {
	assetSvc, _ := newTestAssetService(t)
	_, err := assetSvc.UploadFileAssetStream(context.Background(), "listing-mixed", "", "manual.pdf", "application/pdf", bytes.NewReader([]byte("pdf")), 3)
	require.NoError(t, err)
	_, err = assetSvc.ImportLicenseKeys("listing-mixed", "", "app-mixed", []string{"KEY-1"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	lines, err := assetSvc.LicenseKeyPoolSupplyLinesForOrderItems([]OrderLineItem{
		{ListingSlug: "listing-mixed", Quantity: 1},
	})
	require.NoError(t, err)
	require.Len(t, lines, 1)
	require.Equal(t, contracts.SupplyKindLicenseKeyPool, lines[0].SupplyKind)
}

func TestDigitalAssetService_LicenseKeySupplyLinesWorkWithProvider(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-provider", "", "app-provider", []string{"KEY-1", "KEY-2"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	provider := NewLicenseKeyPoolProvider(db)

	lines, err := assetSvc.LicenseKeyPoolSupplyLinesForOrderItems([]OrderLineItem{
		{ListingSlug: "listing-provider", VariantSKU: "buyer-selected-blue", Quantity: 2},
	})
	require.NoError(t, err)
	require.Len(t, lines, 1)

	availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{Line: lines[0]})
	require.NoError(t, err)
	require.True(t, availability.Available)
	require.Equal(t, int64(2), availability.AvailableQuantity)

	reservation, err := provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:    "order-provider",
		OrderType:   "standard",
		BuyerPeerID: "buyer-provider",
		Line:        lines[0],
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, 2, reservation.Quantity)
	require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
		OrderRef:  "order-provider",
		OrderType: "standard",
	}))
	require.Equal(t, int64(2), assetSvc.CountAllocatedKeys("order-provider", "listing-provider", ""))
}

func TestDigitalAssetService_UnlimitedDigitalSupplyLinesForOrderItems(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.UploadFileAssetStream(context.Background(), "listing-file-supply", "", "guide.pdf", "application/pdf", bytes.NewReader([]byte("pdf")), 3)
	require.NoError(t, err)
	_, err = assetSvc.CreateLinkAsset("listing-link-supply", "", "https://example.com/link")
	require.NoError(t, err)
	provider := NewUnlimitedDigitalProvider(db)

	lines, err := assetSvc.UnlimitedDigitalSupplyLinesForOrderItems([]OrderLineItem{
		{ListingSlug: "listing-file-supply", Quantity: 2},
		{ListingSlug: "listing-link-supply", Quantity: 0},
	})
	require.NoError(t, err)
	require.Len(t, lines, 2)
	require.Equal(t, contracts.SupplyKindUnlimitedDigital, lines[0].SupplyKind)
	require.Equal(t, 2, lines[0].Quantity)
	require.Equal(t, contracts.SupplyKindUnlimitedDigital, lines[1].SupplyKind)
	require.Equal(t, 1, lines[1].Quantity)

	for _, line := range lines {
		availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{Line: line})
		require.NoError(t, err)
		require.True(t, availability.Available)
		require.True(t, availability.Unlimited)
	}
}

func TestDigitalAssetService_UnlimitedDigitalSupplyLinesSkipLicenseControlledItems(t *testing.T) {
	assetSvc, _ := newTestAssetService(t)
	_, err := assetSvc.CreateLinkAsset("listing-license-controls", "", "https://example.com/link")
	require.NoError(t, err)
	_, err = assetSvc.ImportLicenseKeys("listing-license-controls", "", "app-controls", []string{"KEY-1"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	lines, err := assetSvc.UnlimitedDigitalSupplyLinesForOrderItems([]OrderLineItem{
		{ListingSlug: "listing-license-controls", Quantity: 1},
	})
	require.NoError(t, err)
	require.Empty(t, lines)
}

func TestDigitalAssetService_SupplyAvailabilityLinesForOrderItemsCombinesDigitalKinds(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.CreateLinkAsset("listing-link-combined", "", "https://example.com/link")
	require.NoError(t, err)
	_, err = assetSvc.ImportLicenseKeys("listing-license-combined", "", "app-license", []string{"KEY-1"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	_, err = assetSvc.UploadFileAssetStream(context.Background(), "listing-mixed-combined", "", "manual.pdf", "application/pdf", bytes.NewReader([]byte("pdf")), 3)
	require.NoError(t, err)
	_, err = assetSvc.ImportLicenseKeys("listing-mixed-combined", "", "app-mixed", []string{"KEY-2"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)

	lines, err := assetSvc.SupplyAvailabilityLinesForOrderItems([]OrderLineItem{
		{ListingSlug: "listing-link-combined", Quantity: 0},
		{ListingSlug: "listing-missing-combined", Quantity: 1},
		{ListingSlug: "listing-license-combined", VariantSKU: "buyer-blue", Quantity: 1},
		{ListingSlug: "listing-mixed-combined", Quantity: 1},
	})
	require.NoError(t, err)
	require.Len(t, lines, 3)
	require.Equal(t, contracts.SupplyKindUnlimitedDigital, lines[0].SupplyKind)
	require.Equal(t, "listing-link-combined", lines[0].ListingSlug)
	require.Equal(t, 1, lines[0].Quantity)
	require.Equal(t, contracts.SupplyKindLicenseKeyPool, lines[1].SupplyKind)
	require.Equal(t, "listing-license-combined", lines[1].ListingSlug)
	require.Equal(t, "", lines[1].VariantSKU)
	require.Equal(t, contracts.SupplyKindLicenseKeyPool, lines[2].SupplyKind, "license key pool controls scarcity for mixed assets")

	unlimitedProvider := NewUnlimitedDigitalProvider(db)
	availability, err := unlimitedProvider.GetAvailability(context.Background(), contracts.AvailabilityRequest{Line: lines[0]})
	require.NoError(t, err)
	require.True(t, availability.Unlimited)

	licenseProvider := NewLicenseKeyPoolProvider(db)
	availability, err = licenseProvider.GetAvailability(context.Background(), contracts.AvailabilityRequest{Line: lines[1]})
	require.NoError(t, err)
	require.True(t, availability.Available)
}
