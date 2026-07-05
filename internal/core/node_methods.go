package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	aipkg "github.com/mobazha/mobazha/internal/ai"
	"github.com/mobazha/mobazha/internal/config"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/notifier"
	"github.com/mobazha/mobazha/internal/wallet"
	agentstore "github.com/mobazha/mobazha/pkg/agent/store"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// Identity returns the peer ID for this node.
func (n *MobazhaNode) Identity() peer.ID {
	return n.peerID
}

// PrivKey returns the libp2p private key for this node.
func (n *MobazhaNode) PrivKey() crypto.PrivKey {
	return n.privKey
}

// SignMessage signs a payload with the node's identity key via the injected Signer.
func (n *MobazhaNode) SignMessage(payload []byte) ([]byte, []byte, error) {
	if n.signer == nil {
		return nil, nil, fmt.Errorf("signer not available")
	}
	sig, err := n.signer.Sign(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("signing payload: %w", err)
	}
	pubkey, err := n.signer.PublicKey()
	if err != nil {
		return nil, nil, fmt.Errorf("getting public key: %w", err)
	}
	return sig, pubkey, nil
}

// PeerHost returns the libp2p host for this node.
func (n *MobazhaNode) PeerHost() host.Host {
	return n.peerHost
}

// Multiwallet returns the WalletOperator interface.
func (n *MobazhaNode) Multiwallet() contracts.WalletOperator {
	return n.multiwallet
}

// ExchangeRates returns the node's exchange rate provider.
func (n *MobazhaNode) ExchangeRates() *wallet.ExchangeRateProvider {
	return n.exchangeRates
}

// NetService returns the underlying NetworkService for this node.
func (n *MobazhaNode) NetService() contracts.NetworkService {
	return n.networkService
}

// NetConfig returns the network configuration.
func (n *MobazhaNode) NetConfig() *config.NetConfig {
	return n.netConfig
}

// SetCoTenantPublicData injects a resolver for co-located tenant data on the
// same SaaS host.
func (n *MobazhaNode) SetCoTenantPublicData(fn contracts.CoTenantPublicDataFn) {
	if n.sovereign {
		return
	}
	n.coTenantPublicData = fn
}

// SetCoTenantDigitalAssets injects a resolver for co-located tenant digital
// asset services on the same SaaS host.
func (n *MobazhaNode) SetCoTenantDigitalAssets(fn contracts.CoTenantDigitalAssetsFn) {
	if n.sovereign {
		return
	}
	n.coTenantDigitalAssets = fn
	if n.digitalAssetService != nil {
		n.digitalAssetService.SetCoTenantDigitalAssets(fn)
	}
}

func (n *MobazhaNode) SetCoTenantVerifiedPayment(fn contracts.CoTenantVerifiedPaymentFn) {
	if n.sovereign {
		return
	}
	n.coTenantVerifiedPayment = fn
	if n.orderService != nil {
		n.orderService.SetCoTenantVerifiedPayment(fn)
	}
}

func (n *MobazhaNode) ProcessCoTenantVerifiedPayment(ctx context.Context, orderMsg *npb.OrderMessage, tx iwallet.Transaction) error {
	if n.sovereign {
		return fmt.Errorf("co-tenant verified payment not available for sovereign nodes")
	}
	if n.orderService == nil {
		return fmt.Errorf("order service not configured")
	}
	return n.orderService.ProcessVerifiedPaymentMessage(ctx, orderMsg, tx)
}

// coTenantPublicDataDeferred returns a closure that forwards to
// n.coTenantPublicData at call time.
func (n *MobazhaNode) coTenantPublicDataDeferred() contracts.CoTenantPublicDataFn {
	return func(peerID peer.ID) (database.PublicData, error) {
		fn := n.coTenantPublicData
		if fn == nil {
			return nil, fmt.Errorf("co-tenant resolver not configured")
		}
		return fn(peerID)
	}
}

// coTenantDigitalAssetsDeferred returns a closure that forwards to
// n.coTenantDigitalAssets at call time.
func (n *MobazhaNode) coTenantDigitalAssetsDeferred() contracts.CoTenantDigitalAssetsFn {
	return func(peerID peer.ID) (contracts.DigitalAssetService, error) {
		fn := n.coTenantDigitalAssets
		if fn == nil {
			return nil, fmt.Errorf("co-tenant digital asset resolver not configured")
		}
		return fn(peerID)
	}
}

// NotifierSink returns the node's channel notification sink (may be nil).
func (n *MobazhaNode) NotifierSink() *notifier.ChannelNotificationSink {
	return n.notifierSink
}

// SaveNotificationChannels persists channel configs to the database.
func (n *MobazhaNode) SaveNotificationChannels(channels []notifier.ChannelConfig) error {
	data, err := json.Marshal(channels)
	if err != nil {
		return fmt.Errorf("marshal notification channels: %w", err)
	}
	return n.saveSetting(models.SettingsKeyNotificationChannels, string(data))
}

