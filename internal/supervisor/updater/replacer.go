package updater

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"
)

// ProcessController abstracts process lifecycle for the replacer.
type ProcessController interface {
	Stop()
	Start()
	Done() <-chan struct{}
}

// HealthChecker abstracts health checking for the replacer.
type HealthChecker interface {
	CheckOK() bool
}

// Replacer performs atomic binary replacement with rollback support.
type Replacer struct {
	logger         *log.Logger
	healthTimeout  time.Duration
}

func NewReplacer(logger *log.Logger) *Replacer {
	return &Replacer{logger: logger, healthTimeout: 60 * time.Second}
}

// Replace stops the node, swaps the binary, restarts, and verifies health.
// On failure, it rolls back to the old binary.
func (r *Replacer) Replace(currentPath, newPath string, proc ProcessController, health HealthChecker) error {
	backupPath := currentPath + ".bak"

	r.logger.Println("Stopping node for update...")
	proc.Stop()

	// Wait for process to fully exit
	if done := proc.Done(); done != nil {
		select {
		case <-done:
		case <-time.After(15 * time.Second):
		}
	}

	r.logger.Println("Replacing binary...")
	if err := r.atomicReplace(currentPath, newPath, backupPath); err != nil {
		r.logger.Printf("Replace failed, attempting rollback: %v", err)
		_ = os.Rename(backupPath, currentPath)
		proc.Start()
		return fmt.Errorf("atomic replace: %w", err)
	}

	// Start new version
	r.logger.Println("Starting new version...")
	proc.Start()

	if !r.waitForHealth(health, r.healthTimeout) {
		r.logger.Println("New version unhealthy, rolling back...")
		proc.Stop()
		if done := proc.Done(); done != nil {
			select {
			case <-done:
			case <-time.After(15 * time.Second):
			}
		}
		_ = os.Rename(backupPath, currentPath)
		proc.Start()
		return fmt.Errorf("new version failed health check, rolled back")
	}

	// Cleanup backup
	r.logger.Println("Update verified, cleaning up...")
	_ = os.Remove(backupPath)
	_ = os.Remove(newPath)

	return nil
}

func (r *Replacer) atomicReplace(currentPath, newPath, backupPath string) error {
	// Remove old backup if exists
	_ = os.Remove(backupPath)

	if runtime.GOOS == "windows" {
		// Windows: rename current -> backup, then new -> current
		if err := os.Rename(currentPath, backupPath); err != nil {
			return fmt.Errorf("backup current: %w", err)
		}
		if err := os.Rename(newPath, currentPath); err != nil {
			_ = os.Rename(backupPath, currentPath) // rollback
			return fmt.Errorf("install new: %w", err)
		}
		return nil
	}

	// Unix: rename is atomic on the same filesystem
	if err := os.Rename(currentPath, backupPath); err != nil {
		return fmt.Errorf("backup current: %w", err)
	}
	if err := os.Rename(newPath, currentPath); err != nil {
		_ = os.Rename(backupPath, currentPath)
		return fmt.Errorf("install new: %w", err)
	}
	if err := os.Chmod(currentPath, 0755); err != nil {
		r.logger.Printf("Warning: chmod failed: %v", err)
	}
	return nil
}

func (r *Replacer) waitForHealth(health HealthChecker, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if health.CheckOK() {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}
