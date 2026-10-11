package events

// BuyRequestOfferReceived tells the buyer that a seller answered their wanted
// request. The offer itself is already stored on this node; the event exists so
// the buyer sees it without having the board open.
type BuyRequestOfferReceived struct {
	Notification
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	OfferID      string `json:"offerID"`
	SellerPeerID string `json:"sellerPeerID"`
	UnitPrice    string `json:"unitPrice"`
	Currency     string `json:"currency"`
	Quantity     int    `json:"quantity"`
	LeadTimeDays int    `json:"leadTimeDays"`
	Note         string `json:"note,omitempty"`
}

// BuyRequestOfferDecided tells a seller what the buyer decided about its offer.
// Acceptance and refusal share one event type because on the seller's node they
// close the same thing: the offer is no longer waiting, and only the reason
// differs. Acceptance is the interesting case — it is what lets a seller turn
// the offer into a product.
type BuyRequestOfferDecided struct {
	Notification
	Slug string `json:"slug"`
	// Title is the request's title, so the seller recognises the deal without
	// another round trip to the node that owns it.
	Title string `json:"title"`
	// OwnerPeerID is the node that holds the request and made this decision.
	OwnerPeerID string `json:"ownerPeerID"`
	OfferID     string `json:"offerID"`
	Accepted    bool   `json:"accepted"`
	UnitPrice   string `json:"unitPrice,omitempty"`
	Currency    string `json:"currency,omitempty"`
}
