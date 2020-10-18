export default {
    OnboardingWrapper: {
        unknown: 'Unknown',
    },
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
                unknown: 'Unknown'
            },
            ModalImageIndicator: {
                posInfo: "%{pos} of %{size}"
            },
            MoreButton: {
                more: 'More'
            },
            PayBanner: {
                calculating: 'calculating...',
            },
            PaymentMethod: {
                wallet_empty: 'Your wallet is empty',
                add_funds: 'Add Funds'
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
                unavailable: 'Unavailable'
            }
        },

        molecules: {
            BlockedNodeItem: {
                unknown: 'Unknown',
                unblock: 'Unblock',
                block: 'Block'
            },
            BuyerReview: {
                from: "From %{content}",
                no_review: "No review message from buyer"
            },
            BuyWyre: {
                ask_crypto: "Need crypto?",
                top_up: "Top-up your wallet with Wyre!"
            },
            CheckoutNote: {
                add_note: "Add a note to your order (optional)",
                add_seller_note: "Add a note for the seller"
            },
            DirectPaymentOption: {
                direct_payment: "Direct Payment",
                description: "Proceed without a moderator. Send funds directly to the vendor. Use with caution. Do not use unless you completely trust the vendor"
            },
            FeedItem: {
                reposted: "reposted"
            },
            FeedPreview: {
                anonymous: "Anonymous"
            },
            ListingPaymentOptions: {
                not_accepted: "Not accepted",
                payment_options: "Payment Options"
            },
            ListingReview: {
                from: "From %{name}",
                no_message: "No review message from buyer"
            },
            ModerationFee: {
                percentage: "Percentage (%)",
                flat_fee: "Flat Fee (%)",
                fee: "Fee ($)"
            },
            ProductDescription: {
                empty_text: "No description provided",
                read_more: "Read more"
            },
            ProductPolicy: {
                no_provided: "No %{policy} provided"
            },
            RadioModalFilter: {
                reason_reporting: "Please enter a reason for reporting this content.",
                other: "Other: please explain",
            },
            SellerInfo: {
                about: "About the Seller",
                unknown: "Unknown",
                message: "Message",
                visit_store: "Visit store"
            },
            ShopCard: {
                unknown: "Unknown"
            },
            ShopInfo: {
                unknown: "Unknown",
                following: "following",
                followers: "followers",
                edit_profile: "Edit Profile",
                message: "Message",
            }
        },
        WalletCoinItem: {
            coming_soon: "Coming Soon"
        },

        organism: {
            AverageRating: {
                no_reviews: "No reviews yet"
            },
            Balance: {
                unconfirmed:"%{balance} unconfirmed"
            }, 
            CategorySelector: {
                category: "Category",
                select_category: "Select a category"
            }, 
            CheckoutHeader: {
                anonymous: "Anonymous",
                from_seller: "from %{seller}",
                each_price: "%{price} / each"
            }, 
            CheckoutSummary: {
                remove_coupon: "Remove coupon?",
                remove_coupon_description: "Are you sure you want to remove this coupon?",
                cancel: "Cancel",
                remove: "Remove",
                quantity_info: "Quantity: %{quantity}",
                coupon_info: "Coupon: %{info}",
                change: "Change",
                free: "FREE",
                network_fee: "Network Fee",
                fee_alert_description: "Fee is too high. Please use a lower fee level or a different coin.",
                learn_more: "Learn more",
                total: "Total",
                calculating: "calculating..."
            }, 
            CoinTypeSelector: {
                coming_soon: "Coming Soon"
            }, 
            DefaultInventoryItem: {
                no_title: "No listing title",
                sku: "SKU",
                sku_info: "SKU, ID, etc",
                quantity: "Quantity",
                unlimited: "Unlimited",
                quantity_sold_out: 'If the quantity reaches 0, it will display as "sold out".',
                quantity_unlimit: 'Consumers can purchase as much as they\'d like.'
            }, 
            EmptyCoupons: {}, EmptyShippingMethods: {}, ErrorModal: {}, InventoryItem: {}, ItemDetail: {}, ModerationSettingsEditor: {}, Moderator: {}, ModeratorPreview: {}, MultiSelector: {}, OptionSelector: {}, OrderBrief: {}, OrderDispute: {}, OrderFooter: {}, OrderFulfillment: {}, OrderRating: {}, PayPanel: {}, ProductRatings: {}, QRScanner: {}, SearchFilterHeader: {}, SelectableModerator: {}, SelectorModal: {}, SendingAddressSelector: {}, SendingAmount: {}, ShippingMethod: {}, ShippingOptions: {}, ShippingPriceEditor: {}, SingleVariantEditor: {}, SupportHaven: {}, TagEditor: {}, TagSuggestion: {}, PanelView: { PanelViewBase: {} }
        },

        templates: {
            AboutTab: {}, BackupWallet: {}, CategoryList: {}, ChatDetail: {}, Checkout: {}, ContractModal: {}, CouponApplyModal: {}, CouponModal: {}, CovidModal: {}, DisputeModal: {}, EULAModal: {}, feed: {}, FeedDetail: {}, FulfillModal: {}, GlobalFeed: {}, InfiniteProducts: {}, InventoryEditor: {}, InventoryList: {}, ListingAdvancedDetails: {}, ListingsTab: {}, NeedCoin: {}, OrderState: {}, OrderSummary: {}, PurchaseState: {}, RatingModal: {}, ReportTemplate: {}, SearchResults: {}, SendMoney: {}, SendReceiveMoney: {}, Settings: {}, StoreModeratorList: {}, Toast: {}, TransactionHistory: {}, UserSearchResults: {}, wishlist: {}
        }
    },

    screens: {
        acceptedCoins: {}, addShippingMethod: {}, analytics: {}, backupProfileInit: {}, backupProfilePassword: {}, backupProfileUpload: {}, categoryOverview: {}, chats: {}, checkoutModerators: {}, editShippingAddress: {}, externalPay: {}, externalStore: {}, followers: {}, followings: {}, listing: {}, listingAdvancedOptions: {}, Me: {}, moderatorDetails: {}, onboarding: {}, orderDetails: {}, paymentMethod: {}, paymentSuccess: {}, policies: {}, privacy: {}, ProductRatings: {}, profileSettings: {}, purchaseSuccess: {}, receiveMoney: {}, restoreProfileInit: {}, restoreProfilePassword: {}, Resync: {}, searchFilter: {}, shippingAddress: {}, StoreRatings: {}
    },
    utils: { ratings: {} },
}
