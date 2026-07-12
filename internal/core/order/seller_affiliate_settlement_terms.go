package order

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// affiliatePayoutFromEscrowRelease returns the affiliate leg that the seller
// committed to in its signed shipment release. Settlement executors must use
// this message data rather than a node-local affiliate database so every Safe
// signer reconstructs the same transaction.
func affiliatePayoutFromEscrowRelease(release *pb.EscrowRelease) (*models.AffiliateSettlementPayout, error) {
	if release == nil {
		return nil, models.ErrInvalidSellerAffiliate
	}

	address := strings.TrimSpace(release.GetAffiliateAddress())
	amount := strings.TrimSpace(release.GetAffiliateAmount())
	if address == "" && amount == "" {
		return nil, nil
	}
	if address == "" && amount == "0" {
		return &models.AffiliateSettlementPayout{Amount: "0"}, nil
	}
	if address == "" || amount == "" || !common.IsHexAddress(address) {
		return nil, models.ErrInvalidSellerAffiliate
	}

	affiliateAmount, ok := new(big.Int).SetString(amount, 10)
	if !ok || affiliateAmount.Sign() <= 0 {
		return nil, models.ErrInvalidSellerAffiliate
	}
	sellerAmount, ok := new(big.Int).SetString(strings.TrimSpace(release.GetToAmount()), 10)
	if !ok || sellerAmount.Sign() <= 0 || affiliateAmount.Cmp(sellerAmount) >= 0 {
		return nil, models.ErrInvalidSellerAffiliate
	}

	return &models.AffiliateSettlementPayout{
		Address: common.HexToAddress(address).Hex(),
		Amount:  affiliateAmount.String(),
	}, nil
}

// affiliateUTXOPayoutFromEscrowRelease returns the seller-signed native UTXO
// payout leg. Unlike Safe, a UTXO release stores ToAmount after the affiliate
// amount has already been deducted, so only the chain wallet can validate the
// address format and dust threshold during transaction reconstruction.
func affiliateUTXOPayoutFromEscrowRelease(release *pb.EscrowRelease) (*models.AffiliateSettlementPayout, error) {
	if release == nil {
		return nil, models.ErrInvalidSellerAffiliate
	}
	address := strings.TrimSpace(release.GetAffiliateAddress())
	amount := strings.TrimSpace(release.GetAffiliateAmount())
	if address == "" && amount == "" {
		return nil, nil
	}
	if address == "" && amount == "0" {
		return &models.AffiliateSettlementPayout{Amount: "0"}, nil
	}
	if address == "" || amount == "" {
		return nil, models.ErrInvalidSellerAffiliate
	}
	affiliateAmount, ok := new(big.Int).SetString(amount, 10)
	if !ok || affiliateAmount.Sign() <= 0 {
		return nil, models.ErrInvalidSellerAffiliate
	}
	return &models.AffiliateSettlementPayout{Address: address, Amount: affiliateAmount.String()}, nil
}

// affiliatePayoutFromDisputeRelease preserves the seller-signed commission
// ratio when a dispute pays the seller only partially. The integer division is
// deliberate: any rounding remainder remains with the seller, and every party
// can deterministically rebuild the same Safe transaction.
func affiliatePayoutFromDisputeRelease(
	shipments []*pb.OrderShipment,
	release *pb.DisputeClose_ModeratedEscrowRelease,
) (*models.AffiliateSettlementPayout, error) {
	if release == nil {
		return nil, models.ErrInvalidSellerAffiliate
	}

	for _, shipment := range shipments {
		if shipment == nil || shipment.GetReleaseInfo() == nil {
			continue
		}
		original := shipment.GetReleaseInfo()
		if strings.TrimSpace(original.GetAffiliateAddress()) == "" && strings.TrimSpace(original.GetAffiliateAmount()) == "" {
			continue
		}
		payout, err := affiliatePayoutFromEscrowRelease(original)
		if err != nil {
			return nil, fmt.Errorf("invalid seller-signed affiliate release: %w", err)
		}
		if !common.IsHexAddress(original.GetToAddress()) || !common.IsHexAddress(release.GetVendorAddress()) ||
			common.HexToAddress(original.GetToAddress()) != common.HexToAddress(release.GetVendorAddress()) {
			return nil, models.ErrInvalidSellerAffiliate
		}

		vendorAmount, ok := new(big.Int).SetString(strings.TrimSpace(release.GetVendorAmount()), 10)
		if !ok || vendorAmount.Sign() < 0 {
			return nil, models.ErrInvalidSellerAffiliate
		}
		if vendorAmount.Sign() == 0 {
			return nil, nil
		}
		originalSellerAmount, ok := new(big.Int).SetString(strings.TrimSpace(original.GetToAmount()), 10)
		if !ok || originalSellerAmount.Sign() <= 0 || vendorAmount.Cmp(originalSellerAmount) > 0 {
			return nil, models.ErrInvalidSellerAffiliate
		}
		affiliateAmount, ok := new(big.Int).SetString(payout.Amount, 10)
		if !ok {
			return nil, models.ErrInvalidSellerAffiliate
		}
		scaledAmount := new(big.Int).Mul(affiliateAmount, vendorAmount)
		scaledAmount.Div(scaledAmount, originalSellerAmount)
		if scaledAmount.Sign() == 0 {
			return &models.AffiliateSettlementPayout{Amount: "0"}, nil
		}
		return &models.AffiliateSettlementPayout{
			Address: payout.Address,
			Amount:  scaledAmount.String(),
		}, nil
	}

	return nil, nil
}

