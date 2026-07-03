package supervisor

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mobazha/mobazha/internal/repo"
)

// Status represents the current state of the supervised node.
type Status string

const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopped  Status = "stopped"
	StatusUpdating Status = "updating"
	StatusFailed   Status = "failed"
)

// LauncherUI abstracts the presentation layer (systray vs headless).
type LauncherUI interface {
	// Run blocks until the UI exits (systray.Run or signal.Notify).
	Run(s *Supervisor)
	// OnStatusChange is called whenever the node status changes.
	OnStatusChange(status Status)
}

// Supervisor orchestrates node lifecycle, health monitoring, crash recovery,
// and (when enabled) automatic updates.
type Supervisor struct {
	mu     sync.RWMutex
	status Status

	proc    *ProcessManager
	health  *HealthMonitor
	updater *UpdateManager
	config  *ConfigManager
	statusW *StatusWriter
	trigger *TriggerWatcher

	ui      LauncherUI
	dataDir string
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *log.Logger
}

// Options configures the Supervisor.
type Options struct {
	DataDir     string
	GatewayPort string
	NodeArgs    []string // additional arguments passed to the node binary after "start"
	UI          LauncherUI
	Logger      *log.Logger
}

// noopUI is a fallback UI that blocks on context cancellation.
type noopUI struct{}

func (noopUI) Run(s *Supervisor) {
	<-s.ctx.Done()
}

func (noopUI) OnStatusChange(Status) {}

// New creates a Supervisor. Call Run() to start.
func New(opts Options) *Supervisor {
	if opts.DataDir == "" {
		home, _ := os.UserHomeDir()
		opts.DataDir = filepath.Join(home, ".mobazha")
	}
	if opts.GatewayPort == "" {
		opts.GatewayPort = repo.DefaultGatewayPort
	}
	if opts.Logger == nil {
		opts.Logger = log.New(os.Stdout, "[launcher] ", log.LstdFlags)
	}
	if opts.UI == nil {
		opts.UI = noopUI{}
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Supervisor{
		status:  StatusStopped,
		dataDir: opts.DataDir,
		ui:      opts.UI,
		ctx:     ctx,
		cancel:  cancel,
		logger:  opts.Logger,
	}

	s.config = NewConfigManager(opts.DataDir)
	s.statusW = NewStatusWriter(opts.DataDir)
	s.trigger = NewTriggerWatcher(opts.DataDir)
	s.proc = NewProcessManager(opts.DataDir, opts.NodeArgs, opts.Logger)
	s.health = NewHealthMonitor(opts.GatewayPort)
	s.updater = NewUpdateManager(opts.DataDir, opts.Logger)

	return s
}

// Run starts the supervisor loop. It blocks until the UI exits or context is cancelled.
func (s *Supervisor) Run() {
	s.logger.Println("Starting supervisor, data dir:", s.dataDir)

	s.setStatus(StatusStarting)
	go s.proc.Start()
	go s.supervisionLoop()
	go s.triggerLoop()

	s.ui.Run(s)
}

// Stop gracefully shuts down the node and supervisor.
func (s *Supervisor) Stop() {
	s.logger.Println("Stopping supervisor...")
	s.cancel()
	s.proc.Stop() // Stop() sets the stopped flag internally, preventing any future Start()
	s.setStatus(StatusStopped)
}

// Status returns the current node status.
func (s *Supervisor) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// ProcessManager returns the process manager for UI interaction.
func (s *Supervisor) ProcessManager() *ProcessManager {
	return s.proc
}

// HealthMonitor returns the health monitor.
func (s *Supervisor) HealthMonitor() *HealthMonitor {
	return s.health
}

// UpdateManager returns the update manager.
func (s *Supervisor) UpdateManager() *UpdateManager {
	return s.updater
}

// DataDir returns the base data directory.
func (s *Supervisor) DataDir() string {
	return s.dataDir
}

func (s *Supervisor) setStatus(st Status) {
	s.mu.Lock()
	old := s.status
	s.status = st
	s.mu.Unlock()

	if old != st {
		s.logger.Printf("Status: %s -> %s", old, st)
		if s.ui != nil {
			s.ui.OnStatusChange(st)
		}
	}
}

func (s *Supervisor) supervisionLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Supervisor) tick() {
	// Bail out early if supervisor is shutting down.
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	s.config.Reload()
	hr := s.health.Check()
	procRunning := s.proc.IsRunning()

	if hr.OK {
		s.setStatus(StatusRunning)
		s.proc.ResetBackoff()
		_ = s.statusW.WriteStatus(s.buildStatusContent())
		return
	}

	if procRunning {
		s.setStatus(StatusStarting)
		return
	}

	// Node process exited — attempt crash recovery
	if s.proc.ShouldRestart() {
		delay := s.proc.NextBackoff()
		s.logger.Printf("Node exited, restarting in %v (attempt %d)", delay, s.proc.Attempts())
		select {
		case <-time.After(delay):
			go s.proc.Start()
		case <-s.ctx.Done():
		}
	} else {
		s.setStatus(StatusFailed)
		s.logger.Println("Max restart attempts reached, waiting for manual intervention")
	}
}

// buildStatusContent composes a full StatusFileContent from all sources.
func (s *Supervisor) buildStatusContent() StatusFileContent {
	cfg := s.config.Get()
	snap := s.updater.Snapshot()

	return StatusFileContent{
		LauncherVersion:  Version,
		LauncherPID:      os.Getpid(),
		AutoUpdateEnabled: cfg.AutoUpdateEnabled,
		CheckIntervalMin: cfg.CheckIntervalMin,
		UpdateChannel:    cfg.UpdateChannel,
		UpdateStatus:     string(snap.Status),
		LatestVersion:    snap.LatestVersion,
		LatestReleaseURL: snap.LatestReleaseURL,
		ReleaseNotes:     snap.ReleaseNotes,
		DownloadProgress: snap.Progress,
		LastError:        snap.LastErr,
	}
}

func (s *Supervisor) triggerLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			action := s.trigger.Poll()
			if action == "" {
				continue
			}
			s.logger.Printf("Received trigger: %s", action)
			switch action {
			case "check":
				go s.updater.CheckNow()
			case "apply":
				go s.applyUpdate()
			}
		}
	}
}

func (s *Supervisor) applyUpdate() {
	s.setStatus(StatusUpdating)
	if err := s.updater.ApplyUpdate(s.proc, s.health); err != nil {
		s.logger.Printf("Update failed: %v", err)
		s.setStatus(StatusFailed)
		return
	}
	s.setStatus(StatusRunning)
}
