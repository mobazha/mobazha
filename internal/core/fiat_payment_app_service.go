package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/payment/fiat/paypal"
	"github.com/mobazha/mobazha/internal/payment/fiat/stripe"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

// FiatChainType maps a provider ID to a ChainType for ReceivingAccount lookup.
func FiatChainType(providerID string) iwallet.ChainType {
	return iwallet.ChainType("fiat:" + providerID)
}

// FiatWebhookHandler is called by FiatPaymentAppService when a webhook event
// triggers an order-level action. The node layer implements this to wire into
// the order processing engine.
type FiatWebhookHandler func(ctx context.Context, event *contracts.WebhookEvent) error

// FiatPaymentAppService orchestrates fiat payment operations and implements contracts.FiatService.
type FiatPaymentAppService struct {
	registry             contracts.FiatProviderRegistry
	db                   database.Database
	nodeID               string
	testnet              bool
	webhookHandler       FiatWebhookHandler
	orderRepo            contracts.OrderRepo
	eventBus             events.Bus
	platformMu           sync.RWMutex
	platformIDs          map[string]struct{}
	provisioningPolicies []corepayment.SessionProvisioningPolicy
}

func NewFiatPaymentAppService(
	registry contracts.FiatProviderRegistry,
	db database.Database,
	nodeID string,
	testnet bool,
) *FiatPaymentAppService {
	return &FiatPaymentAppService{
		registry:    registry,
		db:          db,
		nodeID:      nodeID,
		testnet:     testnet,
		platformIDs: make(map[string]struct{}),
	}
}

// SetWebhookHandler registers a callback for webhook-driven order actions.
// Must be called during node initialization before any webhooks are processed.
func (s *FiatPaymentAppService) SetWebhookHandler(h FiatWebhookHandler) {
	s.webhookHandler = h
}

// SetOrderRepo sets the order repository for webhook event handlers that need
// direct order access (refund, dispute, etc.).
func (s *FiatPaymentAppService) SetOrderRepo(repo contracts.OrderRepo) {
	s.orderRepo = repo
}

// SetEventBus injects the event bus for emitting FiatPaymentReady after successful capture.
func (s *FiatPaymentAppService) SetEventBus(bus events.Bus) {
	s.eventBus = bus
}

// AddProvisioningPolicy registers a policy at the provider boundary. Legacy
// provider-scoped REST routes and the unified payment-session facade both end
// up in CreatePayment, so this is the fail-closed enforcement point for fiat.
func (s *FiatPaymentAppService) AddProvisioningPolicy(policy corepayment.SessionProvisioningPolicy) {
	if policy != nil {
		s.provisioningPolicies = append(s.provisioningPolicies, policy)
	}
}

func (s *FiatPaymentAppService) EnabledProviders(ctx context.Context) ([]contracts.ProviderInfo, error) {
	registered := s.registry.Registered()
	result := make([]contracts.ProviderInfo, 0, len(registered))

	for _, pid := range registered {
		info := contracts.ProviderInfo{
			ProviderID: pid,
			Status:     "not_connected",
		}

		ra, err := s.getActiveAccount(pid)
		if err == nil && ra != nil {
			info.AccountID = ra.Address
			info.Status = "active"

			if onboarder, ok := s.providerAsOnboarder(pid); ok {
				status, err := onboarder.GetAccountStatus(ctx, ra.Address)
				if err == nil {
					info.Status = status.Status
				}
			}
		}

		result = append(result, info)
	}

	return result, nil
}

func (s *FiatPaymentAppService) CreatePayment(ctx context.Context, providerID string, params contracts.CreatePaymentParams) (*contracts.FiatProviderSession, error) {
	if strings.TrimSpace(params.OrderID) == "" {
		return nil, fmt.Errorf("create %s payment: order ID is required", providerID)
	}
	if strings.TrimSpace(s.nodeID) == "" {
		return nil, fmt.Errorf("create %s payment: stable node identity is required", providerID)
	}
	if err := s.authorizePaymentCreation(ctx, providerID, params); err != nil {
		return nil, err
	}
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, err
	}

	ra, err := s.getActiveAccount(providerID)
	if err != nil {
		return nil, fmt.Errorf("seller has no %s account configured: %w", providerID, err)
	}
	params.SellerAccountID = ra.Address
	attempt, route, err := s.prepareFiatPaymentAttempt(providerID, params, ra)
	if err != nil {
		return nil, err
	}
	params.IdempotencyKey = attempt.IdempotencyKey
	params.Metadata = cloneStringMap(params.Metadata)
	params.Metadata["mobazha_payment_attempt_id"] = attempt.AttemptID
	params.Metadata["mobazha_route_binding_id"] = route.RouteBindingID

	session, err := provider.CreatePayment(ctx, params)
	if err != nil {
		providerErr := fmt.Errorf("create %s payment: %w", providerID, err)
		if persistErr := s.markFiatAttemptReconcileRequired(attempt.AttemptID, providerErr); persistErr != nil {
			return nil, errors.Join(providerErr, persistErr)
		}
		return nil, providerErr
	}
	if session == nil || strings.TrimSpace(session.SessionID) == "" {
		providerErr := fmt.Errorf("create %s payment: provider returned an empty session", providerID)
		if persistErr := s.markFiatAttemptReconcileRequired(attempt.AttemptID, providerErr); persistErr != nil {
			return nil, errors.Join(providerErr, persistErr)
		}
		return nil, providerErr
	}
	if err := s.commitFiatPaymentAttempt(attempt.AttemptID, session.SessionID); err != nil {
		persistErr := fmt.Errorf("persist %s payment attempt: %w", providerID, err)
		if markErr := s.markFiatAttemptReconcileRequired(attempt.AttemptID, persistErr); markErr != nil {
			return nil, errors.Join(persistErr, markErr)
		}
		return nil, persistErr
	}

	if s.orderRepo != nil && params.OrderID != "" {
		meta := map[string]string{
			"fiat_provider":         providerID,
			"fiat_session_id":       session.SessionID,
			"payment_attempt_id":    attempt.AttemptID,
			"payment_route_binding": route.RouteBindingID,
			// fiat_currency is stored so the PaymentSessionProjector can reconstruct
			// the canonical paymentCoin ("fiat:{provider}:{currency}") for orders
			// where PaymentSent has not yet been written (e.g. awaiting buyer action).
			"fiat_currency": params.Currency,
		}
		if specJSON, err := payment.FiatMetadataSettlementSpecJSON(); err == nil {
			meta["settlement_spec"] = specJSON
		}
		if err := s.orderRepo.MergeFiatMetadata(ctx, params.OrderID, meta); err != nil {
			persistErr := fmt.Errorf("persist %s payment metadata: %w", providerID, err)
			if markErr := s.markFiatAttemptReconcileRequired(attempt.AttemptID, persistErr); markErr != nil {
				return nil, errors.Join(persistErr, markErr)
			}
			return nil, persistErr
		}
	}

	logger.LogInfoWithIDf(log, s.nodeID, "fiat payment created: provider=%s session=%s order=%s", providerID, session.SessionID, params.OrderID)
	return session, nil
}

