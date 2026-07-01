package core

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedEVMKeyProvider struct {
	*mockKeyProvider
	key *btcec.PrivateKey
}

func (p *fixedEVMKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error) { return p.key, nil }

func TestDistributionEVMDigestSigner_SignsWithoutExposingKey(t *testing.T) {
	key, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	provider := &fixedEVMKeyProvider{mockKeyProvider: &mockKeyProvider{}, key: key}
	ecdsaKey, err := crypto.ToECDSA(key.Serialize())
	require.NoError(t, err)
	owner := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	signer := distributionManagedEVMSigner{keys: provider}
	request := distribution.ManagedEVMSignRequest{
		Chain:         iwallet.ChainEthereum,
		ChainID:       1,
		ManagedEscrowAddress:   common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Owners:        []common.Address{owner},
		Threshold:     1,
		Digest:        [32]byte{1, 2, 3},
		Purpose:       distribution.ManagedEVMSignManagedEscrowTransaction,
		CorrelationID: "order-1",
	}

	address, signature, err := signer.SignManagedManagedEscrowTransaction(context.Background(), request)
	require.NoError(t, err)
	assert.NotZero(t, address)
	require.Len(t, signature, 65)
	assert.Contains(t, []byte{27, 28}, signature[64])
}

func TestDistributionEVMDigestSigner_RejectsUnauditableRequest(t *testing.T) {
	signer := distributionManagedEVMSigner{keys: &mockKeyProvider{}}
	_, _, err := signer.SignManagedManagedEscrowTransaction(context.Background(), distribution.ManagedEVMSignRequest{
		Chain:  iwallet.ChainEthereum,
		Digest: [32]byte{1},
	})
	require.ErrorContains(t, err, "purpose")
}

func TestDistributionEVMDigestSigner_HonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := (distributionManagedEVMSigner{keys: &mockKeyProvider{}}).SignManagedManagedEscrowTransaction(
		ctx,
		distribution.ManagedEVMSignRequest{
			Chain:         iwallet.ChainEthereum,
			Digest:        [32]byte{1},
			Purpose:       "managed_escrow_confirm",
			CorrelationID: "order-1",
		},
	)
	require.ErrorIs(t, err, context.Canceled)
}
