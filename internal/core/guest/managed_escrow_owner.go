package guest

import (
	"context"
	"fmt"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha/pkg/contracts"
)

// GuestEVMSellerOwnerResolver exposes only the seller's public EVM owner
// address to a managed-escrow projector.
type GuestEVMSellerOwnerResolver interface {
	SellerEVMOwnerAddress(ctx context.Context, railID string) (common.Address, error)
}

// NodeEVMSellerOwnerResolver derives the public owner address inside Core
// without exposing the key provider to a commercial module.
type NodeEVMSellerOwnerResolver struct {
	Signer   contracts.SettlementSigner
	TenantID string
}

// SellerEVMOwnerAddress returns the node's public EVM owner address.
func (r *NodeEVMSellerOwnerResolver) SellerEVMOwnerAddress(ctx context.Context, railID string) (common.Address, error) {
	if err := ctx.Err(); err != nil {
		return common.Address{}, err
	}
	if r == nil || r.Signer == nil {
		return common.Address{}, fmt.Errorf("guest managed escrow: settlement signer unavailable")
	}
	keyRef := contracts.SettlementKeyRef{
		TenantID: r.TenantID, RailID: railID,
		Purpose: "guest-managed-escrow-owner", ReferenceID: "owner-v1",
	}
	publicKey, err := r.Signer.PublicKey(ctx, keyRef)
	if err != nil {
		return common.Address{}, fmt.Errorf("guest managed escrow: settlement public key: %w", err)
	}
	parsed, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return common.Address{}, fmt.Errorf("guest managed escrow: parse settlement public key: %w", err)
	}
	return crypto.PubkeyToAddress(*parsed.ToECDSA()), nil
}
