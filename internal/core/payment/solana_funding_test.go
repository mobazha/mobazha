package payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidSolanaFundingAddress(t *testing.T) {
	assert.True(t, IsValidSolanaFundingAddress("11111111111111111111111111111112"))
	assert.False(t, IsValidSolanaFundingAddress("not-a-pubkey"))
	assert.False(t, IsValidSolanaFundingAddress(""))
}
