package models

import (
	"encoding/json"
	"time"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// ============== GORM 持久化实体 ==============

// ShippingLocationEntity is the GORM model for the shipping_locations table.
type ShippingLocationEntity struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	TenantID  string    `json:"tenantID" gorm:"column:tenant_id;type:text;not null;default:'_default'"`
	Name      string    `json:"name" gorm:"type:text;not null"`
	Address   string    `json:"address,omitempty" gorm:"type:text;default:''"`
	IsDefault bool      `json:"isDefault" gorm:"column:is_default;not null;default:false"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (ShippingLocationEntity) TableName() string { return "shipping_locations" }

// ShippingProfileEntity is the GORM model for the shipping_profiles table.
// LocationGroups is stored as a JSON string; use Get/SetLocationGroups for typed access.
type ShippingProfileEntity struct {
	ID                 string    `json:"id" gorm:"primaryKey;type:text"`
	TenantID           string    `json:"tenantID" gorm:"column:tenant_id;type:text;not null;default:'_default'"`
	Name               string    `json:"name" gorm:"type:text;not null"`
	IsDefault          bool      `json:"isDefault" gorm:"column:is_default;not null;default:false"`
	Version            int       `json:"version" gorm:"type:integer;not null;default:1"`
	LocationGroupsJSON string    `json:"-" gorm:"column:location_groups;type:text;not null;default:'[]'"`
	CreatedAt          time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`

	ListingCount int `json:"listingCount,omitempty" gorm:"-"`
}

func (ShippingProfileEntity) TableName() string { return "shipping_profiles" }

// GetLocationGroups deserializes the JSON column into typed LocationGroup slice.
func (p *ShippingProfileEntity) GetLocationGroups() ([]*LocationGroup, error) {
	if p.LocationGroupsJSON == "" || p.LocationGroupsJSON == "[]" {
		return nil, nil
	}
	var groups []*LocationGroup
	if err := json.Unmarshal([]byte(p.LocationGroupsJSON), &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// SetLocationGroups serializes LocationGroup slice to the JSON column.
func (p *ShippingProfileEntity) SetLocationGroups(groups []*LocationGroup) error {
	if groups == nil {
		p.LocationGroupsJSON = "[]"
		return nil
	}
	data, err := json.Marshal(groups)
	if err != nil {
		return err
	}
	p.LocationGroupsJSON = string(data)
	return nil
}

// UnmarshalJSON handles incoming API requests by populating LocationGroupsJSON
// from the structured locationGroups field (since LocationGroupsJSON has json:"-").
func (p *ShippingProfileEntity) UnmarshalJSON(data []byte) error {
	type Alias ShippingProfileEntity
	aux := &struct {
		*Alias
		LocationGroups json.RawMessage `json:"locationGroups,omitempty"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if len(aux.LocationGroups) > 0 && string(aux.LocationGroups) != "null" {
		p.LocationGroupsJSON = string(aux.LocationGroups)
	}
	return nil
}

// MarshalJSON produces the API response including locationGroups as structured JSON.
func (p ShippingProfileEntity) MarshalJSON() ([]byte, error) {
	groups, err := p.GetLocationGroups()
	if err != nil {
		groups = nil
	}
	type Alias ShippingProfileEntity
	return json.Marshal(&struct {
		Alias
		LocationGroups []*LocationGroup `json:"locationGroups"`
	}{
		Alias:          (Alias)(p),
		LocationGroups: groups,
	})
}

// ListingShippingRef tracks the association between a listing and its shipping profile,
// including snapshot version for staleness detection.
type ListingShippingRef struct {
	ID                string    `json:"id" gorm:"primaryKey;type:text"`
	TenantID          string    `json:"tenantID" gorm:"column:tenant_id;type:text;not null;default:'_default'"`
	ListingSlug       string    `json:"listingSlug" gorm:"column:listing_slug;type:text;not null"`
	ShippingProfileID string    `json:"shippingProfileID" gorm:"column:shipping_profile_id;type:text;not null"`
	SnapshotVersion   int       `json:"snapshotVersion" gorm:"column:snapshot_version;type:integer;not null;default:0"`
	IsStale           bool      `json:"isStale" gorm:"column:is_stale;not null;default:false"`
	CreatedAt         time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`

	ListingTitle string `json:"listingTitle,omitempty" gorm:"-"`
	ProfileName  string `json:"profileName,omitempty" gorm:"-"`
}

func (ListingShippingRef) TableName() string { return "listing_shipping_refs" }

// ConvertShippingEntityToProto converts a ShippingProfileEntity (DB layer) to the
// protobuf ShippingProfile embedded in listings. It deserializes LocationGroupsJSON
// and reuses the same ShippingProfile JSON model as the intermediate representation.
func ConvertShippingEntityToProto(entity *ShippingProfileEntity) *pb.ShippingProfile {
	if entity == nil {
		return nil
	}
	groups, _ := entity.GetLocationGroups()
	return ConvertShippingProfileToProto(&ShippingProfile{
		ProfileID:      entity.ID,
		Name:           entity.Name,
		IsDefault:      entity.IsDefault,
		LocationGroups: groups,
	})
}

// ShippingProfilePatch holds optional fields for PATCH partial update.
// Non-nil fields will be applied; nil fields are left unchanged.
type ShippingProfilePatch struct {
	Name           *string `json:"name,omitempty"`
	IsDefault      *bool   `json:"isDefault,omitempty"`
	LocationGroups *string `json:"locationGroups,omitempty"`
	Version        int     `json:"version"`
}

// HasLocationGroupsChange returns true if the patch includes locationGroups modification.
func (p *ShippingProfilePatch) HasLocationGroupsChange() bool {
	return p.LocationGroups != nil
}

// StaleListingInfo is the API response type for stale-listings queries.
type StaleListingInfo struct {
	ListingSlug     string `json:"listingSlug"`
	ListingTitle    string `json:"listingTitle"`
	ProfileName     string `json:"profileName"`
	SnapshotVersion int    `json:"snapshotVersion"`
	CurrentVersion  int    `json:"currentVersion"`
}
