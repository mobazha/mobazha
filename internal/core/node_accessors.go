//go:build !private_distribution

package core

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
)

// Context returns the node's lifecycle context. It is cancelled when
// the node is stopped (via Stop or LRU eviction). External goroutines
// tied to this node should select on this context to avoid leaking.
func (n *MobazhaNode) Context() context.Context {
	if n.nodeCtx != nil {
		return n.nodeCtx
	}
	return context.Background()
}

// Service accessors implement contracts.NodeService's accessor pattern.
// Each returns the corresponding App Service (or a thin facade that
// composes multiple App Services when a domain spans several).

func (n *MobazhaNode) IdentityInfo() contracts.IdentityService {
	return &identityInfoAdapter{
		nodeID:         n.nodeID,
		peerID:         n.peerID,
		testnet:        n.testnet,
		signer:         n.signer,
		listingService: n.listingService,
	}
}

// Nil guards below prevent the "typed nil interface" problem: when a concrete
// pointer field is nil, returning it directly yields a non-nil interface value
// that passes `== nil` checks but panics on method calls.

func (n *MobazhaNode) Notification() contracts.NotificationService {
	if n.notificationService == nil {
		return nil
	}
	return n.notificationService
}
func (n *MobazhaNode) Wallet() contracts.WalletService {
	if n.paymentService == nil {
		return nil
	}
	return n.paymentService
}
func (n *MobazhaNode) Media() contracts.MediaService {
	if n.mediaService == nil {
		return nil
	}
	return n.mediaService
}

// MatrixChat returns the node-side Matrix chat service (mautrix-go backed).
// Returns nil if the service hasn't been initialized (e.g. no Matrix config).
func (n *MobazhaNode) MatrixChat() contracts.MatrixChatService {
	if n.matrixChatService == nil {
		return nil
	}
	return n.matrixChatService
}

func (n *MobazhaNode) Preferences() contracts.PreferencesService {
	if n.preferencesService == nil {
		return nil
	}
	return n.preferencesService
}
func (n *MobazhaNode) GuestOrder() contracts.GuestOrderService {
	if n.guestOrderService == nil {
		return nil
	}
	return n.guestOrderService
}
func (n *MobazhaNode) ReceivingAccounts() contracts.ReceivingAccountService {
	if n.receivingAccountService == nil {
		return nil
	}
	return n.receivingAccountService
}

// UnifiedOrders returns the unified order view combining standard and guest orders.
func (n *MobazhaNode) UnifiedOrders() contracts.UnifiedOrderViewService {
	if n.unifiedOrderView == nil {
		return nil
	}
	return n.unifiedOrderView
}

// Features returns the feature-flag resolver composed during node
// construction. May be nil for mock nodes that skipped applyOptions;
// callers should nil-check before invoking resolver methods.
//
// See pkg/contracts.FeaturesProvider for the canonical type assertion
// pattern that handlers should use.
func (n *MobazhaNode) Features() pkgconfig.ResolverInterface {
	if n == nil {
		return nil
	}
	return n.featureResolver
}

// TenantFeatureStore returns the tenant-layer feature override store used
// by administrative write handlers (e.g. PUT /v1/settings/features/{key}).
// Returns nil when the node was constructed without a tenant store (e.g.
// bare test harnesses); callers should nil-check before invoking store
// methods or surface 501 to the client.
//
// Implements contracts.FeatureAdminProvider.
func (n *MobazhaNode) TenantFeatureStore() pkgconfig.TenantFeatureStore {
	if n == nil {
		return nil
	}
	return n.tenantFeatureStore
}

// FeatureAuditLogger returns the feature-flag audit log persister used by
// administrative write handlers to record who toggled which flag. Returns
// nil on bare test harnesses / infrastructure-only nodes (no DB); callers
// MUST nil-check and log-and-continue when it is absent — audit gaps are
// surfaced by ops alerting rather than blocking the underlying write.
//
// Implements contracts.FeatureAuditProvider.
func (n *MobazhaNode) FeatureAuditLogger() contracts.FeatureAuditLogger {
	if n == nil {
		return nil
	}
	return n.featureAuditLogger
}

