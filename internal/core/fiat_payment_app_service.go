package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/payment/fiat/paypal"
	"github.com/mobazha/mobazha/internal/payment/fiat/stripe"
	"github.com/mobazha/mobazha/pkg/contracts"
	pkgcrypto "github.com/mobazha/mobazha/pkg/crypto"
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
	credentialKeys       contracts.ProviderCredentialKeyProvider
	providerFactory      fiatProviderFactory
}

const (
	providerCredentialEncryptionKeyVersion uint64 = 1
	providerActionLeaseDuration                   = 2 * time.Minute
	providerActionBatchSize                       = 100
	providerActionDefaultListLimit                = 50
	providerActionMaxListLimit                    = 100
)

type providerCredentialMaterial struct {
	PublicKey     string `json:"publicKey"`
	SecretKey     string `json:"secretKey"`
	WebhookSecret string `json:"webhookSecret,omitempty"`
}

type fiatProviderFactory func(providerID string, credential providerCredentialMaterial, platformMode bool, opts *contracts.PlatformProviderOpts) (contracts.FiatPaymentProvider, error)

type providerCaptureIntent struct {
	SessionID string `json:"sessionID"`
}

type providerRefundIntent struct {
	Params contracts.RefundParams `json:"params"`
}

type providerCancelIntent struct {
	PaymentID string `json:"paymentID"`
}

func NewFiatPaymentAppService(
	registry contracts.FiatProviderRegistry,
	db database.Database,
	nodeID string,
	testnet bool,
) *FiatPaymentAppService {
	return &FiatPaymentAppService{
		registry:        registry,
		db:              db,
		nodeID:          nodeID,
		testnet:         testnet,
		platformIDs:     make(map[string]struct{}),
		providerFactory: defaultFiatProviderFactory(testnet),
	}
}

// SetProviderCredentialKeyProvider configures the KMS/Vault boundary used by
// direct provider credentials. It must be set before loading or saving direct
// provider configuration.
func (s *FiatPaymentAppService) SetProviderCredentialKeyProvider(keys contracts.ProviderCredentialKeyProvider) {
	s.credentialKeys = keys
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
	if err := s.persistFiatPaymentMetadata(ctx, attempt, route, session.SessionID); err != nil {
		persistErr := fmt.Errorf("persist %s payment metadata: %w", providerID, err)
		if markErr := s.markFiatAttemptReconcileRequired(attempt.AttemptID, persistErr); markErr != nil {
			return nil, errors.Join(persistErr, markErr)
		}
		return nil, persistErr
	}

	logger.LogInfoWithIDf(log, s.nodeID, "fiat payment created: provider=%s session=%s order=%s", providerID, session.SessionID, params.OrderID)
	return session, nil
}

func (s *FiatPaymentAppService) prepareFiatPaymentAttempt(providerID string, params contracts.CreatePaymentParams, account *models.ReceivingAccount) (models.PaymentAttempt, models.PaymentRouteBinding, error) {
	assetID := "fiat:" + strings.ToLower(strings.TrimSpace(providerID)) + ":" + strings.ToUpper(strings.TrimSpace(params.Currency))
	var attempt models.PaymentAttempt
	var route models.PaymentRouteBinding
	err := s.db.Update(func(tx database.Tx) error {
		binding, err := s.ensureProviderBindingTx(tx, providerID, account.Address)
		if err != nil {
			return err
		}
		decision := distribution.DecidePaymentRoute(distribution.PaymentRouteDecisionRequest{
			WorkMode: distribution.PaymentRouteAdmitNew, ContributionID: binding.DriverContributionID,
			ProviderBindingID: binding.BindingID, BindingState: binding.State,
			ContributionAvailability: distribution.PaymentRouteReady, HistoricalImplementationAvailable: true,
		})
		if !decision.Allowed {
			return fmt.Errorf("payment route decision %s: %s", decision.Code, decision.Reason)
		}
		attemptSeed := fmt.Sprintf("%s|%s|%d|%s", strings.TrimSpace(params.OrderID), assetID, params.Amount, binding.BindingID)
		attemptID := stablePaymentIdentity("pa_", attemptSeed)
		routeID := stablePaymentIdentity("prb_", attemptID)
		attempt = models.PaymentAttempt{
			AttemptID: attemptID, PaymentSessionID: "ps_" + strings.TrimSpace(params.OrderID), OrderID: strings.TrimSpace(params.OrderID),
			ProviderID: strings.ToLower(strings.TrimSpace(providerID)), Amount: params.Amount, Currency: strings.ToUpper(strings.TrimSpace(params.Currency)),
			RouteBindingID: routeID, IdempotencyKey: stablePaymentIdentity("mbz_", attemptSeed), State: models.PaymentAttemptPendingExternal,
		}
		route = models.PaymentRouteBinding{
			RouteBindingID: routeID, AttemptID: attemptID,
			ContributionID: binding.DriverContributionID, ModuleID: "mobazha.core.fiat." + strings.ToLower(providerID),
			ImplementationGeneration: fmt.Sprintf("builtin-v1.binding-%d", binding.ConfigurationGeneration), RailKind: string(distribution.PaymentRailProviderSession),
			NetworkID: string(FiatChainType(providerID)), AssetID: assetID, ProtocolVersion: "provider-v1", StateSchemaVersion: "1",
			ProviderBindingID: binding.BindingID, ExternalAccountReference: binding.ExternalAccountReference,
		}
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
		if err := tx.Read().Where("attempt_id = ?", attempt.AttemptID).First(&attempt).Error; err != nil {
			return err
		}
		return tx.Read().Where("route_binding_id = ?", attempt.RouteBindingID).First(&route).Error
	}); loadErr == nil {
		return attempt, route, nil
	}
	return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("prepare fiat payment attempt: %w", err)
}

