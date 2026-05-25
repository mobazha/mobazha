//go:build !private_distribution

package payment

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestShouldInvokeAsyncPaymentVerifiedHandler_BuyerAndVendor(t *testing.T) {
	require.True(t, shouldInvokeAsyncPaymentVerifiedHandler(&models.Order{
		MyRole: string(models.RoleVendor),
	}))
	require.True(t, shouldInvokeAsyncPaymentVerifiedHandler(&models.Order{
		MyRole: string(models.RoleBuyer),
	}))
	require.False(t, shouldInvokeAsyncPaymentVerifiedHandler(nil))
}
