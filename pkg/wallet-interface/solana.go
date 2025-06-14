package wallet_interface

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
)

type EscrowInfo struct {
	Payer              []byte
	Buyer              []byte
	Seller             []byte
	Moderator          []byte
	UniqueId           [20]byte
	RequiredSignatures uint8
	UnlockHours        uint64
	CoinType           CoinType
	Amount             uint64
}

func (e *EscrowInfo) GetSolanaUsersInfo() (payer solana.PublicKey, buyer solana.PublicKey, seller solana.PublicKey, moderator solana.PublicKey, err error) {
	if len(e.Payer) != solana.PublicKeyLength {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid payer")
	}
	payer = solana.PublicKeyFromBytes(e.Payer)
	if len(e.Buyer) != solana.PublicKeyLength {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid buyer")
	}
	buyer = solana.PublicKeyFromBytes(e.Buyer)
	if len(e.Seller) != solana.PublicKeyLength {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid seller")
	}
	seller = solana.PublicKeyFromBytes(e.Seller)
	if len(e.Moderator) > 0 {
		if len(e.Moderator) != solana.PublicKeyLength {
			return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid moderator")
		}
		moderator = solana.PublicKeyFromBytes(e.Moderator)
	}
	return payer, buyer, seller, moderator, nil
}

func (e *EscrowInfo) GetEthereumUsersInfo() (payer common.Address, buyer common.Address, seller common.Address, moderator common.Address, err error) {
	if len(e.Payer) != common.AddressLength {
		return common.Address{}, common.Address{}, common.Address{}, common.Address{}, errors.New("invalid payer")
	}
	payer = common.BytesToAddress(e.Payer)
	if len(e.Buyer) != common.AddressLength {
		return common.Address{}, common.Address{}, common.Address{}, common.Address{}, errors.New("invalid buyer")
	}
	buyer = common.BytesToAddress(e.Buyer)
	if len(e.Seller) != common.AddressLength {
		return common.Address{}, common.Address{}, common.Address{}, common.Address{}, errors.New("invalid seller")
	}
	seller = common.BytesToAddress(e.Seller)
	if len(e.Moderator) > 0 {
		if len(e.Moderator) != common.AddressLength {
			return common.Address{}, common.Address{}, common.Address{}, common.Address{}, errors.New("invalid moderator")
		}
		moderator = common.BytesToAddress(e.Moderator)
	}
	return payer, buyer, seller, moderator, nil
}

// ReleaseEscrowParams 释放参数
type ReleaseEscrowParams struct {
	Initiator []byte
	// PublicKeys与Signatures 一一对应
	Message    []byte
	PublicKeys [][]byte
	Signatures [][]byte
	// Recipients与Amounts 一一对应
	Amounts    []uint64
	Recipients [][]byte
}

func (e *ReleaseEscrowParams) GetSolanaUsersInfo() (initiator solana.PublicKey, publicKeys []solana.PublicKey, recipients []solana.PublicKey, err error) {
	if len(e.Initiator) != solana.PublicKeyLength {
		return solana.PublicKey{}, nil, nil, errors.New("invalid initiator")
	}
	initiator = solana.PublicKeyFromBytes(e.Initiator)

	for _, publicKey := range e.PublicKeys {
		if len(publicKey) == 0 {
			publicKeys = append(publicKeys, solana.PublicKey{})
			continue
		}
		if len(publicKey) != solana.PublicKeyLength {
			return solana.PublicKey{}, nil, nil, errors.New("invalid public key")
		}
		publicKeys = append(publicKeys, solana.PublicKeyFromBytes(publicKey))
	}

	for _, recipient := range e.Recipients {
		if len(recipient) == 0 {
			recipients = append(recipients, solana.PublicKey{})
			continue
		}
		if len(recipient) != solana.PublicKeyLength {
			return solana.PublicKey{}, nil, nil, errors.New("invalid recipient")
		}
		recipients = append(recipients, solana.PublicKeyFromBytes(recipient))
	}

	return initiator, publicKeys, recipients, nil
}

func (e *ReleaseEscrowParams) GetEthereumUsersInfo() (initiator common.Address, publicKeys []common.Address, recipients []common.Address, err error) {
	if len(e.Initiator) != common.AddressLength {
		return common.Address{}, nil, nil, errors.New("invalid initiator")
	}
	initiator = common.BytesToAddress(e.Initiator)

	for _, publicKey := range e.PublicKeys {
		if len(publicKey) == 0 {
			publicKeys = append(publicKeys, common.Address{})
			continue
		}
		if len(publicKey) != common.AddressLength {
			return common.Address{}, nil, nil, errors.New("invalid public key")
		}
		publicKeys = append(publicKeys, common.BytesToAddress(publicKey))
	}

	for _, recipient := range e.Recipients {
		if len(recipient) == 0 {
			recipients = append(recipients, common.Address{})
			continue
		}
		if len(recipient) != common.AddressLength {
			return common.Address{}, nil, nil, errors.New("invalid recipient")
		}
		recipients = append(recipients, common.BytesToAddress(recipient))
		if len(e.Amounts) != len(recipients) {
			return common.Address{}, nil, nil, errors.New("invalid amounts")
		}
	}
	return initiator, publicKeys, recipients, nil
}

type SOLEscrow interface {
	CreateEscrowAddress(escrowInfo EscrowInfo) (Address, error)

	BuildInitSolEscrowInstructions(params EscrowInfo) (Address, any, error)
	BuildReleaseSolEscrowInstructions(escrowInfo EscrowInfo, params ReleaseEscrowParams) (any, error)
}
