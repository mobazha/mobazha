package models

import (
	"encoding/json"
	"time"
)

// BuyRequestStatus is the lifecycle of a buy request (the "wanted" board).
//
// Kept deliberately to four states: the board only has to know whether a
// request is still collecting offers, whether the buyer has picked a winner,
// and how it ended. Expiry is folded into cancelled so there is no separate
// state a client has to render.
type BuyRequestStatus string

const (
	BuyRequestStatusOpen      BuyRequestStatus = "open"
	BuyRequestStatusLocked    BuyRequestStatus = "locked"
	BuyRequestStatusCompleted BuyRequestStatus = "completed"
	BuyRequestStatusCancelled BuyRequestStatus = "cancelled"
)

// BuyRequestCondition is the condition the buyer is willing to accept.
type BuyRequestCondition string

const (
	BuyRequestConditionAny  BuyRequestCondition = "any"
	BuyRequestConditionNew  BuyRequestCondition = "new"
	BuyRequestConditionUsed BuyRequestCondition = "used"
)

// BuyRequestOfferStatus is the lifecycle of a single seller offer.
type BuyRequestOfferStatus string

const (
	BuyRequestOfferSubmitted BuyRequestOfferStatus = "submitted"
	BuyRequestOfferAccepted  BuyRequestOfferStatus = "accepted"
	BuyRequestOfferRejected  BuyRequestOfferStatus = "rejected"
)

// BuyRequest is the mirror image of a listing: the buyer states what they need
// and sellers answer with structured offers.
//
// Monetary values are stored as string amounts to keep full decimal precision
// (crypto amounts do not fit a float), matching how listing prices are carried
// elsewhere in the schema.
type BuyRequest struct {
	ID          string `json:"id" gorm:"primaryKey;type:text"`
	TenantID    string `json:"-" gorm:"column:tenant_id;type:text;not null;default:'_default';index"`
	Slug        string `json:"slug" gorm:"type:text;not null;uniqueIndex"`
	BuyerPeerID string `json:"buyerPeerID" gorm:"column:buyer_peer_id;type:text;not null;index"`

	Title        string `json:"title" gorm:"type:text;not null"`
	CategorySlug string `json:"categorySlug" gorm:"column:category_slug;type:text;index"`
	Description  string `json:"description,omitempty" gorm:"type:text"`
	Quantity     int    `json:"quantity" gorm:"type:integer;not null;default:1"`
	Unit         string `json:"unit,omitempty" gorm:"type:text"`

	BudgetMaxAmount   string `json:"budgetMaxAmount" gorm:"column:budget_max_amount;type:text;not null"`
	BudgetMaxCurrency string `json:"budgetMaxCurrency" gorm:"column:budget_max_currency;type:text;not null"`
	BudgetMinAmount   string `json:"budgetMinAmount,omitempty" gorm:"column:budget_min_amount;type:text"`

	// AcceptedCurrencies is stored as ",USD,USDT," so a currency filter is an
	// exact substring match and "USD" can never match inside "USDT". The board
	// only ever needs membership tests, so a join table would be overkill.
	AcceptedCurrencies string `json:"acceptedCurrencies" gorm:"column:accepted_currencies;type:text;not null"`

	DeliverTo string              `json:"deliverTo" gorm:"column:deliver_to;type:text;index"`
	Condition BuyRequestCondition `json:"condition" gorm:"type:text;not null;default:'any'"`

	Status          BuyRequestStatus `json:"status" gorm:"type:text;not null;default:'open';index"`
	AcceptedOfferID string           `json:"acceptedOfferID,omitempty" gorm:"column:accepted_offer_id;type:text"`

	ExpiresAt time.Time  `json:"expiresAt" gorm:"column:expires_at;index"`
	CreatedAt time.Time  `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"column:deleted_at;index"`

	// OfferCount is computed per read; it is not a column.
	OfferCount int `json:"offerCount" gorm:"-"`

	// ImagesJSON holds the uploaded image hash sets as a JSON array. Images is
	// the typed view; go through SetImages/GetImages rather than touching the
	// column directly, so the two never disagree.
	ImagesJSON string        `json:"-" gorm:"column:images;type:text;not null;default:'[]'"`
	Images     []ImageHashes `json:"images,omitempty" gorm:"-"`
}

