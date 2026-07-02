package guest

// EVM guest checkout settlement strategies (Phase 2 decision).
//
// Authoritative write-up: mobazha_hosting/docs/payment/GUEST_CHECKOUT_CRYPTO_CLOSURE_PLAN.md
// (Phase 2 — DECIDED 2026-05-19).
const (
	// EVMSettlementEOAGasTopUp is Option A: platform tops up derived EOA gas, then EOA
	// signs native transfer to seller. Interim-only; requires product sign-off on gas ops.
	EVMSettlementEOAGasTopUp = "eoa_gas_top_up"

	// EVMSettlementManagedSession is Option C: per-order predicted address + PaymentObservation
	// + existing relay settlement. Target architecture for Phase 3.
	EVMSettlementManagedSession = "managed_payment_session"
)

// chosenEVMSettlementStrategy records the architecture decision for Phase 3 work.
// chosenEVMSettlementStrategy is implemented via Phase 3A–3C; buyer visibility is gated at runtime (Phase 3D).
const chosenEVMSettlementStrategy = EVMSettlementManagedSession
