// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

type railAwareSettlementSignerStub struct {
	*settlementKeyOfferSignerStub
	solanaKey []byte
}

func (s *railAwareSettlementSignerStub) SolanaPublicKey(_ context.Context, _ contracts.SettlementKeyRef) ([]byte, error) {
	return append([]byte(nil), s.solanaKey...), nil
}

func (s *railAwareSettlementSignerStub) SignSolanaMessage(
	context.Context,
	contracts.SolanaMessageSettlementSignRequest,
) ([]byte, []byte, error) {
	return append([]byte(nil), s.solanaKey...), nil, nil
}

func TestSettlementPublicKeyForRail_SelectsSolanaEd25519Key(t *testing.T) {
	solanaRail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainSolana)
	require.True(t, ok)
	secpKey := []byte("secp-public-key")
	ed25519Key := make([]byte, 32)
	ed25519Key[0] = 1
	signer := &railAwareSettlementSignerStub{
		settlementKeyOfferSignerStub: &settlementKeyOfferSignerStub{publicKey: secpKey},
		solanaKey:                    ed25519Key,
	}
	keyRef := contracts.SettlementKeyRef{
		TenantID: "tenant-a", RailID: string(solanaRail), Purpose: "standard-order-participant:seller", ReferenceID: "attempt-context",
	}

	publicKey, algorithm, err := SettlementPublicKeyForRail(t.Context(), signer, keyRef)
	require.NoError(t, err)
	require.Equal(t, ed25519Key, publicKey)
	require.Equal(t, models.SettlementKeyAlgorithmEd25519, algorithm)
	require.Empty(t, signer.keyRefs)

	keyRef.RailID = "crypto:eip155:1:native"
	publicKey, algorithm, err = SettlementPublicKeyForRail(t.Context(), signer, keyRef)
	require.NoError(t, err)
	require.Equal(t, secpKey, publicKey)
	require.Empty(t, algorithm)
	require.Len(t, signer.keyRefs, 1)
}
