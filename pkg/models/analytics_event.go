package models

import "time"

// AnalyticsEvent records a single visitor interaction (page view, product view,
// add-to-cart, etc.) for store-level visitor analytics and conversion funnels.
type AnalyticsEvent struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	TenantID    string    `gorm:"column:tenant_id;type:varchar(255);not null;index:idx_ae_lookup,priority:1" json:"-"`
	EventType   string    `gorm:"column:event_type;type:varchar(32);not null;index:idx_ae_lookup,priority:2" json:"eventType"`
	SessionID   string    `gorm:"column:session_id;type:varchar(64);not null" json:"sessionId"`
	VisitorID   string    `gorm:"column:visitor_id;type:varchar(64);not null;index:idx_ae_visitor" json:"visitorId"`
	PagePath    string    `gorm:"column:page_path;type:varchar(512)" json:"pagePath"`
	ProductSlug string    `gorm:"column:product_slug;type:varchar(256)" json:"productSlug,omitempty"`
	Referrer    string    `gorm:"column:referrer;type:varchar(512)" json:"referrer,omitempty"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index:idx_ae_lookup,priority:3" json:"createdAt"`
}

func (AnalyticsEvent) TableName() string { return "analytics_events" }

const (
	EventTypePageView      = "page_view"
	EventTypeProductView   = "product_view"
	EventTypeAddToCart     = "add_to_cart"
	EventTypeCheckoutStart = "checkout_start"
)

// ValidEventTypes enumerates accepted event types for input validation.
var ValidEventTypes = map[string]bool{
	EventTypePageView:      true,
	EventTypeProductView:   true,
	EventTypeAddToCart:     true,
	EventTypeCheckoutStart: true,
}