func (n *MobazhaNode) ShoppingCart() contracts.ShoppingCartService {
	if n.shoppingCartService == nil {
		return nil
	}
	return n.shoppingCartService
}
func (n *MobazhaNode) Wishlist() contracts.WishlistService {
	if n.wishlistService == nil {
		return nil
	}
	return n.wishlistService
}
func (n *MobazhaNode) ExchangeRate() contracts.ExchangeRateService {
	return &exchangeRateAdapter{n.exchangeRates}
}

// Analytics returns the analytics service or nil if not initialized.
func (n *MobazhaNode) Analytics() contracts.AnalyticsService {
	if n.analyticsService == nil {
		return nil
	}
	return n.analyticsService
}

// FiatPaymentProviderAccessor implementation — generic fiat payment subsystem.
func (n *MobazhaNode) Fiat() contracts.FiatService {
	if n.fiatPaymentService == nil {
		return nil
	}
	return n.fiatPaymentService
}

// FiatRegistry returns the fiat provider registry for external provider registration.
// Hosting (SaaS) uses this to register platform-level providers after node creation.
func (n *MobazhaNode) FiatRegistry() contracts.FiatProviderRegistry { return n.fiatRegistry }

// SupplyChainProvider implementation — supply chain fulfillment subsystem.
func (n *MobazhaNode) SupplyChain() contracts.SupplyChainService {
	if n.supplyChainService == nil {
		return nil
	}
	return n.supplyChainService
}

// SupplyChainRegistry returns the fulfillment provider registry.
// Hosting (SaaS) may use this for platform-level provider management.
func (n *MobazhaNode) SupplyChainRegistry() contracts.FulfillmentProviderRegistry {
	return n.supplyChainRegistry
}

// SupplyChainChecker returns the narrow port for checking if a listing is supply-chain-managed.
// Used by PaymentAppService to suppress auto-confirm.
func (n *MobazhaNode) SupplyChainChecker() contracts.SupplyChainChecker {
	if n.supplyChainService == nil {
		return nil
	}
	return n.supplyChainService
}

// WebhookProvider implementation — per-node webhook subsystem.
func (n *MobazhaNode) WebhookStore() wh.EndpointStore { return n.webhookStore }
func (n *MobazhaNode) WebhookEngine() *wh.Engine      { return n.webhookEngine }

// DiscountProvider implementation — per-node discount subsystem.
func (n *MobazhaNode) Discount() contracts.DiscountService {
	if n.discountService == nil {
		return nil
	}
	return n.discountService
}

// CollectionProvider implementation — per-node collection subsystem.
func (n *MobazhaNode) Collection() contracts.CollectionService {
	if n.collectionService == nil {
		return nil
	}
	return n.collectionService
}

// DiscountStore exposes the underlying DiscountStore for cross-tenant wiring
// (e.g., hosting constructs a DiscountEngine with the vendor's store).
func (n *MobazhaNode) DiscountStore() contracts.DiscountStore {
	if n.discountService == nil {
		return nil
	}
	return n.discountService.Store()
}

// ShippingProvider implementation — per-node shipping subsystem.
var _ contracts.ShippingProvider = (*MobazhaNode)(nil)

func (n *MobazhaNode) Shipping() contracts.ShippingService {
	if n.shippingService == nil {
		return nil
	}
	return n.shippingService
}

func (n *MobazhaNode) DigitalAssets() contracts.DigitalAssetService {
	if n.digitalAssetService == nil {
		return nil
	}
	return n.digitalAssetService
}

// PaymentSession returns the unified payment session service (Phase PS / B1).
// Returns nil when the subsystem has not been initialised.
func (n *MobazhaNode) PaymentSession() contracts.PaymentSessionService {
	if n.paymentSessionService == nil {
		return nil
	}
	return n.paymentSessionService
}

func (n *MobazhaNode) Order() contracts.OrderService {
	if n.orderService == nil {
		return nil
	}
	return &orderServiceFacade{
		OrderAppService: n.orderService,
		payment:         n.paymentService,
		settlement:      n.settlementService,
	}
}

func (n *MobazhaNode) Listing() contracts.ListingService {
	if n.listingService == nil {
		return nil
	}
	return &listingServiceFacade{
		ListingAppService: n.listingService,
		moderation:        n.moderationService,
	}
}

