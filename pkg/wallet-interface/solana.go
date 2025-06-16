package wallet_interface

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
)

type EscrowInfo struct {
	ContractAddress    string
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
	if payer, err = PubKeyBytesToEthAddress(e.Payer); err != nil {
		return common.Address{}, common.Address{}, common.Address{}, common.Address{}, errors.New("invalid payer")
	}
	if buyer, err = PubKeyBytesToEthAddress(e.Buyer); err != nil {
		return common.Address{}, common.Address{}, common.Address{}, common.Address{}, errors.New("invalid buyer")
	}
	if seller, err = PubKeyBytesToEthAddress(e.Seller); err != nil {
		return common.Address{}, common.Address{}, common.Address{}, common.Address{}, errors.New("invalid seller")
	}
	if len(e.Moderator) > 0 {
		if moderator, err = PubKeyBytesToEthAddress(e.Moderator); err != nil {
			return common.Address{}, common.Address{}, common.Address{}, common.Address{}, errors.New("invalid moderator")
		}
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
	if initiator, err = PubKeyBytesToEthAddress(e.Initiator); err != nil {
		return common.Address{}, nil, nil, errors.New("invalid initiator")
	}

	for _, publicKey := range e.PublicKeys {
		if len(publicKey) == 0 {
			publicKeys = append(publicKeys, common.Address{})
			continue
		}
		publicKeyAddress, err := PubKeyBytesToEthAddress(publicKey)
		if err != nil {
			return common.Address{}, nil, nil, errors.New("invalid public key")
		}
		publicKeys = append(publicKeys, publicKeyAddress)
	}

	for _, recipient := range e.Recipients {
		if len(recipient) == 0 {
			recipients = append(recipients, common.Address{})
			continue
		}
		recipientAddress, err := PubKeyBytesToEthAddress(recipient)
		if err != nil {
			return common.Address{}, nil, nil, errors.New("invalid recipient")
		}
		recipients = append(recipients, recipientAddress)
		if len(e.Amounts) != len(recipients) {
			return common.Address{}, nil, nil, errors.New("invalid amounts")
		}
	}
	return initiator, publicKeys, recipients, nil
}

type EscrowProcessor interface {
	GetContractAddress() (Address, error)

	CreateEscrowAddress(escrowInfo EscrowInfo) (Address, error)

	BuildInitEscrowInstructions(params EscrowInfo) (Address, any, error)
	BuildReleaseEscrowInstructions(escrowInfo EscrowInfo, params ReleaseEscrowParams) (any, error)
}
