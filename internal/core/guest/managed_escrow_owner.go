package guest

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

// GuestEVMSellerOwnerResolver exposes only the seller's public EVM owner
// address to a managed-escrow projector.
type GuestEVMSellerOwnerResolver interface {
	SellerEVMOwnerAddress(ctx context.Context) (common.Address, error)
}

// NodeEVMSellerOwnerResolver derives the public owner address inside Core
// without exposing the key provider to a commercial module.
type NodeEVMSellerOwnerResolver struct {
	Keys contracts.KeyProvider
}

// SellerEVMOwnerAddress returns the node's public EVM owner address.
func (r *NodeEVMSellerOwnerResolver) SellerEVMOwnerAddress(ctx context.Context) (common.Address, error) {
	if err := ctx.Err(); err != nil {
		return common.Address{}, err
	}
	if r == nil || r.Keys == nil {
		return common.Address{}, fmt.Errorf("guest managed escrow: key provider unavailable")
	}
	key, err := r.Keys.EVMMasterKey()
	if err != nil {
		return common.Address{}, fmt.Errorf("guest managed escrow: EVM master key: %w", err)
	}
	ecdsaKey, err := crypto.ToECDSA(key.Serialize())
	if err != nil {
		return common.Address{}, fmt.Errorf("guest managed escrow: convert EVM master key: %w", err)
	}
	return crypto.PubkeyToAddress(ecdsaKey.PublicKey), nil
}
