//go:build !private_distribution

package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	caddymgr "github.com/mobazha/mobazha3.0/internal/caddy"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

const (
	hostConfigDir = "/hostconfig"
	dockerSocket  = "/var/run/docker.sock"
)

type networkConfigResponse struct {
	Connectivity  string `json:"connectivity"`
	OverlayType   string `json:"overlayType"`
	OverlayDomain string `json:"overlayDomain,omitempty"`
	DockerManaged bool   `json:"dockerManaged"`
	GatewayPort   int    `json:"gatewayPort"`
}

type networkConfigRequest struct {
	OverlayType string `json:"overlayType"` // "tor", "lokinet", or ""
}

func (g *Gateway) handleGETSystemNetwork(w http.ResponseWriter, r *http.Request) {
	connectivity := os.Getenv("CONNECTIVITY")
	if connectivity == "" {
		connectivity = g.config.StandaloneConnectivity
	}
	if connectivity == "" {
		connectivity = "public"
	}

	port := repo.DefaultGatewayPortNum
	if g.listener != nil {
		if addr := g.listener.Addr(); addr != nil {
			if _, p, err := net.SplitHostPort(addr.String()); err == nil {
				if n, err := strconv.Atoi(p); err == nil {
					port = n
				}
			}
		}
	}

	resp := networkConfigResponse{
		Connectivity:  connectivity,
		OverlayType:   os.Getenv("OVERLAY_TYPE"),
		OverlayDomain: os.Getenv("OVERLAY_DOMAIN"),
		DockerManaged: dockerSocketAvailable(),
		GatewayPort:   port,
	}

	response.Success(w, resp)
}

func (g *Gateway) handlePOSTSystemNetwork(w http.ResponseWriter, r *http.Request) {
	var req networkConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	req.OverlayType = strings.TrimSpace(strings.ToLower(req.OverlayType))
	if req.OverlayType != "" && req.OverlayType != "tor" && req.OverlayType != "lokinet" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"overlayType must be 'tor', 'lokinet', or empty to disable")
		return
	}

	envPath := hostConfigDir + "/.env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"host config not mounted — overlay management unavailable")
		return
	}

	if !dockerSocketAvailable() {
		response.Error(w, http.StatusServiceUnavailable, response.CodeServiceUnavail,
			"Docker socket not mounted — cannot manage overlay containers")
		return
	}

	newConnectivity := "public"
	if req.OverlayType != "" {
		newConnectivity = "overlay"
	}

	if err := updateEnvFile(envPath, map[string]string{
		"CONNECTIVITY": newConnectivity,
		"OVERLAY_TYPE": req.OverlayType,
	}); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
			"failed to update configuration: "+err.Error())
		return
	}

	currentOverlay := os.Getenv("OVERLAY_TYPE")

	if currentOverlay != "" && currentOverlay != req.OverlayType {
		_ = dockerContainerAction("mobazha-"+currentOverlay, "stop")
	}

	if req.OverlayType != "" && req.OverlayType != currentOverlay {
		_ = dockerContainerAction("mobazha-"+req.OverlayType, "start")
	}

	os.Setenv("CONNECTIVITY", newConnectivity)
	os.Setenv("OVERLAY_TYPE", req.OverlayType)

	if _, err := os.Stat(goTmplPath); err == nil {
		mgr := caddymgr.NewCaddyManager(goTmplPath, caddyOut, envPath)
		cfg := caddymgr.ProxyConfig{
			Domain:        os.Getenv("STORE_DOMAIN"),
			Connectivity:  newConnectivity,
			OverlayType:   req.OverlayType,
			OverlayDomain: os.Getenv("OVERLAY_DOMAIN"),
			NodePort:      repo.DefaultGatewayPortNum,
			SaaSAPIURL:    os.Getenv("SAAS_API_URL"),
			APIKey:        os.Getenv("STANDALONE_API_KEY"),
		}
		if err := mgr.Apply(cfg); err != nil {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError,
				"config saved but Caddy reload failed: "+err.Error())
			return
		}
	} else {
		_ = dockerContainerAction("mobazha-store", "restart")
	}

	response.Success(w, map[string]interface{}{
		"connectivity": newConnectivity,
		"overlayType":  req.OverlayType,
		"message":      "Network configuration updated.",
	})
}

func dockerSocketAvailable() bool {
	info, err := os.Stat(dockerSocket)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}

func dockerContainerAction(containerName, action string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/containers/%s/%s", containerName, action)

	conn, err := net.Dial("unix", dockerSocket)
	if err != nil {
		return fmt.Errorf("dial docker socket: %w", err)
	}
	defer conn.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if err := req.Write(conn); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker API %s %s returned %d: %s", action, containerName, resp.StatusCode, string(body))
	}
	return nil
}

func updateEnvFile(path string, updates map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := make(map[string]bool)
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			if newVal, ok := updates[key]; ok {
				result = append(result, key+"="+newVal)
				found[key] = true
				continue
			}
		}
		result = append(result, line)
	}

	for key, val := range updates {
		if !found[key] {
			result = append(result, key+"="+val)
		}
	}

	output := strings.Join(result, "\n")
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	return os.WriteFile(path, []byte(output), 0600)
}
