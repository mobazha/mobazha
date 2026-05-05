package contracts

import (
	"context"
	"errors"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ErrUTXOAlreadySpent indicates a UTXO has been spent on-chain.
var ErrUTXOAlreadySpent = errors.New("UTXO already spent on chain")

// UTXOEscrowKeysParams holds the escrow keys derived from chaincode.
type UTXOEscrowKeysParams struct {
	Chaincode                []byte
	BuyerKey                 *btcec.PublicKey
	VendorKey                *btcec.PublicKey
	ModeratorKey             *btcec.PublicKey
	ModeratorEscrowPubkeyHex string
}

// UTXOKeyDeriver is the narrow interface SettlementService needs from the
// payment domain for UTXO escrow key derivation (used by ReleasePartialPayment).
type UTXOKeyDeriver interface {
	GetUTXOEscrowKeys(ctx context.Context, order *models.Order, moderator string) (*UTXOEscrowKeysParams, error)
}
