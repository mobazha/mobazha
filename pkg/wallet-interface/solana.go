package wallet_interface

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
)

type EscrowInfo struct {
	ContractAddress      string
	PayerAddress         string
	BuyerAddress         string
	SellerAddress        string
	ModeratorAddress     string
	PlatformFeeCollector string
	RentCollector        string
	UniqueId             [20]byte
	RequiredSignatures   uint8
	UnlockHours          uint64
	UnlockTime           int64
	FundingDeadline      int64
	EscrowServiceFee     uint64
	CoinType             CoinType
	Amount               uint64
	Testnet              bool // 是否使用测试网
}

func (e *EscrowInfo) GetSolanaUsersInfo() (payer solana.PublicKey, buyer solana.PublicKey, seller solana.PublicKey, moderator solana.PublicKey, err error) {
	payer, err = solana.PublicKeyFromBase58(e.PayerAddress)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid payer")
	}
	buyer, err = solana.PublicKeyFromBase58(e.BuyerAddress)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid buyer")
	}
	seller, err = solana.PublicKeyFromBase58(e.SellerAddress)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid seller")
	}
	if e.ModeratorAddress != "" {
		moderator, err = solana.PublicKeyFromBase58(e.ModeratorAddress)
		if err != nil {
			return solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, solana.PublicKey{}, errors.New("invalid moderator")
		}
	}
	return payer, buyer, seller, moderator, nil
}

func (e *EscrowInfo) GetEthereumUsersInfo() (payer common.Address, buyer common.Address, seller common.Address, moderator common.Address, err error) {
	payer = common.HexToAddress(e.PayerAddress)
	buyer = common.HexToAddress(e.BuyerAddress)
	seller = common.HexToAddress(e.SellerAddress)

	if e.ModeratorAddress != "" {
		moderator = common.HexToAddress(e.ModeratorAddress)
	}
	return payer, buyer, seller, moderator, nil
}

// ReleaseEscrowParams 释放参数
type ReleaseEscrowParams struct {
	InitiatorAddress string
	SettlementKind   string
	RentCollector    string
	// PublicKeys与Signatures 一一对应
	Message    []byte
	PublicKeys [][]byte
	Signatures [][]byte
	// Recipients与Amounts 一一对应
	Amounts    []uint64
	Recipients [][]byte
}

func (e *ReleaseEscrowParams) GetSolanaUsersInfo() (initiator solana.PublicKey, publicKeys []solana.PublicKey, recipients []solana.PublicKey, err error) {
	initiator, err = solana.PublicKeyFromBase58(e.InitiatorAddress)
	if err != nil {
		return solana.PublicKey{}, nil, nil, errors.New("invalid initiator")
	}

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
	initiator = common.HexToAddress(e.InitiatorAddress)

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

type PayDataInfo struct {
	UniqueId      []byte
	Destinations  [][]byte
	Amounts       []uint64
	EscrowAddress []byte
}

type EscrowProcessor interface {
	GetContractAddress() (Address, error)

	CreateEscrowAddress(escrowInfo EscrowInfo) (Address, error)

	BuildInitEscrowInstructions(params EscrowInfo) (escrowAddress Address, instructions any, script []byte, err error)
	BuildReleaseEscrowInstructions(escrowInfo EscrowInfo, params ReleaseEscrowParams) (instructions any, err error)
}
