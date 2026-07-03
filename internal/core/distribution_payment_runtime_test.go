package core

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha/pkg/distribution"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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

	address, signature, err := signer.SignManagedSettlementTransaction(context.Background(), distribution.ManagedEVMSignRequest{
		Chain:         iwallet.ChainEthereum,
		ChainID:       1,
		EscrowAddress: common.HexToAddress("0x2222222222222222222222222222222222222222"),
		Owners:        []common.Address{owner},
		Threshold:     1,
		Digest:        [32]byte{4, 5, 6},
		Purpose:       distribution.ManagedEVMSignSettlementTransaction,
		CorrelationID: "order-neutral-1",
	})
	require.NoError(t, err)
	assert.Equal(t, owner, address)
	require.Len(t, signature, 65)
}

func TestDistributionEVMDigestSigner_RejectsUnauditableRequest(t *testing.T) {
	signer := distributionManagedEVMSigner{keys: &mockKeyProvider{}}
	_, _, err := signer.SignManagedSettlementTransaction(context.Background(), distribution.ManagedEVMSignRequest{
		Chain:  iwallet.ChainEthereum,
		Digest: [32]byte{1},
	})
	require.ErrorContains(t, err, "purpose")
}

func TestDistributionEVMDigestSigner_HonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := (distributionManagedEVMSigner{keys: &mockKeyProvider{}}).SignManagedSettlementTransaction(
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