func affiliatePayoutForDisputeSettlement(coinType iwallet.CoinType, shipments []*pb.OrderShipment, release *pb.DisputeClose_ModeratedEscrowRelease) (*models.AffiliateSettlementPayout, error) {
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err == nil && coinInfo.Chain.IsUTXOChain() {
		return affiliateUTXOPayoutFromDisputeRelease(shipments, release)
	}
	return affiliatePayoutFromDisputeRelease(shipments, release)
}

// requiresInterimAffiliateDisputeTerms protects the current shipment-backed
// payout terms until immutable PaymentAttempt settlement terms become the
// authority. Ordinary orders and disputes that award the seller nothing do not
// need a shipment merely to prove the absence of an affiliate output.
func requiresInterimAffiliateDisputeTerms(orderOpen *pb.OrderOpen, release *pb.DisputeClose_ModeratedEscrowRelease) bool {
	if orderOpen == nil || strings.TrimSpace(orderOpen.GetAffiliateReferralSessionID()) == "" {
		return false
	}
	if release == nil {
		return true
	}
	vendorAmount, ok := new(big.Int).SetString(strings.TrimSpace(release.GetVendorAmount()), 10)
	return !ok || vendorAmount.Sign() > 0
}

func requireInterimAffiliatePayout(orderOpen *pb.OrderOpen, payout *models.AffiliateSettlementPayout) error {
	if orderOpen == nil || strings.TrimSpace(orderOpen.GetAffiliateReferralSessionID()) == "" {
		return nil
	}
	if payout == nil {
		return models.ErrInvalidSellerAffiliate
	}
	return nil
}

func requireInterimAffiliateDisputePayout(
	orderOpen *pb.OrderOpen,
	release *pb.DisputeClose_ModeratedEscrowRelease,
	payout *models.AffiliateSettlementPayout,
) error {
	if requiresInterimAffiliateDisputeTerms(orderOpen, release) && payout == nil {
		return models.ErrInvalidSellerAffiliate
	}
	return nil
}

func affiliateUTXOPayoutFromDisputeRelease(shipments []*pb.OrderShipment, release *pb.DisputeClose_ModeratedEscrowRelease) (*models.AffiliateSettlementPayout, error) {
	if release == nil {
		return nil, models.ErrInvalidSellerAffiliate
	}
	for _, shipment := range shipments {
		if shipment == nil || shipment.GetReleaseInfo() == nil {
			continue
		}
		original := shipment.GetReleaseInfo()
		payout, err := affiliateUTXOPayoutFromEscrowRelease(original)
		if err != nil {
			return nil, err
		}
		if payout == nil {
			continue
		}
		if !payment.SameUTXOAddress(original.GetToAddress(), release.GetVendorAddress()) {
			return nil, models.ErrInvalidSellerAffiliate
		}
		vendorAmount, ok := new(big.Int).SetString(strings.TrimSpace(release.GetVendorAmount()), 10)
		if !ok || vendorAmount.Sign() < 0 {
			return nil, models.ErrInvalidSellerAffiliate
		}
		if vendorAmount.Sign() == 0 {
			return nil, nil
		}
		originalSellerAmount, ok := new(big.Int).SetString(strings.TrimSpace(original.GetToAmount()), 10)
		if !ok || originalSellerAmount.Sign() <= 0 {
			return nil, models.ErrInvalidSellerAffiliate
		}
		originalAffiliateAmount, ok := new(big.Int).SetString(payout.Amount, 10)
		if !ok {
			return nil, models.ErrInvalidSellerAffiliate
		}
		grossSellerAmount := new(big.Int).Add(originalSellerAmount, originalAffiliateAmount)
		if vendorAmount.Cmp(grossSellerAmount) > 0 {
			return nil, models.ErrInvalidSellerAffiliate
		}
		scaledAmount := new(big.Int).Mul(originalAffiliateAmount, vendorAmount)
		scaledAmount.Div(scaledAmount, grossSellerAmount)
		if scaledAmount.Sign() == 0 {
			return &models.AffiliateSettlementPayout{Amount: "0"}, nil
		}
		return &models.AffiliateSettlementPayout{Address: payout.Address, Amount: scaledAmount.String()}, nil
	}
	return nil, nil
}

func executableAffiliatePayout(payout *models.AffiliateSettlementPayout) (*models.AffiliateSettlementPayout, error) {
	if payout == nil {
		return nil, nil
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(payout.Amount), 10)
	if !ok || amount.Sign() < 0 {
		return nil, models.ErrInvalidSellerAffiliate
	}
	if amount.Sign() == 0 {
		return nil, nil
	}
	if strings.TrimSpace(payout.Address) == "" {
		return nil, models.ErrInvalidSellerAffiliate
	}
	return payout, nil
}
