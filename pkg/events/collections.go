package events

type CollectionProductAction string

const (
	CollectionProductActionAdd            CollectionProductAction = "add"
	CollectionProductActionRemove         CollectionProductAction = "remove"
	CollectionProductActionReorder        CollectionProductAction = "reorder"
	CollectionProductActionBulkRemove     CollectionProductAction = "bulk-remove"
)

type CollectionCreated struct {
	CollectionID string `json:"collectionID"`
	Title        string `json:"title"`
	Type         string `json:"type"`
}

type CollectionUpdated struct {
	CollectionID string `json:"collectionID"`
	Title        string `json:"title"`
}

type CollectionDeleted struct {
	CollectionID string `json:"collectionID"`
}

type CollectionProductsChanged struct {
	CollectionID string                 `json:"collectionID"`
	Action       CollectionProductAction `json:"action"`
	Slugs        []string               `json:"slugs,omitempty"`
}
