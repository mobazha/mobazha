//go:build !private_distribution

package core

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/edition"
	"github.com/stretchr/testify/assert"
)

func TestConfiguredEditionNameDefaultsToFullForCompatibility(t *testing.T) {
	t.Setenv("MOBAZHA_EDITION", "")
	assert.Equal(t, edition.FullName, configuredEditionName(nil))
}

func TestConfiguredEditionNameUsesExplicitCommunitySelection(t *testing.T) {
	t.Setenv("MOBAZHA_EDITION", "community")
	assert.Equal(t, edition.CommunityName, configuredEditionName(nil))
}
