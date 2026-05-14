//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ─────────────────────────────────────────────────────────────────────────
// Test doubles
// ─────────────────────────────────────────────────────────────────────────

// fakeObsRepo is an in-memory PaymentObservationRepo that records every
// InsertObservation call and exposes hooks for ListDeduplicatedConfirmed
// / RefreshConfirmations behaviours that the dispatcher contract cares
// about. The dispatcher tests intentionally avoid cross-testing the
// repo's persistence semantics — those have their own coverage in
// payment_observation_repo_gorm_test.go — so the fake stays minimal.
type fakeObsRepo struct {
	mu sync.Mutex

	inserted []*models.PaymentObservation
	// dedupe key -> already inserted (used to surface
	// ErrDuplicateObservation on retry)
	seen map[string]struct{}

	// Optional injection points for negative-path tests.
	insertErr      error
	refreshErr     error
	refreshAffects []contracts.OrderRef
}

func newFakeObsRepo() *fakeObsRepo {
	return &fakeObsRepo{seen: make(map[string]struct{})}
}

func (r *fakeObsRepo) InsertObservation(_ context.Context, obs *models.PaymentObservation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.insertErr != nil {
		return r.insertErr
	}
	key := dedupeKey(obs)
	if _, ok := r.seen[key]; ok {
		return contracts.ErrDuplicateObservation
	}
	r.seen[key] = struct{}{}
	clone := *obs
	r.inserted = append(r.inserted, &clone)
	return nil
}

func (r *fakeObsRepo) ListDeduplicatedConfirmed(_ context.Context, _, _ string) ([]models.PaymentObservation, error) {
	return nil, errors.New("fakeObsRepo.ListDeduplicatedConfirmed not used in dispatcher tests")
}

func (r *fakeObsRepo) ListByOrder(_ context.Context, _, _ string) ([]models.PaymentObservation, error) {
	return nil, errors.New("fakeObsRepo.ListByOrder not used in dispatcher tests")
}

func (r *fakeObsRepo) RefreshConfirmations(_ context.Context, _, _ string, _ int64, _ int) ([]contracts.OrderRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.refreshErr != nil {
		return nil, r.refreshErr
	}
	out := make([]contracts.OrderRef, len(r.refreshAffects))
	copy(out, r.refreshAffects)
	return out, nil
}

func (r *fakeObsRepo) snapshot() []*models.PaymentObservation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*models.PaymentObservation, len(r.inserted))
	for i, obs := range r.inserted {
		clone := *obs
		out[i] = &clone
	}
	return out
}

func dedupeKey(obs *models.PaymentObservation) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%s",
		obs.TenantID, obs.ChainNamespace, obs.ChainReference, obs.TxHash, obs.EventIndex, obs.Observer)
}

// fakeAggregator records every (tenantID, orderID) pair received and
// optionally fails the next call. Tests use it to verify the dispatcher
// invokes the verifier after each successful insert, and that aggregator
// errors propagate / are joined as documented.
type fakeAggregator struct {
	mu       sync.Mutex
	calls    []contracts.OrderRef
	errOnce  error
	errAlways error
}

func (a *fakeAggregator) AggregateAndEmit(_ context.Context, tenantID, orderID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, contracts.OrderRef{TenantID: tenantID, OrderID: orderID})
	if a.errOnce != nil {
		err := a.errOnce
		a.errOnce = nil
		return err
	}
	return a.errAlways
}

func (a *fakeAggregator) snapshot() []contracts.OrderRef {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]contracts.OrderRef, len(a.calls))
	copy(out, a.calls)
	return out
}

// fakeTenantResolver maps orderID → tenantID via a constant map; missing
// lookups return ErrUnknownOrder so tests can exercise the "ignore
// non-Mobazha funding" branch.
type fakeTenantResolver struct {
	tenants map[string]string
	err     error
}

func (r *fakeTenantResolver) ResolveTenant(_ context.Context, orderID string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if t, ok := r.tenants[orderID]; ok {
		return t, nil
	}
	return "", ErrUnknownOrder
}

// validFundingEvent returns a populated FundingEvent that all tests can
// mutate. Centralising the default avoids "missing field" errors leaking
// into negative-path tests when a new required field is added later.
func validFundingEvent() FundingEvent {
	return FundingEvent{
		OrderID:        "order-1",
		ChainNamespace: "eip155",
		ChainReference: "1",
		TxHash:         "0xabc",
		EventIndex:     0,
		EventType:      models.PaymentEventManagedEscrowReceived,
		FromAddress:    "0xbeef",
		ToAddress:      "0xmanagedescrow",
		TokenAddress:   "",
		Amount:         big.NewInt(1_000_000_000_000_000_000), // 1 ETH
		BlockNumber:    100,
		BlockTime:      time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
	}
}

