package settlement

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestExecuteSettlementAction_RejectsUnimplementedActionsBeforeDB(t *testing.T) {
	svc := &SettlementService{}
	_, _, err := svc.ExecuteSettlementAction(context.Background(), "dispute_release", models.OrderID("order-1"), "")
	if err == nil {
		t.Fatal("expected unsupported action error")
	}
	if !strings.Contains(err.Error(), "supported: confirm, cancel, seller_decline_refund") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteSettlementAction_ConfirmModeratedReturnsNoop(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	order := seedModeratedSettlementActionOrder(t, db, "order-moderated-confirm", models.RoleVendor)

	result, coinType, err := svc.ExecuteSettlementAction(
		context.Background(),
		"confirm",
		order.ID,
		"0x2222222222222222222222222222222222222222",
	)
	require.NoError(t, err)
	require.Equal(t, "crypto:eip155:1:native", coinType.String())
	require.NotNil(t, result)
	require.Equal(t, payment.ActionModeCompleted, result.Mode)
}

func TestExecuteSettlementAction_CancelModeratedBeforeConfirmUsesStrategy(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	strategy := &utxoActionStatusStub{
		model:        payment.PaymentModelMonitored,
		cancelResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "act-cancel"},
	}
	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	svc.SetRegistry(reg)
	order := seedModeratedSettlementActionOrder(t, db, "order-moderated-cancel", models.RoleVendor)

	result, coinType, err := svc.ExecuteSettlementAction(
		context.Background(),
		"cancel",
		order.ID,
		"0x3333333333333333333333333333333333333333",
	)
	require.NoError(t, err)
	require.Equal(t, "crypto:eip155:1:native", coinType.String())
	require.NotNil(t, result)
	require.Equal(t, payment.ActionModeSubmitted, result.Mode)
	require.Equal(t, "act-cancel", result.ActionID)
	require.Equal(t, order.ID.String(), strategy.lastCancel.OrderID)
	require.Equal(t, "0x3333333333333333333333333333333333333333", strategy.lastCancel.PayoutAddr)
}

func TestExecuteSettlementAction_SellerDeclineRefundUsesOptionalStrategy(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	strategy := &utxoActionStatusStub{
		model:               payment.PaymentModelMonitored,
		sellerDeclineResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "act-seller-decline-refund"},
	}
	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	svc.SetRegistry(reg)
	order := seedModeratedSettlementActionOrder(t, db, "order-seller-decline-refund", models.RoleVendor)

	result, coinType, err := svc.ExecuteSettlementAction(
		context.Background(),
		payment.SettlementActionSellerDeclineRefund,
		order.ID,
		"0x4444444444444444444444444444444444444444",
	)
	require.NoError(t, err)
	require.Equal(t, "crypto:eip155:1:native", coinType.String())
	require.NotNil(t, result)
	require.Equal(t, payment.ActionModeSubmitted, result.Mode)
	require.Equal(t, "act-seller-decline-refund", result.ActionID)
	require.Equal(t, order.ID.String(), strategy.lastSellerDecline.OrderID)
	require.Equal(t, "0x4444444444444444444444444444444444444444", strategy.lastSellerDecline.PayoutAddr)
}

func TestExecuteSettlementAction_ReusesActiveDurableIntent(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	strategy := &utxoActionStatusStub{
		model:         payment.PaymentModelMonitored,
		confirmResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "unexpected-second-submit"},
	}
	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	svc.SetRegistry(reg)
	order := seedModeratedSettlementActionOrder(t, db, "order-idempotent-confirm", models.RoleVendor)
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		SettlementSpec: payment.NewManagedEscrowSpec(false).ToPaymentSent(),
		Amount:         "1000",
		Coin:           "crypto:eip155:11155111:native",
		PayerAddress:   "0x1111111111111111111111111111111111111111",
		RefundAddress:  "0x1111111111111111111111111111111111111111",
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		return tx.Save(&models.SettlementAction{
			ActionID:   "existing-confirm-action",
			OrderID:    order.ID.String(),
			ActionKind: payment.SettlementActionConfirm,
			State:      "submitted",
			TxHash:     "0xabc",
		})
	}))

	result, coinType, err := svc.ExecuteSettlementAction(
		context.Background(), payment.SettlementActionConfirm, order.ID,
		"0x2222222222222222222222222222222222222222",
	)
	require.NoError(t, err)
	require.Equal(t, "crypto:eip155:1:native", coinType.String())
	require.Equal(t, "existing-confirm-action", result.ActionID)
	require.Equal(t, "0xabc", result.SubmittedTxHash)
	require.Zero(t, strategy.confirmCalls)
}

