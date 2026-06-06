//go:build !private_distribution

package payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidEVMFundingAddress(t *testing.T) {
	assert.True(t, IsValidEVMFundingAddress("0x111122223333444455556666777788889999aaaa"))
	assert.False(t, IsValidEVMFundingAddress("0xdfac9fe89ed092e0b27e5bf1a71639758d799a6cd301476e78475165e7a2b5ae"))
	assert.False(t, IsValidEVMFundingAddress(""))
}
