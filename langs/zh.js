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
                posting: '发送...'
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
            EmptyCoupons: {
                empty_coupon: "You haven’t added any coupons",
                add_coupon: "Add coupon"
            }, 
            EmptyShippingMethods: {
                empty_shipping_option: "You haven’t added any shipping options",
                add_shipping: "Add shipping option"
            }, 
            ErrorModal: {
                error_message: "Error: %{error}"
            }, 
            InventoryItem: {
                quantity_info: "QTY: %{quantity}",
                Unlimited: "Unlimited"
            }, 
            ItemDetail: {
                listing: "Listing",
                type: "Type",
                title: "Title",
                ask_selling: "What are you selling?",
                price: "Price",
                condition: "Condition",
                description: "Description",
                description_hint: "Describe your listing here",
                mature_hint: "Mature Content (NSFW, adult, 18+)"
            }, 
            ModerationSettingsEditor: {
                profile_info: "Profile Information",
                description: "Description",
                terms: "Terms of Service",
                languages: "Languages",
                primary: "Primary",
                secondary: "Secondary",
                third: "Third"
            }, 
            Moderator: {
                unknown_moderator: "Unknown moderator",
                unknown_moderator_description: "Could not fetch moderator profile.",
                verified: "verified"
            }, 
            ModeratorPreview: {
                verified: "verified"
            },
            MultiSelector: {
                select_info: "%{count} Selected"
            },
            OptionSelector: {
            }, 
            OrderBrief: {
                tap_to: "Current market price, tap to ",
                learn_more: "learn more"
            }, 
            OrderDispute: {
                ask_payout: "Accept payout?",
                payout_description: "Once accepted, the dispute will close and the funds will transfer",
                cancel: "Cancel",
                ok: "Ok",
                dispute_expired: "Dispute expired",
                memo_comment1: "The moderator has not proposed an outcome. The seller can claim the payment.",
                dispute_payout: "Dispute payout",
                will_be_issued: " will be issued to you.",
                moderator_takes :"Moderator takes ",
                seller_takes :"Seller takes ",
                buyer_takes :"Buyer takes ",
                accept_payout:"Accept payout",
                started_by: "Dispute started by %{name}",
                the_seller: "the seller",
                the_buyer: "the buyer",
                memo_comment2: "The moderator has stepped in to help. Start chatting to provide more details.",
                message: "Message"
            }, 
            OrderFooter: {

            }, OrderFulfillment: {
                no_tracking_number: "No tracking number to copy!",
                shipping_via: "Shipping via",
                tracking_num: "Tracking #",
                tracking_number_copied: "Tracking number copied!",
                file_url: "File URL:",
                password: "Password:",
                fulfilled_info: "This order has been fulfilled!",
                order_fulfilled: "Order fulfilled"
            }, 
            OrderRating: {
                Overall:"Overall",
                Quality:"Quality",
                as_advertised: "As advertised",
                Delivery: "Delivery",
                Service: "Service",
                no_feedback: "No feedback left by %{name}"
            }, 
            PayPanel: {
                ask_pay: "How would you like to pay?",
                external_wallet: "External Wallet",
                not_available_eth: "Not available for ETH",
                haven_wallet: "Haven Wallet",
                not_enough_funds: "Not enough funds"
            }, 
            ProductRatings: {
                reviews: "Reviews",
            }, 
            QRScanner: {
                scan_qr_payment_address: "Scan the QR code of a payment address",
                scan_qr_store: "Scan the QR code of a store,\na listing, or a payment address"
            }, 
            SearchFilterHeader: {
                results: "%{total} results"
            }, 
            SelectableModerator: {
                view_details:"View Details"
            }, SelectorModal: {}, SendingAddressSelector: {}, SendingAmount: {}, ShippingMethod: {}, ShippingOptions: {}, ShippingPriceEditor: {}, SingleVariantEditor: {}, SupportHaven: {}, TagEditor: {}, TagSuggestion: {}, PanelView: { PanelViewBase: {} }
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
