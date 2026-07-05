// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"crypto/rand"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestAllocationCredentialVerifiesIdentityAudienceFreshnessAndSignature(t *testing.T) {
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	issuer, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	rawPublicKey, err := publicKey.Raw()
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	credential := AllocationCredential{
		CredentialID: "colcred-test", Version: AllocationCredentialVersionV1,
		IssuerPeerID: issuer.String(), AudiencePeerID: "buyer-peer",
		PolicyID: "io.mobazha.collectibles.source-custody", PolicyVersion: "1", ExtensionRevision: 1,
		ExtensionDigest:      "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		AccountExpiresAtUnix: now.Add(time.Hour).Unix(), IssuedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(5 * time.Minute).Unix(),
		Allocation: AllocationReference{
			AllocationID: "alloc-1", CollateralID: "col-1", TenantID: "seller-tenant",
			ProviderID: "io.mobazha.collectibles", ResourceID: "srcdep-1", PrincipalID: issuer.String(),
			OrderID: "order-1", ExtensionID: "ext-1", AssetID: "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955",
			Amount: "100", CollateralRevision: 3, AllocationRevision: 1, State: AllocationActive,
		},
		IssuerPublicKey: rawPublicKey,
	}
	signable, err := credential.SignableBytes()
	require.NoError(t, err)
	credential.Signature, err = privateKey.Sign(signable)
	require.NoError(t, err)
	require.NoError(t, credential.Verify("buyer-peer", now))
	require.ErrorContains(t, credential.Verify("other-buyer", now), "audience mismatch")
	require.ErrorContains(t, credential.Verify("buyer-peer", now.Add(6*time.Minute)), "not fresh")

	tampered := credential
	tampered.PolicyVersion = "2"
	require.ErrorContains(t, tampered.Verify("buyer-peer", now), "signature is invalid")
}
