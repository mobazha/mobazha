package events

// ShippingProfileUpdated is emitted when a shipping profile's content changes
// (via PUT or PATCH). Subscribers can trigger stale listing refresh or
// notify the frontend via WebSocket.
type ShippingProfileUpdated struct {
	ProfileID           string `json:"profileID"`
	LocationGroupsDirty bool   `json:"locationGroupsDirty"`
}

// ShippingProfileDeleted is emitted after a shipping profile is deleted.
type ShippingProfileDeleted struct {
	ProfileID string `json:"profileID"`
	MigratedTo string `json:"migratedTo,omitempty"`
}

// ShippingSnapshotsRefreshed is emitted after a batch of stale listings
// has been republished.
type ShippingSnapshotsRefreshed struct {
	Refreshed int `json:"refreshed"`
	Errors    int `json:"errors"`
}