func (s *FiatPaymentAppService) ensureProviderBindingTx(tx database.Tx, providerID, accountReference string) (models.PaymentProviderBinding, error) {
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	accountReference = strings.TrimSpace(accountReference)
	if providerID == "" || accountReference == "" {
		return models.PaymentProviderBinding{}, fmt.Errorf("provider binding requires provider and external account")
	}
	mode := "runtime"
	configurationBacked := false
	generation := uint64(1)
	fingerprint := providerConfigurationFingerprint(providerID, accountReference, mode)
	credentialReference := "runtime:" + providerID
	if s.isPlatformProvider(providerID) {
		mode = "platform"
		fingerprint = providerConfigurationFingerprint(providerID, accountReference, mode)
		credentialReference = "platform:" + providerID
	} else {
		var cfg models.FiatProviderConfig
		if err := tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error; err == nil {
			mode = "direct"
			configurationBacked = true
			generation = cfg.ConfigurationGeneration
			if generation == 0 {
				generation = 1
			}
			fingerprint = cfg.ConfigurationFingerprint
			if fingerprint == "" || strings.TrimSpace(cfg.CredentialReference) == "" {
				return models.PaymentProviderBinding{}, fmt.Errorf("provider %s configuration has no versioned credential", providerID)
			}
			credentialReference = cfg.CredentialReference
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PaymentProviderBinding{}, err
		}
	}
	if !configurationBacked {
		var current models.PaymentProviderBinding
		if err := tx.Read().Where(
			"provider_id = ? AND mode = ? AND configuration_fingerprint = ? AND external_account_reference = ? AND state = ?",
			providerID, mode, fingerprint, accountReference, models.PaymentProviderBindingActive,
		).First(&current).Error; err == nil {
			return current, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PaymentProviderBinding{}, err
		}
		var latest models.PaymentProviderBinding
		if err := tx.Read().Where("provider_id = ?", providerID).Order("configuration_generation DESC").First(&latest).Error; err == nil {
			generation = latest.ConfigurationGeneration + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PaymentProviderBinding{}, err
		}
	}
	bindingID := stablePaymentIdentity("fpb_", fmt.Sprintf("%s|%s|%s|%d|%s|%s", s.nodeID, providerID, mode, generation, accountReference, fingerprint))
	var binding models.PaymentProviderBinding
	if err := tx.Read().Where("binding_id = ?", bindingID).First(&binding).Error; err == nil {
		return binding, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PaymentProviderBinding{}, err
	}
	var active []models.PaymentProviderBinding
	if err := tx.Read().Where("provider_id = ? AND state = ?", providerID, models.PaymentProviderBindingActive).Find(&active).Error; err != nil {
		return models.PaymentProviderBinding{}, err
	}
	now := time.Now().UTC()
	for _, previous := range active {
		if _, err := tx.UpdateColumns(map[string]interface{}{
			"state": models.PaymentProviderBindingRetired, "retired_at": now,
		}, map[string]interface{}{"binding_id = ?": previous.BindingID}, &models.PaymentProviderBinding{}); err != nil {
			return models.PaymentProviderBinding{}, err
		}
	}
	binding = models.PaymentProviderBinding{
		BindingID: bindingID, ProviderID: providerID, DriverContributionID: "core.fiat." + providerID,
		ExternalAccountReference: accountReference, CredentialReference: credentialReference,
		ConfigurationGeneration: generation, ConfigurationFingerprint: fingerprint,
		Mode: mode, State: models.PaymentProviderBindingActive,
	}
	if err := tx.Create(&binding); err != nil {
		return models.PaymentProviderBinding{}, err
	}
	return binding, nil
}

func providerConfigurationFingerprint(values ...string) string {
	return stablePaymentIdentity("", strings.Join(values, "\x00"))
}

func (s *FiatPaymentAppService) directProviderConfigurationFingerprint(providerID, accountID string, material providerCredentialMaterial) (string, error) {
	if s.credentialKeys == nil {
		return "", fmt.Errorf("provider credential key provider is not configured")
	}
	key, err := s.credentialKeys.ProviderCredentialMasterKey(providerCredentialEncryptionKeyVersion)
	if err != nil {
		return "", fmt.Errorf("resolve provider credential fingerprint key v%d: %w", providerCredentialEncryptionKeyVersion, err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(strings.Join([]string{
		strings.ToLower(strings.TrimSpace(providerID)), strings.TrimSpace(accountID),
		material.PublicKey, material.SecretKey, material.WebhookSecret,
	}, "\x00")))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func providerCredentialReference(providerID string, generation uint64) string {
	return fmt.Sprintf("tenant-config:%s:%d", strings.ToLower(strings.TrimSpace(providerID)), generation)
}

func maskProviderSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 6 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-3:]
}

