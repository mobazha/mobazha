package scheduler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/logging"
)

// --- test helpers ---

// stubNode is a minimal contracts.NodeService stub for scheduler tests.
// Its methods are never called; the scheduler only passes it as an opaque
// argument to NodeFn.
type stubNode struct{ id string }

func (s *stubNode) IdentityInfo() contracts.IdentityService              { return nil }
func (s *stubNode) Notification() contracts.NotificationService          { return nil }
func (s *stubNode) Order() contracts.OrderService                        { return nil }
func (s *stubNode) Listing() contracts.ListingService                    { return nil }
func (s *stubNode) Profile() contracts.ProfileService                    { return nil }
func (s *stubNode) Wallet() contracts.WalletService                      { return nil }
func (s *stubNode) Media() contracts.MediaService                        { return nil }
func (s *stubNode) Social() contracts.SocialService                      { return nil }
func (s *stubNode) MatrixChat() contracts.MatrixChatService              { return nil }
func (s *stubNode) Preferences() contracts.PreferencesService            { return nil }
func (s *stubNode) ExchangeRate() contracts.ExchangeRateService          { return nil }
func (s *stubNode) ShoppingCart() contracts.ShoppingCartService          { return nil }
func (s *stubNode) Wishlist() contracts.WishlistService                  { return nil }
func (s *stubNode) GuestOrder() contracts.GuestOrderService              { return nil }
func (s *stubNode) ReceivingAccounts() contracts.ReceivingAccountService { return nil }
func (s *stubNode) PaymentSession() contracts.PaymentSessionService      { return nil }
func (s *stubNode) EventBus() events.Bus                                 { return nil }
func (s *stubNode) Publish(_ chan<- struct{})                            {}
func (s *stubNode) PingNode(_ context.Context, _ peer.ID) error          { return nil }
func (s *stubNode) SubscribeEvent(_ any) (events.Subscription, error) {
	return nil, nil
}

// stubRegistry returns a fixed set of nodes.
type stubRegistry struct {
	mu    sync.RWMutex
	nodes []contracts.NodeService
}

func (r *stubRegistry) GetNodesSnapshot() []contracts.NodeService {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]contracts.NodeService, len(r.nodes))
	copy(out, r.nodes)
	return out
}

func newStubRegistry(ids ...string) *stubRegistry {
	r := &stubRegistry{}
	for _, id := range ids {
		r.nodes = append(r.nodes, &stubNode{id: id})
	}
	return r
}

// --- tests ---

func TestJobs_PaymentReconcileScanIsSeparateFromVerification(t *testing.T) {
	obs, ok := Jobs["payment-reconcile-scan"]
	if !ok {
		t.Fatal("payment-reconcile-scan job is not registered")
	}
	if obs.Name != "payment-reconcile-scan" {
		t.Fatalf("reconcile job name = %q", obs.Name)
	}
	if obs.Interval != 30*time.Second {
		t.Fatalf("reconcile interval = %s, want 30s", obs.Interval)
	}

	verify, ok := Jobs["payment-verification"]
	if !ok {
		t.Fatal("payment-verification job is not registered")
	}
	if verify.Name == obs.Name {
		t.Fatal("payment reconcile scan and verification must remain separate scheduler jobs")
	}
}

func TestJobs_CollectibleMaintenanceJobs(t *testing.T) {
	release, ok := Jobs["collectible-primary-sale-release"]
	if !ok {
		t.Fatal("collectible-primary-sale-release job is not registered")
	}
	if release.Interval != 10*time.Second {
		t.Fatalf("release interval = %s, want 10s", release.Interval)
	}
	if release.PerNodeTimeout >= release.Interval {
		t.Fatalf("release timeout = %s must stay below interval %s", release.PerNodeTimeout, release.Interval)
	}

	reconcile, ok := Jobs["collectible-reconcile"]
	if !ok {
		t.Fatal("collectible-reconcile job is not registered")
	}
	if reconcile.Interval != 15*time.Minute {
		t.Fatalf("reconcile interval = %s, want 15m", reconcile.Interval)
	}
	if reconcile.PerNodeTimeout >= reconcile.Interval {
		t.Fatalf("reconcile timeout = %s must stay below interval %s", reconcile.PerNodeTimeout, reconcile.Interval)
	}
	if reconcile.Name == release.Name {
		t.Fatal("collectible release and reconcile jobs must remain separate")
	}
}