// loadNotificationChannels reads the persisted channel configs from the database.
func (n *MobazhaNode) loadNotificationChannels() []notifier.ChannelConfig {
	val, err := n.getSetting(models.SettingsKeyNotificationChannels)
	if err != nil || val == "" {
		return nil
	}
	var channels []notifier.ChannelConfig
	if err := json.Unmarshal([]byte(val), &channels); err != nil {
		return nil
	}
	return channels
}

// AIConfigForGenerate returns the best AI config for a generate request.
func (n *MobazhaNode) AIConfigForGenerate(req aipkg.GenerateRequest) (aipkg.Config, error) {
	mc := n.AIMultiConfig()
	userCfg := mc.ActiveConfig()
	if n.sovereign {
		return userCfg, nil
	}
	return n.PlatformAIProfile().ForGenerate(userCfg, req)
}

// AIConfigForChat returns the best AI config for chat messages.
func (n *MobazhaNode) AIConfigForChat(messages []aipkg.ChatMsg) (aipkg.Config, error) {
	mc := n.AIMultiConfig()
	userCfg := mc.ActiveConfig()
	if n.sovereign {
		return userCfg, nil
	}
	return n.PlatformAIProfile().ForChat(userCfg, messages)
}

func (n *MobazhaNode) distributionAIRateLimiter() *aipkg.DailyRateLimiter {
	if n.sovereign {
		return nil
	}
	return n.aiRateLimiter
}

func (n *MobazhaNode) distributionPlatformAIConfig() *aipkg.Config {
	if n.sovereign {
		return nil
	}
	profile := n.PlatformAIProfile()
	if profile.TextAvailable() {
		return profile.Text
	}
	if profile.VisionAvailable() {
		return profile.Vision
	}
	return nil
}

// PlatformAIProfile returns the platform-provided AI routes.
func (n *MobazhaNode) PlatformAIProfile() aipkg.PlatformProfile {
	n.platformAIProfileMu.RLock()
	defer n.platformAIProfileMu.RUnlock()

	return n.platformAIProfile
}

// SetAIProfile updates distribution-provided AI routes.
func (n *MobazhaNode) SetAIProfile(profile contracts.AIProfile) {
	n.platformAIProfileMu.Lock()
	defer n.platformAIProfileMu.Unlock()

	n.platformAIProfile = aipkg.PlatformProfile{
		Text:   aiEndpointConfig(profile.Text, profile.DailyLimit),
		Vision: aiEndpointConfig(profile.Vision, profile.DailyLimit),
	}
}

func aiEndpointConfig(endpoint contracts.AIEndpointConfig, dailyLimit int) *aipkg.Config {
	if endpoint.Provider == "" || endpoint.APIKey == "" {
		return nil
	}
	cfg := &aipkg.Config{
		Provider:   endpoint.Provider,
		APIKey:     endpoint.APIKey,
		Model:      endpoint.Model,
		BaseURL:    endpoint.BaseURL,
		Enabled:    true,
		IsPlatform: true,
		DailyLimit: dailyLimit,
	}
	if !cfg.IsValid() {
		return nil
	}
	return cfg
}

// AgentStore returns an agent runtime store backed by this node's database.
func (n *MobazhaNode) AgentStore() agentstore.Persistence {
	return agentstore.NewGormPersistence(n.db)
}

// ProfileName returns the display name of this node's store profile.
func (n *MobazhaNode) ProfileName() string {
	ps := n.Profile()
	if ps == nil {
		return ""
	}
	profile, err := ps.GetMyProfile()
	if err != nil || profile == nil {
		return ""
	}
	return profile.Name
}

// ProductCatalog returns a lightweight summary of all published listings
// for AI context injection.
func (n *MobazhaNode) ProductCatalog() []aipkg.ListingSummary {
	var index models.ListingIndex
	err := n.db.View(func(tx database.Tx) error {
		var e error
		index, e = tx.GetListingIndex()
		return e
	})
	if err != nil || len(index) == 0 {
		return nil
	}

	var result []aipkg.ListingSummary
	for i := range index {
		lm := &index[i]
		if lm.Status != models.ListingStatusPublished {
			continue
		}
		price := ""
		if lm.Price.Currency != nil {
			price = aipkg.FormatAmountForDisplay(lm.Price.Amount.String(), lm.Price.Currency.Divisibility)
		}
		result = append(result, aipkg.ListingSummary{
			Slug:        lm.Slug,
			Title:       lm.Title,
			Description: lm.Description,
			Price:       price,
			CoinType:    lm.CoinType,
			ProductType: lm.ProductType,
		})
	}
	return result
}

// ---------------------------------------------------------------------------
// contracts.SchedulerHooks — delegate to App Services (Phase AH-3a)
// ---------------------------------------------------------------------------

var _ contracts.SchedulerHooks = (*MobazhaNode)(nil)

func (n *MobazhaNode) RunOrderTimeoutOnce(_ context.Context) {
	if n.orderService != nil {
		n.orderService.RunOrderTimeoutOnce()
	}
}

