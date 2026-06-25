//go:build !private_distribution

package core

import (
	"context"
	"time"
)

// CollectiblePrimarySalePaidSignal is emitted when a collectible primary-sale
// order has a verified payment and can be recorded in the hosting Hub ledger.
type CollectiblePrimarySalePaidSignal struct {
	OrderID      string
	EscrowID     string
	HubSlotID    string
	NFTMint      string
	CertNumber   string
	BuyerPeerID  string
	SellerPeerID string
	PriceAmount  string
	CurrencyCode string
	PaidAt       time.Time
}

// CollectiblePrimarySalePaidHook bridges verified Node orders into hosting's
// collectible_primary_sales table. It must be idempotent by OrderID.
type CollectiblePrimarySalePaidHook func(context.Context, CollectiblePrimarySalePaidSignal) error

type collectiblesFields struct {
	collectiblePrimarySalePaidHook CollectiblePrimarySalePaidHook
}
