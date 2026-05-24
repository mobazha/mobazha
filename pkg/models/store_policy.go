package models

import "time"

// StorePolicy is the per-store commerce policy root.
// Tenant scoping is handled by the database layer; standalone nodes use
// database.StandaloneTenantID.
type StorePolicy struct {
	TenantID  string    `json:"-" gorm:"column:tenant_id;type:text;primaryKey;default:'_default'"`
	Revision  uint64    `json:"revision" gorm:"not null;default:0"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`

	Moderators []StoreModerator `json:"moderators,omitempty" gorm:"foreignKey:TenantID;references:TenantID"`
}

func (StorePolicy) TableName() string { return "store_policies" }

// StoreModerator is an ordered moderator entry for a store.
type StoreModerator struct {
	TenantID  string    `json:"-" gorm:"column:tenant_id;type:text;primaryKey;default:'_default'"`
	PeerID    string    `json:"peerID" gorm:"column:peer_id;type:text;primaryKey"`
	Position  int       `json:"position" gorm:"type:integer;not null;default:0"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (StoreModerator) TableName() string { return "store_moderators" }

type StorePolicyModeratorInput struct {
	PeerID   string `json:"peerID"`
	Position *int   `json:"position,omitempty"`
}

type StorePolicyModeratorsRequest struct {
	ExpectedRevision *uint64                     `json:"expectedRevision,omitempty"`
	Moderators       []StorePolicyModeratorInput `json:"moderators"`
}

type StorePolicyModeratorRequest struct {
	ExpectedRevision *uint64 `json:"expectedRevision,omitempty"`
	PeerID           string  `json:"peerID"`
	Position         *int    `json:"position,omitempty"`
}

type StorePolicyDeleteModeratorRequest struct {
	ExpectedRevision *uint64 `json:"expectedRevision,omitempty"`
}

type StorePolicyPublic struct {
	Revision   uint64           `json:"revision"`
	Moderators []StoreModerator `json:"moderators,omitempty"`
}
