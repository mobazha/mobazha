package guest

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"gorm.io/gorm"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/encryption"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/redact"
	"github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const defaultSweepFeePerByte int64 = 2

// AutoSweepService creates and processes sweep tasks that transfer funds from
// HD-derived payment addresses to the seller's receiving account address.
// Solana orders don't create sweep tasks because buyers pay directly to the seller.
type AutoSweepService struct {
	db          database.Database
	keyDeriver  BIP44KeyDeriver
	eventBus    events.Bus
	chainOps    utxo.ChainOperations
	multiwallet contracts.WalletOperator

	// broadcastCache holds txHash for tasks that were successfully broadcast
	// but whose DB save failed. RecoverStaleTasks consults this cache to
	// promote such tasks to "submitted" instead of re-broadcasting.
	broadcastMu    sync.Mutex
	broadcastCache map[int]string // taskID → txHash
}

// NewAutoSweepService creates an AutoSweepService.
func NewAutoSweepService(db database.Database, keyDeriver BIP44KeyDeriver, eventBus events.Bus) *AutoSweepService {
	return &AutoSweepService{
		db:         db,
		keyDeriver: keyDeriver,
		eventBus:   eventBus,
	}
}

// SetChainOps injects the ChainOperations used for UTXO discovery,
// broadcasting, and confirmation polling. Callers must inject this
// before sweeps can execute or advance to "confirmed".
func (s *AutoSweepService) SetChainOps(ops utxo.ChainOperations) {
	s.chainOps = ops
}

// SetMultiwallet injects the wallet operator used to resolve per-chain
// address utilities and sweep transaction builders.
func (s *AutoSweepService) SetMultiwallet(mw contracts.WalletOperator) {
	s.multiwallet = mw
}

// CreateSweepTask records a pending sweep task for a funded Guest Order.
// Called within the FUNDED transition transaction.
//
// The ChainKey is the chain identifier (e.g. "BTC", "LTC") rather than
// order.PaymentCoin, because PaymentCoin can be a canonical CoinType string
// while broadcasters are registered per chain. resolveBroadcaster does
// fallback parsing as defense in depth, but creating tasks with the canonical
// key keeps registry lookups O(1) and makes ops dashboards readable.
func (s *AutoSweepService) CreateSweepTask(tx database.Tx, order *models.GuestOrder) error {
	if order.SweepToAddress == "" {
		return nil
	}
	chainKey := order.PaymentCoin
	if coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(order.PaymentCoin)); err == nil {
		chainKey = string(coinInfo.Chain)
	}
	task := &models.SweepTask{
		OrderToken:   order.OrderToken,
		ChainKey:     chainKey,
		FromAddress:  order.PaymentAddress,
		ToAddress:    order.SweepToAddress,
		Amount:       order.PaymentAmount,
		AddressIndex: order.AddressIndex,
		Status:       models.SweepStatusPending,
	}
	return tx.Save(task)
}

// ClaimSweepTask atomically transitions a task from pending to processing.
// Returns true if the claim succeeded (this process owns the task).
func (s *AutoSweepService) ClaimSweepTask(taskID int) (bool, error) {
	var claimed bool
	err := s.db.Update(func(tx database.Tx) error {
		var task models.SweepTask
		if err := tx.Read().Where("id = ? AND status = ?", taskID, models.SweepStatusPending).
			First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		task.Status = models.SweepStatusProcessing
		task.UpdatedAt = time.Now()
		if err := tx.Save(&task); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}

// RecoverStaleTasks transitions tasks stuck in "processing" for longer than
// SweepStaleTimeout back to "pending" so they can be retried.
func (s *AutoSweepService) RecoverStaleTasks() {
	cutoff := time.Now().Add(-models.SweepStaleTimeout)
	var promotedIDs []int

	err := s.db.Update(func(tx database.Tx) error {
		var stale []models.SweepTask
		if err := tx.Read().Where("status = ? AND updated_at < ?",
			models.SweepStatusProcessing, cutoff).Find(&stale).Error; err != nil {
			return err
		}
		now := time.Now()
		for i := range stale {
			if cached := s.getBroadcast(stale[i].ID); cached != "" {
				stale[i].TxHash = cached
				stale[i].Status = models.SweepStatusSubmitted
				promotedIDs = append(promotedIDs, stale[i].ID)
			} else if stale[i].TxHash != "" {
				stale[i].Status = models.SweepStatusSubmitted
			} else {
				stale[i].Status = models.SweepStatusPending
				stale[i].RetryCount++
				if stale[i].RetryCount >= models.MaxSweepRetries {
					stale[i].Status = models.SweepStatusFailed
					stale[i].LastError = "stale recovery exhausted max retries"
				}
			}
			stale[i].UpdatedAt = now
			if err := tx.Save(&stale[i]); err != nil {
				return err
			}
		}
		return nil
	})

	if err == nil {
		for _, id := range promotedIDs {
			s.clearBroadcast(id)
		}
	}
}

// ProcessPendingSweeps finds pending sweep tasks and attempts to broadcast them.
// It also checks submitted sweeps for on-chain confirmations and reconciles
// funded orders whose sweep task creation failed.
func (s *AutoSweepService) ProcessPendingSweeps(ctx context.Context) {
	s.RecoverStaleTasks()
	s.reconcileOrphanedFunded()
	s.checkSubmittedSweeps(ctx)

	var tasks []models.SweepTask
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("status = ? AND retry_count < ?", models.SweepStatusPending, models.MaxSweepRetries).
			Find(&tasks).Error
	})
	if err != nil || len(tasks) == 0 {
		return
	}

	for i := range tasks {
		s.processSingleSweep(ctx, &tasks[i])
	}
}