// newDispatcher wires up the test doubles with sane defaults; individual
// tests poke at them via the returned references.
func newDispatcher(t *testing.T, tenantsByOrder map[string]string) (
	*ObservationDispatcher,
	*fakeObsRepo,
	*fakeAggregator,
	*fakeTenantResolver,
) {
	t.Helper()
	repo := newFakeObsRepo()
	agg := &fakeAggregator{}
	res := &fakeTenantResolver{tenants: tenantsByOrder}
	d := NewObservationDispatcher(repo, agg, res, "worker-A").
		withClock(func() time.Time { return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC) })
	return d, repo, agg, res
}

// ─────────────────────────────────────────────────────────────────────────
// Constructor: nil arguments panic
// ─────────────────────────────────────────────────────────────────────────

func TestNewObservationDispatcher_NilRepoPanics(t *testing.T) {
	require.PanicsWithValue(t,
		"payment: NewObservationDispatcher requires a non-nil PaymentObservationRepo",
		func() {
			NewObservationDispatcher(nil, &fakeAggregator{}, &fakeTenantResolver{}, "w")
		})
}

func TestNewObservationDispatcher_NilAggregatorPanics(t *testing.T) {
	require.PanicsWithValue(t,
		"payment: NewObservationDispatcher requires a non-nil PaymentAggregator",
		func() {
			NewObservationDispatcher(newFakeObsRepo(), nil, &fakeTenantResolver{}, "w")
		})
}

func TestNewObservationDispatcher_NilResolverPanics(t *testing.T) {
	require.PanicsWithValue(t,
		"payment: NewObservationDispatcher requires a non-nil TenantResolver",
		func() {
			NewObservationDispatcher(newFakeObsRepo(), &fakeAggregator{}, nil, "w")
		})
}

func TestNewObservationDispatcher_BlankWorkerIDPanics(t *testing.T) {
	require.PanicsWithValue(t,
		"payment: NewObservationDispatcher requires a non-empty workerID",
		func() {
			NewObservationDispatcher(newFakeObsRepo(), &fakeAggregator{}, &fakeTenantResolver{}, "  \n")
		})
}

// ─────────────────────────────────────────────────────────────────────────
// OnFundingEvent — happy path
// ─────────────────────────────────────────────────────────────────────────

func TestObservationDispatcher_OnFundingEvent_PersistsObservationRow(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, map[string]string{"order-1": "tenant-1"})

	require.NoError(t, d.OnFundingEvent(context.Background(), validFundingEvent()))

	rows := repo.snapshot()
	require.Len(t, rows, 1)
	got := rows[0]

	require.Equal(t, "tenant-1", got.TenantID)
	require.Equal(t, "order-1", got.OrderID)
	require.Equal(t, "eip155", got.ChainNamespace)
	require.Equal(t, "1", got.ChainReference)
	require.Equal(t, "0xabc", got.TxHash)
	require.Equal(t, 0, got.EventIndex)
	require.Equal(t, models.PaymentEventManagedEscrowReceived, got.EventType)
	require.Equal(t, "0xbeef", got.FromAddress)
	require.Equal(t, "0xmanagedescrow", got.ToAddress)
	require.Empty(t, got.TokenAddress)
	require.Equal(t, "1000000000000000000", got.Amount)
	require.Equal(t, int64(100), got.BlockNumber)
	require.Equal(t, models.PaymentObservationSourceMonitor, got.Source)
	require.Equal(t, "monitor:eip155:1:worker-A", got.Observer)
	require.Equal(t, models.PaymentObservationStatusPending, got.Status)
	require.Equal(t, 0, got.Confirmations)
	require.NotEmpty(t, got.ID, "dispatcher must assign a UUID per row")

	calls := agg.snapshot()
	require.Equal(t, []contracts.OrderRef{{TenantID: "tenant-1", OrderID: "order-1"}}, calls)
}

func TestObservationDispatcher_OnFundingEvent_ERC20EventCarriesTokenContract(t *testing.T) {
	d, repo, _, _ := newDispatcher(t, map[string]string{"order-1": "tenant-1"})

	evt := validFundingEvent()
	evt.EventType = models.PaymentEventERC20Transfer
	evt.TokenAddress = "0xUSDC"
	evt.EventIndex = 7

	require.NoError(t, d.OnFundingEvent(context.Background(), evt))

	rows := repo.snapshot()
	require.Len(t, rows, 1)
	require.Equal(t, models.PaymentEventERC20Transfer, rows[0].EventType)
	require.Equal(t, "0xUSDC", rows[0].TokenAddress)
	require.Equal(t, 7, rows[0].EventIndex)
}

