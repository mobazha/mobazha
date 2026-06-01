package events

type Notification struct {
	ID  string `json:"notificationID"`
	Typ string `json:"type"`
}

type Thumbnail struct {
	Tiny  string `json:"tiny"`
	Small string `json:"small"`
}

type ListingPrice struct {
	Amount        string  `json:"amount"`
	CurrencyCode  string  `json:"currencyCode"`
	PriceModifier float32 `json:"priceModifier"`
}

type NewOrder struct {
	Notification
	BuyerName   string       `json:"buyerName"`
	BuyerID     string       `json:"buyerID"`
	BuyerAvatar string       `json:"buyerAvatar,omitempty"`
	ListingType string       `json:"listingType"`
	OrderID     string       `json:"orderID"`
	Price       ListingPrice `json:"price"`
	Slug        string       `json:"slug"`
	Thumbnail   Thumbnail    `json:"thumbnail"`
	Title       string       `json:"title"`
}

type OrderFunded struct {
	Notification
	TenantID    string       `json:"tenantID,omitempty"`
	BuyerName   string       `json:"buyerName"`
	BuyerID     string       `json:"buyerID"`
	BuyerAvatar string       `json:"buyerAvatar,omitempty"`
	ListingType string       `json:"listingType"`
	OrderID     string       `json:"orderID"`
	Price       ListingPrice `json:"price"`
	Slug        string       `json:"slug"`
	Thumbnail   Thumbnail    `json:"thumbnail"`
	Title       string       `json:"title"`
}

type OrderPaymentReceived struct {
	Notification
	TenantID     string `json:"tenantID,omitempty"`
	OrderID      string `json:"orderID"`
	FundingTotal string `json:"fundingTotal"`
	CoinType     string `json:"coinType"`
}

type PaymentSentReceived struct {
	OrderID string `json:"orderID"`
	Txid    string `json:"transactionID"`
}

type PaymentLockedReceived struct {
	Notification
	OrderID         string    `json:"orderID"`
	BuyerName       string    `json:"buyerName"`
	BuyerID         string    `json:"buyerID"`
	BuyerAvatar     string    `json:"buyerAvatar,omitempty"`
	Slug            string    `json:"slug"`
	Thumbnail       Thumbnail `json:"thumbnail"`
	Title           string    `json:"title"`
	TransactionHash string    `json:"transactionHash"`
	Coin            string    `json:"coin"`
	Amount          string    `json:"amount"`
	ExpiresAt       uint64    `json:"expiresAt"`
}

type PaymentExpiredNotification struct {
	Notification
	OrderID   string    `json:"orderID"`
	Thumbnail Thumbnail `json:"thumbnail"`
	Amount    string    `json:"amount"`
	Coin      string    `json:"coin"`
}

type PaymentCancelledByBuyer struct {
	Notification
	OrderID     string    `json:"orderID"`
	Thumbnail   Thumbnail `json:"thumbnail"`
	BuyerName   string    `json:"buyerName"`
	BuyerID     string    `json:"buyerID"`
	BuyerAvatar string    `json:"buyerAvatar,omitempty"`
	Amount      string    `json:"amount"`
	Coin        string    `json:"coin"`
}

type RatingSignaturesReceived struct {
	OrderID string `json:"orderID"`
}

type OrderConfirmation struct {
	Notification
	OrderID      string    `json:"orderID"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	VendorName   string    `json:"vendorName"`
	VendorID     string    `json:"vendorID"`
	VendorAvatar string    `json:"vendorAvatar,omitempty"`
}

type OrderDeclined struct {
	Notification
	OrderID      string    `json:"orderID"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	VendorName   string    `json:"vendorName"`
	VendorID     string    `json:"vendorID"`
	VendorAvatar string    `json:"vendorAvatar,omitempty"`
}

type OrderCancel struct {
	Notification
	OrderID     string    `json:"orderID"`
	Thumbnail   Thumbnail `json:"thumbnail"`
	BuyerName   string    `json:"buyerName"`
	BuyerID     string    `json:"buyerID"`
	BuyerAvatar string    `json:"buyerAvatar,omitempty"`
}

type OrderExpired struct {
	Notification
	OrderID      string    `json:"orderID"`
	Reason       string    `json:"reason"`
	BuyerName    string    `json:"buyerName,omitempty"`
	BuyerID      string    `json:"buyerID,omitempty"`
	BuyerAvatar  string    `json:"buyerAvatar,omitempty"`
	VendorName   string    `json:"vendorName,omitempty"`
	VendorID     string    `json:"vendorID,omitempty"`
	VendorAvatar string    `json:"vendorAvatar,omitempty"`
	Thumbnail    Thumbnail `json:"thumbnail,omitempty"`
	Title        string    `json:"title,omitempty"`
}

