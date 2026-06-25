// Package scheduler provides a process-wide shared scheduler for periodic
// jobs that previously ran as per-tenant goroutines.
//
// Phase AH-3a (Foundation):
//   - Replace N*M tickers (N tenants × M workers) with a single ticker per Job.
//   - Iterate active tenants once per Job tick using contracts.NodeRegistry.
//   - Optional lease-based locking via scheduler_locks (multi-instance safety).
//
// Non-goals for AH-3a:
//   - Cross-tenant due-item scanning (deferred to AH-3b).
//   - Eviction-aware SLA for inactive tenants (deferred to AH-3c).
//
// Wiring: hosting (SaaS) instantiates one Scheduler at process start, registers
// Jobs after default node creation (so contracts.NodeRegistry has at least the
// default node), then calls Start. Stop must be called before tenant nodes
// are stopped to avoid Job iterating against half-stopped nodes.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	"gorm.io/gorm"
)

var schedulerLog = logging.MustGetLogger("SCHED")

// OverlapPolicy decides what happens when a previous tick of the same Job
// is still running at the next interval boundary.
type OverlapPolicy int

const (
	// OverlapSkip drops the new tick (default; safe for idempotent jobs).
	OverlapSkip OverlapPolicy = iota
	// OverlapQueue is reserved for future use; currently behaves like Skip.
	OverlapQueue
)

// JobMeta holds the canonical metadata for a scheduled job. Both SaaS
// (hosting scheduler_jobs.go) and standalone (standalone_scheduler.go)
// reference the Jobs registry so that name, interval, concurrency, and
// timeout stay in sync across deployment modes.
//
// ARCHITECTURAL CONSTRAINT: SchedulerHooks is a typed interface — each
// job maps to a dedicated Run*Once method. Do NOT replace this with a
// generic ProcessScheduledWork(jobName) dispatch. The compiler-enforced
// 1:1 mapping between job name and typed method prevents silent drift.
type JobMeta struct {
	Name           string
	Interval       time.Duration
	OverlapPolicy  OverlapPolicy
	MaxConcurrency int           // NodeFn parallelism cap; standalone (GlobalFn) ignores this.
	PerNodeTimeout time.Duration // Single NodeFn invocation timeout; must be < Interval.
}

// Jobs is the single-source-of-truth registry for all scheduler-driven
// workers. The map key equals JobMeta.Name.
var Jobs = map[string]JobMeta{
	"order-timeout":                    {Name: "order-timeout", Interval: 1 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 30 * time.Second},
	"outbox-poll":                      {Name: "outbox-poll", Interval: 5 * time.Second, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 4 * time.Second},
	"outbox-cleanup":                   {Name: "outbox-cleanup", Interval: 1 * time.Hour, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 30 * time.Second},
	"payment-reconcile-scan":           {Name: "payment-reconcile-scan", Interval: 30 * time.Second, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 25 * time.Second},
	"payment-verification":             {Name: "payment-verification", Interval: 30 * time.Second, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 25 * time.Second},
	"settlement-action-confirmations":  {Name: "settlement-action-confirmations", Interval: 10 * time.Second, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 8 * time.Second},
	"collectible-primary-sale-release": {Name: "collectible-primary-sale-release", Interval: 10 * time.Second, OverlapPolicy: OverlapSkip, MaxConcurrency: 1, PerNodeTimeout: 8 * time.Second},
	"collectible-reconcile":            {Name: "collectible-reconcile", Interval: 15 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 1, PerNodeTimeout: 2 * time.Minute},
	"webhook-delivery":                 {Name: "webhook-delivery", Interval: 5 * time.Second, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 4 * time.Second},
	"webhook-cleanup":                  {Name: "webhook-cleanup", Interval: 1 * time.Hour, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 30 * time.Second},
	"analytics-cleanup":                {Name: "analytics-cleanup", Interval: 24 * time.Hour, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 60 * time.Second},
	"fiat-reconciliation":              {Name: "fiat-reconciliation", Interval: 2 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 90 * time.Second},
	"fiat-cleanup":                     {Name: "fiat-cleanup", Interval: 24 * time.Hour, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 30 * time.Second},
	"guest-order-cleanup":              {Name: "guest-order-cleanup", Interval: 1 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 30 * time.Second},
	"follower-connect":                 {Name: "follower-connect", Interval: 30 * time.Second, OverlapPolicy: OverlapSkip, MaxConcurrency: 4, PerNodeTimeout: 25 * time.Second},
	"netdb-reconcile":                  {Name: "netdb-reconcile", Interval: 10 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 60 * time.Second},
	"order-lock-cleanup":               {Name: "order-lock-cleanup", Interval: 30 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 30 * time.Second},

	"supply-chain-retry":           {Name: "supply-chain-retry", Interval: 30 * time.Second, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 25 * time.Second},
	"supply-chain-reconcile":       {Name: "supply-chain-reconcile", Interval: 5 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 4 * time.Minute},
	"supply-chain-cleanup":         {Name: "supply-chain-cleanup", Interval: 1 * time.Hour, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 30 * time.Second},
	"supply-chain-inventory-check": {Name: "supply-chain-inventory-check", Interval: 5 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 4 * time.Minute},
	"supply-chain-price-drift":     {Name: "supply-chain-price-drift", Interval: 30 * time.Minute, OverlapPolicy: OverlapSkip, MaxConcurrency: 2, PerNodeTimeout: 25 * time.Minute},
}

