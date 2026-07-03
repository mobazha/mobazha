package digital

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestUnlimitedDigitalProvider_FileAndLinkAssetsAreUnlimitedNoopReservations(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.UploadFileAssetStream(context.Background(), "listing-file", "", "guide.pdf", "application/pdf", bytes.NewReader([]byte("pdf")), 3)
	require.NoError(t, err)
	_, err = assetSvc.CreateLinkAsset("listing-link", "", "https://example.com/download")
	require.NoError(t, err)
	provider := NewUnlimitedDigitalProvider(db)

	for _, slug := range []string{"listing-file", "listing-link"} {
		line := unlimitedDigitalSupplyLine(slug, "", 2)
		availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{Line: line})
		require.NoError(t, err)
		require.True(t, availability.Available)
		require.True(t, availability.Unlimited)
		require.Equal(t, contracts.SupplyAvailabilityUnlimited, availability.Status)

		reservation, err := provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
			OrderRef:  "order-" + slug,
			OrderType: models.OrderTypeStandard,
			Line:      line,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
		require.Equal(t, contracts.SupplyReservationNoop, reservation.Status)
		require.Equal(t, 2, reservation.Quantity)
		require.NoError(t, provider.Commit(context.Background(), contracts.CommitSupplyRequest{
			OrderRef:  "order-" + slug,
			OrderType: models.OrderTypeStandard,
		}))
		require.NoError(t, provider.Release(context.Background(), contracts.ReleaseSupplyRequest{
			OrderRef:  "order-" + slug,
			OrderType: models.OrderTypeStandard,
			Reason:    "cancelled",
		}))
	}
}

func TestUnlimitedDigitalProvider_MissingAssetRequiresManualAction(t *testing.T) {
	_, db := newTestAssetService(t)
	provider := NewUnlimitedDigitalProvider(db)
	line := unlimitedDigitalSupplyLine("listing-missing", "", 1)

	availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{Line: line})
	require.NoError(t, err)
	require.False(t, availability.Available)
	require.True(t, availability.ManualActionRequired)
	require.Equal(t, contracts.SupplyAvailabilityManualActionRequired, availability.Status)

	_, err = provider.Reserve(context.Background(), contracts.ReserveSupplyRequest{
		OrderRef:  "order-missing",
		OrderType: models.OrderTypeStandard,
		Line:      line,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, contracts.ErrSupplyManualActionRequired))
}

func TestUnlimitedDigitalProvider_DoesNotTreatLicenseKeyAssetAsUnlimited(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.ImportLicenseKeys("listing-license-only", "", "app-license", []string{"KEY-1"}, "perpetual", 1, time.Time{})
	require.NoError(t, err)
	provider := NewUnlimitedDigitalProvider(db)

	availability, err := provider.GetAvailability(context.Background(), contracts.AvailabilityRequest{
		Line: unlimitedDigitalSupplyLine("listing-license-only", "", 1),
	})
	require.NoError(t, err)
	require.True(t, availability.ManualActionRequired)
	require.Equal(t, contracts.SupplyAvailabilityManualActionRequired, availability.Status)
}

func TestUnlimitedDigitalProvider_TransactionMethodsValidateTx(t *testing.T) {
	assetSvc, db := newTestAssetService(t)
	_, err := assetSvc.CreateLinkAsset("listing-tx", "", "https://example.com/tx")
	require.NoError(t, err)
	provider := NewUnlimitedDigitalProvider(db)
	line := unlimitedDigitalSupplyLine("listing-tx", "", 1)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		reservation, err := provider.ReserveTx(context.Background(), tx, contracts.ReserveSupplyRequest{
			OrderRef:  "order-tx",
			OrderType: models.OrderTypeStandard,
			Line:      line,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
		require.Equal(t, contracts.SupplyReservationNoop, reservation.Status)
		require.NoError(t, provider.CommitTx(context.Background(), tx, contracts.CommitSupplyRequest{
			OrderRef:  "order-tx",
			OrderType: models.OrderTypeStandard,
		}))
		return provider.ReleaseTx(context.Background(), tx, contracts.ReleaseSupplyRequest{
			OrderRef:  "order-tx",
			OrderType: models.OrderTypeStandard,
			Reason:    "cancelled",
		})
	}))
}

func unlimitedDigitalSupplyLine(listingSlug string, variantSKU string, quantity int) contracts.SupplyLine {
	return contracts.SupplyLine{
		ListingSlug: listingSlug,
		VariantSKU:  variantSKU,
		Quantity:    quantity,
		SupplyKind:  contracts.SupplyKindUnlimitedDigital,
	}
}