func (BuyRequest) TableName() string { return "buy_requests" }

// MaxBuyRequestImages bounds how many pictures a request may carry: enough to
// show an item from a few angles, few enough that the board stays readable.
const MaxBuyRequestImages = 8

// GetImages deserializes the stored image hash sets.
func (r *BuyRequest) GetImages() ([]ImageHashes, error) {
	if r.ImagesJSON == "" || r.ImagesJSON == "[]" {
		return nil, nil
	}
	var images []ImageHashes
	if err := json.Unmarshal([]byte(r.ImagesJSON), &images); err != nil {
		return nil, err
	}
	return images, nil
}

// SetImages serializes the image hash sets for storage and keeps the typed
// field in step, so a caller that just wrote an entity can read it back.
func (r *BuyRequest) SetImages(images []ImageHashes) error {
	if len(images) == 0 {
		r.ImagesJSON = "[]"
		r.Images = nil
		return nil
	}
	payload, err := json.Marshal(images)
	if err != nil {
		return err
	}
	r.ImagesJSON = string(payload)
	r.Images = images
	return nil
}

// BuyRequestOffer is a seller's structured answer to a buy request. Every offer
// is escrow-backed: acceptsEscrow is always true and is not stored as a column.
type BuyRequestOffer struct {
	ID           string `json:"id" gorm:"primaryKey;type:text"`
	TenantID     string `json:"-" gorm:"column:tenant_id;type:text;not null;default:'_default';index"`
	RequestSlug  string `json:"requestSlug" gorm:"column:request_slug;type:text;not null;index"`
	SellerPeerID string `json:"sellerPeerID" gorm:"column:seller_peer_id;type:text;not null;index"`

	UnitPriceAmount   string `json:"unitPriceAmount" gorm:"column:unit_price_amount;type:text;not null"`
	UnitPriceCurrency string `json:"unitPriceCurrency" gorm:"column:unit_price_currency;type:text;not null"`

	Quantity     int    `json:"quantity" gorm:"type:integer;not null"`
	LeadTimeDays int    `json:"leadTimeDays" gorm:"column:lead_time_days;type:integer;not null;default:0"`
	ShipsFrom    string `json:"shipsFrom" gorm:"column:ships_from;type:text"`
	Note         string `json:"note,omitempty" gorm:"type:text"`

	Status BuyRequestOfferStatus `json:"status" gorm:"type:text;not null;default:'submitted';index"`

	CreatedAt time.Time  `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"column:deleted_at;index"`
}

func (BuyRequestOffer) TableName() string { return "buy_request_offers" }

// MaxBuyRequestCommentLength bounds a single comment body. Long enough for a
// real explanation, short enough that the board stays readable.
const MaxBuyRequestCommentLength = 2000

// BuyRequestComment is a message on a buy request.
//
// Threading is intentionally one level deep: a comment either starts a thread
// (ParentCommentID is empty) or answers one that does. Allowing replies to
// replies produces indentation towers that the board has no room to render, so
// the app service rejects them.
type BuyRequestComment struct {
	ID           string `json:"id" gorm:"primaryKey;type:text"`
	TenantID     string `json:"-" gorm:"column:tenant_id;type:text;not null;default:'_default';index"`
	RequestSlug  string `json:"requestSlug" gorm:"column:request_slug;type:text;not null;index"`
	AuthorPeerID string `json:"authorPeerID" gorm:"column:author_peer_id;type:text;not null;index"`
	// ParentCommentID is empty for a top-level comment.
	ParentCommentID string `json:"parentCommentID,omitempty" gorm:"column:parent_comment_id;type:text;index"`
	Body            string `json:"body" gorm:"type:text;not null"`

	CreatedAt time.Time  `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"column:deleted_at;index"`
}

func (BuyRequestComment) TableName() string { return "buy_request_comments" }
