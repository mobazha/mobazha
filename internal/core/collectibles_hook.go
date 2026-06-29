package core

import (
	"context"
	"sync"
	"time"
)

// CollectiblePrimarySalePaidSignal is emitted when a collectible primary-sale
// order has a verified payment and can be recorded in the hosting Hub ledger.
type CollectiblePrimarySalePaidSignal struct {
	OrderID     string
	EscrowID    string
	HubSlotID   string
	NFTMint     string
	CertNumber  string
	BuyerPeerID string
	// BuyerSolanaAddress is the buyer wallet that should receive the primary-sale NFT.
	BuyerSolanaAddress string
	SellerPeerID       string
	PriceAmount        string
	CurrencyCode       string
	PaidAt             time.Time
}

// CollectiblePrimarySalePaidHook bridges verified Node orders into hosting's
// collectible_primary_sales table. It must be idempotent by OrderID.
type CollectiblePrimarySalePaidHook func(context.Context, CollectiblePrimarySalePaidSignal) error

// CollectibleFirstSaleAuthorizationSignal is emitted before any payment rail
// provisions a funding target for a managed source-custody first sale.
type CollectibleFirstSaleAuthorizationSignal struct {
	OrderID              string
	HubSlotID            string
	CertNumber           string
	SellerPeerID         string
	PaymentCoin          string
	ReservationExpiresAt time.Time
}

type CollectibleFirstSaleAuthorizationHook func(context.Context, CollectibleFirstSaleAuthorizationSignal) error

type CollectibleFirstSaleReservationReleaseSignal struct {
	OrderID string
	Reason  string
}

type CollectibleFirstSaleReservationReleaseHook func(context.Context, CollectibleFirstSaleReservationReleaseSignal) error

// Backwards-compatible aliases keep downstream option users source-compatible
// while the hook's contract now performs an authoritative reservation.
type CollectibleFirstSalePreflightSignal = CollectibleFirstSaleAuthorizationSignal
type CollectibleFirstSalePreflightHook = CollectibleFirstSaleAuthorizationHook

type collectiblesFields struct {
	collectiblePrimarySalePaidHook             CollectiblePrimarySalePaidHook
	collectibleFirstSaleAuthorizationHook      CollectibleFirstSaleAuthorizationHook
	collectibleFirstSaleReservationReleaseHook CollectibleFirstSaleReservationReleaseHook
	collectibleLifecycleDeliveryMu             sync.Mutex
}