func (s *FiatPaymentAppService) prepareFiatPaymentAttempt(providerID string, params contracts.CreatePaymentParams, account *models.ReceivingAccount) (models.PaymentAttempt, models.PaymentRouteBinding, error) {
	assetID := "fiat:" + strings.ToLower(strings.TrimSpace(providerID)) + ":" + strings.ToUpper(strings.TrimSpace(params.Currency))
	// The provider binding identifies the stable settlement destination, not the
	// mutable configuration row. Key rotation or status refreshes must not create
	// a second provider object for the same order; changing the external account
	// address intentionally creates a new binding.
	bindingSeed := fmt.Sprintf("%s|%s|%d|%s", strings.TrimSpace(s.nodeID), providerID, account.ID, account.Address)
	providerBindingID := stablePaymentIdentity("fpb_", bindingSeed)
	attemptSeed := fmt.Sprintf("%s|%s|%d|%s", strings.TrimSpace(params.OrderID), assetID, params.Amount, providerBindingID)
	attemptID := stablePaymentIdentity("pa_", attemptSeed)
	routeID := stablePaymentIdentity("prb_", attemptID)
	attempt := models.PaymentAttempt{
		AttemptID: attemptID, PaymentSessionID: "ps_" + strings.TrimSpace(params.OrderID), OrderID: strings.TrimSpace(params.OrderID),
		RouteBindingID: routeID, IdempotencyKey: stablePaymentIdentity("mbz_", attemptSeed), State: models.PaymentAttemptPendingExternal,
	}
	route := models.PaymentRouteBinding{
		RouteBindingID: routeID, AttemptID: attemptID,
		ContributionID: "core.fiat." + strings.ToLower(providerID), ModuleID: "mobazha.core.fiat." + strings.ToLower(providerID),
		ImplementationGeneration: "builtin-v1", RailKind: string(distribution.PaymentRailProviderSession),
		NetworkID: string(FiatChainType(providerID)), AssetID: assetID, ProtocolVersion: "provider-v1", StateSchemaVersion: "1",
		ProviderBindingID: providerBindingID, ExternalAccountReference: account.Address,
	}
	err := s.db.Update(func(tx database.Tx) error {
		var existing models.PaymentAttempt
		if err := tx.Read().Where("attempt_id = ?", attemptID).First(&existing).Error; err == nil {
			attempt = existing
			return tx.Read().Where("route_binding_id = ?", existing.RouteBindingID).First(&route).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&route); err != nil {
			return err
		}
		return tx.Create(&attempt)
	})
	if err == nil {
		return attempt, route, nil
	}
	// A concurrent caller may have committed the same deterministic claim.
	if loadErr := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("attempt_id = ?", attemptID).First(&attempt).Error; err != nil {
			return err
		}
		return tx.Read().Where("route_binding_id = ?", attempt.RouteBindingID).First(&route).Error
	}); loadErr == nil {
		return attempt, route, nil
	}
	return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("prepare fiat payment attempt: %w", err)
}

func (s *FiatPaymentAppService) commitFiatPaymentAttempt(attemptID, externalReference string) error {
	return s.db.Update(func(tx database.Tx) error {
		var attempt models.PaymentAttempt
		if err := tx.Read().Where("attempt_id = ?", attemptID).First(&attempt).Error; err != nil {
			return err
		}
		if attempt.ExternalReference != "" && attempt.ExternalReference != externalReference {
			return fmt.Errorf("payment attempt %s provider reference conflict: existing=%s returned=%s", attemptID, attempt.ExternalReference, externalReference)
		}
		rows, err := tx.UpdateColumns(map[string]interface{}{
			"state": models.PaymentAttemptExternalCreated, "external_reference": externalReference, "last_error": "",
		}, map[string]interface{}{"attempt_id = ?": attemptID}, &models.PaymentAttempt{})
		if err != nil {
			return err
		}
		if rows != 1 {
			return fmt.Errorf("payment attempt %s not found", attemptID)
		}
		return nil
	})
}

func (s *FiatPaymentAppService) markFiatAttemptReconcileRequired(attemptID string, cause error) error {
	return s.db.Update(func(tx database.Tx) error {
		rows, err := tx.UpdateColumns(map[string]interface{}{
			"state": models.PaymentAttemptReconcileRequired, "last_error": cause.Error(),
		}, map[string]interface{}{"attempt_id = ?": attemptID}, &models.PaymentAttempt{})
		if err != nil {
			return err
		}
		if rows != 1 {
			return fmt.Errorf("payment attempt %s not found", attemptID)
		}
		return nil
	})
}

func stablePaymentIdentity(prefix, value string) string {
	digest := sha256.Sum256([]byte(value))
	return prefix + fmt.Sprintf("%x", digest[:16])
}

func cloneStringMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input)+2)
	for key, value := range input {
		result[key] = value
	}
	return result
}

