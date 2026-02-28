package core

import (
	"context"
	"fmt"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
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

	switch event.Type {
	case contracts.WebhookPaymentSucceeded:
		if err := s.handlePaymentSucceeded(ctx, providerID, event); err != nil {
			return err
		}
	case contracts.WebhookPaymentFailed:
		logger.LogInfoWithIDf(log, s.nodeID, "fiat payment failed: provider=%s payment=%s order=%s", providerID, event.PaymentID, event.OrderID)
	case contracts.WebhookDisputeOpened:
		logger.LogInfoWithIDf(log, s.nodeID, "fiat dispute opened: provider=%s payment=%s", providerID, event.PaymentID)
	case contracts.WebhookRefundCreated:
		logger.LogInfoWithIDf(log, s.nodeID, "fiat refund created: provider=%s payment=%s", providerID, event.PaymentID)
	default:
		logger.LogDebugWithIDf(log, s.nodeID, "unhandled fiat webhook type: %s", event.Type)
	}

	if err := s.markEventProcessed(event.EventID, providerID); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "fiat webhook mark processed failed: %v", err)
	}
	return nil
}

func (s *FiatPaymentAppService) handlePaymentSucceeded(ctx context.Context, providerID string, event *contracts.WebhookEvent) error {
	logger.LogInfoWithIDf(log, s.nodeID, "fiat payment succeeded: provider=%s payment=%s order=%s",
		providerID, event.PaymentID, event.OrderID)

	if event.OrderID == "" {
		logger.LogErrorWithIDf(log, s.nodeID, "fiat payment succeeded but no order_id in metadata")
		return nil
	}

	if s.webhookHandler != nil {
		return s.webhookHandler(ctx, event)
	}
	logger.LogErrorWithIDf(log, s.nodeID, "no webhook handler registered, payment event not processed: order=%s", event.OrderID)
	return nil
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

// getActiveAccountLegacy looks up accounts with the legacy "Stripe" chain type
// for backward compatibility during migration.
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

// SaveProviderConfig stores/updates the fiat provider config (standalone mode).
func (s *FiatPaymentAppService) SaveProviderConfig(providerID string, input contracts.ProviderConfigInput) error {
	cfg := &models.FiatProviderConfig{
		ProviderID:    providerID,
		AccountID:     input.AccountID,
		PublicKey:     input.PublicKey,
		SecretKey:     input.SecretKey,
		WebhookSecret: input.WebhookSecret,
		IsActive:      true,
	}
	return s.db.Update(func(tx database.Tx) error {
		return tx.Save(cfg)
	})
}

// DeleteProviderConfig removes the fiat provider config and deactivates the receiving account.
func (s *FiatPaymentAppService) DeleteProviderConfig(providerID string) error {
	return s.db.Update(func(tx database.Tx) error {
		if err := tx.Delete("provider_id", providerID, nil, &models.FiatProviderConfig{}); err != nil {
			return err
		}
		chainType := "fiat:" + providerID
		return tx.Delete("chain_type", chainType, nil, &models.ReceivingAccount{})
	})
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
func (s *FiatPaymentAppService) GetOnboardingURL(ctx context.Context, providerID string, params contracts.OnboardingParams) (string, error) {
	provider, err := s.registry.ForProvider(providerID)
	if err != nil {
		return "", contracts.ErrProviderNotFound
	}
	onboarder, ok := provider.(contracts.FiatOnboardingProvider)
	if !ok {
		return "", fmt.Errorf("provider %s does not support onboarding", providerID)
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

// Compile-time check.
var _ contracts.FiatService = (*FiatPaymentAppService)(nil)
