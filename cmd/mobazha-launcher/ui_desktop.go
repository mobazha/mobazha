//go:build desktop

package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"fyne.io/systray"

	"github.com/mobazha/mobazha3.0/internal/supervisor"
	"github.com/mobazha/mobazha3.0/internal/version"
)

//go:embed assets/icon.png
var iconRunningMac []byte

//go:embed assets/icon-starting.png
var iconStartingMac []byte

//go:embed assets/icon-stopped.png
var iconStoppedMac []byte

//go:embed assets/icon-win.ico
var iconRunningWin []byte

//go:embed assets/icon-win-starting.ico
var iconStartingWin []byte

//go:embed assets/icon-win-stopped.ico
var iconStoppedWin []byte

var (
	iconRunning  []byte
	iconStarting []byte
	iconStopped  []byte
)

type desktopUI struct {
	logger *log.Logger
	sup    *supervisor.Supervisor

	mOpen   *systray.MenuItem
	mNotif  *systray.MenuItem
	mStatus *systray.MenuItem
	mStart  *systray.MenuItem
	mStop   *systray.MenuItem
}

func createUI(logger *log.Logger) supervisor.LauncherUI {
	// macOS: auto-detach from terminal
	if runtime.GOOS == "darwin" && os.Getenv(envBgKey) == "" {
		selfDetach()
		// selfDetach calls os.Exit, so this is unreachable
	}

	switch runtime.GOOS {
	case "windows":
		iconRunning = iconRunningWin
		iconStarting = iconStartingWin
		iconStopped = iconStoppedWin
	case "darwin":
		iconRunning = iconRunningMac
		iconStarting = iconStartingMac
		iconStopped = iconStoppedMac
	default:
		// Linux (and other Unix-like systems) — fyne.io/systray uses PNG via
		// AppIndicator, so the macOS PNG set renders correctly. If dedicated
		// Linux icons are added later, switch on them here.
		logger.Printf("systray: using PNG icon set for GOOS=%s", runtime.GOOS)
		iconRunning = iconRunningMac
		iconStarting = iconStartingMac
		iconStopped = iconStoppedMac
	}

	return &desktopUI{logger: logger}
}

func (d *desktopUI) Run(s *supervisor.Supervisor) {
	d.sup = s
	systray.Run(d.onReady, d.onExit)
}

func (d *desktopUI) OnStatusChange(status supervisor.Status) {
	switch status {
	case supervisor.StatusRunning:
		setIcon(iconRunning)
		systray.SetTooltip("Mobazha — Running")
		if d.mStatus != nil {
			d.mStatus.SetTitle("Status: Running ✓")
		}
		if d.mOpen != nil {
			d.mOpen.Enable()
		}
		if d.mStop != nil {
			d.mStop.Show()
		}
		if d.mStart != nil {
			d.mStart.Hide()
		}
	case supervisor.StatusStarting:
		setIcon(iconStarting)
		systray.SetTooltip("Mobazha — Starting…")
		if d.mStatus != nil {
			d.mStatus.SetTitle("Status: Starting…")
		}
		if d.mOpen != nil {
			d.mOpen.Disable()
		}
	case supervisor.StatusStopped, supervisor.StatusFailed:
		setIcon(iconStopped)
		systray.SetTooltip("Mobazha — Stopped")
		if d.mStatus != nil {
			d.mStatus.SetTitle("Status: Stopped")
		}
		if d.mOpen != nil {
			d.mOpen.Disable()
		}
		if d.mStop != nil {
			d.mStop.Hide()
		}
		if d.mStart != nil {
			d.mStart.Show()
		}
		systray.SetTitle("")
	case supervisor.StatusUpdating:
		setIcon(iconStarting)
		systray.SetTooltip("Mobazha — Updating…")
		if d.mStatus != nil {
			d.mStatus.SetTitle("Status: Updating…")
		}
	}
}

func (d *desktopUI) onReady() {
	setIcon(iconStarting)
	systray.SetTitle("")
	systray.SetTooltip("Mobazha — Starting…")

	d.mOpen = systray.AddMenuItem("Open Store", "Open the Web UI in your browser")
	systray.AddSeparator()
	d.mNotif = systray.AddMenuItem("No new notifications", "")
	d.mNotif.Disable()
	d.mNotif.Hide()
	d.mStatus = systray.AddMenuItem("Status: Starting…", "")
	d.mStatus.Disable()
	mLogs := systray.AddMenuItem("View Logs", "Open the node log file")
	systray.AddSeparator()
	d.mStart = systray.AddMenuItem("Start Node", "Start the Mobazha node")
	d.mStart.Hide()
	d.mStop = systray.AddMenuItem("Stop Node", "Stop the Mobazha node")
	systray.AddSeparator()
	mVersion := systray.AddMenuItem("Mobazha "+version.String(), "")
	mVersion.Disable()
	mQuit := systray.AddMenuItem("Quit", "Quit Mobazha")

	// Notification badge polling
	go d.notificationPoller()

	// Auto-open browser on first ready
	go d.autoOpenBrowser()

	go func() {
		webUIURL := d.sup.HealthMonitor().WebUIURL()
		for {
			select {
			case <-d.mOpen.ClickedCh:
				openURL(webUIURL)
			case <-d.mNotif.ClickedCh:
				openURL(webUIURL + "/#/notifications")
			case <-mLogs.ClickedCh:
				openLogInEditor(d.sup.ProcessManager().LogFilePath())
			case <-d.mStart.ClickedCh:
				d.sup.ProcessManager().ResetStopped()
				go d.sup.ProcessManager().Start()
			case <-d.mStop.ClickedCh:
				go d.sup.ProcessManager().Stop()
			case <-mQuit.ClickedCh:
				d.sup.Stop()
				systray.Quit()
				return
			}
		}
	}()
}

func (d *desktopUI) onExit() {
	d.sup.Stop()
	d.sup.ProcessManager().Cleanup()
}

func (d *desktopUI) notificationPoller() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		hr := d.sup.HealthMonitor().Check()
		if hr.OK && hr.UnreadNotifications > 0 {
			systray.SetTitle(strconv.Itoa(hr.UnreadNotifications))
			d.mNotif.SetTitle(strconv.Itoa(hr.UnreadNotifications) + " unread notifications")
			d.mNotif.Enable()
			d.mNotif.Show()
		} else {
			systray.SetTitle("")
			d.mNotif.Hide()
		}
	}
}

func (d *desktopUI) autoOpenBrowser() {
	webUIURL := d.sup.HealthMonitor().WebUIURL()
	for i := 0; i < 60; i++ {
		time.Sleep(2 * time.Second)
		if d.sup.HealthMonitor().Check().OK {
			openURL(webUIURL)
			return
		}
	}
}

func setIcon(icon []byte) {
	if runtime.GOOS == "darwin" {
		systray.SetTemplateIcon(icon, icon)
	} else {
		systray.SetIcon(icon)
	}
}

func selfDetach() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot determine executable path: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), envBgKey+"=1")
	setDetachedProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start background launcher: %v\n", err)
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	logPath := home + "/.mobazha/logs/launcher.log"
	fmt.Printf("Mobazha launcher started (PID %d). Logs: %s\n", cmd.Process.Pid, logPath)
	os.Exit(0)
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func openLogInEditor(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("notepad", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}
