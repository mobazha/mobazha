package models

// Key holds raw key data used by the node and stored in the
// database. The name field identifies the key and is used as
// the primary key.
type Key struct {
	Name  string `gorm:"primaryKey"`
	Value []byte
}
