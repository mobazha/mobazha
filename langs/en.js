export default {
    AppNavigator: {
        Please_wait: "Please wait..."
    },
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
                quantity:"Quantity",
                quantity_info: "Quantity: %{quantity}",
                coupon_info: "Coupon: %{info}",
                change: "Change",
                free: "FREE",
                network_fee: "Network Fee",
                fee_alert_description: "Fee is too high. Please use a lower fee level or a different coin.",
                learn_more: "Learn more",
                summary:"Summary",
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
                current_price: " Current market price, tap to",
                learn_more: "learn more"
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
            PanelView: {
                PanelViewBase: {
                    cancel: "Cancel"
                },
                PlusPanelView: {
                    Sell: "Sell",
                    Post: "Post",
                    Chat: "Chat",
                    Pay: "Pay",
                    Choose_action: "Choose action"
                },
                PanelViewBase: {
                    cancel: "Cancel"
                }
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
            SearchHeader: {
                search: "Search..."
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
            CategoryProductGrid: {
                see_all: "SEE ALL"
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
                input_groupon_title: "Essential Supplies Needed (COVID-19)",
                description11: "People, states and hospitals all around the world are running very low on essential supplies to stay safe in these difficult times. If you, or anyone you know, can quickly produce, source or dropship",
                description12: "face masks, N95 masks, surgical masks, hand sanitizer, hand soaps, ventilators, thermometers, wet wipes, toilet paper",
                description13: ", etc. Please get these items into circulation and into the right hands as soon as possible.",
                description21: "Lives can be saved if we all do our part. The world needs your support to help get essential items into circulation. If you have access to a",
                description22: " sewing machine, 3D printer",
                description23: ", or even a",
                description24: " distillery",
                description25: ", please consider creating items and parts that can be donated and/or sold at a fair price.",
                description31: "Your essential items can be immediately distributed on Haven without fees. No account needed. No questions asked. Please do your part to help.",
                create_listing_title: "Create Listing"
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
                privacy_description1: "End User License Agreement terms and conditions governing download and use of this mobile application, downloaded by you via Apple, Inc.’s (“Apple”) App Store (the “App Store”) or Google Play. Please read this End User License Agreement terms and conditions carefully.",
                privacy_description2: "This End User License Agreement sets forth the terms and conditions (“Terms”) under which OB1 (“OB1”) (alternatively referred to as “us,” “we,” or “our”) offers you the right to download and use the Haven mobile application (including any updates thereto, the “Application”) and your use of the Application is governed by these Terms. By accepting these Terms (i) you represent that you are of legal age to enter into a binding contract and (ii) you signify that you have read, understood and agree to these Terms (and that such Terms are enforceable like any other written negotiated agreement signed by you) and certify that you are at least 17 years old or older. If you do not agree to these Terms, or you are not at least 17 years old, you may not use the Application. Violations of these Terms will result in a permanent removal from the Application.",
                privacy_description3: "These Terms constitute an agreement strictly between OB1 and you and you acknowledge that OB1 (in accordance with the limitations herein) rather than Apple and Google is responsible for any claim or liability arising from your use of the Application including, but not limited to, any third party claim of infringement of intellectual property rights. Nevertheless, you agree to abide by all terms, conditions or usage rules imposed by Apple and Google applicable to the use of this Application, including, but not limited to, any terms, conditions or usage rules set forth in the App Store Terms of Service.",
                privacy_description4: "1. License and Restrictions",
                privacy_description5: "The Application is licensed, not sold, to you. All rights, title and interest (including, without limitation, all copyrights, trademarks and other intellectual property rights) in and to this Application belong to us or our licensors. Subject to your compliance with these Terms, we grant you a non-transferable, non-assignable, revocable, limited license to download and install one copy of this Application on a mobile device that you personally own or control and to use that copy of this Application on that mobile device solely for your own personal use. You may not install or use a copy of the Application on a device you do not own or control. You may not distribute or make the Application available over a network where it could be used by multiple devices at the same time. You may not sell, rent, lend, lease, redistribute, or sublicense the Application or circumvent any technical limitations in the Application or otherwise interfere in any manner with the operation of the Application, or the hardware or network used to operate the Application. You may not copy, reverse engineer, decompile, disassemble, modify, create derivative works or otherwise attempt to derive the source code of this Application. This Application and its content are protected by copyright under both United States and foreign laws. Any use of the Application and its content not explicitly permitted by these Terms is a breach of this agreement and may violate the law. If you violate these Terms, your license to use this Application automatically terminates and you must immediately cease using the Application and destroy all copies, full or partial, of the Application.",
                privacy_description6: "2. Ownership",
                privacy_description7: "We alone (and our licensors, where applicable) shall own all right, title and interest, including, without limitation, all intellectual property rights, in and to the Application and any suggestions, ideas, enhancement requests, feedback, recommendations or other information provided by you or any other party relating to the Application. Any copy, modification, revision, enhancement, adaptation, translation, or derivative work of or created from the Application shall be owned solely and exclusively by us, and/or, as applicable, our third-party vendors, as shall any and all patent rights, copyrights, trade secret rights, trademark rights, and all other proprietary rights, worldwide therein and thereto, and you hereby assign to OB1 any and all of your rights, title or interests that you may have or obtain in the Application or any modification to or derivative work of the Application. You shall not remove or authorize or permit any third party to remove any proprietary rights legend from the Application.",
                privacy_description8: "3. Your Responsibilities as the Application User",
                privacy_description9: "Use of the Application requires third party services and equipment such as a compatible mobile device, internet access and a telecommunications carrier. Obtaining and maintaining the equipment and services necessary to use the Application is your responsibility. OB1 is not responsible for equipment defects, lack of service, dropped calls, or other issues arising from third party services or equipment. You are solely responsible for your use of those services on your mobile device and compliance with any applicable third party terms and payment of all applicable third party fees associated with any carrier service plan you use in connection with your use of those services (such as voice, data, SMS, MMS, roaming or other applicable fees charged by the carrier). You agree not to use the Application to communicate in an offensive or obscene manner, or to spam, threaten, defame or harass other users. OB1 is not in any way responsible for any such use by you or by any person using your device, nor for any harassing, threatening, defamatory, offensive, or illegal messages or transmissions that you may receive as a result of using the Application. OB1 reserves the right, but does not assume the obligation, to remove any objectionable activity or language used in the Application at any time. OB1 reserves the right, but does not assume the obligation, to not publish or to terminate any communication, or posting it determines objectionable in its sole discretion. Use of the Application is void where prohibited. You shall not use the Application to falsely state or otherwise misrepresent yourself or your affiliation with any person or entity; or to intentionally or unintentionally violate any applicable local, state, national or international law, including, but not limited to, U.S. regulations pertaining to the export of software from the U.S. to embargoed countries. You will ensure that the information you provide to us through the Application is accurate and complete. We reserve the right to immediately terminate your use of the Application should you fail to comply with any of the foregoing.",
                privacy_description10: "4. Third Party Sites, Services and Devices",
                privacy_description11: "The Application may enable you to access third-party mobile applications and websites (“Third Party Materials”). Access to Third Party Materials may require you to accept additional terms and conditions and privacy policies. You acknowledge that OB1 is not responsible for the terms and conditions or privacy policies of Third Party Materials. You understand that by using any of the Third Party Materials you may encounter content that may be deemed offensive, indecent, or objectionable, which content may or may not be identified as having explicit language, and that the results of any search or entering of a particular URL may automatically and unintentionally generate links or references to objectionable material. Nevertheless, you agree to use the Third Party Materials at your sole risk and that neither OB1 nor its agents shall have any liability to you for content that may be found to be offensive, indecent, or objectionable.\
                Certain Third Party applications or materials may provide links to additional third party websites or allow you to upload or enter your own data. By using the Third Party Materials, you acknowledge and agree that neither OB1 nor its agents is responsible for the content, accuracy, completeness, timeliness, validity, copyright compliance, legality, decency, quality or any other aspect of such Third Party Materials, or the data you choose to upload or enter into the Application through those Third Party Materials. Neither OB1 nor its agents warrant or endorse, and each does not assume and will not have any liability or responsibility to you or any other person for, any Third Party Materials. Links to Third Party Materials are provided solely as a convenience to you.",
                privacy_description12: "5. User Submissions",
                privacy_description13: "Any information submitted through the Application, including listings, posts, messages, may be provided to our staff and may be viewable to other Application users. OB1 is not responsible for the content of any communication submitted or posted by Application users nor do we guarantee the truthfulness, accuracy or validity of any posted communication. Any action you take or do not take based upon information posted to the Application, including, but not limited to, investment, purchasing, trading, employment or other decisions, is done at your own risk.\
                By submitting communications or content to any part of this Application that is viewable by other Application users, you acknowledge that the submission may be viewed and further disclosed by other Application users. We encourage you to not include personally identifiable information in such submissions and cannot be held liable for the further disclosure of your personally identifiable information by other Application users. You acknowledge that OB1 only acts as a passive conduit for the distribution of content and other material posted by Application users and is not responsible or liable to you or any third party for the content or accuracy of those materials. We, however, reserve the right, but assume no obligation, to monitor any submissions or postings and delete, move or edit any content that we consider inappropriate or unacceptable for any reason. You shall not submit any communication or content that infringes or violates any right of any party or that is not original to you. Illicit or abusive content is strictly prohibited. Where we do moderate interactive features, we will endeavor to review comments and postings for relevance, topicality and appropriateness, and we may withhold or remove postings for any reason, within our sole discretion. We are unlikely to post comments relating to ongoing legal matters or regulatory issues.\
                We reserve the right to republish and use any material contributed by Application users as permitted by these Terms or otherwise by law. By posting a message, content or other material in any public area of the Application or submitting any correspondence to us, you expressly grant us, and anyone authorized by us, a global, royaltyfree, perpetual, irrevocable, unrestricted, nonexclusive license to publish, reproduce, sell, disclose, modify, create derivative works from, distribute, publicly perform or display, or otherwise use such material, in whole or in part, in any manner or medium (whether now known or hereafter developed), for any purpose whatsoever. You hereby further grant us, and anyone authorized by us, the global, royalty-free, perpetual, irrevocable, unrestricted, nonexclusive right to use any ideas, concepts or techniques, in whole or in part, in any manner or medium (whether now known or hereafter developed), embodied in such materials for any purpose whatsoever. In addition, you hereby waive any and all moral rights you may have in any such materials. You also agree that all such material will be deemed to be provided to us on a non-confidential and non-proprietary basis. Material that is copyright protected may not be submitted without permission from the copyright owner, and you are solely responsible for the failure to obtain any such permission.\
                We will comply with any legal requests to disclose any submissions, communications or postings to others, including to law enforcement agencies.",
                privacy_description14: "6. Privacy Statement",
                privacy_description15: "Your use of the Application is also subject to the terms and conditions of the Mobile Application Privacy Policy.",
                privacy_description16: "7. Legal Compliance",
                privacy_description17: "The Application is subject to United States export laws and regulations. You will not use or otherwise export the Application except as authorized by United States law and the laws of the jurisdiction in which the Application was obtained. You represent and warrant that (i) you are not located in a country that is subject to a U.S. Government embargo, or that has been designated by the U.S. Government as a “terrorist supporting” country; and (ii) you are not listed in any U.S. Government list of prohibited or restricted parties. OB1 does not represent that the Application is appropriate or available for use in all countries. OB1 prohibits accessing materials from countries or states where such content is illegal. You are using the Application on your own initiative and you are responsible for compliance with all applicable laws.",
                privacy_description18: "8. Disclaimer of Warranty",
                privacy_description19: "Any use of the Application shall be at your sole risk. This Application and the information you access through the Application is provided on an 'AS IS', 'WITH ALL FAULTS' and 'AS AVAILABLE' basis and without any warranty, express or implied, of any kind, to the fullest extent permissible pursuant to applicable law. OB1, Apple, Google, wireless carriers over whose network the Application is distributed, and each of our respective affiliates and suppliers (collectively, “Distributors”) give no express or implied warranties, guarantees, or conditions under or in relation to the Application. Distributors disclaim all express or implied warranties related to the Application including, but not limited to, implied warranties for merchantability, non-infringement, and fitness for a particular purpose. Distributors make no warranty as to the reliability, accuracy, timeliness, usefulness or completeness of the Application or any information accessed through the Application. Distributors cannot and do not warrant against human, services and machine errors, omissions, delays, failures, interruptions or losses. Distributors cannot and do not guarantee or warrant that the Application will be free of infection or viruses, worms, malware, Trojan Horses or other malicious codes. OB1 reserves the right to terminate, without notice, your use of the Application at any time and for any reason. Please note that some jurisdictions may not allow the exclusion of implied warranties, so some of the above exclusions may not apply to you. In such case, exclusions will apply to the greatest extent consistent with applicable law. You are solely responsible for any damages to your hardware device(s) or loss of data that results from the download or use of the Application. Your sole and exclusive remedy for dissatisfaction with the Application is to stop using it.",
                privacy_description20: "9. Limitation of Liability",
                privacy_description21: "Under no circumstances will Distributors be liable for any damages you suffer as a result of your reliance on this Application or any content provided by the Application or Third Party Materials, nor will Distributors be liable to you or any third party for any incidental, special, consequential, indirect or punitive damages whatsoever, including, without limitation, loss of profits, loss of data, business interruption or any other personal injury or commercial damages or losses arising out of or that result from the use of, or the inability to use, the Application, regardless of the theory of liability (contract, tort, strict liability, negligence, guarantee or condition, or otherwise), even if advised of the possibility of such damages or repair or replacement of the Application does not fully compensate you for any losses. In no event shall Distributor's total liability to you for all damages (other than as may be required by applicable law in cases involving personal injury) exceed the amount of One Hundred ($100) Dollars. The foregoing limitations will apply even if the above stated remedy fails of its essential purpose.",
                privacy_description22: "10. Maintenance and Support Services",
                privacy_description23: "Any maintenance and support services made available by OB1 are at the discretion of OB1 which may initiate or cease providing maintenance and support services at any time without notice to you. You acknowledge that Apple, Google, and your wireless carrier are not responsible for providing maintenance and support services for the Application.",
                privacy_description24: "11. Location Data",
                privacy_description25: "OB1, Apple, Google, Distributors or other providers or their partners may collect, maintain, process and use your location data, including the real-time geographic location of your mobile device as necessary to provide the Application’s full functionality. By using or activating any location-based services on your mobile device, you agree and consent to OB1's, and such parties' collection, maintenance, publishing, processing and use of your location data to provide you with such services. You may withdraw this consent at any time by turning off the location-based feature on your mobile device or by not using any location-based features. Turning off or not using these features may impact the functionality of the Application. Location data provided by the Application is for basic navigational purposes only and is not intended to be relied upon in situations where precise location information is needed or where erroneous, inaccurate or incomplete location data may lead to death, personal injury, property or environmental damage. Use of real time route guidance is at your sole risk. Location data may not be accurate. Neither OB1, nor such parties guarantee the availability, accuracy, completeness, reliability or timeliness of information or location displayed by the Application.",
                privacy_description26: "12. Choice of Laws, Jurisdiction, Entire Agreement",
                privacy_description27: "By downloading or using the Application, you expressly agree that these Terms shall be governed by and construed in accordance with the laws of the State of Delaware, without giving effect to its conflict of laws provisions or your actual state or country of residence. You further expressly agree that exclusive jurisdiction for any dispute with OB1 in any way relating to your use of this Application is in the federal or district courts of the State of Delaware, and you agree and expressly consent to the exercise of personal jurisdiction in state or federal court in the State of Delaware, in connection with any such dispute including any claim involving OB1 or its affiliates or content providers. If any provision of these Terms shall be unlawful, void, or for any reason unenforceable, then that provision shall be deemed severable from these Terms and shall not affect the validity and enforceability of any remaining provisions. This is the entire agreement between the parties relating to the subject matter herein and it supersedes all previous or contemporaneous agreements, proposals and communications, written or oral, relating to that subject matter. As a user of the Application, you agree to contact us prior to seeking legal recourse for any harm you believe you have suffered from your use of the Application. In the event that you believe our Application has harmed you, you agree to inform us and to give us thirty (30) days to cure the harm before initiating any action. You also agree that you must initiate any cause of action within one (1) year after the claim has arisen, or you will be barred from pursuing any cause of action.",
                privacy_description28: "13. Indemnity",
                privacy_description29: "You will defend, indemnify and hold OB1, its officers, directors, employees, agents, licensors, and vendors, harmless from and against any and all claims, actions or demands, liabilities and settlements including without limitation, reasonable legal and accounting fees, resulting from, or alleged to result from, (i) your violation of these Terms, whether by act, omission or negligence, or by any other person using your account, (ii) your use of the Application, (iii) your violation of any rights of another, and/or (iv) any communications, content or other material posted to or transmitted through the Application by you or by others using your account.",
                privacy_description30: "14. Third Party Beneficiary",
                privacy_description31: "OB1 and you acknowledge that Apple, Apple’s subsidiaries, Google, Google’s subsidiaries are third party beneficiaries to this agreement. Upon your acceptance of these Terms, Apple and Google will have the right (and will be deemed to have accepted the right) to enforce these Terms against you as a third party beneficiary. Aside from Apple and Google, there are no third party beneficiaries to this agreement.",
                privacy_description32: "15. Amendment",
                privacy_description33: "We have the right, at any time and without prior written notice, to add to or modify the Terms, by amending the Terms available within the Home page or by requiring you to accept an updated agreement upon accessing the Application. Your access or use of the Application after the date of such amended Terms constitutes acceptance of such amended Terms. By continuing to access or use the Application after we post such changes, you agree to these Terms, as modified.",
                privacy_description34: "16. Contact Us",
                privacy_description35: "For Questions, please email us at haven@ob1.io",
                privacy_description36: "17. Copyright Infringement – DMCA Notice",
                privacy_description36: "The Digital Millennium Copyright Act of 1998 (the “DMCA”) provides recourse for copyright owners who believe that material appearing on the Internet infringes their rights under US copyright law. If you believe in good faith that content or material on this Application infringes a copyright owned by you, you (or your agent) may send OB1 a notice requesting that the material be removed, or access to it blocked. This request should be sent to: haven@ob1.io. The notice must include the following information: (a) a physical or electronic signature of a person authorized to act on behalf of the owner of an exclusive right that is allegedly infringed; (b) identification of the copyrighted work claimed to have been infringed; (c) identification of the material that is claimed to be infringing or the subject of infringing activity; (d) the name, address, telephone number, and email address of the complaining party; (e) a statement that the complaining party has a good faith belief that use of the material in the manner complained of is not authorized by the copyright owner, its agent or the law; and (f) a statement that the information in the notification is accurate and, under penalty of perjury, that the complaining party is authorized to act on behalf of the owner of an exclusive right that is allegedly infringed. If you believe in good faith that a notice of copyright infringement has been wrongly filed against you, the DMCA permits you to send us a counter-notice. Notices and counter-notices must meet the then-current statutory requirements imposed by the DMCA. Notices and counternotices with respect to the Application should be sent to the address above.",
                iaccept: "I Accept"
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
                Trending: "Trending",
                Most_Recent: "Most Recent",
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
                total: "Total",
                sku: "SKU",
                sku_description: "SKU, ID, etc",
                quantity: "quantity",
                unlimited: "Unlimited",
                quantity_sold_out: 'If the quantity reaches 0, it will display as "sold out".'
            },
            InventoryList: {
                combos_info: "%{count} variant combos"
            },
            ListingAdvancedDetails: {
                Return_Policy: "Return Policy",
                Refunds: "Refunds",
                refunds_description: "What is your return policy? How long are returns accepted for? Who pays for return shipping?",
                terms: "Terms and Conditions",
                terms2: "T&Cs",
                terms_description: "What are the terms and conditions of the listing? What are you responsible for as the vendor? Is there a warranty?"
            },
            ListingCustomOptions:{
                variant:"variant",
               add_variant:"Add variant",
               track_Inventory:"Track Inventory",
               inventory:"Inventory",
               ListEmptyComponent:  "Add sizes, colours, materials, etc."
            },
            ListingsTab: {
                loading: "Loading...",
                no_sale: "There's nothing for sale at the moment.",
                check_later: "Check back again later!",
                store_empty: "Your store is empty",
                put_for_sale: "Put something up for sale!",
                create_listing: "Create listing"
            },
            NeedCoin: {
                coinbase: "Coinbase",
                cryptocurrency_exchange: "Cryptocurrency Exchange"
            },
            Notification: {
                Social: "Social",
                Orders: "Orders",
                social_empty: "If someone follows you or interacts with your posts, you’ll see it here.",
                order_empty: "Stay tuned. Updates on your orders will show up here."
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
                total: "Total",
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
                done:"Done",
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
                Country: "Country",
                currency: "Currency",
                shipping_address: "Shipping address",
                blocked: "Blocked",
                notifications: "Notifications",
                push_notifications: "Push notifications",
                store: "store",
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
            SocialPostTemplate:{
                RePostTemplate:{
                    delete_repost:"Delete repost?",
                    delete_feed:"Deleting your repost will remove it from your feed",
                    cancel:"Cancel",
                    delete:"Delete",
                    repost:"Repost",
                    repost_with_comment:"Repost with comment",
                }
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
                Transactions: "Transactions",
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

    config: {
        categories: {
            Books: "Books",
            Electronics: "Electronics",
            Games: "Games",
            Clothing: "Clothing",
            Apparel_for_Men: "Apparel for Men",
            Cellphones_Telecommunications: "Cellphones & Telecommunications",
            Computer_Office: "Computer & Office",
            Jewelry_Accessories: "Jewelry & Accessories",
            Home_Garden: "Home & Garden",
            Luggage_Bags: "Luggage & Bags",
            Shoes: "Shoes",
            Mother_Kids: "Mother & Kids",
            Sports_Entertainment: "Sports & Entertainment",
            Beauty_Health: "Beauty & Health",
            Watches: "Watches",
            Automobiles_Motorcycles: "Automobiles & Motorcycles",
            Lights_Lighting: "Lights & Lighting",
            Furniture: "Furniture",
            Electronic_Components_Supplies: "Electronic Components & Supplies"
        },
        productTypes: {
            Any: "Any",
            Physical_Good: "Physical Good",
            Digital_Good: "Digital Good",
            Service: "Service"
        }
    },

    screens: {
        acceptedCoins: {
            update_listings: "Update Listings?",
            sure_about_update: "All your listings will be updated. Are you sure?",
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
            destinations: "Destinations",
            save: "Save",
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
            ensure_backup1: "Ensure your data is safe by backing it upfrequently.",
            ensure_backup2: " For the time being, you re required to manually back up your data. ",
            ensure_backup3: "We'll be rolling out a better backup system in the future.",
            ensure_backup4: "Your backup will include all of your data, including wallet funds.",
            next: "NEXT"
        },
        backupProfilePassword: {
            password_empty: "Password empty",
            password_empty_hint: "Please set a password",
            password_mismatch: "Password mismatch",
            password_mismatch_hint: "Please set a correct password",
            take_a_minute: "It might take a minute...",
            backup_done:"Backup done",
            backup_failed:"Backup failed" ,
            backupProfileUpload:"BackupProfileUpload",
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
            message:  "Here is the backup file!",
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
            no_discussion: "No order discussions found" ,
            chat:"Chat"
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
            unblock:"Unblock",
            large:"Large",
            loading: "Loading...",
            failed_load: "Oops! This profile failed to load.",
            retry:"Retry",
            reported: "Reported",

        },
        feed: {
            My_Feed: "My Feed",
            Global: "Global",
            New_features: "New features",
            feature_description: "Social has improved. Personalized feeds, in-app notifications, and more!",
            Social: "Social"
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
            policy1: "Return policy",
            policy2:"Terms and conditions",
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
        newChat: {
            Search: "Search...",
            Search_user: "Search for a user"
        },
        newFeed: {
            Create_failed: "Create post failed",
            unknown_error_create: "Unknown error occured while creating post",
            char_left: " char left",
            what_going_on: "What's going on?"
        },
        notifications: {
            Notifications: "Notifications"
        },
        NotificationSettings:{
            notification_preferences:"Notification preferences",
            all1:"All",
            Receive_all:"Receive all push notifications", 
            all2:"All",
            featured_content: "Featured content",
            notify1: "Notify me of deals, discounts, and other cool content on Haven",
            promotions: "promotions",
            giveaways1:"Giveaways",
            Notify2:"Notify me of giveaways and other promotional events on Haven",
            giveaways2:"giveaways",
            announcements1: "Announcements",
            notify3:'Notify me of new features, updates, and other app-related announcements',
            announcements2: "announcements",
            chat1: "Chat",
            notify4:'Notify me when I receive a chat message.',
            chat2: "chat",
            likes1: "Likes",
            notify5:"Notify me when someone likes my post.",
            likes2: "likes",
            comments1: "Comments",
            notify6:"Notify me when someone comments on my post.",
            comments2:"comments"
        },
        onboarding: {
            HELLO: "HELLO!",
            restore_profile: "Restore profile",
            name: "Name",
            optional: "optional",
            country: "Country",
            currency: "Currency",
            code:"Code",
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
            order_discussions:"No order discussions found",
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
            email:"Email",
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
        searchResult: {
            Listings: "Listings",
            User: "User"
        },
        Server_Logs:{
            server_Logs:"Server Logs",
            details1:"Your server logs contain information that can help troubleshoot issues and/or bugs you may be experiencing.",
            details2:"Tapping the buttons below will prompt you to share your logs. Please only share your logs with people you trust. Avoid posting your logs publicly; they contain sensitive information.",
            share_Server_Log:"Share Server Log",
            share_ifpS_Log:"Share IPFS Log"
        },
        shippingAddress: {
            are_you_sure: "Are you sure?",
            remove_address: "Remove the address",
            cancel: "Cancel",
            OK: "OK",
            free: "FREE",
            cannot_ship: "Sorry, this item can not be shipped to the selected address",
            Shipping: "Shipping",
            done:"Done",
            ships_to: "Ship To",
            no_address: "No shipping address",
            add_address: "+ Add new address"
        },
        shop: {
            Trending: "Trending",
            Featured_stores: "Featured stores",
            Featured_listings: "Featured listings",
            Best_Sellers: "Best Sellers",
            Gaming: "Gaming",
            Munchies: "Munchies",
            Devices: "Devices"
        },
        StoreRatings: {
            Reviews: "Reviews",
            no_reviews1: "%{user} hasn't received any reviews",
            no_reviews2: "You haven\'t received any reviews"
        },
        wallet: {
            Wallet: "Wallet",
            View_history: "View transaction history",
            Cancel: "Cancel"
        },
        wishlist: {
            Wishlist: "Wishlist"
        },
        Me:{
            my_Profile :"My Profile",
            screenName1 :"Store",
            wallet: "Wallet",
            screenName2:"Wallet",
            purchases: "Purchases",
            screenName3 :"Orders",
            sales : "Sales",
            screenName4 :"Orders",
            wishlist:"Wishlist",
            screenName5:"WishList",
            settings:"Settings",
            screenName6:"Settings",
            notifications:"Notifications",
            screenName7:"Notifications",
            support:"Support",
            screenName8 :"Support",
            me:"Me",
            support2:"Support",
            Description1:"Have questions, feature suggestions or bugs to report? Please check our FAQs first." ,
            Description2:"Our Telegram group is a great resource to report bugs or ask for support.",
            Description3:"Our ability to offer email support is very limited." ,
            Description4:"Please attempt to use the FAQ or Telegram group primarily.",
            Description5:"For any critical issues, concerns, or problems with the app and/or content in the marketplace, contact us via email.",
            fAQs:"FAQs",
            telegram:"Telegram",
            email_Support:"Email Support",


        }
    },

    utils: {
        listings: {
            Electronics: "Electronics",
            Women_Clothing: "Women's Clothing",
            Men_Clothing: "Men's Clothing",
            Toys_Games: "Toys and Games",
            Jewelry: "Jewelry",
            Tools: "Tools",
            Gift_Cards: "Gift Cards",
        }
    }
}