func (s *FiatPaymentAppService) storeProviderCredentialTx(
	tx database.Tx,
	reference, providerID, accountReference string,
	generation uint64,
	fingerprint string,
	material providerCredentialMaterial,
) error {
	if s.credentialKeys == nil {
		return fmt.Errorf("provider credential key provider is not configured")
	}
	var existing models.PaymentProviderCredential
	err := tx.Read().Where("credential_reference = ?", reference).First(&existing).Error
	if err == nil {
		if existing.ProviderID != providerID || existing.ExternalAccountReference != accountReference || existing.ConfigurationGeneration != generation || existing.ConfigurationFingerprint != fingerprint {
			return fmt.Errorf("credential reference %s is immutable and already identifies different material", reference)
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	payload, err := json.Marshal(material)
	if err != nil {
		return fmt.Errorf("marshal provider credential: %w", err)
	}
	key, err := s.credentialKeys.ProviderCredentialMasterKey(providerCredentialEncryptionKeyVersion)
	if err != nil {
		return fmt.Errorf("resolve provider credential encryption key v%d: %w", providerCredentialEncryptionKeyVersion, err)
	}
	ciphertext, err := pkgcrypto.EncryptAESGCM(payload, key)
	if err != nil {
		return fmt.Errorf("encrypt provider credential: %w", err)
	}
	return tx.Create(&models.PaymentProviderCredential{
		CredentialReference: reference, ProviderID: providerID, ExternalAccountReference: accountReference,
		ConfigurationGeneration: generation, ConfigurationFingerprint: fingerprint,
		EncryptionKeyVersion: providerCredentialEncryptionKeyVersion, Ciphertext: ciphertext,
	})
}

func (s *FiatPaymentAppService) loadProviderCredentialTx(
	tx database.Tx,
	reference, providerID, accountReference string,
	generation uint64,
	fingerprint string,
) (providerCredentialMaterial, error) {
	if s.credentialKeys == nil {
		return providerCredentialMaterial{}, fmt.Errorf("provider credential key provider is not configured")
	}
	if strings.TrimSpace(reference) == "" {
		return providerCredentialMaterial{}, fmt.Errorf("provider credential reference is empty")
	}
	var stored models.PaymentProviderCredential
	if err := tx.Read().Where("credential_reference = ?", reference).First(&stored).Error; err != nil {
		return providerCredentialMaterial{}, fmt.Errorf("credential reference %s is unavailable: %w", reference, err)
	}
	if stored.ProviderID != providerID || stored.ExternalAccountReference != accountReference || stored.ConfigurationGeneration != generation || stored.ConfigurationFingerprint != fingerprint {
		return providerCredentialMaterial{}, fmt.Errorf("credential reference %s does not match provider binding", reference)
	}
	key, err := s.credentialKeys.ProviderCredentialMasterKey(stored.EncryptionKeyVersion)
	if err != nil {
		return providerCredentialMaterial{}, fmt.Errorf("resolve provider credential encryption key v%d: %w", stored.EncryptionKeyVersion, err)
	}
	payload, err := pkgcrypto.DecryptAESGCM(stored.Ciphertext, key)
	if err != nil {
		return providerCredentialMaterial{}, fmt.Errorf("decrypt credential reference %s: %w", reference, err)
	}
	var material providerCredentialMaterial
	if err := json.Unmarshal(payload, &material); err != nil {
		return providerCredentialMaterial{}, fmt.Errorf("decode credential reference %s: %w", reference, err)
	}
	actualFingerprint, err := s.directProviderConfigurationFingerprint(providerID, accountReference, material)
	if err != nil {
		return providerCredentialMaterial{}, err
	}
	if !hmac.Equal([]byte(actualFingerprint), []byte(fingerprint)) {
		return providerCredentialMaterial{}, fmt.Errorf("credential reference %s failed integrity verification", reference)
	}
	return material, nil
}

func (s *FiatPaymentAppService) persistFiatPaymentMetadata(ctx context.Context, attempt models.PaymentAttempt, route models.PaymentRouteBinding, sessionID string) error {
	if s.orderRepo == nil {
		return nil
	}
	meta := map[string]string{
		"fiat_provider":         attempt.ProviderID,
		"fiat_session_id":       sessionID,
		"payment_attempt_id":    attempt.AttemptID,
		"payment_route_binding": route.RouteBindingID,
		"fiat_currency":         attempt.Currency,
	}
	if specJSON, err := payment.FiatMetadataSettlementSpecJSON(); err == nil {
		meta["settlement_spec"] = specJSON
	}
	return s.orderRepo.MergeFiatMetadata(ctx, attempt.OrderID, meta)
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
	action, err := s.prepareProviderAction(providerID, models.PaymentProviderActionCapture, sessionID, "", "", providerCaptureIntent{SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	result, err := s.executeProviderAction(ctx, action)
	if err != nil {
		return nil, err
	}
	return result.Capture, nil
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
	if strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, fmt.Errorf("fiat refund requires an idempotency key")
	}
	orderID := ""
	if params.Metadata != nil {
		orderID = params.Metadata["orderID"]
	}
	action, err := s.prepareProviderAction(providerID, models.PaymentProviderActionRefund, params.PaymentID, orderID, params.IdempotencyKey, providerRefundIntent{Params: params})
	if err != nil {
		return nil, err
	}
	result, err := s.executeProviderAction(ctx, action)
	if err != nil {
		return nil, err
	}
	return result.Refund, nil
}

// CancelPayment cancels a fiat payment session via the provider registry.
// Implements contracts.FiatPaymentOperations.
func (s *FiatPaymentAppService) CancelPayment(ctx context.Context, providerID string, paymentID string) error {
	action, err := s.prepareProviderAction(providerID, models.PaymentProviderActionCancel, paymentID, "", "", providerCancelIntent{PaymentID: paymentID})
	if err != nil {
		return err
	}
	_, err = s.executeProviderAction(ctx, action)
	return err
}

type providerActionResult struct {
	Capture         *contracts.PaymentResult `json:"capture,omitempty"`
	Refund          *contracts.RefundResult  `json:"refund,omitempty"`
	Canceled        bool                     `json:"canceled,omitempty"`
	AlreadyRefunded bool                     `json:"alreadyRefunded,omitempty"`
}

func (s *FiatPaymentAppService) prepareProviderAction(
	providerID, actionKind, externalReference, orderID, callerIdempotencyKey string,
	intent interface{},
) (models.PaymentProviderAction, error) {
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	externalReference = strings.TrimSpace(externalReference)
	if providerID == "" || externalReference == "" {
		return models.PaymentProviderAction{}, fmt.Errorf("provider action requires provider and external reference")
	}
	intentPayload, err := json.Marshal(intent)
	if err != nil {
		return models.PaymentProviderAction{}, fmt.Errorf("marshal provider action intent: %w", err)
	}
	intentFingerprint := providerConfigurationFingerprint(string(intentPayload))
	var action models.PaymentProviderAction
	err = s.db.Update(func(tx database.Tx) error {
		attempt, route, err := s.resolveProviderActionRouteTx(tx, providerID, externalReference, orderID)
		if err != nil {
			return err
		}
		identityKey := strings.TrimSpace(callerIdempotencyKey)
		if identityKey == "" {
			identityKey = externalReference
		}
		seed := strings.Join([]string{actionKind, providerID, attempt.AttemptID, identityKey}, "\x00")
		actionID := stablePaymentIdentity("fpa_", seed)
		// Keep the provider-facing key within PayPal's 38-character limit.
		idempotencyKey := stablePaymentIdentity("mbza_", seed)

		var existing models.PaymentProviderAction
		if err := tx.Read().Where("action_id = ?", actionID).First(&existing).Error; err == nil {
			if existing.IntentFingerprint != intentFingerprint || existing.ActionKind != actionKind || existing.ExternalReference != externalReference {
				return contracts.ErrActionIntentConflict
			}
			action = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		action = models.PaymentProviderAction{
			ActionID: actionID, ActionKind: actionKind, ProviderID: providerID,
			AttemptID: attempt.AttemptID, RouteBindingID: route.RouteBindingID,
			ProviderBindingID: route.ProviderBindingID, ExternalReference: externalReference,
			IdempotencyKey: idempotencyKey, IntentFingerprint: intentFingerprint,
			IntentPayload: intentPayload, State: models.PaymentProviderActionPendingExternal,
		}
		return tx.Create(&action)
	})
	if err != nil {
		if action.ActionID != "" && isProviderActionUniqueViolation(err) {
			var existing models.PaymentProviderAction
			if loadErr := s.db.View(func(tx database.Tx) error {
				return tx.Read().Where("action_id = ?", action.ActionID).First(&existing).Error
			}); loadErr != nil {
				return models.PaymentProviderAction{}, errors.Join(err, loadErr)
			}
			if existing.IntentFingerprint != intentFingerprint || existing.ActionKind != actionKind || existing.ExternalReference != externalReference {
				return models.PaymentProviderAction{}, contracts.ErrActionIntentConflict
			}
			return existing, nil
		}
		return models.PaymentProviderAction{}, err
	}
	return action, nil
}

func isProviderActionUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key")
}

func (s *FiatPaymentAppService) resolveProviderActionRouteTx(
	tx database.Tx, providerID, externalReference, orderID string,
) (models.PaymentAttempt, models.PaymentRouteBinding, error) {
	var attempt models.PaymentAttempt
	query := tx.Read().Where("provider_id = ? AND external_reference = ?", providerID, externalReference)
	err := query.First(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && strings.TrimSpace(orderID) == "" {
		var order models.Order
		if orderErr := tx.Read().Where("payment_transaction_id = ?", externalReference).First(&order).Error; orderErr == nil {
			orderID = order.ID.String()
		} else if !errors.Is(orderErr, gorm.ErrRecordNotFound) {
			return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("resolve provider action order: %w", orderErr)
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) && strings.TrimSpace(orderID) != "" {
		err = tx.Read().Where("provider_id = ? AND order_id = ?", providerID, strings.TrimSpace(orderID)).Order("created_at DESC").First(&attempt).Error
	}
	if err != nil {
		return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("resolve provider action payment attempt: %w", err)
	}
	var route models.PaymentRouteBinding
	if err := tx.Read().Where("route_binding_id = ?", attempt.RouteBindingID).First(&route).Error; err != nil {
		return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("resolve provider action route: %w", err)
	}
	return attempt, route, nil
}

func (s *FiatPaymentAppService) executeProviderAction(ctx context.Context, action models.PaymentProviderAction) (providerActionResult, error) {
	if action.State == models.PaymentProviderActionCompleted {
		return decodeProviderActionResult(action)
	}
	claimed, current, err := s.claimProviderAction(action, time.Now().UTC())
	if err != nil {
		return providerActionResult{}, err
	}
	if !claimed {
		if current.State == models.PaymentProviderActionCompleted {
			return decodeProviderActionResult(current)
		}
		return providerActionResult{}, contracts.ErrActionInProgress
	}
	action = current
	logger.LogDebugWithIDf(log, s.nodeID,
		"provider action claimed: action=%s kind=%s provider=%s attempt=%d lease_expires=%s",
		action.ActionID, action.ActionKind, action.ProviderID, action.Attempts+1, action.LeaseExpiresAt.UTC().Format(time.RFC3339))
	var route models.PaymentRouteBinding
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("route_binding_id = ?", action.RouteBindingID).First(&route).Error
	}); err != nil {
		return providerActionResult{}, s.markProviderActionReconcileRequired(action, fmt.Errorf("load provider action route: %w", err))
	}
	binding, provider, err := s.resolveHistoricalProviderBinding(route)
	if err != nil {
		return providerActionResult{}, s.markProviderActionReconcileRequired(action, fmt.Errorf("resolve provider action binding: %w", err))
	}

	var result providerActionResult
	switch action.ActionKind {
	case models.PaymentProviderActionCapture:
		var intent providerCaptureIntent
		if err := json.Unmarshal(action.IntentPayload, &intent); err != nil {
			return providerActionResult{}, s.markProviderActionReconcileRequired(action, fmt.Errorf("decode capture intent: %w", err))
		}
		result.Capture, err = provider.CapturePayment(ctx, contracts.CapturePaymentParams{
			SessionID: intent.SessionID, IdempotencyKey: action.IdempotencyKey,
			SellerAccountID: binding.ExternalAccountReference,
		})
		if err == nil && result.Capture == nil {
			err = fmt.Errorf("provider returned an empty capture result")
		}
	case models.PaymentProviderActionRefund:
		var intent providerRefundIntent
		if err := json.Unmarshal(action.IntentPayload, &intent); err != nil {
			return providerActionResult{}, s.markProviderActionReconcileRequired(action, fmt.Errorf("decode refund intent: %w", err))
		}
		intent.Params.IdempotencyKey = action.IdempotencyKey
		intent.Params.Metadata = cloneStringMap(intent.Params.Metadata)
		intent.Params.Metadata["connectedAccountID"] = binding.ExternalAccountReference
		result.Refund, err = provider.RefundPayment(ctx, intent.Params)
		if errors.Is(err, contracts.ErrAlreadyRefunded) {
			result.AlreadyRefunded = true
			err = nil
		} else if err == nil && result.Refund == nil {
			err = fmt.Errorf("provider returned an empty refund result")
		}
	case models.PaymentProviderActionCancel:
		var intent providerCancelIntent
		if err := json.Unmarshal(action.IntentPayload, &intent); err != nil {
			return providerActionResult{}, s.markProviderActionReconcileRequired(action, fmt.Errorf("decode cancel intent: %w", err))
		}
		err = provider.CancelPayment(ctx, contracts.CancelPaymentParams{
			PaymentID: intent.PaymentID, IdempotencyKey: action.IdempotencyKey,
			SellerAccountID: binding.ExternalAccountReference,
		})
		result.Canceled = err == nil
	default:
		err = fmt.Errorf("unknown provider action kind %q", action.ActionKind)
	}
	if err != nil {
		return providerActionResult{}, s.markProviderActionReconcileRequired(action, fmt.Errorf("%s %s payment: %w", action.ActionKind, action.ProviderID, err))
	}
	if err := s.completeProviderAction(action, result); err != nil {
		outcome := "persist_error"
		if errors.Is(err, contracts.ErrActionLeaseLost) {
			outcome = "lease_lost"
		}
		payment.RecordFiatProviderActionOutcome(action.ProviderID, action.ActionKind, outcome)
		return providerActionResult{}, err
	}
	payment.RecordFiatProviderActionOutcome(action.ProviderID, action.ActionKind, "completed")
	payment.ObserveFiatProviderActionAttempts(action.ProviderID, action.ActionKind, "completed", action.Attempts+1)
	logger.LogInfoWithIDf(log, s.nodeID, "provider action completed: action=%s kind=%s provider=%s attempts=%d",
		action.ActionID, action.ActionKind, action.ProviderID, action.Attempts+1)
	if result.AlreadyRefunded {
		return providerActionResult{}, contracts.ErrAlreadyRefunded
	}
	return result, nil
}

func (s *FiatPaymentAppService) claimProviderAction(action models.PaymentProviderAction, now time.Time) (bool, models.PaymentProviderAction, error) {
	if action.ActionID == "" {
		return false, action, fmt.Errorf("claim provider action: action ID is empty")
	}
	if action.State == models.PaymentProviderActionCompleted {
		return false, action, nil
	}
	leaseOwner := strings.TrimSpace(s.nodeID) + ":" + uuid.NewString()
	leaseExpiresAt := now.Add(providerActionLeaseDuration)
	claimed := false
	err := s.db.Update(func(tx database.Tx) error {
		updated, err := tx.UpdateColumns(
			map[string]interface{}{
				"lease_owner": leaseOwner, "lease_expires_at": leaseExpiresAt,
				"last_attempt_at": now, "updated_at": now,
			},
			map[string]interface{}{
				"action_id = ?": action.ActionID,
				"state = ?":     action.State,
				"attempts = ?":  action.Attempts,
				"(next_attempt_at IS NULL OR next_attempt_at <= ?)":   now,
				"(lease_expires_at IS NULL OR lease_expires_at <= ?)": now,
			},
			&models.PaymentProviderAction{},
		)
		if err != nil {
			return err
		}
		claimed = updated == 1
		return nil
	})
	if err != nil {
		payment.RecordFiatProviderActionClaim(action.ProviderID, action.ActionKind, "storage_error")
		return false, models.PaymentProviderAction{}, fmt.Errorf("claim provider action %s: %w", action.ActionID, err)
	}
	if claimed {
		payment.RecordFiatProviderActionClaim(action.ProviderID, action.ActionKind, "claimed")
		action.LeaseOwner = leaseOwner
		action.LeaseExpiresAt = &leaseExpiresAt
		action.LastAttemptAt = &now
		action.UpdatedAt = now
		return true, action, nil
	}
	var current models.PaymentProviderAction
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", action.ActionID).First(&current).Error
	}); err != nil {
		payment.RecordFiatProviderActionClaim(action.ProviderID, action.ActionKind, "reload_error")
		return false, models.PaymentProviderAction{}, fmt.Errorf("reload provider action %s after claim conflict: %w", action.ActionID, err)
	}
	claimResult := "contended"
	if current.State == models.PaymentProviderActionCompleted {
		claimResult = "completed"
	} else if current.NextAttemptAt != nil && current.NextAttemptAt.After(now) {
		claimResult = "scheduled"
	}
	payment.RecordFiatProviderActionClaim(action.ProviderID, action.ActionKind, claimResult)
	return false, current, nil
}

func decodeProviderActionResult(action models.PaymentProviderAction) (providerActionResult, error) {
	var result providerActionResult
	if err := json.Unmarshal(action.ResultPayload, &result); err != nil {
		return providerActionResult{}, fmt.Errorf("decode completed provider action %s: %w", action.ActionID, err)
	}
	if result.AlreadyRefunded {
		return providerActionResult{}, contracts.ErrAlreadyRefunded
	}
	return result, nil
}

func (s *FiatPaymentAppService) markProviderActionReconcileRequired(action models.PaymentProviderAction, actionErr error) error {
	delay := time.Minute << min(action.Attempts, 6)
	if delay > time.Hour {
		delay = time.Hour
	}
	nextAttemptAt := time.Now().UTC().Add(delay)
	updateErr := s.db.Update(func(tx database.Tx) error {
		updated, err := tx.UpdateColumns(map[string]interface{}{
			"state": models.PaymentProviderActionReconcileRequired, "attempts": action.Attempts + 1,
			"last_error": payment.SanitizeProviderError(actionErr), "next_attempt_at": nextAttemptAt,
			"lease_owner": "", "lease_expires_at": nil, "updated_at": time.Now().UTC(),
		}, map[string]interface{}{
			"action_id = ?": action.ActionID, "lease_owner = ?": action.LeaseOwner,
		}, &models.PaymentProviderAction{})
		if err != nil {
			return err
		}
		if updated != 1 {
			return contracts.ErrActionLeaseLost
		}
		return nil
	})
	if updateErr != nil {
		outcome := "persist_error"
		if errors.Is(updateErr, contracts.ErrActionLeaseLost) {
			outcome = "lease_lost"
		}
		payment.RecordFiatProviderActionOutcome(action.ProviderID, action.ActionKind, outcome)
		return errors.Join(actionErr, updateErr)
	}
	payment.RecordFiatProviderActionOutcome(action.ProviderID, action.ActionKind, "reconcile_required")
	payment.ObserveFiatProviderActionAttempts(action.ProviderID, action.ActionKind, "reconcile_required", action.Attempts+1)
	logger.LogWarningWithIDf(log, s.nodeID,
		"provider action scheduled for reconciliation: action=%s kind=%s provider=%s attempts=%d next_attempt=%s error=%v",
		action.ActionID, action.ActionKind, action.ProviderID, action.Attempts+1, nextAttemptAt.Format(time.RFC3339), payment.SanitizeProviderError(actionErr))
	return actionErr
}

func (s *FiatPaymentAppService) completeProviderAction(action models.PaymentProviderAction, result providerActionResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx database.Tx) error {
		now := time.Now().UTC()
		updated, err := tx.UpdateColumns(map[string]interface{}{
			"state": models.PaymentProviderActionCompleted, "result_payload": payload,
			"attempts": action.Attempts + 1, "last_error": "", "next_attempt_at": nil,
			"lease_owner": "", "lease_expires_at": nil, "completed_at": now, "updated_at": now,
		}, map[string]interface{}{
			"action_id = ?": action.ActionID, "lease_owner = ?": action.LeaseOwner,
		}, &models.PaymentProviderAction{})
		if err != nil {
			return err
		}
		if updated != 1 {
			return contracts.ErrActionLeaseLost
		}
		return nil
	})
}

