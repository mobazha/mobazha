package core

import (
	"context"
	"sync"
	"time"
)

// CollectiblePrimarySalePaidSignal is emitted when a collectible primary-sale
// order has a verified payment and can be recorded by a distribution adapter.
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

// CollectiblePrimarySalePaidHook bridges verified Node orders into a
// distribution-provided lifecycle adapter. It must be idempotent by OrderID.
type CollectiblePrimarySalePaidHook func(context.Context, CollectiblePrimarySalePaidSignal) error

// CollectibleFirstSaleAuthorizationSignal is emitted before any payment rail
// provisions a funding target for a managed source-custody first sale.
// CollectibleFirstSaleAuthorizationSignal describes a managed collectible
// reservation request at the payment-provisioning boundary.
type CollectibleFirstSaleAuthorizationSignal struct {
	OrderID              string
	HubSlotID            string
	CertNumber           string
	SellerPeerID         string
	PaymentCoin          string
	ReservationExpiresAt time.Time
}

// CollectibleFirstSaleAuthorizationHook reserves source-custody inventory
// before a payment rail provisions a funding target.
type CollectibleFirstSaleAuthorizationHook func(context.Context, CollectibleFirstSaleAuthorizationSignal) error

// CollectibleFirstSaleReservationReleaseSignal identifies a terminal order
// whose source-custody reservation can be released.
type CollectibleFirstSaleReservationReleaseSignal struct {
	OrderID string
	Reason  string
}

// CollectibleFirstSaleReservationReleaseHook releases a prior first-sale reservation.
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
