package models

import (
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// TransactionMetadata is the data model for wallet transaction
// metadata that is stored in the database. This is extra metadata
// beyond what is saved by the Multiwallet.
type TransactionMetadata struct {
	TenantMixin
	Txid           iwallet.TransactionID `gorm:"primaryKey"`
	PaymentAddress string
	Memo           string
	OrderID        OrderID
	Thumbnail      string
}