func (n *MobazhaNode) Profile() contracts.ProfileService {
	if n.profileService == nil {
		return nil
	}
	return &profileServiceFacade{
		ProfileAppService: n.profileService,
		moderation:        n.moderationService,
	}
}

func (n *MobazhaNode) Social() contracts.SocialService {
	if n.followService == nil {
		return nil
	}
	return &socialServiceFacade{
		FollowAppService:  n.followService,
		RatingsAppService: n.ratingsService,
		PostsAppService:   n.postsService,
	}
}

// --- Facades ---

// exchangeRateAdapter wraps the wallet.ExchangeRateProvider to satisfy
// contracts.ExchangeRateService.
type exchangeRateAdapter struct {
	provider *wallet.ExchangeRateProvider
}

func (a *exchangeRateAdapter) GetAllRates(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("exchange rate provider not available")
	}
	return a.provider.GetAllRates(base, breakCache)
}

func (a *exchangeRateAdapter) GetRate(base models.CurrencyCode, to models.CurrencyCode, breakCache bool) (iwallet.Amount, error) {
	if a.provider == nil {
		return iwallet.NewAmount(0), fmt.Errorf("exchange rate provider not available")
	}
	return a.provider.GetRate(base, to, breakCache)
}

// listingServiceFacade composes ListingAppService + ModerationAppService
// to satisfy contracts.ListingService.
type listingServiceFacade struct {
	*ListingAppService
	moderation *ModerationAppService
}

// profileServiceFacade composes ProfileAppService + ModerationAppService
// to satisfy contracts.ProfileService.
type profileServiceFacade struct {
	*ProfileAppService
	moderation *ModerationAppService
}

func (f *profileServiceFacade) SetSelfAsModerator(ctx context.Context, modInfo *models.ModeratorInfo, done chan struct{}) error {
	return f.moderation.SetSelfAsModerator(ctx, modInfo, done)
}
func (f *profileServiceFacade) RemoveSelfAsModerator(ctx context.Context, done chan<- struct{}) error {
	return f.moderation.RemoveSelfAsModerator(ctx, done)
}
func (f *profileServiceFacade) GetModerators(ctx context.Context) []peer.ID {
	return f.moderation.GetModerators(ctx)
}
func (f *profileServiceFacade) GetModeratorsAsync(ctx context.Context) <-chan peer.ID {
	return f.moderation.GetModeratorsAsync(ctx)
}
func (f *profileServiceFacade) GetVerifiedModerators(ctx context.Context) []peer.ID {
	return f.moderation.GetVerifiedModerators(ctx)
}

// socialServiceFacade composes Follow + Ratings + Posts App Services
// to satisfy contracts.SocialService. All method names are unique across the three
// embedded services, so Go struct embedding handles promotion automatically.
type socialServiceFacade struct {
	*FollowAppService
	*RatingsAppService
	*PostsAppService
}

// identityInfoAdapter composes infrastructure fields and ListingAppService
// to satisfy contracts.IdentityService without MobazhaNode returning itself.
type identityInfoAdapter struct {
	nodeID         string
	peerID         peer.ID
	testnet        bool
	signer         contracts.Signer
	listingService *ListingAppService
}

func (a *identityInfoAdapter) GetNodeID() string  { return a.nodeID }
func (a *identityInfoAdapter) Identity() peer.ID  { return a.peerID }
func (a *identityInfoAdapter) UsingTestnet() bool { return a.testnet }

func (a *identityInfoAdapter) SignMessage(payload []byte) ([]byte, []byte, error) {
	if a.signer == nil {
		return nil, nil, fmt.Errorf("signer not available")
	}
	sig, err := a.signer.Sign(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("signing payload: %w", err)
	}
	pubkey, err := a.signer.PublicKey()
	if err != nil {
		return nil, nil, fmt.Errorf("getting public key: %w", err)
	}
	return sig, pubkey, nil
}

func (a *identityInfoAdapter) IsGlobalBanned(peerID peer.ID) bool {
	if a.listingService == nil {
		return false
	}
	return a.listingService.IsGlobalBanned(peerID)
}

// Compile-time checks for optional accessor interfaces.
var _ contracts.FiatPaymentProviderAccessor = (*MobazhaNode)(nil)
var _ contracts.FeaturesProvider = (*MobazhaNode)(nil)
var _ contracts.FeatureAdminProvider = (*MobazhaNode)(nil)
