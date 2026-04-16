package updater

import (
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type mockProc struct {
	stopped atomic.Bool
	started atomic.Bool
	doneCh  chan struct{}
}

func newMockProc() *mockProc {
	return &mockProc{doneCh: make(chan struct{})}
}

func (m *mockProc) Stop() {
	m.stopped.Store(true)
	select {
	case <-m.doneCh:
	default:
		close(m.doneCh)
	}
}

func (m *mockProc) Start() { m.started.Store(true) }
func (m *mockProc) Done() <-chan struct{} { return m.doneCh }

type mockHealth struct {
	ok atomic.Bool
}

func (m *mockHealth) CheckOK() bool { return m.ok.Load() }

func TestReplacer_Success(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "mobazha")
	newBin := filepath.Join(dir, "mobazha-new")

	os.WriteFile(currentBin, []byte("old-binary-v1"), 0755)
	os.WriteFile(newBin, []byte("new-binary-v2"), 0755)

	proc := newMockProc()
	health := &mockHealth{}
	health.ok.Store(true)

	r := NewReplacer(log.New(os.Stderr, "[test] ", 0))
	err := r.Replace(currentBin, newBin, proc, health)
	if err != nil {
		t.Fatalf("Replace() unexpected error: %v", err)
	}

	if !proc.stopped.Load() {
		t.Error("expected process to be stopped")
	}
	if !proc.started.Load() {
		t.Error("expected process to be restarted")
	}

	data, err := os.ReadFile(currentBin)
	if err != nil {
		t.Fatalf("read current binary: %v", err)
	}
	if string(data) != "new-binary-v2" {
		t.Errorf("expected new binary content, got %q", data)
	}

	backupPath := currentBin + ".bak"
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup should be cleaned up after success")
	}
	if _, err := os.Stat(newBin); !os.IsNotExist(err) {
		t.Error("temp new file should be cleaned up after success")
	}
}

func TestReplacer_RollbackOnUnhealthy(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "mobazha")
	newBin := filepath.Join(dir, "mobazha-new")

	os.WriteFile(currentBin, []byte("old-binary-v1"), 0755)
	os.WriteFile(newBin, []byte("bad-binary-v2"), 0755)

	proc := newMockProc()
	health := &mockHealth{}
	health.ok.Store(false) // health check always fails → triggers rollback

	r := &Replacer{logger: log.New(os.Stderr, "[test] ", 0), healthTimeout: 100 * time.Millisecond}
	err := r.Replace(currentBin, newBin, proc, health)
	if err == nil {
		t.Fatal("expected error from failed health check")
	}

	data, err := os.ReadFile(currentBin)
	if err != nil {
		t.Fatalf("read current binary after rollback: %v", err)
	}
	if string(data) != "old-binary-v1" {
		t.Errorf("expected rollback to old binary, got %q", data)
	}
}

func TestReplacer_RollbackOnMissingNewBinary(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "mobazha")
	newBin := filepath.Join(dir, "does-not-exist")

	os.WriteFile(currentBin, []byte("old-binary"), 0755)

	proc := newMockProc()
	health := &mockHealth{}
	health.ok.Store(true)

	r := NewReplacer(log.New(os.Stderr, "[test] ", 0))
	err := r.Replace(currentBin, newBin, proc, health)
	if err == nil {
		t.Fatal("expected error when new binary doesn't exist")
	}

	data, _ := os.ReadFile(currentBin)
	if string(data) != "old-binary" {
		t.Errorf("expected original binary preserved after failed replace, got %q", data)
	}
}

func TestAtomicReplace_PreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "mobazha")
	newBin := filepath.Join(dir, "mobazha-new")
	backupBin := currentBin + ".bak"

	os.WriteFile(currentBin, []byte("old"), 0755)
	os.WriteFile(newBin, []byte("new"), 0644)

	r := NewReplacer(log.New(os.Stderr, "[test] ", 0))
	if err := r.atomicReplace(currentBin, newBin, backupBin); err != nil {
		t.Fatalf("atomicReplace: %v", err)
	}

	info, err := os.Stat(currentBin)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("expected executable permissions on replaced binary")
	}
}
