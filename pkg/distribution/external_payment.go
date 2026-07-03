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

// ExternalPaymentAddressRequest asks a trusted runtime for a fresh receiving
// address. Label is operational metadata and must not be treated as identity.
type ExternalPaymentAddressRequest struct {
	Label string
	Asset iwallet.CoinType
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
type ExternalPaymentRuntime interface {
	PaymentHealth(ctx context.Context) ExternalPaymentHealth
	CreatePaymentAddress(ctx context.Context, request ExternalPaymentAddressRequest) (ExternalPaymentAddress, error)
	WatchPayment(watch *ExternalPaymentWatch) error
	UnwatchPayment(addressIndex uint32)
	ReapPayment(addressIndex uint32)
	PaymentPollInterval() time.Duration
	PaymentGracePeriod(asset iwallet.CoinType) time.Duration
	PaymentHeight(ctx context.Context) (uint64, error)
}
