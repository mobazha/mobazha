package payment

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// OrderEscrowInfo projects signed order and payment messages into the neutral
// escrow descriptor consumed by chain implementations. It is shared Core
// semantics: commercial adapters must not fork owner, threshold, amount, or
// refund-address derivation.
func OrderEscrowInfo(orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent, testnet bool) (iwallet.EscrowInfo, error) {
	if orderOpen == nil || paymentSent == nil || orderOpen.GetBuyerID() == nil || len(orderOpen.GetListings()) == 0 ||
		orderOpen.GetListings()[0].GetListing() == nil || orderOpen.GetListings()[0].GetListing().GetVendorID() == nil {
		return iwallet.EscrowInfo{}, fmt.Errorf("order escrow info requires complete order and payment messages")
	}
	escrowInfo := iwallet.EscrowInfo{Testnet: testnet}
	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return iwallet.EscrowInfo{}, fmt.Errorf("payment_sent missing settlement spec")
	}
	method := spec.GetMethod()
	if MethodIsDirect(method) {
		return iwallet.EscrowInfo{}, nil
	}
	switch method {
	case pb.PaymentSent_CANCELABLE:
		escrowInfo.RequiredSignatures = 1
	case pb.PaymentSent_MODERATED:
		escrowInfo.RequiredSignatures = 2
	default:
		return iwallet.EscrowInfo{}, nil
	}

	escrowInfo.ContractAddress = paymentSent.ContractAddress
	coinType := iwallet.CoinType(paymentSent.Coin)
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return iwallet.EscrowInfo{}, fmt.Errorf("invalid coin type %q: %w", coinType, err)
	}
	escrowInfo.CoinType = coinType
	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil || len(chainCode) < len(escrowInfo.UniqueId) {
		return iwallet.EscrowInfo{}, fmt.Errorf("payment chaincode must contain at least %d decoded bytes", len(escrowInfo.UniqueId))
	}
	copy(escrowInfo.UniqueId[:], chainCode[:len(escrowInfo.UniqueId)])
	escrowInfo.Amount = iwallet.NewAmount(paymentSent.Amount).Uint64()
	escrowInfo.UnlockHours = uint64(paymentSent.EscrowTimeoutHours)

	buyerID := orderOpen.GetBuyerID()
	vendorID := orderOpen.GetListings()[0].GetListing().GetVendorID()
	switch {
	case coinInfo.Chain == iwallet.ChainSolana:
		payer, err := solana.PublicKeyFromBase58(paymentSent.PayerAddress)
		if err != nil {
			return iwallet.EscrowInfo{}, fmt.Errorf("decode payer address: %w", err)
		}
		if len(buyerID.GetPubkeys().GetSolana()) != solana.PublicKeyLength || len(vendorID.GetPubkeys().GetSolana()) != solana.PublicKeyLength {
			return iwallet.EscrowInfo{}, fmt.Errorf("buyer and seller Solana public keys must be %d bytes", solana.PublicKeyLength)
		}
		escrowInfo.PayerAddress = payer.String()
		escrowInfo.BuyerAddress = solana.PublicKeyFromBytes(buyerID.GetPubkeys().GetSolana()).String()
		escrowInfo.SellerAddress = solana.PublicKeyFromBytes(vendorID.GetPubkeys().GetSolana()).String()
	case coinInfo.IsEthTypeChain():
		escrowInfo.PayerAddress = common.HexToAddress(paymentSent.PayerAddress).Hex()
		buyer, err := iwallet.PubKeyBytesToEthAddress(buyerID.GetPubkeys().GetEth())
		if err != nil {
			return iwallet.EscrowInfo{}, fmt.Errorf("decode buyer address: %w", err)
		}
		seller, err := iwallet.PubKeyBytesToEthAddress(vendorID.GetPubkeys().GetEth())
		if err != nil {
			return iwallet.EscrowInfo{}, fmt.Errorf("decode seller address: %w", err)
		}
		escrowInfo.BuyerAddress = buyer.String()
		escrowInfo.SellerAddress = seller.String()
	default:
		return iwallet.EscrowInfo{}, fmt.Errorf("unsupported escrow coin %q", coinType)
	}
	escrowInfo.RefundAddress = paymentSent.RefundAddress
	if escrowInfo.RefundAddress == "" {
		escrowInfo.RefundAddress = escrowInfo.BuyerAddress
	}
	escrowInfo.ModeratorAddress = paymentSent.ModeratorAddress
	return escrowInfo, nil
}
