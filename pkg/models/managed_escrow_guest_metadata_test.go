package models

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManagedEscrowGuestMetadataIsBoundedOpaqueJSON(t *testing.T) {
	order := &GuestOrder{}
	metadata := []byte(`{"provider":"test","version":1}`)
	require.NoError(t, order.SetManagedEscrowGuestMetadata(metadata))
	require.True(t, order.HasManagedEscrowGuestFundingTarget())

	metadata[2] = 'X'
	stored := order.ManagedEscrowGuestMetadata()
	require.JSONEq(t, `{"provider":"test","version":1}`, string(stored))
	stored[2] = 'Y'
	require.JSONEq(t, `{"provider":"test","version":1}`, string(order.ManagedEscrowGuestMetadata()))

	require.Error(t, order.SetManagedEscrowGuestMetadata([]byte(`[]`)))
	require.Error(t, order.SetManagedEscrowGuestMetadata([]byte(`{}`)))
	require.Error(t, order.SetManagedEscrowGuestMetadata(bytes.Repeat([]byte{'x'}, maxManagedEscrowGuestMetadataBytes+1)))
	require.NoError(t, order.SetManagedEscrowGuestMetadata(nil))
	require.False(t, order.HasManagedEscrowGuestFundingTarget())
}
