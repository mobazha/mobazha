package models

import (
	"math/big"
	"sort"
	"time"
)

// PaymentObservation is the append-only fact table that records every chain
// event observed by either an on-chain monitor or a buyer-reported PaymentSent
// envelope. It is the source of truth for payment verification under the
// Monitor-Driven Payment model introduced in Phase EVM-ManagedEscrow v0.3.0.
//
// The model intentionally avoids any UPSERT/UPDATE semantics for content
// fields. Each (observer, chain event) pair gets exactly one row; the
// VerificationService aggregates rows via DISTINCT ON / ROW_NUMBER() and
// writes the resulting envelope to Order.SerializedPaymentSent.
//
// Authoritative design: docs/escrow/MONITOR_DRIVEN_PAYMENT.md (v2.0).
//
// ─────────────────────────────────────────────────────────────────────────
// Multi-tenant + multi-observer dedupe semantics
// ─────────────────────────────────────────────────────────────────────────
//
// The composite UNIQUE index `idx_payment_obs_dedupe` is:
//
//	(tenant_id, chain_namespace, chain_reference, tx_hash, event_index, observer)
//
// Field-by-field rationale:
//   - tenant_id: hard cross-tenant isolation; Tenant A's observations cannot
//     collide with Tenant B's even if they share the same chain event.
//   - chain_namespace + chain_reference: CAIP-2 (eip155:1, solana:mainnet,
//     external_payment:mainnet, bip122:<genesis>...). Lets all chains share one table.
//   - tx_hash + event_index: identifies the exact log/event within a tx
//     (event_index = 0 for native transfers, log index for ERC-20 Transfer,
//     etc.).
//   - observer: each writer (a specific monitor worker, or a specific
//     buyer-reported peerID) owns its own row. Restart / RPC replay by the
//     same observer hits UNIQUE → ErrDuplicateObservation (idempotent). A
//     different observer sees the same event ⇒ separate row, by design,
//     and the aggregator (DISTINCT ON priority monitor > buyer_reported)
//     picks the most trustworthy source.
//
// ─────────────────────────────────────────────────────────────────────────
// What can change after insert?
// ─────────────────────────────────────────────────────────────────────────
//
// The fact fields (amount, from_address, to_address, token_address, block_*,
// event_type) are immutable. Only two derived fields may be updated by a
// background worker: confirmations (rolling) and status (pending → confirmed
// → reverted on reorg). Everything else is append-only.
//
// ─────────────────────────────────────────────────────────────────────────
// Amount encoding
// ─────────────────────────────────────────────────────────────────────────
//
// Amount stores a 256-bit integer in the smallest unit (wei / sat / atomic
// units / lamports) as a decimal string. We use TEXT rather than NUMERIC(78,0)
// because:
//   - SQLite (standalone mode) has no native arbitrary-precision numeric
//     type; the existing pkg/database double-dialect contract uses TEXT.
//   - Order.CancelFeeAmount already establishes the same convention for wei
//     amounts (pkg/models/orders.go ~line 211).
//   - Go-side codec is uniform across dialects (big.Int.SetString / .String).
//
// See AmountBigInt() / SetAmountBigInt() helpers for the conversion.
type PaymentObservation struct {
	// Composite primary key: (TenantID, ID).
	//
	// We do NOT embed TenantMixin here because the dedupe uniqueIndex must
	// include tenant_id at priority:1, and embedding obscures the field
	// position from GORM's index parser. Spelling the tag inline keeps the
	// composite index unambiguous and matches the precedent set by
	// ProcessedFulfillmentEvent (pkg/models/supply_chain.go).
	TenantID string `gorm:"column:tenant_id;type:varchar(64);primaryKey;default:'';uniqueIndex:idx_payment_obs_dedupe,priority:1" json:"tenantId,omitempty"`
	ID       string `gorm:"column:id;type:varchar(64);primaryKey" json:"id"` // UUID v7 (caller-provided)

	OrderID string `gorm:"column:order_id;type:varchar(64);not null;index:idx_payment_obs_order,priority:2" json:"orderId"`

	// CAIP-2 chain identification. ChainNamespace is e.g. "eip155", "solana",
	// "external_payment", "bip122". ChainReference is the chain instance id within that
	// namespace (EVM chainID string, Solana cluster, EXTERNAL_PAYMENT network, UTXO genesis
	// hash). Combined they identify a chain unambiguously across all rows.
	ChainNamespace string `gorm:"column:chain_namespace;type:varchar(16);not null;uniqueIndex:idx_payment_obs_dedupe,priority:2;index:idx_payment_obs_chain_tx,priority:1" json:"chainNamespace"`
	ChainReference string `gorm:"column:chain_reference;type:varchar(64);not null;uniqueIndex:idx_payment_obs_dedupe,priority:3;index:idx_payment_obs_chain_tx,priority:2" json:"chainReference"`

	// Transaction-level identity. TxHash holds whatever the chain natively
	// uses (32-byte hex for EVM, 88-byte base58 for Solana, 32-byte hex for
	// EXTERNAL_PAYMENT txid, 32-byte hex for UTXO). EventIndex disambiguates multiple
	// events within a single tx (e.g. multiple ERC-20 Transfer logs); native
	// transfers use 0.
	TxHash     string `gorm:"column:tx_hash;type:varchar(128);not null;uniqueIndex:idx_payment_obs_dedupe,priority:4;index:idx_payment_obs_chain_tx,priority:3" json:"txHash"`
	EventIndex int    `gorm:"column:event_index;not null;default:0;uniqueIndex:idx_payment_obs_dedupe,priority:5" json:"eventIndex"`

	// Event classification. Free-form string so future chains can add new
	// event types without schema migrations. Established values today:
	// "managed_escrow_received", "erc20_transfer", "external_payment_deposit", "utxo_funding",
	// "solana_transfer".
	EventType string `gorm:"column:event_type;type:varchar(32);not null" json:"eventType"`

	// Address fields are evidence-only. FromAddress can be empty when the
	// chain does not expose it (or in CEX direct-pay scenarios where the
	// observed sender is the CEX hot wallet). Refund routing MUST use
	// Order.RefundAddress (D-Hybrid-27) and never derive from FromAddress.
	FromAddress  string `gorm:"column:from_address;type:varchar(128)" json:"fromAddress,omitempty"`
	ToAddress    string `gorm:"column:to_address;type:varchar(128);not null" json:"toAddress"`
	TokenAddress string `gorm:"column:token_address;type:varchar(128)" json:"tokenAddress,omitempty"` // empty = native

	// Amount in smallest unit, decimal string. See encoding notes above.
	Amount string `gorm:"column:amount;type:text;not null" json:"amount"`

	BlockNumber   int64     `gorm:"column:block_number;not null;index:idx_payment_obs_status,priority:2" json:"blockNumber"`
	BlockTime     time.Time `gorm:"column:block_time;not null" json:"blockTime"`
	Confirmations int       `gorm:"column:confirmations;not null;default:0" json:"confirmations"`

	// Source classifies the business path that produced the observation:
	// "monitor" (chain watcher) or "buyer_reported" (PaymentSent envelope
	// inbound from a buyer peer). Used by the aggregator to prefer monitor
	// rows over buyer-reported rows when both observed the same event.
	Source string `gorm:"column:source;type:varchar(32);not null" json:"source"`

	// Observer is a stable identifier of the writer instance. Conventional
	// formats:
	//   monitor:<chain_ref>:<workerID>
	//   buyer:<peerID>
	// Each unique observer string can have at most one row per (tenant, chain,
	// tx, event_index) tuple — the UNIQUE index enforces this for replay
	// idempotency.
	Observer string `gorm:"column:observer;type:varchar(64);not null;uniqueIndex:idx_payment_obs_dedupe,priority:6" json:"observer"`

	// Status flows: pending → confirmed (when confirmations ≥ chain quorum)
	// → reverted (only on detected reorg). Only "confirmed" rows feed the
	// aggregator. The status+block_number index supports the reorg-rescan
	// worker scanning recent pending rows on each new chain head.
	Status string `gorm:"column:status;type:varchar(16);not null;default:'pending';index:idx_payment_obs_status,priority:1" json:"status"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	// No UpdatedAt: append-only. Confirmations / Status are derived fields
	// updated by background worker; contract is "value at creation never
	// changes", which is what consumers should rely on.
}

// TableName overrides the default GORM table name.
func (PaymentObservation) TableName() string { return "payment_observations" }

// Source values.
const (
	PaymentObservationSourceMonitor       = "monitor"
	PaymentObservationSourceBuyerReported = "buyer_reported"
)

// Status values.
const (
	PaymentObservationStatusPending   = "pending"
	PaymentObservationStatusConfirmed = "confirmed"
	PaymentObservationStatusReverted  = "reverted"
)

// EventType values (well-known set; new chains may add more).
const (
	PaymentEventManagedEscrowReceived   = "managed_escrow_received"
	PaymentEventERC20Transfer  = "erc20_transfer"
	PaymentEventEXTERNAL_PAYMENTDeposit     = "external_payment_deposit"
	PaymentEventUTXOFunding    = "utxo_funding"
	PaymentEventSolanaTransfer = "solana_transfer"
)

// AmountBigInt parses Amount as a 256-bit unsigned integer. Returns nil and
// false if Amount is empty or not a valid decimal integer.
func (p *PaymentObservation) AmountBigInt() (*big.Int, bool) {
	if p == nil || p.Amount == "" {
		return nil, false
	}
	v, ok := new(big.Int).SetString(p.Amount, 10)
	return v, ok
}

// SetAmountBigInt encodes a *big.Int as a decimal string in Amount. A nil
// pointer is encoded as the empty string (which AmountBigInt then reports
// as missing).
func (p *PaymentObservation) SetAmountBigInt(v *big.Int) {
	if v == nil {
		p.Amount = ""
		return
	}
	p.Amount = v.String()
}

// Observer-priority constants for the dedupe rule.
//
// Lower number wins. The ordering encodes the trust hierarchy laid out in
// MONITOR_DRIVEN_PAYMENT.md §3.2: monitor (we observed the chain
// directly) outranks buyer_reported (peer told us about a tx), and
// anything we don't recognise sorts last so unexpected sources can never
// silently outrank a known one.
const (
	dedupePriorityMonitor       = 0
	dedupePriorityBuyerReported = 1
	dedupePriorityUnknown       = 2
)

func observerPriority(source string) int {
	switch source {
	case PaymentObservationSourceMonitor:
		return dedupePriorityMonitor
	case PaymentObservationSourceBuyerReported:
		return dedupePriorityBuyerReported
	default:
		return dedupePriorityUnknown
	}
}

// DedupePaymentObservations collapses a slice of PaymentObservation rows
// to one row per (chain_namespace, chain_reference, tx_hash, event_index)
// tuple. The winning row per tuple is selected by:
//
//  1. Lowest observer priority (monitor > buyer_reported > unknown).
//  2. Earliest BlockTime as a tie-breaker.
//  3. Lexicographically smallest ID for full determinism (UUIDv7 is
//     monotonic, so this also tracks observation order).
//
// The function is the single source of truth for dedupe semantics and is
// shared by:
//
//   - GormPaymentObservationRepo.ListDeduplicatedConfirmed (read path)
//   - AggregatingVerifier.AggregateAndEmit (verification path)
//
// Keeping the rule in pkg/models (the package that owns PaymentObservation)
// guarantees both call sites can never drift apart, which is the entire
// reason §3.2 of the design doc treats "monitor outranks buyer_reported"
// as an invariant: if the verifier and the audit repo ever disagreed on
// dedupe, an order could appear "verified" via one path and "still
// pending" via another.
//
// Behaviour notes:
//
//   - Pre-sort is not required; the function is order-insensitive. The
//     output is sorted by (BlockTime ASC, ID ASC) for stable iteration in
//     downstream sums.
//   - Slices of length ≤ 1 are returned as-is — no allocation.
//   - The §14.1 pessimistic worst case bounds a single order at ~4 rows
//     (multiple deposits × dual observers); allocating one map entry per
//     unique tuple is a constant cost dwarfed by the network round-trip
//     to fetch the rows in the first place.
func DedupePaymentObservations(rows []PaymentObservation) []PaymentObservation {
	if len(rows) <= 1 {
		return rows
	}

	type tupleKey struct {
		ns, ref, tx string
		idx         int
	}

	type candidate struct {
		row       PaymentObservation
		priority  int
		blockUnix int64
	}

	best := make(map[tupleKey]candidate, len(rows))
	for _, r := range rows {
		key := tupleKey{
			ns:  r.ChainNamespace,
			ref: r.ChainReference,
			tx:  r.TxHash,
			idx: r.EventIndex,
		}
		c := candidate{
			row:       r,
			priority:  observerPriority(r.Source),
			blockUnix: r.BlockTime.UnixNano(),
		}
		existing, ok := best[key]
		if !ok || dedupeCandidateLess(c, existing) {
			best[key] = c
		}
	}

	out := make([]PaymentObservation, 0, len(best))
	for _, c := range best {
		out = append(out, c.row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BlockTime.Equal(out[j].BlockTime) {
			return out[i].ID < out[j].ID
		}
		return out[i].BlockTime.Before(out[j].BlockTime)
	})
	return out
}

func dedupeCandidateLess(a, b struct {
	row       PaymentObservation
	priority  int
	blockUnix int64
}) bool {
	if a.priority != b.priority {
		return a.priority < b.priority
	}
	if a.blockUnix != b.blockUnix {
		return a.blockUnix < b.blockUnix
	}
	return a.row.ID < b.row.ID
}
