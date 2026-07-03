package orderextensions_test

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestPersistTx_IsIdempotentAndAppendsChangedRevision(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.OrderExtensionRecord{}); err != nil {
			return err
		}
		return tx.Migrate(&models.OrderExtensionEventSequence{})
	}))

	first, err := extensions.NewOrderExtension("order-1", "provider", "type", "v1", "resource", map[string]string{"value": "one"})
	require.NoError(t, err)
	first.Revision = 99 // Persistence, not a module, owns revision assignment.
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := orderextensions.PersistTx(tx, "order-1", first); err != nil {
			return err
		}
		return orderextensions.PersistTx(tx, "order-1", first)
	}))

	second, err := extensions.NewOrderExtension("order-1", "provider", "type", "v1", "resource", map[string]string{"value": "two"})
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error { return orderextensions.PersistTx(tx, "order-1", second) }))

	var records []models.OrderExtensionRecord
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().Order("revision ASC").Find(&records).Error }))
	require.Len(t, records, 2)
	require.Equal(t, uint64(1), records[0].Revision)
	require.Equal(t, uint64(2), records[1].Revision)
}

func TestEnqueueTx_DeduplicatesEventID(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.OrderExtensionEventSequence{}); err != nil {
			return err
		}
		return tx.Migrate(&models.ExtensionDelivery{})
	}))
	event := extensions.Event{EventID: "event-1", ProviderID: "provider", Type: "paid", Version: "v1", TenantID: database.StandaloneTenantID, SourceID: "peer-1", OrderRole: "vendor", OrderID: "order-1", ExtensionID: "ext-1", IdempotencyKey: "idem-1", OccurredAt: time.Now().UTC()}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := orderextensions.EnqueueTx(tx, event); err != nil {
			return err
		}
		return orderextensions.EnqueueTx(tx, event)
	}))
	var count int64
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().Model(&models.ExtensionDelivery{}).Count(&count).Error }))
	require.Equal(t, int64(1), count)
	var delivery models.ExtensionDelivery
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("event_id = ?", event.EventID).First(&delivery).Error
	}))
	require.Equal(t, uint64(1), delivery.OrderVersion)
	require.Equal(t, database.StandaloneTenantID, orderextensions.EventFromDelivery(delivery).TenantID)

	event.EventID = "event-2"
	event.IdempotencyKey = "idem-2"
	require.NoError(t, db.Update(func(tx database.Tx) error { return orderextensions.EnqueueTx(tx, event) }))
	delivery = models.ExtensionDelivery{}
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("event_id = ?", event.EventID).First(&delivery).Error
	}))
	require.Equal(t, uint64(2), delivery.OrderVersion)
}

func TestRecordAttestationTx_RejectsIdempotencyReuseWithDifferentEvidence(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Migrate(&models.SettlementAttestationRecord{}) }))
	now := time.Now().UTC()
	attestation := extensions.SettlementAttestation{AttestationID: "att-1", IdempotencyKey: "idem-1", Issuer: "provider", TenantID: database.StandaloneTenantID, OrderID: "order-1", SettlementID: "settlement-1", ExtensionID: "ext-1", ExpectedExtensionRevision: 1, ExpectedOrderStateVersion: "sha256:state", ConditionType: "delivered", ConditionVersion: "v1", EvidenceDigest: "sha256:one", ObservedAt: now, ExpiresAt: now.Add(time.Minute)}
	require.NoError(t, db.Update(func(tx database.Tx) error { return orderextensions.RecordAttestationTx(tx, attestation, now) }))
	attestation.AttestationID = "att-2"
	attestation.EvidenceDigest = "sha256:two"
	err = db.Update(func(tx database.Tx) error { return orderextensions.RecordAttestationTx(tx, attestation, now) })
	require.ErrorContains(t, err, "reused with different evidence")
}

func TestRecordAttestationTx_RejectsEvidenceReplayWithNewIDs(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Migrate(&models.SettlementAttestationRecord{}) }))
	now := time.Now().UTC()
	attestation := extensions.SettlementAttestation{AttestationID: "att-1", IdempotencyKey: "idem-1", Issuer: "provider", TenantID: database.StandaloneTenantID, OrderID: "order-1", SettlementID: "settlement-1", ExtensionID: "ext-1", ExpectedExtensionRevision: 1, ExpectedOrderStateVersion: "sha256:state", ConditionType: "delivered", ConditionVersion: "v1", EvidenceDigest: "sha256:one", ObservedAt: now, ExpiresAt: now.Add(time.Minute)}
	require.NoError(t, db.Update(func(tx database.Tx) error { return orderextensions.RecordAttestationTx(tx, attestation, now) }))
	attestation.AttestationID = "att-2"
	attestation.IdempotencyKey = "idem-2"
	err = db.Update(func(tx database.Tx) error { return orderextensions.RecordAttestationTx(tx, attestation, now) })
	require.ErrorContains(t, err, "evidence was replayed")
}

func TestRecordReservationTx_PersistsProviderBindingIdempotently(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.OrderExtensionReservationRecord{})
	}))
	extension, err := extensions.NewOrderExtension("order-1", "provider", "type", "v1", "resource", map[string]string{"value": "one"})
	require.NoError(t, err)
	request := extensions.ReservationRequest{
		OrderID: "order-1", Extension: extension, PaymentCoin: "crypto:eip155:1:native",
		IdempotencyKey: "reserve:ext:order-1", ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	reservation := extensions.Reservation{ID: "reservation-1", Version: 3, Status: "reserved"}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := orderextensions.RecordReservationTx(tx, request, reservation); err != nil {
			return err
		}
		return orderextensions.RecordReservationTx(tx, request, reservation)
	}))
	var binding *extensions.ReservationBinding
	require.NoError(t, db.View(func(tx database.Tx) error {
		var loadErr error
		binding, loadErr = orderextensions.ReservationByExtensionTx(tx, request.OrderID, extension.ExtensionID)
		return loadErr
	}))
	require.NotNil(t, binding)
	require.Equal(t, reservation.ID, binding.ReservationID)
	require.Equal(t, reservation.Version, binding.ReservationVersion)
	require.Equal(t, extension.Revision, binding.ExtensionRevision)
	require.Equal(t, request.PaymentCoin, binding.PaymentCoin)

	conflicting := reservation
	conflicting.ID = "reservation-2"
	err = db.Update(func(tx database.Tx) error { return orderextensions.RecordReservationTx(tx, request, conflicting) })
	require.ErrorContains(t, err, "immutable")
	conflicting = reservation
	conflicting.Status = "released"
	err = db.Update(func(tx database.Tx) error { return orderextensions.RecordReservationTx(tx, request, conflicting) })
	require.ErrorContains(t, err, "immutable")
}
