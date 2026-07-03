package settlement

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ExecuteConditionalSettlement verifies module evidence and invokes Core's standard confirm action.
func (s *SettlementService) ExecuteConditionalSettlement(
	ctx context.Context,
	attestation extensions.SettlementAttestation,
) (*payment.ActionResult, iwallet.CoinType, error) {
	if err := attestation.Validate(time.Now().UTC()); err != nil {
		return nil, "", fmt.Errorf("%w: %v", coreiface.ErrBadRequest, err)
	}
	if s == nil || s.db == nil {
		return nil, "", fmt.Errorf("database not initialized")
	}
	var order models.Order
	var extension extensions.OrderExtension
	if err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("id = ?", attestation.OrderID).First(&order).Error; err != nil {
			return err
		}
		var err error
		extension, err = orderextensions.LatestByIDTx(tx, attestation.OrderID, attestation.ExtensionID)
		return err
	}); err != nil {
		return nil, "", err
	}
	orderTenantID := strings.TrimSpace(order.TenantID)
	if orderTenantID == "" {
		orderTenantID = database.StandaloneTenantID
	}
	if strings.TrimSpace(attestation.TenantID) != orderTenantID {
		return nil, "", fmt.Errorf("%w: settlement attestation tenant mismatch", coreiface.ErrBadRequest)
	}
	if extension.ProviderID != attestation.Issuer ||
		extension.ExtensionID != attestation.ExtensionID ||
		extension.SettlementPolicy != extensions.SettlementPolicyExtensionAttested ||
		attestation.ConditionType != extensions.ConditionOrderExtensionDeliveryConfirmed ||
		attestation.ConditionVersion != extensions.ContractVersionV1 {
		return nil, "", fmt.Errorf("%w: settlement attestation is not authorized for this extension", coreiface.ErrBadRequest)
	}
	if extension.Revision != attestation.ExpectedExtensionRevision {
		return nil, "", fmt.Errorf("%w: settlement attestation expected extension revision %d, current revision is %d", coreiface.ErrBadRequest, attestation.ExpectedExtensionRevision, extension.Revision)
	}
	if err := validateConditionalSettlementOrder(&order); err != nil {
		return nil, "", err
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, "", fmt.Errorf("%w: payment not recorded for this order", coreiface.ErrBadRequest)
	}
	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return nil, "", fmt.Errorf("%w: payment settlement spec is missing", coreiface.ErrBadRequest)
	}
	if spec.GetMethod() != pb.PaymentSent_CANCELABLE {
		return nil, "", fmt.Errorf("%w: conditional settlement requires a cancelable escrow", coreiface.ErrBadRequest)
	}
	settlementID := orderextensions.SettlementIDFromPaymentSent(paymentSent)
	if settlementID == "" || settlementID != strings.TrimSpace(attestation.SettlementID) {
		return nil, "", fmt.Errorf("%w: settlement attestation settlement mismatch", coreiface.ErrBadRequest)
	}
	reference, err := orderextensions.SettlementReferenceForOrder(&order)
	if err != nil || reference.OrderStateVersion != strings.TrimSpace(attestation.ExpectedOrderStateVersion) {
		return nil, "", fmt.Errorf("%w: settlement attestation order state version is stale", coreiface.ErrBadRequest)
	}
	if s.attestationVerifier == nil {
		return nil, "", fmt.Errorf("%w: no attestation verifier for issuer %q", coreiface.ErrBadRequest, attestation.Issuer)
	}
	verifier := s.attestationVerifier(attestation.Issuer)
	if verifier == nil {
		return nil, "", fmt.Errorf("%w: no attestation verifier for issuer %q", coreiface.ErrBadRequest, attestation.Issuer)
	}
	if err := verifier.VerifySettlementAttestation(ctx, attestation, extension); err != nil {
		return nil, "", fmt.Errorf("%w: settlement attestation verification failed: %v", coreiface.ErrBadRequest, err)
	}

	// Re-read and compare the Core-issued state version while holding the same
	// lock used for settlement submission. This closes the verifier-to-command
	// TOCTOU window without granting the module access to Core state.
	s.settlementActionMu.Lock()
	defer s.settlementActionMu.Unlock()
	var current models.Order
	var currentExtension extensions.OrderExtension
	if err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("id = ?", attestation.OrderID).First(&current).Error; err != nil {
			return err
		}
		var loadErr error
		currentExtension, loadErr = orderextensions.LatestByIDTx(tx, attestation.OrderID, attestation.ExtensionID)
		return loadErr
	}); err != nil {
		return nil, "", err
	}
	if currentExtension.Revision != attestation.ExpectedExtensionRevision {
		return nil, "", fmt.Errorf("%w: settlement attestation extension revision is stale", coreiface.ErrBadRequest)
	}
	if err := validateConditionalSettlementOrder(&current); err != nil {
		return nil, "", err
	}
	currentPayment, err := current.PaymentSentMessage()
	if err != nil || orderextensions.SettlementIDFromPaymentSent(currentPayment) != strings.TrimSpace(attestation.SettlementID) {
		return nil, "", fmt.Errorf("%w: settlement attestation settlement is stale", coreiface.ErrBadRequest)
	}
	currentReference, err := orderextensions.SettlementReferenceForOrder(&current)
	if err != nil || currentReference.OrderStateVersion != strings.TrimSpace(attestation.ExpectedOrderStateVersion) {
		return nil, "", fmt.Errorf("%w: settlement attestation order state version is stale", coreiface.ErrBadRequest)
	}
	if err := s.db.Update(func(tx database.Tx) error {
		return orderextensions.RecordAttestationTx(tx, attestation, time.Now().UTC())
	}); err != nil {
		return nil, "", fmt.Errorf("%w: record settlement attestation: %v", coreiface.ErrBadRequest, err)
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(currentPayment)
	if err != nil {
		return nil, "", err
	}
	if current.SerializedOrderConfirmation != nil {
		return &payment.ActionResult{Mode: payment.ActionModeCompleted}, coinType, nil
	}
	payoutAddress, err := s.GetPayoutAddress(coinType.String())
	if err != nil {
		return nil, coinType, fmt.Errorf("resolve Core-owned seller payout address: %w", err)
	}
	return s.executeSettlementActionForOrderLocked(ctx, payment.SettlementActionConfirm, &current, payoutAddress.String())
}

func validateConditionalSettlementOrder(order *models.Order) error {
	if order == nil {
		return fmt.Errorf("%w: order is required", coreiface.ErrBadRequest)
	}
	if order.Role() != models.RoleVendor {
		return fmt.Errorf("%w: conditional settlement requires the seller node", coreiface.ErrBadRequest)
	}
	if !order.IsPaymentVerified() {
		return fmt.Errorf("%w: conditional settlement requires verified payment", coreiface.ErrBadRequest)
	}
	return nil
}
