//go:build !private_distribution

package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	paymentmetrics "github.com/mobazha/mobazha3.0/pkg/payment"
	"gorm.io/gorm"
)

// isUniqueViolationErr matches both the SQLite ("UNIQUE constraint failed")
// and PostgreSQL ("duplicate key value violates unique constraint") wordings
// used by tx.Save when a secondary UNIQUE index rejects the row. We do NOT
// rely on dialect-specific error codes (sqlite3.ErrConstraintUnique, pq's
// "23505") because the database/sql driver layer wraps them in opaque
// error types that vary across builds.
func isUniqueViolationErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

// GormPaymentObservationRepo is the GORM-backed implementation of
// contracts.PaymentObservationRepo.
//
// It deliberately mixes two transactional surfaces:
//
//   - db (database.Database): tenant-scoped. Used by InsertObservation,
//     ListDeduplicatedConfirmed and ListByOrder, all of which operate inside
//     a single tenant's slice of the table. The underlying tenantTx adds
//     WHERE tenant_id = ? to every query and stamps tenant_id on writes,
//     so the SQL we issue is naturally guarded.
//
//   - raw (*gorm.DB): global. Used only by RefreshConfirmations, which
//     drives "pending → confirmed" transitions from a chain head. Confirmation
//     depth is a property of the chain, not of the tenant, so iterating
//     tenants would be both wasteful and dependent on the caller knowing the
//     full active-tenant set. The raw session lets us issue exactly one
//     UPDATE per chain head event regardless of tenancy. The cross-tenant
//     write is sound here because (a) the SET clause depends only on
//     block_number and the bound chain head, never on tenant data, and
//     (b) we still surface the affected (tenantID, orderID) tuples so
//     downstream aggregation runs back through tenant-scoped paths.
//
// This split is documented at the interface level (see
// pkg/contracts/payment_observation_repo.go).
var _ contracts.PaymentObservationRepo = (*GormPaymentObservationRepo)(nil)

type GormPaymentObservationRepo struct {
	db  database.Database
	raw *gorm.DB
}

// NewGormPaymentObservationRepo wires the repo with the tenant-scoped
// database handle (used for per-tenant ops) and the underlying *gorm.DB
// (used for the cross-tenant RefreshConfirmations sweep).
//
// Both arguments are required. Passing nil for raw will surface only when
// RefreshConfirmations is called, since the other methods do not need it;
// constructors should fail fast if their callers cannot satisfy both
// surfaces.
func NewGormPaymentObservationRepo(db database.Database, raw *gorm.DB) *GormPaymentObservationRepo {
	return &GormPaymentObservationRepo{db: db, raw: raw}
}

// InsertObservation appends a single observation row.
//
// The implementation funnels through tx.Save so multi-tenant DBs stamp
// tenant_id correctly and SQLite's composite-PK UPSERT machinery stays in
// effect for the (tenant_id, id) PK. Because the caller is required to
// allocate a fresh UUID per call, the (tenant_id, id) tuple should never
// collide; the only conflict that ever fires is the dedupe UNIQUE
// (tenant_id, chain_namespace, chain_reference, tx_hash, event_index,
// observer), which surfaces as a generic "UNIQUE constraint failed" error
// that we translate to ErrDuplicateObservation.
//
// Caller contract (see PaymentObservation godoc):
//
//   - obs.ID must be a fresh, unique value (UUID v7 is the convention).
//     Reusing IDs WILL silently UPSERT and is a programming error.
//   - obs.TenantID must already match the order's tenant. We do not derive
//     it inside the repo so SaaS callers can fail loudly when they mis-route.
func (r *GormPaymentObservationRepo) InsertObservation(_ context.Context, obs *models.PaymentObservation) error {
	if obs == nil {
		return fmt.Errorf("payment observation: obs must not be nil")
	}
	if obs.ID == "" {
		return fmt.Errorf("payment observation: ID must be set (UUID v7 expected)")
	}
	if obs.TenantID == "" {
		return fmt.Errorf("payment observation: TenantID must be set")
	}
	err := r.db.Update(func(tx database.Tx) error {
		return tx.Save(obs)
	})
	if err == nil {
		paymentmetrics.RecordPaymentObservationInserted(
			obs.TenantID,
			obs.ChainNamespace,
			obs.ChainReference,
			obs.Source,
		)
		r.refreshPendingMetric(obs.ChainNamespace, obs.ChainReference)
		return nil
	}
	if isUniqueViolationErr(err) {
		paymentmetrics.RecordPaymentObservationDuplicate(
			obs.TenantID,
			obs.ChainNamespace,
			obs.Source,
		)
		r.refreshPendingMetric(obs.ChainNamespace, obs.ChainReference)
		return contracts.ErrDuplicateObservation
	}
	return err
}

