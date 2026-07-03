package api

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/deploy"
)

func TestDetectDeploymentModePrefersConfiguredDistribution(t *testing.T) {
	defer deploy.SetProcessMode(deploy.Standalone)

	deploy.SetProcessMode(deploy.SaaS)
	if got := detectDeploymentMode(); got != "saas" {
		t.Fatalf("SaaS deployment mode = %q, want %q", got, "saas")
	}

	deploy.SetProcessMode(deploy.Sovereign)
	if got := detectDeploymentMode(); got != "sovereign" {
		t.Fatalf("Sovereign deployment mode = %q, want %q", got, "sovereign")
	}
}