func (s *FiatPaymentAppService) authorizePaymentCreation(ctx context.Context, providerID string, params contracts.CreatePaymentParams) error {
	if len(s.provisioningPolicies) == 0 || strings.TrimSpace(params.OrderID) == "" {
		return nil
	}
	var order models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", params.OrderID).First(&order).Error
	}); err != nil {
		return fmt.Errorf("authorize fiat payment: load order %s: %w", params.OrderID, err)
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("authorize fiat payment: decode order %s: %w", params.OrderID, err)
	}
	expiresAt := time.Time{}
	if order.ExpiresAt != nil {
		expiresAt = order.ExpiresAt.UTC()
	}
	input := corepayment.SessionProvisioningPolicyInput{
		OrderID:               strings.TrimSpace(params.OrderID),
		PaymentCoin:           "fiat:" + strings.ToLower(strings.TrimSpace(providerID)) + ":" + strings.ToUpper(strings.TrimSpace(params.Currency)),
		SettlementMethod:      pb.PaymentSent_FIAT,
		SettlementMethodKnown: true,
		ExpiresAt:             expiresAt,
		OrderOpen:             orderOpen,
	}
	for _, policy := range s.provisioningPolicies {
		if err := policy.AuthorizeSessionProvisioning(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

func (s *FiatPaymentAppService) CapturePayment(ctx context.Context, providerID string, sessionID string) (*contracts.PaymentResult, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, err
	}

	result, err := provider.CapturePayment(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("capture %s payment: %w", providerID, err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "fiat payment captured: provider=%s session=%s status=%s", providerID, sessionID, result.Status)
	return result, nil
}

func (s *FiatPaymentAppService) GetPayment(ctx context.Context, providerID string, paymentID string) (*contracts.PaymentDetail, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, err
	}
	return provider.GetPayment(ctx, paymentID)
}

func (s *FiatPaymentAppService) RefundPayment(
	ctx context.Context,
	providerID string,
	params contracts.RefundParams,
) (*contracts.RefundResult, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, fmt.Errorf("unknown fiat provider %q: %w", providerID, err)
	}

	result, err := provider.RefundPayment(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fiat refund via %s: %w", providerID, err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "fiat refund %s via %s: status=%s amount=%d %s",
		result.RefundID, providerID, result.Status, result.Amount, result.Currency)

	return result, nil
}

// CancelPayment cancels a fiat payment session via the provider registry.
// Implements contracts.FiatPaymentOperations.
func (s *FiatPaymentAppService) CancelPayment(ctx context.Context, providerID string, paymentID string) error {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return err
	}
	return provider.CancelPayment(ctx, paymentID)
}

// GetPaymentStatus returns the normalized status of a fiat payment session.
// Implements contracts.FiatPaymentOperations.
func (s *FiatPaymentAppService) GetPaymentStatus(ctx context.Context, providerID string, paymentID string) (string, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return "", err
	}
	detail, err := provider.GetPayment(ctx, paymentID)
	if err != nil {
		return "", err
	}
	return detail.Status, nil
}

func (s *FiatPaymentAppService) HandleWebhook(ctx context.Context, providerID string, payload []byte, headers map[string]string) error {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return err
	}

	event, err := provider.ParseWebhook(ctx, payload, headers)
	if err != nil {
		return fmt.Errorf("parse %s webhook: %w", providerID, err)
	}

	processed, err := s.isEventProcessed(event.EventID)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "fiat webhook idempotency check failed: %v", err)
		return fmt.Errorf("check event idempotency: %w", err)
	}
	if processed {
		logger.LogDebugWithIDf(log, s.nodeID, "fiat webhook already processed: %s", event.EventID)
		return nil
	}

	var handled bool
	switch event.Type {
	case contracts.WebhookPaymentSucceeded:
		if err := s.handlePaymentSucceeded(ctx, providerID, event); err != nil {
			return err
		}
		handled = true
	case contracts.WebhookPaymentFailed:
		if err := s.handlePaymentFailed(ctx, event); err != nil {
			return err
		}
		handled = true
	case contracts.WebhookRefundCreated:
		if err := s.handleRefundCreated(ctx, event); err != nil {
			return err
		}
		handled = true
	case contracts.WebhookDisputeOpened:
		if err := s.handleDisputeOpened(ctx, event); err != nil {
			return err
		}
		handled = true
	case contracts.WebhookDisputeResolved:
		if err := s.handleDisputeResolved(ctx, event); err != nil {
			return err
		}
		handled = true
	case contracts.WebhookPaymentCanceled:
		if err := s.handlePaymentCanceled(ctx, event); err != nil {
			return err
		}
		handled = true
	case contracts.WebhookAccountUpdated:
		s.handleAccountUpdated(event)
		handled = true
	default:
		logger.LogDebugWithIDf(log, s.nodeID, "unhandled fiat webhook type: %s", event.Type)
	}

	if handled {
		if err := s.markEventProcessed(event.EventID, providerID); err != nil {
			return fmt.Errorf("mark event processed: %w", err)
		}
	}
	return nil
}

