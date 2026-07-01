package api

import (
	"os"

	"github.com/mobazha/mobazha3.0/pkg/deploy"
)

// detectDeploymentMode returns the configured distribution before inspecting
// the standalone process environment.
func detectDeploymentMode() string {
	switch deploy.GetProcessMode() {
	case deploy.SaaS:
		return "saas"
	case deploy.Sovereign:
		return "sovereign"
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if os.Getenv("DOCKER_CONTAINER") == "true" {
		return "docker"
	}
	return "native"
}