// ListProviderActions returns a tenant-scoped operational projection without
// exposing provider credentials, raw intent/result payloads, or lease owners.
func (s *FiatPaymentAppService) ListProviderActions(ctx context.Context, query contracts.ProviderActionQuery) ([]contracts.ProviderActionView, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = providerActionDefaultListLimit
	}
	if limit > providerActionMaxListLimit {
		limit = providerActionMaxListLimit
	}
	var actions []models.PaymentProviderAction
	err := s.db.View(func(tx database.Tx) error {
		db := tx.Read().WithContext(ctx)
		if providerID := strings.TrimSpace(query.ProviderID); providerID != "" {
			db = db.Where("provider_id = ?", strings.ToLower(providerID))
		}
		if actionKind := strings.TrimSpace(query.ActionKind); actionKind != "" {
			db = db.Where("action_kind = ?", strings.ToLower(actionKind))
		}
		if state := strings.TrimSpace(query.State); state != "" {
			db = db.Where("state = ?", strings.ToLower(state))
		}
		return db.Order("created_at DESC").Limit(limit).Find(&actions).Error
	})
	if err != nil {
		return nil, fmt.Errorf("list provider actions: %w", err)
	}
	now := time.Now().UTC()
	result := make([]contracts.ProviderActionView, 0, len(actions))
	for _, action := range actions {
		result = append(result, providerActionView(action, now))
	}
	return result, nil
}

