package models

import "time"

// WishlistItem represents a product saved to the buyer's wishlist.
// Composite primary key (TenantID, VendorPeerID, Slug) ensures uniqueness per tenant.
// Snapshot fields (Title, Thumbnail, Price, Currency) are captured at add-time
// so the wishlist page can render meaningful product info without extra API calls.
type WishlistItem struct {
	TenantID     string    `gorm:"column:tenant_id;type:varchar(255);not null;default:'';primaryKey" json:"-"`
	VendorPeerID string    `gorm:"column:vendor_peer_id;type:varchar(128);not null;primaryKey" json:"peerID"`
	Slug         string    `gorm:"column:slug;type:varchar(256);not null;primaryKey" json:"slug"`
	Title        string    `gorm:"column:title;type:varchar(512)" json:"title"`
	Thumbnail    string    `gorm:"column:thumbnail;type:text" json:"thumbnail"`
	Price        string    `gorm:"column:price;type:varchar(64)" json:"price"`
	Currency     string    `gorm:"column:currency;type:varchar(16)" json:"currency"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

func (WishlistItem) TableName() string { return "wishlist_items" }
