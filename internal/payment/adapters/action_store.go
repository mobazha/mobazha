package adapters

import "github.com/mobazha/mobazha3.0/pkg/payment"

// Transitional aliases keep existing internal adapters source-compatible while
// trusted distribution modules adopt the public payment contracts directly.
type (
	ActionRecord      = payment.ActionRecord
	ActionStore       = payment.ActionStore
	ActionRecorder    = payment.ActionRecorder
	MemoryActionStore = payment.MemoryActionStore
)

var (
	ErrActionRecordNotFound = payment.ErrActionRecordNotFound
	NewMemoryActionStore    = payment.NewMemoryActionStore
)
