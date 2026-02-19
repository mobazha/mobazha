package core

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	wallet "github.com/mobazha/mobazha3.0/internal/wallet"
)

// Service accessors implement contracts.NodeService's accessor pattern.
// Each returns the corresponding App Service (or a thin facade that
// composes multiple App Services when a domain spans several).

func (n *MobazhaNode) IdentityInfo() contracts.IdentityService { return n }

func (n *MobazhaNode) IsGlobalBanned(peerID peer.ID) bool {
	if n.listingService == nil {
		return false
	}
	return n.listingService.IsGlobalBanned(peerID)
}
func (n *MobazhaNode) Chat() contracts.ChatService                 { return n.chatService }
func (n *MobazhaNode) Notification() contracts.NotificationService { return n.notificationService }
func (n *MobazhaNode) Wallet() contracts.WalletService             { return n.paymentService }
func (n *MobazhaNode) Media() contracts.MediaService               { return n.mediaService }
func (n *MobazhaNode) Matrix() contracts.MatrixService             { return n.matrixService }
func (n *MobazhaNode) Preferences() contracts.PreferencesService   { return n.preferencesService }
func (n *MobazhaNode) ShoppingCart() contracts.ShoppingCartService  { return n.shoppingCartService }
func (n *MobazhaNode) Stripe() contracts.StripeService             { return n.paymentService }
func (n *MobazhaNode) ExchangeRate() contracts.ExchangeRateService { return &exchangeRateAdapter{n.exchangeRates} }

// Order still returns MobazhaNode itself because ViaRelay methods and
// PaymentService-backed instructions are not yet on OrderAppService.
func (n *MobazhaNode) Order() contracts.OrderService { return n }

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
