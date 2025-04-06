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