// ListDeduplicatedConfirmed pulls all confirmed rows for the order and
// reduces them to one per (chain_namespace, chain_reference, tx_hash,
// event_index) tuple, applying the priority rule documented on the
// interface (monitor > buyer_reported, then earliest BlockTime).
//
// The dedupe runs in Go rather than in SQL. Two reasons:
//
//   - Portability. ROW_NUMBER() OVER PARTITION BY works on PostgreSQL and
//     SQLite ≥3.25 but the existing pkg/database double-dialect contract
//     (see helpers.go) avoids window functions entirely; keeping that
//     pattern means the Repo doesn't fork SQL by dialect.
//
//   - Volume. §14.1 of the design doc bounds a single order at ~4 rows in
//     extreme cases (multiple deposits × dual observers). Sorting and
//     scanning a slice of that size in Go is dwarfed by the network cost
//     of fetching it, so SQL-side dedupe would be an unjustified complexity
//     tax.
//
// If we ever need to dedupe across an entire tenant (audit) the contract
// allows swapping the implementation without touching callers — what we
// promise is the result, not the mechanism.
func (r *GormPaymentObservationRepo) ListDeduplicatedConfirmed(_ context.Context, tenantID, orderID string) ([]models.PaymentObservation, error) {
	if tenantID == "" || orderID == "" {
		return nil, fmt.Errorf("payment observation: tenantID and orderID must be set")
	}

	var rows []models.PaymentObservation
	err := r.db.View(func(tx database.Tx) error {
		// We scope by tenant_id explicitly even though tenantTx.Read()
		// already adds the predicate. The redundancy is intentional:
		//   - in standalone mode the underlying SqliteDB does NOT apply
		//     the tenantTx wrapping, so the predicate must come from us;
		//   - in SaaS mode the predicate matches what tenantTx adds, so
		//     the optimizer collapses it.
		return tx.Read().
			Where("tenant_id = ? AND order_id = ? AND status = ?",
				tenantID, orderID, models.PaymentObservationStatusConfirmed).
			Order("block_time ASC, id ASC").
			Find(&rows).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return models.DedupePaymentObservations(rows), nil
}

// ListByOrder returns every observation row for the order in deterministic
// order. Used by audit / dispute review and by tests; the verification hot
// path uses ListDeduplicatedConfirmed.
func (r *GormPaymentObservationRepo) ListByOrder(_ context.Context, tenantID, orderID string) ([]models.PaymentObservation, error) {
	if tenantID == "" || orderID == "" {
		return nil, fmt.Errorf("payment observation: tenantID and orderID must be set")
	}

	var rows []models.PaymentObservation
	err := r.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("tenant_id = ? AND order_id = ?", tenantID, orderID).
			Order("created_at ASC, id ASC").
			Find(&rows).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return rows, nil
}

// RefreshConfirmations advances pending observations to confirmed status
// for the given chain when the chain head has buried them by at least
// requiredConfirmations blocks.
//
// Cross-tenant by design: in SaaS mode every tenant shares the same chain
// head, so a single UPDATE with WHERE chain_namespace = ? AND
// chain_reference = ? is the natural unit of work. We use the raw GORM
// session to skip tenant scoping, but we read back the affected
// (tenantID, orderID) tuples in a strictly typed slice so callers can fan
// out aggregation work back into per-tenant transactions.
//
// Steps:
//
//  1. Pre-scan: collect (tenant_id, order_id) for rows that WILL transition
//     in this call. We need the snapshot before the UPDATE because, after
//     UPDATE, "rows that just transitioned" are indistinguishable from
//     "rows that were already confirmed".
//  2. UPDATE pending → confirmed for any row whose block was buried deep
//     enough. We also refresh Confirmations to the rolling depth, capped at
//     the required threshold so the gauge can't drift unbounded for very
//     old rows.
//  3. Return the unique tuples sorted (TenantID, OrderID) for stable
//     downstream behaviour.
//
// Both steps run inside a single GORM transaction so the snapshot and the
// UPDATE see a consistent slice of the table even under concurrent INSERTs.
func (r *GormPaymentObservationRepo) RefreshConfirmations(
	_ context.Context,
	chainNamespace, chainReference string,
	currentBlockNumber int64,
	requiredConfirmations int,
) ([]contracts.OrderRef, error) {
	if r.raw == nil {
		return nil, fmt.Errorf("payment observation: RefreshConfirmations requires a raw *gorm.DB session")
	}
	if chainNamespace == "" || chainReference == "" {
		return nil, fmt.Errorf("payment observation: chain namespace and reference must be set")
	}
	if requiredConfirmations < 0 {
		return nil, fmt.Errorf("payment observation: requiredConfirmations must be ≥ 0, got %d", requiredConfirmations)
	}

	// Threshold: a row inserted at block N becomes confirmed once the head
	// reaches N + requiredConfirmations, i.e. when block_number ≤ head -
	// requiredConfirmations. We reject blockNumber values < required to
	// avoid a negative threshold matching everything (the early chain
	// problem on devnets).
	threshold := currentBlockNumber - int64(requiredConfirmations)
	if threshold < 0 {
		// No row can possibly satisfy the depth yet; this is a no-op but
		// not an error — chain just started, scheduler will retry next tick.
		r.refreshPendingMetric(chainNamespace, chainReference)
		return nil, nil
	}

	var refs []contracts.OrderRef
	err := r.raw.Transaction(func(tx *gorm.DB) error {
		// Step 1: snapshot of (tenant, order) for rows about to flip. Even
		// if a concurrent INSERT lands a new pending row before our UPDATE,
		// it cannot land with block_number ≤ threshold AND status='pending'
		// AND already deep enough — the chain head was below threshold a
		// moment ago by definition.
		type tup struct {
			TenantID string
			OrderID  string
		}
		var tuples []tup
		if err := tx.
			Model(&models.PaymentObservation{}).
			Distinct("tenant_id", "order_id").
			Where("chain_namespace = ? AND chain_reference = ? AND status = ? AND block_number <= ?",
				chainNamespace, chainReference, models.PaymentObservationStatusPending, threshold).
			Find(&tuples).Error; err != nil {
			return fmt.Errorf("snapshot pending tuples: %w", err)
		}

		// Step 2: UPDATE pending → confirmed.
		//
		// We bound Confirmations at requiredConfirmations to keep the
		// gauge meaningful. A row that's been on chain for a year would
		// otherwise read as "Confirmations=2,628,000" which is correct
		// but useless for downstream UX.
		updateRes := tx.
			Model(&models.PaymentObservation{}).
			Where("chain_namespace = ? AND chain_reference = ? AND status = ? AND block_number <= ?",
				chainNamespace, chainReference, models.PaymentObservationStatusPending, threshold).
			Updates(map[string]interface{}{
				"status":        models.PaymentObservationStatusConfirmed,
				"confirmations": requiredConfirmations,
			})
		if updateRes.Error != nil {
			return fmt.Errorf("transition pending→confirmed: %w", updateRes.Error)
		}

		// Map tuples → OrderRef and dedupe (DISTINCT in SQL above already
		// deduplicates, but if the dialect doesn't honour Distinct on
		// composite columns we want to be defensive).
		seen := make(map[contracts.OrderRef]struct{}, len(tuples))
		for _, t := range tuples {
			ref := contracts.OrderRef{TenantID: t.TenantID, OrderID: t.OrderID}
			if _, ok := seen[ref]; ok {
				continue
			}
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Stable order: tenant_id, then order_id. Avoids a flaky test surface
	// across SQLite/PG and gives downstream consumers a predictable
	// fan-out order.
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].TenantID != refs[j].TenantID {
			return refs[i].TenantID < refs[j].TenantID
		}
		return refs[i].OrderID < refs[j].OrderID
	})
	r.refreshPendingMetric(chainNamespace, chainReference)
	return refs, nil
}

func (r *GormPaymentObservationRepo) refreshPendingMetric(chainNamespace, chainReference string) {
	if r.raw == nil || chainNamespace == "" || chainReference == "" {
		return
	}
	var count int64
	if err := r.raw.
		Model(&models.PaymentObservation{}).
		Where("chain_namespace = ? AND chain_reference = ? AND status = ?",
			chainNamespace, chainReference, models.PaymentObservationStatusPending).
		Count(&count).Error; err != nil {
		return
	}
	paymentmetrics.SetPaymentObservationsPendingCount(chainNamespace, chainReference, count)
}
