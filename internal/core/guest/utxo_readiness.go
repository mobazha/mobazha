package guest

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// UTXOMonitorReadiness exposes UTXO monitor health used for guest checkout gating.
type UTXOMonitorReadiness interface {
	IsHealthy(chain iwallet.ChainType) bool
	GetHealthySourceCount(chain iwallet.ChainType) int
	GetWatchedAddressCount() int
}

// SetUTXOMonitor wires the shared UTXO monitor for health and watch counts.
func (s *GuestOrderAppService) SetUTXOMonitor(mon UTXOMonitorReadiness) {
	s.utxoMonitor = mon
}

// SetMultiwallet wires the wallet operator for per-chain load checks.
func (s *GuestOrderAppService) SetMultiwallet(mw contracts.WalletOperator) {
	s.multiwallet = mw
}

func (s *GuestOrderAppService) hasActiveReceivingAccount(chain iwallet.ChainType) bool {
	if s == nil || s.db == nil {
		return false
	}
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.ReceivingAccount{}).
			Where("chain_type = ? AND is_active = ?", string(chain), true).
			Count(&count).Error
	})
	return err == nil && count > 0
}

func (s *GuestOrderAppService) evaluateUTXOClosureReadiness(coinType iwallet.CoinType, coinInfo iwallet.CoinInfo) error {
	if err := s.validateCoinAvailability(coinType, coinInfo); err != nil {
		return err
	}
	if s.walletAccounts == nil {
		return fmt.Errorf("%w: wallet account service is not configured", contracts.ErrCoinUnavailable)
	}
	capabilities, err := s.walletAccounts.Capabilities(context.Background(), string(coinType))
	if err != nil {
		return fmt.Errorf("%w: wallet account capability unavailable: %v", contracts.ErrCoinUnavailable, err)
	}
	if !capabilities.Guest {
		return fmt.Errorf("%w: wallet account spend and recovery are not complete for %s", contracts.ErrCoinUnavailable, coinInfo.Chain)
	}
	if s.multiwallet == nil {
		return fmt.Errorf("%w: multiwallet is not configured", contracts.ErrCoinUnavailable)
	}
	wallet, loaded := s.multiwallet.WalletForChain(coinInfo.Chain)
	if !loaded {
		return fmt.Errorf("%w: wallet for %s is not loaded", contracts.ErrCoinUnavailable, coinInfo.Chain)
	}
	if _, sweepable := wallet.(iwallet.UTXOSweeper); !sweepable {
		return fmt.Errorf("%w: wallet sweep-all transfer is not supported for chain %s", contracts.ErrCoinUnavailable, coinInfo.Chain)
	}
	if _, transferable := wallet.(iwallet.UTXOTransferBuilder); !transferable {
		return fmt.Errorf("%w: exact wallet transfer is not supported for chain %s", contracts.ErrCoinUnavailable, coinInfo.Chain)
	}
	if s.utxoMonitor == nil {
		return fmt.Errorf("%w: UTXO monitor is not configured", contracts.ErrCoinUnavailable)
	}
	if !s.utxoMonitor.IsHealthy(coinInfo.Chain) {
		return fmt.Errorf("%w: UTXO monitor has no healthy sources for %s", contracts.ErrCoinUnavailable, coinInfo.Chain)
	}
	return nil
}

// GetGuestCheckoutReadiness returns operator-facing UTXO guest checkout health.
func (s *GuestOrderAppService) GetGuestCheckoutReadiness(ctx context.Context) (*contracts.GuestCheckoutReadiness, error) {
	if s == nil {
		return &contracts.GuestCheckoutReadiness{}, nil
	}
	out := &contracts.GuestCheckoutReadiness{
		GuestCheckoutEnabled: s.IsEnabled(ctx),
	}
	if s.utxoMonitor != nil {
		out.WatchedAddressCount = s.utxoMonitor.GetWatchedAddressCount()
	}
	if s.db != nil {
		var pending, submitted, failed int64
		_ = s.db.View(func(tx database.Tx) error {
			r := tx.Read()
			if err := r.Model(&models.WalletTransfer{}).Where("state IN ? AND retry_count < ?", []string{
				string(contracts.WalletTransferPending), string(contracts.WalletTransferBuilt), string(contracts.WalletTransferReorged),
			}, models.MaxWalletTransferRetries).Count(&pending).Error; err != nil {
				return err
			}
			if err := r.Model(&models.WalletTransfer{}).Where("state = ?", string(contracts.WalletTransferSubmitted)).Count(&submitted).Error; err != nil {
				return err
			}
			return r.Model(&models.WalletTransfer{}).Where("retry_count >= ?", models.MaxWalletTransferRetries).Count(&failed).Error
		})
		out.SweepTasksPending = int(pending)
		out.SweepTasksSubmitted = int(submitted)
		out.SweepTasksFailed = int(failed)
	}

	chains := s.supportedUTXOChainList()
	out.Chains = make([]contracts.GuestUTXOChainReadiness, 0, len(chains))
	for _, chain := range chains {
		coinType, _ := iwallet.CanonicalNativeCoinType(chain)
		coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
		if err != nil {
			continue
		}
		entry := contracts.GuestUTXOChainReadiness{Chain: string(chain)}
		if s.utxoMonitor != nil {
			entry.HealthySourceCount = s.utxoMonitor.GetHealthySourceCount(chain)
		}
		if s.multiwallet != nil {
			_, entry.WalletLoaded = s.multiwallet.WalletForChain(chain)
		}
		entry.ReceivingAccountActive = s.hasActiveReceivingAccount(chain)
		cap := s.evaluateGuestPaymentCapability(coinType, coinInfo)
		entry.BuyerVisible = cap.BuyerVisible
		entry.Reason = cap.Reason
		out.Chains = append(out.Chains, entry)
	}
	s.appendEVMReadiness(out)
	return out, nil
}

func (s *GuestOrderAppService) supportedUTXOChainList() []iwallet.ChainType {
	s.utxoMu.RLock()
	defer s.utxoMu.RUnlock()
	out := make([]iwallet.ChainType, 0, len(s.supportedUTXOChains))
	for chain := range s.supportedUTXOChains {
		out = append(out, chain)
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i]) < string(out[j])
	})
	return out
}

// LogGuestUTXOReadinessSummary emits a one-line summary for ops logs after monitor startup.
func (s *GuestOrderAppService) LogGuestUTXOReadinessSummary(ctx context.Context, nodeID string) {
	if s == nil {
		return
	}
	report, err := s.GetGuestCheckoutReadiness(ctx)
	if err != nil || report == nil {
		return
	}
	var parts []string
	for _, ch := range report.Chains {
		vis := "hidden"
		if ch.BuyerVisible {
			vis = "visible"
		}
		parts = append(parts, fmt.Sprintf("%s:sources=%d/%s", ch.Chain, ch.HealthySourceCount, vis))
	}
	logger.LogInfoWithIDf(log, nodeID,
		"Guest UTXO readiness: watched=%d transfer_pending=%d transfer_failed=%d [%s]",
		report.WatchedAddressCount, report.SweepTasksPending, report.SweepTasksFailed,
		strings.Join(parts, " "))
}
