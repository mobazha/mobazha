package models

import (
	"time"
)

// Discount method: how the discount is triggered.
type DiscountMethod string

const (
	DiscountMethodCode      DiscountMethod = "code"
	DiscountMethodAutomatic DiscountMethod = "automatic"
)

// Discount value type: how the discount value is calculated.
type DiscountValueType string

const (
	DiscountValuePercentage   DiscountValueType = "percentage"
	DiscountValueFixed        DiscountValueType = "fixed_amount"
	DiscountValueFreeShipping DiscountValueType = "free_shipping"
)

// Discount scope: what the discount applies to.
type DiscountScope string

const (
	DiscountScopeOrder   DiscountScope = "order"
	DiscountScopeProduct DiscountScope = "product"
)

// Discount applies-to selector.
type DiscountAppliesTo string

const (
	DiscountAppliesToAll                 DiscountAppliesTo = "all"
	DiscountAppliesToSpecificProducts    DiscountAppliesTo = "specific_products"
	DiscountAppliesToSpecificCollections DiscountAppliesTo = "specific_collections"
)

// Discount status lifecycle.
type DiscountStatus string

const (
	DiscountStatusDraft     DiscountStatus = "draft"
	DiscountStatusScheduled DiscountStatus = "scheduled"
	DiscountStatusActive    DiscountStatus = "active"
	DiscountStatusExpired   DiscountStatus = "expired"
)

// Minimum purchase type for discount conditions.
type DiscountMinPurchaseType string

const (
	DiscountMinPurchaseNone      DiscountMinPurchaseType = "none"
	DiscountMinPurchaseAmount    DiscountMinPurchaseType = "min_amount"
	DiscountMinPurchaseQuantity  DiscountMinPurchaseType = "min_quantity"
)

// Discount is the core discount entity, independent of any listing.
type Discount struct {
	ID                   string                  `json:"id" gorm:"primaryKey;type:text"`
	TenantID             string                  `json:"tenantID" gorm:"column:tenant_id;type:text;not null;default:'_default'"`
	Title                string                  `json:"title" gorm:"type:text;not null"`
	Description          string                  `json:"description,omitempty" gorm:"type:text"`
	Method               DiscountMethod          `json:"method" gorm:"type:text;not null"`
	Status               DiscountStatus          `json:"status" gorm:"type:text;not null;default:'active'"`
	ValueType            DiscountValueType       `json:"valueType" gorm:"column:value_type;type:text;not null"`
	Value                string                  `json:"value" gorm:"type:text;not null;default:'0'"`
	Currency             string                  `json:"currency" gorm:"type:text;not null;default:''"`
	MaxDiscountAmount    *string                 `json:"maxDiscountAmount,omitempty" gorm:"column:max_discount_amount;type:text"`
	Scope                DiscountScope           `json:"scope" gorm:"type:text;not null;default:'order'"`
	AppliesTo            DiscountAppliesTo       `json:"appliesTo" gorm:"column:applies_to;type:text;not null;default:'all'"`
	ProductIDs           StringSlice             `json:"productIDs,omitempty" gorm:"column:product_ids;type:text"`
	CollectionIDs        StringSlice             `json:"collectionIDs,omitempty" gorm:"column:collection_ids;type:text"`
	MinPurchaseType      DiscountMinPurchaseType `json:"minPurchaseType" gorm:"column:min_purchase_type;type:text;not null;default:'none'"`
	MinAmount            *string                 `json:"minAmount,omitempty" gorm:"column:min_amount;type:text"`
	MinQuantity          *int                    `json:"minQuantity,omitempty" gorm:"column:min_quantity;type:integer"`
	UsageLimit           int                     `json:"usageLimit" gorm:"column:usage_limit;type:integer;not null;default:0"`
	UsageCount           int                     `json:"usageCount" gorm:"column:usage_count;type:integer;not null;default:0"`
	PerCustomerLimit     int                     `json:"perCustomerLimit" gorm:"column:per_customer_limit;type:integer;not null;default:0"`
	CombinesWithProduct  bool                    `json:"combinesWithProduct" gorm:"column:combines_with_product;type:integer;not null;default:1"`
	CombinesWithOrder    bool                    `json:"combinesWithOrder" gorm:"column:combines_with_order;type:integer;not null;default:0"`
	CombinesWithShipping bool                    `json:"combinesWithShipping" gorm:"column:combines_with_shipping;type:integer;not null;default:1"`
	StartsAt             time.Time               `json:"startsAt" gorm:"column:starts_at;not null"`
	EndsAt               *time.Time              `json:"endsAt,omitempty" gorm:"column:ends_at"`
	DeletedAt            *time.Time              `json:"deletedAt,omitempty" gorm:"column:deleted_at;index"`
	CreatedAt            time.Time               `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt            time.Time               `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`

	Codes []DiscountCode `json:"codes,omitempty" gorm:"foreignKey:DiscountID;references:ID"`
}

func (Discount) TableName() string { return "discounts" }

// DiscountCode represents a redeemable code linked to a Discount.
// One Discount can have multiple codes (e.g. batch-generated one-time codes).
type DiscountCode struct {
	ID         string    `json:"id" gorm:"primaryKey;type:text"`
	DiscountID string    `json:"discountID" gorm:"column:discount_id;type:text;not null;index"`
	Code       string    `json:"code" gorm:"type:text;not null"`
	CodeHash   string    `json:"-" gorm:"column:code_hash;type:text;not null;uniqueIndex"`
	UsageCount int       `json:"usageCount" gorm:"column:usage_count;type:integer;not null;default:0"`
	UsageLimit int       `json:"usageLimit" gorm:"column:usage_limit;type:integer;not null;default:0"`
	CreatedAt  time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
}

func (DiscountCode) TableName() string { return "discount_codes" }

// DiscountRedemption records a single use of a discount on an order.
type DiscountRedemption struct {
	ID             string    `json:"id" gorm:"primaryKey;type:text"`
	DiscountID     string    `json:"discountID" gorm:"column:discount_id;type:text;not null;index"`
	CodeID         *string   `json:"codeID,omitempty" gorm:"column:code_id;type:text"`
	OrderID        string    `json:"orderID" gorm:"column:order_id;type:text;not null;index"`
	CustomerPeerID string    `json:"customerPeerID" gorm:"column:customer_peer_id;type:text;not null"`
	DiscountAmount string    `json:"discountAmount" gorm:"column:discount_amount;type:text;not null"`
	Currency       string    `json:"currency" gorm:"type:text;not null"`
	RedeemedAt     time.Time `json:"redeemedAt" gorm:"column:redeemed_at;autoCreateTime"`
}

func (DiscountRedemption) TableName() string { return "discount_redemptions" }
