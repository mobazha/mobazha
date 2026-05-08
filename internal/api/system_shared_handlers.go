package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mobazha/mobazha3.0/internal/supervisor"
	"github.com/mobazha/mobazha3.0/internal/version"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// ── config ──────────────────────────────────────────────────────────

type nodeConfig struct {
	PeerID  string   `json:"peerID"`
	Testnet bool     `json:"testnet"`
	Tor     bool     `json:"tor"`
	Wallets []string `json:"wallets"`
}

func (g *Gateway) handleGETConfig(w http.ResponseWriter, r *http.Request) {
	identity := getIdentityService(r)

	ret := nodeConfig{
		PeerID:  identity.Identity().String(),
		Testnet: identity.UsingTestnet(),
	}

	// Tor and Wallets are only available on full nodes (CoreIface).
	// In SaaS mode these default to false/empty, which is correct.
	if ci, ok := getCoreIface(r); ok {
		ret.Tor = ci.UsingTorMode()
		if mw := ci.Multiwallet(); mw != nil {
			for _, chain := range mw.SupportedChains() {
				ret.Wallets = append(ret.Wallets, chain.String())
			}
		}
	}

	sanitizedJSONResponse(w, &ret)
}

// ── shutdown ────────────────────────────────────────────────────────

func (g *Gateway) handlePOSTShutdown(w http.ResponseWriter, r *http.Request) {
	node, ok := getCoreIface(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Not available in SaaS mode")
		return
	}

	node.Stop(true)
	os.Exit(1)

	sanitizedStringResponse(w, "{}")
}

// ── logs ────────────────────────────────────────────────────────────

type systemLogsResponse struct {
	Lines []string `json:"lines"`
	Total int      `json:"total"`
}

func (g *Gateway) handleGETSystemLogs(w http.ResponseWriter, r *http.Request) {
	dataDir := g.setupDataDir()
	if dataDir == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Logs not available in this mode")
		return
	}

	logFile := dataDir + "/logs/mobazha.log"
	content, err := readLastLines(logFile, 100)
	if err != nil {
		if os.IsNotExist(err) {
			response.Success(w, systemLogsResponse{Lines: []string{}, Total: 0})
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "failed to read logs")
		return
	}

	response.Success(w, systemLogsResponse{
		Lines: content,
		Total: len(content),
	})
}

func readLastLines(path string, maxLines int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := splitNonEmpty(string(data))
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}

func splitNonEmpty(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// ── launcher / auto-update ──────────────────────────────────────────

// launcherDataDir returns the Launcher's IPC directory (~/.mobazha/).
// This is where update-status.json, update-trigger.json, and launcher-config.json live.
// It is separate from the Node's data directory (setupDataDir).
func launcherDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mobazha")
}

// handlePOSTUpdateTrigger writes an update-trigger.json for the Launcher to pick up.
func (g *Gateway) handlePOSTUpdateTrigger(w http.ResponseWriter, r *http.Request) {
	dir := launcherDataDir()
	if dir == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Not available in this deployment mode")
		return
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	if req.Action != "check" && req.Action != "apply" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"action must be 'check' or 'apply'")
		return
	}

	if err := supervisor.WriteTrigger(dir, req.Action); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
			"failed to write trigger file")
		return
	}

	response.NoContent(w)
}

type updateConfigResponse struct {
	AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
	CheckIntervalMin  int    `json:"checkIntervalMinutes"`
	UpdateChannel     string `json:"updateChannel"`
}

// handleGETUpdateConfig reads the launcher-config.json.
func (g *Gateway) handleGETUpdateConfig(w http.ResponseWriter, r *http.Request) {
	dir := launcherDataDir()
	if dir == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Not available in this deployment mode")
		return
	}

	cfg := supervisor.NewConfigManager(dir)
	c := cfg.Get()
	response.Success(w, updateConfigResponse{
		AutoUpdateEnabled: c.AutoUpdateEnabled,
		CheckIntervalMin:  c.CheckIntervalMin,
		UpdateChannel:     c.UpdateChannel,
	})
}

