//go:build !private_distribution

package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mobazha/mobazha3.0/internal/core/guest"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type distributionManagedEscrowWatchSource struct {
	node *MobazhaNode
}

func (s distributionManagedEscrowWatchSource) ListManagedEscrowWatches(ctx context.Context) ([]distribution.ManagedEscrowWatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.node == nil || s.node.db == nil {
		return nil, fmt.Errorf("managed escrow watch source: database unavailable")
	}
	watches, err := s.regularOrderWatches()
	if err != nil {
		return nil, err
	}
	guestWatches, err := s.guestOrderWatches()
	if err != nil {
		return nil, err
	}
	return append(watches, guestWatches...), nil
}

func (s distributionManagedEscrowWatchSource) regularOrderWatches() ([]distribution.ManagedEscrowWatch, error) {
	var orders []models.Order
	if err := s.node.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("payment_address <> ''").
			Where("payment_verification_status <> ?", models.PaymentVerificationStatusVerified).
			Find(&orders).Error
	}); err != nil {
		return nil, fmt.Errorf("managed escrow watch source: load pending orders: %w", err)
	}
	watches := make([]distribution.ManagedEscrowWatch, 0, len(orders))
	for i := range orders {
		watch, err := s.regularOrderWatch(&orders[i])
		if err != nil {
			logger.LogWarningWithIDf(log, s.node.nodeID, "Managed escrow watch skipped for order %s: %v", orders[i].ID, err)
			continue
		}
		if watch.OrderID != "" {
			watches = append(watches, watch)
		}
	}
	return watches, nil
}

func (s distributionManagedEscrowWatchSource) regularOrderWatch(order *models.Order) (distribution.ManagedEscrowWatch, error) {
	info, err := order.GetPendingManagedEscrowPaymentInfo()
	if err != nil || info == nil {
		return distribution.ManagedEscrowWatch{}, err
	}
	if !common.IsHexAddress(info.Address) || info.Amount == 0 {
		return distribution.ManagedEscrowWatch{}, fmt.Errorf("invalid address or amount")
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(strings.TrimSpace(info.Coin)))
	if err != nil || !coinInfo.IsEthTypeChain() {
		return distribution.ManagedEscrowWatch{}, fmt.Errorf("unsupported managed escrow coin %q", info.Coin)
	}
	tokenAddress := ""
	if !coinInfo.IsNative {
		tokenAddress = coinInfo.ContractAddress(s.node.managed_escrowRuntimeUsesTestnet(coinInfo.Chain))
		if !common.IsHexAddress(tokenAddress) || common.HexToAddress(tokenAddress) == (common.Address{}) {
			return distribution.ManagedEscrowWatch{}, fmt.Errorf("missing token contract")
		}
	}
	chainID := s.node.runtimeManagedEscrowChainID(coinInfo.Chain)
	if chainID == 0 {
		return distribution.ManagedEscrowWatch{}, fmt.Errorf("missing runtime chain ID")
	}
	return distribution.ManagedEscrowWatch{
		OrderID: order.ID.String(), Chain: coinInfo.Chain, ChainID: chainID,
		Address: common.HexToAddress(info.Address).Hex(), TokenAddress: tokenAddress,
		ExpectedAmount: strconv.FormatUint(info.Amount, 10),
		Deadline:       time.Now().Add(defaultManagedEscrowRewatchFundingTimeout),
	}, nil
}

func (s distributionManagedEscrowWatchSource) guestOrderWatches() ([]distribution.ManagedEscrowWatch, error) {
	var orders []models.GuestOrder
	if err := s.node.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("state IN ?", []int{int(models.GuestOrderAwaitingPayment), int(models.GuestOrderPaymentDetected)}).
			Where("evm_managed_escrow_metadata IS NOT NULL AND evm_managed_escrow_metadata <> ''").
			Find(&orders).Error
	}); err != nil {
		return nil, fmt.Errorf("managed escrow watch source: load pending guest orders: %w", err)
	}
	watches := make([]distribution.ManagedEscrowWatch, 0, len(orders))
	for i := range orders {
		if !orders[i].HasEVMManagedEscrowFundingTarget() || time.Now().After(orders[i].ExpiresAt.Add(evmGuestManagedEscrowRewatchGrace)) {
			continue
		}
		watch, err := guest.ManagedEscrowWatchForGuestOrder(&orders[i], s.node.walletTestnet)
		if err != nil {
			logger.LogWarningWithIDf(log, s.node.nodeID, "Managed escrow guest watch skipped for %s: %v", orders[i].OrderToken, err)
			continue
		}
		watches = append(watches, watch)
	}
	return watches, nil
}

var _ distribution.ManagedEscrowWatchSource = distributionManagedEscrowWatchSource{}
