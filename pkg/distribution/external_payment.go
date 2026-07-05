// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package distribution

import (
	"context"
	"time"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ExternalPaymentStatus is a provider-neutral observation state for a direct
// address payment rail. It deliberately does not encode a chain family.
type ExternalPaymentStatus string

const (
	ExternalPaymentPending   ExternalPaymentStatus = "pending"
	ExternalPaymentConfirmed ExternalPaymentStatus = "confirmed"
	ExternalPaymentOverpaid  ExternalPaymentStatus = "overpaid"
	ExternalPaymentPartial   ExternalPaymentStatus = "partial"
	ExternalPaymentExpired   ExternalPaymentStatus = "expired"
)

// ExternalPaymentAddressRequest asks a trusted runtime to create or retrieve
// one receiving address. IdempotencyKey is the durable operation identity and
// must map to the same address after retries and process restarts.
type ExternalPaymentAddressRequest struct {
	IdempotencyKey string
	Asset          iwallet.CoinType
}

// ExternalPaymentAddress is an allocated address plus its opaque runtime
// index. Core persists the index so a watch can be restored after restart.
type ExternalPaymentAddress struct {
	Address               string
	Index                 uint32
	RequiredConfirmations int
}

// ExternalPaymentEvent is the normalized observation emitted by a direct
// payment runtime. Amounts are expressed in the asset's smallest unit.
type ExternalPaymentEvent struct {
	Status         ExternalPaymentStatus
	LastTxHash     string
	TotalConfirmed uint64
	TotalPending   uint64
	MaxBlockHeight uint64
}

// ExternalPaymentWatch is the validated Core projection registered with a
// trusted direct-observation runtime.
type ExternalPaymentWatch struct {
	AddressIndex   uint32
	OrderID        string
	Asset          iwallet.CoinType
	ExpectedAmount uint64
	ExpiresAt      time.Time
	OnPayment      func(ExternalPaymentEvent)
}

// ExternalPaymentHealthState is the setup-aware lifecycle state of a direct
// observed rail. Only ready may be advertised for checkout.
type ExternalPaymentHealthState string

const (
	ExternalPaymentStopped    ExternalPaymentHealthState = "stopped"
	ExternalPaymentNeedsSetup ExternalPaymentHealthState = "needs_setup"
	ExternalPaymentDegraded   ExternalPaymentHealthState = "degraded"
	ExternalPaymentReady      ExternalPaymentHealthState = "ready"
)

// ExternalPaymentHealth is a provider-neutral, product-safe health snapshot.
type ExternalPaymentHealth struct {
	State  ExternalPaymentHealthState
	Detail string
}

func (health ExternalPaymentHealth) Ready() bool { return health.State == ExternalPaymentReady }

// ExternalPaymentRuntime is the provider-neutral direct observed rail used by
// trusted first-party distributions. Wallet administration remains outside
// this port; implementations expose only address allocation, observation,
// health, and observation operations required by Core. Module lifecycle stays
// on the private module runner and is not granted through this data-plane port.
// EnsurePaymentAddress must be safe under concurrent retries and must return
// the same normalized result for one idempotency key across process restarts.
type ExternalPaymentRuntime interface {
	PaymentHealth(ctx context.Context) ExternalPaymentHealth
	EnsurePaymentAddress(ctx context.Context, request ExternalPaymentAddressRequest) (ExternalPaymentAddress, error)
	WatchPayment(watch *ExternalPaymentWatch) error
	UnwatchPayment(addressIndex uint32)
	// ReapPayment releases runtime-local observation state for an address that
	// Core has terminalized. Implementations must make repeated calls safe so a
	// crash between cleanup and durable completion can be replayed.
	ReapPayment(addressIndex uint32)
	PaymentPollInterval() time.Duration
	// PaymentGracePeriod is immutable timing policy. It must remain safe to
	// call after the runtime has stopped so active order deadlines do not
	// change when a module is temporarily unbound.
	PaymentGracePeriod(asset iwallet.CoinType) time.Duration
	PaymentHeight(ctx context.Context) (uint64, error)
}
