package doctor

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/common"
	"github.com/mobazha/mobazha/internal/repo"
)

type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
)

type CheckResult struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type Summary struct {
	Pass    int           `json:"pass"`
	Warn    int           `json:"warn"`
	Fail    int           `json:"fail"`
	Results []CheckResult `json:"results"`
}

func (s *Summary) Overall() Status {
	if s.Fail > 0 {
		return StatusFail
	}
	if s.Warn > 0 {
		return StatusWarn
	}
	return StatusPass
}

type Config struct {
	DataDir           string
	Testnet           bool
	NodePort          int
	SaaSURL           string
	SkipNetworkChecks bool
}

func DefaultConfig() Config {
	return Config{
		NodePort: repo.DefaultGatewayPortNum,
		SaaSURL:  "https://app.mobazha.org",
	}
}

type Runner struct {
	cfg Config
}

func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

func (r *Runner) RunAll() Summary {
	checks := []func() CheckResult{
		r.CheckDataDir,
		r.CheckDisk,
		r.CheckSystem,
	}
	if !r.cfg.SkipNetworkChecks {
		checks = append(checks,
			r.CheckDNS,
			r.CheckHTTPS,
			r.CheckSaaSReachability,
			r.CheckDocker,
			r.CheckNodeAPI,
			r.CheckCaddy,
			r.CheckTLSCert,
		)
	}

	var s Summary
	for _, fn := range checks {
		cr := fn()
		s.Results = append(s.Results, cr)
		switch cr.Status {
		case StatusPass:
			s.Pass++
		case StatusWarn:
			s.Warn++
		case StatusFail:
			s.Fail++
		}
	}
	return s
}

func (r *Runner) CheckDataDir() CheckResult {
	cr := CheckResult{Name: "Data directory"}
	if r.cfg.DataDir == "" {
		cr.Status = StatusWarn
		cr.Detail = "data directory not configured"
		return cr
	}
	if _, err := os.Stat(r.cfg.DataDir); os.IsNotExist(err) {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("%s does not exist", r.cfg.DataDir)
		return cr
	}

	for _, dbPath := range r.databaseCandidates() {
		if _, err := os.Stat(dbPath); err == nil {
			cr.Status = StatusPass
			cr.Detail = fmt.Sprintf("%s (database found: %s)", r.cfg.DataDir, dbPath)
			return cr
		}
	}

	cr.Status = StatusWarn
	cr.Detail = fmt.Sprintf("%s exists but database not found", r.cfg.DataDir)
	return cr
}

func (r *Runner) databaseCandidates() []string {
	legacyDatabase := "mainnet.db"
	if r.cfg.Testnet {
		legacyDatabase = "testnet.db"
	}

	return []string{
		filepath.Join(r.cfg.DataDir, "nodes", repo.DefaultNodeID, common.DatabaseFileName),
		filepath.Join(r.cfg.DataDir, common.DatabaseFileName),
		filepath.Join(r.cfg.DataDir, "datastore", legacyDatabase),
	}
}

func (r *Runner) CheckDisk() CheckResult {
	cr := CheckResult{Name: "Disk space"}
	path := r.cfg.DataDir
	if path == "" {
		path = "/"
	}
	out, err := exec.Command("df", "-h", path).Output()
	if err != nil {
		cr.Status = StatusWarn
		cr.Detail = "could not check disk space"
		return cr
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 4 {
			cr.Status = StatusPass
			cr.Detail = fmt.Sprintf("available: %s", fields[3])
			return cr
		}
	}
	cr.Status = StatusWarn
	cr.Detail = "could not parse disk info"
	return cr
}

func (r *Runner) CheckDNS() CheckResult {
	cr := CheckResult{Name: "DNS resolution"}
	host := "app.mobazha.org"
	if r.cfg.SaaSURL != "" {
		host = extractHost(r.cfg.SaaSURL)
	}
	if _, err := net.LookupHost(host); err != nil {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("cannot resolve %s: %v", host, err)
		return cr
	}
	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("%s resolved", host)
	return cr
}

func (r *Runner) CheckHTTPS() CheckResult {
	cr := CheckResult{Name: "HTTPS connectivity"}
	url := r.cfg.SaaSURL
	if url == "" {
		url = "https://app.mobazha.org"
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		cr.Status = StatusFail
		cr.Detail = fmt.Sprintf("cannot reach %s: %v", url, err)
		return cr
	}
	resp.Body.Close()
	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return cr
}

func (r *Runner) CheckDocker() CheckResult {
	cr := CheckResult{Name: "Docker"}
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		cr.Status = StatusWarn
		cr.Detail = "Docker not available or not running"
		return cr
	}
	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("version %s", strings.TrimSpace(string(out)))
	return cr
}

func (r *Runner) CheckNodeAPI() CheckResult {
	cr := CheckResult{Name: "Node API"}
	port := r.cfg.NodePort
	if port == 0 {
		port = repo.DefaultGatewayPortNum
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	if err != nil {
		cr.Status = StatusWarn
		cr.Detail = fmt.Sprintf("node not running or not reachable at :%d", port)
		return cr
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		cr.Status = StatusPass
		cr.Detail = "healthy"
	} else {
		cr.Status = StatusWarn
		cr.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return cr
}

func (r *Runner) CheckSaaSReachability() CheckResult {
	cr := CheckResult{Name: "SaaS API reachability"}
	url := r.cfg.SaaSURL
	if url == "" {
		url = "https://app.mobazha.org"
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url + "/platform/v1/health")
	if err != nil {
		cr.Status = StatusWarn
		cr.Detail = "cannot reach SaaS health endpoint"
		return cr
	}
	resp.Body.Close()
	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return cr
}

func (r *Runner) CheckSystem() CheckResult {
	cr := CheckResult{Name: "System"}
	cr.Status = StatusPass
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	cr.Detail = fmt.Sprintf("OS=%s ARCH=%s CPUs=%d HeapMB=%d",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), m.HeapAlloc/1024/1024)
	return cr
}

func (r *Runner) CheckCaddy() CheckResult {
	cr := CheckResult{Name: "Caddy reverse proxy"}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:2019/config/")
	if err != nil {
		cr.Status = StatusWarn
		cr.Detail = "Caddy admin API not reachable (port 2019)"
		return cr
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		cr.Status = StatusPass
		cr.Detail = "Caddy admin API responding"
	} else {
		cr.Status = StatusWarn
		cr.Detail = fmt.Sprintf("Caddy admin API returned HTTP %d", resp.StatusCode)
	}
	return cr
}

func (r *Runner) CheckTLSCert() CheckResult {
	cr := CheckResult{Name: "TLS certificate"}
	domain := os.Getenv("STORE_DOMAIN")
	if domain == "" {
		cr.Status = StatusWarn
		cr.Detail = "STORE_DOMAIN not set — using IP-only mode"
		return cr
	}

	conn, err := net.DialTimeout("tcp", domain+":443", 5*time.Second)
	if err != nil {
		cr.Status = StatusWarn
		cr.Detail = fmt.Sprintf("cannot reach %s:443: %v", domain, err)
		return cr
	}
	conn.Close()

	cr.Status = StatusPass
	cr.Detail = fmt.Sprintf("%s:443 reachable", domain)
	return cr
}

func extractHost(rawURL string) string {
	s := rawURL
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.IndexByte(s, ':'); idx >= 0 {
		s = s[:idx]
	}
	return s
}
