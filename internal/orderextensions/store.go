package orderextensions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PersistTx appends a Core-owned extension revision in the caller's transaction.
func PersistTx(tx database.Tx, orderID string, extension extensions.OrderExtension) error {
	if tx == nil {
		return fmt.Errorf("order extension transaction is required")
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return fmt.Errorf("order extension order ID is required")
	}
	if err := extension.ValidateForOrder(orderID); err != nil {
		return err
	}

	lifecycleEvents, err := json.Marshal(extension.LifecycleEvents)
	if err != nil {
		return fmt.Errorf("marshal order extension lifecycle events: %w", err)
	}
	var latest models.OrderExtensionRecord
	err = tx.Read().Where("extension_id = ?", extension.ExtensionID).
		Order("revision DESC").First(&latest).Error
	if err == nil {
		if latest.OrderID != orderID || latest.ProviderID != extension.ProviderID || latest.ExtensionType != extension.Type || latest.ResourceID != extension.ResourceID || latest.ReservationRequired != extension.ReservationRequired || latest.SettlementPolicy != string(extension.SettlementPolicy) || string(latest.LifecycleEvents) != string(lifecycleEvents) {
			return fmt.Errorf("order extension identity fields are immutable")
		}
		if latest.PayloadHash == extension.PayloadHash && latest.SchemaVersion == extension.SchemaVersion {
			return ensureEventSequenceTx(tx, orderID, extension.ExtensionID)
		}
		extension.Revision = latest.Revision + 1
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("load latest order extension: %w", err)
	} else {
		extension.Revision = 1
	}

	createdAt := time.Now().UTC()
	if err := tx.Create(&models.OrderExtensionRecord{
		ExtensionID:         extension.ExtensionID,
		OrderID:             orderID,
		ProviderID:          extension.ProviderID,
		ExtensionType:       extension.Type,
		SchemaVersion:       extension.SchemaVersion,
		Revision:            extension.Revision,
		ResourceID:          extension.ResourceID,
		ReservationRequired: extension.ReservationRequired,
		SettlementPolicy:    string(extension.SettlementPolicy),
		LifecycleEvents:     lifecycleEvents,
		Payload:             append([]byte(nil), extension.Payload...),
		PayloadHash:         extension.PayloadHash,
		CreatedAt:           createdAt,
	}); err != nil {
		return err
	}
	return ensureEventSequenceTx(tx, orderID, extension.ExtensionID)
}

// LatestByIDTx returns the newest tenant-scoped extension revision.
func LatestByIDTx(tx database.Tx, orderID, extensionID string) (extensions.OrderExtension, error) {
	var record models.OrderExtensionRecord
	err := tx.Read().Where("order_id = ? AND extension_id = ?", strings.TrimSpace(orderID), strings.TrimSpace(extensionID)).
		Order("revision DESC").First(&record).Error
	if err != nil {
		return extensions.OrderExtension{}, err
	}
	return extensionFromRecord(record)
}

// LatestByIDGorm returns the newest extension revision from a scoped GORM transaction.
func LatestByIDGorm(db *gorm.DB, tenantID, orderID, extensionID string) (extensions.OrderExtension, error) {
	if db == nil {
		return extensions.OrderExtension{}, fmt.Errorf("order extension database is required")
	}
	if tenantID = strings.TrimSpace(tenantID); tenantID == "" {
		return extensions.OrderExtension{}, fmt.Errorf("order extension tenant ID is required")
	}
	query := db.Where("tenant_id = ? AND order_id = ? AND extension_id = ?", tenantID, strings.TrimSpace(orderID), strings.TrimSpace(extensionID))
	var record models.OrderExtensionRecord
	if err := query.Order("revision DESC").First(&record).Error; err != nil {
		return extensions.OrderExtension{}, err
	}
	return extensionFromRecord(record)
}

