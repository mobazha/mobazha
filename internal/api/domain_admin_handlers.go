package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	caddymgr "github.com/mobazha/mobazha3.0/internal/caddy"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

const (
	goTmplPath = "/etc/caddy/Caddyfile.go.tmpl"
	caddyOut   = "/etc/caddy/Caddyfile"
)

type domainConfigResponse struct {
	Domain        string `json:"domain"`
	Connectivity  string `json:"connectivity"`
	OverlayType   string `json:"overlayType"`
	OverlayDomain string `json:"overlayDomain,omitempty"`
	TLSMode       string `json:"tlsMode"`
}

type domainConfigRequest struct {
	Domain string `json:"domain"`
}

func (g *Gateway) handleGETSystemDomain(w http.ResponseWriter, r *http.Request) {
	domain := os.Getenv("STORE_DOMAIN")
	connectivity := os.Getenv("CONNECTIVITY")
	if connectivity == "" {
		connectivity = "public"
	}

	tlsMode := "acme"
	if domain == "" {
		tlsMode = "self-signed"
	}
	if connectivity == "overlay" {
		tlsMode = "internal"
	}

	resp := domainConfigResponse{
		Domain:        domain,
		Connectivity:  connectivity,
		OverlayType:   os.Getenv("OVERLAY_TYPE"),
		OverlayDomain: os.Getenv("OVERLAY_DOMAIN"),
		TLSMode:       tlsMode,
	}
	response.Success(w, resp)
}

func (g *Gateway) handlePOSTSystemDomain(w http.ResponseWriter, r *http.Request) {
	var req domainConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	req.Domain = strings.TrimSpace(req.Domain)

	envPath := hostConfigDir + "/.env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"host config not mounted — domain management unavailable")
		return
	}

	if err := updateEnvFile(envPath, map[string]string{
		"STORE_DOMAIN": req.Domain,
	}); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
			"failed to update .env: "+err.Error())
		return
	}
	os.Setenv("STORE_DOMAIN", req.Domain)

	if _, err := os.Stat(goTmplPath); err == nil {
		mgr := caddymgr.NewCaddyManager(goTmplPath, caddyOut, envPath)

		nodePort := 5102
		connectivity := os.Getenv("CONNECTIVITY")
		if connectivity == "" {
			connectivity = "public"
		}

		cfg := caddymgr.ProxyConfig{
			Domain:        req.Domain,
			Connectivity:  connectivity,
			OverlayType:   os.Getenv("OVERLAY_TYPE"),
			OverlayDomain: os.Getenv("OVERLAY_DOMAIN"),
			NodePort:      nodePort,
			SaaSAPIURL:    os.Getenv("SAAS_API_URL"),
			APIKey:        os.Getenv("STANDALONE_API_KEY"),
		}

		if err := mgr.Apply(cfg); err != nil {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
				"domain saved but Caddy reload failed: "+err.Error())
			return
		}
	}

	tlsMode := "acme"
	if req.Domain == "" {
		tlsMode = "self-signed"
	}

	response.Success(w, map[string]interface{}{
		"domain":  req.Domain,
		"tlsMode": tlsMode,
		"message": "Domain updated and Caddy reloaded",
	})
}
