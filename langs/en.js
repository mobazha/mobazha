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
                add_link: "Add",
                none_select: "none",
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
                unlimited: "Unlimited"
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
                claim: "Claim",

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
                see_all_reviews: "See all %{ratings.length} reviews",
                no_reviews_yet: "No reviews yet"
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
            SelectorModal: {

            },
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
                phone: "Phone",
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

            InfiniteProducts: {
                loading_listings: "Loading Listings..."
            },
            InventoryEditor: {
                details: "Details",
                surcharge: "Surcharge",
                sku: "SKU",
                sku_description: "SKU, ID, etc",
                quantity: "quantity",
                unlimited: "Unlimited",
                quantity_sold_out: 'If the quantity reaches 0, it will display as "sold out".'
            },
            InventoryList: {
                combos_info: "%{count} variant combos",
            },
            ListingAdvancedDetails: {
                Return_Policy: "Return Policy",
                Refunds: "Refunds",
                refunds_description: "What is your return policy? How long are returns accepted for? Who pays for return shipping?",
                terms: "Terms and Conditions",
                terms2: "T&Cs",
                terms_description: "What are the terms and conditions of the listing? What are you responsible for as the vendor? Is there a warranty?"
            },
            ListingsTab: {
                loading: "Loading...",
                no_sale: "There's nothing for sale at the moment.",
                check_later: "Check back again later!",
                store_empty: "Your store is empty",
                put_for_sale: "Put something up for sale!",
                create_listing: "Create listing",

            },
            NeedCoin: {
                coinbase: "Coinbase",
                cryptocurrency_exchange: "Cryptocurrency Exchange"
            },
            OrderState: {
                no_orders: "No orders found",
            },

            OrderSummary: {
                oops: "Oops!",
                dispute_pending_alert: "You can\'t start a dispute while the order is still pending.",
                dispute_not_fulfilled_alert: "You can\'t start a dispute until you\'ve fulfilled the order",
                dispute_cancel_alert: "You can\'t start a dispute for canceled order.",
                dispute_refund_alert: "You can\'t start a dispute for refunded order.",
                dispute_resolved_alert: "You can\'t start a dispute for dispute-closed order.",
                dispute_completed_alert: "You can\'t start a dispute for completed order.",
                dispute_finalized_alert: "This order can\'t be disputed. The seller has claimed payment for this order.",
                dispute_processing_alert: "This order can\'t be disputed. Please cancel your order to receive a full refund.",
                quantity_info: "Quantity: {quantity}",
                view: "View",
                view_transaction: "View transaction",
                payment: "Payment",
                no_payment: "A payment has not been found for this order yet. It may take up to a minute for the payment to be detected.",
                cannot_dispute: "The funds were sent directly to %{user}. You cannot dispute this order.",
                escrow_released: "The funds have already been released from escrow. The order can no longer be disputed.",
                order_in_dispute: "The order is in dispute for up to ",
                until_accept: " or until a party accepts a payout.",
                period_expired_claim: "The dispute period has expired. The seller can now claim the payment.",
                period_expired_claim2: "The dispute period has expired. The funds can now be claimed in full.",
                order_in_escrow1: "The order funds are being held in escrow for approximately ",
                order_in_escrow2: " or until the buyer completes the order.\n\nIf you have any issues with this order, you can open a dispute with the moderator.",
                dispute_order: "Dispute Order",
                claim_payment: "Claim Payment",
                dispute_error_possible: "An error occurred while processing this order.\nPlease start a dispute to recover your funds.",
                dispute_error: "An error occurred while processing this order.",
                order_refunded: "Order refunded",
                full_refund: "The seller has issued a full refund for this order",
                order_completed: "Order completed",
                release_to_seller: "The payment has been released to the seller",
                dispute_closed: "Dispute closed",
                dispute_closed_info: "%{user} has accepted the payout. This dispute is now closed.`",
                payment_claimed: "Payment claimed",
                seller_claim: "The seller has claimed payment for this order.",
                order_canceled: "Order canceled",
                user_cancel_order: "The %{user} has canceled this order. The money has been refunded in full.",
                period_expired: "Dispute period expired",
                no_dispute: "No dispute was opened during the 45-day dispute period. The seller can now claim payment.",
                Shipping: "Shipping",
                no_buyer_note: "No note left by buyer",
                address_copied: "Address copied!"
            },
            PurchaseState: {
                thank_you: "Thank you!",
                order_placed: "Your order has been placed. You can track or manage your order at any time.",
                processing: "Processing...",
                hang_tight: "Hang tight! This may take up to a minute.",
                Uh_oh: "Uh oh!",
                transaction_failed: "Your transaction failed to go through. Please try again.",
                retry: "Retry",
                order_details: "Order Details",
                error: "Error:"
            },
            RatingModal: {
                Overall: "Overall",
                Quality: "Quality",
                As_advertised: "As advertised",
                Delivery: "Delivery",
                Service: "Service",
                Write_a_review: "Write a review here",
                Post_anonymously: "Post anonymously"
            },
            ReportTemplate: {
                Ooops: "Ooops!",
                enter_reason: "Please enter a reason for reporting this content.",
                why_report_profile: "Why are you reporting this profile?",
                why_report: "Why are you reporting this?",
                next: "Next",
                submit: "Submit",
                describe_issue: "Please describe the issue (optional)",
                provide_details: "Provide as much details as possible"
            },
            SearchResults: {
                loading_results: "Loading Search Results ...",
                no_found: "No results found.",

            },
            SendMoney: {
                NEXT: "NEXT",
                send_to: "Send to",
                paste_or_scan: "Paste or scan address"
            },
            SendReceiveMoney: {
                Receive: "Receive",
                Send: "Send"
            },
            Settings: {
                are_you_sure: "Are you sure?",
                check_backup: "Have you backed up your current store?",
                cancel: "Cancel",
                OK: "OK",
                profile: "Profile",
                currency: "Currency",
                shipping_address: "Shipping address",
                blocked: "Blocked",
                notifications: "Notifications",
                push_notifications: "Push notifications",
                Policies: "Policies",
                Moderators: "Moderators",
                coins_accepted: "Coins accepted",
                Advanced: "Advanced",
                Analytics: "Analytics",
                On: "On",
                Off: "Off",
                Backup_wallet: "Backup wallet",
                Backup_profile: "Backup profile",
                Restore_profile: "Restore profile",
                Resync_transactions: "Resync transactions",
                Server_Log: "Server Log",
                Version: "Version 1.3.7"
            },
            StoreModeratorList: {
                moderators_count: "%{count} moderators",
                moderators_added: "New moderators are automatically added to your store"
            },
            Toast: {
                post_created: "Post created",
                view: "View"
            },
            TransactionHistory: {
                no_transaction_recorded: "No transactions have been recorded yet",
                no_transactions: "No transactions yet",
                notes: "Please note some payments may not display in the transaction history. However, the total balance reflects all sent and received transactions."
            },
            UserSearchResults: {
                loading_results: "Loading Search Results ...",
                no_results: "No results found."
            },
            wishlist: {
                wishlist_empty: "Your Wishlist is empty"
            }
        }
    },

    screens: {
        acceptedCoins: {
            update_listings: "Update Listings?",
            sure_about_update: "All your listings will be updated.\nAre you sure?",
            cancel: "Cancel",
            OK: "OK",
            coins_accepted: "Coins accepted",
            save: "Save",
            clear_all: "Clear all"
        },
        addShippingMethod: {
            fill_required: "Please fill out all the required fields",
            must_be_less: "Shipping option name length must be less than the max of 40",
            select_destination: "Please select a shipping destination",
            add_shipping_option: "Add Shipping Option",
            shipping_option: "Shipping option",
            title: "Title",
            option_description: "USA Shipping, International, etc",
            destinations: "Destinations"
        },
        analytics: {
            details1: 'Session information, such as how often you use the App and for how long.',
            details2: 'Basic device information; e.g., which type of phone you are using.',
            details3: 'The country you are accessing the App from.',
            details4: 'Which version of the App you are using.',
            details5: 'Which language you have selected.',
            details6: 'When you enter checkout for a purchase (no information is collected about what is being purchased).',
            details7: 'When you send funds, and which type of payment is used (no details are collected about the payment itself such as addresses or values).',
            details8: 'When you create a listing (no information about the listing itself is collected).',
            details9: 'Actions taken within Haven, such as tapping on the social feed or how often you make new posts. The content of the actions themselves are never recorded, only the fact that you took the action.',
            Analytics: "Analytics",
            Share_anonymous: "Share anonymous analytics",
            description: "If you opt into sharing analytics, you agree to share the following information with the OB1 Company:"
        },
        backupProfileInit: {
            back_up_profile: "Back up profile",
            ensure_backup1: "Ensure your data is safe by backing it up\nfrequently.",
            ensure_backup2: " For the time being, you\'re required to manually back up your data. ",
            ensure_backup3: "We\'ll be rolling out a better backup system in the future.",
            ensure_backup4: "Your backup will include all of your data, including wallet funds.",
            next: "NEXT"
        },
        backupProfilePassword: {
            password_empty: "Password empty",
            password_empty_hint: "Please set a password",
            password_mismatch: "Password mismatch",
            password_mismatch_hint: "Please set a correct password",
            take_a_minute: "It might take a minute...",
            set_password: "Set a password",
            password: "Password",
            confirm: "Confirm",
            confirm_password: "Confirm password",
            hint1: "Set a password and ",
            hint2: "make sure to write it down.",
            hint3: "\nYou\'ll need your password to restore your profile.",
            next: "NEXT"
        },
        backupProfileUpload: {
            upload_1: "Please upload your backup to a secure\nexternal location ",
            upload_2: " to ensure you can recover your\ndata if you lose your phone.",
            upload_backup: "UPLOAD BACKUP",
            done: "DONE"
        },
        blockedNodes: {
            no_block: "You haven’t blocked anyone yet"
        },
        categoryOverview: {
            see_all: "See all"
        },
        chats: {
            start_conversation: "Start a conversation",
            new_chat: "New Chat",
            no_discussion: "No order discussions found"
        },
        checkoutModerators: {
            select_moderator: "Select a moderator",
            no_available: "No Available Moderators"
        },
        editShippingAddress: {
            name_required: "Name is required",
            address_required: "Address is required",
            city_required: "City is required",
            country_required: "Country is required",
            new_address: "New Address",
            done: "Done",
            your_address: "Your Address",
            name: "Name",
            company: "Company",
            address: "Address",
            address2: "Address 2",
            city: "City",
            state: "State",
            postal_code: "Postal Code",
            country: "Country",
            delivery_notes: "Delivery Notes"
        },
        externalPay: {
            address_copied: "Address copied!",
            amount_copied: "Amount copied!",
            pay_order: "Pay to complete your order",
            copy_address: "Copy Address"
        },
        externalStore: {
            unblock_user: "Unblock this user to see their content",
            loading: "Loading...",
            failed_load: "Oops! This profile failed to load.",
            reported: "Reported",

        },
        followers: {
            followers: "Followers",
            no_followers1: "%{name} doesn't have any followers",
            no_followers2: "You don\'t have any followers"
        },
        followings: {
            Following: "Following",
            no_following1: "%{name} isn't following anyone",
            no_following2: "You are not following anyone"
        },
        listing: {
            are_you_sure: "Are you sure?",
            ask_block: "Block this user?",
            cancel: "Cancel",
            OK: "OK",
            ask_delete: "Delete listing?",
            delete_hint: "You can't undo this action.",
            cancel: "Cancel",
            remove: "Remove",
            failed_load: "Ooops! This listing failed to load.",
            retry: "Retry",
            loading: "Loading...",
            add_wishlist: "Added to Wishlist!",
            remove_wishlist: "Removed from Wishlist!",
            reported: "Reported!"
        },

        listingAdvancedOptions: {
            add_coupons: "Add coupons",
            advanced: "Advanced",
            Variants_Inventory: "Variants & Inventory",
            add_hint: "Add variants and manage your store inventory",
            store_policies: "Store Policies",
            policies_hint: "Add a return policy or terms of service",
            coupons: "Coupons"
        },
        Me: {
            support1: "Have questions, feature suggestions or bugs to report? Please check our FAQs first. Our Telegram group is a great resource to report bugs or ask for support. ",
            support2: "Our ability to offer email support is very limited. ",
            support3: " Please attempt to use the FAQ or Telegram group primarily.",
            description: "For any critical issues, concerns, or problems with the app and/or content in the marketplace, contact us via email."
        },
        moderatorDetails: {
            remove_moderator: "Remove moderator?",
            remove_hint: "This moderator will be removed from your store permanently. You won\'t be able to add them again",
            cancel: "Cancel",
            OK: "OK",
            verified: "verified",
            fee_description: "The fee only applies when a dispute is opened.",
            moderator_verified: " This moderator has been verified",
            terms: "Terms of Service",
            selected: " Selected",
            select: "SELECT"
        },
        newFeed: {
            Create_failed: "Create post failed",
            unknown_error_create: "Unknown error occured while creating post",
            char_left: " char left",
            what_going_on: "What's going on?"
        },
        onboarding: {
            HELLO: "HELLO!",
            restore_profile: "Restore profile",
            name: "Name",
            optional: "optional",
            country: "Country",
            currency: "Currency",
            share_analytics: "Share anonymous analytics",
            help_improve: "Help us improve Haven"
        },
        orderDetails: {
            decline_order: "Decline order?",
            decline_hint: "This order will be canceled and the money will be refunded to the buyer",
            nevermind: "Nevermind",
            ok: "Ok",
            refund_order: "Refund order?",
            refund_hint: "This order will be canceled and the money will be refunded to the buyer.",
            cancel_order: "Cancel order?",
            cancel_hint: "This order will be canceled and your money will be refunded in full.",
            have_refunded: "You have refunded the order",
            error_happened: "Error happened because of unknown issues",
            fund_order: "Fund Order",
            leave_notes: "Leave Notes",
            number_copied: "Order number copied!",
            learn_more: "Due to changes in the exchange rate, the current market price for an order may differ from the total price of the item at the time of purchase."
        },
        paymentMethod: {
            select_fee_level: "Please select fee level",
            not_accepted: "Not accepted",
            coming_soon: "Coming Soon",
            payment_method: "Payment Method",
            done: "Done",
            transaction_speed: "Transaction speed"
        },
        paymentSuccess: {
            transaction_details: "Transaction details",
            processing: "Processing…",
            hang_tight: "Hang tight! This may take up to a minute.",
            Uh_oh: "Uh oh!",
            failed_go_through: "Your transaction failed to go through. Please try again.",
            retry: "Retry",
            error: "Error:"
        },

        policies: {
            store_policies: "Store Policies",
            save: "Save",
            terms: "Terms and Conditions",
            terms_hint: "What are the terms and conditions of the listing? What are you responsible for as the vendor? Is there a warranty?",
            refunds: "Refunds",
            refund_hint: "What is your return policy? How long are returns accepted for? Who pays for return shipping?"
        },
        privacy: {
            privacy_policy: "Privacy Policy",
            terms: "Terms of Service",
            privacy: "PRIVACY",
            privacyDescription1: "Haven is built to give you far more privacy in your commerce, messaging, and payments than other apps.It uses several advanced technologies to keep your information from prying eyes, such as peer-to-peer networking and end-to-end encryption.",
            privacyDescription2: "There are ways to use Haven which improve or diminish your privacy. To learn more about how the underlying technology works, and what steps you can take to improve your privacy, tap the privacy policy link below.",
            privacyDescription3: "Before you proceed, you must accept the Haven https://gethaven.app/terms and https://gethaven.app/privacy.",
            cancel: "Cancel",
            I_accept: "I Accept"
        },
        ProductRatings: {
            reviews: "Reviews",
            No_reviews: "No reviews yet"
        },
        profileSettings: {
            warning: "Warning",
            warning_info: "If you go back, you will lose your progress",
            Cancel: "Cancel",
            OK: "OK",
            profile_information: "Profile Information",
            name: "Name",
            name_hint: "Satoshi Nakamoto",
            bio: "Bio",
            bio_hint: "Write a short description",
            location: "Location",
            location_hint: "e.g. Seattle",
            contact: "Contact",
            contact_hint: "satoshin@gmx.com",
            phone_number: "Phone Number",
            phone_hint: "+123456789",
            website: "Website",
            website_hint: "hello.com",
            Aaout: "About",
            about_hint: "Share more about yourself here"
        },
        purchaseSuccess: {
            successfully_sent: "Successfully sent message",
            received_message: "You have received a message!",
            sent: "Sent",
            close: "Close",
            order_complete: "Order Complete",
            view_transaction: "View Transaction",
            message_for: "Message for %{handle}",
            provide_details: "Provide additional details, ask a question, etc (optional)",
            send: "Send"
        },
        receiveMoney: {
            share_address: "Share Wallet Address",
            copy_address: "Copy Address",
            address_copied: "Address copied!"
        },
        restoreProfileInit: {
            restore_profile: "Restore profile",
            restore_hint: "Select your haven backup file to restore\nyour profile, including your wallet funds.",
            select_file: "SELECT FILE"
        },
        restoreProfilePassword: {
            Ooops: "Ooops!",
            loading_hint: "It might take a minute...",
            wrong_password: "Wrong password!",
            failed_download: "Failed to download zip file",
            enter_password: "Enter password",
            password: "Password",
            enter_password_hint: "Enter your password to proceed. You set this password when creating the backup.",
            restore: "RESTORE"
        },
        Resync: {
            unknown_error: "Unknown error!",
            resync_transactions: "Resync transactions",
            resync_content1: "If you believe you’re missing an order, or if your order details are out-of-sync with a buyer/seller, ",
            resync_content2: "you can rescan the blockchain for transactions related to your order.",
            resync_content3: "Resyncing transactions doesn’t need to be performed frequently. ",
            resync_content4: "It should only be done if you think there’s a problem. A scan is performed each time you start the app.",
            resync_content5: "You may leave this view while the resync process is active.",
            resyncing: "Resyncing...",
            resync_info: "Resynced %{lastSyncedAgo} ago",
            resync: "Resync"
        },
        searchFilter: {
            filter: "Filter",
            sortBy: "Sort by",
            accepts: "Accepts",
            ships_to: "Ships to",
            rating: "Rating",
            listing_type: "Listing type",
            item_condition: "Item Condition",
            adult_content: "Adult content",
            adult_content2: "Show adult content (18+)",
            filters_reset: "Filters reset"
        },
        shippingAddress: {
            are_you_sure: "Are you sure?",
            remove_address: "Remove the address",
            cancel: "Cancel",
            OK: "OK",
            free: "FREE",
            cannot_ship: "Sorry, this item can not be shipped to the selected address",
            Shipping: "Shipping",
            ships_to: "Ship To",
            no_address: "No shipping address",
            add_address: "+ Add new address"
        },
        StoreRatings: {
            Reviews: "Reviews",
            no_reviews1: "%{user} hasn't received any reviews",
            no_reviews2: "You haven\'t received any reviews"
        }
    },
}
