//go:build !private_distribution

package core

import (
	"context"
	"crypto/ed25519"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	gosolana "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha3.0/pkg/distribution"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type fixedSolanaKeyProvider struct {
	key *gosolana.PrivateKey
}

func (*fixedSolanaKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error) {
	key, err := btcec.NewPrivateKey()
	return key, err
}

func (p *fixedSolanaKeyProvider) SolanaMasterKey() (*gosolana.PrivateKey, error) {
	return p.key, nil
}

func (*fixedSolanaKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error) {
	key, err := btcec.NewPrivateKey()
	return key, err
}

func (*fixedSolanaKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error) {
	key, err := btcec.NewPrivateKey()
	return key, err
}

func (*fixedSolanaKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error) {
	key, err := btcec.NewPrivateKey()
	return key, err
}

func (*fixedSolanaKeyProvider) DigitalContentMasterKey(int) ([]byte, error) {
	return make([]byte, 32), nil
}

func TestDistributionManagedSolanaSigner_SignsAuthorizedAnchorMessage(t *testing.T) {
	wallet := gosolana.NewWallet()
	program := gosolana.NewWallet().PublicKey()
	escrow := gosolana.NewWallet().PublicKey()
	signer := distributionManagedSolanaSigner{keys: &fixedSolanaKeyProvider{key: &wallet.PrivateKey}}
	message := []byte("deterministic-anchor-settlement-message")

	publicKey, signature, err := signer.SignManagedSolanaMessage(context.Background(), distribution.ManagedSolanaSignRequest{
		Chain: iwallet.ChainSolana, OrderID: "order-1", ActionKind: "confirm",
		ProgramAddress: program.String(), EscrowAddress: escrow.String(),
		AuthorizedSigners: []string{wallet.PublicKey().String()}, Message: message,
		Purpose: distribution.ManagedSolanaSignAnchorSettlement, CorrelationID: "order-1:confirm",
	})
	require.NoError(t, err)
	assert.Equal(t, wallet.PublicKey().String(), publicKey)
	assert.True(t, ed25519.Verify(wallet.PublicKey().Bytes(), message, signature))
}

func TestDistributionManagedSolanaSigner_RejectsUnauthorizedOwner(t *testing.T) {
	wallet := gosolana.NewWallet()
	signer := distributionManagedSolanaSigner{keys: &fixedSolanaKeyProvider{key: &wallet.PrivateKey}}

	_, _, err := signer.SignManagedSolanaMessage(context.Background(), distribution.ManagedSolanaSignRequest{
		Chain: iwallet.ChainSolana, OrderID: "order-1", ActionKind: "cancel",
		ProgramAddress:    gosolana.NewWallet().PublicKey().String(),
		EscrowAddress:     gosolana.NewWallet().PublicKey().String(),
		AuthorizedSigners: []string{gosolana.NewWallet().PublicKey().String()},
		Message:           []byte("cancel-message"),
		Purpose:           distribution.ManagedSolanaSignAnchorSettlement,
		CorrelationID:     "order-1:cancel",
	})
	require.ErrorContains(t, err, "outside the authorized owner set")
}

func TestDistributionManagedSolanaSigner_RejectsTransactionLikeOrUnknownPurpose(t *testing.T) {
	wallet := gosolana.NewWallet()
	signer := distributionManagedSolanaSigner{keys: &fixedSolanaKeyProvider{key: &wallet.PrivateKey}}
	request := distribution.ManagedSolanaSignRequest{
		Chain: iwallet.ChainSolana, OrderID: "order-1", ActionKind: "relay_submit",
		ProgramAddress:    gosolana.NewWallet().PublicKey().String(),
		EscrowAddress:     gosolana.NewWallet().PublicKey().String(),
		AuthorizedSigners: []string{wallet.PublicKey().String()}, Message: []byte("serialized-transaction"),
		Purpose: "generic_transaction", CorrelationID: "order-1:relay",
	}

	_, _, err := signer.SignManagedSolanaMessage(context.Background(), request)
	require.ErrorContains(t, err, "unsupported purpose")
}
