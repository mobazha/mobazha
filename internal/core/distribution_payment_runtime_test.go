//go:build !private_distribution

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/distribution"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistributionEVMDigestSigner_SignsWithoutExposingKey(t *testing.T) {
	signer := distributionEVMDigestSigner{keys: &mockKeyProvider{}}
	request := distribution.EVMDigestSignRequest{
		Chain:         iwallet.ChainEthereum,
		Digest:        [32]byte{1, 2, 3},
		Purpose:       "managed_escrow_confirm",
		CorrelationID: "order-1",
	}

	address, signature, err := signer.SignEVMDigest(context.Background(), request)
	require.NoError(t, err)
	assert.NotZero(t, address)
	require.Len(t, signature, 65)
	assert.Contains(t, []byte{27, 28}, signature[64])
}

func TestDistributionEVMDigestSigner_RejectsUnauditableRequest(t *testing.T) {
	signer := distributionEVMDigestSigner{keys: &mockKeyProvider{}}
	_, _, err := signer.SignEVMDigest(context.Background(), distribution.EVMDigestSignRequest{
		Chain:  iwallet.ChainEthereum,
		Digest: [32]byte{1},
	})
	require.ErrorContains(t, err, "purpose")
}

func TestDistributionEVMDigestSigner_HonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := (distributionEVMDigestSigner{keys: &mockKeyProvider{}}).SignEVMDigest(
		ctx,
		distribution.EVMDigestSignRequest{
			Chain:         iwallet.ChainEthereum,
			Digest:        [32]byte{1},
			Purpose:       "managed_escrow_confirm",
			CorrelationID: "order-1",
		},
	)
	require.ErrorIs(t, err, context.Canceled)
}
