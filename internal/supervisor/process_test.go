package supervisor

import (
	"log"
	"os"
	"testing"
	"time"
)

func TestProcessManager_BackoffSchedule(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 5 * time.Second},
		{1, 5 * time.Second},
		{2, 10 * time.Second},
		{3, 30 * time.Second},
		{4, 60 * time.Second},
		{5, 5 * time.Minute},
		{6, 5 * time.Minute}, // clamped to last
	}

	for _, tt := range tests {
		pm.mu.Lock()
		pm.attempts = tt.attempt
		pm.mu.Unlock()

		got := pm.NextBackoff()
		if got != tt.expected {
			t.Errorf("NextBackoff(attempts=%d) = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestProcessManager_ShouldRestart(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))

	for i := 0; i < maxRestartAttempts; i++ {
		if !pm.ShouldRestart() {
			t.Errorf("attempt %d: ShouldRestart() = false, want true", i)
		}
		pm.mu.Lock()
		pm.attempts++
		pm.mu.Unlock()
	}

	if pm.ShouldRestart() {
		t.Error("after max attempts, ShouldRestart() should be false")
	}
}

func TestProcessManager_ResetBackoff(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))

	pm.mu.Lock()
	pm.attempts = 3
	pm.mu.Unlock()

	pm.ResetBackoff()

	if pm.Attempts() != 0 {
		t.Errorf("after ResetBackoff, Attempts() = %d, want 0", pm.Attempts())
	}
	if !pm.ShouldRestart() {
		t.Error("after ResetBackoff, ShouldRestart() should be true")
	}
}

func TestProcessManager_IsRunning_InitiallyFalse(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))
	if pm.IsRunning() {
		t.Error("new ProcessManager should not be running")
	}
}

func TestProcessManager_Done_NilWhenNotRunning(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))
	if pm.Done() != nil {
		t.Error("Done() should be nil when no process is running")
	}
}

func TestProcessManager_StoppedFlag_PreventsStart(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))

	pm.mu.Lock()
	pm.stopped = true
	pm.mu.Unlock()

	pm.Start()

	if pm.IsRunning() {
		t.Error("Start() should be a no-op when stopped flag is set")
	}
}

func TestProcessManager_ShouldRestart_FalseWhenStopped(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))

	if !pm.ShouldRestart() {
		t.Error("ShouldRestart() should be true initially")
	}

	pm.mu.Lock()
	pm.stopped = true
	pm.mu.Unlock()

	if pm.ShouldRestart() {
		t.Error("ShouldRestart() should be false when stopped flag is set")
	}
}

func TestProcessManager_ResetStopped_AllowsRestart(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))

	pm.mu.Lock()
	pm.stopped = true
	pm.attempts = 3
	pm.mu.Unlock()

	if pm.ShouldRestart() {
		t.Error("ShouldRestart() should be false when stopped")
	}

	pm.ResetStopped()

	if !pm.ShouldRestart() {
		t.Error("after ResetStopped, ShouldRestart() should be true")
	}
	if pm.Attempts() != 0 {
		t.Errorf("after ResetStopped, Attempts() = %d, want 0", pm.Attempts())
	}
}

func TestProcessManager_Stop_SetsStoppedFlag(t *testing.T) {
	pm := NewProcessManager(t.TempDir(), nil, log.New(os.Stderr, "[test] ", 0))

	pm.Stop()

	pm.mu.Lock()
	stopped := pm.stopped
	pm.mu.Unlock()

	if !stopped {
		t.Error("Stop() should set the stopped flag")
	}
	if pm.ShouldRestart() {
		t.Error("ShouldRestart() should be false after Stop()")
	}
}