// ─────────────────────────────────────────────────────────────────────────
// OnFundingEvent — idempotency
// ─────────────────────────────────────────────────────────────────────────

func TestObservationDispatcher_OnFundingEvent_DuplicateInsertSwallowsAndSkipsAggregator(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, map[string]string{"order-1": "tenant-1"})

	require.NoError(t, d.OnFundingEvent(context.Background(), validFundingEvent()))
	require.NoError(t, d.OnFundingEvent(context.Background(), validFundingEvent()),
		"second insert from same observer must be a silent no-op")

	require.Len(t, repo.snapshot(), 1, "dedupe tuple collapsed in fake repo")
	require.Len(t, agg.snapshot(), 1,
		"aggregator MUST NOT be called for duplicate observations — design §5.1 idempotency contract")
}

// ─────────────────────────────────────────────────────────────────────────
// OnFundingEvent — unknown order
// ─────────────────────────────────────────────────────────────────────────

func TestObservationDispatcher_OnFundingEvent_UnknownOrderIsIgnored(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, map[string]string{}) // empty registry

	err := d.OnFundingEvent(context.Background(), validFundingEvent())
	require.NoError(t, err, "non-Mobazha funding must not surface as caller error")

	require.Empty(t, repo.snapshot(), "no observation persisted for unknown order")
	require.Empty(t, agg.snapshot(), "aggregator must not be triggered for unknown orders")
}

func TestObservationDispatcher_OnFundingEvent_TenantResolverErrorPropagates(t *testing.T) {
	d, _, agg, res := newDispatcher(t, nil)
	res.err = errors.New("db down")

	err := d.OnFundingEvent(context.Background(), validFundingEvent())
	require.Error(t, err)
	require.Contains(t, err.Error(), "resolve tenant for order order-1")
	require.Contains(t, err.Error(), "db down")
	require.Empty(t, agg.snapshot(), "aggregator must not run when tenant resolution fails")
}

func TestObservationDispatcher_OnFundingEvent_BlankTenantRejected(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, map[string]string{"order-1": "   "})

	err := d.OnFundingEvent(context.Background(), validFundingEvent())
	require.Error(t, err)
	require.Contains(t, err.Error(), "TenantResolver returned empty tenant")
	require.Empty(t, repo.snapshot())
	require.Empty(t, agg.snapshot())
}

// ─────────────────────────────────────────────────────────────────────────
// OnFundingEvent — validation
// ─────────────────────────────────────────────────────────────────────────

func TestObservationDispatcher_OnFundingEvent_RejectsInvalidEvents(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*FundingEvent)
		expect string
	}{
		{"empty OrderID", func(e *FundingEvent) { e.OrderID = "" }, "empty OrderID"},
		{"empty ChainNamespace", func(e *FundingEvent) { e.ChainNamespace = "" }, "empty ChainNamespace"},
		{"empty ChainReference", func(e *FundingEvent) { e.ChainReference = "" }, "empty ChainReference"},
		{"empty TxHash", func(e *FundingEvent) { e.TxHash = "" }, "empty TxHash"},
		{"negative EventIndex", func(e *FundingEvent) { e.EventIndex = -1 }, "negative EventIndex"},
		{"empty EventType", func(e *FundingEvent) { e.EventType = "" }, "empty EventType"},
		{"empty ToAddress", func(e *FundingEvent) { e.ToAddress = "" }, "empty ToAddress"},
		{"nil Amount", func(e *FundingEvent) { e.Amount = nil }, "nil Amount"},
		{"zero Amount", func(e *FundingEvent) { e.Amount = big.NewInt(0) }, "Amount must be > 0"},
		{"negative Amount", func(e *FundingEvent) { e.Amount = big.NewInt(-5) }, "Amount must be > 0"},
		{"zero BlockNumber", func(e *FundingEvent) { e.BlockNumber = 0 }, "BlockNumber must be > 0"},
		{"zero BlockTime", func(e *FundingEvent) { e.BlockTime = time.Time{} }, "BlockTime must be set"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, repo, agg, _ := newDispatcher(t, map[string]string{"order-1": "tenant-1"})
			evt := validFundingEvent()
			tc.mutate(&evt)

			err := d.OnFundingEvent(context.Background(), evt)
			require.ErrorIs(t, err, ErrInvalidFundingEvent)
			require.Contains(t, err.Error(), tc.expect)
			require.Empty(t, repo.snapshot(), "rejected events must not persist")
			require.Empty(t, agg.snapshot(), "rejected events must not trigger aggregator")
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────
// OnFundingEvent — aggregator failure / persistence failure
// ─────────────────────────────────────────────────────────────────────────

func TestObservationDispatcher_OnFundingEvent_RepoErrorWrapped(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, map[string]string{"order-1": "tenant-1"})
	repo.insertErr = errors.New("database is locked")

	err := d.OnFundingEvent(context.Background(), validFundingEvent())
	require.Error(t, err)
	require.Contains(t, err.Error(), "insert observation for order order-1 tx 0xabc")
	require.Contains(t, err.Error(), "database is locked")
	require.Empty(t, agg.snapshot(), "aggregator must not run when insert fails")
}

func TestObservationDispatcher_OnFundingEvent_AggregatorErrorPropagates(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, map[string]string{"order-1": "tenant-1"})
	agg.errAlways = errors.New("verifier offline")

	err := d.OnFundingEvent(context.Background(), validFundingEvent())
	require.Error(t, err)
	require.Contains(t, err.Error(), "aggregate after insert")
	require.Contains(t, err.Error(), "verifier offline")
	require.Len(t, repo.snapshot(), 1, "observation IS persisted even though aggregator fails")
}