type OrderStaleWarning struct {
	Notification
	OrderID      string    `json:"orderID"`
	State        string    `json:"state"`
	StuckFor     string    `json:"stuckFor"`
	BuyerName    string    `json:"buyerName,omitempty"`
	BuyerID      string    `json:"buyerID,omitempty"`
	BuyerAvatar  string    `json:"buyerAvatar,omitempty"`
	VendorName   string    `json:"vendorName,omitempty"`
	VendorID     string    `json:"vendorID,omitempty"`
	VendorAvatar string    `json:"vendorAvatar,omitempty"`
	Thumbnail    Thumbnail `json:"thumbnail,omitempty"`
	Title        string    `json:"title,omitempty"`
}

type Refund struct {
	Notification
	OrderID      string    `json:"orderID"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	VendorName   string    `json:"vendorName"`
	VendorID     string    `json:"vendorID"`
	VendorAvatar string    `json:"vendorAvatar,omitempty"`
}

type OrderShipment struct {
	Notification
	OrderID      string    `json:"orderID"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	VendorName   string    `json:"vendorName"`
	VendorID     string    `json:"vendorID"`
	VendorAvatar string    `json:"vendorAvatar,omitempty"`
}

type OrderCompletion struct {
	Notification
	OrderID     string    `json:"orderID"`
	Thumbnail   Thumbnail `json:"thumbnail"`
	BuyerName   string    `json:"buyerName"`
	BuyerID     string    `json:"buyerID"`
	BuyerAvatar string    `json:"buyerAvatar,omitempty"`
}

type DisputeOpen struct {
	Notification
	OrderID        string    `json:"orderID"`
	Thumbnail      Thumbnail `json:"thumbnail"`
	DisputerID     string    `json:"disputerID"`
	DisputerName   string    `json:"disputerName"`
	DisputerAvatar string    `json:"disputerAvatar,omitempty"`
	DisputeeID     string    `json:"disputeeID"`
	DisputeeName   string    `json:"disputeeName"`
	DisputeeAvatar string    `json:"disputeeAvatar,omitempty"`
}

type CaseOpen struct {
	Notification
	CaseID         string    `json:"caseID"`
	Thumbnail      Thumbnail `json:"thumbnail"`
	DisputerID     string    `json:"disputerID"`
	DisputerName   string    `json:"disputerName"`
	DisputerAvatar string    `json:"disputerAvatar,omitempty"`
	DisputeeID     string    `json:"disputeeID"`
	DisputeeName   string    `json:"disputeeName"`
	DisputeeAvatar string    `json:"disputeeAvatar,omitempty"`
}

type CaseUpdate struct {
	Notification
	CaseID         string    `json:"caseID"`
	Thumbnail      Thumbnail `json:"thumbnail"`
	DisputerID     string    `json:"disputerID"`
	DisputerName   string    `json:"disputerName"`
	DisputerAvatar string    `json:"disputerAvatar,omitempty"`
	DisputeeID     string    `json:"disputeeID"`
	DisputeeName   string    `json:"disputeeName"`
	DisputeeAvatar string    `json:"disputeeAvatar,omitempty"`
}

type DisputeClose struct {
	Notification
	OrderID          string    `json:"orderID"`
	Thumbnail        Thumbnail `json:"thumbnail"`
	OtherPartyID     string    `json:"otherPartyID"`
	OtherPartyName   string    `json:"otherPartyName"`
	OtherPartyAvatar string    `json:"otherPartyAvatar,omitempty"`
	Buyer            string    `json:"buyer"`
	BuyerRefunded    bool      `json:"buyerRefunded"`
}

type DisputeAccepted struct {
	Notification
	OrderID          string    `json:"orderID"`
	Thumbnail        Thumbnail `json:"thumbnail"`
	OtherPartyID     string    `json:"otherPartyID"`
	OtherPartyName   string    `json:"otherPartyName"`
	OtherPartyAvatar string    `json:"otherPartyAvatar,omitempty"`
	Buyer            string    `json:"buyer"`
}

type VendorDisputeTimeout struct {
	OrderID   string    `json:"purchaseOrderID"`
	ExpiresIn uint      `json:"expiresIn"`
	Thumbnail Thumbnail `json:"thumbnail"`
}

type BuyerDisputeTimeout struct {
	OrderID   string    `json:"orderID"`
	ExpiresIn uint      `json:"expiresID"`
	Thumbnail Thumbnail `json:"thumbnail"`
}

type BuyerDisputeExpiry struct {
	OrderID   string    `json:"orderID"`
	ExpiresIn uint      `json:"expiresIn"`
	Thumbnail Thumbnail `json:"thumbnail"`
}

