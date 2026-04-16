package supervisor

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/supervisor/updater"
)

// --- helpers ---

func newTestSupervisor(t *testing.T, healthHandler http.Handler) *Supervisor {
	t.Helper()
	ts := httptest.NewServer(healthHandler)
	t.Cleanup(ts.Close)

	u, _ := url.Parse(ts.URL)
	dataDir := t.TempDir()
	logger := log.New(os.Stderr, "[test] ", 0)

	s := New(Options{
		DataDir:     dataDir,
		GatewayPort: u.Port(),
		Logger:      logger,
	})
	return s
}

type recordingUI struct {
	last atomic.Value
}

func (r *recordingUI) Run(s *Supervisor)           { <-s.ctx.Done() }
func (r *recordingUI) OnStatusChange(st Status)    { r.last.Store(st) }
func (r *recordingUI) lastStatus() Status {
	v := r.last.Load()
	if v == nil {
		return ""
	}
	return v.(Status)
}

// --- Integration: tick() coordination ---

func TestSupervisor_Tick_HealthyNode_WritesStatusFile(t *testing.T) {
	s := newTestSupervisor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]int{"unreadNotifications": 3})
	}))

	s.proc.mu.Lock()
	s.proc.running = true
	s.proc.mu.Unlock()

	s.tick()

	if s.GetStatus() != StatusRunning {
		t.Errorf("status = %q, want running", s.GetStatus())
	}

	status, err := ReadStatusFile(s.dataDir)
	if err != nil {
		t.Fatalf("ReadStatusFile: %v", err)
	}
	if status.LauncherVersion != Version {
		t.Errorf("LauncherVersion = %q, want %q", status.LauncherVersion, Version)
	}
	if status.LauncherPID == 0 {
		t.Error("LauncherPID should be non-zero")
	}
}

func TestSupervisor_Tick_HealthyNode_ResetsBackoff(t *testing.T) {
	s := newTestSupervisor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))

	s.proc.mu.Lock()
	s.proc.running = true
	s.proc.attempts = 3
	s.proc.mu.Unlock()

	s.tick()

	if s.proc.Attempts() != 0 {
		t.Errorf("after healthy tick, attempts = %d, want 0", s.proc.Attempts())
	}
}

func TestSupervisor_Tick_UnhealthyButRunning_StatusStarting(t *testing.T) {
	s := newTestSupervisor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	s.proc.mu.Lock()
	s.proc.running = true
	s.proc.mu.Unlock()

	s.tick()

	if s.GetStatus() != StatusStarting {
		t.Errorf("status = %q, want starting", s.GetStatus())
	}
}

func TestSupervisor_Tick_ProcessDead_MaxAttempts_StatusFailed(t *testing.T) {
	s := newTestSupervisor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	s.proc.mu.Lock()
	s.proc.attempts = maxRestartAttempts
	s.proc.mu.Unlock()

	s.tick()

	if s.GetStatus() != StatusFailed {
		t.Errorf("status = %q, want failed after max attempts", s.GetStatus())
	}
}

func TestSupervisor_Tick_ProcessDead_CancelledContext_NoBlock(t *testing.T) {
	s := newTestSupervisor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	s.cancel()

	done := make(chan struct{})
	go func() {
		s.tick()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("tick() should not block when context is cancelled")
	}
}

// --- Integration: buildStatusContent merges config + updater ---

func TestSupervisor_BuildStatusContent_MergesAllSources(t *testing.T) {
	s := newTestSupervisor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	cfg := LauncherConfig{AutoUpdateEnabled: false, CheckIntervalMin: 120, UpdateChannel: "beta"}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(s.dataDir, "launcher-config.json"), data, 0644)
	s.config.Reload()

	s.updater.mu.Lock()
	s.updater.status = UpdateAvailable
	s.updater.latest = &updater.ReleaseInfo{
		Version:    "2.0.0",
		ReleaseURL: "https://example.com/release",
		Notes:      "New features",
	}
	s.updater.progress = 50
	s.updater.mu.Unlock()

	content := s.buildStatusContent()

	if content.AutoUpdateEnabled {
		t.Error("AutoUpdateEnabled should be false from config")
	}
	if content.CheckIntervalMin != 120 {
		t.Errorf("CheckIntervalMin = %d, want 120", content.CheckIntervalMin)
	}
	if content.UpdateChannel != "beta" {
		t.Errorf("UpdateChannel = %q, want beta", content.UpdateChannel)
	}
	if content.UpdateStatus != "available" {
		t.Errorf("UpdateStatus = %q, want available", content.UpdateStatus)
	}
	if content.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %q, want 2.0.0", content.LatestVersion)
	}
	if content.LatestReleaseURL != "https://example.com/release" {
		t.Errorf("LatestReleaseURL = %q", content.LatestReleaseURL)
	}
	if content.ReleaseNotes != "New features" {
		t.Errorf("ReleaseNotes = %q", content.ReleaseNotes)
	}
	if content.DownloadProgress != 50 {
		t.Errorf("DownloadProgress = %d, want 50", content.DownloadProgress)
	}
}

// --- Integration: UI callback ---

func TestSupervisor_StatusChange_NotifiesUI(t *testing.T) {
	ui := &recordingUI{}
	s := New(Options{DataDir: t.TempDir(), UI: ui})

	s.setStatus(StatusRunning)
	if got := ui.lastStatus(); got != StatusRunning {
		t.Errorf("UI received %v, want running", got)
	}

	s.setStatus(StatusFailed)
	if got := ui.lastStatus(); got != StatusFailed {
		t.Errorf("UI received %v, want failed", got)
	}
}

func TestSupervisor_StatusChange_NoDuplicateCallback(t *testing.T) {
	var callCount atomic.Int32
	ui := &countingUI{count: &callCount}
	s := New(Options{DataDir: t.TempDir(), UI: ui})

	s.setStatus(StatusRunning)
	s.setStatus(StatusRunning) // same status, should not callback again

	if callCount.Load() != 1 {
		t.Errorf("callback count = %d, want 1 (no duplicate)", callCount.Load())
	}
}

type countingUI struct {
	count *atomic.Int32
}

func (c *countingUI) Run(s *Supervisor)        { <-s.ctx.Done() }
func (c *countingUI) OnStatusChange(st Status) { c.count.Add(1) }

// --- Integration: Stop lifecycle ---

func TestSupervisor_Stop_SetsStatusStopped(t *testing.T) {
	s := New(Options{DataDir: t.TempDir()})
	s.setStatus(StatusRunning)
	s.Stop()

	if s.GetStatus() != StatusStopped {
		t.Errorf("after Stop, status = %q, want stopped", s.GetStatus())
	}
}

// --- Integration: Run + Stop full lifecycle ---

func TestSupervisor_RunAndStop(t *testing.T) {
	s := New(Options{DataDir: t.TempDir()})

	done := make(chan struct{})
	go func() {
		s.Run()
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	s.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return after Stop()")
	}

	if s.GetStatus() != StatusStopped {
		t.Errorf("status = %q, want stopped", s.GetStatus())
	}
}

// --- Safety: nil UI ---

func TestSupervisor_NilUI_NoPanic(t *testing.T) {
	s := New(Options{DataDir: t.TempDir(), UI: nil})

	s.setStatus(StatusRunning)
	if s.GetStatus() != StatusRunning {
		t.Errorf("status = %q", s.GetStatus())
	}
}