// Job describes a scheduled task.
//
// Exactly one of GlobalFn or NodeFn must be set:
//   - GlobalFn runs once per tick (e.g. cross-tenant DB scan, FiatRecon).
//   - NodeFn runs once per active node per tick (e.g. OrderTimeout iteration).
//
// MaxConcurrency caps NodeFn parallelism (≤0 means 1 — sequential).
// PerNodeTimeout aborts a single NodeFn invocation that exceeds the budget.
// UseLease guards GlobalFn against multi-instance races via scheduler_locks.
type Job struct {
	Name           string
	Interval       time.Duration
	GlobalFn       func(ctx context.Context) error
	NodeFn         func(ctx context.Context, node contracts.NodeService) error
	MaxConcurrency int
	PerNodeTimeout time.Duration
	OverlapPolicy  OverlapPolicy
	UseLease       bool
}

// Scheduler is a process-wide shared scheduler. Implementations must be safe
// for concurrent Register / Start / Stop, but Register may not be called after
// Start (return ErrAlreadyStarted).
type Scheduler interface {
	Register(job Job) error
	Start(ctx context.Context) error
	Stop()
}

// Config holds the wiring parameters for a Scheduler instance.
type Config struct {
	// HolderID identifies this process for lease-based locking.
	// Required; pass a stable identifier such as hostname+pid.
	HolderID string

	// Registry exposes active nodes for NodeFn jobs. May be nil if only
	// GlobalFn jobs are registered.
	Registry contracts.NodeRegistry

	// DB backs scheduler_locks. May be nil if no Job sets UseLease=true.
	// The caller must run MigrateLocks(db) before Start.
	DB *gorm.DB

	// LeaseTTL is the per-Job lease duration. Defaults to 30s when zero.
	LeaseTTL time.Duration

	// Now is the time provider (override for tests). Defaults to time.Now.
	Now func() time.Time
}

// Errors returned by Scheduler.
var (
	ErrAlreadyStarted   = errors.New("scheduler: already started")
	ErrInvalidJob       = errors.New("scheduler: invalid job")
	ErrDuplicateJob     = errors.New("scheduler: duplicate job name")
	ErrNodeFnNoRegistry = errors.New("scheduler: NodeFn requires Config.Registry")
	ErrLeaseRequiresDB  = errors.New("scheduler: UseLease requires Config.DB")
	ErrHolderIDRequired = errors.New("scheduler: Config.HolderID required")
)

// New constructs a Scheduler with the given config.
func New(cfg Config) (Scheduler, error) {
	if cfg.HolderID == "" {
		return nil, ErrHolderIDRequired
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 30 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &scheduler{
		cfg:      cfg,
		jobs:     make(map[string]Job),
		inflight: make(map[string]*sync.Mutex),
	}, nil
}

type scheduler struct {
	cfg Config

	mu       sync.RWMutex
	jobs     map[string]Job
	inflight map[string]*sync.Mutex // per-Job mutex for OverlapSkip
	started  bool

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (s *scheduler) Register(j Job) error {
	if err := validateJob(j, s.cfg); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return ErrAlreadyStarted
	}
	if _, dup := s.jobs[j.Name]; dup {
		return fmt.Errorf("%w: %s", ErrDuplicateJob, j.Name)
	}
	s.jobs[j.Name] = j
	s.inflight[j.Name] = &sync.Mutex{}
	return nil
}

func (s *scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return ErrAlreadyStarted
	}
	s.started = true
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	jobs := make([]Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	s.mu.Unlock()

	for _, j := range jobs {
		s.wg.Add(1)
		go s.runJob(runCtx, j)
	}
	return nil
}

func (s *scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *scheduler) runJob(ctx context.Context, j Job) {
	defer s.wg.Done()
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx, j)
		}
	}
}

