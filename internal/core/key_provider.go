package core

import (
	"crypto/sha256"
	"fmt"
	"io"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/mobazha/mobazha/pkg/contracts"
	"golang.org/x/crypto/hkdf"
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

func (p *fileKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error)     { return p.ethKey, nil }
func (p *fileKeyProvider) SolanaMasterKey() (*solana.PrivateKey, error) { return p.solKey, nil }
func (p *fileKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error)  { return p.escrowKey, nil }
func (p *fileKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error)  { return p.ratingKey, nil }
func (p *fileKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error)    { return p.tronKey, nil }

// DigitalContentMasterKey derives a 32-byte master key for digital asset
// encryption from the escrow master key using HKDF. The version parameter
// supports key rotation — old versions remain derivable until all content
// is re-encrypted.
func (p *fileKeyProvider) DigitalContentMasterKey(version int) ([]byte, error) {
	if p.escrowKey == nil {
		return nil, fmt.Errorf("escrow master key not available")
	}
	ikm := p.escrowKey.Serialize()
	salt := []byte("mobazha-digital-content-master-v1")
	info := []byte(fmt.Sprintf("digital-content-master:v%d", version))

	kdf := hkdf.New(sha256.New, ikm, salt, info)
	key := make([]byte, 32)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return nil, fmt.Errorf("HKDF expand failed: %w", err)
	}
	return key, nil
}