func TestExecuteSettlementAction_SellerDeclineRefundFallsBackToCancel(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	base := &utxoActionStatusStub{
		model:        payment.PaymentModelMonitored,
		cancelResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "act-managed-refund"},
	}
	strategy := &cancelOnlySettlementStrategy{ChainEscrowV2: base}
	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	svc.SetRegistry(reg)
	order := seedModeratedSettlementActionOrder(t, db, "order-managed-seller-refund", models.RoleVendor)

	result, _, err := svc.ExecuteSettlementAction(
		context.Background(),
		payment.SettlementActionSellerDeclineRefund,
		order.ID,
		"0x4444444444444444444444444444444444444444",
	)
	require.NoError(t, err)
	require.Equal(t, "act-managed-refund", result.ActionID)
	require.Equal(t, order.ID.String(), base.lastCancel.OrderID)
	require.Equal(t, "0x4444444444444444444444444444444444444444", base.lastCancel.PayoutAddr)
}

func TestSettlementActionStatusMatchesSellerDeclineCancelAlias(t *testing.T) {
	base := &utxoActionStatusStub{model: payment.PaymentModelMonitored}
	cancelOnly := &cancelOnlySettlementStrategy{ChainEscrowV2: base}

	require.True(t, settlementActionStatusMatches(
		cancelOnly,
		payment.SettlementActionCancel,
		payment.SettlementActionSellerDeclineRefund,
	))
	require.False(t, settlementActionStatusMatches(
		base,
		payment.SettlementActionCancel,
		payment.SettlementActionSellerDeclineRefund,
	), "a chain with a dedicated seller-decline action must not accept cancel status")
	require.False(t, settlementActionStatusMatches(
		cancelOnly,
		payment.SettlementActionConfirm,
		payment.SettlementActionSellerDeclineRefund,
	))
}

func TestExecuteConditionalSettlement_AcceptsAuthorizedDeliveryAttestation(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	strategy := &utxoActionStatusStub{model: payment.PaymentModelMonitored, confirmResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "conditional-action"}}
	registry := payment.NewRegistry()
	registry.RegisterV2(iwallet.ChainEthereum, strategy)
	locker := &recordingOrderLocker{}
	svc := NewSettlementService(SettlementServiceConfig{DB: db, OrderLocker: locker})
	svc.SetRegistry(registry)
	const corePayoutAddress = "0x5555555555555555555555555555555555555555"
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ReceivingAccount{ID: 1, Name: "seller", ChainType: iwallet.ChainEthereum, Address: corePayoutAddress, IsActive: true})
	}))
	order := seedCollectiblePrimarySaleReleaseOrder(t, db, "order-conditional-release", models.RoleVendor, true)
	extension := persistCollectibleTestExtension(t, db, order)
	svc.SetAttestationVerifierResolver(func(string) extensions.AttestationVerifier {
		return attestationVerifierFunc(func(context.Context, extensions.SettlementAttestation, extensions.OrderExtension) error { return nil })
	})
	now := time.Now().UTC()
	reference, err := orderextensions.SettlementReferenceForOrder(order)
	require.NoError(t, err)
	result, _, err := svc.ExecuteConditionalSettlement(context.Background(), extensions.SettlementAttestation{
		AttestationID: "att-1", IdempotencyKey: "att-1", Issuer: models.CollectibleExtensionProviderID, TenantID: database.StandaloneTenantID,
		OrderID: order.ID.String(), SettlementID: "settlement:" + order.ID.String(), ExtensionID: extension.ExtensionID, ExpectedExtensionRevision: extension.Revision, ExpectedOrderStateVersion: reference.OrderStateVersion,
		ConditionType: extensions.ConditionOrderExtensionDeliveryConfirmed, ConditionVersion: extensions.ContractVersionV1,
		EvidenceDigest: "sha256:evidence", ObservedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	require.Equal(t, payment.ActionModeSubmitted, result.Mode)
	require.Equal(t, corePayoutAddress, strategy.lastConfirm.PayoutAddr)
	var attestation models.SettlementAttestationRecord
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("attestation_id = ?", "att-1").First(&attestation).Error
	}))
	require.Equal(t, order.ID.String(), attestation.OrderID)
	require.Equal(t, "conditional-action", attestation.ExecutionActionID)
	require.Equal(t, corePayoutAddress, attestation.ExecutionPayoutAddress)
	require.True(t, locker.locked)
	require.True(t, locker.unlocked)
}

