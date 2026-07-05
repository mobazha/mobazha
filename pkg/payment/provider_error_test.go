// SPDX-License-Identifier: MPL-2.0

package payment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeProviderError_RedactsCredentialShapedTokens(t *testing.T) {
	err := errors.New("stripe request sk_test_super-secret failed near pk_live_public-secret, retry")

	got := SanitizeProviderError(err)

	assert.Equal(t, "stripe request sk_test_*** failed near pk_live_***, retry", got)
	assert.NotContains(t, got, "super-secret")
	assert.NotContains(t, got, "public-secret")
}

func TestSanitizeProviderError_Nil(t *testing.T) {
	assert.Empty(t, SanitizeProviderError(nil))
}