func TestJobs_MarketplaceDomainVerification(t *testing.T) {
	job, ok := Jobs["marketplace-domain-verification"]
	if !ok {
		t.Fatal("marketplace-domain-verification job is not registered")
	}
	if job.Interval != 10*time.Minute {
		t.Fatalf("domain verification interval = %s, want 10m", job.Interval)
	}
	if job.OverlapPolicy != OverlapSkip || job.MaxConcurrency != 1 {
		t.Fatalf("domain verification scheduling = overlap %v concurrency %d", job.OverlapPolicy, job.MaxConcurrency)
	}
	if job.PerNodeTimeout >= job.Interval {
		t.Fatalf("domain verification timeout = %s must stay below interval %s", job.PerNodeTimeout, job.Interval)
	}
}

func TestNew_RequiresHolderID(t *testing.T) {
	_, err := New(Config{})
	if err != ErrHolderIDRequired {
		t.Fatalf("expected ErrHolderIDRequired, got %v", err)
	}
}

func TestRegister_ValidatesJob(t *testing.T) {
	s, _ := New(Config{HolderID: "test"})

	tests := []struct {
		name string
		job  Job
		want error
	}{
		{"empty name", Job{Interval: time.Second}, ErrInvalidJob},
		{"zero interval", Job{Name: "x"}, ErrInvalidJob},
		{"both fns set", Job{Name: "x", Interval: time.Second,
			GlobalFn: func(context.Context) error { return nil },
			NodeFn:   func(context.Context, contracts.NodeService) error { return nil },
		}, ErrInvalidJob},
		{"neither fn set", Job{Name: "x", Interval: time.Second}, ErrInvalidJob},
		{"NodeFn without registry", Job{Name: "x", Interval: time.Second,
			NodeFn: func(context.Context, contracts.NodeService) error { return nil },
		}, ErrNodeFnNoRegistry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Register(tt.job)
			if err == nil || !containsError(err, tt.want) {
				t.Fatalf("got %v, want wrapping %v", err, tt.want)
			}
		})
	}
}

func TestRegister_DuplicateName(t *testing.T) {
	s, _ := New(Config{HolderID: "test"})
	j := Job{Name: "a", Interval: time.Second, GlobalFn: func(context.Context) error { return nil }}
	if err := s.Register(j); err != nil {
		t.Fatal(err)
	}
	if err := s.Register(j); !containsError(err, ErrDuplicateJob) {
		t.Fatalf("expected ErrDuplicateJob, got %v", err)
	}
}