func TestExecuteConditionalSettlement_CompletedActionEmitsBoundConfirmation(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	bus := events.NewBus()
	subscription, err := bus.Subscribe(&events.OrderAutoConfirmRequest{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, subscription.Close()) })
	strategy := &utxoActionStatusStub{model: payment.PaymentModelMonitored, confirmResult: &payment.ActionResult{
		Mode: payment.ActionModeCompleted, SubmittedTxHash: "0xconditional-confirmed",
	}}
	registry := payment.NewRegistry()
	registry.RegisterV2(iwallet.ChainEthereum, strategy)
	svc := NewSettlementService(SettlementServiceConfig{
		DB: db, EventBus: bus, OrderLocker: &recordingOrderLocker{},
	})
	svc.SetRegistry(registry)
	const payoutAddress = "0x6666666666666666666666666666666666666666"
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ReceivingAccount{ID: 2, Name: "seller", ChainType: iwallet.ChainEthereum, Address: payoutAddress, IsActive: true})
	}))
	order := seedCollectiblePrimarySaleReleaseOrder(t, db, "order-conditional-completed", models.RoleVendor, true)
	extension := persistCollectibleTestExtension(t, db, order)
	svc.SetAttestationVerifierResolver(func(string) extensions.AttestationVerifier {
		return attestationVerifierFunc(func(context.Context, extensions.SettlementAttestation, extensions.OrderExtension) error { return nil })
	})
	reference, err := orderextensions.SettlementReferenceForOrder(order)
	require.NoError(t, err)
	now := time.Now().UTC()
	_, _, err = svc.ExecuteConditionalSettlement(context.Background(), extensions.SettlementAttestation{
		AttestationID: "att-completed", IdempotencyKey: "att-completed", Issuer: extension.ProviderID,
		TenantID: database.StandaloneTenantID, OrderID: order.ID.String(), SettlementID: reference.SettlementID,
		ExtensionID: extension.ExtensionID, ExpectedExtensionRevision: extension.Revision, ExpectedOrderStateVersion: reference.OrderStateVersion,
		ConditionType: extensions.ConditionOrderExtensionDeliveryConfirmed, ConditionVersion: extensions.ContractVersionV1,
		EvidenceDigest: "sha256:completed", ObservedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	event := (<-subscription.Out()).(*events.OrderAutoConfirmRequest)
	require.Equal(t, order.ID.String(), event.OrderID)
	require.Equal(t, "0xconditional-confirmed", event.TxID)
	require.Equal(t, payoutAddress, event.PayoutAddress)
	require.Equal(t, "att-completed", event.SettlementAttestationID)
	var record models.SettlementAttestationRecord
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("attestation_id = ?", "att-completed").First(&record).Error
	}))
	require.Equal(t, "0xconditional-confirmed", record.ExecutionTransactionID)
	require.Equal(t, payoutAddress, record.ExecutionPayoutAddress)
}

func TestDefaultConfirmationSurfaces_RejectExtensionAttestedOrder(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	order := &models.Order{ID: "order-attested-default-confirm"}
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(order) }))
	persistCollectibleTestExtension(t, db, order)
	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	_, _, err = svc.ExecuteSettlementAction(context.Background(), payment.SettlementActionConfirm, order.ID, "")
	require.ErrorContains(t, err, "requires conditional settlement")
	_, _, err = svc.GetConfirmOrderInstructions(order.ID, "initiator", "payout")
	require.ErrorContains(t, err, "does not expose client-signed confirmation instructions")
}

func TestExecuteConditionalSettlement_RejectsExpiredAttestation(t *testing.T) {
	svc := &SettlementService{}
	now := time.Now().UTC()
	_, _, err := svc.ExecuteConditionalSettlement(context.Background(), extensions.SettlementAttestation{
		AttestationID: "att-expired", IdempotencyKey: "att-expired", Issuer: "provider", TenantID: database.StandaloneTenantID, OrderID: "order", SettlementID: "settlement-1", ExtensionID: "ext-1", ExpectedExtensionRevision: 1, ExpectedOrderStateVersion: "sha256:state",
		ConditionType: "condition", ConditionVersion: "v1", EvidenceDigest: "sha256:evidence", ObservedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute),
	})
	require.ErrorContains(t, err, "time window")
}

