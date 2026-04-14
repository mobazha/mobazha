package caddy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/mobazha/mobazha3.0/internal/repo"
)

type ReverseProxyManager interface {
	Apply(cfg ProxyConfig) error
	CurrentConfig() ProxyConfig
}

type ProxyConfig struct {
	Domain        string
	Connectivity  string // "public", "overlay", "nat"
	OverlayType   string // "tor", "lokinet", ""
	OverlayDomain string
	NodePort      int
	NextJSPort    int
	SaaSAPIURL    string
	APIKey        string
}

type CaddyManager struct {
	tmplPath    string
	outputPath  string
	adminAPIURL string
	envFilePath string
	current     ProxyConfig
}

func NewCaddyManager(tmplPath, outputPath, envFilePath string) *CaddyManager {
	return &CaddyManager{
		tmplPath:    tmplPath,
		outputPath:  outputPath,
		adminAPIURL: "http://localhost:2019",
		envFilePath: envFilePath,
	}
}

func (m *CaddyManager) Apply(cfg ProxyConfig) error {
	if cfg.NodePort == 0 {
		cfg.NodePort = repo.DefaultGatewayPortNum
	}
	if cfg.NextJSPort == 0 {
		cfg.NextJSPort = 3000
	}
	if cfg.SaaSAPIURL == "" {
		cfg.SaaSAPIURL = "https://app.mobazha.org"
	}

	rendered, err := m.render(cfg)
	if err != nil {
		return fmt.Errorf("render Caddyfile: %w", err)
	}

	if err := os.WriteFile(m.outputPath, rendered, 0644); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
	}

	if err := m.reload(); err != nil {
		return fmt.Errorf("caddy reload: %w", err)
	}

	m.current = cfg
	return nil
}

func (m *CaddyManager) CurrentConfig() ProxyConfig {
	return m.current
}

func (m *CaddyManager) render(cfg ProxyConfig) ([]byte, error) {
	tmplContent, err := os.ReadFile(m.tmplPath)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", m.tmplPath, err)
	}

	tmpl, err := template.New("caddyfile").Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

func (m *CaddyManager) reload() error {
	caddyfile, err := os.ReadFile(m.outputPath)
	if err != nil {
		return fmt.Errorf("read output Caddyfile: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, m.adminAPIURL+"/load",
		bytes.NewReader(caddyfile))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s/load: %w", m.adminAPIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy reload returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
