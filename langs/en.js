export default {
    english: 'english',
    chinese: 'chinese',
    Onboarding: {
        unknown: 'Unknown',
    },

    OnboardingWrapper: {},
    components: {
        atoms: {
            ChatBubble: {
                fail_retry: 'Couldn\'t send. Tap to retry',
            },
            Comment: {
                posting: 'Posting...'
            },
            FollowButton: {
                following: 'Following',
                follow: 'Follow',
            },
            Inventory: {
                surcharge: 'Surcharge',
                stock: 'Stock',
                unlimited: 'Unlimited',
            }, 
            LocationPin: {
                unknown:'Unknown'
            }, 
            ModalImageIndicator: {
                posInfo: '%{pos} of %{size}'
            }, 
            MoreButton: {
                more:'More'
            }, 
            PayBanner: {
                calculating: 'calculating...',
            }, 
            PaymentMethod: {
                wallet_empty: 'Your wallet is empty',
                add_funds:'Add Funds'
            }, 
            PostButton: {
                post: 'Post'
            }, 
            ProductPrice: {
                free_shipping: 'Free shipping',
                shipping: 'shipping',
            }, 
            ResetFilter: {
                reset_filters: 'Reset filters'
            }, 
            SecureFund: {
                secure_funds: 'Secure your funds',
                backup_wallet: 'Backup wallet'
            }, 
            UnavailableButton: {
                unavailable:'Unavailable'
            }
        },

        molecules: { BlockedNodeItem: {}, BuyerReview: {}, BuyWyre: {}, CheckoutNote: {}, DirectPaymentOption: {}, FeedItem: {}, FeedPreview: {}, ListingPaymentOptions: {}, ListingReview: {}, ModerationFee: {}, ProductDescription: {}, ProductPolicy: {}, RadioModalFilter: {}, SellerInfo: {}, ShopCard: {}, ShopInfo: {} }, WalletCoinItem: {},

        organism: { AverageRating: {}, Balance: {}, CategorySelector: {}, CheckoutHeader: {}, CheckoutSummary: {}, CoinTypeSelector: {}, DefaultInventoryItem: {}, EmptyCoupons: {}, EmptyShippingMethods: {}, ErrorModal: {}, InventoryItem: {}, ItemDetail: {}, ModerationSettingsEditor: {}, Moderator: {}, ModeratorPreview: {}, MultiSelector: {}, OptionSelector: {}, OrderBrief: {}, OrderDispute: {}, OrderFooter: {}, OrderFulfillment: {}, OrderRating: {}, PayPanel: {}, ProductRatings: {}, QRScanner: {}, SearchFilterHeader: {}, SelectableModerator: {}, SelectorModal: {}, SendingAddressSelector: {}, SendingAmount: {}, ShippingMethod: {}, ShippingOptions: {}, ShippingPriceEditor: {}, SingleVariantEditor: {}, SupportHaven: {}, TagEditor: {}, TagSuggestion: {}, PanelView: { PanelViewBase: {} } },

        templates: {
            AboutTab: {}, BackupWallet: {}, CategoryList: {}, ChatDetail: {}, Checkout: {}, ContractModal: {}, CouponApplyModal: {}, CouponModal: {}, CovidModal: {}, DisputeModal: {}, EULAModal: {}, feed: {}, FeedDetail: {}, FulfillModal: {}, GlobalFeed: {}, InfiniteProducts: {}, InventoryEditor: {}, InventoryList: {}, ListingAdvancedDetails: {}, ListingsTab: {}, NeedCoin: {}, OrderState: {}, OrderSummary: {}, PurchaseState: {}, RatingModal: {}, ReportTemplate: {}, SearchResults: {}, SendMoney: {}, SendReceiveMoney: {}, Settings: {}, StoreModeratorList: {}, Toast: {}, TransactionHistory: {}, UserSearchResults: {}, wishlist: {}
        }
    },

    screens: {
        acceptedCoins: {}, addShippingMethod: {}, analytics: {}, backupProfileInit: {}, backupProfilePassword: {}, backupProfileUpload: {}, categoryOverview: {}, chats: {}, checkoutModerators: {}, editShippingAddress: {}, externalPay: {}, externalStore: {}, followers: {}, followings: {}, listing: {}, listingAdvancedOptions: {}, Me: {}, moderatorDetails: {}, onboarding: {}, orderDetails: {}, paymentMethod: {}, paymentSuccess: {}, policies: {}, privacy: {}, ProductRatings: {}, profileSettings: {}, purchaseSuccess: {}, receiveMoney: {}, restoreProfileInit: {}, restoreProfilePassword: {}, Resync: {}, searchFilter: {}, shippingAddress: {}, StoreRatings: {}
    },
    utils: { ratings: {} },
}
