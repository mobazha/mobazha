package supervisor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HealthResult holds the outcome of a single health check.
type HealthResult struct {
	OK                  bool
	UnreadNotifications int
}

// HealthMonitor polls the node's /healthz endpoint.
type HealthMonitor struct {
	url    string
	client *http.Client
}

func NewHealthMonitor(gatewayPort string) *HealthMonitor {
	return &HealthMonitor{
		url:    fmt.Sprintf("http://127.0.0.1:%s/healthz", gatewayPort),
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

func (hm *HealthMonitor) Check() HealthResult {
	resp, err := hm.client.Get(hm.url)
	if err != nil {
		return HealthResult{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return HealthResult{}
	}

	var body struct {
		UnreadNotifications int `json:"unreadNotifications"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return HealthResult{OK: true}
	}
	return HealthResult{OK: true, UnreadNotifications: body.UnreadNotifications}
}

// CheckOK returns true if the node is healthy. Satisfies updater.HealthChecker.
func (hm *HealthMonitor) CheckOK() bool {
	return hm.Check().OK
}

// WebUIURL returns the base URL for the Web UI.
func (hm *HealthMonitor) WebUIURL() string {
	return hm.url[:len(hm.url)-len("/healthz")]
}
