package wallet_interface

import (
	"errors"

	"github.com/gagliardetto/solana-go"
)

type CreateEscrowAddressParams struct {
	Buyer              solana.PublicKey
	Seller             solana.PublicKey
	Moderator          *solana.PublicKey
	IsSPLToken         bool
	UniqueId           [20]byte
	RequiredSignatures uint8
	UnlockHours        uint64
	TimeoutKey         solana.PublicKey
}

type EscrowInfo struct {
	Buyer              []byte
	Seller             []byte
	Moderator          []byte
	UniqueId           [20]byte
	RequiredSignatures uint8
	UnlockHours        uint64
	CoinType           CoinType
	Amount             uint64
}

func (e *EscrowInfo) GetSolanaUsersInfo() (solana.PublicKey, solana.PublicKey, solana.PublicKey, error) {
	if len(e.Buyer) != solana.PublicKeyLength {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid buyer")
	}
	buyer := solana.PublicKeyFromBytes(e.Buyer)
	if len(e.Seller) != solana.PublicKeyLength {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid seller")
	}
	seller := solana.PublicKeyFromBytes(e.Seller)
	var moderator solana.PublicKey
	if len(e.Moderator) > 0 {
		if len(e.Moderator) != solana.PublicKeyLength {
			return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid moderator")
		}
		moderator = solana.PublicKeyFromBytes(e.Moderator)
	}
	return buyer, seller, moderator, nil
}

// InitializeSolEscrowParams 初始化SOL托管参数
type InitializeSolEscrowParams struct {
	Payer              solana.PublicKey
	Buyer              solana.PublicKey
	Seller             solana.PublicKey
	Moderator          *solana.PublicKey
	UniqueId           [20]byte
	RequiredSignatures uint8
	UnlockHours        uint64
	CoinType           CoinType
	Amount             uint64
}

// ReleaseSolEscrowParams 释放SOL参数
type ReleaseSolEscrowParams struct {
	Initiator solana.PublicKey
	// PublicKeys与Signatures 一一对应
	Message    []byte
	PublicKeys []solana.PublicKey
	Signatures [][]byte
	// Recipients与Amounts 一一对应
	Amounts    []uint64
	Recipients []solana.PublicKey
}

type SOLEscrow interface {
	CreateEscrowAddress(escrowInfo EscrowInfo) (Address, error)

	BuildInitSolEscrowInstructions(params InitializeSolEscrowParams) (solana.PublicKey, solana.PublicKey, []solana.Instruction, error)
	BuildReleaseSolEscrowInstructions(escrowInfo EscrowInfo, params ReleaseSolEscrowParams) ([]solana.Instruction, error)
}