// RetryProviderAction performs one explicit, tenant-scoped retry. It may
// override persisted backoff, but never a live lease; the normal CAS claim and
// provider idempotency key remain authoritative.
func (s *FiatPaymentAppService) RetryProviderAction(ctx context.Context, request contracts.ProviderActionRetryRequest) (*contracts.ProviderActionView, error) {
	actionID := strings.TrimSpace(request.ActionID)
	if actionID == "" {
		return nil, contracts.ErrActionNotFound
	}
	action, err := s.loadProviderAction(ctx, actionID)
	if err != nil {
		return nil, err
	}
	if err := s.auditProviderActionManualRetry(action, request.RequestedBy); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if action.State == models.PaymentProviderActionCompleted {
		view := providerActionView(action, now)
		payment.RecordFiatProviderActionManualRetry(action.ProviderID, action.ActionKind, "already_completed")
		return &view, nil
	}
	if action.LeaseExpiresAt != nil && action.LeaseExpiresAt.After(now) {
		payment.RecordFiatProviderActionManualRetry(action.ProviderID, action.ActionKind, "in_progress")
		return nil, contracts.ErrActionInProgress
	}
	if action.State != models.PaymentProviderActionPendingExternal && action.State != models.PaymentProviderActionReconcileRequired {
		payment.RecordFiatProviderActionManualRetry(action.ProviderID, action.ActionKind, "not_retryable")
		return nil, contracts.ErrActionNotRetryable
	}
	if action.NextAttemptAt != nil && action.NextAttemptAt.After(now) {
		if err := s.clearProviderActionBackoff(action, now); err != nil {
			payment.RecordFiatProviderActionManualRetry(action.ProviderID, action.ActionKind, "claim_conflict")
			return nil, err
		}
		action.NextAttemptAt = nil
		action.UpdatedAt = now
	}
	if _, err := s.executeProviderAction(ctx, action); err != nil && !errors.Is(err, contracts.ErrAlreadyRefunded) {
		result := "failed"
		if errors.Is(err, contracts.ErrActionInProgress) || errors.Is(err, contracts.ErrActionLeaseLost) {
			result = "claim_conflict"
		}
		payment.RecordFiatProviderActionManualRetry(action.ProviderID, action.ActionKind, result)
		return nil, err
	}
	completed, err := s.loadProviderAction(ctx, actionID)
	if err != nil {
		payment.RecordFiatProviderActionManualRetry(action.ProviderID, action.ActionKind, "reload_error")
		return nil, err
	}
	payment.RecordFiatProviderActionManualRetry(action.ProviderID, action.ActionKind, "completed")
	view := providerActionView(completed, time.Now().UTC())
	return &view, nil
}

