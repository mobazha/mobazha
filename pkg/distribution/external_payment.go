package distribution

import (
	"context"
	"time"
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
}

// ExternalPaymentAddress is an allocated address plus its opaque runtime
// index. Core persists the index so a watch can be restored after restart.
type ExternalPaymentAddress struct {
	Address string
	Index   uint32
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
	ExpectedAmount uint64
	ExpiresAt      time.Time
	OnPayment      func(ExternalPaymentEvent)
}

// ExternalPaymentRuntime is the provider-neutral direct observed rail used by
// trusted first-party distributions. Wallet administration remains outside
// this port; implementations expose only address allocation, observation,
// health, and reversible lifecycle operations required by Core.
type ExternalPaymentRuntime interface {
	Start(ctx context.Context) error
	Close() error
	PaymentAvailable(ctx context.Context) bool
	CreatePaymentAddress(ctx context.Context, request ExternalPaymentAddressRequest) (ExternalPaymentAddress, error)
	WatchPayment(watch *ExternalPaymentWatch) error
	UnwatchPayment(addressIndex uint32)
	ReapPayment(addressIndex uint32)
	PaymentPollInterval() time.Duration
	PaymentGracePeriod() time.Duration
	PaymentHeight(ctx context.Context) (uint64, error)
	PaymentHealthy() bool
}
