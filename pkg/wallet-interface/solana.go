package wallet_interface

import (
	"github.com/gagliardetto/solana-go"
)

// InitializeSolEscrowParams 初始化SOL托管参数
type InitializeSolEscrowParams struct {
	Payer              solana.PublicKey
	Buyer              solana.PublicKey
	Seller             solana.PublicKey
	Moderator          solana.PublicKey
	UniqueId           [20]byte
	RequiredSignatures uint8
	UnlockHours        uint64
	Amount             uint64
}

// ReleaseSolEscrowParams 释放SOL参数
type ReleaseSolEscrowParams struct {
	EscrowAccount solana.PublicKey
	Initiator     solana.PublicKey
	Buyer         solana.PublicKey
	UniqueId      [20]byte
	Amounts       []uint64
	Signatures    [][]byte
	PublicKeys    []solana.PublicKey
	Recipients    []solana.PublicKey
}

// InitializeSPLTokenParams 初始化SPL Token托管参数
type InitializeSPLTokenParams struct {
	Payer              solana.PublicKey
	Buyer              solana.PublicKey
	Seller             solana.PublicKey
	Moderator          solana.PublicKey
	UniqueId           [20]byte
	RequiredSignatures uint8
	UnlockHours        uint64
	Mint               solana.PublicKey
	BuyerTokenAccount  solana.PublicKey
	Amount             uint64
}

// ReleaseSPLTokenParams 释放SPL Token参数
type ReleaseSPLTokenParams struct {
	EscrowAccount          solana.PublicKey
	EscrowTokenAccount     solana.PublicKey
	Initiator              solana.PublicKey
	Buyer                  solana.PublicKey
	Amounts                []uint64
	Signatures             [][]byte
	PublicKeys             []solana.PublicKey
	RecipientTokenAccounts []solana.PublicKey
	UniqueId               [20]byte
}

type SOLEscrow interface {
	InitializeSolEscrow(params InitializeSolEscrowParams) (solana.PublicKey, []solana.Instruction, error)
	ReleaseSolEscrow(params ReleaseSolEscrowParams) ([]solana.Instruction, error)

	InitializeSPLToken(params InitializeSPLTokenParams) (solana.PublicKey, solana.PublicKey, []solana.Instruction, error)
	ReleaseSPLToken(params ReleaseSPLTokenParams) ([]solana.Instruction, error)
}