type VendorFinalizedPayment struct {
	Notification
	OrderID string `json:"orderID"`
}

type ModeratorDisputeExpiry struct {
	CaseID    string    `json:"disputeCaseID"`
	ExpiresIn uint      `json:"expiresIn"`
	Thumbnail Thumbnail `json:"thumbnail"`
}

// CancelablePaymentReady is emitted when a CANCELABLE payment is ready to be auto-confirmed
// This is triggered when the seller receives PAYMENT_SENT for a CANCELABLE payment
// Works for all chain types: UTXO, EVM, and Solana
// Handled by dispatchCancelablePayment() in payment_dispatcher.go, which routes to:
// - UTXO chains: handleCancelablePaymentForUTXO → releases via multisig
// - EVM chains: handleCancelablePaymentForEVM → releases via platform relay API
// - Solana chains: (future) will use similar relay pattern
type CancelablePaymentReady struct {
	TenantID      string `json:"tenantID"`
	OrderID       string `json:"orderID"`
	TransactionID string `json:"transactionID"`
	Coin          string `json:"coin"`
	Amount        uint64 `json:"amount"`
}

// FiatPaymentReady is emitted when a fiat payment (Stripe/PayPal) capture succeeds.
// Triggers OrderAutoConfirmRequest so the order transitions to confirmed.
// No on-chain escrow involved — funds are held by the payment processor.
type FiatPaymentReady struct {
	OrderID    string `json:"orderID"`
	ProviderID string `json:"providerID"`
	SessionID  string `json:"sessionID"`
}

// RwaInstantBuyCompleted is emitted when an RWA instant buy (atomic swap) has completed on-chain.
// This is triggered when the seller receives PAYMENT_SENT with method=RWA_INSTANT_BUY.
// The atomic swap has already transferred tokens, so this just triggers the order confirmation.
// Handled by OrderAppService.handleRwaAutoComplete() in order_app_service_events.go.
type RwaInstantBuyCompleted struct {
	OrderID       string `json:"orderID"`
	TransactionID string `json:"transactionID"`
	Coin          string `json:"coin"`
}

// PartialPaymentReceived is emitted when buyer's payment is less than expected
// Frontend should refresh QR code to show remaining amount
type PartialPaymentReceived struct {
	OrderID         string `json:"orderID"`
	PaidAmount      uint64 `json:"paidAmount"`
	ExpectedAmount  uint64 `json:"expectedAmount"`
	RemainingAmount uint64 `json:"remainingAmount"`
	Coin            string `json:"coin"`
	PaymentAddress  string `json:"paymentAddress"`
}

// ExcessPaymentRefunded is emitted when an excess payment is automatically refunded
// This happens when buyer sends additional payment after PaymentSent was already sent
type ExcessPaymentRefunded struct {
	OrderID        string `json:"orderID"`
	RefundTxID     string `json:"refundTxID"`
	RefundedAmount uint64 `json:"refundedAmount"`
	Coin           string `json:"coin"`
}

// PaymentVerificationExpired is emitted when a PaymentSent transaction cannot be
// verified on-chain within the allowed window (48h). The vendor should inspect the
// order and decide whether to cancel or manually re-verify.
type PaymentVerificationExpired struct {
	OrderID       string `json:"orderID"`
	TransactionID string `json:"transactionID"`
	Coin          string `json:"coin"`
	Reason        string `json:"reason"` // "timeout" or "address_mismatch"
}

// ── S6 Buyer-protection lifecycle events ──────────────────────────────

// OrderAutoCompleted is emitted when a shipped order is automatically completed
// after the protection period expires without buyer action.
type OrderAutoCompleted struct {
	OrderID      string    `json:"orderID"`
	Reason       string    `json:"reason,omitempty"` // "protection_expired" | "unshipped_cancelable"
	BuyerName    string    `json:"buyerName"`
	BuyerID      string    `json:"buyerID"`
	BuyerAvatar  string    `json:"buyerAvatar"`
	VendorName   string    `json:"vendorName"`
	VendorID     string    `json:"vendorID"`
	VendorAvatar string    `json:"vendorAvatar"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	Title        string    `json:"title"`
}

// OrderAutoCancelled is emitted when an order is automatically cancelled because
// the vendor did not ship within maxShipDays after payment.
type OrderAutoCancelled struct {
	OrderID      string    `json:"orderID"`
	Reason       string    `json:"reason"` // "shipment_overdue"
	BuyerName    string    `json:"buyerName"`
	BuyerID      string    `json:"buyerID"`
	BuyerAvatar  string    `json:"buyerAvatar"`
	VendorName   string    `json:"vendorName"`
	VendorID     string    `json:"vendorID"`
	VendorAvatar string    `json:"vendorAvatar"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	Title        string    `json:"title"`
}

