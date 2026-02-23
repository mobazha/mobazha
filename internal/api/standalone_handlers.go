package api

import (
	"encoding/json"
	"net/http"
	"runtime"
)

// Version is set at build time via -ldflags.
var Version = "dev"

type healthResponse struct {
	Status string `json:"status"`
}

func (g *Gateway) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}

type versionResponse struct {
	Version string `json:"version"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func (g *Gateway) handleAdminVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versionResponse{
		Version: Version,
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	})
}
