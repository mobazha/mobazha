// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettlementSignRequest_ValidateRequiresOpaqueScopeAndDomain(t *testing.T) {
	valid := SettlementSignRequest{
		KeyRef: SettlementKeyRef{
			TenantID: "tenant-1", RailID: "crypto:eip155:1:native",
			Purpose: "guest-safe-owner", ReferenceID: "order-1",
		},
		Domain: "mobazha:settlement:eip155:1:v1", OrderID: "order-1", AttemptID: "attempt-1",
		Action: "release", TermsHash: strings.Repeat("a", 64), Payload: []byte{1},
	}
	require.NoError(t, valid.Validate())

	invalid := valid
	invalid.Domain = ""
	require.Error(t, invalid.Validate())
	invalid = valid
	invalid.KeyRef.ReferenceID = ""
	require.Error(t, invalid.Validate())
	invalid = valid
	invalid.TermsHash = "not-a-hash"
	require.Error(t, invalid.Validate())
}

func TestSettlementSigner_ExposesOnlyOpaqueOperations(t *testing.T) {
	signerType := reflect.TypeOf((*SettlementSigner)(nil)).Elem()
	require.Equal(t, 2, signerType.NumMethod())
	_, hasPublicKey := signerType.MethodByName("PublicKey")
	_, hasSign := signerType.MethodByName("Sign")
	_, hasPrivateKey := signerType.MethodByName("PrivateKey")
	require.True(t, hasPublicKey)
	require.True(t, hasSign)
	require.False(t, hasPrivateKey)
}
