
export default {
    AppNavigator: {
        Please_wait: "请稍等..."
    },
    common: {
        anonymous: '匿名',
        buyer: "买家",
        seller: "卖家",
        the_seller: "卖家",
        you: "你"
    },
    OnboardingWrapper: {
        unknown: '未知',
    },
    components: {
        atoms: {
            ChatBubble: {
                fail_retry: '不能发送、点击重试',
            },
            Comment: {
                posting: '发送...'
            },
            EditListingFooter: {
                save: "保存"
            },
            EditProfileBanner: {
                save: "保存"
            },
            FollowButton: {
                following: '关注中',
                follow: '关注',
            },
            Inventory: {
                surcharge: '附加费用',
                stock: '库存',
                unlimited: '没有限制的',
            },
            LocationPin: {
                unknown: '未知'
            },
            ModalImageIndicator: {
                posInfo: "%{pos} of %{size}"
            },
            MoreButton: {
                more: '更多'
            },
            PayBanner: {
                calculating: '正在计算',
                PAY: "支付"
            },
            PaymentMethod: {
                payment_method: "支付方式",
                wallet_empty: '你的钱包是空的',
                add_funds: '添加资金'
            },
            PostButton: {
                post: '发送'
            },
            ProductPrice: {
                free_shipping: '免费快递',
                shipping: '快递',
                Digital_unavailable: '目前无法购买数字商品。',
                BUY_NOW: "立即购买"
            },
            ResetFilter: {
                reset_filters: '重置过滤器'
            },
            SecureFund: {
                secure_funds: '保护你的资金',
                backup_wallet: '备份钱包'
            },
            UnavailableButton: {
                unavailable: '不可用的'
            }
        },

        molecules: {
            BlockedNodeItem: {
                unknown: '未知',
                unblock: '没有屏蔽',
                block: '屏蔽'
            },
            BuyerReview: {
                from: "来自 %{name}",
                no_review: "还没有收到买家的评论"
            },
            BuyWyre: {
                ask_crypto: "需要加密货币吗?",
                top_up: "用Wyre充值您的钱包!"
            },
            CheckoutNote: {
                Note: "备注",
                add_note: "添加备注到订单上(可选)",
                add_seller_note: "给买家添加一条附注"
            },
            DirectPaymentOption: {
                direct_payment: "直接支付",
                description: "在没有仲裁者的情况下直接汇款给卖方请谨慎使用，除非您完全信任卖家，否则请勿使用"
            },
            FeedItem: {
                delete_post:"删除帖子",
                undo_action1: "你不能撤销此操作.",
                cancel1:"取消",
                delete1:"删除",
                hide_post:"隐藏帖子?",
                undo_action2: "你不能撤销此操作.",
                cancel2:"取消",
                hide: "隐藏",
                share_to1:"分享",
                share_to2:"分享至....",
                delete2:"删除",
                cancel3:"取消",
                go_to_profile:"转到用户页面",
                report_user:"举报用户",
                block_user:"屏蔽用户",
                hide_post:"隐藏帖子",
                cancel4:"取消",
                reposted: "重发"
            },
            FeedPreview: {
                anonymous: "匿名"
            },
            ListingPaymentOptions: {
                not_accepted: "否",
                payment_options: "支付选项"
            },
            ListingReview: {
                from: "从 %{name}",
                no_message: "买家没有留下评论"
            },
            ModerationFee: {
                percentage: "百分比 (%)",
                flat_fee: "法币费用 (%)",
                fee: "费用($)"
            },
            ProductDescription: {
                empty_text: "没有描述",
                read_more: "阅读更多"
            },
            ProductPolicy: {
                no_provided: "不提供%{policy} "
            },
            ProductRating: {
                reviews: "评论",
                Overall: "综合",
                Quality: "质量",
                as_advertised: "广告匹配度",
                Delivery: "配送",
                Service: "服务"
            },
            RadioModalFilter: {
                reason_reporting: "请输入报告此内容的原因.",
                other: "其他: 请解释",
            },
            SellerInfo: {
                about: "关于卖家",
                unknown: "未知",
                message: "消息",
                visit_store: "进入商店"
            },
            ShopCard: {
                unknown: "未知"
            },
            ShopInfo: {
                unknown: "未知",
                reviews: "%{count} 评论",
                following: "关注中",
                followers: "关注者",
                edit_profile: "编辑个人信息",
                message: "消息",
            },
            SocialNotification: {
                unfollowed_you: ' 不再关注你',
                one_of_moderators: ' 现在是你的仲裁人之一',
                removed_from_moderator_list: ' 已从你的仲裁人列表里移除',
                liked_your_post: ' 喜欢了你的帖子',
                commented_on_your_post: ' 评论了你的帖子',
                reposted_your_post: ' 转发了你的帖子',
                followed_you: ' 关注了你'
            },
            WalletCoinItem: {
                coming_soon: "即将上线"
            }
        },
        organism: {
            AverageRating: {
                no_reviews: "没有评论"
            },
            Balance: {
                unconfirmed: "%{balance} 没有确认"
            },
            CategorySelector: {
                category: "类别",
                select_category: "选择一个类别" 
            },
            CheckoutHeader: {
                anonymous: "匿名",
                from_seller: "从 %{seller}",
                each_price: "单价 %{price} "
            },
            CheckoutSummary: {
                add_link: "添加",
                none_select: "无",
                remove_coupon: "移除优惠券?",
                remove_coupon_description: "你确定你想要移除优惠券?",
                cancel: "取消",
                remove: "移除",
                quantity:"数量",
                quantity_info: "数量: %{quantity}",
                coupon_info: "优惠券: %{info}",
                change: "改变",
                free: "免费",
                network_fee: "网络费用",
                fee_alert_description: "费用太高请使用较低的级别费用或其他币种.",
                learn_more: "学习更多",
                summary:"总结",
                total: "总共",
                calculating: "正在计算..."
            },
            CoinTypeSelector: {
                coming_soon: "即将上线"
            },
            DefaultInventoryItem: {
                no_title: "没有标题",
                sku: "SKU",
                sku_info: "库存单位, ID, 等",
                quantity: "数量",
                unlimited: "没有限制的",
                quantity_sold_out: '如果数量达到0，它将显示为“已售完”.',
                quantity_unlimit: '消费者可以购买任意数量的商品.'
            },
            EmptyCoupons: {
                empty_coupon: "您尚未添加任何优惠券",
                add_coupon: "添加优惠券"
            },
            EmptyShippingMethods: {
                empty_shipping_option: "您尚未添加任何快递选项",
                add_shipping: "添加快递选项"
            },
            ErrorModal: {
                error_message: "错误: %{error}"
            },
            ImageSelector: {
                ask_delete: '删除照片吗？',
                cannot_undo: '你不能撤销该操作。',
                cancel: '取消',
                delete: '删除',
                set_primary: "设置为主照片",
                take_photo: '拍照',
                choose_from_gallery: '从图库中选择',
                delte_photo: '删除照片'
            },
            InventoryItem: {
                quantity_info: "QTY: %{quantity}",
                Unlimited: "没有限制的"
            },
            ItemDetail: {
                listing: "商品",
                type: "类型",
                title: "标题",
                ask_selling: "你卖什么?",
                price: "价格",
                condition: "新旧程度",
                description: "描述",
                description_hint: "在这里描述你的商品",
                mature_hint: "成人内容 (NSFW, 成人, 18+)"
            },
            ModerationSettingsEditor: {
                profile_info: "简要信息",
                description: "描述",
                terms: "服务条款",
                languages: "语言",
                primary: "首要",
                secondary: "第二",
                third: "第三"
            },
            Moderator: {
                unknown_moderator: "未知仲裁者",
                unknown_moderator_description: "无法获取仲裁者的信息.",
                verified: "已经验证"
            },
            ModeratorPreview: {
                verified: "已经验证"
            },
            MultiSelector: {
                none: "无",
                all: "全部",
                done: "完成",
                select_info: "%{count} 已选择",
                reset: "重置"
            },
            OptionSelector: {
                current_price: "当前市场价格，请点击",
                learn_more: "了解更多"
            },
            OrderBrief: {
                from: "来自",
                to: "给",
                tap_to: "当前市场价格, 点击 ",
                learn_more: "了解更多"
            },
            OrderDispute: {
                ask_payout: "接受付款?",
                payout_description: "一旦接受，争议将结束，资金将转移",
                cancel: "取消",
                ok: "好的",
                dispute_expired: "争议过期",
                memo_comment1: "仲裁员尚未提出结果。 卖方可以要求付款.",
                dispute_payout: "争议结果",
                will_be_issued: " 将发送给你.",
                moderator_takes: "仲裁者取得",
                seller_takes: "卖家信息 ",
                buyer_takes: "买家信息 ",
                accept_payout: "接受付款",
                started_by: "纠纷开始于 %{name} .",
                the_seller: "卖家",
                the_buyer: "买家",
                memo_comment2: "仲裁人已介入提供帮助,开始聊天以提供更多详细信息.",
                message: "消息"
            },
            OrderFooter: {
                claim: "申明"

            }, 
            OrderFulfillment: {
                no_tracking_number: "没有运单号码可以复制!",
                shipping_via: "快递方式",
                tracking_num: "运单跟踪#",
                tracking_number_copied: "复制运单号!",
                file_url: "文件链接:",
                password: "密码:",
                fulfilled_info: "订单已经完成!",
                order_fulfilled: "订单完成"
            },
            OrderRating: {
                Overall: "整体",
                Quality: "质量",
                as_advertised: "广告匹配度",
                Delivery: "配送",
                Service: "服务",
                no_feedback: "没有反馈留下来 %{name}"
            },
            PanelView: {
                PanelViewBase: {
                    cancel: "取消"
                },
                PlusPanelView: {
                    Sell: "出售",
                    Post: "发帖",
                    Chat: "聊天",
                    Pay: "支付",
                    Choose_action: "选择操作"
                },
                PanelViewBase: {
                    cancel: "取消"
                },
                SharePanelView: {
                    Social: '社交',
                    External: '外部',
                    Share_to: "分享到..."
                }
            },
            PayPanel: {
                ask_pay: "你要如何付款?",
                external_wallet: "外部钱包",
                not_available_eth: "不适用于ETH",
                haven_wallet: "Mobazha 钱包",
                not_enough_funds: "没有足够的资金"
            },
            ProductModeSelector: {
                listings: "%{counts} 个商品"
            },
            ProductRatings: {
                reviews: "评论",
                see_all_reviews: "查看所有 %{length} 条评论",
                no_reviews_yet: "还没有评论"
            },
            ProfileImages: {
                take_photo: '拍照',
                choose_from_gallery: '从图库中选择',
                cancel: '取消'
            },
            QRScanner: {
                scan_qr_payment_address: "扫描付款地址的二维码",
                scan_qr_store: "扫描商店，商家信息或付款地址的QR码"
            },
            SearchFilterHeader: {
                results: "%{total} 结果"
            },
            SearchHeader: {
                search: "搜索..."
            },
            SelectableModerator: {
                view_details: "查看详情"
            },
            SelectorModal: {
                none: '无'
            },
            SendingAddressSelector: {
                send: "发送"
            },
            SendingAmount: {
                send_all: "全部发送"
            },
            ShippingMethod: {
                free: "免费"
            },
            ShippingOptions: {
                add_option: "添加快递选项",
                options_count: "%{count} 快递选项",
                shipping: "快递",
            },
            ShippingPriceEditor: {
                shipping_service: "快递服务 #%{pos}",
                delete: "删除",
                service: "服务",
                shipping_hint: "标准，特快等",
                Duration: "时长",
                Duration_hint: "5-7 天",
                price: "价格",
                additional_price: "额外的价格"
            },
            ShippingPrices: {
                delete_service: '删除快递服务？',
                cannot_undo: "你不能撤销该操作。",
                cancel: '取消',
                delete: '删除',
                add_service: "添加服务"
            },
            SingleVariantEditor: {
                delete: "删除",
                variant_id: "种类 %{id}",
                title: "标题",
                title_hint: "例如：大小",
                description: "描述",
                description_hint: "例如：产品尺寸",
                choices: "选项",
                choices_hint: "例如：小号、中号、大号"
            },
            SupportHaven: {
                support_haven: "支持 Mobazha",
                description: "Haven完全免费，并依靠您的支持来帮助开发。"
            },
            TagEditor: {
                tags: "标签",
                tags_info: "%{count} 个标签",
                add_hint: "在你的商品列表上添加标签"
            },
            TagSuggestion: {
                none: "无"
            }
        },

        templates: {
            AboutTab: {
                phone: "手机",
                copy: "复制",
                link_copied: "店铺链接已经复制!"
            },
            BackupWallet: {
                backup_wallet: "备份钱包",
                backup_description1: "如果您无法访问设备，备份钱包是恢复资金的唯一方法。",
                backup_description2: "您只需要一支纸笔来记下您的助记词。 准备就绪后，请继续下一步。",
                next: "下一步",
                recovery_phrase: "你的助记词",
                phrase_hint: "请按顺序写下每个单词",
                writedown_hint: "确保您的助记词安全。 如果您丢失或更换了手机，则可以使用它来重新获得资金的使用权；从不与任何人分享助记词。 为了安全起见，请避免截图，也不要将其存储在移动设备上。",
                done: "确定"
            },
            CategoryList: {
                more: "更多"
            },
            CategoryProductGrid: {
                see_all: "查看全部"
            },
            ChatDetail: {
                is_typing: "%{peer}正在书写...",
                unread: "没有阅读",
                block_message: "这个用户已经被屏蔽.",
                start_with: "开始会话：",
                moderator_join: "(仲裁者) 加入了讨论",
                say_something_nice: "说点什么好的..."
            },
            Checkout: {
                address_required: "*需要购买的送货地址",
                new_address: "新地址",
                cannot_ship: "此商品无法送到所选地址",
                shipping: "送货",
                payment_protection: "付款保护",
                protect_up_to: "保护我的付款长达 ",
                protect_days: "45 days",
                moderator_not_available: "不适用于此订单",
                change_moderator: "变更仲裁者",
                no_moderator_description: "您的付款将直接发送给卖方。 如果您的订单遇到问题，将无法提出争议。"
            },
            ContractModal: {
                view_contract: "查看合同",
                copied: "复制完毕!"
            },
            CouponApplyModal: {
                apply: "应用",
                enter_coupon: "输入优惠券代码",
                enter_hint: "例如 COUPON123",
                coupon: "优惠券 ",
                not_valid: " 无效."

            }, 
            CouponModal: {
                title_empty_alert: "折扣标题不能为空",
                code_empty_alert: "折扣代码不能为空",
                percentage_alert: "抱歉，该值必须在1到99之间。",
                value_empty_alert: "折扣数值不能空",
                exceed_alert: "抱歉，折扣超出了商品的价值。",
                edit_coupon: "编辑优惠券",
                new_coupon: "新建优惠券",
                save: "保存",
                title: "标题",
                title_hint: "输入一个标题", 
                code: "券码",
                code_hint: "输入一个优惠券码",
                discount: "折扣",
                discount_hint1: "例如：10%",
                discount_hint2: "例如：$10",
                percent: "百分比"
            },
            CovidModal: {
                input_groupon_title: "所需的必需用品（COVID-19",
                description11: "为了在这些困难时期保持安全，世界各地的人们，州和医院的基本用品都非常少。 如果您或您认识的任何人可以快速生产，采购或快递",
                description12: "口罩，N95口罩，外科口罩，洗手液，洗手液，呼吸机，温度计，湿纸巾，卫生纸",
                description13: " 等,请尽快让这些物品流通并运到正确的人手中.",
                description21: "如果我们全力以赴，就可以挽救生命。 世界需要您的支持，以帮助使重要物品流通。 如果您有权使用",
                description22: "缝纫机，3D打印机",
                description23: "，甚至",
                description24: " 酒厂",
                description25: "请考虑创建可以捐赠和/或以合理价格出售的物品和零件.",
                description31: "您的基本物品可以立即在港口免费分发，不需要账户。不问任何问题。请尽你的责任来帮忙。",
                create_listing_title: "创建商品"
            },
            DisputeModal: {
                submit_dispute: "提出争议?",
                submit_hint: "仲裁者将介入以帮助解决争议。 您无法撤消此操作",
                cancel: "取消",
                ok: "好的",
                enter_reason: "请输入争议的原因!",
                content_hint: "您为什么要提出争议？ 提供尽可能多的细节。"
            },
            EULAModal: {
                eula: "最终用户许可协议",
                privacy_description1: "End User License Agreement terms and conditions governing download and use of this mobile application, downloaded by you via Apple, Inc.’s (“Apple”) App Store (the “App Store”) or Google Play. Please read this End User License Agreement terms and conditions carefully.",
                privacy_description2: "This End User License Agreement sets forth the terms and conditions (“Terms”) under which Mogaolei (“Mogaolei”) (alternatively referred to as “us,” “we,” or “our”) offers you the right to download and use the Mobazha mobile application (including any updates thereto, the “Application”) and your use of the Application is governed by these Terms. By accepting these Terms (i) you represent that you are of legal age to enter into a binding contract and (ii) you signify that you have read, understood and agree to these Terms (and that such Terms are enforceable like any other written negotiated agreement signed by you) and certify that you are at least 17 years old or older. If you do not agree to these Terms, or you are not at least 17 years old, you may not use the Application. Violations of these Terms will result in a permanent removal from the Application.",
                privacy_description3: "These Terms constitute an agreement strictly between Mogaolei and you and you acknowledge that Mogaolei (in accordance with the limitations herein) rather than Apple and Google is responsible for any claim or liability arising from your use of the Application including, but not limited to, any third party claim of infringement of intellectual property rights. Nevertheless, you agree to abide by all terms, conditions or usage rules imposed by Apple and Google applicable to the use of this Application, including, but not limited to, any terms, conditions or usage rules set forth in the App Store Terms of Service.",
                privacy_description4: "1. License and Restrictions",
                privacy_description5: "The Application is licensed, not sold, to you. All rights, title and interest (including, without limitation, all copyrights, trademarks and other intellectual property rights) in and to this Application belong to us or our licensors. Subject to your compliance with these Terms, we grant you a non-transferable, non-assignable, revocable, limited license to download and install one copy of this Application on a mobile device that you personally own or control and to use that copy of this Application on that mobile device solely for your own personal use. You may not install or use a copy of the Application on a device you do not own or control. You may not distribute or make the Application available over a network where it could be used by multiple devices at the same time. You may not sell, rent, lend, lease, redistribute, or sublicense the Application or circumvent any technical limitations in the Application or otherwise interfere in any manner with the operation of the Application, or the hardware or network used to operate the Application. You may not copy, reverse engineer, decompile, disassemble, modify, create derivative works or otherwise attempt to derive the source code of this Application. This Application and its content are protected by copyright under both United States and foreign laws. Any use of the Application and its content not explicitly permitted by these Terms is a breach of this agreement and may violate the law. If you violate these Terms, your license to use this Application automatically terminates and you must immediately cease using the Application and destroy all copies, full or partial, of the Application.",
                privacy_description6: "2. Ownership",
                privacy_description7: "We alone (and our licensors, where applicable) shall own all right, title and interest, including, without limitation, all intellectual property rights, in and to the Application and any suggestions, ideas, enhancement requests, feedback, recommendations or other information provided by you or any other party relating to the Application. Any copy, modification, revision, enhancement, adaptation, translation, or derivative work of or created from the Application shall be owned solely and exclusively by us, and/or, as applicable, our third-party vendors, as shall any and all patent rights, copyrights, trade secret rights, trademark rights, and all other proprietary rights, worldwide therein and thereto, and you hereby assign to Mogaolei any and all of your rights, title or interests that you may have or obtain in the Application or any modification to or derivative work of the Application. You shall not remove or authorize or permit any third party to remove any proprietary rights legend from the Application.",
                privacy_description8: "2. Your Responsibilities as the Application User",
                privacy_description9: "Use of the Application requires third party services and equipment such as a compatible mobile device, internet access and a telecommunications carrier. Obtaining and maintaining the equipment and services necessary to use the Application is your responsibility. Mogaolei is not responsible for equipment defects, lack of service, dropped calls, or other issues arising from third party services or equipment. You are solely responsible for your use of those services on your mobile device and compliance with any applicable third party terms and payment of all applicable third party fees associated with any carrier service plan you use in connection with your use of those services (such as voice, data, SMS, MMS, roaming or other applicable fees charged by the carrier). You agree not to use the Application to communicate in an offensive or obscene manner, or to spam, threaten, defame or harass other users. Mogaolei is not in any way responsible for any such use by you or by any person using your device, nor for any harassing, threatening, defamatory, offensive, or illegal messages or transmissions that you may receive as a result of using the Application. Mogaolei reserves the right, but does not assume the obligation, to remove any objectionable activity or language used in the Application at any time. Mogaolei reserves the right, but does not assume the obligation, to not publish or to terminate any communication, or posting it determines objectionable in its sole discretion. Use of the Application is void where prohibited. You shall not use the Application to falsely state or otherwise misrepresent yourself or your affiliation with any person or entity; or to intentionally or unintentionally violate any applicable local, state, national or international law, including, but not limited to, U.S. regulations pertaining to the export of software from the U.S. to embargoed countries. You will ensure that the information you provide to us through the Application is accurate and complete. We reserve the right to immediately terminate your use of the Application should you fail to comply with any of the foregoing.",
                privacy_description10: "4. Third Party Sites, Services and Devices",
                privacy_description11: "The Application may enable you to access third-party mobile applications and websites (“Third Party Materials”). Access to Third Party Materials may require you to accept additional terms and conditions and privacy policies. You acknowledge that Mogaolei is not responsible for the terms and conditions or privacy policies of Third Party Materials. You understand that by using any of the Third Party Materials you may encounter content that may be deemed offensive, indecent, or objectionable, which content may or may not be identified as having explicit language, and that the results of any search or entering of a particular URL may automatically and unintentionally generate links or references to objectionable material. Nevertheless, you agree to use the Third Party Materials at your sole risk and that neither Mogaolei nor its agents shall have any liability to you for content that may be found to be offensive, indecent, or objectionable.\
                Certain Third Party applications or materials may provide links to additional third party websites or allow you to upload or enter your own data. By using the Third Party Materials, you acknowledge and agree that neither Mogaolei nor its agents is responsible for the content, accuracy, completeness, timeliness, validity, copyright compliance, legality, decency, quality or any other aspect of such Third Party Materials, or the data you choose to upload or enter into the Application through those Third Party Materials. Neither Mogaolei nor its agents warrant or endorse, and each does not assume and will not have any liability or responsibility to you or any other person for, any Third Party Materials. Links to Third Party Materials are provided solely as a convenience to you.",
                privacy_description12: "5. User Submissions",
                privacy_description13: "Any information submitted through the Application, including listings, posts, messages, may be provided to our staff and may be viewable to other Application users. Mogaolei is not responsible for the content of any communication submitted or posted by Application users nor do we guarantee the truthfulness, accuracy or validity of any posted communication. Any action you take or do not take based upon information posted to the Application, including, but not limited to, investment, purchasing, trading, employment or other decisions, is done at your own risk.\
                By submitting communications or content to any part of this Application that is viewable by other Application users, you acknowledge that the submission may be viewed and further disclosed by other Application users. We encourage you to not include personally identifiable information in such submissions and cannot be held liable for the further disclosure of your personally identifiable information by other Application users. You acknowledge that Mogaolei only acts as a passive conduit for the distribution of content and other material posted by Application users and is not responsible or liable to you or any third party for the content or accuracy of those materials. We, however, reserve the right, but assume no obligation, to monitor any submissions or postings and delete, move or edit any content that we consider inappropriate or unacceptable for any reason. You shall not submit any communication or content that infringes or violates any right of any party or that is not original to you. Illicit or abusive content is strictly prohibited. Where we do moderate interactive features, we will endeavor to review comments and postings for relevance, topicality and appropriateness, and we may withhold or remove postings for any reason, within our sole discretion. We are unlikely to post comments relating to ongoing legal matters or regulatory issues.\
                We reserve the right to republish and use any material contributed by Application users as permitted by these Terms or otherwise by law. By posting a message, content or other material in any public area of the Application or submitting any correspondence to us, you expressly grant us, and anyone authorized by us, a global, royaltyfree, perpetual, irrevocable, unrestricted, nonexclusive license to publish, reproduce, sell, disclose, modify, create derivative works from, distribute, publicly perform or display, or otherwise use such material, in whole or in part, in any manner or medium (whether now known or hereafter developed), for any purpose whatsoever. You hereby further grant us, and anyone authorized by us, the global, royalty-free, perpetual, irrevocable, unrestricted, nonexclusive right to use any ideas, concepts or techniques, in whole or in part, in any manner or medium (whether now known or hereafter developed), embodied in such materials for any purpose whatsoever. In addition, you hereby waive any and all moral rights you may have in any such materials. You also agree that all such material will be deemed to be provided to us on a non-confidential and non-proprietary basis. Material that is copyright protected may not be submitted without permission from the copyright owner, and you are solely responsible for the failure to obtain any such permission.\
                We will comply with any legal requests to disclose any submissions, communications or postings to others, including to law enforcement agencies.",
                privacy_description14: "6. Privacy Statement",
                privacy_description15: "Your use of the Application is also subject to the terms and conditions of the Mobile Application Privacy Policy.",
                privacy_description16: "7. Legal Compliance",
                privacy_description17: "The Application is subject to United States export laws and regulations. You will not use or otherwise export the Application except as authorized by United States law and the laws of the jurisdiction in which the Application was obtained. You represent and warrant that (i) you are not located in a country that is subject to a U.S. Government embargo, or that has been designated by the U.S. Government as a “terrorist supporting” country; and (ii) you are not listed in any U.S. Government list of prohibited or restricted parties. Mogaolei does not represent that the Application is appropriate or available for use in all countries. Mogaolei prohibits accessing materials from countries or states where such content is illegal. You are using the Application on your own initiative and you are responsible for compliance with all applicable laws.",
                privacy_description18: "8. Disclaimer of Warranty",
                privacy_description19: "Any use of the Application shall be at your sole risk. This Application and the information you access through the Application is provided on an 'AS IS', 'WITH ALL FAULTS' and 'AS AVAILABLE' basis and without any warranty, express or implied, of any kind, to the fullest extent permissible pursuant to applicable law. Mogaolei, Apple, Google, wireless carriers over whose network the Application is distributed, and each of our respective affiliates and suppliers (collectively, “Distributors”) give no express or implied warranties, guarantees, or conditions under or in relation to the Application. Distributors disclaim all express or implied warranties related to the Application including, but not limited to, implied warranties for merchantability, non-infringement, and fitness for a particular purpose. Distributors make no warranty as to the reliability, accuracy, timeliness, usefulness or completeness of the Application or any information accessed through the Application. Distributors cannot and do not warrant against human, services and machine errors, omissions, delays, failures, interruptions or losses. Distributors cannot and do not guarantee or warrant that the Application will be free of infection or viruses, worms, malware, Trojan Horses or other malicious codes. Mogaolei reserves the right to terminate, without notice, your use of the Application at any time and for any reason. Please note that some jurisdictions may not allow the exclusion of implied warranties, so some of the above exclusions may not apply to you. In such case, exclusions will apply to the greatest extent consistent with applicable law. You are solely responsible for any damages to your hardware device(s) or loss of data that results from the download or use of the Application. Your sole and exclusive remedy for dissatisfaction with the Application is to stop using it.",
                privacy_description20: "9. Limitation of Liability",
                privacy_description21: "Under no circumstances will Distributors be liable for any damages you suffer as a result of your reliance on this Application or any content provided by the Application or Third Party Materials, nor will Distributors be liable to you or any third party for any incidental, special, consequential, indirect or punitive damages whatsoever, including, without limitation, loss of profits, loss of data, business interruption or any other personal injury or commercial damages or losses arising out of or that result from the use of, or the inability to use, the Application, regardless of the theory of liability (contract, tort, strict liability, negligence, guarantee or condition, or otherwise), even if advised of the possibility of such damages or repair or replacement of the Application does not fully compensate you for any losses. In no event shall Distributor's total liability to you for all damages (other than as may be required by applicable law in cases involving personal injury) exceed the amount of One Hundred ($100) Dollars. The foregoing limitations will apply even if the above stated remedy fails of its essential purpose.",
                privacy_description22: "10. Maintenance and Support Services",
                privacy_description23: "Any maintenance and support services made available by Mogaolei are at the discretion of Mogaolei which may initiate or cease providing maintenance and support services at any time without notice to you. You acknowledge that Apple, Google, and your wireless carrier are not responsible for providing maintenance and support services for the Application.",
                privacy_description24: "11. Location Data",
                privacy_description25: "Mogaolei, Apple, Google, Distributors or other providers or their partners may collect, maintain, process and use your location data, including the real-time geographic location of your mobile device as necessary to provide the Application’s full functionality. By using or activating any location-based services on your mobile device, you agree and consent to Mogaolei's, and such parties' collection, maintenance, publishing, processing and use of your location data to provide you with such services. You may withdraw this consent at any time by turning off the location-based feature on your mobile device or by not using any location-based features. Turning off or not using these features may impact the functionality of the Application. Location data provided by the Application is for basic navigational purposes only and is not intended to be relied upon in situations where precise location information is needed or where erroneous, inaccurate or incomplete location data may lead to death, personal injury, property or environmental damage. Use of real time route guidance is at your sole risk. Location data may not be accurate. Neither Mogaolei, nor such parties guarantee the availability, accuracy, completeness, reliability or timeliness of information or location displayed by the Application.",
                privacy_description26: "12. Choice of Laws, Jurisdiction, Entire Agreement",
                privacy_description27: "By downloading or using the Application, you expressly agree that these Terms shall be governed by and construed in accordance with the laws of the State of Delaware, without giving effect to its conflict of laws provisions or your actual state or country of residence. You further expressly agree that exclusive jurisdiction for any dispute with Mogaolei in any way relating to your use of this Application is in the federal or district courts of the State of Delaware, and you agree and expressly consent to the exercise of personal jurisdiction in state or federal court in the State of Delaware, in connection with any such dispute including any claim involving Mogaolei or its affiliates or content providers. If any provision of these Terms shall be unlawful, void, or for any reason unenforceable, then that provision shall be deemed severable from these Terms and shall not affect the validity and enforceability of any remaining provisions. This is the entire agreement between the parties relating to the subject matter herein and it supersedes all previous or contemporaneous agreements, proposals and communications, written or oral, relating to that subject matter. As a user of the Application, you agree to contact us prior to seeking legal recourse for any harm you believe you have suffered from your use of the Application. In the event that you believe our Application has harmed you, you agree to inform us and to give us thirty (30) days to cure the harm before initiating any action. You also agree that you must initiate any cause of action within one (1) year after the claim has arisen, or you will be barred from pursuing any cause of action.",
                privacy_description28: "13. Indemnity",
                privacy_description29: "You will defend, indemnify and hold Mogaolei, its officers, directors, employees, agents, licensors, and vendors, harmless from and against any and all claims, actions or demands, liabilities and settlements including without limitation, reasonable legal and accounting fees, resulting from, or alleged to result from, (i) your violation of these Terms, whether by act, omission or negligence, or by any other person using your account, (ii) your use of the Application, (iii) your violation of any rights of another, and/or (iv) any communications, content or other material posted to or transmitted through the Application by you or by others using your account.",
                privacy_description30: "14. Third Party Beneficiary",
                privacy_description31: "Mogaolei and you acknowledge that Apple, Apple’s subsidiaries, Google, Google’s subsidiaries are third party beneficiaries to this agreement. Upon your acceptance of these Terms, Apple and Google will have the right (and will be deemed to have accepted the right) to enforce these Terms against you as a third party beneficiary. Aside from Apple and Google, there are no third party beneficiaries to this agreement.",
                privacy_description32: "15. Amendment",
                privacy_description33: "We have the right, at any time and without prior written notice, to add to or modify the Terms, by amending the Terms available within the Home page or by requiring you to accept an updated agreement upon accessing the Application. Your access or use of the Application after the date of such amended Terms constitutes acceptance of such amended Terms. By continuing to access or use the Application after we post such changes, you agree to these Terms, as modified.",
                privacy_description34: "16. Contact Us",
                privacy_description35: "For Questions, please email us at admin@mobazha.com",
                privacy_description36: "17. Copyright Infringement – DMCA Notice",
                privacy_description36: "The Digital Millennium Copyright Act of 1998 (the “DMCA”) provides recourse for copyright owners who believe that material appearing on the Internet infringes their rights under US copyright law. If you believe in good faith that content or material on this Application infringes a copyright owned by you, you (or your agent) may send Mogaolei a notice requesting that the material be removed, or access to it blocked. This request should be sent to: admin@mobazha.com. The notice must include the following information: (a) a physical or electronic signature of a person authorized to act on behalf of the owner of an exclusive right that is allegedly infringed; (b) identification of the copyrighted work claimed to have been infringed; (c) identification of the material that is claimed to be infringing or the subject of infringing activity; (d) the name, address, telephone number, and email address of the complaining party; (e) a statement that the complaining party has a good faith belief that use of the material in the manner complained of is not authorized by the copyright owner, its agent or the law; and (f) a statement that the information in the notification is accurate and, under penalty of perjury, that the complaining party is authorized to act on behalf of the owner of an exclusive right that is allegedly infringed. If you believe in good faith that a notice of copyright infringement has been wrongly filed against you, the DMCA permits you to send us a counter-notice. Notices and counter-notices must meet the then-current statutory requirements imposed by the DMCA. Notices and counternotices with respect to the Application should be sent to the address above.",
                iaccept: "接受"
            }, feed: {
                not_post: "\n还没有发布任何信息",
                post_hint1: "你还没有发布任何信息.",
                post_hint2: "在社区分享一些东西!",
                Create_post: "创建帖子",
                reported: "报告"
            },
            FeedDetail: {
                fail_to_load: "糟糕！这个发布装载失败.",
                retry: "重试",
                Loading: "加载中....",
                reported: "报告"
            },
            FeedTabContent:{
                first_comment:"首次评论!",
                first_likes: "首次置顶!",
                first_repost:"首次转发!",
                },
            FulfillModal: {
                done: "完成",
                shipping_carrier: "快递公司",
                carrier_hint: "顺丰、中通、韵达，等等",
                tracking_number: "快递单号",
                tracking_number_hint: "快递单号码",
                file_url: "文件链接",
                file_url_hint: "https://fileurl.com",
                password: "密码",
                password_hint: "可选",
                note: "备注",
                note_hint: "可选",
                add_a_note: "添加备注（可选）"
            },
            GlobalFeed: {
                Trending: "流行",
                Most_Recent: "最近",
                customise_feed: "按照一些配置文件自定义！",
                not_found: "什么都没有找到",
                share_with_community: "在社区分享一些东西!",
                create_post: "创建一个帖子",
                reported: "报告"
            },

            InfiniteProducts: {
                loading_listings: "正在加载商品....."
            },
            InventoryEditor: {
                details: "详细",
                surcharge: "附加费用",
                total: "总共",
                sku: "SKU",
                sku_description: "SKU, ID, etc",
                quantity: "数量",
                unlimited: "没有限制的",
                quantity_sold_out: '如果数量达到0，它将显示为“已售完”.'
            },
            InventoryList: {
                combos_info: "%{count} 各类组合",
            },
            ListingAdvancedDetails: {
                Return_Policy: "退款政策",
                Refunds: "退款",
                return_description: "您的退货政策是什么？接受多长时间退货？谁为退货付款？",
                terms: "条款和条件",
                T_C: "条款和条件",
                terms_description: "商品购买的条款和条件是什么？您作为供应商负责什么？有保修吗？"
            },
            ListingBasicInfo: {
                advanced: "高级",
                advanced_description: "添加种类，店铺政策，优惠券和管理库存"
            },
            ListingCustomOptions:{
               variant:"种类",
               add_variant:"添加种类",
               track_Inventory:"跟踪存货",
               inventory:"存货",
               add_description:  "添加大小、颜色、材质等。"
            },
            ListingsTab: {
                loading: "加载中.....",
                no_sale: "目前无任何出售。",
                check_later: "请稍后再检查！",
                store_empty: "你的店铺是空的",
                put_for_sale: "布置一些商品出售！",
                create_listing: "创建商品",
            },
            ListShippingMethod: {
                delete_option: '删除快递选项？',
                cannot_undo: "你不能撤销改操作。",
                edit: '编辑',
                cancel: '取消',
                delete: '删除',
                add_option: "添加快递选项"
            },
            NeedCoin: {
                coinbase: "Coinbase",
                cryptocurrency_exchange: "加密货币交换"
            },
            NewsFeedFooter: {
                take_photo: '拍照',
                choose_from_gallery: '从图库中选择',
                cancel: '取消'
            },
            Notification: {
                Social: "社交",
                Orders: "订单",
                social_empty: "如果有人关注您或者回复您的帖子，您会在这里看到。",
                order_empty: "敬请关注。您的订单更新将显示在这里。"
            },
            OrderCategorySelector: {
                All: "全部",
                all: "所有%{type}",
                purchases: "购买",
                sales: "出售",
            },
            OrderState: {
                no_orders: "没有订单",
            },

            OrderSummary: {
                Summary: "概要",
                Note: "备注",
                oops: "糟糕!",
                dispute_pending_alert: "订单仍待处理时，您无法提出争议。",
                dispute_not_fulfilled_alert: "您必须先完成订单，才能提出争议",
                dispute_cancel_alert: "您无法对已取消的订单提出争议.",
                dispute_refund_alert: "您无法对退款订单提出争议.",
                dispute_resolved_alert: "您无法针对已结案的订单提出争议。",
                dispute_completed_alert: "您无法对已完成的订单提出争议.",
                dispute_finalized_alert: "此订单无异议。 卖方已要求为此订单付款。",
                dispute_processing_alert: "此订单无异议。 请取消您的订单以全额退款。",
                quantity_info: "数量: %{quantity}",
                view: "查看",
                view_transaction: "查看交易",
                payment: "支付",
                no_payment: "尚未找到此订单的付款。 最多可能需要一分钟才能检测到付款。",
                cannot_dispute: "资金直接发送给%{user}. 您不能对此订单提出争议。",
                escrow_released: "资金已从托管处释放。 订单不再有争议。",
                order_in_dispute: "该订单有争议 ",
                until_accept: "或直到一方接受付款为止。",
                period_expired_claim: "争议期已过。 卖方现在可以要求付款。",
                period_expired_claim2: "争议期已过。 现在可以全额领取资金.",
                order_in_escrow1: " 订单资金正在第三方托管",
                order_in_escrow2: "或直到买家完成订单为止。如果您对此订单有任何疑问，可以与仲裁人提出争议.",
                dispute_order: "订单争议",
                claim_payment: "索偿",
                dispute_error_possible: "处理此订单时发生错误。 请提出争议以收回您的资金.",
                dispute_error: "处理此订单时发生错误。",
                order_refunded: "订单已经退款",
                full_refund: "卖方已对此订单全额退款",
                order_completed: "订单完成",
                release_to_seller: "资金已发放给卖方",
                dispute_closed: "争议关闭",
                dispute_closed_info: "%{user} 已接受付款。 现在该纠纷已经结案.",
                the_seller: "卖家",
                the_buyer: "买家",
                payment_claimed: "已付款",
                seller_claim: "卖方已要求为此订单付款。",
                order_canceled: "订单取消",
                user_cancel_order: " %{user} 已经取消订单. 资金已经全额退还",
                period_expired: "争议期限已过",
                no_dispute: "在45天的争议期内，没有任何争议。 卖方现在可以要求付款。",
                Shipping: "快递",
                total: "总共",
                no_buyer_note: "买家没有留下附注",
                address_copied: "地址已经复制"
            },
            PurchaseState: {
                thank_you: "谢谢你!",
                order_placed: "您的订单已下达。 您可以随时跟踪或管理您的订单。",
                processing: "处理中...",
                hang_tight: "稍等一下, 这可能需要一分钟的时间。",
                Uh_oh: "Uh_oh",
                transaction_failed: "您的交易失败, 请再试一次.",
                retry: "重试",
                order_details: "订单详情",
                error: "错误："
            },
            RatingModal: {
                done :"确定",
                Overall: "整体",
                Quality: "质量",
                As_advertised: "如广告所示",
                Delivery: "快递",
                Service: "服务",
                Write_a_review: "在这里写下评论",
                Post_anonymously: "匿名发送"
            },
            RecentSearch: {
                Recent: "最近",
                Suggestions: "建议"
            },
            ReportTemplate: {
                Ooops: "糟糕！",
                enter_reason: "请输入报告此内容的原因。",
                why_report_profile: "您为什么要举报此个人资料？",
                why_report: "你为什么要举报?",
                next: "下一步",
                submit: "提交",
                describe_issue: "请描述问题（可选）",
                provide_details: "尽可能提供详细资料"
            },
            SearchResults: {
                loading_results: "正在加载搜索结果...",
                no_found: "没有结果.",

            },
            SendMoney: {
                NEXT: "下一步",
                send_to: "发送到",
                paste_or_scan: "请扫描二维码地址"
            },
            SendReceiveMoney: {
                Receive: "接收",
                Send: "发送"
            },
            Settings: {
                Settings: "设置",
                are_you_sure: "您确定吗?",
                check_backup: "您是否备份了当前商店？",
                cancel: "取消",
                OK: "好的",
                profile: "个人资料",
                Country: "国家",
                currency: "货币",
                shipping_address: "邮寄地址",
                blocked: "屏蔽",
                notifications: "通知事项",
                push_notifications: "推送通知事项",
                store: "店铺",
                Policies: "政策",
                Moderators: "仲裁者",
                coins_accepted: "接受货币",
                selected: "已选择",
                Advanced: "高级",
                Analytics: "匿名分析",
                On: "开",
                Off: "关",
                Backup_wallet: "备份钱包",
                Backup_profile: "备份用户数据",
                Restore_profile: "恢复用户数据",
                Resync_transactions: "同步交易",
                Server_Log: "后台日志",
                Version: "版本号 0.7.6"
            },
            SocialPostTemplate:{
                RePostTemplate:{
                    delete_repost:"删除转发?",
                    delete_feed:"删除转发，将会从你的数据中清除",
                    cancel:"取消",
                    delete:"删除",
                    repost:"转发",
                    repost_with_comment:"转发并评论",
                }
            },
            StoreModeratorList: {
                moderators_count: "%{count} 个仲裁者",
                moderators_added: "新的仲裁者会自动添加到您的商店"
            },
            StoreTabs: {
                store: "商店",
                posts: '信息发布',
                about: '关于'
            },
            Toast: {
                post_created: "帖子已经创建",
                view: "查看"
            },
            TransactionHistory: {
                Transactions: "交易",
                no_transaction_recorded: "尚无交易记录",
                no_transactions: "还没有交易",
                notes: "请注意，某些付款可能不会显示在交易记录中。 但是，总余额反映了所有已发送和已接收的交易."
            },
            UserSearchResults: {
                loading_results: "加载搜索结果 ...",
                no_results: "没有结果."
            },
            VariantEditor: {
                Delete_variant: '删除种类？',
                cannot_undo: "你不能撤销改操作。",
                cancel: '取消',
                delete: '删除',
                Add_variant: "添加种类"
            },
            wishlist: {
                wishlist_empty: "你的收藏夹是空的"
            }
        }
    },

    config: {
        categories: {
            Books: "书籍",
            Electronics: "电子产品",
            Games: "游戏",
            Clothing: "女装",
            Apparel_for_Men: "男装",
            Cellphones_Telecommunications: "手机和通讯",
            Computer_Office: "电脑和办公",
            Jewelry_Accessories: "珠宝和配饰",
            Home_Garden: "家居",
            Luggage_Bags: "箱包",
            Shoes: "鞋子",
            Mother_Kids: "母婴",
            Sports_Entertainment: "运动和娱乐",
            Beauty_Health: "美容与健康",
            Watches: "手表",
            Automobiles_Motorcycles: "汽车和摩托车",
            Lights_Lighting: "灯饰",
            Furniture: "家具",
            Electronic_Components_Supplies: "电子配件",
            Consumer_Electronics:"消费类电子产品",
            Toys_Hobbies: "玩具爱好",
            Apparel_Women: "女装",
            Weddings_Events: "婚礼和活动",
            Novelty_Special_Use: "新奇和特殊用途",
            Office_School_Supplies: "办公和学校用品",
            Home_Appliances: "家用电器",
            Home_Improvement: "家居装修",
            Security_Protection: "安全与保护",
            Tools: "工具",
            Hair_Extensions_Wigs: "接发和假发",
            Apparel_Accessories: "服饰与配饰",
            Underwear_Sleepwears: "内衣和睡衣",
            Gift_Cards: "礼品卡",
            Other: "其他"
        },
        feePlans: {
            Super_economic_v: '十分经济（最便宜，最慢）',
            Super_Economic: '十分经济',
            Economic_v: '经济（便宜，慢）',
            Economic: '经济',
            Normal_v: '普通（平均费用和等待时间）',
            Normal: '普通',
            Priority_v: '优先（最贵，最快）',
            Priority: '优先'
        },
        productTypes: {
            Any: "任意",
            Physical_Good: "物理商品",
            Digital_Good: "数字商品",
            Service: "服务"
        }
    },

    reducers: {
        createListing: {
            Free_Worldwide_Shipping: '全球免邮',
            Standard: '标准',
            days_30: '30天'
        },
        saga: {
            order: {
                Unknown_error: '暂不支持离线支付'
            }
        }
    },

    screens: {
        acceptedCoins: {
            update_listings: "更新商品页面?",
            sure_about_update: "您确定所有列表都会更新吗？",
            cancel: "取消",
            OK: "好的",
            coins_accepted: "接受货币",
            save: "保存",
            selected: "已选择",
            clear_all: "全部清除"
        },
        addListingCoupon: {
            Coupons: "优惠券"
        },
        addShippingMethod: {
            fill_required: "请填写所有必填字段",
            must_be_less: "快递选项名称的长度必须小于40个最大值",
            select_destination: "请选择一个快递选项",
            add_shipping_option: "添加一个快递选项",
            shipping_option: "快递选项",
            title: "标题",
            option_description: "顺丰快递，国际快递，等",
            destinations: "目的地",
            save: "保存",
        },
        analytics: {
            details1: '会话信息，例如您多久使用一次该应用程序以及持续多长时间。',
            details2: '基本设备信息，例如，您使用的是哪种电话。',
            details3: '您访问应用程序的国家/地区。',
            details4: '您正在使用哪个版本的应用程序。',
            details5: '您选择了哪种语言。',
            details6: '当您购买结帐时（不收集所购买商品的信息）。',
            details7: '汇款时以及使用哪种付款方式（不会收集有关付款本身的详细信息，例如地址或金额）。',
            details8: '当您创建一个商品时（不会收集商品自身信息）。',
            details9: '在Haven内采取的动作，例如点击社交动态或您发新帖的频率。这些动作本身的内容永远不会被记录，只会记录您执行该动作的事实。',
            Analytics: "匿名分析",
            Share_anonymous: "分享匿名分析",
            description: "如果您选择共享分析，则表示您同意与我们共享以下信息："
        },
        backupProfileInit: {
            back_up_profile: "备份资料",
            ensure_backup1: " 通过频繁备份确保您的数据安全。",
            ensure_backup2: " 目前，您需要手动备份数据。 ",
            ensure_backup3: "将来我们将推出更好的备份系统。",
            ensure_backup4: "您的备份将包括您的所有数据，包括钱包资金。",
            next: "下一步"
        },
        backupProfilePassword: {
            password_empty: "密码为空",
            password_empty_hint: "请设置一个密码", 
            password_mismatch: "密码错误",
            password_mismatch_hint: "请设置正确的密码",
            take_a_minute: "这可能要花费一分钟...",
            backup_done: "备份完成",
            backup_failed:"备份失败",
            backupProfileUpload: "备份个人资料上传",
            set_password: "设置一个密码",
            password: "密码",
            confirm: "确认",
            confirm_password: "确认密码",
            hint1: "设置一个密码",
            hint2: "确保写下来",
            hint3: "您将需要密码来恢复您的个人资料。",
            next: "下一步"
        },
        backupProfileUpload: {
            message:  "这里是备份用户数据!",
            upload_1: "请将您的备份上传到安全的外部地址\n",
            upload_2: " 以确保您在丢失手机后\n可以恢复数据。",
            upload_backup: "上传备份",
            done: "确定"
        },
        blockedNodes: {
            no_block: "没有屏蔽任何人"
        },
        categoryOverview: {
            see_all: "查看全部"
        },
        chats: {
            start_conversation: "开始会话",
            new_chat: "新聊天",
            no_discussion: "无订单讨论",
            chat: "聊天"
        },
        chatDetail: {
            Unfollowed: '已取消关注',
            Followed: '已关注！',
            Delete_conversation: '删除这个会话?',
            cannot_undo: "你不能撤销该操作。",
            Cancel: '取消',
            Delete: '删除',
            Go_to_profile: '转到用户页面',
            Unblock_user: '取消屏蔽用户',
            Block_user: '屏蔽用户',
            Unfollow: '取消关注',
            Follow: '关注',
            Delete_conversation: '删除会话'
        },
        checkout: {
            pay_info: "支付 %{amount}?",
            cancel: "取消",
            pay_now: "立即支付",
            Super_Economic: '十分经济'
        },
        checkoutOption: {
            Checkout: "查看"
        },
        checkoutModerators: {
            select_moderator: "选择一个仲裁者",
            no_available: "没有可用的仲裁者"
        },
        createListing: {
            title_required: '商品标题为必填项',
            price_required: '商品价格为必填项',
            type_required: '商品类型为必填项',
            listing_created: '商品已创建！',
            has_created: '这个商品已经被创建了。',
            back_to_store: '回到店铺',
            see_listing: '查看商品',
            warning: '警告',
            warning_info: '如果你回退，你将丢失你的进度。',
            cancel: '取消',
            ok: '好的'
        },
        customOptions: {
            Variants_Inventory: "种类和库存",
            Save: "保存"
        },
        editInventory: {
            edit_variant_combo: "编辑种类组合",
            apply: "应用"
        },
        editShippingAddress: {
            name_required: "名称为必填项",
            address_required: "地址为必填项",
            city_required: "城市为必填项",
            country_required: "国家为必填项",
            new_address: "新地址",
            done: "确定",
            your_address: "您的地址",
            name: "姓名",
            company: "公司",
            address: "地址",
            address2: "地址 2",
            city: "城市",
            state: "省",
            postal_code: "邮政编码",
            country: "国家",
            delivery_notes: "交货单"
        },
        editVariants: {
            Manage_Variants: "管理种类",
            Save: "保存",
            fill_required: '请填写所有必填（*）字段。',
            fill_choices: '请至少填写两个选项。',
            are_you_sure: '您确定吗？',
            unsaved_discard: '任何未保存的更改都将被丢弃。',
            ok: "好的",
            cancel: '取消'
        },
        externalPay: {
            address_copied: "地址已经复制",
            amount_copied: "金额已经复制",
            pay_order: "付款完成订单",
            copy_address: "复制地址"
        },
        externalStore: {
            unblock_user: "取消屏蔽该用户以查看其内容",
            unblock:"解除屏蔽",
            large:"大",
            loading: "加载中.....",
            failed_load: "糟糕！ 此用户信息无法加载。",
            retry:"重试",
            reported: "报告",
            Create_Listing: '创建商品', 
            Create_Post: '创建帖子',
            Share_to: '分享到...',
            Cancel: '取消',
            Report_user: '举报用户',
            Block_user: '屏蔽用户'
        },
        feed: {
            My_Feed: "我的订阅",
            Global: "全球",
            New_features: "新功能！",
            feature_description: "社交改善。个性化的订阅，应用内提醒，以及更多！",
            Social: "社交"
        },
        followers: {
            followers: "关注者",
            no_followers1: "%{name} 还没有任何关注者",
            no_followers2: "你还没有任何关注者"
        },
        followings: {
            following: "关注中",
            no_following1: "%{name} 没有关注任何人",
            no_following2: "你还没有关注任何人"
        },
        listing: {
            are_you_sure: "您确定吗?",
            ask_block: "屏蔽这个用户?",
            cancel: "取消",
            OK: "好的",
            ask_delete: "删除商品页面?",
            delete_hint: "你不能撤销这个操作.",
            cancel: "取消",
            remove: "移除",
            failed_load: "糟糕！ 此商品无法加载.",
            retry: "重试",
            loading: "加载中.....",
            policy1: "退款条件",
            policy2:"条款和条件",
            add_wishlist: "添加到收藏夹!",
            remove_wishlist: "从收藏夹移除!",
            reported: "已举报!",
            report_listing: '举报商品',
            block_user: '屏蔽用户',
            Edit_Listing: '编辑商品',
            Delete_Listing: '删除商品'
        },

        listingAdvancedDetails: {
            store_policies: "店铺政策",
            apply: "应用"
        },

        listingAdvancedOptions: {
            add_coupons: "添加优惠券",
            advanced: "高级",
            Variants_Inventory: "种类和库存",
            add_hint: "添加种类并管理你店铺库存",
            store_policies: "店铺规则",
            policies_hint: "添加退货政策或服务条款",
            coupons: "优惠券"
        },
        Me: {
            support1: "有问题，功能建议或错误要报告吗？ 请先查看我们的常见问题解答。 我们的微信小组是报告错误或寻求支持的理想资源.",
            support2: "我们提供电子邮件支持的能力非常有限。",
            support3: " 请尝试主要使用FAQ或微信组.",
            description: "对于市场中应用程序与内容的任何关键问题，疑虑或问题，请通过电子邮件与我们联系."
        },
        moderatorDetails: {
            remove_moderator: "移除仲裁者?",
            remove_hint: "该仲裁将从您的商店中永久删除。您将无法再次添加它们",
            cancel: "取消",
            OK: "好的",
            verified: "验证的",
            fee_description: "该费用仅在发生争议时适用。",
            moderator_verified: "该仲裁者已经通过验证",
            terms: "服务条款",
            selected: "已选",
            select: "选择"
        },
        newChat: {
            Search: "搜索...",
            Search_user: "查找用户"
        },
        newFeed: {
            Create_failed: "帖子创建失败",
            unknown_error_create: "创建帖子时发生未知错误",
            char_left: " char left",
            what_going_on: "分享一些好玩的事情"
        },
        notifications: {
            Notifications: "通知"
        },
        notificationSettings: {
            notification_preferences:"通知首选项",
            all1:"全部",
            Receive_all:"接收全部通知", 
            all2:"全部",
            featured_content: "特色内容",
            notify1: "通知我有关Haven的优惠，折扣和其他有趣内容",
            promotions: "促销活动",
            giveaways1:"赠品",
            Notify2: "通知我有关Haven的赠品和其他促销活动",
            giveaways2:"赠品",
            announcements1: "公告",
            notify3:'通知我新功能，更新和其他与App有关的公告',
            announcements2: "公告",
            chat1: "聊天",
            notify4:'收到聊天消息时通知我',
            chat2: "聊天",
            likes1: "点赞",
            notify5:"有人点赞我的帖子时通知我.",
            likes2: "点赞",
            comments1: "评论",
            notify6:"有人评论我的帖子时通知我.",
            comments2:"评论"

        },
        onboarding: {
            HELLO: "您好!",
            restore_profile: "恢复个人资料",
            name: "姓名",
            optional: "可选的",
            country: "国家",
            currency: "币种",
            code:"邮编",
            share_analytics: "分享匿名分析数据",
            help_improve: "帮助我们改进Haven"
        },
        order: {
            orders: "订单",
            purchases: "购买",
            sales: "出售"
        },
        orderDetails: {
            details: "详情",
            discussion: "讨论",
            decline_order: "拒绝下单?",
            decline_hint: "该订单将被取消，这笔钱将退还给买方",
            nevermind: "没关系",
            ok: "好的",
            refund_order: "退款订单?",
            refund_hint: "该订单将被取消，这笔钱将退还给买方.",
            cancel_order: "取消订单?",
            cancel_hint: "该订单将被取消，您的钱将全额退还.",
            have_refunded: "您已退还订单",
            error_happened: "由于未知问题而发生错误",
            order_discussions:"没有订单讨论",
            fund_order: "资金订单",
            leave_notes: "留下附注",
            number_copied: "订单编号已经复制",
            learn_more: "由于汇率的变化，订单的当前市场价格可能与购买时商品的总价格不同.",
            go_to_seller_profile: '转到卖家页面',
            go_to_buyer_profile: '转到买家页面',
            view_listing: '查看商品',
            copy_order_number: '复制订单号',
            view_contract: '查看交易合约',
            cancel: '取消'
        },
        paymentMethod: {
            select_fee_level: "请选择费用等级",
            not_accepted: "否",
            coming_soon: "即将上线",
            payment_method: "支付方式",
            done: "确定",
            transaction_speed: "交易速度",
            Super_Economic: '十分经济',
            Economic: '经济',
            Normal: '普通',
            Priority: '优先'
        },
        paymentSuccess: {
            transaction_details: "交易详情",
            processing: "处理中…",
            hang_tight: "稍等一下 这可能需要一分钟.",
            Uh_oh: "Uh oh!",
            failed_go_through: "您的交易失败。 请再试一次.",
            retry: "重试",
            error: "错误:"
        },

        policies: {
            store_policies: "店铺规则",
            save: "保存",
            terms: "条款和条件",
            terms_hint: "商品购买的条款和条件是什么？ 您作为供应商负责什么？ 有保修吗？",
            refunds: "退款",
            refund_hint: "您的退货政策是什么？ 退货接受多长时间？ 谁为退货付款？"
        },
        privacy: {
            privacy_policy: "隐私政策",
            terms: "服务条款",
            privacy: "隐私",
            privacyDescription1: "Mobazha is built to give you far more privacy in your commerce, messaging, and payments than other apps.It uses several advanced technologies to keep your information from prying eyes, such as peer-to-peer networking and end-to-end encryption.",
            privacyDescription2: "There are ways to use Mobazha which improve or diminish your privacy. To learn more about how the underlying technology works, and what steps you can take to improve your privacy, tap the privacy policy link below.",
            privacyDescription3: "Before you proceed, you must accept the Mobazha https://mobazha.com/terms and https://mobazha.com/privacy.",
            cancel: "取消",
            I_accept: "我接受"
        },
        ProductRatings: {
            reviews: "评论",
            No_reviews: "尚无评论"
        },
        profileSettings: {
            warning: "警告",
            warning_info: "如果您返回，将会丢失你的进度。",
            Cancel: "取消",
            OK: "好的",
            profile_information: "档案信息",
            name: "姓名",
            name_hint: "张三",
            bio: "简介",
            bio_hint: "写一条简短的描述",
            location: "位置",
            email:"邮箱",
            location_hint: "比如，上海",
            contact: "联系方式",
            contact_hint: "satoshin@gmx.com",
            phone_number: "手机号",
            phone_hint: "+123456789",
            website: "站点",
            website_hint: "hello.com",
            Aaout: "关于",
            about_hint: "分享更多有关您自己的信息"
        },
        purchaseSuccess: {
            successfully_sent: "成功发送信息",
            received_message: "你接受了一条信息",
            sent: "发送",
            close: "关闭",
            order_complete: "订单完成",
            view_transaction: "查看交易",
            message_for: " %{handle}的信息",
            provide_details: "提供其他详细信息，提出问题等（可选）",
            send: "发送"
        },
        receiveMoney: {
            share_address: "分享钱包地址",
            copy_address: "复制地址",
            address_copied: "地址已经复制"
        },
        restoreProfileInit: {
            restore_profile: "恢复用户数据",
            restore_hint: "选择您的Haven用户数据以恢复您的个人资料，包括您的钱包资金。",
            select_file: "选择文件"
        },
        restoreProfilePassword: {
            Ooops: "糟糕！",
            loading_hint: "这可能花费一分钟...",
            wrong_password: "密码错误!",
            failed_download: "下载zip文件失败",
            enter_password: "输入密码",
            password: "密码",
            enter_password_hint: "输入密码以继续，您在创建备份时设置此密码.",
            restore: "恢复"
        },
        Resync: {
            unknown_error: "未知错误!",
            resync_transactions: "同步交易",
            resync_content1: "如果您认为您缺少订单，或者您的订单详细信息与买方/卖方不同步，",
            resync_content2: "您可以重新扫描区块链以查找与您的订单相关的交易。",
            resync_content3: " 不需要频繁地重新同步交易。",
            resync_content4: "仅当您认为有问题时才应该这样做。 每次启动应用程序时都会执行一次扫描.",
            resync_content5: "重新同步过程处于活动状态时，您可以离开此视图.",
            resyncing: "同步中...",
            resync_info: "在 %{lastSyncedAgo} 同步过",
            resync: "重新同步"
        },
        searchFilter: {
            filter: "过滤",
            sortBy: "排序方式",
            accepts: "接受",
            ships_to: "快递到",
            rating: "评分",
            listing_type: "清单类型",
            item_condition: "商品情况",
            adult_content: "成人内容",
            adult_content2: "显示成人内容(18+)",
            filters_reset: "重启过滤"
        },
        searchResult: {
            Listings: "商品",
            User: "商户"
        },
        serverLog:{
            server_logs:"后台日志",
            details1:"您的后台日志有助于解决您遇到的问题及错误.",
            details2:"点击下面的按钮将提示您共享日志，请仅与您信任的人共享，避免公开发布，它们包含敏感信息.",
            share_server_log:"共享后台日志",
            share_ifps_log:"共享IPFS日志"
        },
        settings: {
            Settings: "设置"
        },
        shippingAddress: {
            are_you_sure: "您确定吗?",
            remove_address: "移除地址",
            cancel: "取消",
            OK: "好的",
            free: "免费",
            cannot_ship: "抱歉，该商品无法运送到所选地址",
            Shipping: "快递",
            done:"确定",
            ships_to: "快递到",
            no_address: "没有货运地址",
            add_address: "添加新地址"
        },
        shop: {
            Trending: "流行",
            Featured_stores: "特色商店",
            Featured_listings: "特色商品",
            Best_Sellers: "最佳商家",
            Gaming: "游戏",
            Munchies: "餐饮",
            Devices: "设备"
        },
        store: {
            Create_Listing: '创建商品', 
            Create_Post: '创建帖子',
            Share_to: '分享到...',
            Cancel: '取消'
        },
        StoreRatings: {
            Reviews: "评价",
            no_reviews1: "%{user} 没有收到任何评论",
            no_reviews2: "你还没有收到任何评价"
        },
        tagEditor: {
            tags: "标签",
            done: "完成",
            recent: "最近",
            remove_tag: '删除标签？',
            remove: "删除",
            cannot_undo: "你不能撤销该操作。",
            are_you_sure: '您确定吗？',
            unsaved_discard: '任何未保存的更改都将被丢弃。',
            ok: "好的",
            cancel: '取消'
        },
        wallet: {
            Wallet: "钱包",
            View_history: "查看交易历史",
            Cancel: "取消"
        },
        wishlist: {
            Wishlist: "收藏夹"
        },
        Me: {
            my_Profile :"个人信息",
            screenName1 :"店铺",
            wallet: "钱包",
            screenName2:"钱包",
            purchases: "购买",
            screenName3 :"订单",
            sales : "出售",
            screenName4 :"订单",
            wishlist:"收藏夹",
            screenName5:"收藏夹",
            settings:"设置",
            screenName6:"设置",
            notifications:"通知",
            screenName7:"通知",
            support:"支持",
            screenName8 :"支持",
            me :"我",
            support2:"支持",
            Description1:"有问题，功能建议或错误要报告吗？ 请先查看我们的常见问题解答。" ,
            Description2:"我们的电报组是报告错误或寻求支持的理想资源。",
            Description3:"我们提供电子邮件支持的能力非常有限." ,
            Description4:"请尝试主要使用FAQ或电报.",
            Description5:"对于市场上应用程序或内容的任何关键问题，疑虑或问题，请通过电子邮件与我们联系。",
            fAQs:"常见问答",
            discord: "Discord",
            telegram:"电报",
            email_Support:"邮箱"

        },
    },

    utils: {
        fee: {
            Super_Economic: '十分经济',
            Super_economic_v: '最便宜，最慢',
            Economic: '经济',
            Economic_v: '便宜，慢',
            Normal: '普通',
            Normal_v: '平均费用和等待时间',
            Priority: '优先',
            Priority_v: '最贵，最快'
        },
        listings: {
            Electronics: "电子商品",
            Women_Clothing: "女装",
            Men_Clothing: "男装",
            Toys_Games: "玩具与游戏",
            Jewelry: "珠宝",
            Tools: "工具",
            Gift_Cards: "礼品卡",
            Art: "艺术品"
        },
        notification: {
            you: "你",
            started_disputed: '开启了一个争议',
            proposed_dispute_outcome: '建议了一个争议结果',
            accepted_dispute_payout: '接受了争议支付',
            claimed_their_payment: '获取了付款',
            day: "天",
            days: "天",
            has_left: "还剩余%{days}%{daysLeft}天来建议一个争议结果",
            you_placed_order: "你下了订单",
            placed_order: ' 下了订单',
            your_payment_sent: '你的付款已发送',
            sent_payment: ' 已发送了付款',
            cancelled_your_order: ' 取消了你的订单',
            declined_order: '拒绝了这个订单',
            accepted_your_order: ' 接受了你的订单',
            accepted_order: '接受了这个订单',
            cancelled_this_order: '取消了这个订单',
            cancelled_their_order: ' 取消了订单',
            refunded_your_order: ' 已对你的订单退款',
            refunded_this_order: '已对这个订单退款',
            fulfilled_your_order: ' 完成了你的订单',
            fulfilled_order: '已完成了这个订单',
            completed_their_order: ' 结束了订单'
        },
        order: {
            order: "订单 %{id}"
        }
    }
}
