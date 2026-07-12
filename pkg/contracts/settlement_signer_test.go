// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettlementSignRequest_ValidateRequiresOpaqueScopeAndDomain(t *testing.T) {
	valid := SettlementSignRequest{
		KeyRef: SettlementKeyRef{
			TenantID: "tenant-1", RailID: "crypto:eip155:1:native",
			Purpose: "guest-safe-owner", ReferenceID: "order-1",
		},
		Domain: "mobazha:settlement:eip155:1:v1", Payload: []byte{1},
	}
	require.NoError(t, valid.Validate())

	invalid := valid
	invalid.Domain = ""
	require.Error(t, invalid.Validate())
	invalid = valid
	invalid.KeyRef.ReferenceID = ""
	require.Error(t, invalid.Validate())
}
