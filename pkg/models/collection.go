package models

import "time"

type CollectionType string

const (
	CollectionTypeManual CollectionType = "manual"
	CollectionTypeAuto   CollectionType = "auto"
)

type CollectionSortOrder string

const (
	CollectionSortManual      CollectionSortOrder = "manual"
	CollectionSortAlphaAsc    CollectionSortOrder = "alpha-asc"
	CollectionSortAlphaDesc   CollectionSortOrder = "alpha-desc"
	CollectionSortPriceAsc    CollectionSortOrder = "price-asc"
	CollectionSortPriceDesc   CollectionSortOrder = "price-desc"
	CollectionSortCreatedDesc CollectionSortOrder = "created-desc"
	CollectionSortCreatedAsc  CollectionSortOrder = "created-asc"
)

type Collection struct {
	ID          string              `json:"id" gorm:"primaryKey;type:text"`
	TenantID    string              `json:"-" gorm:"column:tenant_id;type:text;not null;default:'_default';index"`
	Title       string              `json:"title" gorm:"type:text;not null"`
	Description string              `json:"description,omitempty" gorm:"type:text"`
	Image       string              `json:"image,omitempty" gorm:"type:text"`
	Type        CollectionType      `json:"type" gorm:"type:text;not null;default:'manual'"`
	Rules       *string             `json:"rules,omitempty" gorm:"type:text"`
	SortOrder   CollectionSortOrder `json:"sortOrder" gorm:"column:sort_order;type:text;not null;default:'manual'"`
	Published   bool                `json:"published" gorm:"not null;default:true"`
	CreatedAt   time.Time           `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time           `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt   *time.Time          `json:"deletedAt,omitempty" gorm:"column:deleted_at;index"`

	Products []CollectionProduct `json:"products,omitempty" gorm:"foreignKey:CollectionID"`
}

func (Collection) TableName() string { return "collections" }

type CollectionProduct struct {
	CollectionID string    `json:"collectionId" gorm:"primaryKey;type:text"`
	ListingSlug  string    `json:"listingSlug" gorm:"primaryKey;type:text"`
	Position     int       `json:"position" gorm:"type:integer;not null;default:0"`
	CreatedAt    time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
}

func (CollectionProduct) TableName() string { return "collection_products" }
