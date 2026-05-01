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
	"gorm.io/gorm"
)

// OverlapPolicy decides what happens when a previous tick of the same Job
// is still running at the next interval boundary.
type OverlapPolicy int

const (
	// OverlapSkip drops the new tick (default; safe for idempotent jobs).
	OverlapSkip OverlapPolicy = iota
	// OverlapQueue is reserved for future use; currently behaves like Skip.
	OverlapQueue
)

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
	ErrAlreadyStarted    = errors.New("scheduler: already started")
	ErrInvalidJob        = errors.New("scheduler: invalid job")
	ErrDuplicateJob      = errors.New("scheduler: duplicate job name")
	ErrNodeFnNoRegistry  = errors.New("scheduler: NodeFn requires Config.Registry")
	ErrLeaseRequiresDB   = errors.New("scheduler: UseLease requires Config.DB")
	ErrHolderIDRequired  = errors.New("scheduler: Config.HolderID required")
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
		_ = j.GlobalFn(ctx)
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
