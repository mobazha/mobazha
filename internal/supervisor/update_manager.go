package supervisor

import (
	"log"
	"sync"

	"github.com/mobazha/mobazha/internal/supervisor/updater"
)

// UpdateStatus tracks the update lifecycle.
type UpdateStatus string

const (
	UpdateUpToDate    UpdateStatus = "up-to-date"
	UpdateAvailable   UpdateStatus = "available"
	UpdateDownloading UpdateStatus = "downloading"
	UpdateReady       UpdateStatus = "ready"
	UpdateApplying    UpdateStatus = "applying"
	UpdateFailed      UpdateStatus = "failed"
)

// UpdateManager orchestrates version checking, downloading, and applying updates.
type UpdateManager struct {
	mu       sync.RWMutex
	status   UpdateStatus
	latest   *updater.ReleaseInfo
	progress int
	lastErr  string

	checker    *updater.Checker
	downloader *updater.Downloader
	replacer   *updater.Replacer
	dataDir    string
	logger     *log.Logger
}

func NewUpdateManager(dataDir string, logger *log.Logger) *UpdateManager {
	return &UpdateManager{
		status:     UpdateUpToDate,
		checker:    updater.NewChecker(logger),
		downloader: updater.NewDownloader(dataDir, logger),
		replacer:   updater.NewReplacer(logger),
		dataDir:    dataDir,
		logger:     logger,
	}
}

func (um *UpdateManager) Status() UpdateStatus {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.status
}

func (um *UpdateManager) Latest() *updater.ReleaseInfo {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.latest
}

func (um *UpdateManager) Progress() int {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.progress
}

func (um *UpdateManager) LastError() string {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.lastErr
}

// CheckNow queries GitHub for a newer release.
func (um *UpdateManager) CheckNow() {
	um.logger.Println("Checking for updates...")
	info, err := um.checker.Check(Version)
	if err != nil {
		um.logger.Printf("Update check failed: %v", err)
		return
	}
	if info == nil {
		um.logger.Println("Already up to date")
		um.mu.Lock()
		um.status = UpdateUpToDate
		um.latest = nil
		um.mu.Unlock()
		return
	}
	um.logger.Printf("New version available: %s", info.Version)
	um.mu.Lock()
	um.status = UpdateAvailable
	um.latest = info
	um.mu.Unlock()
}

// ApplyUpdate downloads, verifies, and replaces the node binary.
func (um *UpdateManager) ApplyUpdate(proc *ProcessManager, health *HealthMonitor) error {
	um.mu.RLock()
	info := um.latest
	um.mu.RUnlock()

	if info == nil {
		return nil
	}

	// Download
	um.setStatus(UpdateDownloading)
	tmpPath, err := um.downloader.Download(info, func(pct int) {
		um.mu.Lock()
		um.progress = pct
		um.mu.Unlock()
	})
	if err != nil {
		um.setError("download failed: " + err.Error())
		return err
	}

	// Apply
	um.setStatus(UpdateApplying)
	binaryPath := proc.BinaryPath()
	if err := um.replacer.Replace(binaryPath, tmpPath, proc, health); err != nil {
		um.setError("replace failed: " + err.Error())
		return err
	}

	um.mu.Lock()
	um.status = UpdateUpToDate
	um.latest = nil
	um.progress = 0
	um.lastErr = ""
	um.mu.Unlock()

	um.logger.Printf("Update to %s applied successfully", info.Version)
	return nil
}

// Snapshot returns a consistent point-in-time snapshot of all update state.
func (um *UpdateManager) Snapshot() UpdateSnapshot {
	um.mu.RLock()
	defer um.mu.RUnlock()
	snap := UpdateSnapshot{
		Status:   um.status,
		Progress: um.progress,
		LastErr:  um.lastErr,
	}
	if um.latest != nil {
		snap.LatestVersion = um.latest.Version
		snap.LatestReleaseURL = um.latest.ReleaseURL
		snap.ReleaseNotes = um.latest.Notes
	}
	return snap
}

// UpdateSnapshot holds a consistent view of the update manager's state.
type UpdateSnapshot struct {
	Status           UpdateStatus
	LatestVersion    string
	LatestReleaseURL string
	ReleaseNotes     string
	Progress         int
	LastErr          string
}

func (um *UpdateManager) setStatus(s UpdateStatus) {
	um.mu.Lock()
	um.status = s
	um.mu.Unlock()
}

func (um *UpdateManager) setError(msg string) {
	um.mu.Lock()
	um.status = UpdateFailed
	um.lastErr = msg
	um.mu.Unlock()
}