func (s *FiatPaymentAppService) handlePaymentSucceeded(ctx context.Context, providerID string, event *contracts.WebhookEvent) error {
	logger.LogInfoWithIDf(log, s.nodeID, "fiat payment succeeded: provider=%s payment=%s order=%s",
		providerID, event.PaymentID, event.OrderID)

	if event.OrderID == "" {
		logger.LogErrorWithIDf(log, s.nodeID, "fiat payment succeeded but no order_id in metadata")
		return fmt.Errorf("fiat payment succeeded but no order_id in metadata")
	}

	// Race condition handling: check if order exists and its state before processing.
	// Scenario 1: Webhook arrives before ORDER_OPEN P2P message → order not yet created.
	// Scenario 2: Buyer canceled the order but payment was already captured.
	if s.orderRepo != nil {
		order, err := s.orderRepo.FindByID(ctx, event.OrderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logger.LogWarningWithIDf(log, s.nodeID,
					"fiat webhook for order %s arrived before ORDER_OPEN, requesting retry",
					event.OrderID)
				return &contracts.RetryableError{
					Err:        fmt.Errorf("order %s not yet created", event.OrderID),
					RetryAfter: 30 * time.Second,
				}
			}
			return fmt.Errorf("find order %s for webhook race check: %w", event.OrderID, err)
		}

		if order.State == models.OrderState_CANCELED {
			logger.LogInfoWithIDf(log, s.nodeID,
				"order %s already CANCELED, auto-refunding fiat payment %s",
				event.OrderID, event.PaymentID)
			s.autoRefundCanceledOrder(ctx, providerID, event)
			return nil
		}
	}

	// Best-effort: enrich event with payment details from provider API.
	// Failure here must not block webhook processing.
	if event.PaymentID != "" {
		provider, provErr := s.registry.ForProvider(providerID)
		if provErr == nil {
			detail, detailErr := provider.GetPayment(ctx, event.PaymentID)
			if detailErr != nil {
				logger.LogWarningWithIDf(log, s.nodeID, "best-effort payment detail fetch failed for %s: %v", event.PaymentID, detailErr)
			} else if detail != nil {
				if detail.PaymentID != "" {
					event.PaymentID = detail.PaymentID
				}
				event.Amount = detail.Amount
				event.Currency = detail.Currency
				event.PaymentMethod = detail.PaymentMethod
			}
		}
	}

	if s.webhookHandler == nil {
		return fmt.Errorf("no webhook handler registered, cannot process fiat payment for order %s", event.OrderID)
	}
	if err := s.webhookHandler(ctx, event); err != nil {
		return err
	}

	if s.eventBus != nil {
		s.eventBus.Emit(&events.FiatPaymentReady{
			OrderID:    event.OrderID,
			ProviderID: providerID,
			SessionID:  event.PaymentID,
		})
		logger.LogInfoWithIDf(log, s.nodeID, "emitted FiatPaymentReady for order %s (provider=%s)", event.OrderID, providerID)
	}
	return nil
}

// autoRefundCanceledOrder attempts a full refund for a payment on a canceled order.
// Failure is logged but not returned — the webhook is marked as processed to avoid
// an infinite retry loop. Admin can manually refund via the provider dashboard.
func (s *FiatPaymentAppService) autoRefundCanceledOrder(ctx context.Context, providerID string, event *contracts.WebhookEvent) {
	if event.PaymentID == "" {
		logger.LogWarningWithIDf(log, s.nodeID, "cannot auto-refund order %s: no payment ID", event.OrderID)
		return
	}

	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "auto-refund: provider %s not found: %v", providerID, err)
		return
	}

	_, err = provider.RefundPayment(ctx, contracts.RefundParams{
		PaymentID: event.PaymentID,
		Reason:    "order_canceled",
		Metadata:  map[string]string{"orderID": event.OrderID},
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID,
			"auto-refund failed for canceled order %s (payment %s): %v — manual refund required",
			event.OrderID, event.PaymentID, err)
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"auto-refund succeeded for canceled order %s (payment %s)",
		event.OrderID, event.PaymentID)
}

func (s *FiatPaymentAppService) handlePaymentFailed(ctx context.Context, event *contracts.WebhookEvent) error {
	logger.LogInfoWithIDf(log, s.nodeID,
		"fiat payment failed: provider=%s payment=%s order=%s reason=%s",
		event.ProviderID, event.PaymentID, event.OrderID, event.FailureReason)

	// Keep retry semantics: do not auto-cancel order on provider failure.
	// If buyer already submitted PAYMENT_SENT and order is awaiting verification,
	// persist a terminal verification-failed marker for UI clarity and explicit retry.
	if s.orderRepo != nil {
		order, err := s.findOrderForWebhook(ctx, event)
		if err != nil {
			return err
		}
		if order.State == models.OrderState_AWAITING_PAYMENT_VERIFICATION {
			order.MarkPaymentVerificationFailed("provider_failed")
			if err := s.orderRepo.Save(ctx, order); err != nil {
				return fmt.Errorf("mark payment verification failed for order %s: %w", order.ID, err)
			}
			logger.LogInfoWithIDf(log, s.nodeID,
				"order %s marked verification failed (provider_failed)", order.ID)
		}
	}

	// Order timeout (S3-1) still handles stale cleanup for retried/abandoned orders.
	return nil
}

func (s *FiatPaymentAppService) handleRefundCreated(ctx context.Context, event *contracts.WebhookEvent) error {
	logger.LogInfoWithIDf(log, s.nodeID,
		"fiat refund created: provider=%s payment=%s refund=%s",
		event.ProviderID, event.PaymentID, event.RefundID)

	if s.orderRepo == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "orderRepo not set, cannot process refund webhook")
		return nil
	}

	order, err := s.findOrderForWebhook(ctx, event)
	if err != nil {
		return err
	}

	if order.State == models.OrderState_REFUNDED {
		logger.LogInfoWithIDf(log, s.nodeID, "order %s already REFUNDED, skipping refund webhook", order.ID)
		return nil
	}

	order.SetFSMState(models.OrderState_REFUNDED)
	if err := s.orderRepo.Save(ctx, order); err != nil {
		return fmt.Errorf("update order %s to REFUNDED: %w", order.ID, err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "order %s → REFUNDED via %s refund %s",
		order.ID, event.ProviderID, event.RefundID)
	return nil
}

