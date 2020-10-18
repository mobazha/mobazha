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
                unconfirmed: "%{balance} unconfirmed"
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
                moderator_takes: "Moderator takes ",
                seller_takes: "Seller takes ",
                buyer_takes: "Buyer takes ",
                accept_payout: "Accept payout",
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
                Overall: "Overall",
                Quality: "Quality",
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
                view_details: "View Details"
            },
            SelectorModal: {},
            SendingAddressSelector: {
                send: "SEND"
            },
            SendingAmount: {
                send_all: "Send all"
            },
            ShippingMethod: {
                free: "FREE"
            },
            ShippingOptions: {
                add_option: "Add a shipping option",
                options_count: "%{count} shipping options",
                shipping: "Shipping",
            },
            ShippingPriceEditor: {
                shipping_service: "Shipping service #${pos}",
                delete: "Delete",
                service: "Service",
                shipping_hint: "Standard, Express, etc.",
                Duration: "Duration",
                Duration_hint: "5-7 days",
                price: "Price",
                additional_price: "Addl. Price"
            },
            SingleVariantEditor: {
                variant_id: "Variant %{id}",
                title: "Title",
                title_hint: "e.g. Size",
                description: "Description",
                description_hint: "e.g. Size of Product",
                choices: "Choices",
                choices_hint: "e.g. Small, Medium, Large"
            },
            SupportHaven: {
                support_haven: "Support Haven",
                description: "Haven is completely free to use and relies on your support to help fund development."
            },
            TagEditor: {
                tags: "Tags",
                tags_info: "%{count} %{tag}",
                add_hint: "Add #tags to get your listing discovered"
            },
            TagSuggestion: {
                none: "None"
            },
            PanelView: {
                PanelViewBase: {
                    cancel: "Cancel"
                }
            }
        },

        templates: {
            AboutTab: {
                copy: "Copy",
                link_copied: "Store link copied!"
            },
            BackupWallet: {
                backup_wallet: "Backup wallet",
                backup_description1: "Backing up your wallet is the only way to restore your funds if you ever lose access to your device.",
                backup_description2: "All you’ll need is a paper and pen to jot down your recovery phrase. When you’re ready, continue to the next step.",
                next: "Next",
                recovery_phrase: "Your recovery phrase",
                phrase_hint: "Please write down each word in order",
                writedown_hint: "Keep your recovery phrase safe. If you ever lose or replace your phone, you can use it to regain access to your funds\n\nNever share your recovery phrase with anyone. To be safe, avoid screenshots and don’t store it on your mobile device.",
                done: "Done"
            },
            CategoryList: {
                more: "More"
            },
            ChatDetail: {
                is_typing: "%{peer} is typing...",
                unread: "UNREAD",
                block_message: "This user has been blocked.",
                start_with: "Start conversation with",
                moderator_join: "(moderator) joined this discussion",
                say_something: "Say something nice..."
            },
            Checkout: {
                address_required: "*Shipping address required for purchase",
                new_address: "New Address",
                cannot_ship: "This item can't be shipped to the selected address",
                shipping: "Shipping",
                payment_protection: "Payment Protection",
                protect_up_to: "Protect my payment for up to ",
                protect_days: "45 days",
                moderator_not_available: "Not available for this order",
                change_moderator: "Change moderator",
                no_moderator_description: "Your payment will be sent directly to the seller. If you experience problems with your order, you won't be able to open a dispute."
            },
            ContractModal: {
                view_contract: "View Contract",
                copied: "Copied!"
            },
            CouponApplyModal: {
                apply: "Apply",
                enter_coupon: "Enter a coupon code",
                enter_hint: "e.g. COUPON123",
                coupon: "Coupon ",
                not_valid: " is not valid."

            }, CouponModal: {
                title_empty_alert: "Discount title cannot be empty",
                code_empty_alert: "Discount code cannot be empty",
                percentage_alert: "Sorry, but the value must be between 1 and 99.",
                value_empty_alert: "Discount value cannot be empty",
                exceed_alert: "Sorry, but the discount exceeds the value of the item.",
                edit_coupon: "Edit Coupon",
                new_coupon: "New Coupon",
                title: "Title",
                title_hint: "Enter a title",
                code: "Code",
                code_hint: "Enter a coupon code",
                discount: "Discount",
                discount_hint1: "e.g. 10%",
                discount_hint2: "e.g. $10",
                percent: "Percent"
            },
            CovidModal: {
                description11: "People, states and hospitals all around the world are running very low on essential supplies to stay safe in these difficult times. If you, or anyone you know, can quickly produce, source or dropship",
                description12: "face masks, N95 masks, surgical masks, hand sanitizer, hand soaps, ventilators, thermometers, wet wipes, toilet paper",
                description13: ", etc. Please get these items into circulation and into the right hands as soon as possible.",
                description21: "Lives can be saved if we all do our part. The world needs your support to help get essential items into circulation. If you have access to a",
                description22: " sewing machine, 3D printer",
                description23: ", or even a",
                description24: " distillery",
                description25: ", please consider creating items and parts that can be donated and/or sold at a fair price.",
                description31: "Your essential items can be immediately distributed on Haven without fees. No account needed. No questions asked. Please do your part to help."
            },
            DisputeModal: {
                submit_dispute: "Submit dispute?",
                submit_hint: "The moderator will step in to help resolve the dispute. You can\'t undo this action",
                cancel: "Cancel",
                ok: "Ok",
                enter_reason: "Please enter dispute reason!",
                content_hint: "Why are you starting a dispute? Provide as much detail as possible."
            },
            EULAModal: {
                eula: "EULA",
                privacy_description3: "End User License Agreement terms and conditions governing download and use of this mobile application, downloaded by you via Apple, Inc.’s (“Apple”) App Store (the “App Store”) or Google Play. Please read this End User License Agreement terms and conditions carefully.",
                privacy_description2: "This End User License Agreement sets forth the terms and conditions (“Terms”) under which OB1 (“OB1”) (alternatively referred to as “us,” “we,” or “our”) offers you the right to download and use the Haven mobile application (including any updates thereto, the “Application”) and your use of the Application is governed by these Terms. By accepting these Terms (i) you represent that you are of legal age to enter into a binding contract and (ii) you signify that you have read, understood and agree to these Terms (and that such Terms are enforceable like any other written negotiated agreement signed by you) and certify that you are at least 17 years old or older. If you do not agree to these Terms, or you are not at least 17 years old, you may not use the Application. Violations of these Terms will result in a permanent removal from the Application.",
                privacy_description3: "These Terms constitute an agreement strictly between OB1 and you and you acknowledge that OB1 (in accordance with the limitations herein) rather than Apple and Google is responsible for any claim or liability arising from your use of the Application including, but not limited to, any third party claim of infringement of intellectual property rights. Nevertheless, you agree to abide by all terms, conditions or usage rules imposed by Apple and Google applicable to the use of this Application, including, but not limited to, any terms, conditions or usage rules set forth in the App Store Terms of Service.",
                privacy_description4: "1. License and Restrictions",
                privacy_description5: "The Application is licensed, not sold, to you. All rights, title and interest (including, without limitation, all copyrights, trademarks and other intellectual property rights) in and to this Application belong to us or our licensors. Subject to your compliance with these Terms, we grant you a non-transferable, non-assignable, revocable, limited license to download and install one copy of this Application on a mobile device that you personally own or control and to use that copy of this Application on that mobile device solely for your own personal use. You may not install or use a copy of the Application on a device you do not own or control. You may not distribute or make the Application available over a network where it could be used by multiple devices at the same time. You may not sell, rent, lend, lease, redistribute, or sublicense the Application or circumvent any technical limitations in the Application or otherwise interfere in any manner with the operation of the Application, or the hardware or network used to operate the Application. You may not copy, reverse engineer, decompile, disassemble, modify, create derivative works or otherwise attempt to derive the source code of this Application. This Application and its content are protected by copyright under both United States and foreign laws. Any use of the Application and its content not explicitly permitted by these Terms is a breach of this agreement and may violate the law. If you violate these Terms, your license to use this Application automatically terminates and you must immediately cease using the Application and destroy all copies, full or partial, of the Application.",
                privacy_description6: "2. Ownership",
                privacy_description7: "We alone (and our licensors, where applicable) shall own all right, title and interest, including, without limitation, all intellectual property rights, in and to the Application and any suggestions, ideas, enhancement requests, feedback, recommendations or other information provided by you or any other party relating to the Application. Any copy, modification, revision, enhancement, adaptation, translation, or derivative work of or created from the Application shall be owned solely and exclusively by us, and/or, as applicable, our third-party vendors, as shall any and all patent rights, copyrights, trade secret rights, trademark rights, and all other proprietary rights, worldwide therein and thereto, and you hereby assign to OB1 any and all of your rights, title or interests that you may have or obtain in the Application or any modification to or derivative work of the Application. You shall not remove or authorize or permit any third party to remove any proprietary rights legend from the Application.",

            }, feed: {
                not_post: "%{name} \nhasn\'t posted anything yet.",
                post_hint1: "You haven’t posted anything yet.",
                post_hint2: "Share something with the community!",
                reported: "Reported"
            },
            FeedDetail: {
                fail_to_load: "Ooops! This post failed to load.",
                retry: "Retry",
                Loading: "Loading...",
                reported: "Reported"
            },
            FulfillModal: {
                shipping_carrier: "Shipping Carrier",
                carrier_hint: "USPS, FedEX, etc",
                tracking_number: "Tracking No.",
                tracking_number_hint: "Tracking number",
                file_url: "File URL",
                password: "Password",
                password_hint: "Optional",
                note: "Note",
                note_hint: "Optional",
                add_a_note: "Add a note (optional)"
            },
            GlobalFeed: {
                customise_feed: "Follow some profiles to customise your feed!",
                not_found: "No results found",
                share_with_community: "Share something with the community!",
                create_post: "Create post",
                reported: "Reported"
            },

            InfiniteProducts: {}, InventoryEditor: {}, InventoryList: {}, ListingAdvancedDetails: {}, ListingsTab: {}, NeedCoin: {}, OrderState: {},

            OrderSummary: {}, PurchaseState: {}, RatingModal: {}, ReportTemplate: {}, SearchResults: {}, SendMoney: {}, SendReceiveMoney: {}, Settings: {}

            , StoreModeratorList: {}, Toast: {}, TransactionHistory: {}, UserSearchResults: {}, wishlist: {}
        }
    },

    screens: {
        acceptedCoins: {}, addShippingMethod: {}, analytics: {}, backupProfileInit: {}, backupProfilePassword: {}, backupProfileUpload: {}, categoryOverview: {}, chats: {}, checkoutModerators: {}, editShippingAddress: {}, externalPay: {}, externalStore: {}, followers: {}, followings: {}, listing: {}, listingAdvancedOptions: {}, Me: {}, moderatorDetails: {}, onboarding: {}, orderDetails: {}, paymentMethod: {}, paymentSuccess: {}, policies: {}, privacy: {}, ProductRatings: {}, profileSettings: {}, purchaseSuccess: {}, receiveMoney: {}, restoreProfileInit: {}, restoreProfilePassword: {}, Resync: {}, searchFilter: {}, shippingAddress: {}, StoreRatings: {}
    },
    utils: { ratings: {} },
}
