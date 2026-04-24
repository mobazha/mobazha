package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/mobazha/mobazha3.0/internal/repo"
)

// Service manages the Mobazha background service.
type Service struct {
	DataDir string `short:"d" long:"datadir" description:"Data directory"`
	Testnet bool   `short:"t" long:"testnet" description:"Use the test network"`
}

// ServiceInstall installs the system service.
type ServiceInstall struct {
	Service
}

// ServiceUninstall removes the system service.
type ServiceUninstall struct {
	Service
}

// ServiceStart starts the service (must be installed first).
type ServiceStart struct {
	Service
}

// ServiceStop stops the running service.
type ServiceStop struct {
	Service
}

// ServiceStatus checks the service status.
type ServiceStatus struct {
	Service
}

func (s *Service) dataDir() string {
	if s.DataDir != "" {
		return s.DataDir
	}
	d := repo.DefaultHomeDir
	if s.Testnet {
		d += "-testnet"
	}
	return d
}

func (s *Service) binaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// launcherBinaryPath returns the path to mobazha-launcher if it exists
// alongside the current binary. The launcher provides crash recovery,
// auto-update, and health monitoring on top of the bare node.
func (s *Service) launcherBinaryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	launcher := filepath.Join(filepath.Dir(resolved), "mobazha-launcher")
	if _, err := os.Stat(launcher); err == nil {
		return launcher
	}
	return ""
}

// Execute installs the Mobazha system service.
func (x *ServiceInstall) Execute(args []string) error {
	switch runtime.GOOS {
	case "linux":
		return x.installSystemd()
	case "darwin":
		return x.installLaunchd()
	default:
		return fmt.Errorf("service management is not supported on %s", runtime.GOOS)
	}
}

// Execute removes the Mobazha system service.
func (x *ServiceUninstall) Execute(args []string) error {
	switch runtime.GOOS {
	case "linux":
		return x.uninstallSystemd()
	case "darwin":
		return x.uninstallLaunchd()
	default:
		return fmt.Errorf("service management is not supported on %s", runtime.GOOS)
	}
}

// Execute starts the Mobazha service.
func (x *ServiceStart) Execute(args []string) error {
	switch runtime.GOOS {
	case "linux":
		return x.startSystemd()
	case "darwin":
		return x.startLaunchd()
	default:
		return fmt.Errorf("service management is not supported on %s", runtime.GOOS)
	}
}

// Execute stops the Mobazha service.
func (x *ServiceStop) Execute(args []string) error {
	switch runtime.GOOS {
	case "linux":
		return x.stopSystemd()
	case "darwin":
		return x.stopLaunchd()
	default:
		return fmt.Errorf("service management is not supported on %s", runtime.GOOS)
	}
}

// Execute checks the service status.
func (x *ServiceStatus) Execute(args []string) error {
	switch runtime.GOOS {
	case "linux":
		return x.statusSystemd()
	case "darwin":
		return x.statusLaunchd()
	default:
		return fmt.Errorf("service management is not supported on %s", runtime.GOOS)
	}
}

// --- Linux systemd ---

const systemdUnitTmpl = `[Unit]
Description=Mobazha Node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.ExecStart}}
Restart=on-failure
RestartSec={{.RestartSec}}
User={{.User}}
WorkingDirectory={{.Home}}

[Install]
WantedBy=multi-user.target
`

func (x *ServiceInstall) installSystemd() error {
	bin, err := x.binaryPath()
	if err != nil {
		return fmt.Errorf("cannot resolve binary path: %w", err)
	}

	u, err := user.Current()
	if err != nil {
		return err
	}

	launcherBin := x.launcherBinaryPath()
	var execStart string
	var restartSec int
	useLauncher := launcherBin != ""

	if useLauncher {
		execStart = launcherBin
		if x.DataDir != "" {
			execStart += " --node-data-dir " + x.DataDir
		}
		if x.Testnet {
			execStart += " --testnet"
		}
		restartSec = 30
	} else {
		execStart = bin + " start -d " + x.dataDir()
		if x.Testnet {
			execStart += " -t"
		}
		restartSec = 5
	}

	data := struct {
		ExecStart  string
		RestartSec int
		User       string
		Home       string
	}{
		ExecStart:  execStart,
		RestartSec: restartSec,
		User:       u.Username,
		Home:       u.HomeDir,
	}

	userMode := canUserSystemd()

	var unitPath string
	if userMode {
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			configDir = filepath.Join(u.HomeDir, ".config")
		}
		unitDir := filepath.Join(configDir, "systemd", "user")
		if err := os.MkdirAll(unitDir, 0755); err != nil {
			return err
		}
		unitPath = filepath.Join(unitDir, "mobazha.service")
	} else {
		unitPath = "/etc/systemd/system/mobazha.service"
	}

	tmpl, err := template.New("unit").Parse(systemdUnitTmpl)
	if err != nil {
		return err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	if userMode {
		if err := os.WriteFile(unitPath, []byte(buf.String()), 0644); err != nil {
			return err
		}
		run("systemctl", "--user", "daemon-reload")
		run("systemctl", "--user", "enable", "mobazha")
		run("systemctl", "--user", "restart", "mobazha")
		run("loginctl", "enable-linger", u.Username)
	} else {
		if err := sudoWriteFile(unitPath, []byte(buf.String())); err != nil {
			return err
		}
		run("sudo", "systemctl", "daemon-reload")
		run("sudo", "systemctl", "enable", "mobazha")
		run("sudo", "systemctl", "restart", "mobazha")
	}

	fmt.Println()
	if useLauncher {
		fmt.Println("✅ Mobazha service installed and started (with auto-update).")
	} else {
		fmt.Println("✅ Mobazha service installed and started.")
		fmt.Println("   ℹ️  Install mobazha-launcher alongside mobazha for auto-update.")
	}
	fmt.Println()
	fmt.Println("   Check status:  mobazha service status")
	if userMode {
		fmt.Println("   View logs:     journalctl --user -u mobazha -f")
	} else {
		fmt.Println("   View logs:     sudo journalctl -u mobazha -f")
	}
	fmt.Println("   Stop:          mobazha service stop")
	fmt.Println("   Uninstall:     mobazha service uninstall")
	return nil
}

