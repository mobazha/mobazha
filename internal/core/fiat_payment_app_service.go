package core

import (
	"context"
	"fmt"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/payment/fiat/paypal"
	"github.com/mobazha/mobazha3.0/internal/payment/fiat/stripe"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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
	registry       contracts.FiatProviderRegistry
	db             database.Database
	nodeID         string
	webhookHandler FiatWebhookHandler
	orderRepo      contracts.OrderRepo
}

func NewFiatPaymentAppService(
	registry contracts.FiatProviderRegistry,
	db database.Database,
	nodeID string,
) *FiatPaymentAppService {
	return &FiatPaymentAppService{
		registry: registry,
		db:       db,
		nodeID:   nodeID,
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

func (s *FiatPaymentAppService) CreatePayment(ctx context.Context, providerID string, params contracts.CreatePaymentParams) (*contracts.PaymentSession, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return nil, err
	}

	ra, err := s.getActiveAccount(providerID)
	if err != nil {
		return nil, fmt.Errorf("seller has no %s account configured: %w", providerID, err)
	}
	params.SellerAccountID = ra.Address

	session, err := provider.CreatePayment(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create %s payment: %w", providerID, err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "fiat payment created: provider=%s session=%s order=%s", providerID, session.SessionID, params.OrderID)
	return session, nil
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

	// Best-effort: enrich event with payment details from provider API.
	// Failure here must not block webhook processing.
	if event.PaymentID != "" {
		provider, provErr := s.registry.ForProvider(providerID)
		if provErr == nil {
			detail, detailErr := provider.GetPayment(ctx, event.PaymentID)
			if detailErr != nil {
				logger.LogWarningWithIDf(log, s.nodeID, "best-effort payment detail fetch failed for %s: %v", event.PaymentID, detailErr)
			} else if detail != nil {
				event.Amount = detail.Amount
				event.Currency = detail.Currency
				event.PaymentMethod = detail.PaymentMethod
			}
		}
	}

	if s.webhookHandler == nil {
		return fmt.Errorf("no webhook handler registered, cannot process fiat payment for order %s", event.OrderID)
	}
	return s.webhookHandler(ctx, event)
}

func (s *FiatPaymentAppService) handlePaymentFailed(_ context.Context, event *contracts.WebhookEvent) error {
	logger.LogInfoWithIDf(log, s.nodeID,
		"fiat payment failed: provider=%s payment=%s order=%s reason=%s",
		event.ProviderID, event.PaymentID, event.OrderID, event.FailureReason)
	// Design decision: do not auto-cancel — buyer may retry with a different payment method.
	// Order timeout (S3-1) handles cleanup.
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
		"fiat_dispute_id":       event.DisputeID,
		"fiat_dispute_reason":   event.DisputeReason,
		"fiat_dispute_opened_at": time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.orderRepo.MergeFiatMetadata(ctx, string(order.ID), metadata); err != nil {
		return fmt.Errorf("update dispute metadata for order %s: %w", order.ID, err)
	}

	logger.LogInfoWithIDf(log, s.nodeID, "order %s marked with fiat dispute %s",
		order.ID, event.DisputeID)
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

	if outcome == "lost" {
		order.SetFSMState(models.OrderState_REFUNDED)
		if err := s.orderRepo.Save(ctx, order); err != nil {
			return fmt.Errorf("dispute lost but REFUNDED sync failed for order %s: %w", order.ID, err)
		}
		logger.LogInfoWithIDf(log, s.nodeID, "order %s → REFUNDED (dispute lost)", order.ID)
	}

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
		return tx.Read().Where("chain_type = ? AND is_active = ?", iwallet.ChainStripe, true).First(&record).Error
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
		ProviderID:    cfg.ProviderID,
		AccountID:     cfg.AccountID,
		PublicKey:     cfg.PublicKey,
		SecretKey:     cfg.SecretKey,
		WebhookSecret: cfg.WebhookSecret,
		IsActive:      cfg.IsActive,
	}, nil
}

// SaveProviderConfig stores/updates the fiat provider config, creates a ReceivingAccount,
// and registers the provider in the registry (standalone mode).
func (s *FiatPaymentAppService) SaveProviderConfig(providerID string, input contracts.ProviderConfigInput) error {
	cfg := &models.FiatProviderConfig{
		ProviderID:    providerID,
		AccountID:     input.AccountID,
		PublicKey:     input.PublicKey,
		SecretKey:     input.SecretKey,
		WebhookSecret: input.WebhookSecret,
		IsActive:      true,
	}

	chainType := FiatChainType(providerID)
	ra := &models.ReceivingAccount{
		ChainType: chainType,
		Address:   input.AccountID,
		IsActive:  true,
	}

	if err := s.db.Update(func(tx database.Tx) error {
		if err := tx.Save(cfg); err != nil {
			return err
		}
		return tx.Save(ra)
	}); err != nil {
		return err
	}

	s.registerProviderFromConfig(providerID, input.SecretKey, input.PublicKey, input.WebhookSecret)
	return nil
}

// DeleteProviderConfig removes the fiat provider config, deactivates the receiving account,
// and unregisters the provider from the registry.
func (s *FiatPaymentAppService) DeleteProviderConfig(providerID string) error {
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
	s.registry.Unregister(providerID)
	return nil
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

// RegisterPlatformProvider registers a provider using the platform's keys in Connected mode.
// Called by the hosting layer after SaaS node creation to inject platform-level Stripe Connect keys.
func (s *FiatPaymentAppService) RegisterPlatformProvider(providerID, secretKey, publishableKey, webhookSecret string) {
	s.registerProvider(providerID, secretKey, publishableKey, webhookSecret, true)
}

// registerProviderFromConfig creates and registers a provider instance in Direct mode.
// Used for standalone sellers who configure their own API keys.
func (s *FiatPaymentAppService) registerProviderFromConfig(providerID, secretKey, publishableKey, webhookSecret string) {
	s.registerProvider(providerID, secretKey, publishableKey, webhookSecret, false)
}

func (s *FiatPaymentAppService) registerProvider(providerID, secretKey, publishableKey, webhookSecret string, platformMode bool) {
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
		p := paypal.NewProvider(paypal.Config{
			ClientID:     publishableKey,
			ClientSecret: secretKey,
			WebhookID:    webhookSecret,
			Mode:         mode,
		})
		s.registry.Register(p)
		logger.LogInfoWithIDf(log, s.nodeID, "registered PayPal provider (%s mode)", mode)
	default:
		logger.LogErrorWithIDf(log, s.nodeID, "unknown fiat provider %q, cannot register", providerID)
	}
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

// Compile-time checks.
var _ contracts.FiatService = (*FiatPaymentAppService)(nil)
var _ contracts.FiatPlatformConfigurer = (*FiatPaymentAppService)(nil)