func (s *FiatPaymentAppService) auditProviderActionManualRetry(action models.PaymentProviderAction, requestedBy string) error {
	requestedBy = strings.TrimSpace(requestedBy)
	if requestedBy == "" {
		requestedBy = "unknown"
	}
	audit := &models.PaymentProviderActionAudit{
		AuditID: uuid.NewString(), ActionID: action.ActionID,
		Event: models.PaymentProviderActionAuditManualRetryRequested, Actor: requestedBy,
		ActionKind: action.ActionKind, ProviderID: action.ProviderID, State: action.State,
		Attempts: action.Attempts, CreatedAt: time.Now().UTC(),
	}
	if err := s.db.Update(func(tx database.Tx) error { return tx.Create(audit) }); err != nil {
		return fmt.Errorf("audit manual retry for provider action %s: %w", action.ActionID, err)
	}
	return nil
}

func (s *FiatPaymentAppService) loadProviderAction(ctx context.Context, actionID string) (models.PaymentProviderAction, error) {
	var action models.PaymentProviderAction
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).Where("action_id = ?", actionID).First(&action).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PaymentProviderAction{}, contracts.ErrActionNotFound
	}
	if err != nil {
		return models.PaymentProviderAction{}, fmt.Errorf("load provider action %s: %w", actionID, err)
	}
	return action, nil
}

func (s *FiatPaymentAppService) clearProviderActionBackoff(action models.PaymentProviderAction, now time.Time) error {
	return s.db.Update(func(tx database.Tx) error {
		updated, err := tx.UpdateColumns(map[string]interface{}{
			"next_attempt_at": nil, "updated_at": now,
		}, map[string]interface{}{
			"action_id = ?": action.ActionID, "state = ?": action.State, "attempts = ?": action.Attempts,
			"(lease_expires_at IS NULL OR lease_expires_at <= ?)": now,
		}, &models.PaymentProviderAction{})
		if err != nil {
			return err
		}
		if updated != 1 {
			return contracts.ErrActionInProgress
		}
		return nil
	})
}

