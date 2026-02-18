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
	BuyerHandle string       `json:"buyerHandle"`
	BuyerID     string       `json:"buyerID"`
	ListingType string       `json:"listingType"`
	OrderID     string       `json:"orderID"`
	Price       ListingPrice `json:"price"`
	Slug        string       `json:"slug"`
	Thumbnail   Thumbnail    `json:"thumbnail"`
	Title       string       `json:"title"`
}

type OrderFunded struct {
	Notification
	BuyerHandle string       `json:"buyerHandle"`
	BuyerID     string       `json:"buyerID"`
	ListingType string       `json:"listingType"`
	OrderID     string       `json:"orderID"`
	Price       ListingPrice `json:"price"`
	Slug        string       `json:"slug"`
	Thumbnail   Thumbnail    `json:"thumbnail"`
	Title       string       `json:"title"`
}

type OrderPaymentReceived struct {
	Notification
	OrderID      string `json:"orderID"`
	FundingTotal string `json:"fundingTotal"`
	CoinType     string `json:"coinType"`
}

type PaymentSentReceived struct {
	OrderID string `json:"orderID"`
	Txid    string `json:"transactionID"`
}

// PaymentLockedReceived is emitted when the vendor receives a PaymentLocked message
// for RWA confirm-required orders. The buyer has locked payment in the escrow contract.
type PaymentLockedReceived struct {
	Notification
	OrderID         string    `json:"orderID"`
	BuyerHandle     string    `json:"buyerHandle"`
	BuyerID         string    `json:"buyerID"`
	Slug            string    `json:"slug"`
	Thumbnail       Thumbnail `json:"thumbnail"`
	Title           string    `json:"title"`
	TransactionHash string    `json:"transactionHash"`
	Coin            string    `json:"coin"`
	Amount          string    `json:"amount"`
	ExpiresAt       uint64    `json:"expiresAt"`
}

// PaymentExpiredNotification is emitted when an escrow payment expires
// and funds are returned to the buyer
type PaymentExpiredNotification struct {
	Notification
	OrderID   string    `json:"orderID"`
	Thumbnail Thumbnail `json:"thumbnail"`
	Amount    string    `json:"amount"`
	Coin      string    `json:"coin"`
}

// PaymentCancelledByBuyer is emitted when buyer cancels an escrow payment
type PaymentCancelledByBuyer struct {
	Notification
	OrderID     string    `json:"orderID"`
	Thumbnail   Thumbnail `json:"thumbnail"`
	BuyerHandle string    `json:"buyerHandle"`
	BuyerID     string    `json:"buyerID"`
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
	VendorHandle string    `json:"vendorHandle"`
	VendorID     string    `json:"vendorID"`
}

type OrderDeclined struct {
	Notification
	OrderID      string    `json:"orderID"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	VendorHandle string    `json:"vendorHandle"`
	VendorID     string    `json:"vendorID"`
}

type OrderCancel struct {
	Notification
	OrderID     string    `json:"orderID"`
	Thumbnail   Thumbnail `json:"thumbnail"`
	BuyerHandle string    `json:"buyerHandle"`
	BuyerID     string    `json:"buyerID"`
}

type Refund struct {
	Notification
	OrderID      string    `json:"orderID"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	VendorHandle string    `json:"vendorHandle"`
	VendorID     string    `json:"vendorID"`
}

type OrderFulfillment struct {
	Notification
	OrderID      string    `json:"orderID"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	VendorHandle string    `json:"vendorHandle"`
	VendorID     string    `json:"vendorID"`
}

type OrderCompletion struct {
	Notification
	OrderID     string    `json:"orderID"`
	Thumbnail   Thumbnail `json:"thumbnail"`
	BuyerHandle string    `json:"buyerHandle"`
	BuyerID     string    `json:"buyerID"`
}

type DisputeOpen struct {
	Notification
	OrderID        string    `json:"orderID"`
	Thumbnail      Thumbnail `json:"thumbnail"`
	DisputerID     string    `json:"disputerID"`
	DisputerHandle string    `json:"disputerHandle"`
	DisputeeID     string    `json:"disputeeID"`
	DisputeeHandle string    `json:"disputeeHandle"`
}

type CaseOpen struct {
	Notification
	CaseID         string    `json:"caseID"`
	Thumbnail      Thumbnail `json:"thumbnail"`
	DisputerID     string    `json:"disputerID"`
	DisputerHandle string    `json:"disputerHandle"`
	DisputeeID     string    `json:"disputeeID"`
	DisputeeHandle string    `json:"disputeeHandle"`
}

type CaseUpdate struct {
	Notification
	CaseID         string    `json:"caseID"`
	Thumbnail      Thumbnail `json:"thumbnail"`
	DisputerID     string    `json:"disputerID"`
	DisputerHandle string    `json:"disputerHandle"`
	DisputeeID     string    `json:"disputeeID"`
	DisputeeHandle string    `json:"disputeeHandle"`
}

type DisputeClose struct {
	Notification
	OrderID          string    `json:"orderID"`
	Thumbnail        Thumbnail `json:"thumbnail"`
	OtherPartyID     string    `json:"otherPartyID"`
	OtherPartyHandle string    `json:"otherPartyHandle"`
	Buyer            string    `json:"buyer"`
}

type DisputeAccepted struct {
	Notification
	OrderID          string    `json:"orderID"`
	Thumbnail        Thumbnail `json:"thumbnail"`
	OtherPartyID     string    `json:"otherPartyID"`
	OtherPartyHandle string    `json:"otherPartyHandle"`
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
	OrderID       string `json:"orderID"`
	TransactionID string `json:"transactionID"`
	Coin          string `json:"coin"`
	Amount        uint64 `json:"amount"`
}

// RwaInstantBuyCompleted is emitted when an RWA instant buy (atomic swap) has completed on-chain
// This is triggered when the seller receives PAYMENT_SENT with method=RWA_INSTANT_BUY
// The atomic swap has already transferred tokens, so this just triggers the order confirmation
// Handled by handleRwaInstantBuyCompleted() in payment_rwa.go
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
