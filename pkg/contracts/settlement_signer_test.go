// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"reflect"
	"strings"
	"testing"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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

func TestUTXOMultisigSettlementSignRequest_ValidateRequiresFrozenActionScope(t *testing.T) {
	valid := UTXOMultisigSettlementSignRequest{
		KeyRef: SettlementKeyRef{
			TenantID: "tenant-1", RailID: "BTC",
			Purpose: "standard-order-participant:buyer", ReferenceID: "authorization-context-1",
		},
		OrderID: "order-1", AttemptID: "attempt-1", Action: "cancel", Sequence: 1,
		TermsHash: strings.Repeat("a", 64), CoinCode: "BTC",
		Transaction: iwallet.Transaction{
			From: []iwallet.SpendInfo{{ID: []byte{1}}},
			To:   []iwallet.SpendInfo{{ID: []byte{2}}},
		},
		RedeemScript: []byte{3},
	}
	require.NoError(t, valid.Validate())

	invalid := valid
	invalid.CoinCode = "LTC"
	require.Error(t, invalid.Validate())
	invalid = valid
	invalid.Transaction.To = nil
	require.Error(t, invalid.Validate())
	invalid = valid
	invalid.RedeemScript = nil
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

	utxoSignerType := reflect.TypeOf((*UTXOSettlementSigner)(nil)).Elem()
	require.Equal(t, 1, utxoSignerType.NumMethod())
	_, hasUTXOSign := utxoSignerType.MethodByName("SignUTXOMultisig")
	_, hasUTXOPrivateKey := utxoSignerType.MethodByName("PrivateKey")
	require.True(t, hasUTXOSign)
	require.False(t, hasUTXOPrivateKey)
}