func providerActionView(action models.PaymentProviderAction, now time.Time) contracts.ProviderActionView {
	leased := action.LeaseExpiresAt != nil && action.LeaseExpiresAt.After(now)
	retryable := action.State == models.PaymentProviderActionPendingExternal || action.State == models.PaymentProviderActionReconcileRequired
	return contracts.ProviderActionView{
		ActionID: action.ActionID, ActionKind: action.ActionKind, ProviderID: action.ProviderID,
		ExternalReference: action.ExternalReference, State: action.State, Attempts: action.Attempts,
		ErrorSummary: payment.SanitizeProviderErrorMessage(action.LastError), Retryable: retryable && !leased, Leased: leased,
		NextAttemptAt: action.NextAttemptAt, LeaseExpiresAt: action.LeaseExpiresAt, LastAttemptAt: action.LastAttemptAt,
		CompletedAt: action.CompletedAt, CreatedAt: action.CreatedAt, UpdatedAt: action.UpdatedAt,
	}
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

	_, err := s.RefundPayment(ctx, providerID, contracts.RefundParams{
		PaymentID: event.PaymentID, IdempotencyKey: "auto-refund-canceled:" + event.OrderID,
		Reason: "order_canceled", Metadata: map[string]string{"orderID": event.OrderID},
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
	var credential providerCredentialMaterial
	err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error; err != nil {
			return err
		}
		var err error
		credential, err = s.loadProviderCredentialTx(tx, cfg.CredentialReference, cfg.ProviderID, cfg.AccountID, cfg.ConfigurationGeneration, cfg.ConfigurationFingerprint)
		return err
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, contracts.ErrProviderNotFound
		}
		return nil, err
	}
	return &contracts.ProviderConfigView{
		ProviderID:            cfg.ProviderID,
		AccountID:             cfg.AccountID,
		PublicKey:             credential.PublicKey,
		SecretKey:             maskProviderSecret(credential.SecretKey),
		WebhookSecret:         maskProviderSecret(credential.WebhookSecret),
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
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	chainType := FiatChainType(providerID)
	var resolvedCredential providerCredentialMaterial
	if err := s.db.Update(func(tx database.Tx) error {
		cfg := &models.FiatProviderConfig{
			ProviderID: providerID,
			AccountID:  input.AccountID,
			PublicKey:  input.PublicKey,
			IsActive:   true,
		}
		credential := providerCredentialMaterial{
			PublicKey: input.PublicKey, SecretKey: input.SecretKey, WebhookSecret: input.WebhookSecret,
		}

		// Partial-update: empty input fields keep existing values
		var existing models.FiatProviderConfig
		existingErr := tx.Read().Where("provider_id = ?", providerID).First(&existing).Error
		if existingErr == nil {
			existingCredential, err := s.loadProviderCredentialTx(tx, existing.CredentialReference, existing.ProviderID, existing.AccountID, existing.ConfigurationGeneration, existing.ConfigurationFingerprint)
			if err != nil {
				return fmt.Errorf("load current %s credential: %w", providerID, err)
			}
			if credential.SecretKey == "" {
				credential.SecretKey = existingCredential.SecretKey
			}
			if credential.PublicKey == "" {
				credential.PublicKey = existingCredential.PublicKey
			}
			if cfg.AccountID == "" {
				cfg.AccountID = existing.AccountID
			}
			if credential.WebhookSecret == "" {
				credential.WebhookSecret = existingCredential.WebhookSecret
			}
			cfg.PublicKey = credential.PublicKey
			cfg.WebhookID = existing.WebhookID
			cfg.WebhookAutoConfigured = existing.WebhookAutoConfigured
		} else if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}
		fingerprint, err := s.directProviderConfigurationFingerprint(providerID, cfg.AccountID, credential)
		if err != nil {
			return err
		}
		generation := existing.ConfigurationGeneration
		if generation == 0 {
			generation = 1
		}
		if errors.Is(existingErr, gorm.ErrRecordNotFound) {
			var latest models.PaymentProviderBinding
			if err := tx.Read().Where("provider_id = ?", providerID).Order("configuration_generation DESC").First(&latest).Error; err == nil {
				generation = latest.ConfigurationGeneration + 1
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if existing.ProviderID != "" && existing.ConfigurationFingerprint != "" && existing.ConfigurationFingerprint != fingerprint {
			generation++
		}
		cfg.ConfigurationGeneration = generation
		cfg.ConfigurationFingerprint = fingerprint
		cfg.CredentialReference = providerCredentialReference(providerID, generation)
		if err := s.storeProviderCredentialTx(tx, cfg.CredentialReference, providerID, cfg.AccountID, generation, fingerprint, credential); err != nil {
			return err
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
			if err := database.SaveByBusinessKey(tx, ra, "chain_type = ?", chainType); err != nil {
				return err
			}
			if _, err := s.ensureProviderBindingTx(tx, providerID, cfg.AccountID); err != nil {
				return err
			}
		}
		resolvedCredential = credential
		return nil
	}); err != nil {
		return err
	}

	s.registerProviderFromConfig(providerID, resolvedCredential.SecretKey, resolvedCredential.PublicKey, resolvedCredential.WebhookSecret)
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
			credential, err := s.loadProviderCredentialTx(tx, cfg.CredentialReference, cfg.ProviderID, cfg.AccountID, cfg.ConfigurationGeneration, cfg.ConfigurationFingerprint)
			if err != nil {
				return err
			}
			credential.WebhookSecret = result.WebhookSecret
			cfg.WebhookID = result.WebhookID
			cfg.WebhookAutoConfigured = true
			fingerprint, err := s.directProviderConfigurationFingerprint(providerID, cfg.AccountID, credential)
			if err != nil {
				return err
			}
			if cfg.ConfigurationFingerprint != fingerprint {
				cfg.ConfigurationGeneration++
				if cfg.ConfigurationGeneration == 0 {
					cfg.ConfigurationGeneration = 1
				}
				cfg.ConfigurationFingerprint = fingerprint
			}
			cfg.CredentialReference = providerCredentialReference(providerID, cfg.ConfigurationGeneration)
			if err := s.storeProviderCredentialTx(tx, cfg.CredentialReference, providerID, cfg.AccountID, cfg.ConfigurationGeneration, fingerprint, credential); err != nil {
				return err
			}
			if err := database.SaveByBusinessKey(tx, &cfg, "provider_id = ?", providerID); err != nil {
				return err
			}
			if cfg.AccountID != "" {
				_, err := s.ensureProviderBindingTx(tx, providerID, cfg.AccountID)
				return err
			}
			return nil
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
	var credential providerCredentialMaterial
	err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("provider_id = ?", providerID).First(&cfg).Error; err != nil {
			return err
		}
		var err error
		credential, err = s.loadProviderCredentialTx(tx, cfg.CredentialReference, cfg.ProviderID, cfg.AccountID, cfg.ConfigurationGeneration, cfg.ConfigurationFingerprint)
		return err
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "reload provider config for %s: %v", providerID, err)
		return
	}
	s.registerProviderFromConfig(providerID, credential.SecretKey, credential.PublicKey, credential.WebhookSecret)
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
		var bindings []models.PaymentProviderBinding
		if err := tx.Read().Where("provider_id = ? AND state = ?", providerID, models.PaymentProviderBindingActive).Find(&bindings).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, binding := range bindings {
			if _, err := tx.UpdateColumns(map[string]interface{}{
				"state": models.PaymentProviderBindingRetired, "retired_at": now,
			}, map[string]interface{}{"binding_id = ?": binding.BindingID}, &models.PaymentProviderBinding{}); err != nil {
				return err
			}
		}
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
		if err := s.CancelPayment(ctx, providerID, sessionID); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID,
				"cancel fiat session %s for order %s: %v", sessionID, order.ID, err)
		} else {
			logger.LogInfoWithIDf(log, s.nodeID,
				"canceled fiat session %s for order %s during provider disconnect", sessionID, order.ID)
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
	credential := providerCredentialMaterial{PublicKey: publishableKey, SecretKey: secretKey, WebhookSecret: webhookSecret}
	p, err := s.providerFactory(providerID, credential, platformMode, opts)
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "%v", err)
		return
	}
	s.registry.Register(p)
	mode := "direct"
	if platformMode {
		mode = "platform"
	}
	logger.LogInfoWithIDf(log, s.nodeID, "registered %s provider (%s mode)", providerID, mode)
}

func defaultFiatProviderFactory(testnet bool) fiatProviderFactory {
	return func(providerID string, credential providerCredentialMaterial, platformMode bool, opts *contracts.PlatformProviderOpts) (contracts.FiatPaymentProvider, error) {
		switch providerID {
		case "stripe":
			mode := stripe.ModeDirect
			if platformMode {
				mode = stripe.ModeConnected
			}
			return stripe.NewProvider(stripe.Config{
				SecretKey:      credential.SecretKey,
				PublishableKey: credential.PublicKey,
				WebhookSecret:  credential.WebhookSecret,
				Mode:           mode,
			}), nil
		case "paypal":
			mode := paypal.ModeDirect
			if platformMode {
				mode = paypal.ModePartner
			}
			cfg := paypal.Config{
				ClientID:     credential.PublicKey,
				ClientSecret: credential.SecretKey,
				WebhookID:    credential.WebhookSecret,
				Mode:         mode,
				Sandbox:      testnet,
			}
			if opts != nil && opts.PayPalPartnerID != "" {
				cfg.PartnerID = opts.PayPalPartnerID
			}
			return paypal.NewProvider(cfg), nil
		default:
			return nil, fmt.Errorf("unknown fiat provider %q, cannot register", providerID)
		}
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
	credentials := make(map[string]providerCredentialMaterial)
	err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("is_active = ?", true).Find(&configs).Error; err != nil {
			return err
		}
		for _, cfg := range configs {
			credential, err := s.loadProviderCredentialTx(tx, cfg.CredentialReference, cfg.ProviderID, cfg.AccountID, cfg.ConfigurationGeneration, cfg.ConfigurationFingerprint)
			if err != nil {
				return fmt.Errorf("load %s provider credential: %w", cfg.ProviderID, err)
			}
			credentials[cfg.ProviderID] = credential
		}
		return nil
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "failed to load fiat provider configs: %v", err)
		return
	}
	for _, cfg := range configs {
		credential := credentials[cfg.ProviderID]
		s.registerProviderFromConfig(cfg.ProviderID, credential.SecretKey, credential.PublicKey, credential.WebhookSecret)
	}
}

