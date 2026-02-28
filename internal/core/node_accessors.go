package core

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	corecontracts "github.com/mobazha/mobazha-core/contracts"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
)

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
func (n *MobazhaNode) Chat() contracts.ChatService                 { return n.chatService }
func (n *MobazhaNode) Notification() contracts.NotificationService { return n.notificationService }
func (n *MobazhaNode) Wallet() contracts.WalletService             { return n.paymentService }
func (n *MobazhaNode) Media() contracts.MediaService               { return n.mediaService }
func (n *MobazhaNode) Matrix() contracts.MatrixService             { return n.matrixService }
func (n *MobazhaNode) Preferences() contracts.PreferencesService   { return n.preferencesService }
func (n *MobazhaNode) ShoppingCart() contracts.ShoppingCartService  { return n.shoppingCartService }
func (n *MobazhaNode) Wishlist() contracts.WishlistService          { return n.wishlistService }
// Deprecated: Stripe returns the legacy StripeService backed by PaymentAppService.
// New code should use FiatPaymentProviderAccessor via type assertion instead.
func (n *MobazhaNode) Stripe() contracts.StripeService             { return n.paymentService }
func (n *MobazhaNode) ExchangeRate() contracts.ExchangeRateService { return &exchangeRateAdapter{n.exchangeRates} }

// FiatPaymentProviderAccessor implementation — generic fiat payment subsystem.
func (n *MobazhaNode) Fiat() contracts.FiatService { return n.fiatPaymentService }

// FiatRegistry returns the fiat provider registry for external provider registration.
// Hosting (SaaS) uses this to register platform-level providers after node creation.
func (n *MobazhaNode) FiatRegistry() contracts.FiatProviderRegistry { return n.fiatRegistry }

// WebhookProvider implementation — per-node webhook subsystem.
func (n *MobazhaNode) WebhookStore() wh.EndpointStore { return n.webhookStore }
func (n *MobazhaNode) WebhookEngine() *wh.Engine      { return n.webhookEngine }

// DiscountProvider implementation — per-node discount subsystem.
func (n *MobazhaNode) Discount() contracts.DiscountService { return n.discountService }

// CollectionProvider implementation — per-node collection subsystem.
func (n *MobazhaNode) Collection() contracts.CollectionService { return n.collectionService }

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

func (n *MobazhaNode) Shipping() contracts.ShippingService { return n.shippingService }

func (n *MobazhaNode) Order() contracts.OrderService {
	return &orderServiceFacade{
		OrderAppService: n.orderService,
		payment:         n.paymentService,
	}
}

func (n *MobazhaNode) Listing() contracts.ListingService {
	return &listingServiceFacade{
		ListingAppService: n.listingService,
		moderation:        n.moderationService,
	}
}

func (n *MobazhaNode) Profile() contracts.ProfileService {
	return &profileServiceFacade{
		ProfileAppService: n.profileService,
		moderation:        n.moderationService,
	}
}

func (n *MobazhaNode) Social() contracts.SocialService {
	return &socialServiceFacade{
		FollowAppService:   n.followService,
		RatingsAppService:  n.ratingsService,
		PostsAppService:    n.postsService,
		ChannelsAppService: n.channelsService,
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

func (f *listingServiceFacade) SetModeratorsOnListings(mods []peer.ID, done chan struct{}) error {
	return f.moderation.SetModeratorsOnListings(mods, done)
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

// socialServiceFacade composes Follow + Ratings + Posts + Channels App Services
// to satisfy contracts.SocialService. All method names are unique across the four
// embedded services, so Go struct embedding handles promotion automatically.
type socialServiceFacade struct {
	*FollowAppService
	*RatingsAppService
	*PostsAppService
	*ChannelsAppService
}

// identityInfoAdapter composes infrastructure fields and ListingAppService
// to satisfy contracts.IdentityService without MobazhaNode returning itself.
type identityInfoAdapter struct {
	nodeID         string
	peerID         peer.ID
	testnet        bool
	signer         corecontracts.Signer
	listingService *ListingAppService
}

func (a *identityInfoAdapter) GetNodeID() string    { return a.nodeID }
func (a *identityInfoAdapter) Identity() peer.ID     { return a.peerID }
func (a *identityInfoAdapter) UsingTestnet() bool    { return a.testnet }

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
