package supervisor

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const maxRestartAttempts = 5

var backoffSchedule = []time.Duration{
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
	60 * time.Second,
	5 * time.Minute,
}

// ProcessManager handles starting, stopping, and crash recovery of the node process.
type ProcessManager struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	stopped bool // set by Stop(); prevents any future Start()
	done    chan struct{}
	logFile *os.File

	dataDir  string
	nodeArgs []string
	logger   *log.Logger
	attempts int
}

func NewProcessManager(dataDir string, nodeArgs []string, logger *log.Logger) *ProcessManager {
	return &ProcessManager{
		dataDir:  dataDir,
		nodeArgs: nodeArgs,
		logger:   logger,
	}
}

func (pm *ProcessManager) Start() {
	pm.mu.Lock()
	if pm.running || pm.stopped {
		pm.mu.Unlock()
		return
	}
	pm.mu.Unlock()

	lf, err := pm.openLogFile()
	if err != nil {
		pm.logger.Printf("Failed to open log file: %v", err)
		return
	}

	bin := pm.findBinary()
	args := append([]string{"start"}, pm.nodeArgs...)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = lf
	cmd.Stderr = lf
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		pm.logger.Printf("Failed to start node: %v", err)
		lf.Close()
		return
	}
	pm.logger.Printf("Node started (PID %d), binary: %s", cmd.Process.Pid, bin)

	done := make(chan struct{})

	pm.mu.Lock()
	if pm.stopped {
		// Stop() was called while we were spawning — kill the orphaned process.
		pm.mu.Unlock()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		lf.Close()
		pm.logger.Println("Node killed immediately: Stop() was called during startup")
		return
	}
	if pm.logFile != nil {
		pm.logFile.Close()
	}
	pm.logFile = lf
	pm.cmd = cmd
	pm.running = true
	pm.done = done
	pm.attempts++
	pm.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		pm.mu.Lock()
		pm.running = false
		pm.cmd = nil
		pm.mu.Unlock()
		close(done)
	}()
}

func (pm *ProcessManager) Stop() {
	pm.mu.Lock()
	pm.stopped = true
	cmd := pm.cmd
	done := pm.done
	pm.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	pm.logger.Println("Sending SIGINT to node...")
	_ = cmd.Process.Signal(os.Interrupt)

	select {
	case <-done:
		pm.logger.Println("Node stopped gracefully")
	case <-time.After(10 * time.Second):
		pm.logger.Println("Node did not stop in 10s, sending SIGKILL")
		_ = cmd.Process.Kill()
		<-done
	}
}

func (pm *ProcessManager) IsRunning() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.running
}

// Done returns a channel that closes when the current process exits.
// Returns nil if no process is running.
func (pm *ProcessManager) Done() <-chan struct{} {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.done
}

func (pm *ProcessManager) ShouldRestart() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return !pm.stopped && pm.attempts < maxRestartAttempts
}

func (pm *ProcessManager) Attempts() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.attempts
}

func (pm *ProcessManager) NextBackoff() time.Duration {
	pm.mu.Lock()
	a := pm.attempts
	pm.mu.Unlock()

	idx := a - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(backoffSchedule) {
		idx = len(backoffSchedule) - 1
	}
	return backoffSchedule[idx]
}

func (pm *ProcessManager) ResetBackoff() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.attempts = 0
}

// ResetStopped clears the stopped flag so Start() can work again.
// Used when the user explicitly clicks "Start Node" in the UI.
func (pm *ProcessManager) ResetStopped() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.stopped = false
	pm.attempts = 0
}

func (pm *ProcessManager) findBinary() string {
	exe, err := os.Executable()
	if err != nil {
		return "mobazha"
	}
	dir := filepath.Dir(exe)
	name := "mobazha"
	if runtime.GOOS == "windows" {
		name = "mobazha.exe"
	}
	candidate := filepath.Join(dir, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}

// BinaryPath returns the resolved path to the node binary.
func (pm *ProcessManager) BinaryPath() string {
	return pm.findBinary()
}

func (pm *ProcessManager) logDir() string {
	return filepath.Join(pm.dataDir, "logs")
}

func (pm *ProcessManager) LogFilePath() string {
	return filepath.Join(pm.logDir(), "mobazha.log")
}

func (pm *ProcessManager) openLogFile() (*os.File, error) {
	dir := pm.logDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	return os.OpenFile(pm.LogFilePath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

func (pm *ProcessManager) Cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.logFile != nil {
		pm.logFile.Close()
		pm.logFile = nil
	}
}
