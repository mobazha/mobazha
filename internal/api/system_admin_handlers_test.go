package api

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/deploy"
)

func TestDetectDeploymentModePrefersConfiguredDistribution(t *testing.T) {
	defer deploy.SetProcessMode(deploy.Standalone)

	deploy.SetProcessMode(deploy.SaaS)
	if got := detectDeploymentMode(); got != "saas" {
		t.Fatalf("SaaS deployment mode = %q, want %q", got, "saas")
	}

	deploy.SetProcessMode(deploy.PrivateDistribution)
	if got := detectDeploymentMode(); got != "private_distribution" {
		t.Fatalf("PrivateDistribution deployment mode = %q, want %q", got, "private_distribution")
	}
}