func (s *AutoSweepService) processSingleSweep(ctx context.Context, task *models.SweepTask) {
	claimed, err := s.ClaimSweepTask(task.ID)
	if err != nil || !claimed {
		return
	}

	txHash, err := s.broadcastUTXOSweep(task)
	if err != nil {
		s.failTask(task, err.Error())
		return
	}

	s.recordBroadcast(task.ID, txHash)

	var saveErr error
	for attempt := 0; attempt < 3; attempt++ {
		saveErr = s.db.Update(func(tx database.Tx) error {
			task.Status = models.SweepStatusSubmitted
			task.TxHash = txHash
			task.UpdatedAt = time.Now()
			return tx.Save(task)
		})
		if saveErr == nil {
			s.clearBroadcast(task.ID)
			break
		}
		time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
	}
	if saveErr != nil {
		// TECHDEBT(TD-050): broadcastCache is process-local; if the process
		// crashes between here and the next RecoverStaleTasks cycle, the
		// txHash is lost.
		log.Errorf("sweep %d: broadcast succeeded (tx=%s) but DB save failed after 3 attempts: %v — cached for stale-recovery promotion", task.ID, txHash, saveErr)
	}
}

func (s *AutoSweepService) broadcastUTXOSweep(task *models.SweepTask) (string, error) {
	if s.chainOps == nil || s.multiwallet == nil {
		return "", fmt.Errorf("sweep infrastructure not initialised")
	}

	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(task.ChainKey))
	if err != nil {
		chain := iwallet.ChainType(task.ChainKey)
		if !chain.IsValid() {
			return "", fmt.Errorf("resolve chain for %s: %w", task.ChainKey, err)
		}
		coinInfo = iwallet.CoinInfo{Chain: chain}
	}

	addrUtils, err := utxoAddressUtilsFor(s.multiwallet, coinInfo.Chain)
	if err != nil {
		return "", fmt.Errorf("address utils for %s: %w", coinInfo.Chain, err)
	}

	sweeper, err := utxoSweeperFor(s.multiwallet, coinInfo.Chain)
	if err != nil {
		return "", fmt.Errorf("sweeper for %s: %w", coinInfo.Chain, err)
	}

	privKeyBytes, err := s.keyDeriver.DerivePrivateKey(coinInfo.Chain, task.AddressIndex)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}
	privKey, _ := btcec.PrivKeyFromBytes(privKeyBytes)
	defer encryption.ZeroBytes(privKeyBytes)

	pkScript, err := addrUtils.AddressToScriptPubKey(task.FromAddress)
	if err != nil {
		return "", fmt.Errorf("decode from address: %w", err)
	}

	utxos, err := s.chainOps.ListUnspent(coinInfo.Chain, pkScript)
	if err != nil {
		return "", fmt.Errorf("list unspent: %w", err)
	}
	if len(utxos) == 0 {
		return "", fmt.Errorf("no UTXOs available at %s", task.FromAddress)
	}

	inputs := make([]iwallet.SweepInput, 0, len(utxos))
	for _, u := range utxos {
		inputs = append(inputs, iwallet.SweepInput{
			TxHash:      u.TxHash,
			OutputIndex: u.OutputIndex,
			Value:       int64(u.Value),
		})
	}

	rawTx, _, err := sweeper.BuildSweepTx(inputs, *privKey, task.ToAddress, defaultSweepFeePerByte)
	if err != nil {
		return "", fmt.Errorf("build sweep tx: %w", err)
	}

	return s.chainOps.BroadcastTransaction(coinInfo.Chain, hex.EncodeToString(rawTx))
}

