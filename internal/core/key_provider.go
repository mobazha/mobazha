package core

import (
	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

var _ contracts.KeyProvider = (*fileKeyProvider)(nil)

// fileKeyProvider implements KeyProvider by holding key references directly.
// Used in standalone mode where keys are loaded from the node's data directory.
type fileKeyProvider struct {
	ethKey    *btcec.PrivateKey
	solKey    *solana.PrivateKey
	escrowKey *btcec.PrivateKey
	ratingKey *btcec.PrivateKey
	tronKey   *btcec.PrivateKey
}

func newFileKeyProvider(ethKey, escrowKey, ratingKey *btcec.PrivateKey, solKey *solana.PrivateKey, tronKey *btcec.PrivateKey) *fileKeyProvider {
	return &fileKeyProvider{
		ethKey:    ethKey,
		solKey:    solKey,
		escrowKey: escrowKey,
		ratingKey: ratingKey,
		tronKey:   tronKey,
	}
}

func (p *fileKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error)      { return p.ethKey, nil }
func (p *fileKeyProvider) SolanaMasterKey() (*solana.PrivateKey, error)  { return p.solKey, nil }
func (p *fileKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error)   { return p.escrowKey, nil }
func (p *fileKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error)   { return p.ratingKey, nil }
func (p *fileKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error)     { return p.tronKey, nil }