func (n *MobazhaNode) RunOutboxPollOnce(ctx context.Context) {
	if n.orderService != nil {
		n.orderService.RunOutboxPollOnce()
	}
	n.runExtensionDeliveries(ctx)
}

func (n *MobazhaNode) RunOutboxCleanupOnce(_ context.Context) {
	if n.orderService != nil {
		n.orderService.RunOutboxCleanupOnce()
	}
}

// RunPaymentReconcileScanOnce resumes persistence-first payment provisioning.
// First-party modules still own observation loops; Core owns durable attempt
// recovery because it owns the idempotency claim and historical route.
func (n *MobazhaNode) RunPaymentReconcileScanOnce(ctx context.Context) {
	if n.directPaymentService != nil {
		if _, err := n.directPaymentService.RecoverPendingExternalPaymentAddresses(ctx); err != nil {
			logger.LogWarningWithIDf(log, n.nodeID, "direct observed address reconciliation: %v", err)
		}
	}
}

func (n *MobazhaNode) RunPaymentVerificationOnce(_ context.Context) {
	if n.paymentService != nil {
		n.paymentService.RunPaymentVerificationOnce()
	}
}

func (n *MobazhaNode) RunSettlementActionConfirmationsOnce(ctx context.Context) {
	n.runSettlementActionConfirmationsOnce(ctx)
}

func (n *MobazhaNode) RunManagedRelayConfirmationsOnce(ctx context.Context) {
	n.runManagedRelayConfirmationsOnce(ctx)
}

func (n *MobazhaNode) RunWebhookDeliveryOnce(_ context.Context) {
	if n.webhookEngine != nil {
		n.webhookEngine.RunDeliveryOnce()
	}
}

func (n *MobazhaNode) RunWebhookCleanupOnce(_ context.Context) {
	if n.webhookEngine != nil {
		n.webhookEngine.RunCleanupOnce()
	}
}

func (n *MobazhaNode) RunAnalyticsCleanupOnce(_ context.Context) {
	if n.analyticsService != nil {
		n.analyticsService.RunAnalyticsCleanupOnce()
	}
}

func (n *MobazhaNode) RunFiatReconciliationOnce(_ context.Context) {
	if n.fiatPaymentService != nil {
		n.fiatPaymentService.RunFiatReconciliationOnce()
	}
}

func (n *MobazhaNode) RunFiatCleanupOnce(_ context.Context) {
	if n.fiatPaymentService != nil {
		n.fiatPaymentService.RunFiatCleanupOnce()
	}
}

func (n *MobazhaNode) RunGuestOrderCleanupOnce(_ context.Context) {
	if n.guestOrderService != nil {
		n.guestOrderService.RunGuestCleanupOnce()
	}
}

func (n *MobazhaNode) RunFollowerConnectOnce(_ context.Context) {
	if n.followerTracker != nil {
		n.followerTracker.RunFollowerConnectOnce()
	}
}

func (n *MobazhaNode) RunNetDBReconcileOnce(_ context.Context) {
	if n.netDBSyncService != nil {
		n.netDBSyncService.Reconcile()
	}
}

func (n *MobazhaNode) RunOrderLockCleanupOnce(_ context.Context) {
	if n.orderLockManager != nil {
		n.orderLockManager.RunLockCleanupOnce()
	}
}

func (n *MobazhaNode) supplyChainWorkersEnabled() bool {
	if n.supplyChainService == nil {
		return false
	}
	// Feature gate via Resolver (SSOT) — featureManager only reads DefaultValue
	// and would silently keep workers off when hosting flips the platform flag.
	if n.featureResolver == nil {
		return true
	}
	return n.featureResolver.IsEnabled(context.Background(), pkgconfig.FeatureSupplyChainEnabled.Key)
}

func (n *MobazhaNode) RunSupplyChainRetryOnce(ctx context.Context) {
	if n.supplyChainWorkersEnabled() {
		n.supplyChainService.RunSupplyChainRetryOnce(ctx)
	}
}

func (n *MobazhaNode) RunSupplyChainReconcileOnce(ctx context.Context) {
	if n.supplyChainWorkersEnabled() {
		n.supplyChainService.RunSupplyChainReconcileOnce(ctx)
	}
}

func (n *MobazhaNode) RunSupplyChainCleanupOnce(_ context.Context) {
	if n.supplyChainWorkersEnabled() {
		n.supplyChainService.RunSupplyChainCleanupOnce()
	}
}

func (n *MobazhaNode) RunSupplyChainInventoryCheckOnce(ctx context.Context) {
	if n.supplyChainWorkersEnabled() {
		n.supplyChainService.RunSupplyChainInventoryCheckOnce(ctx)
	}
}

func (n *MobazhaNode) RunSupplyChainPriceDriftOnce(ctx context.Context) {
	if n.supplyChainWorkersEnabled() {
		n.supplyChainService.RunSupplyChainPriceDriftOnce(ctx)
	}
}