func (s *FiatPaymentAppService) handleDisputeOpened(ctx context.Context, event *contracts.WebhookEvent) error {
	logger.LogInfoWithIDf(log, s.nodeID,
		"fiat dispute opened: provider=%s payment=%s dispute=%s reason=%s",
		event.ProviderID, event.PaymentID, event.DisputeID, event.DisputeReason)

	if s.orderRepo == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "orderRepo not set, cannot process dispute webhook")
		return nil
	}

	order, err := s.findOrderForWebhook(ctx, event)
	if err != nil {
		return err
	}

	metadata := map[string]string{
		"fiat_dispute_status":    "opened",
		"fiat_dispute_id":        event.DisputeID,
		"fiat_dispute_reason":    event.DisputeReason,
		"fiat_dispute_provider":  event.ProviderID,
		"fiat_dispute_opened_at": time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.orderRepo.MergeFiatMetadata(ctx, string(order.ID), metadata); err != nil {
		return fmt.Errorf("update dispute metadata for order %s: %w", order.ID, err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "order %s: fiat dispute %s opened via %s (reason: %s), order state unchanged",
		order.ID, event.DisputeID, event.ProviderID, event.DisputeReason)
	return nil
}

func (s *FiatPaymentAppService) handleDisputeResolved(ctx context.Context, event *contracts.WebhookEvent) error {
	outcome := event.DisputeOutcome
	logger.LogInfoWithIDf(log, s.nodeID,
		"fiat dispute resolved: provider=%s payment=%s dispute=%s outcome=%s",
		event.ProviderID, event.PaymentID, event.DisputeID, outcome)

	if s.orderRepo == nil {
		logger.LogWarningWithIDf(log, s.nodeID, "orderRepo not set, cannot process dispute resolved webhook")
		return nil
	}

	order, err := s.findOrderForWebhook(ctx, event)
	if err != nil {
		return err
	}

	metadata := map[string]string{
		"fiat_dispute_status":      "resolved",
		"fiat_dispute_outcome":     outcome,
		"fiat_dispute_resolved_at": time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.orderRepo.MergeFiatMetadata(ctx, string(order.ID), metadata); err != nil {
		return fmt.Errorf("update dispute resolved metadata for order %s: %w", order.ID, err)
	}

	switch outcome {
	case "lost":
		order.SetFSMState(models.OrderState_REFUNDED)
		if err := s.orderRepo.Save(ctx, order); err != nil {
			return fmt.Errorf("dispute lost but REFUNDED sync failed for order %s: %w", order.ID, err)
		}
		logger.LogInfoWithIDf(log, s.nodeID, "order %s → REFUNDED (dispute lost)", order.ID)
	case "won":
		order.SetFSMState(models.OrderState_RESOLVED)
		if err := s.orderRepo.Save(ctx, order); err != nil {
			return fmt.Errorf("dispute won but RESOLVED sync failed for order %s: %w", order.ID, err)
		}
		logger.LogInfoWithIDf(log, s.nodeID, "order %s → RESOLVED (dispute won)", order.ID)
	default:
		logger.LogInfoWithIDf(log, s.nodeID, "order %s dispute resolved with outcome=%s, no state change", order.ID, outcome)
	}

	return nil
}

func (s *FiatPaymentAppService) handlePaymentCanceled(_ context.Context, event *contracts.WebhookEvent) error {
	logger.LogInfoWithIDf(log, s.nodeID,
		"fiat payment canceled: provider=%s payment=%s order=%s",
		event.ProviderID, event.PaymentID, event.OrderID)
	return nil
}

func (s *FiatPaymentAppService) handleAccountUpdated(event *contracts.WebhookEvent) {
	if event.WebhookAccountStatus != nil {
		logger.LogInfoWithIDf(log, s.nodeID,
			"fiat account %s updated: charges=%v payouts=%v",
			event.AccountID, event.WebhookAccountStatus.ChargesEnabled, event.WebhookAccountStatus.PayoutsEnabled)
	} else {
		logger.LogInfoWithIDf(log, s.nodeID, "fiat account updated: provider=%s account=%s",
			event.ProviderID, event.AccountID)
	}
}

// findOrderForWebhook locates the order associated with a webhook event,
// trying PaymentTransactionID first, then falling back to OrderID from metadata.
func (s *FiatPaymentAppService) findOrderForWebhook(ctx context.Context, event *contracts.WebhookEvent) (*models.Order, error) {
	if event.PaymentID != "" {
		order, err := s.orderRepo.FindByPaymentTransactionID(ctx, event.PaymentID)
		if err == nil {
			return order, nil
		}
	}
	if event.OrderID != "" {
		return s.orderRepo.FindByID(ctx, event.OrderID)
	}
	return nil, fmt.Errorf("webhook event %s has no PaymentID or OrderID", event.EventID)
}

// --- helpers ---

func (s *FiatPaymentAppService) getActiveAccount(providerID string) (*models.ReceivingAccount, error) {
	chainType := FiatChainType(providerID)

	var record models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ? AND is_active = ?", chainType, true).First(&record).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			if providerID == "stripe" {
				return s.getActiveAccountLegacy()
			}
			return nil, err
		}
		return nil, err
	}
	return &record, nil
}

// TECHDEBT(TD-003): getActiveAccountLegacy looks up accounts with the legacy "Stripe" chain type
// for backward compatibility during migration.
// 清除条件: 所有生产租户 ReceivingAccount 记录迁移到 fiat:provider_name 格式
func (s *FiatPaymentAppService) getActiveAccountLegacy() (*models.ReceivingAccount, error) {
	var record models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ? AND is_active = ?", iwallet.ChainFiat, true).First(&record).Error
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *FiatPaymentAppService) providerAsOnboarder(providerID string) (contracts.FiatOnboardingProvider, bool) {
	p, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, false
	}
	onboarder, ok := p.(contracts.FiatOnboardingProvider)
	return onboarder, ok
}

func (s *FiatPaymentAppService) isEventProcessed(eventID string) (bool, error) {
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.ProcessedFiatEvent{}).Where("event_id = ?", eventID).Count(&count).Error
	})
	return count > 0, err
}

func (s *FiatPaymentAppService) markEventProcessed(eventID, providerID string) error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ProcessedFiatEvent{
			EventID:     eventID,
			ProviderID:  providerID,
			ProcessedAt: time.Now(),
		})
	})
}

// --- Seller Configuration (standalone mode) ---

// GetProviderConfig returns the (masked) fiat provider config for a standalone seller.
func (s *FiatPaymentAppService) GetProviderConfig(providerID string) (*contracts.ProviderConfigView, error) {
	var cfg models.FiatProviderConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, contracts.ErrProviderNotFound
		}
		return nil, err
	}
	cfg.MaskSecrets()
	return &contracts.ProviderConfigView{
		ProviderID:            cfg.ProviderID,
		AccountID:             cfg.AccountID,
		PublicKey:             cfg.PublicKey,
		SecretKey:             cfg.SecretKey,
		WebhookSecret:         cfg.WebhookSecret,
		IsActive:              cfg.IsActive,
		WebhookAutoConfigured: cfg.WebhookAutoConfigured,
	}, nil
}

