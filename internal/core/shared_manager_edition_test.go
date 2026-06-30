//go:build !private_distribution

package core

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/edition"
	"github.com/stretchr/testify/assert"
)

func TestConfiguredEditionNameUsesFailClosedDefault(t *testing.T) {
	assert.Empty(t, configuredEditionName(nil))
}

func TestConfiguredEditionNameIgnoresUntrustedEnvironmentEscalation(t *testing.T) {
	t.Setenv("MOBAZHA_EDITION", edition.FullName)
	assert.Empty(t, configuredEditionName(nil))
}

func TestConfiguredEditionNamePrefersTrustedComposition(t *testing.T) {
	assert.Equal(t, edition.FullName, configuredEditionName(&repo.Config{Edition: edition.FullName}))
}