func TestRegister_AfterStart(t *testing.T) {
	s, _ := New(Config{HolderID: "test"})
	j := Job{Name: "a", Interval: time.Second, GlobalFn: func(context.Context) error { return nil }}
	if err := s.Register(j); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = s.Start(ctx)
	defer s.Stop()

	err := s.Register(Job{Name: "b", Interval: time.Second, GlobalFn: func(context.Context) error { return nil }})
	if err != ErrAlreadyStarted {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}

func TestGlobalFn_Fires(t *testing.T) {
	var count atomic.Int32
	s, _ := New(Config{HolderID: "test"})
	_ = s.Register(Job{
		Name:     "counter",
		Interval: 20 * time.Millisecond,
		GlobalFn: func(context.Context) error { count.Add(1); return nil },
	})
	ctx, cancel := context.WithCancel(context.Background())
	_ = s.Start(ctx)

	time.Sleep(80 * time.Millisecond)
	cancel()
	s.Stop()

	got := count.Load()
	if got < 2 {
		t.Fatalf("expected at least 2 ticks, got %d", got)
	}
}

func TestGlobalFn_ErrorIsLogged(t *testing.T) {
	var logs bytes.Buffer
	oldLogConfig := logging.CurrentConfig()
	logging.Configure(logging.Config{
		Level:   logging.WARNING,
		Format:  logging.FormatText,
		Writers: []io.Writer{&logs},
	})
	t.Cleanup(func() { logging.Configure(oldLogConfig) })

	job := Job{
		Name:     "failing-global",
		Interval: time.Second,
		GlobalFn: func(context.Context) error {
			return errors.New("boom")
		},
	}
	raw, _ := New(Config{HolderID: "test"})
	s := raw.(*scheduler)
	if err := s.Register(job); err != nil {
		t.Fatal(err)
	}
	s.tick(context.Background(), job)

	got := logs.String()
	if !strings.Contains(got, "scheduler job") || !strings.Contains(got, "failing-global") || !strings.Contains(got, "boom") {
		t.Fatalf("log output = %q, want scheduler job failure", got)
	}
}

func TestNodeFn_FanOut(t *testing.T) {
	reg := newStubRegistry("n1", "n2", "n3")
	var seen sync.Map
	s, _ := New(Config{HolderID: "test", Registry: reg})
	_ = s.Register(Job{
		Name:           "fanout",
		Interval:       20 * time.Millisecond,
		MaxConcurrency: 3,
		NodeFn: func(_ context.Context, node contracts.NodeService) error {
			seen.Store(node.(*stubNode).id, true)
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	_ = s.Start(ctx)
	time.Sleep(60 * time.Millisecond)
	cancel()
	s.Stop()

	for _, id := range []string{"n1", "n2", "n3"} {
		if _, ok := seen.Load(id); !ok {
			t.Fatalf("node %s was not visited", id)
		}
	}
}

func TestOverlapSkip(t *testing.T) {
	var running atomic.Int32
	var maxConcurrent atomic.Int32
	s, _ := New(Config{HolderID: "test"})
	_ = s.Register(Job{
		Name:     "slow",
		Interval: 10 * time.Millisecond,
		GlobalFn: func(ctx context.Context) error {
			cur := running.Add(1)
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			running.Add(-1)
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	_ = s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()
	s.Stop()

	if mc := maxConcurrent.Load(); mc > 1 {
		t.Fatalf("expected overlap skip (max 1 concurrent), got %d", mc)
	}
}

func TestPerNodeTimeout(t *testing.T) {
	reg := newStubRegistry("slow")
	var timedOut atomic.Bool
	s, _ := New(Config{HolderID: "test", Registry: reg})
	_ = s.Register(Job{
		Name:           "timeout",
		Interval:       20 * time.Millisecond,
		PerNodeTimeout: 5 * time.Millisecond,
		NodeFn: func(ctx context.Context, _ contracts.NodeService) error {
			select {
			case <-ctx.Done():
				timedOut.Store(true)
			case <-time.After(time.Second):
			}
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	_ = s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
	s.Stop()

	if !timedOut.Load() {
		t.Fatal("expected per-node timeout to fire")
	}
}

// TestNodeFn_CtxCancelDuringFanOut verifies that cancelling ctx mid-fan-out
// does not deadlock. Regression test for an earlier bug where `break` in the
// fan-out select only exited the select (not the for), causing nodeWg.Add()
// without a matching sem token, deadlocking nodeWg.Wait().
func TestNodeFn_CtxCancelDuringFanOut(t *testing.T) {
	// Many nodes + concurrency=1 + slow NodeFn forces queueing on sem.
	ids := make([]string, 0, 50)
	for i := 0; i < 50; i++ {
		ids = append(ids, "n")
	}
	reg := newStubRegistry(ids...)

	s, _ := New(Config{HolderID: "test", Registry: reg})
	_ = s.Register(Job{
		Name:           "slow-fanout",
		Interval:       10 * time.Millisecond,
		MaxConcurrency: 1,
		NodeFn: func(ctx context.Context, _ contracts.NodeService) error {
			select {
			case <-ctx.Done():
			case <-time.After(50 * time.Millisecond):
			}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	_ = s.Start(ctx)
	// Let one tick start so a fan-out is in progress.
	time.Sleep(20 * time.Millisecond)
	cancel()

	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2s — likely deadlocked in fan-out")
	}
}

func TestStop_WaitsForJobs(t *testing.T) {
	var done atomic.Bool
	s, _ := New(Config{HolderID: "test"})
	_ = s.Register(Job{
		Name:     "block",
		Interval: 10 * time.Millisecond,
		GlobalFn: func(ctx context.Context) error {
			time.Sleep(50 * time.Millisecond)
			done.Store(true)
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	_ = s.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()
	s.Stop()

	if !done.Load() {
		t.Fatal("Stop returned before GlobalFn completed")
	}
}

func containsError(err, target error) bool {
	if err == target {
		return true
	}
	for e := err; e != nil; {
		if e.Error() == target.Error() {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