func (s *scheduler) tick(ctx context.Context, j Job) {
	s.mu.RLock()
	inflight := s.inflight[j.Name]
	s.mu.RUnlock()

	// OverlapSkip: if previous tick is still running, drop this one.
	if !inflight.TryLock() {
		return
	}
	defer inflight.Unlock()

	// Lease guard for multi-instance safety.
	if j.UseLease {
		ok, err := tryAcquire(s.cfg.DB, j.Name, s.cfg.HolderID, s.cfg.LeaseTTL, s.cfg.Now)
		if err != nil || !ok {
			return
		}
		defer func() { _ = release(s.cfg.DB, j.Name, s.cfg.HolderID) }()
	}

	if j.GlobalFn != nil {
		if err := j.GlobalFn(ctx); err != nil {
			schedulerLog.Warningf("scheduler job %q failed: %v", j.Name, err)
		}
		return
	}

	// NodeFn fan-out.
	nodes := s.cfg.Registry.GetNodesSnapshot()
	if len(nodes) == 0 {
		return
	}
	concurrency := j.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var nodeWg sync.WaitGroup
loop:
	for _, n := range nodes {
		select {
		case <-ctx.Done():
			// Break out of the for loop; do not Add() more workers without
			// a matching sem token (would deadlock the goroutine's <-sem).
			break loop
		case sem <- struct{}{}:
		}
		nodeWg.Add(1)
		go func(node contracts.NodeService) {
			defer nodeWg.Done()
			defer func() { <-sem }()

			// Per-(job, tenant) work lock: prevents double-running across
			// multiple hosting instances. Only active when DB is configured.
			tenantID := ""
			if id := node.IdentityInfo(); id != nil {
				tenantID = id.GetNodeID()
			}
			if s.cfg.DB != nil && tenantID != "" {
				ok, claimErr := TryClaimWork(s.cfg.DB, j.Name, tenantID, s.cfg.HolderID, s.cfg.LeaseTTL, s.cfg.Now)
				if claimErr != nil || !ok {
					return
				}

				workCtx, workCancel := context.WithCancel(ctx)
				renewer := StartLeaseRenewer(ctx, workCancel, s.cfg.DB, j.Name, tenantID, s.cfg.HolderID, s.cfg.LeaseTTL, s.cfg.Now)
				defer renewer.Stop()
				defer func() { _ = ReleaseWork(s.cfg.DB, j.Name, tenantID, s.cfg.HolderID) }()

				nodeCtx := workCtx
				if j.PerNodeTimeout > 0 {
					var cancel context.CancelFunc
					nodeCtx, cancel = context.WithTimeout(workCtx, j.PerNodeTimeout)
					defer cancel()
				}
				_ = j.NodeFn(nodeCtx, node)
				workCancel()
				return
			}

			nodeCtx := ctx
			if j.PerNodeTimeout > 0 {
				var cancel context.CancelFunc
				nodeCtx, cancel = context.WithTimeout(ctx, j.PerNodeTimeout)
				defer cancel()
			}
			_ = j.NodeFn(nodeCtx, node)
		}(n)
	}
	nodeWg.Wait()
}

func validateJob(j Job, cfg Config) error {
	if j.Name == "" {
		return fmt.Errorf("%w: empty Name", ErrInvalidJob)
	}
	if j.Interval <= 0 {
		return fmt.Errorf("%w: %s Interval must be > 0", ErrInvalidJob, j.Name)
	}
	if (j.GlobalFn == nil) == (j.NodeFn == nil) {
		return fmt.Errorf("%w: %s must specify exactly one of GlobalFn / NodeFn", ErrInvalidJob, j.Name)
	}
	if j.NodeFn != nil && cfg.Registry == nil {
		return fmt.Errorf("%w: %s", ErrNodeFnNoRegistry, j.Name)
	}
	if j.UseLease && cfg.DB == nil {
		return fmt.Errorf("%w: %s", ErrLeaseRequiresDB, j.Name)
	}
	return nil
}
