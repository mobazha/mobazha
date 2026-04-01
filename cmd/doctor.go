package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/internal/repo"
)

type Doctor struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"use the test network"`
	JSON    bool   `long:"json" description:"output results as JSON"`
}

type checkResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

func (x *Doctor) Execute(args []string) error {
	if x.DataDir == "" {
		x.DataDir = repo.DefaultHomeDir
		if x.Testnet {
			x.DataDir = repo.DefaultHomeDir + "-testnet"
		}
	}

	var results []checkResult

	results = append(results, x.checkDataDir())
	results = append(results, x.checkDisk())
	results = append(results, x.checkDNS())
	results = append(results, x.checkHTTPS())
	results = append(results, x.checkDocker())
	results = append(results, x.checkNodeAPI())
	results = append(results, x.checkSaaSReachability())
	results = append(results, x.checkSystem())

	if x.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	allPass := true
	for _, r := range results {
		icon := "✅"
		if r.Status == "WARN" {
			icon = "⚠️"
		} else if r.Status == "FAIL" {
			icon = "❌"
			allPass = false
		}
		line := fmt.Sprintf("%s  %s", icon, r.Name)
		if r.Detail != "" {
			line += fmt.Sprintf(" — %s", r.Detail)
		}
		fmt.Println(line)
	}

	fmt.Println()
	if allPass {
		fmt.Println("All checks passed.")
	} else {
		fmt.Println("Some checks failed. Review the output above.")
		os.Exit(1)
	}
	return nil
}

func (x *Doctor) checkDataDir() checkResult {
	r := checkResult{Name: "Data directory"}
	if _, err := os.Stat(x.DataDir); os.IsNotExist(err) {
		r.Status = "FAIL"
		r.Detail = fmt.Sprintf("%s does not exist", x.DataDir)
		return r
	}

	dbPath := filepath.Join(x.DataDir, "datastore", "mainnet.db")
	if x.Testnet {
		dbPath = filepath.Join(x.DataDir, "datastore", "testnet.db")
	}
	if _, err := os.Stat(dbPath); err == nil {
		r.Status = "PASS"
		r.Detail = fmt.Sprintf("%s (database found)", x.DataDir)
	} else {
		r.Status = "WARN"
		r.Detail = fmt.Sprintf("%s exists but database not found", x.DataDir)
	}
	return r
}

func (x *Doctor) checkDisk() checkResult {
	r := checkResult{Name: "Disk space"}

	out, err := exec.Command("df", "-h", x.DataDir).Output()
	if err != nil {
		r.Status = "WARN"
		r.Detail = "could not check disk space"
		return r
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 4 {
			r.Status = "PASS"
			r.Detail = fmt.Sprintf("available: %s", fields[3])
			return r
		}
	}
	r.Status = "WARN"
	r.Detail = "could not parse disk info"
	return r
}

func (x *Doctor) checkDNS() checkResult {
	r := checkResult{Name: "DNS resolution"}
	_, err := net.LookupHost("store.mobazha.org")
	if err != nil {
		r.Status = "FAIL"
		r.Detail = fmt.Sprintf("cannot resolve store.mobazha.org: %v", err)
		return r
	}
	r.Status = "PASS"
	r.Detail = "store.mobazha.org resolved"
	return r
}

func (x *Doctor) checkHTTPS() checkResult {
	r := checkResult{Name: "HTTPS connectivity"}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://store.mobazha.org")
	if err != nil {
		r.Status = "FAIL"
		r.Detail = fmt.Sprintf("cannot reach https://store.mobazha.org: %v", err)
		return r
	}
	resp.Body.Close()
	r.Status = "PASS"
	r.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return r
}

func (x *Doctor) checkDocker() checkResult {
	r := checkResult{Name: "Docker"}
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		r.Status = "WARN"
		r.Detail = "Docker not available or not running"
		return r
	}
	r.Status = "PASS"
	r.Detail = fmt.Sprintf("version %s", strings.TrimSpace(string(out)))
	return r
}

func (x *Doctor) checkNodeAPI() checkResult {
	r := checkResult{Name: "Node API"}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:5102/healthz")
	if err != nil {
		r.Status = "WARN"
		r.Detail = "node not running or not reachable at :5102"
		return r
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		r.Status = "PASS"
		r.Detail = "healthy"
	} else {
		r.Status = "WARN"
		r.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return r
}

func (x *Doctor) checkSaaSReachability() checkResult {
	r := checkResult{Name: "SaaS API reachability"}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://store.mobazha.org/platform/v1/health")
	if err != nil {
		r.Status = "WARN"
		r.Detail = "cannot reach SaaS health endpoint"
		return r
	}
	resp.Body.Close()
	r.Status = "PASS"
	r.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return r
}

func (x *Doctor) checkSystem() checkResult {
	r := checkResult{Name: "System"}
	r.Status = "PASS"

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	r.Detail = fmt.Sprintf("OS=%s ARCH=%s CPUs=%d HeapMB=%d",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), m.HeapAlloc/1024/1024)
	return r
}
