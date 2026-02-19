package core

import "github.com/mobazha/mobazha3.0/pkg/contracts"

// Service accessors implement contracts.NodeService's accessor pattern.
// Each returns the corresponding App Service (or MobazhaNode itself for
// services that are implemented directly on the node or span multiple
// internal services).

func (n *MobazhaNode) IdentityInfo() contracts.IdentityService     { return n }
func (n *MobazhaNode) Chat() contracts.ChatService                 { return n.chatService }
func (n *MobazhaNode) Notification() contracts.NotificationService { return n.notificationService }
func (n *MobazhaNode) Wallet() contracts.WalletService             { return n.paymentService }
func (n *MobazhaNode) Media() contracts.MediaService               { return n.mediaService }
func (n *MobazhaNode) Matrix() contracts.MatrixService             { return n.matrixService }
func (n *MobazhaNode) Preferences() contracts.PreferencesService   { return n.preferencesService }
func (n *MobazhaNode) ShoppingCart() contracts.ShoppingCartService  { return n.shoppingCartService }

// The following return MobazhaNode itself because the App Service doesn't
// yet implement the full contract (cross-cutting methods like relay orders,
// moderation, and social are still on MobazhaNode). Future iteration will
// move these into the App Services.
func (n *MobazhaNode) Order() contracts.OrderService               { return n }
func (n *MobazhaNode) Listing() contracts.ListingService           { return n }
func (n *MobazhaNode) Profile() contracts.ProfileService           { return n }
func (n *MobazhaNode) Social() contracts.SocialService             { return n }
func (n *MobazhaNode) Stripe() contracts.StripeService             { return n }
func (n *MobazhaNode) ExchangeRate() contracts.ExchangeRateService { return n }
