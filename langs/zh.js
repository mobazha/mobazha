export default {
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
            },
            PaymentMethod: {
                wallet_empty: '你的钱包是空的',
                add_funds: '添加资金'
            },
            PostButton: {
                post: '发送'
            },
            ProductPrice: {
                free_shipping: '免费运送',
                shipping: '运送',
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
                from: "从%{content}",
                no_review: "还没有收到买家的评论"
            },
            BuyWyre: {
                ask_crypto: "需要加密吗?",
                top_up: "用Wyre充值您的钱包!"
            },
            CheckoutNote: {
                add_note: "添加备注到订单上(可选)",
                add_seller_note: "给买家添加一条附注"
            },
            DirectPaymentOption: {
                direct_payment: "直接支付",
                description: "在没有仲裁者的情况下直接汇款给卖方请谨慎使用， 除非您完全信任卖家，否则请勿使用"
            },
            FeedItem: {
                reposted: "重新发布"
            },
            FeedPreview: {
                anonymous: "匿名"
            },
            ListingPaymentOptions: {
                not_accepted: "没有接受",
                payment_options: "支付选项"
            },
            ListingReview: {
                from: "从 %{name}",
                no_message: "买家没有留下评论"
            },
            ModerationFee: {
                percentage: "百分比 (%)",
                flat_fee: "法币费用 (%)",
                fee: "费用 ($)"
            },
            ProductDescription: {
                empty_text: "没有提供描述",
                read_more: "阅读更多"
            },
            ProductPolicy: {
                no_provided: "不提供%{policy} "
            },
            RadioModalFilter: {
                reason_reporting: "请输入报告此内容的原因.",
                other: "其他: 请解释",
            },
            SellerInfo: {
                about: "关于卖家",
                unknown: "未知",
                message: "消息",
                visit_store: "访问商店"
            },
            ShopCard: {
                unknown: "未知"
            },
            ShopInfo: {
                unknown: "未知",
                following: "关注中",
                followers: "关注者",
                edit_profile: "编辑文件",
                message: "消息",
            }
        },
        WalletCoinItem: {
            coming_soon: "快来"
        },

        organism: {
            AverageRating: {
                no_reviews: "还没有评论"
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
                each_price: "%{price} / each"
            },
            CheckoutSummary: {
                add_link: "添加",
                none_select: "无",
                remove_coupon: "移除优惠券?",
                remove_coupon_description: "你确定你想要移除优惠券?",
                cancel: "取消",
                remove: "移除",
                quantity_info: "质量: %{quantity}",
                coupon_info: "优惠券: %{info}",
                change: "Change",
                free: "免费",
                network_fee: "网络费用",
                fee_alert_description: "费用太高， 请使用较低的费用级别或其他硬币.",
                learn_more: "学习更多",
                total: "总共",
                calculating: "正在计算..."
            },
            CoinTypeSelector: {
                coming_soon: "快来"
            },
            DefaultInventoryItem: {
                no_title: "产品页面没有标题",
                sku: "SKU",
                sku_info: "SKU, ID, etc",
                quantity: "数量",
                unlimited: "没有限制的",
                quantity_sold_out: '如果数量达到0，它将显示为“已售完”。',
                quantity_unlimit: '消费者可以购买任意数量的商品.'
            },
            EmptyCoupons: {
                empty_coupon: "您尚未添加任何优惠券",
                add_coupon: "添加优惠券"
            },
            EmptyShippingMethods: {
                empty_shipping_option: "您尚未添加任何运输选项",
                add_shipping: "添加运输选项"
            },
            ErrorModal: {
                error_message: "错误: %{error}"
            },
            InventoryItem: {
                quantity_info: "QTY: %{quantity}",
                Unlimited: "没有限制的"
            },
            ItemDetail: {
                listing: "产品页面",
                type: "类型",
                title: "标题",
                ask_selling: "你售卖什么?",
                price: "价格",
                condition: "场景",
                description: "描述",
                description_hint: "在这里描述你的产品页面",
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
                select_info: "%{count} 筛选"
            },
            OptionSelector: {
            },
            OrderBrief: {
                tap_to: "当前市场价格, 点击 ",
                learn_more: "学习更多"
            },
            OrderDispute: {
                ask_payout: "接受付款?",
                payout_description: "一旦接受，争议将结束，资金将转移",
                cancel: "取消",
                ok: "OK",
                dispute_expired: "争议过期",
                memo_comment1: "仲裁员尚未提出结果。 卖方可以要求付款.",
                dispute_payout: "争议结果",
                will_be_issued: " 将发送给你.",
                moderator_takes: "仲裁者取得",
                seller_takes: "卖家取得 ",
                buyer_takes: "买家取得 ",
                accept_payout: "接受付款",
                started_by: "纠纷开始于 %{name}",
                the_seller: "卖家",
                the_buyer: "买家",
                memo_comment2: "主持人已介入以提供帮助。 开始聊天以提供更多详细信息.",
                message: "消息"
            },
            OrderFooter: {
                claim: "声称",

            }, OrderFulfillment: {
                no_tracking_number: "没有运单号码可以复制!",
                shipping_via: "运输方式",
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
                as_advertised: "如广告所示",
                Delivery: "运输",
                Service: "服务",
                no_feedback: "没有反馈留下来 %{name}"
            },
            PayPanel: {
                ask_pay: "你要如何付款?",
                external_wallet: "外部钱包",
                not_available_eth: "不适用于ETH",
                haven_wallet: "Haven 钱包",
                not_enough_funds: "没有足够的资金"
            },
            ProductRatings: {
                reviews: "评论",
                see_all_reviews: "查看所有 %{ratings.length} 评论",
                no_reviews_yet: "还没有评论"
            },
            QRScanner: {
                scan_qr_payment_address: "扫描付款地址的二维码",
                scan_qr_store: "Scan the QR code of a store,\na listing, or a payment address"
            },
            SearchFilterHeader: {
                results: "%{total} 结果"
            },
            SelectableModerator: {
                view_details: "查看详情"
            },
            SelectorModal: {

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
                add_option: "添加运输选项",
                options_count: "%{count} 运输选项",
                shipping: "运输",
            },
            ShippingPriceEditor: {
                shipping_service: "运输服务 #${pos}",
                delete: "删除",
                service: "服务",
                shipping_hint: "标准，特快等",
                Duration: "长短",
                Duration_hint: "5-7 天",
                price: "价格",
                additional_price: "额外的价格"
            },
            SingleVariantEditor: {
                variant_id: "多样的 %{id}",
                title: "标题",
                title_hint: "例如大小",
                description: "描述",
                description_hint: "例如产品尺寸",
                choices: "选项",
                choices_hint: "例如 大小 ,材料, "
            },
            SupportHaven: {
                support_haven: "支持 Haven",
                description: "Haven完全免费，并依靠您的支持来帮助开发。"
            },
            TagEditor: {
                tags: "标签",
                tags_info: "%{count} %{tag}",
                add_hint: "在你的商品列表上添加标签"
            },
            TagSuggestion: {
                none: "无"
            },
            PanelView: {
                PanelViewBase: {
                    cancel: "取消"
                }
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
                next: "下一个",
                recovery_phrase: "你的助记词",
                phrase_hint: "请按顺序写下每个单词",
                writedown_hint: "确保您的助记词安全。 如果您丢失或更换了手机，则可以使用它来重新获得资金的使用权；从不与任何人分享助记词。 为了安全起见，请避免截图，也不要将其存储在移动设备上。",
                done: "确定"
            },
            CategoryList: {
                more: "更多"
            },
            ChatDetail: {
                is_typing: "%{peer} 正在书写...",
                unread: "没有阅读",
                block_message: "这个用户已经被屏蔽.",
                start_with: "开始与。。会话",
                moderator_join: "(仲裁者) 加入了讨论",
                say_something: "说点好的..."
            },
            Checkout: {
                address_required: "*需要购买的送货地址",
                new_address: "新地址",
                cannot_ship: "此商品无法送到所选地址",
                shipping: "送货",
                payment_protection: "付款保护",
                protect_up_to: "尽到保护我的付款责任 ",
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

            }, CouponModal: {
                title_empty_alert: "折扣标题不能为空",
                code_empty_alert: "折扣代码不能为空",
                percentage_alert: "抱歉，该值必须在1到99之间。",
                value_empty_alert: "折扣数值不能空",
                exceed_alert: "抱歉，折扣超出了商品的价值。",
                edit_coupon: "编辑优惠券",
                new_coupon: "新优惠券",
                title: "标题",
                title_hint: "输入一个标题", 
                code: "代码",
                code_hint: "输入一个优惠券代码",
                discount: "折扣",
                discount_hint1: "例如. 10%",
                discount_hint2: "例如. $10",
                percent: "百分比"
            },
            CovidModal: {
                description11: "为了在这些困难时期保持安全，世界各地的人们，州和医院的基本用品都非常少。 如果您或您认识的任何人可以快速生产，采购或运输",
                description12: "口罩，N95口罩，外科口罩，洗手液，洗手液，呼吸机，温度计，湿纸巾，卫生纸",
                description13: " 等,请尽快让这些物品流通并运到正确的人手中.",
                description21: "如果我们全力以赴，就可以挽救生命。 世界需要您的支持，以帮助使重要物品流通。 如果您有权使用",
                description22: "缝纫机，3D打印机",
                description23: "，甚至",
                description24: " 酒厂",
                description25: "请考虑创建可以捐赠和/或以合理价格出售的物品和零件.",
                description31: "您的基本物品可以立即在港口免费分发，不需要账户。不问任何问题。请尽你的责任来帮忙。"
            },
            DisputeModal: {
                submit_dispute: "提出争议?",
                submit_hint: "仲裁者将介入以帮助解决争议。 您无法撤消此操作",
                cancel: "取消",
                ok: "Ok",
                enter_reason: "请输入争议的原因!",
                content_hint: "您为什么要提出争议？ 提供尽可能多的细节。"
            },
            EULAModal: {
                eula: "最终用户许可协议",
                privacy_description3: "End User License Agreement terms and conditions governing download and use of this mobile application, downloaded by you via Apple, Inc.’s (“Apple”) App Store (the “App Store”) or Google Play. Please read this End User License Agreement terms and conditions carefully",
                privacy_description2: "This End User License Agreement sets forth the terms and conditions (“Terms”) under which OB1 (“OB1”) (alternatively referred to as “us,” “we,” or “our”) offers you the right to download and use the Haven mobile application (including any updates thereto, the “Application”) and your use of the Application is governed by these Terms. By accepting these Terms (i) you represent that you are of legal age to enter into a binding contract and (ii) you signify that you have read, understood and agree to these Terms (and that such Terms are enforceable like any other written negotiated agreement signed by you) and certify that you are at least 17 years old or older. If you do not agree to these Terms, or you are not at least 17 years old, you may not use the Application. Violations of these Terms will result in a permanent removal from the Application.",
                privacy_description3: "These Terms constitute an agreement strictly between OB1 and you and you acknowledge that OB1 (in accordance with the limitations herein) rather than Apple and Google is responsible for any claim or liability arising from your use of the Application including, but not limited to, any third party claim of infringement of intellectual property rights. Nevertheless, you agree to abide by all terms, conditions or usage rules imposed by Apple and Google applicable to the use of this Application, including, but not limited to, any terms, conditions or usage rules set forth in the App Store Terms of Service.",
                privacy_description4: "1. License and Restrictions",
                privacy_description5: "The Application is licensed, not sold, to you. All rights, title and interest (including, without limitation, all copyrights, trademarks and other intellectual property rights) in and to this Application belong to us or our licensors. Subject to your compliance with these Terms, we grant you a non-transferable, non-assignable, revocable, limited license to download and install one copy of this Application on a mobile device that you personally own or control and to use that copy of this Application on that mobile device solely for your own personal use. You may not install or use a copy of the Application on a device you do not own or control. You may not distribute or make the Application available over a network where it could be used by multiple devices at the same time. You may not sell, rent, lend, lease, redistribute, or sublicense the Application or circumvent any technical limitations in the Application or otherwise interfere in any manner with the operation of the Application, or the hardware or network used to operate the Application. You may not copy, reverse engineer, decompile, disassemble, modify, create derivative works or otherwise attempt to derive the source code of this Application. This Application and its content are protected by copyright under both United States and foreign laws. Any use of the Application and its content not explicitly permitted by these Terms is a breach of this agreement and may violate the law. If you violate these Terms, your license to use this Application automatically terminates and you must immediately cease using the Application and destroy all copies, full or partial, of the Application.",
                privacy_description6: "2. Ownership",
                privacy_description7: "We alone (and our licensors, where applicable) shall own all right, title and interest, including, without limitation, all intellectual property rights, in and to the Application and any suggestions, ideas, enhancement requests, feedback, recommendations or other information provided by you or any other party relating to the Application. Any copy, modification, revision, enhancement, adaptation, translation, or derivative work of or created from the Application shall be owned solely and exclusively by us, and/or, as applicable, our third-party vendors, as shall any and all patent rights, copyrights, trade secret rights, trademark rights, and all other proprietary rights, worldwide therein and thereto, and you hereby assign to OB1 any and all of your rights, title or interests that you may have or obtain in the Application or any modification to or derivative work of the Application. You shall not remove or authorize or permit any third party to remove any proprietary rights legend from the Application.",

            }, feed: {
                not_post: "%{name} 还没有发布任何信息",
                post_hint1: "你还没有发布任何信息.",
                post_hint2: "在社区分享一些东西!",
                reported: "报告"
            },
            FeedDetail: {
                fail_to_load: "糟糕！这个发布装载失败.",
                retry: "重试",
                Loading: "装载.....",
                reported: "报告"
            },
            FulfillModal: {
                shipping_carrier: "货运公司",
                carrier_hint: "USPS, FedEX, etc",
                tracking_number: "快递单号",
                tracking_number_hint: "快递单号码",
                file_url: "文件链接",
                password: "密码",
                password_hint: "可选",
                note: "备注",
                note_hint: "可选",
                add_a_note: "添加备注（可选）"
            },
            GlobalFeed: {
                customise_feed: "按照一些配置文件自定义！",
                not_found: "什么都没有找到",
                share_with_community: "在社区分享一些东西!",
                create_post: "创建一个社交点",
                reported: "报告"
            },

            InfiniteProducts: {
                loading_listings: "正在加载商品页面....."
            },
            InventoryEditor: {
                details: "详细",
                surcharge: "附加费用",
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
                refunds_description: "您的退货政策是什么？ 退货接受多长时间？ 谁为退货付款？",
                terms: "条款和条件",
                terms2: "T&Cs",
                terms_description: "上市的条款和条件是什么？ 您作为供应商负责什么？ 有保修吗？"
            },
            ListingsTab: {
                loading: "装载.....",
                no_sale: "目前无任何出售。",
                check_later: "请稍后再检查！",
                store_empty: "你的店铺是空的",
                put_for_sale: "布置一些商品出售！",
                create_listing: "建立商品页面",

            },
            NeedCoin: {
                coinbase: "Coinbase",
                cryptocurrency_exchange: "加密货币交换"
            },
            OrderState: {
                no_orders: "没有发现订单",
            },

            OrderSummary: {
                oops: "糟糕!",
                dispute_pending_alert: "订单仍待处理时，您无法提出争议。",
                dispute_not_fulfilled_alert: "您必须先完成订单，才能提出争议",
                dispute_cancel_alert: "您无法对已取消的订单提出争议.",
                dispute_refund_alert: "您无法对退款订单提出争议.",
                dispute_resolved_alert: "您无法针对已结案的订单提出争议。",
                dispute_completed_alert: "您无法对已完成的订单提出争议.",
                dispute_finalized_alert: "此订单无异议。 卖方已要求为此订单付款。",
                dispute_processing_alert: "此订单无异议。 请取消您的订单以全额退款。",
                quantity_info: "数量: {quantity}",
                view: "查看",
                view_transaction: "查看交易",
                payment: "支付",
                no_payment: "尚未找到此订单的付款。 最多可能需要一分钟才能检测到付款。",
                cannot_dispute: "资金直接发送给 %{user}. 您不能对此订单提出争议。",
                escrow_released: "资金已从托管处释放。 订单不再有争议。",
                order_in_dispute: "该订单有争议 ",
                until_accept: "或直到一方接受付款为止。",
                period_expired_claim: "争议期已过。 卖方现在可以要求付款。",
                period_expired_claim2: "争议期已过。 现在可以全额领取资金.",
                order_in_escrow1: " 订单资金正在第三方托管",
                order_in_escrow2: "或直到买家完成订单为止。如果您对此订单有任何疑问，可以与主持人提出争议.",
                dispute_order: "订单争议",
                claim_payment: "索偿",
                dispute_error_possible: "处理此订单时发生错误。 请提出争议以收回您的资金.",
                dispute_error: "处理此订单时发生错误。",
                order_refunded: "订单已经退款",
                full_refund: "卖方已对此订单全额退款",
                order_completed: "订单完成",
                release_to_seller: "资金已发放给卖方",
                dispute_closed: "争议关闭",
                dispute_closed_info: "%{user} 已接受付款。 现在该纠纷已经结案。",
                payment_claimed: "已付款",
                seller_claim: "卖方已要求为此订单付款。",
                order_canceled: "订单取消",
                user_cancel_order: " %{user} 已经取消订单. 资金已经全额退还",
                period_expired: "争议期限已过",
                no_dispute: "在45天的争议期内，没有任何争议。 卖方现在可以要求付款。",
                Shipping: "运输",
                no_buyer_note: "买家没有留下附注",
                address_copied: "地址已经复制"
            },
            PurchaseState: {
                thank_you: "谢谢你!",
                order_placed: "您的订单已下达。 您可以随时跟踪或管理您的订单。",
                processing: "处理中...",
                hang_tight: "等一下, 这可能需要一分钟的时间。",
                Uh_oh: "Uh oh!",
                transaction_failed: "您的交易失败, 请再试一次.",
                retry: "重试",
                order_details: "订单详情",
                error: "错误:"
            },
            RatingModal: {
                Overall: "整体",
                Quality: "质量",
                As_advertised: "如广告所示",
                Delivery: "运输",
                Service: "服务",
                Write_a_review: "在这里写下评论",
                Post_anonymously: "匿名发送"
            },
            ReportTemplate: {
                Ooops: "糟糕！",
                enter_reason: "请输入报告此内容的原因。",
                why_report_profile: "您为什么要举报此个人资料？",
                why_report: "你为什么要举报?",
                next: "下一个",
                submit: "Submit",
                describe_issue: "请描述问题（可选）",
                provide_details: "Provide as much details as possible"
            },
            SearchResults: {
                loading_results: "正在加载搜索结果...",
                no_found: "没有结果.",

            },
            SendMoney: {
                NEXT: "下一个",
                send_to: "发送到",
                paste_or_scan: "请扫描二维码地址"
            },
            SendReceiveMoney: {
                Receive: "接收",
                Send: "Send"
            },
            Settings: {
                are_you_sure: "你确定吗?",
                check_backup: "您是否备份了当前商店？",
                cancel: "取消",
                OK: "OK",
                profile: "个人资料",
                currency: "现金",
                shipping_address: "邮寄地址",
                blocked: "阻止",
                notifications: "通知事项",
                push_notifications: "推送通知事项",
                Policies: "政策",
                Moderators: "仲裁者",
                coins_accepted: "接收货币",
                Advanced: "高级",
                Analytics: "分析",
                On: "开",
                Off: "关",
                Backup_wallet: "备份钱包",
                Backup_profile: "备份文件",
                Restore_profile: "同步文件",
                Resync_transactions: "同步交易",
                Server_Log: "服务日志",
                Version: "Version 1.3.7"
            },
            StoreModeratorList: {
                moderators_count: "%{count} 仲裁者",
                moderators_added: "新的仲裁者会自动添加到您的商店"
            },
            Toast: {
                post_created: "帖子已经创建",
                view: "查看"
            },
            TransactionHistory: {
                no_transaction_recorded: "尚无交易记录",
                no_transactions: "还没有交易",
                notes: "请注意，某些付款可能不会显示在交易记录中。 但是，总余额反映了所有已发送和已接收的交易."
            },
            UserSearchResults: {
                loading_results: "加载搜索结果 ...",
                no_results: "没有结果."
            },
            wishlist: {
                wishlist_empty: "你的愿望清单是空的"
            }
        }
    },

    screens: {
        acceptedCoins: {
            update_listings: "更新商品页面?",
            sure_about_update: "您确定所有列表都会更新吗？",
            cancel: "取消",
            OK: "OK",
            coins_accepted: "接受货币",
            save: "保存",
            clear_all: "全部清除"
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
            next: "下一个"
        },
        backupProfilePassword: {
            password_empty: "Password empty",
            password_empty_hint: "Please set a password",
            password_mismatch: "Password mismatch",
            password_mismatch_hint: "Please set a correct password",
            take_a_minute: "It might take a minute...",
            set_password: "Set a password",
            password: "密码",
            confirm: "Confirm",
            confirm_password: "Confirm password",
            hint1: "Set a password and ",
            hint2: "make sure to write it down.",
            hint3: "\nYou\'ll need your password to restore your profile.",
            next: "下一个"
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
            new_address: "新地址",
            done: "确定",
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
            address_copied: "地址已经复制",
            amount_copied: "Amount copied!",
            pay_order: "Pay to complete your order",
            copy_address: "复制地址"
        },
        externalStore: {
            unblock_user: "Unblock this user to see their content",
            loading: "装载.....",
            failed_load: "Oops! This profile failed to load.",
            reported: "报告",

        },
        followers: {
            followers: "Followers",
            no_followers1: "%{name} doesn't have any followers",
            no_followers2: "You don\'t have any followers"
        },
        followings: {
            Following: "关注中",
            no_following1: "%{name} isn't following anyone",
            no_following2: "You are not following anyone"
        },
        listing: {
            are_you_sure: "你确定吗?",
            ask_block: "Block this user?",
            cancel: "取消",
            OK: "OK",
            ask_delete: "Delete listing?",
            delete_hint: "You can't undo this action.",
            cancel: "取消",
            remove: "Remove",
            failed_load: "Ooops! This listing failed to load.",
            retry: "重试",
            loading: "装载.....",
            add_wishlist: "Added to Wishlist!",
            remove_wishlist: "Removed from Wishlist!",
            reported: "报告!"
        },

        listingAdvancedOptions: {
            add_coupons: "Add coupons",
            advanced: "Advanced",
            Variants_Inventory: "Variants & Inventory",
            add_hint: "Add variants and manage your store inventory",
            store_policies: "店铺规则",
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
            cancel: "取消",
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
            currency: "现金",
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
            done: "确定",
            transaction_speed: "Transaction speed"
        },
        paymentSuccess: {
            transaction_details: "Transaction details",
            processing: "Processing…",
            hang_tight: "Hang tight! This may take up to a minute.",
            Uh_oh: "Uh oh!",
            failed_go_through: "Your transaction failed to go through. Please try again.",
            retry: "重试",
            error: "错误:"
        },

        policies: {
            store_policies: "店铺规则",
            save: "保存",
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
            cancel: "取消",
            I_accept: "I Accept"
        },
        ProductRatings: {
            reviews: "Reviews",
            No_reviews: "还没有评论"
        },
        profileSettings: {
            warning: "Warning",
            warning_info: "If you go back, you will lose your progress",
            Cancel: "取消",
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
            sent: "发送",
            close: "Close",
            order_complete: "Order Complete",
            view_transaction: "View Transaction",
            message_for: "Message for %{handle}",
            provide_details: "Provide additional details, ask a question, etc (optional)",
            send: "Send"
        },
        receiveMoney: {
            share_address: "分享钱包地址",
            copy_address: "复制地址",
            address_copied: "地址已经复制"
        },
        restoreProfileInit: {
            restore_profile: "Restore profile",
            restore_hint: "Select your haven backup file to restore\nyour profile, including your wallet funds.",
            select_file: "SELECT FILE"
        },
        restoreProfilePassword: {
            Ooops: "糟糕！",
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
            are_you_sure: "你确定吗?",
            remove_address: "Remove the address",
            cancel: "取消",
            OK: "OK",
            free: "FREE",
            cannot_ship: "Sorry, this item can not be shipped to the selected address",
            Shipping: "运输",
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