func (x *ServiceUninstall) uninstallSystemd() error {
	userMode := canUserSystemd()
	if userMode {
		run("systemctl", "--user", "stop", "mobazha")
		run("systemctl", "--user", "disable", "mobazha")
		u, _ := user.Current()
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			configDir = filepath.Join(u.HomeDir, ".config")
		}
		os.Remove(filepath.Join(configDir, "systemd", "user", "mobazha.service"))
		run("systemctl", "--user", "daemon-reload")
	} else {
		run("sudo", "systemctl", "stop", "mobazha")
		run("sudo", "systemctl", "disable", "mobazha")
		os.Remove("/etc/systemd/system/mobazha.service")
		run("sudo", "systemctl", "daemon-reload")
	}
	fmt.Println("✅ Mobazha service removed.")
	return nil
}

func (x *ServiceStart) startSystemd() error {
	userMode := canUserSystemd()
	if userMode {
		if err := runPassthrough("systemctl", "--user", "start", "mobazha"); err != nil {
			return fmt.Errorf("failed to start service — is it installed? Run: mobazha service install")
		}
	} else {
		if err := runPassthrough("sudo", "systemctl", "start", "mobazha"); err != nil {
			return fmt.Errorf("failed to start service — is it installed? Run: mobazha service install")
		}
	}
	fmt.Println("✅ Mobazha service started.")
	return nil
}

func (x *ServiceStop) stopSystemd() error {
	if canUserSystemd() {
		if err := runPassthrough("systemctl", "--user", "stop", "mobazha"); err != nil {
			return err
		}
	} else {
		if err := runPassthrough("sudo", "systemctl", "stop", "mobazha"); err != nil {
			return err
		}
	}
	fmt.Println("✅ Mobazha service stopped.")
	return nil
}

func (x *ServiceStatus) statusSystemd() error {
	var args []string
	if canUserSystemd() {
		args = []string{"systemctl", "--user", "is-active", "mobazha"}
	} else {
		args = []string{"systemctl", "is-active", "mobazha"}
	}

	out, err := exec.Command(args[0], args[1:]...).Output()
	state := strings.TrimSpace(string(out))

	if err != nil || state != "active" {
		fmt.Println("⏹  Mobazha service is stopped.")
		fmt.Println("   To start:     mobazha service start")
		return nil
	}

	var pidArgs []string
	if canUserSystemd() {
		pidArgs = []string{"systemctl", "--user", "show", "-p", "MainPID", "--value", "mobazha"}
	} else {
		pidArgs = []string{"systemctl", "show", "-p", "MainPID", "--value", "mobazha"}
	}
	pidOut, _ := exec.Command(pidArgs[0], pidArgs[1:]...).Output()
	pid := strings.TrimSpace(string(pidOut))

	if pid != "" && pid != "0" {
		fmt.Printf("✅ Mobazha is running (PID %s)\n", pid)
	} else {
		fmt.Println("✅ Mobazha is running.")
	}
	if canUserSystemd() {
		fmt.Println("   Logs: journalctl --user -u mobazha -f")
	} else {
		fmt.Println("   Logs: sudo journalctl -u mobazha -f")
	}
	return nil
}

func canUserSystemd() bool {
	uid := os.Getuid()
	if uid == 0 {
		return false
	}
	busAddr := fmt.Sprintf("/run/user/%d/bus", uid)
	_, err := os.Stat(busAddr)
	return err == nil
}

// --- macOS launchd ---

const launchdPlistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>org.mobazha.node</string>
	<key>ProgramArguments</key>
	<array>
{{- range .Args}}
		<string>{{.}}</string>
{{- end}}
	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>MOBAZHA_LAUNCHER_BG</key>
		<string>1</string>
	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>StandardOutPath</key>
	<string>{{.LogDir}}/mobazha.log</string>
	<key>StandardErrorPath</key>
	<string>{{.LogDir}}/mobazha.log</string>
</dict>
</plist>
`

func (x *ServiceInstall) installLaunchd() error {
	bin, err := x.binaryPath()
	if err != nil {
		return fmt.Errorf("cannot resolve binary path: %w", err)
	}

	u, err := user.Current()
	if err != nil {
		return err
	}

	logDir := filepath.Join(u.HomeDir, "Library", "Logs", "Mobazha")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	launcherBin := x.launcherBinaryPath()
	useLauncher := launcherBin != ""
	var args []string

	if useLauncher {
		args = append(args, launcherBin)
		if x.DataDir != "" {
			args = append(args, "--node-data-dir", x.DataDir)
		}
		if x.Testnet {
			args = append(args, "--testnet")
		}
	} else {
		args = append(args, bin, "start", "-d", x.dataDir())
		if x.Testnet {
			args = append(args, "-t")
		}
	}

	plistDir := filepath.Join(u.HomeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return err
	}
	plistPath := filepath.Join(plistDir, "org.mobazha.node.plist")

	data := struct {
		Args   []string
		LogDir string
	}{
		Args:   args,
		LogDir: logDir,
	}

	tmpl, err := template.New("plist").Parse(launchdPlistTmpl)
	if err != nil {
		return err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	if err := os.WriteFile(plistPath, []byte(buf.String()), 0644); err != nil {
		return err
	}

	run("launchctl", "unload", plistPath)
	run("launchctl", "load", plistPath)

	fmt.Println()
	if useLauncher {
		fmt.Println("✅ Mobazha service installed and started (with auto-update).")
	} else {
		fmt.Println("✅ Mobazha service installed and started.")
		fmt.Println("   ℹ️  Install mobazha-launcher alongside mobazha for auto-update.")
	}
	fmt.Println("   The node will start automatically on login.")
	fmt.Println()
	fmt.Println("   Check status:  mobazha service status")
	fmt.Println("   View logs:     tail -f ~/Library/Logs/Mobazha/mobazha.log")
	fmt.Println("   Stop:          mobazha service stop")
	fmt.Println("   Uninstall:     mobazha service uninstall")
	return nil
}

func (x *ServiceUninstall) uninstallLaunchd() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(u.HomeDir, "Library", "LaunchAgents", "org.mobazha.node.plist")
	run("launchctl", "unload", plistPath)
	os.Remove(plistPath)
	fmt.Println("✅ Mobazha service removed.")
	return nil
}

func (x *ServiceStart) startLaunchd() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(u.HomeDir, "Library", "LaunchAgents", "org.mobazha.node.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return fmt.Errorf("service is not installed — run: mobazha service install")
	}
	run("launchctl", "load", plistPath)
	fmt.Println("✅ Mobazha service started.")
	return nil
}

func (x *ServiceStop) stopLaunchd() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(u.HomeDir, "Library", "LaunchAgents", "org.mobazha.node.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Println("Mobazha service is not installed.")
		return nil
	}
	run("launchctl", "unload", plistPath)
	fmt.Println("✅ Mobazha service stopped.")
	fmt.Println("   To start again: mobazha service start")
	return nil
}

func (x *ServiceStatus) statusLaunchd() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(u.HomeDir, "Library", "LaunchAgents", "org.mobazha.node.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Println("Mobazha service is not installed.")
		fmt.Println("   To install: mobazha service install")
		return nil
	}

	out, err := exec.Command("launchctl", "list", "org.mobazha.node").Output()
	if err != nil {
		fmt.Println("⏹  Mobazha service is stopped.")
		fmt.Println("   To start:     mobazha service start")
		fmt.Println("   To uninstall: mobazha service uninstall")
		return nil
	}

	pid := parseLaunchdField(string(out), "PID")
	if pid != "" && pid != "0" {
		fmt.Printf("✅ Mobazha is running (PID %s)\n", pid)
	} else {
		fmt.Println("⏹  Mobazha service is loaded but not running.")
	}
	logDir := filepath.Join(u.HomeDir, "Library", "Logs", "Mobazha")
	fmt.Printf("   Logs: %s/mobazha.log\n", logDir)
	return nil
}

func parseLaunchdField(output, key string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "\""+key+"\"") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.TrimSuffix(val, ";")
				val = strings.TrimSpace(val)
				return val
			}
		}
	}
	return ""
}

// --- helpers ---

func run(name string, args ...string) {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	_ = c.Run()
}

func runPassthrough(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func sudoWriteFile(path string, data []byte) error {
	c := exec.Command("sudo", "tee", path)
	c.Stdin = strings.NewReader(string(data))
	c.Stdout = nil
	c.Stderr = os.Stderr
	return c.Run()
}
