package utils

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func GetOrderEscrowInfo(orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent) (iwallet.EscrowInfo, error) {
	escrowInfo := iwallet.EscrowInfo{}

	if paymentSent.Method == pb.PaymentSent_DIRECT {
		return iwallet.EscrowInfo{}, nil
	}
	switch paymentSent.Method {
	case pb.PaymentSent_DIRECT:
		return iwallet.EscrowInfo{}, nil
	case pb.PaymentSent_CANCELABLE:
		escrowInfo.RequiredSignatures = 1
	case pb.PaymentSent_MODERATED:
		escrowInfo.RequiredSignatures = 2
	}

	coinType := iwallet.CoinType(paymentSent.Coin)
	if !iwallet.IsValidCoinType(coinType) {
		return iwallet.EscrowInfo{}, fmt.Errorf("invalid coin type: %v", coinType)
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return iwallet.EscrowInfo{}, fmt.Errorf("failed to get coin info: %w", err)
	}
	escrowInfo.CoinType = coinType

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return iwallet.EscrowInfo{}, fmt.Errorf("failed to decode chaincode: %w", err)
	}
	escrowInfo.UniqueId = [20]byte(chainCode[:20])
	escrowInfo.Amount = iwallet.NewAmount(paymentSent.Amount).Uint64()
	escrowInfo.UnlockHours = uint64(paymentSent.EscrowTimeoutHours)

	buyerID := orderOpen.BuyerID
	vendorID := orderOpen.Listings[0].Listing.VendorID
	if coinInfo.Chain == iwallet.ChainSolana {
		payer, err := solana.PublicKeyFromBase58(paymentSent.PayerAddress)
		if err != nil {
			return iwallet.EscrowInfo{}, fmt.Errorf("failed to decode payer address: %w", err)
		}
		escrowInfo.Payer = payer.Bytes()
		escrowInfo.Buyer = buyerID.Pubkeys.Solana
		escrowInfo.Seller = vendorID.Pubkeys.Solana
		escrowInfo.Moderator = nil
		if paymentSent.ModeratorAddress != "" {
			moderator, err := solana.PublicKeyFromBase58(paymentSent.ModeratorAddress)
			if err != nil {
				return iwallet.EscrowInfo{}, fmt.Errorf("failed to decode moderator address: %w", err)
			}
			escrowInfo.Moderator = moderator.Bytes()
		}
	} else if coinInfo.IsEthTypeChain() {
		payer := common.HexToAddress(paymentSent.PayerAddress)
		escrowInfo.Payer = payer.Bytes()
		escrowInfo.Buyer = buyerID.Pubkeys.Eth
		escrowInfo.Seller = vendorID.Pubkeys.Eth
		escrowInfo.Moderator = nil
		if paymentSent.ModeratorAddress != "" {
			moderator := common.HexToAddress(paymentSent.ModeratorAddress)
			escrowInfo.Moderator = moderator.Bytes()
		}
	} else {
		return iwallet.EscrowInfo{}, fmt.Errorf("invalid coin type: %v", escrowInfo.CoinType)
	}

	return escrowInfo, nil
}