// SaveProviderConfig stores/updates the fiat provider config, creates a ReceivingAccount,
// and registers the provider in the registry (standalone mode).
// Uses upsert semantics: updates existing config if (tenant_id, provider_id) already exists.
// When webhookSecret is empty and the provider supports FiatWebhookConfigurator,
// it attempts automated webhook creation. The webhookURL must be set by the caller
// (handler knows the public-facing URL).
func (s *FiatPaymentAppService) SaveProviderConfig(providerID string, input contracts.ProviderConfigInput) error {
	chainType := FiatChainType(providerID)

	if err := s.db.Update(func(tx database.Tx) error {
		cfg := &models.FiatProviderConfig{
			ProviderID:    providerID,
			AccountID:     input.AccountID,
			PublicKey:     input.PublicKey,
			SecretKey:     input.SecretKey,
			WebhookSecret: input.WebhookSecret,
			IsActive:      true,
		}

		// Partial-update: empty input fields keep existing values
		var existing models.FiatProviderConfig
		if tx.Read().Where("provider_id = ?", providerID).First(&existing).Error == nil {
			if cfg.SecretKey == "" {
				cfg.SecretKey = existing.SecretKey
			}
			if cfg.PublicKey == "" {
				cfg.PublicKey = existing.PublicKey
			}
			if cfg.AccountID == "" {
				cfg.AccountID = existing.AccountID
			}
			if cfg.WebhookSecret == "" {
				cfg.WebhookSecret = existing.WebhookSecret
			}
		}

		if err := database.SaveByBusinessKey(tx, cfg, "provider_id = ?", providerID); err != nil {
			return err
		}

		if cfg.AccountID != "" {
			ra := &models.ReceivingAccount{
				ChainType: chainType,
				Address:   cfg.AccountID,
				IsActive:  true,
			}
			return database.SaveByBusinessKey(tx, ra, "chain_type = ?", chainType)
		}
		return nil
	}); err != nil {
		return err
	}

	s.registerProviderFromConfig(providerID, input.SecretKey, input.PublicKey, input.WebhookSecret)
	return nil
}

// SetupWebhook programmatically creates a webhook endpoint via the provider's API,
// then updates the stored config with the auto-generated webhook secret.
func (s *FiatPaymentAppService) SetupWebhook(ctx context.Context, providerID string, webhookURL string) (*contracts.WebhookSetupResult, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, err
	}

	wc, ok := provider.(contracts.FiatWebhookConfigurator)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support automated webhook setup", providerID)
	}

	result, err := wc.SetupWebhook(ctx, webhookURL)
	if err != nil {
		return nil, err
	}

	if result.WebhookSecret != "" {
		if dbErr := s.db.Update(func(tx database.Tx) error {
			var cfg models.FiatProviderConfig
			if err := tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error; err != nil {
				return err
			}
			cfg.WebhookSecret = result.WebhookSecret
			cfg.WebhookID = result.WebhookID
			cfg.WebhookAutoConfigured = true
			return database.SaveByBusinessKey(tx, &cfg, "provider_id = ?", providerID)
		}); dbErr != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "auto-webhook: saved webhook but failed to update config: %v", dbErr)
			return result, dbErr
		}

		s.registerProviderFromConfigReload(providerID)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "auto-webhook: %s webhook created (id=%s)", providerID, result.WebhookID)
	return result, nil
}

// registerProviderFromConfigReload reloads the config from DB and re-registers the provider.
func (s *FiatPaymentAppService) registerProviderFromConfigReload(providerID string) {
	var cfg models.FiatProviderConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "reload provider config for %s: %v", providerID, err)
		return
	}
	s.registerProviderFromConfig(providerID, cfg.SecretKey, cfg.PublicKey, cfg.WebhookSecret)
}

// deleteProviderConfig removes the fiat provider config and receiving account.
// If the webhook was auto-configured, it attempts to clean up the remote webhook endpoint.
// Standalone providers are unregistered from the registry; platform-injected providers stay
// registered so SaaS onboarding can be started again after disconnect.
// Called internally by DisconnectProvider.
func (s *FiatPaymentAppService) deleteProviderConfig(ctx context.Context, providerID string) error {
	var cfg models.FiatProviderConfig
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error
	})

	if cfg.WebhookAutoConfigured {
		if provider, err := s.registry.ForProvider(providerID); err == nil {
			if wc, ok := provider.(contracts.FiatWebhookConfigurator); ok {
				webhookURL := s.buildWebhookURL(providerID)
				if cleanupErr := wc.CleanupWebhook(ctx, webhookURL); cleanupErr != nil {
					logger.LogWarningWithIDf(log, s.nodeID,
						"cleanup webhook for %s during disconnect: %v", providerID, cleanupErr)
				} else {
					logger.LogInfoWithIDf(log, s.nodeID,
						"cleaned up auto-configured webhook for %s", providerID)
				}
			}
		}
	}

	err := s.db.Update(func(tx database.Tx) error {
		if err := tx.Delete("provider_id", providerID, nil, &models.FiatProviderConfig{}); err != nil {
			return err
		}
		chainType := "fiat:" + providerID
		return tx.Delete("chain_type", chainType, nil, &models.ReceivingAccount{})
	})
	if err != nil {
		return err
	}
	if !s.isPlatformProvider(providerID) {
		s.registry.Unregister(providerID)
	}
	return nil
}

// buildWebhookURL constructs the webhook URL for a provider.
// This is a best-effort reconstruction; the actual URL should be passed by the caller.
func (s *FiatPaymentAppService) buildWebhookURL(providerID string) string {
	return "/v1/fiat/" + providerID + "/webhooks"
}