// LatestByOrderTx returns one newest revision for every extension attached to
// an order. Product-specific signed-order parsing is intentionally not used.
func LatestByOrderTx(tx database.Tx, orderID string) ([]extensions.OrderExtension, error) {
	if tx == nil {
		return nil, fmt.Errorf("order extension transaction is required")
	}
	if !tx.Read().Migrator().HasTable(&models.OrderExtensionRecord{}) {
		return nil, nil
	}
	var records []models.OrderExtensionRecord
	if err := tx.Read().Where("order_id = ?", strings.TrimSpace(orderID)).
		Order("extension_id ASC, revision DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	return latestExtensions(records)
}

// LatestByOrderGorm is the tenant-scoped GORM variant of LatestByOrderTx.
func LatestByOrderGorm(db *gorm.DB, tenantID, orderID string) ([]extensions.OrderExtension, error) {
	if db == nil {
		return nil, fmt.Errorf("order extension database is required")
	}
	if !db.Migrator().HasTable(&models.OrderExtensionRecord{}) {
		return nil, nil
	}
	if tenantID = strings.TrimSpace(tenantID); tenantID == "" {
		return nil, fmt.Errorf("order extension tenant ID is required")
	}
	query := db.Where("tenant_id = ? AND order_id = ?", tenantID, strings.TrimSpace(orderID))
	var records []models.OrderExtensionRecord
	if err := query.Order("extension_id ASC, revision DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	return latestExtensions(records)
}

// RequiresAttestedSettlementTx reports whether any persisted extension blocks
// Core's default confirmation path for this order.
func RequiresAttestedSettlementTx(tx database.Tx, orderID string) (bool, error) {
	declared, err := LatestByOrderTx(tx, orderID)
	if err != nil {
		return false, err
	}
	for _, extension := range declared {
		if extension.SettlementPolicy == extensions.SettlementPolicyExtensionAttested {
			return true, nil
		}
	}
	return false, nil
}

// EnsureSettlementMethodAllowedTx rejects rails that cannot execute the
// conditional settlement promised by an extension-attested declaration.
func EnsureSettlementMethodAllowedTx(tx database.Tx, orderID string, method pb.PaymentSent_Method) error {
	required, err := RequiresAttestedSettlementTx(tx, orderID)
	if err != nil {
		return err
	}
	if required && method != pb.PaymentSent_CANCELABLE {
		return fmt.Errorf("extension-attested order requires CANCELABLE settlement, got %s", method.String())
	}
	return nil
}

func latestExtensions(records []models.OrderExtensionRecord) ([]extensions.OrderExtension, error) {
	latest := make([]extensions.OrderExtension, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if _, exists := seen[record.ExtensionID]; exists {
			continue
		}
		seen[record.ExtensionID] = struct{}{}
		extension, err := extensionFromRecord(record)
		if err != nil {
			return nil, err
		}
		latest = append(latest, extension)
	}
	return latest, nil
}

// RecordReservationTx persists the provider reservation returned before a
// funding target is created. Repeated calls must describe the same binding.
func RecordReservationTx(tx database.Tx, request extensions.ReservationRequest, reservation extensions.Reservation) error {
	if tx == nil {
		return fmt.Errorf("order extension reservation transaction is required")
	}
	if err := request.Validate(time.Now().UTC()); err != nil {
		return err
	}
	if err := reservation.Validate(); err != nil {
		return err
	}
	var existing models.OrderExtensionReservationRecord
	err := tx.Read().Where("order_id = ? AND extension_id = ?", request.OrderID, request.Extension.ExtensionID).First(&existing).Error
	if err == nil {
		if existing.ProviderID != request.Extension.ProviderID || existing.ReservationID != reservation.ID ||
			existing.ReservationVersion != reservation.Version || existing.IdempotencyKey != request.IdempotencyKey ||
			existing.PaymentCoin != request.PaymentCoin || existing.ExtensionRevision != request.Extension.Revision {
			return fmt.Errorf("order extension reservation binding is immutable")
		}
		if existing.Status != reservation.Status {
			return fmt.Errorf("order extension reservation binding is immutable")
		}
		// Provisioning may pass through both the public session service and its
		// underlying payment facade. The provider owns one idempotent reservation
		// and may monotonically extend its lease between those calls. Preserve the
		// longest observed lease without weakening any identity or policy binding.
		if request.ExpiresAt.After(existing.ExpiresAt) {
			existing.ExpiresAt = request.ExpiresAt
			return tx.Save(&existing)
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&models.OrderExtensionReservationRecord{
		OrderID: request.OrderID, ExtensionID: request.Extension.ExtensionID, ProviderID: request.Extension.ProviderID,
		ReservationID: reservation.ID, ReservationVersion: reservation.Version, ExtensionRevision: request.Extension.Revision, Status: reservation.Status,
		PaymentCoin: request.PaymentCoin, IdempotencyKey: request.IdempotencyKey, ExpiresAt: request.ExpiresAt,
	})
}

// ReservationByExtensionTx loads the binding for an extension, if one exists.
func ReservationByExtensionTx(tx database.Tx, orderID, extensionID string) (*extensions.ReservationBinding, error) {
	var record models.OrderExtensionReservationRecord
	err := tx.Read().Where("order_id = ? AND extension_id = ?", strings.TrimSpace(orderID), strings.TrimSpace(extensionID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return reservationBinding(record), nil
}

// ReservationByExtensionGorm is the tenant-scoped GORM variant.
func ReservationByExtensionGorm(db *gorm.DB, tenantID, orderID, extensionID string) (*extensions.ReservationBinding, error) {
	if db == nil {
		return nil, fmt.Errorf("order extension database is required")
	}
	if tenantID = strings.TrimSpace(tenantID); tenantID == "" {
		return nil, fmt.Errorf("order extension tenant ID is required")
	}
	query := db.Where("tenant_id = ? AND order_id = ? AND extension_id = ?", tenantID, strings.TrimSpace(orderID), strings.TrimSpace(extensionID))
	var record models.OrderExtensionReservationRecord
	if err := query.First(&record).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return reservationBinding(record), nil
}

func reservationBinding(record models.OrderExtensionReservationRecord) *extensions.ReservationBinding {
	return &extensions.ReservationBinding{
		ReservationID: record.ReservationID, ReservationVersion: record.ReservationVersion, ExtensionRevision: record.ExtensionRevision, Status: record.Status,
		PaymentCoin: record.PaymentCoin, IdempotencyKey: record.IdempotencyKey, ExpiresAt: record.ExpiresAt, RecordedAt: record.CreatedAt,
	}
}

// ProjectReservations copies immutable provider reservation bindings to a
// co-tenant counterparty order before verified payment is processed there.
// The target must already contain the exact same persisted extension
// revision and payload; this prevents a reservation from being attached to a
// different declaration while allowing each tenant's Core state machine to
// emit a complete, independently versioned lifecycle event.
func ProjectReservations(source, target database.Database, orderID string) error {
	if source == nil || target == nil || strings.TrimSpace(orderID) == "" {
		return fmt.Errorf("order extension reservation projection requires source, target, and order ID")
	}
	type projection struct {
		extension extensions.OrderExtension
		binding   extensions.ReservationBinding
	}
	var projected []projection
	if err := source.View(func(tx database.Tx) error {
		declared, err := LatestByOrderTx(tx, orderID)
		if err != nil {
			return err
		}
		for _, extension := range declared {
			binding, err := ReservationByExtensionTx(tx, orderID, extension.ExtensionID)
			if err != nil {
				return err
			}
			if binding == nil {
				continue
			}
			if binding.ExtensionRevision != extension.Revision {
				return fmt.Errorf("source order extension %s reservation revision %d does not match declaration revision %d", extension.ExtensionID, binding.ExtensionRevision, extension.Revision)
			}
			projected = append(projected, projection{extension: extension, binding: *binding})
		}
		return nil
	}); err != nil {
		return fmt.Errorf("load source order extension reservations: %w", err)
	}
	if len(projected) == 0 {
		return nil
	}
	return target.Update(func(tx database.Tx) error {
		declared, err := LatestByOrderTx(tx, orderID)
		if err != nil {
			return err
		}
		targetByID := make(map[string]extensions.OrderExtension, len(declared))
		for _, extension := range declared {
			targetByID[extension.ExtensionID] = extension
		}
		for _, item := range projected {
			targetExtension, ok := targetByID[item.extension.ExtensionID]
			if !ok || !sameReservationDeclaration(targetExtension, item.extension) {
				return fmt.Errorf("target order extension %s does not match reservation declaration", item.extension.ExtensionID)
			}
			if err := RecordReservationTx(tx, extensions.ReservationRequest{
				OrderID: orderID, Extension: targetExtension, PaymentCoin: item.binding.PaymentCoin,
				IdempotencyKey: item.binding.IdempotencyKey, ExpiresAt: item.binding.ExpiresAt,
			}, extensions.Reservation{
				ID: item.binding.ReservationID, Version: item.binding.ReservationVersion, Status: item.binding.Status,
			}); err != nil {
				return fmt.Errorf("project order extension reservation %s: %w", item.extension.ExtensionID, err)
			}
		}
		return nil
	})
}

func sameReservationDeclaration(left, right extensions.OrderExtension) bool {
	return left.ExtensionID == right.ExtensionID &&
		left.ProviderID == right.ProviderID &&
		left.Type == right.Type &&
		left.SchemaVersion == right.SchemaVersion &&
		left.Revision == right.Revision &&
		left.ResourceID == right.ResourceID &&
		left.ReservationRequired == right.ReservationRequired &&
		left.SettlementPolicy == right.SettlementPolicy &&
		slices.Equal(left.LifecycleEvents, right.LifecycleEvents) &&
		left.PayloadHash == right.PayloadHash &&
		bytes.Equal(left.Payload, right.Payload)
}

// SettlementReferenceForOrder derives the opaque financial-state version that
// a Controller must echo in a settlement attestation. Any relevant order or
// payment transition changes the digest and makes stale evidence fail closed.
func SettlementReferenceForOrder(order *models.Order) (extensions.SettlementReference, error) {
	if order == nil {
		return extensions.SettlementReference{}, fmt.Errorf("order is required")
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return extensions.SettlementReference{}, err
	}
	settlementID := SettlementIDFromPaymentSent(paymentSent)
	if settlementID == "" {
		return extensions.SettlementReference{}, fmt.Errorf("payment settlement identity is required")
	}
	canonical := struct {
		OrderID             string `json:"orderID"`
		Role                string `json:"role"`
		State               string `json:"state"`
		PaymentVerification string `json:"paymentVerification"`
		PaymentSent         string `json:"paymentSent"`
		Confirmation        string `json:"confirmation"`
		Cancel              string `json:"cancel"`
		Decline             string `json:"decline"`
		Refunds             string `json:"refunds"`
	}{
		OrderID: order.ID.String(), Role: string(order.Role()), State: order.State.String(),
		PaymentVerification: string(order.CurrentPaymentVerificationStatus()),
		PaymentSent:         digestBytes(order.SerializedPaymentSent), Confirmation: digestBytes(order.SerializedOrderConfirmation),
		Cancel: digestBytes(order.SerializedOrderCancel), Decline: digestBytes(order.SerializedOrderDecline), Refunds: digestBytes(order.SerializedRefunds),
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return extensions.SettlementReference{}, err
	}
	digest := sha256.Sum256(encoded)
	return extensions.SettlementReference{SettlementID: settlementID, OrderStateVersion: "sha256:" + hex.EncodeToString(digest[:])}, nil
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

// SettlementIDFromPaymentSent derives the stable escrow/payment identity used
// by Core events and conditional settlement attestations.
func SettlementIDFromPaymentSent(paymentSent *pb.PaymentSent) string {
	if paymentSent == nil {
		return ""
	}
	if value := strings.TrimSpace(paymentSent.GetToAddress()); value != "" {
		return value
	}
	if value := strings.TrimSpace(paymentSent.GetContractAddress()); value != "" {
		return value
	}
	return strings.TrimSpace(paymentSent.GetTransactionID())
}

// EnqueueTx inserts an event once in the caller's tenant transaction.
func EnqueueTx(tx database.Tx, event extensions.Event) error {
	if tx == nil {
		return fmt.Errorf("extension delivery transaction is required")
	}
	if err := validateEventBeforeVersionAssignment(event); err != nil {
		return err
	}
	var existing models.ExtensionDelivery
	err := tx.Read().Where("event_id = ?", event.EventID).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	orderVersion, err := nextEventVersionTx(tx, event.OrderID, event.ExtensionID)
	if err != nil {
		return fmt.Errorf("assign extension event order version: %w", err)
	}
	event.OrderVersion = orderVersion
	if err := event.Validate(); err != nil {
		return err
	}
	return tx.Create(deliveryFromEvent(event))
}

// EnqueueGorm inserts an event once through a scoped GORM transaction.
func EnqueueGorm(tx *gorm.DB, event extensions.Event) error {
	if tx == nil {
		return fmt.Errorf("extension delivery transaction is required")
	}
	if err := validateEventBeforeVersionAssignment(event); err != nil {
		return err
	}
	var existing models.ExtensionDelivery
	lookup := tx.Where("tenant_id = ? AND event_id = ?", strings.TrimSpace(event.TenantID), event.EventID).First(&existing)
	if lookup.Error == nil {
		return nil
	}
	if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
		return lookup.Error
	}
	orderVersion, err := nextEventVersionGorm(tx, event.TenantID, event.OrderID, event.ExtensionID)
	if err != nil {
		return fmt.Errorf("assign extension event order version: %w", err)
	}
	event.OrderVersion = orderVersion
	if err := event.Validate(); err != nil {
		return err
	}
	record := deliveryFromEvent(event)
	record.TenantID = strings.TrimSpace(event.TenantID)
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error
}

// EventFromDelivery reconstructs the public event envelope from its outbox record.
func EventFromDelivery(record models.ExtensionDelivery) extensions.Event {
	return extensions.Event{
		EventID:        record.EventID,
		ProviderID:     record.ProviderID,
		Type:           record.EventType,
		Version:        record.EventVersion,
		TenantID:       record.TenantID,
		SourceID:       record.SourceID,
		OrderRole:      record.OrderRole,
		OrderID:        record.OrderID,
		OrderVersion:   record.OrderVersion,
		ExtensionID:    record.ExtensionID,
		IdempotencyKey: record.IdempotencyKey,
		OccurredAt:     record.CreatedAt,
		Payload:        append([]byte(nil), record.Payload...),
	}
}

// RequeueDeliveryTx resets a dead-lettered or delayed delivery for operator replay.
func RequeueDeliveryTx(tx database.Tx, eventID string) error {
	if tx == nil || strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("extension delivery transaction and event ID are required")
	}
	updated, err := tx.UpdateColumns(map[string]interface{}{
		"attempts":         0,
		"last_error":       "",
		"next_attempt_at":  nil,
		"delivered_at":     nil,
		"dead_lettered_at": nil,
		"lease_owner":      "",
		"lease_expires_at": nil,
	}, map[string]interface{}{"event_id = ?": strings.TrimSpace(eventID)}, &models.ExtensionDelivery{})
	if err != nil {
		return err
	}
	if updated == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// RecordAttestationTx records accepted evidence and rejects conflicting reuse.
func RecordAttestationTx(tx database.Tx, attestation extensions.SettlementAttestation, acceptedAt time.Time) error {
	if tx == nil {
		return fmt.Errorf("settlement attestation transaction is required")
	}
	replayFingerprint, err := settlementAttestationReplayFingerprint(attestation)
	if err != nil {
		return err
	}
	var existing models.SettlementAttestationRecord
	err = tx.Read().Where("attestation_id = ? OR idempotency_key = ? OR replay_fingerprint = ?", attestation.AttestationID, attestation.IdempotencyKey, replayFingerprint).First(&existing).Error
	if err == nil {
		if existing.OrderID != attestation.OrderID || existing.ExtensionID != attestation.ExtensionID ||
			existing.Issuer != attestation.Issuer || existing.SettlementID != attestation.SettlementID ||
			existing.ExpectedExtensionRevision != attestation.ExpectedExtensionRevision || existing.EvidenceDigest != attestation.EvidenceDigest ||
			existing.ExpectedOrderStateVersion != attestation.ExpectedOrderStateVersion ||
			existing.ConditionType != attestation.ConditionType || existing.ConditionVersion != attestation.ConditionVersion {
			return fmt.Errorf("settlement attestation ID was reused with different evidence")
		}
		if existing.AttestationID != attestation.AttestationID && existing.IdempotencyKey != attestation.IdempotencyKey {
			return fmt.Errorf("settlement attestation evidence was replayed")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&models.SettlementAttestationRecord{
		TenantID:                  strings.TrimSpace(attestation.TenantID),
		AttestationID:             attestation.AttestationID,
		IdempotencyKey:            attestation.IdempotencyKey,
		ReplayFingerprint:         replayFingerprint,
		Issuer:                    attestation.Issuer,
		OrderID:                   attestation.OrderID,
		ExtensionID:               attestation.ExtensionID,
		SettlementID:              attestation.SettlementID,
		ExpectedExtensionRevision: attestation.ExpectedExtensionRevision,
		ExpectedOrderStateVersion: attestation.ExpectedOrderStateVersion,
		ConditionType:             attestation.ConditionType,
		ConditionVersion:          attestation.ConditionVersion,
		EvidenceDigest:            attestation.EvidenceDigest,
		ObservedAt:                attestation.ObservedAt,
		ExpiresAt:                 attestation.ExpiresAt,
		AcceptedAt:                acceptedAt,
	})
}

// ConfirmationAuthorization is the Core-owned proof carried only by an
// internal auto-confirm event after an attested settlement has executed.
type ConfirmationAuthorization struct {
	AttestationID string
	ActionID      string
	TransactionID string
	PayoutAddress string
}

// BindAttestationExecutionTx binds accepted evidence to the exact Core-issued
// settlement execution before order confirmation can be emitted.
func BindAttestationExecutionTx(tx database.Tx, attestationID, actionID, transactionID, payoutAddress string) error {
	if tx == nil {
		return fmt.Errorf("settlement attestation transaction is required")
	}
	attestationID = strings.TrimSpace(attestationID)
	actionID = strings.TrimSpace(actionID)
	transactionID = strings.TrimSpace(transactionID)
	payoutAddress = strings.TrimSpace(payoutAddress)
	if attestationID == "" || (actionID == "" && transactionID == "") || payoutAddress == "" {
		return fmt.Errorf("settlement attestation execution requires attestation, action or transaction, and payout identities")
	}
	var record models.SettlementAttestationRecord
	if err := tx.Read().Where("attestation_id = ?", attestationID).First(&record).Error; err != nil {
		return err
	}
	if record.ExecutionActionID != "" && record.ExecutionActionID != actionID {
		return fmt.Errorf("settlement attestation execution action is immutable")
	}
	if record.ExecutionTransactionID != "" && transactionID != "" && record.ExecutionTransactionID != transactionID {
		return fmt.Errorf("settlement attestation execution transaction is immutable")
	}
	if record.ExecutionPayoutAddress != "" && record.ExecutionPayoutAddress != payoutAddress {
		return fmt.Errorf("settlement attestation execution payout is immutable")
	}
	if actionID != "" {
		var conflicting models.SettlementAttestationRecord
		err := tx.Read().Where(
			"order_id = ? AND execution_action_id = ? AND attestation_id <> ?",
			record.OrderID, actionID, record.AttestationID,
		).First(&conflicting).Error
		if err == nil {
			return fmt.Errorf("settlement action is already bound to another attestation")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if transactionID != "" {
		var conflicting models.SettlementAttestationRecord
		err := tx.Read().Where(
			"order_id = ? AND execution_transaction_id = ? AND attestation_id <> ?",
			record.OrderID, transactionID, record.AttestationID,
		).First(&conflicting).Error
		if err == nil {
			return fmt.Errorf("settlement transaction is already bound to another attestation")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	columns := map[string]interface{}{"execution_payout_address": payoutAddress}
	if actionID != "" {
		columns["execution_action_id"] = actionID
	}
	if transactionID != "" {
		columns["execution_transaction_id"] = transactionID
	}
	updated, err := tx.UpdateColumns(columns, map[string]interface{}{"attestation_id = ?": attestationID}, &models.SettlementAttestationRecord{})
	if err != nil {
		return err
	}
	if updated != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// AuthorizationForSettlementActionTx resolves and finalizes the attestation
// bound to a confirmed backend settlement action.
func AuthorizationForSettlementActionTx(tx database.Tx, order *models.Order, actionID, transactionID string) (ConfirmationAuthorization, error) {
	if tx == nil || order == nil {
		return ConfirmationAuthorization{}, fmt.Errorf("settlement confirmation transaction and order are required")
	}
	actionID = strings.TrimSpace(actionID)
	transactionID = strings.TrimSpace(transactionID)
	if actionID == "" || transactionID == "" {
		return ConfirmationAuthorization{}, fmt.Errorf("settlement confirmation action and transaction IDs are required")
	}
	var record models.SettlementAttestationRecord
	if err := tx.Read().Where("order_id = ? AND execution_action_id = ?", order.ID.String(), actionID).First(&record).Error; err != nil {
		return ConfirmationAuthorization{}, err
	}
	if err := BindAttestationExecutionTx(tx, record.AttestationID, actionID, transactionID, record.ExecutionPayoutAddress); err != nil {
		return ConfirmationAuthorization{}, err
	}
	record.ExecutionTransactionID = transactionID
	return validateConfirmationAuthorizationRecord(tx, order, record, transactionID)
}

// ValidateConfirmationAuthorizationTx verifies the internal authorization
// again under the order lock immediately before the Core state transition.
func ValidateConfirmationAuthorizationTx(tx database.Tx, order *models.Order, attestationID, transactionID string) (ConfirmationAuthorization, error) {
	if tx == nil || order == nil {
		return ConfirmationAuthorization{}, fmt.Errorf("settlement confirmation transaction and order are required")
	}
	var record models.SettlementAttestationRecord
	if err := tx.Read().Where("attestation_id = ?", strings.TrimSpace(attestationID)).First(&record).Error; err != nil {
		return ConfirmationAuthorization{}, err
	}
	return validateConfirmationAuthorizationRecord(tx, order, record, strings.TrimSpace(transactionID))
}

func validateConfirmationAuthorizationRecord(tx database.Tx, order *models.Order, record models.SettlementAttestationRecord, transactionID string) (ConfirmationAuthorization, error) {
	if record.OrderID != order.ID.String() || record.ExecutionTransactionID == "" || record.ExecutionTransactionID != transactionID || record.ExecutionPayoutAddress == "" {
		return ConfirmationAuthorization{}, fmt.Errorf("settlement confirmation execution binding mismatch")
	}
	extension, err := LatestByIDTx(tx, order.ID.String(), record.ExtensionID)
	if err != nil {
		return ConfirmationAuthorization{}, err
	}
	if extension.ProviderID != record.Issuer || extension.Revision != record.ExpectedExtensionRevision ||
		extension.SettlementPolicy != extensions.SettlementPolicyExtensionAttested ||
		record.ConditionType != extensions.ConditionOrderExtensionDeliveryConfirmed || record.ConditionVersion != extensions.ContractVersionV1 {
		return ConfirmationAuthorization{}, fmt.Errorf("settlement confirmation attestation is no longer authorized")
	}
	reference, err := SettlementReferenceForOrder(order)
	if err != nil || reference.SettlementID != record.SettlementID || reference.OrderStateVersion != record.ExpectedOrderStateVersion {
		return ConfirmationAuthorization{}, fmt.Errorf("settlement confirmation order state is stale")
	}
	orderTenantID := strings.TrimSpace(order.TenantID)
	if orderTenantID == "" {
		orderTenantID = database.StandaloneTenantID
	}
	if record.TenantID != orderTenantID {
		return ConfirmationAuthorization{}, fmt.Errorf("settlement confirmation tenant mismatch")
	}
	return ConfirmationAuthorization{
		AttestationID: record.AttestationID,
		ActionID:      record.ExecutionActionID,
		TransactionID: record.ExecutionTransactionID,
		PayoutAddress: record.ExecutionPayoutAddress,
	}, nil
}

func ensureEventSequenceTx(tx database.Tx, orderID, extensionID string) error {
	var sequence models.OrderExtensionEventSequence
	err := tx.Read().Where("extension_id = ?", strings.TrimSpace(extensionID)).First(&sequence).Error
	if err == nil {
		if sequence.OrderID != strings.TrimSpace(orderID) {
			return fmt.Errorf("order extension event sequence order mismatch")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&models.OrderExtensionEventSequence{
		ExtensionID: strings.TrimSpace(extensionID),
		OrderID:     strings.TrimSpace(orderID),
	})
}

func nextEventVersionTx(tx database.Tx, orderID, extensionID string) (uint64, error) {
	if err := ensureEventSequenceTx(tx, orderID, extensionID); err != nil {
		return 0, err
	}
	for attempts := 0; attempts < 8; attempts++ {
		var sequence models.OrderExtensionEventSequence
		if err := tx.Read().Where("extension_id = ?", strings.TrimSpace(extensionID)).First(&sequence).Error; err != nil {
			return 0, err
		}
		next := sequence.LastVersion + 1
		updated, err := tx.UpdateColumns(
			map[string]interface{}{"last_version": next},
			map[string]interface{}{
				"extension_id = ?": strings.TrimSpace(extensionID),
				"last_version = ?": sequence.LastVersion,
			},
			&models.OrderExtensionEventSequence{},
		)
		if err != nil {
			return 0, err
		}
		if updated == 1 {
			return next, nil
		}
	}
	return 0, fmt.Errorf("order extension event sequence update conflicted repeatedly")
}

func nextEventVersionGorm(tx *gorm.DB, tenantID, orderID, extensionID string) (uint64, error) {
	tenantID = strings.TrimSpace(tenantID)
	orderID = strings.TrimSpace(orderID)
	extensionID = strings.TrimSpace(extensionID)
	sequence := models.OrderExtensionEventSequence{TenantID: tenantID, ExtensionID: extensionID, OrderID: orderID}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&sequence).Error; err != nil {
		return 0, err
	}
	result := tx.Model(&models.OrderExtensionEventSequence{}).
		Where("tenant_id = ? AND extension_id = ? AND order_id = ?", tenantID, extensionID, orderID).
		UpdateColumn("last_version", gorm.Expr("last_version + 1"))
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, fmt.Errorf("order extension event sequence not found")
	}
	if err := tx.Where("tenant_id = ? AND extension_id = ?", tenantID, extensionID).First(&sequence).Error; err != nil {
		return 0, err
	}
	return sequence.LastVersion, nil
}

func validateEventBeforeVersionAssignment(event extensions.Event) error {
	event.OrderVersion = 1
	return event.Validate()
}

func settlementAttestationReplayFingerprint(attestation extensions.SettlementAttestation) (string, error) {
	canonical := struct {
		Issuer                    string `json:"issuer"`
		OrderID                   string `json:"orderID"`
		SettlementID              string `json:"settlementID"`
		ExtensionID               string `json:"extensionID"`
		ExpectedExtensionRevision uint64 `json:"expectedExtensionRevision"`
		ExpectedOrderStateVersion string `json:"expectedOrderStateVersion"`
		ConditionType             string `json:"conditionType"`
		ConditionVersion          string `json:"conditionVersion"`
		EvidenceDigest            string `json:"evidenceDigest"`
	}{
		Issuer:                    strings.TrimSpace(attestation.Issuer),
		OrderID:                   strings.TrimSpace(attestation.OrderID),
		SettlementID:              strings.TrimSpace(attestation.SettlementID),
		ExtensionID:               strings.TrimSpace(attestation.ExtensionID),
		ExpectedExtensionRevision: attestation.ExpectedExtensionRevision,
		ExpectedOrderStateVersion: strings.TrimSpace(attestation.ExpectedOrderStateVersion),
		ConditionType:             strings.TrimSpace(attestation.ConditionType),
		ConditionVersion:          strings.TrimSpace(attestation.ConditionVersion),
		EvidenceDigest:            strings.TrimSpace(attestation.EvidenceDigest),
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal settlement attestation replay identity: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func extensionFromRecord(record models.OrderExtensionRecord) (extensions.OrderExtension, error) {
	var lifecycleEvents []string
	if err := json.Unmarshal(record.LifecycleEvents, &lifecycleEvents); err != nil {
		return extensions.OrderExtension{}, fmt.Errorf("decode order extension lifecycle events: %w", err)
	}
	extension := extensions.OrderExtension{
		ExtensionID:         record.ExtensionID,
		ProviderID:          record.ProviderID,
		Type:                record.ExtensionType,
		SchemaVersion:       record.SchemaVersion,
		Revision:            record.Revision,
		ResourceID:          record.ResourceID,
		ReservationRequired: record.ReservationRequired,
		SettlementPolicy:    extensions.SettlementPolicy(record.SettlementPolicy),
		LifecycleEvents:     lifecycleEvents,
		Payload:             append([]byte(nil), record.Payload...),
		PayloadHash:         record.PayloadHash,
		CreatedAt:           record.CreatedAt,
	}
	if err := extension.ValidateForOrder(record.OrderID); err != nil {
		return extensions.OrderExtension{}, fmt.Errorf("validate persisted order extension: %w", err)
	}
	return extension, nil
}

func deliveryFromEvent(event extensions.Event) *models.ExtensionDelivery {
	return &models.ExtensionDelivery{
		TenantID:       strings.TrimSpace(event.TenantID),
		EventID:        event.EventID,
		SourceID:       event.SourceID,
		OrderRole:      event.OrderRole,
		ProviderID:     event.ProviderID,
		EventType:      event.Type,
		EventVersion:   event.Version,
		OrderID:        event.OrderID,
		OrderVersion:   event.OrderVersion,
		ExtensionID:    event.ExtensionID,
		IdempotencyKey: event.IdempotencyKey,
		Payload:        append([]byte(nil), event.Payload...),
		CreatedAt:      event.OccurredAt,
	}
}