func (s *AutoSweepService) recordBroadcast(taskID int, txHash string) {
	s.broadcastMu.Lock()
	defer s.broadcastMu.Unlock()
	if s.broadcastCache == nil {
		s.broadcastCache = make(map[int]string)
	}
	s.broadcastCache[taskID] = txHash
}

func (s *AutoSweepService) getBroadcast(taskID int) string {
	s.broadcastMu.Lock()
	defer s.broadcastMu.Unlock()
	return s.broadcastCache[taskID]
}

func (s *AutoSweepService) clearBroadcast(taskID int) {
	s.broadcastMu.Lock()
	defer s.broadcastMu.Unlock()
	delete(s.broadcastCache, taskID)
}

func (s *AutoSweepService) failTask(task *models.SweepTask, errMsg string) {
	_ = s.db.Update(func(tx database.Tx) error {
		task.RetryCount++
		task.LastError = errMsg
		if task.RetryCount >= models.MaxSweepRetries {
			task.Status = models.SweepStatusFailed
		} else {
			task.Status = models.SweepStatusPending
		}
		task.UpdatedAt = time.Now()
		return tx.Save(task)
	})
}

// ConfirmSweep marks a submitted sweep as confirmed after on-chain verification.
func (s *AutoSweepService) ConfirmSweep(orderToken string, txHash string) error {
	return s.db.Update(func(tx database.Tx) error {
		var task models.SweepTask
		q := tx.Read().Where("order_token = ? AND status = ?",
			orderToken, models.SweepStatusSubmitted)
		if txHash != "" {
			q = q.Where("tx_hash = ?", txHash)
		}
		if err := q.First(&task).Error; err != nil {
			return fmt.Errorf("sweep task not found: %w", err)
		}
		task.Status = models.SweepStatusConfirmed
		task.UpdatedAt = time.Now()
		return tx.Save(&task)
	})
}

const sweepConfirmThreshold = 3

func (s *AutoSweepService) checkSubmittedSweeps(ctx context.Context) {
	if s.chainOps == nil {
		return
	}

	var submitted []models.SweepTask
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("status = ? AND tx_hash != ''",
			models.SweepStatusSubmitted).Find(&submitted).Error
	})
	if len(submitted) == 0 {
		return
	}

	for i := range submitted {
		task := &submitted[i]
		chain := chainFromTaskKey(task.ChainKey)
		if chain == "" || !s.chainOps.IsHealthy(chain) {
			continue
		}
		confs, err := s.chainOps.GetTxConfirmations(chain, task.TxHash)
		if err != nil {
			continue
		}
		if confs >= sweepConfirmThreshold {
			_ = s.ConfirmSweep(task.OrderToken, task.TxHash)
		}
	}
}

func chainFromTaskKey(chainKey string) iwallet.ChainType {
	if chainKey == "" {
		return ""
	}
	chain := iwallet.ChainType(chainKey)
	if chain.IsValid() {
		return chain
	}
	if coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(chainKey)); err == nil {
		return coinInfo.Chain
	}
	return ""
}

// reconcileOrphanedFunded finds guest orders that have a sweep_to_address
// but no corresponding SweepTask, and back-fills the missing task. This
// covers the case where the FUNDED DB save succeeded but CreateSweepTask
// failed (e.g. transient DB error, migration gap), as well as orders that
// advanced to SHIPPED/COMPLETED while the sweep task was lost.
func (s *AutoSweepService) reconcileOrphanedFunded() {
	sweepableStates := []int{
		int(models.GuestOrderFunded),
		int(models.GuestOrderShipped),
		int(models.GuestOrderCompleted),
	}
	_ = s.db.Update(func(tx database.Tx) error {
		var orphans []models.GuestOrder
		if err := tx.Read().
			Where("state IN ? AND sweep_to_address != ''", sweepableStates).
			Find(&orphans).Error; err != nil {
			return err
		}
		for i := range orphans {
			var count int64
			if err := tx.Read().Model(&models.SweepTask{}).
				Where("order_token = ?", orphans[i].OrderToken).
				Count(&count).Error; err != nil {
				log.Warningf("reconcile: count sweep tasks for %s: %v", redact.Token(orphans[i].OrderToken), err)
				continue
			}
			if count > 0 {
				continue
			}
			log.Infof("reconcile: creating missing sweep task for order %s (state=%s)", redact.Token(orphans[i].OrderToken), orphans[i].State)
			if err := s.CreateSweepTask(tx, &orphans[i]); err != nil {
				log.Warningf("reconcile: create sweep for %s: %v", redact.Token(orphans[i].OrderToken), err)
			}
		}
		return nil
	})
}

// RestorePendingSweeps is called on node restart to retry any incomplete sweeps.
func (s *AutoSweepService) RestorePendingSweeps(ctx context.Context) {
	s.ProcessPendingSweeps(ctx)
}
