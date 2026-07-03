package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	caddymgr "github.com/mobazha/mobazha/internal/caddy"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/response"
)

const (
	goTmplPath     = "/etc/caddy/Caddyfile.go.tmpl"
	caddyOut       = "/etc/caddy/Caddyfile"
	domainConfFile = "domain.conf"
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

// readPersistedDomain returns the domain from the data-dir domain.conf
// file. Returns empty string if unreadable or not set.
func readPersistedDomain(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dataDir, domainConfFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (g *Gateway) handleGETSystemDomain(w http.ResponseWriter, r *http.Request) {
	domain := os.Getenv("STORE_DOMAIN")
	if domain == "" {
		domain = readPersistedDomain(g.config.DataDir)
	}

	connectivity := os.Getenv("CONNECTIVITY")
	if connectivity == "" {
		connectivity = g.config.StandaloneConnectivity
	}
	if connectivity == "" {
		connectivity = "public"
	}

	tlsMode := "self-signed"
	if domain != "" {
		envPath := hostConfigDir + "/.env"
		if _, err := os.Stat(envPath); err == nil {
			tlsMode = "acme"
		} else {
			tlsMode = "external"
		}
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

	// Docker mode: persist to host .env + reload Caddy
	envPath := hostConfigDir + "/.env"
	if _, err := os.Stat(envPath); err == nil {
		if err := updateEnvFile(envPath, map[string]string{
			"STORE_DOMAIN": req.Domain,
		}); err != nil {
			log.Errorf("failed to update env file %s: %v", envPath, err)
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
				"failed to persist domain configuration")
			return
		}
		os.Setenv("STORE_DOMAIN", req.Domain)

		if _, err := os.Stat(goTmplPath); err == nil {
			mgr := caddymgr.NewCaddyManager(goTmplPath, caddyOut, envPath)

			nodePort := repo.DefaultGatewayPortNum
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
				log.Errorf("Caddy reload failed after domain update: %v", err)
				response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
					"domain saved but server reload failed")
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
		return
	}

	// Native binary fallback: persist to data-dir/domain.conf
	if g.config.DataDir != "" {
		confPath := filepath.Join(g.config.DataDir, domainConfFile)
		if err := os.WriteFile(confPath, []byte(req.Domain+"\n"), 0644); err != nil {
			log.Errorf("failed to write domain config %s: %v", confPath, err)
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
				"failed to save domain configuration")
			return
		}
		os.Setenv("STORE_DOMAIN", req.Domain)

		response.Success(w, map[string]interface{}{
			"domain":  req.Domain,
			"tlsMode": "external",
			"message": "Domain registered. Configure your reverse proxy to point to this store.",
		})
		return
	}

	response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
		"domain management unavailable — no config directory")
}