// handlePUTUpdateConfig writes the launcher-config.json.
func (g *Gateway) handlePUTUpdateConfig(w http.ResponseWriter, r *http.Request) {
	dir := launcherDataDir()
	if dir == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Not available in this deployment mode")
		return
	}

	var req updateConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	if req.CheckIntervalMin <= 0 {
		req.CheckIntervalMin = 360
	}
	if req.UpdateChannel == "" {
		req.UpdateChannel = "stable"
	}

	cfg := supervisor.LauncherConfig{
		AutoUpdateEnabled: req.AutoUpdateEnabled,
		CheckIntervalMin:  req.CheckIntervalMin,
		UpdateChannel:     req.UpdateChannel,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "marshal config")
		return
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "create config dir")
		return
	}

	configPath := filepath.Join(dir, "launcher-config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "write config")
		return
	}

	response.Success(w, req)
}

// ── system health ──────────────────────────────────────────────────

type systemHealthResponse struct {
	Status         string              `json:"status"`
	Version        string              `json:"version"`
	Uptime         int64               `json:"uptimeSeconds"`
	Timestamp      int64               `json:"timestamp"`
	DeploymentMode string              `json:"deploymentMode"`
	System         systemResourceInfo  `json:"system"`
	Node           nodeHealthInfo      `json:"node"`
	Update         *updateInfoResponse `json:"update,omitempty"`
}

type systemResourceInfo struct {
	GoVersion    string  `json:"goVersion"`
	OS           string  `json:"os"`
	Arch         string  `json:"arch"`
	NumCPU       int     `json:"numCPU"`
	NumGoroutine int     `json:"numGoroutine"`
	MemAllocMB   float64 `json:"memAllocMB"`
	MemSysMB     float64 `json:"memSysMB"`
	DiskTotalGB  float64 `json:"diskTotalGB"`
	DiskFreeGB   float64 `json:"diskFreeGB"`
	DiskUsedPct  float64 `json:"diskUsedPercent"`
}

type nodeHealthInfo struct {
	PeerID  string `json:"peerID"`
	DataDir string `json:"dataDir"`
}

type updateInfoResponse struct {
	LauncherVersion   string `json:"launcherVersion,omitempty"`
	AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
	UpdateStatus      string `json:"updateStatus"`
	LatestVersion     string `json:"latestVersion,omitempty"`
	LatestReleaseURL  string `json:"latestReleaseURL,omitempty"`
	ReleaseNotes      string `json:"releaseNotes,omitempty"`
	DownloadProgress  int    `json:"downloadProgress"`
	LastCheckTime     string `json:"lastCheckTime,omitempty"`
	LastError         string `json:"lastError,omitempty"`
}

var nodeStartTime = time.Now()

func (g *Gateway) handleGETSystemHealth(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	var peerID string
	if ns, ok := r.Context().Value(nodeContextKey).(contracts.NodeService); ok && ns != nil {
		peerID = ns.IdentityInfo().Identity().String()
	}

	dataDir := g.setupDataDir()
	if dataDir == "" {
		dataDir = "/"
	}
	diskTotal, diskFree, diskPct := getDiskUsage(dataDir)

	resp := systemHealthResponse{
		Status:         "healthy",
		Version:        version.String(),
		Uptime:         int64(time.Since(nodeStartTime).Seconds()),
		Timestamp:      time.Now().Unix(),
		DeploymentMode: detectDeploymentMode(),
		System: systemResourceInfo{
			GoVersion:    runtime.Version(),
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			NumCPU:       runtime.NumCPU(),
			NumGoroutine: runtime.NumGoroutine(),
			MemAllocMB:   float64(memStats.Alloc) / 1024 / 1024,
			MemSysMB:     float64(memStats.Sys) / 1024 / 1024,
			DiskTotalGB:  diskTotal,
			DiskFreeGB:   diskFree,
			DiskUsedPct:  diskPct,
		},
		Node: nodeHealthInfo{
			PeerID:  peerID,
			DataDir: dataDir,
		},
	}

	resp.Update = readUpdateInfo(launcherDataDir())

	response.Success(w, resp)
}

// readUpdateInfo reads the update-status.json written by the Launcher.
func readUpdateInfo(dataDir string) *updateInfoResponse {
	status, err := supervisor.ReadStatusFile(dataDir)
	if err != nil {
		return nil
	}
	return &updateInfoResponse{
		LauncherVersion:   status.LauncherVersion,
		AutoUpdateEnabled: status.AutoUpdateEnabled,
		UpdateStatus:      status.UpdateStatus,
		LatestVersion:     status.LatestVersion,
		LatestReleaseURL:  status.LatestReleaseURL,
		ReleaseNotes:      status.ReleaseNotes,
		DownloadProgress:  status.DownloadProgress,
		LastCheckTime:     status.LastCheckTime,
		LastError:         status.LastError,
	}
}
