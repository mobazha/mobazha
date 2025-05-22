package utils

import (
	"encoding/hex"
	"fmt"

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
		escrowInfo.Buyer = buyerID.Pubkeys.Solana
		escrowInfo.Seller = vendorID.Pubkeys.Solana
	} else if coinInfo.IsEthTypeChain() {
		escrowInfo.Buyer = buyerID.Pubkeys.Eth
		escrowInfo.Seller = vendorID.Pubkeys.Eth
	} else {
		return iwallet.EscrowInfo{}, fmt.Errorf("invalid coin type: %v", escrowInfo.CoinType)
	}
	escrowInfo.Moderator = paymentSent.ModeratorPubKey

	return escrowInfo, nil
}