// DisconnectProvider safely disconnects a fiat provider after verifying no active orders depend on it.
// Orders in verification/shipment/dispute states block disconnect. AWAITING_PAYMENT sessions are canceled.
func (s *FiatPaymentAppService) DisconnectProvider(ctx context.Context, providerID string) error {
	if s.orderRepo == nil {
		return s.deleteProviderConfig(ctx, providerID)
	}

	blockingStates := []models.OrderState{
		models.OrderState_AWAITING_PAYMENT_VERIFICATION,
		models.OrderState_AWAITING_SHIPMENT,
		models.OrderState_PARTIALLY_SHIPPED,
		models.OrderState_DISPUTED,
		models.OrderState_DECIDED,
	}

	var blockingOrders []models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("payment_transaction_id != '' AND state IN ?", blockingStates).
			Find(&blockingOrders).Error
	}); err != nil {
		return fmt.Errorf("query active fiat orders: %w", err)
	}

	fiatBlockingCount := 0
	for _, order := range blockingOrders {
		meta, _ := order.GetFiatMetadata()
		if meta["fiat_provider"] == providerID {
			fiatBlockingCount++
		}
	}
	if fiatBlockingCount > 0 {
		return fmt.Errorf("%w: %d active orders using %s", contracts.ErrActiveOrdersExist, fiatBlockingCount, providerID)
	}

	var awaitingPaymentOrders []models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state = ? AND fiat_metadata IS NOT NULL AND length(fiat_metadata) > 2", models.OrderState_AWAITING_PAYMENT).
			Find(&awaitingPaymentOrders).Error
	}); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "query awaiting-payment fiat orders: %v", err)
	}

	provider, _ := s.registry.ForProvider(providerID)
	for _, order := range awaitingPaymentOrders {
		meta, err := order.GetFiatMetadata()
		if err != nil {
			continue
		}
		if p := meta["fiat_provider"]; p != providerID {
			continue
		}
		sessionID := meta["fiat_session_id"]
		if sessionID == "" {
			continue
		}
		if provider != nil {
			if err := provider.CancelPayment(ctx, sessionID); err != nil {
				logger.LogWarningWithIDf(log, s.nodeID,
					"cancel fiat session %s for order %s: %v", sessionID, order.ID, err)
			} else {
				logger.LogInfoWithIDf(log, s.nodeID,
					"canceled fiat session %s for order %s during provider disconnect", sessionID, order.ID)
			}
		}
	}

	return s.deleteProviderConfig(ctx, providerID)
}

// VerifyProviderConfig tests the provider config by calling the provider's health check.
func (s *FiatPaymentAppService) VerifyProviderConfig(providerID string) error {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return err
	}

	// Try getting account status as a connectivity test
	var cfg models.FiatProviderConfig
	if dbErr := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error
	}); dbErr != nil {
		return fmt.Errorf("no config found for provider %s", providerID)
	}

	if onboarder, ok := provider.(contracts.FiatOnboardingProvider); ok && cfg.AccountID != "" {
		_, err = onboarder.GetAccountStatus(context.Background(), cfg.AccountID)
		return err
	}
	return nil
}

// GetProviderStatus returns the connection status for a specific provider.
func (s *FiatPaymentAppService) GetProviderStatus(_ context.Context, providerID string) (*contracts.AccountStatus, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return &contracts.AccountStatus{Status: "not_registered"}, nil
	}

	account, err := s.getActiveAccount(providerID)
	if err != nil {
		return &contracts.AccountStatus{Status: "not_connected"}, nil
	}

	if onboarder, ok := provider.(contracts.FiatOnboardingProvider); ok {
		return onboarder.GetAccountStatus(context.Background(), account.Address)
	}
	return &contracts.AccountStatus{AccountID: account.Address, Status: "active", IsActive: true}, nil
}

// GetOnboardingURL delegates to the provider's FiatOnboardingProvider interface.
func (s *FiatPaymentAppService) GetOnboardingURL(ctx context.Context, providerID string, params contracts.OnboardingParams) (*contracts.OnboardingResult, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, contracts.ErrProviderNotFound
	}
	onboarder, ok := provider.(contracts.FiatOnboardingProvider)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support onboarding", providerID)
	}
	return onboarder.GetOnboardingURL(ctx, params)
}

// HandleOnboardingCallback delegates to the provider, then fetches the full account status.
func (s *FiatPaymentAppService) HandleOnboardingCallback(ctx context.Context, providerID string, params contracts.CallbackParams) (*contracts.AccountStatus, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, contracts.ErrProviderNotFound
	}
	onboarder, ok := provider.(contracts.FiatOnboardingProvider)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support onboarding", providerID)
	}
	account, err := onboarder.HandleOnboardingCallback(ctx, params)
	if err != nil {
		return nil, err
	}
	return onboarder.GetAccountStatus(ctx, account.AccountID)
}

// RegisterPlatformProvider registers a provider using the platform's keys in Connected/Partner mode.
// Called by the hosting layer after SaaS node creation to inject platform-level keys.
func (s *FiatPaymentAppService) RegisterPlatformProvider(providerID, secretKey, publishableKey, webhookSecret string, opts *contracts.PlatformProviderOpts) {
	s.markPlatformProvider(providerID)
	s.registerProvider(providerID, secretKey, publishableKey, webhookSecret, true, opts)
}

// registerProviderFromConfig creates and registers a provider instance in Direct mode.
// Used for standalone sellers who configure their own API keys.
func (s *FiatPaymentAppService) registerProviderFromConfig(providerID, secretKey, publishableKey, webhookSecret string) {
	s.registerProvider(providerID, secretKey, publishableKey, webhookSecret, false, nil)
}

func (s *FiatPaymentAppService) registerProvider(providerID, secretKey, publishableKey, webhookSecret string, platformMode bool, opts *contracts.PlatformProviderOpts) {
	switch providerID {
	case "stripe":
		mode := stripe.ModeDirect
		if platformMode {
			mode = stripe.ModeConnected
		}
		p := stripe.NewProvider(stripe.Config{
			SecretKey:      secretKey,
			PublishableKey: publishableKey,
			WebhookSecret:  webhookSecret,
			Mode:           mode,
		})
		s.registry.Register(p)
		logger.LogInfoWithIDf(log, s.nodeID, "registered Stripe provider (%s mode)", mode)
	case "paypal":
		mode := paypal.ModeDirect
		if platformMode {
			mode = paypal.ModePartner
		}
		cfg := paypal.Config{
			ClientID:     publishableKey,
			ClientSecret: secretKey,
			WebhookID:    webhookSecret,
			Mode:         mode,
			Sandbox:      s.testnet,
		}
		if opts != nil && opts.PayPalPartnerID != "" {
			cfg.PartnerID = opts.PayPalPartnerID
		}
		p := paypal.NewProvider(cfg)
		s.registry.Register(p)
		logger.LogInfoWithIDf(log, s.nodeID, "registered PayPal provider (%s mode, partnerID=%q)", mode, cfg.PartnerID)
	default:
		logger.LogErrorWithIDf(log, s.nodeID, "unknown fiat provider %q, cannot register", providerID)
	}
}