func TestExecuteConditionalSettlement_RejectsStaleExtensionRevision(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	order := seedCollectiblePrimarySaleReleaseOrder(t, db, "order-stale-extension", models.RoleVendor, true)
	extension := persistCollectibleTestExtension(t, db, order)
	reference, err := orderextensions.SettlementReferenceForOrder(order)
	require.NoError(t, err)
	now := time.Now().UTC()
	_, _, err = svc.ExecuteConditionalSettlement(context.Background(), extensions.SettlementAttestation{
		AttestationID: "att-stale", IdempotencyKey: "att-stale", Issuer: extension.ProviderID, TenantID: database.StandaloneTenantID,
		OrderID: order.ID.String(), SettlementID: "settlement:" + order.ID.String(), ExtensionID: extension.ExtensionID, ExpectedExtensionRevision: extension.Revision + 1, ExpectedOrderStateVersion: reference.OrderStateVersion,
		ConditionType: extensions.ConditionOrderExtensionDeliveryConfirmed, ConditionVersion: extensions.ContractVersionV1,
		EvidenceDigest: "sha256:stale", ObservedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.ErrorContains(t, err, "expected extension revision")
}

func TestExecuteConditionalSettlement_RequiresProviderVerifier(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	order := seedCollectiblePrimarySaleReleaseOrder(t, db, "order-missing-verifier", models.RoleVendor, true)
	extension := persistCollectibleTestExtension(t, db, order)
	reference, err := orderextensions.SettlementReferenceForOrder(order)
	require.NoError(t, err)
	now := time.Now().UTC()
	_, _, err = svc.ExecuteConditionalSettlement(context.Background(), extensions.SettlementAttestation{
		AttestationID: "att-no-verifier", IdempotencyKey: "att-no-verifier", Issuer: extension.ProviderID, TenantID: database.StandaloneTenantID,
		OrderID: order.ID.String(), SettlementID: "settlement:" + order.ID.String(), ExtensionID: extension.ExtensionID, ExpectedExtensionRevision: extension.Revision, ExpectedOrderStateVersion: reference.OrderStateVersion,
		ConditionType: extensions.ConditionOrderExtensionDeliveryConfirmed, ConditionVersion: extensions.ContractVersionV1,
		EvidenceDigest: "sha256:no-verifier", ObservedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.ErrorContains(t, err, "no attestation verifier")
}

func TestExecuteConditionalSettlement_RejectsWrongTenant(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	order := seedCollectiblePrimarySaleReleaseOrder(t, db, "order-wrong-tenant", models.RoleVendor, true)
	extension := persistCollectibleTestExtension(t, db, order)
	reference, err := orderextensions.SettlementReferenceForOrder(order)
	require.NoError(t, err)
	now := time.Now().UTC()
	_, _, err = svc.ExecuteConditionalSettlement(context.Background(), extensions.SettlementAttestation{
		AttestationID: "att-wrong-tenant", IdempotencyKey: "att-wrong-tenant", Issuer: extension.ProviderID, TenantID: "other-tenant",
		OrderID: order.ID.String(), SettlementID: "settlement:" + order.ID.String(), ExtensionID: extension.ExtensionID,
		ExpectedExtensionRevision: extension.Revision, ExpectedOrderStateVersion: reference.OrderStateVersion,
		ConditionType: extensions.ConditionOrderExtensionDeliveryConfirmed, ConditionVersion: extensions.ContractVersionV1,
		EvidenceDigest: "sha256:tenant", ObservedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.ErrorContains(t, err, "tenant mismatch")
}

func TestExecuteConditionalSettlement_RejectsStateChangedDuringVerification(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	svc := NewSettlementService(SettlementServiceConfig{DB: db, OrderLocker: &recordingOrderLocker{}})
	order := seedCollectiblePrimarySaleReleaseOrder(t, db, "order-state-race", models.RoleVendor, true)
	extension := persistCollectibleTestExtension(t, db, order)
	reference, err := orderextensions.SettlementReferenceForOrder(order)
	require.NoError(t, err)
	svc.SetAttestationVerifierResolver(func(string) extensions.AttestationVerifier {
		return attestationVerifierFunc(func(context.Context, extensions.SettlementAttestation, extensions.OrderExtension) error {
			return db.Update(func(tx database.Tx) error {
				_, updateErr := tx.UpdateColumns(
					map[string]interface{}{"serialized_order_cancel": []byte(`{"changed":true}`)},
					map[string]interface{}{"id = ?": order.ID.String()},
					&models.Order{},
				)
				return updateErr
			})
		})
	})
	now := time.Now().UTC()
	_, _, err = svc.ExecuteConditionalSettlement(context.Background(), extensions.SettlementAttestation{
		AttestationID: "att-race", IdempotencyKey: "att-race", Issuer: extension.ProviderID, TenantID: database.StandaloneTenantID,
		OrderID: order.ID.String(), SettlementID: "settlement:" + order.ID.String(), ExtensionID: extension.ExtensionID,
		ExpectedExtensionRevision: extension.Revision, ExpectedOrderStateVersion: reference.OrderStateVersion,
		ConditionType: extensions.ConditionOrderExtensionDeliveryConfirmed, ConditionVersion: extensions.ContractVersionV1,
		EvidenceDigest: "sha256:race", ObservedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.ErrorContains(t, err, "order state version is stale")
}

type attestationVerifierFunc func(context.Context, extensions.SettlementAttestation, extensions.OrderExtension) error

func (f attestationVerifierFunc) VerifySettlementAttestation(ctx context.Context, attestation extensions.SettlementAttestation, extension extensions.OrderExtension) error {
	return f(ctx, attestation, extension)
}

type recordingOrderLocker struct {
	locked   bool
	unlocked bool
}

func (l *recordingOrderLocker) Lock(context.Context, string, string) error {
	l.locked = true
	return nil
}

func (l *recordingOrderLocker) Unlock(string, string) { l.unlocked = true }

func persistCollectibleTestExtension(t *testing.T, db database.Database, order *models.Order) extensions.OrderExtension {
	t.Helper()
	extension, err := extensions.NewOrderExtension(order.ID.String(), models.CollectibleExtensionProviderID, models.CollectibleExtensionTypePrimarySale, extensions.ContractVersionV1, "slot-test", map[string]string{"slot": "slot-test"})
	require.NoError(t, err)
	extension.SettlementPolicy = extensions.SettlementPolicyExtensionAttested
	extension.LifecycleEvents = []string{extensions.EventOrderPaymentVerified}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return orderextensions.PersistTx(tx, order.ID.String(), extension)
	}))
	return extension
}

func seedModeratedSettlementActionOrder(
	t *testing.T,
	db database.Database,
	id string,
	role models.OrderRole,
) *models.Order {
	t.Helper()

	orderOpen, err := anypb.New(&pb.OrderOpen{
		Timestamp: timestamppb.Now(),
		Amount:    "1000",
		BuyerID:   &pb.ID{PeerID: "12D3KooWBuyer"},
		Listings: []*pb.SignedListing{{
			Cid: "listing-moderated",
			Listing: &pb.Listing{
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item:     &pb.Listing_Item{Title: "Moderated order"},
			},
		}},
		Items: []*pb.OrderOpen_Item{{
			ListingHash: "listing-moderated",
			Quantity:    "1",
		}},
	})
	require.NoError(t, err)

	order := &models.Order{ID: models.OrderID(id)}
	order.SetRole(role)
	order.PaymentVerificationStatus = models.PaymentVerificationStatusVerified
	require.NoError(t, order.PutMessage(&npb.OrderMessage{
		OrderID:     id,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Message:     orderOpen,
	}))
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		SettlementSpec: payment.NewManagedEscrowSpec(true).ToPaymentSent(),
		Amount:         "1000",
		Coin:           "crypto:eip155:11155111:native",
		PayerAddress:   "0x1111111111111111111111111111111111111111",
		RefundAddress:  "0x1111111111111111111111111111111111111111",
		Moderator:      "12D3KooWModerator",
	}))

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
	return order
}

func seedCollectiblePrimarySaleReleaseOrder(
	t *testing.T,
	db database.Database,
	id string,
	role models.OrderRole,
	verified bool,
) *models.Order {
	t.Helper()

	order := seedModeratedSettlementActionOrder(t, db, id, role)
	if verified {
		order.MarkPaymentVerified()
	} else {
		order.MarkPaymentVerificationPending()
	}
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		SettlementSpec: payment.NewManagedEscrowSpec(false).ToPaymentSent(),
		ToAddress:      "settlement:" + id,
		Amount:         "1000",
		Coin:           "crypto:eip155:11155111:native",
		PayerAddress:   "0x1111111111111111111111111111111111111111",
		RefundAddress:  "0x1111111111111111111111111111111111111111",
		Moderator:      "12D3KooWModerator",
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
	return order
}

type utxoActionStatusStub struct {
	model               payment.PaymentModel
	confirmResult       *payment.ActionResult
	cancelResult        *payment.ActionResult
	sellerDeclineResult *payment.ActionResult
	confirmCalls        int
	lastConfirm         payment.ActionParams
	lastCancel          payment.ActionParams
	lastSellerDecline   payment.ActionParams
}

// Embedding the base interface deliberately hides the optional
// SellerDeclineRefunder capability while preserving the full V2 contract.
type cancelOnlySettlementStrategy struct {
	payment.ChainEscrowV2
}

func (s *utxoActionStatusStub) Model() payment.PaymentModel { return s.model }
func (s *utxoActionStatusStub) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{}
}
func (s *utxoActionStatusStub) GetActionStatus(context.Context, string) (*payment.ActionStatus, error) {
	return nil, payment.ErrUnsupportedAction
}

func (s *utxoActionStatusStub) SetupPayment(context.Context, payment.PaymentSetupParams) (*payment.ActionResult, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) Confirm(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	s.confirmCalls++
	s.lastConfirm = params
	if s.confirmResult != nil {
		return s.confirmResult, nil
	}
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) Cancel(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	s.lastCancel = params
	if s.cancelResult != nil {
		return s.cancelResult, nil
	}
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) SellerDeclineRefund(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	s.lastSellerDecline = params
	if s.sellerDeclineResult != nil {
		return s.sellerDeclineResult, nil
	}
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) Complete(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) DisputeRelease(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) AutoConfirm(context.Context, *events.CancelablePaymentReady) error {
	return nil
}
func (s *utxoActionStatusStub) SignEscrowRelease(context.Context, payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, payment.ErrUnsupportedAction
}
func (s *utxoActionStatusStub) EstimateEscrowFee(string, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (s *utxoActionStatusStub) VerifyDeposit(context.Context, payment.DepositVerifyParams) error {
	return nil
}
func (s *utxoActionStatusStub) ValidatePaymentMessage(payment.PaymentMessageParams) error { return nil }
func (s *utxoActionStatusStub) VerifyPreRelease(context.Context, payment.PreReleaseParams) error {
	return nil
}

func TestGetSettlementActionStatus_UTXOSyncActionFromStore(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	const orderID = "utxo-sync-status-order"
	const actionID = "sync-complete-utxo-sync-status-order-deadbeef"
	order := seedModeratedSettlementActionOrder(t, db, orderID, models.RoleVendor)
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		SettlementSpec: payment.NewUTXOSpec(true).ToPaymentSent(),
		Amount:         "1000",
		Coin:           "BCH",
		PayerAddress:   "bitcoincash:qpayer",
		RefundAddress:  "bitcoincash:qpayer",
		Moderator:      "12D3KooWModerator",
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.SettlementAction{
			ActionID:       actionID,
			OrderID:        orderID,
			ActionKind:     "complete",
			State:          "confirmed",
			TxHash:         "utxo-tx-hash-abc",
			SettlementCoin: "BCH",
			GrossAmount:    "1000",
		})
	}))

	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainBitcoinCash, &utxoActionStatusStub{model: payment.PaymentModelMonitored})

	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	svc.SetRegistry(reg)

	status, coinType, err := svc.GetSettlementActionStatus(
		context.Background(),
		"complete",
		order.ID,
		actionID,
	)
	require.NoError(t, err)
	require.Equal(t, iwallet.CoinType("crypto:bitcoincash:mainnet:native"), coinType)
	require.NotNil(t, status)
	require.Equal(t, "confirmed", status.State)
	require.Equal(t, "utxo-tx-hash-abc", status.TxHash)
	require.Equal(t, orderID, status.OrderID)
	require.Equal(t, "complete", status.SettlementAction)
}
