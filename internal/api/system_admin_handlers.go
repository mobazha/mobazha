//go:build !private_distribution

package api

import "os"

// detectDeploymentMode returns "docker", "native", or "saas".
func detectDeploymentMode() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if os.Getenv("DOCKER_CONTAINER") == "true" {
		return "docker"
	}
	return "native"
}
