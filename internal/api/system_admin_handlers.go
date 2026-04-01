package api

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/response"
	"golang.org/x/sys/unix"
)

type systemHealthResponse struct {
	Status    string             `json:"status"`
	Uptime    int64              `json:"uptimeSeconds"`
	Timestamp int64              `json:"timestamp"`
	System    systemResourceInfo `json:"system"`
	Node      nodeHealthInfo     `json:"node"`
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

var nodeStartTime = time.Now()

func (g *Gateway) handleGETSystemHealth(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	ns := getNodeService(r)
	peerID := ns.IdentityInfo().Identity().String()

	dataDir := g.setupDataDir()
	if dataDir == "" {
		dataDir = "/"
	}
	diskTotal, diskFree, diskPct := getDiskUsage(dataDir)

	resp := systemHealthResponse{
		Status:    "healthy",
		Uptime:    int64(time.Since(nodeStartTime).Seconds()),
		Timestamp: time.Now().Unix(),
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

	response.Success(w, resp)
}

func getDiskUsage(path string) (totalGB, freeGB, usedPct float64) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)

	totalGB = float64(total) / 1024 / 1024 / 1024
	freeGB = float64(free) / 1024 / 1024 / 1024
	if totalGB > 0 {
		usedPct = (1 - freeGB/totalGB) * 100
	}
	return
}

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