// ─────────────────────────────────────────────────────────────────────────
// OnNewBlock
// ─────────────────────────────────────────────────────────────────────────

func TestObservationDispatcher_OnNewBlock_TriggersAggregatorForAffectedOrders(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, nil)
	repo.refreshAffects = []contracts.OrderRef{
		{TenantID: "tenant-1", OrderID: "order-1"},
		{TenantID: "tenant-1", OrderID: "order-2"},
		{TenantID: "tenant-2", OrderID: "order-3"},
	}

	require.NoError(t, d.OnNewBlock(context.Background(), "eip155", "1", 200, 12))
	require.Equal(t, repo.refreshAffects, agg.snapshot(),
		"aggregator must be invoked once per affected order, in repo-returned order")
}

func TestObservationDispatcher_OnNewBlock_NoAffectedOrdersIsNoOp(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, nil)
	repo.refreshAffects = nil

	require.NoError(t, d.OnNewBlock(context.Background(), "eip155", "1", 200, 12))
	require.Empty(t, agg.snapshot())
}

func TestObservationDispatcher_OnNewBlock_RefreshErrorWrapped(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, nil)
	repo.refreshErr = errors.New("conn reset")

	err := d.OnNewBlock(context.Background(), "eip155", "1", 200, 12)
	require.Error(t, err)
	require.Contains(t, err.Error(), "refresh confirmations on eip155:1")
	require.Contains(t, err.Error(), "conn reset")
	require.Empty(t, agg.snapshot(),
		"aggregator must not be called when RefreshConfirmations fails")
}

func TestObservationDispatcher_OnNewBlock_AggregatorFailuresAreJoined(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, nil)
	repo.refreshAffects = []contracts.OrderRef{
		{TenantID: "tenant-1", OrderID: "order-1"},
		{TenantID: "tenant-1", OrderID: "order-2"},
		{TenantID: "tenant-2", OrderID: "order-3"},
	}
	agg.errAlways = errors.New("verifier offline")

	err := d.OnNewBlock(context.Background(), "eip155", "1", 200, 12)
	require.Error(t, err)

	// Every order must have been visited regardless of failures —
	// design §5.1 forbids short-circuiting on the first aggregator error.
	require.Equal(t, repo.refreshAffects, agg.snapshot(),
		"all affected orders must be visited even when aggregator fails on each")

	require.Contains(t, err.Error(), "tenant=tenant-1 order=order-1")
	require.Contains(t, err.Error(), "tenant=tenant-1 order=order-2")
	require.Contains(t, err.Error(), "tenant=tenant-2 order=order-3")
}

func TestObservationDispatcher_OnNewBlock_FirstFailureDoesNotMaskSuccess(t *testing.T) {
	d, repo, agg, _ := newDispatcher(t, nil)
	repo.refreshAffects = []contracts.OrderRef{
		{TenantID: "tenant-1", OrderID: "order-1"},
		{TenantID: "tenant-2", OrderID: "order-2"},
	}
	agg.errOnce = errors.New("transient")

	err := d.OnNewBlock(context.Background(), "eip155", "1", 200, 12)
	require.Error(t, err, "even one transient failure must surface")
	require.Contains(t, err.Error(), "tenant=tenant-1 order=order-1")
	require.NotContains(t, err.Error(), "tenant=tenant-2",
		"order-2 succeeded; only order-1 should be in the joined error")
	require.Equal(t, 2, len(agg.snapshot()), "both orders visited despite first failing")
}

func TestObservationDispatcher_OnNewBlock_RejectsBlankChainArgs(t *testing.T) {
	d, _, _, _ := newDispatcher(t, nil)

	require.Error(t, d.OnNewBlock(context.Background(), "", "1", 100, 12))
	require.Error(t, d.OnNewBlock(context.Background(), "eip155", "  ", 100, 12))
}