// OrderProtectionReminder is emitted as a countdown warning before an order
// auto-completes or auto-cancels.
type OrderProtectionReminder struct {
	OrderID       string    `json:"orderID"`
	Type          string    `json:"type"` // "auto_complete" or "auto_cancel"
	DaysRemaining int       `json:"daysRemaining"`
	BuyerName     string    `json:"buyerName"`
	BuyerID       string    `json:"buyerID"`
	BuyerAvatar   string    `json:"buyerAvatar"`
	VendorName    string    `json:"vendorName"`
	VendorID      string    `json:"vendorID"`
	VendorAvatar  string    `json:"vendorAvatar"`
	Thumbnail     Thumbnail `json:"thumbnail"`
	Title         string    `json:"title"`
}

// AfterSaleDisputeOpened is emitted when a buyer opens a dispute on a
// COMPLETED order within the after-sale window. Unlike on-chain disputes,
// after-sale disputes are application-level: funds are already released, so
// resolution depends on seller voluntary refund or platform mediation.
type AfterSaleDisputeOpened struct {
	Notification
	OrderID      string    `json:"orderID"`
	Reason       string    `json:"reason"` // e.g. "NOT_RECEIVED", "NOT_AS_DESCRIBED", "QUALITY_ISSUE", "OTHER"
	Description  string    `json:"description"`
	BuyerName    string    `json:"buyerName"`
	BuyerID      string    `json:"buyerID"`
	BuyerAvatar  string    `json:"buyerAvatar"`
	VendorName   string    `json:"vendorName"`
	VendorID     string    `json:"vendorID"`
	VendorAvatar string    `json:"vendorAvatar"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	Title        string    `json:"title"`
}

// AfterSaleDisputeReceived is emitted on the seller's node when a P2P
// after-sale dispute message is received from the buyer.
type AfterSaleDisputeReceived struct {
	Notification
	OrderID     string    `json:"orderID"`
	Reason      string    `json:"reason"`
	Description string    `json:"description"`
	BuyerName   string    `json:"buyerName"`
	BuyerID     string    `json:"buyerID"`
	Thumbnail   Thumbnail `json:"thumbnail"`
	Title       string    `json:"title"`
}

// ── Internal domain events (PaymentAppService → OrderAppService) ──────

// OrderAutoConfirmRequest is emitted by PaymentAppService when a CANCELABLE
// payment should be auto-confirmed (UTXO or EVM). OrderAppService subscribes
// and calls ConfirmOrder. This replaces the direct cross-service method call.
type OrderAutoConfirmRequest struct {
	TenantID      string `json:"tenantID,omitempty"`
	OrderID       string `json:"orderID"`
	TxID          string `json:"txID"`
	PayoutAddress string `json:"payoutAddress"`
}

// PaymentVerified is the canonical "this order's chain payment has been
// verified" signal emitted by the monitor-driven AggregatingVerifier when
// confirmed observations sum to ≥ the expected amount.
//
// Internal-only: it is the trigger that downstream services (OrderAppService,
// notifications) listen on to advance the order FSM and produce user-facing
// notifications such as OrderFunded. It deliberately carries no order
// metadata — subscribers re-load the order to see the freshly-saved
// SerializedPaymentSent envelope, OverpaidAmount, TotalReceived and
// PaymentVerificationStatus committed in the same DB transaction that
// emitted this event.
//
// Idempotency contract: AggregatingVerifier only emits this event on the
// first transition from pending → verified. Subsequent re-aggregations of
// the same order (e.g. additional observation rows landing) update
// TotalReceived / OverpaidAmount in place but do NOT re-emit, so
// subscribers may treat receipt as a one-shot "go" signal per order.
type PaymentVerified struct {
	TenantID string `json:"tenantID,omitempty"`
	OrderID  string `json:"orderID"`
}

// UTXOPaymentDetected is emitted by PaymentAppService when a buyer's UTXO
// payment reaches the expected amount. OrderAppService subscribes and calls
// ProcessOrderPayment. This replaces the direct cross-service method call.
type UTXOPaymentDetected struct {
	OrderID          string `json:"orderID"`
	TransactionID    string `json:"transactionID"`
	Coin             string `json:"coin"`
	Method           int32  `json:"method"`
	Amount           uint64 `json:"amount"`
	ToAddress        string `json:"toAddress"`
	ToID             []byte `json:"toID,omitempty"`
	Timestamp        int64  `json:"timestamp"`
	Script           string `json:"script"`
	PayerAddress     string `json:"payerAddress"`
	RefundAddress    string `json:"refundAddress"`
	Moderator        string `json:"moderator"`
	ModeratorAddress string `json:"moderatorAddress"`
	UnlockHours      uint32 `json:"unlockHours"`
}
