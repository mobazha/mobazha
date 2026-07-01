package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mobazha/mobazha3.0/internal/doctor"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

func (g *Gateway) handleGETSystemDoctor(w http.ResponseWriter, r *http.Request) {
	cfg := doctor.DefaultConfig()
	cfg.DataDir = g.setupDataDir()

	if detectDeploymentMode() == "sovereign" {
		cfg.SkipNetworkChecks = true
	} else {
		if portStr := os.Getenv("NODE_PORT"); portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil {
				cfg.NodePort = p
			}
		}
		if saasURL := os.Getenv("SAAS_API_URL"); saasURL != "" {
			cfg.SaaSURL = saasURL
		}
	}

	runner := doctor.NewRunner(cfg)
	summary := runner.RunAll()

	response.Success(w, summary)
}

func (g *Gateway) handleGETSystemDiagnostics(w http.ResponseWriter, r *http.Request) {
	dataDir := g.setupDataDir()
	if dataDir == "" {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"data directory not available — diagnostics export unavailable")
		return
	}

	cfg := doctor.DefaultConfig()
	cfg.DataDir = dataDir

	if detectDeploymentMode() == "sovereign" {
		cfg.SkipNetworkChecks = true
	} else {
		if portStr := os.Getenv("NODE_PORT"); portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil {
				cfg.NodePort = p
			}
		}
		if saasURL := os.Getenv("SAAS_API_URL"); saasURL != "" {
			cfg.SaaSURL = saasURL
		}
	}

	runner := doctor.NewRunner(cfg)
	summary := runner.RunAll()

	tmpDir := os.TempDir()
	outPath := filepath.Join(tmpDir, "mobazha-diag-"+time.Now().Format("20060102-150405")+".tar.gz")

	if err := doctor.ExportBundle(outPath, cfg, summary); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
			"failed to create diagnostic bundle")
		return
	}
	defer os.Remove(outPath)

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(outPath)+"\"")
	http.ServeFile(w, r, outPath)
}