func (s *FiatPaymentAppService) markPlatformProvider(providerID string) {
	if providerID == "" {
		return
	}
	s.platformMu.Lock()
	s.platformIDs[providerID] = struct{}{}
	s.platformMu.Unlock()
}

func (s *FiatPaymentAppService) isPlatformProvider(providerID string) bool {
	s.platformMu.RLock()
	_, ok := s.platformIDs[providerID]
	s.platformMu.RUnlock()
	return ok
}

// LoadAndRegisterProviders scans existing FiatProviderConfig records and registers
// the corresponding providers. Called during node startup for standalone mode.
func (s *FiatPaymentAppService) LoadAndRegisterProviders() {
	var configs []models.FiatProviderConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("is_active = ?", true).Find(&configs).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "failed to load fiat provider configs: %v", err)
		return
	}
	for _, cfg := range configs {
		s.registerProviderFromConfig(cfg.ProviderID, cfg.SecretKey, cfg.PublicKey, cfg.WebhookSecret)
	}
}

// ReconcileFiatOrders checks AWAITING_PAYMENT orders with fiat metadata against
// the payment provider. If the provider reports the payment as succeeded but the
// order has not reached verified states (missed webhook), it triggers the payment flow.
// Orders in AWAITING_PAYMENT_VERIFICATION are handled by PaymentVerificationLoop.
// If the provider reports canceled/failed, it's a no-op (order timeout handles cancellation).
func (s *FiatPaymentAppService) ReconcileFiatOrders(ctx context.Context) {
	if s.webhookHandler == nil {
		return
	}

	var orders []models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state = ? AND open = ? AND fiat_metadata IS NOT NULL AND length(fiat_metadata) > 2",
				models.OrderState_AWAITING_PAYMENT, true).
			Find(&orders).Error
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "fiat reconciliation: query failed: %v", err)
		return
	}
	if len(orders) == 0 {
		return
	}

	logger.LogInfoWithIDf(log, s.nodeID, "fiat reconciliation: checking %d awaiting-payment fiat orders", len(orders))

	for i := range orders {
		order := &orders[i]
		meta, err := order.GetFiatMetadata()
		if err != nil {
			continue
		}
		providerID := meta["fiat_provider"]
		sessionID := meta["fiat_session_id"]
		if providerID == "" || sessionID == "" {
			continue
		}

		provider, err := s.registry.ForProvider(providerID)
		if err != nil {
			continue
		}

		detail, err := provider.GetPayment(ctx, sessionID)
		if err != nil {
			logger.LogDebugWithIDf(log, s.nodeID, "fiat reconciliation: GetPayment(%s) for order %s: %v",
				sessionID, order.ID, err)
			continue
		}

		if detail.Status == "succeeded" {
			paymentID := sessionID
			if detail.PaymentID != "" {
				paymentID = detail.PaymentID
			}
			logger.LogInfoWithIDf(log, s.nodeID,
				"fiat reconciliation: order %s has succeeded payment %s (missed webhook), triggering payment flow",
				order.ID, paymentID)

			coin := iwallet.CoinType("fiat:" + providerID + ":" + strings.ToUpper(detail.Currency))
			if err := s.webhookHandler(ctx, &contracts.WebhookEvent{
				EventID:       "reconcile_" + sessionID,
				Type:          contracts.WebhookPaymentSucceeded,
				ProviderID:    providerID,
				PaymentID:     paymentID,
				OrderID:       string(order.ID),
				Coin:          string(coin),
				Amount:        detail.Amount,
				Currency:      detail.Currency,
				PaymentMethod: detail.PaymentMethod,
			}); err != nil {
				logger.LogErrorWithIDf(log, s.nodeID,
					"fiat reconciliation: ProcessOrderPayment for order %s failed: %v", order.ID, err)
			}
		}
	}
}

// RunFiatReconciliationOnce executes a single pass of fiat order reconciliation.
// Called by the shared scheduler (SaaS) or standalone maintenance scripts.
func (s *FiatPaymentAppService) RunFiatReconciliationOnce() {
	s.ReconcileFiatOrders(context.Background())
}

// CleanupProcessedEvents deletes ProcessedFiatEvent records older than the given TTL.
// Should be called periodically (e.g. daily) to prevent unbounded table growth.
//
// TECHDEBT(TD-048): Uses tx.Read().Delete() because the Tx.Delete interface does not
// support range-based bulk deletes (processed_at < ?). This is safe for DELETE operations
// because tx.Read() already provides tenant-scoped WHERE clause. See db-transaction-rules.mdc.
// 清除条件: Tx interface supports arbitrary WHERE conditions for Delete
func (s *FiatPaymentAppService) CleanupProcessedEvents(ttl time.Duration) (int64, error) {
	cutoff := time.Now().Add(-ttl)
	var deleted int64
	err := s.db.Update(func(tx database.Tx) error {
		result := tx.Read().Where("processed_at < ?", cutoff).Delete(&models.ProcessedFiatEvent{})
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("cleanup processed fiat events: %w", err)
	}
	if deleted > 0 {
		logger.LogInfoWithIDf(log, s.nodeID, "cleaned up %d processed fiat events older than %v", deleted, ttl)
	}
	return deleted, nil
}

// RunFiatCleanupOnce executes a single pass of processed fiat event cleanup
// with a 7-day TTL. Called by the shared scheduler (SaaS) or standalone scripts.
func (s *FiatPaymentAppService) RunFiatCleanupOnce() {
	const defaultTTL = 7 * 24 * time.Hour
	if _, err := s.CleanupProcessedEvents(defaultTTL); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "fiat event cleanup failed: %v", err)
	}
}

// Compile-time checks.
var _ contracts.FiatService = (*FiatPaymentAppService)(nil)
var _ contracts.FiatPlatformConfigurer = (*FiatPaymentAppService)(nil)
var _ contracts.FiatPaymentOperations = (*FiatPaymentAppService)(nil)