// ReconcileFiatOrders checks AWAITING_PAYMENT orders with fiat metadata against
// the payment provider. If the provider reports the payment as succeeded but the
// order has not reached verified states (missed webhook), it triggers the payment flow.
// Orders in AWAITING_PAYMENT_VERIFICATION are handled by PaymentVerificationLoop.
// If the provider reports canceled/failed, it's a no-op (order timeout handles cancellation).
func (s *FiatPaymentAppService) ReconcileFiatOrders(ctx context.Context) {
	s.reconcileFiatPaymentAttempts(ctx)
	s.reconcileProviderActions(ctx)
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

func (s *FiatPaymentAppService) reconcileProviderActions(ctx context.Context) {
	var actions []models.PaymentProviderAction
	now := time.Now().UTC()
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"(state = ? OR (state = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?))) AND (lease_expires_at IS NULL OR lease_expires_at <= ?)",
			models.PaymentProviderActionPendingExternal, models.PaymentProviderActionReconcileRequired, now, now,
		).Order("created_at ASC").Limit(providerActionBatchSize).Find(&actions).Error
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "provider action reconciliation: query failed: %v", err)
		return
	}
	payment.ObserveFiatProviderActionReconcileBatchSize(len(actions))
	if len(actions) > 0 {
		oldestCreatedAt := actions[0].CreatedAt
		for i := 1; i < len(actions); i++ {
			if actions[i].CreatedAt.Before(oldestCreatedAt) {
				oldestCreatedAt = actions[i].CreatedAt
			}
		}
		payment.ObserveFiatProviderActionOldestDueAge(now.Sub(oldestCreatedAt))
	}
	for _, action := range actions {
		if ctx.Err() != nil {
			return
		}
		if _, err := s.executeProviderAction(ctx, action); err != nil &&
			!errors.Is(err, contracts.ErrAlreadyRefunded) && !errors.Is(err, contracts.ErrActionInProgress) {
			logger.LogWarningWithIDf(log, s.nodeID, "provider action reconciliation: action=%s kind=%s: %v", action.ActionID, action.ActionKind, err)
		}
	}
}

func (s *FiatPaymentAppService) reconcileFiatPaymentAttempts(ctx context.Context) {
	var attempts []models.PaymentAttempt
	cutoff := time.Now().Add(-2 * time.Minute)
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"state = ? OR (state = ? AND updated_at < ?)",
			models.PaymentAttemptReconcileRequired, models.PaymentAttemptPendingExternal, cutoff,
		).Find(&attempts).Error
	}); err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "fiat attempt reconciliation: query failed: %v", err)
		return
	}
	for _, attempt := range attempts {
		var route models.PaymentRouteBinding
		if err := s.db.View(func(tx database.Tx) error {
			return tx.Read().Where("route_binding_id = ?", attempt.RouteBindingID).First(&route).Error
		}); err != nil {
			_ = s.markFiatAttemptReconcileRequired(attempt.AttemptID, fmt.Errorf("load route binding: %w", err))
			continue
		}
		binding, provider, err := s.resolveHistoricalProviderBinding(route)
		if err != nil {
			_ = s.markFiatAttemptReconcileRequired(attempt.AttemptID, fmt.Errorf("route decision denied historical provider binding: %w", err))
			continue
		}
		params := contracts.CreatePaymentParams{
			OrderID: attempt.OrderID, Amount: attempt.Amount, Currency: attempt.Currency,
			SellerAccountID: binding.ExternalAccountReference, IdempotencyKey: attempt.IdempotencyKey,
			Metadata: map[string]string{
				"mobazha_payment_attempt_id": attempt.AttemptID,
				"mobazha_route_binding_id":   route.RouteBindingID,
			},
		}
		session, err := provider.CreatePayment(ctx, params)
		if err != nil || session == nil || strings.TrimSpace(session.SessionID) == "" {
			if err == nil {
				err = fmt.Errorf("provider returned an empty session")
			}
			_ = s.markFiatAttemptReconcileRequired(attempt.AttemptID, fmt.Errorf("reconcile provider create: %w", err))
			continue
		}
		if err := s.commitFiatPaymentAttempt(attempt.AttemptID, session.SessionID); err != nil {
			_ = s.markFiatAttemptReconcileRequired(attempt.AttemptID, fmt.Errorf("reconcile provider reference: %w", err))
			continue
		}
		if err := s.persistFiatPaymentMetadata(ctx, attempt, route, session.SessionID); err != nil {
			_ = s.markFiatAttemptReconcileRequired(attempt.AttemptID, fmt.Errorf("reconcile payment metadata: %w", err))
		}
	}
}

func (s *FiatPaymentAppService) resolveHistoricalProviderBinding(route models.PaymentRouteBinding) (models.PaymentProviderBinding, contracts.FiatPaymentProvider, error) {
	if strings.TrimSpace(route.ProviderBindingID) == "" {
		return models.PaymentProviderBinding{}, nil, fmt.Errorf("route %s has no provider binding", route.RouteBindingID)
	}
	var binding models.PaymentProviderBinding
	var credential providerCredentialMaterial
	var historicalCredentialErr error
	err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("binding_id = ?", route.ProviderBindingID).First(&binding).Error; err != nil {
			return err
		}
		if binding.Mode != "direct" {
			return nil
		}
		credential, historicalCredentialErr = s.loadProviderCredentialTx(
			tx, binding.CredentialReference, binding.ProviderID, binding.ExternalAccountReference,
			binding.ConfigurationGeneration, binding.ConfigurationFingerprint,
		)
		return nil
	})
	if err != nil {
		return models.PaymentProviderBinding{}, nil, err
	}
	if binding.ExternalAccountReference != route.ExternalAccountReference {
		return models.PaymentProviderBinding{}, nil, fmt.Errorf("route account does not match provider binding")
	}
	if binding.DriverContributionID != route.ContributionID {
		return models.PaymentProviderBinding{}, nil, fmt.Errorf("route contribution does not match provider binding")
	}
	availability := distribution.PaymentRouteReady
	if binding.State == models.PaymentProviderBindingRetired {
		availability = distribution.PaymentRouteExistingOnly
	}
	decision := distribution.DecidePaymentRoute(distribution.PaymentRouteDecisionRequest{
		WorkMode: distribution.PaymentRouteReconcile, ContributionID: binding.DriverContributionID,
		ProviderBindingID: binding.BindingID, BindingState: binding.State,
		ContributionAvailability: availability, HistoricalImplementationAvailable: historicalCredentialErr == nil,
	})
	if !decision.Allowed {
		if historicalCredentialErr != nil {
			return models.PaymentProviderBinding{}, nil, fmt.Errorf("payment route decision %s: %s: %w", decision.Code, decision.Reason, historicalCredentialErr)
		}
		return models.PaymentProviderBinding{}, nil, fmt.Errorf("payment route decision %s: %s", decision.Code, decision.Reason)
	}
	if binding.Mode == "direct" {
		provider, err := s.providerFactory(binding.ProviderID, credential, false, nil)
		if err != nil {
			return models.PaymentProviderBinding{}, nil, fmt.Errorf("construct provider from credential reference %s: %w", binding.CredentialReference, err)
		}
		return binding, provider, nil
	}
	provider, err := s.registry.ForProvider(binding.ProviderID)
	if err != nil {
		return models.PaymentProviderBinding{}, nil, fmt.Errorf("resolve provider %s: %w", binding.ProviderID, err)
	}
	return binding, provider, nil
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
