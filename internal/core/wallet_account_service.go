package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

var _ contracts.WalletAccountService = (*walletAccountService)(nil)

var errWalletAddressCursorCreateRace = errors.New("wallet address cursor creation raced")

// walletAccountService is the UTXO implementation of the Wallet Account
// Service. It owns BIP44 account selection and address encoding, while callers
// only receive durable destinations.
type walletAccountService struct {
	db            database.Database
	masterKey     *hdkeychain.ExtendedKey
	multiwallet   contracts.WalletOperator
	reservationMu sync.Mutex
	transferMu    sync.Mutex
	chainOps      utxo.ChainOperations
}

// NewWalletAccountService constructs the node-local wallet account adapter.
// Hosted deployments provide the same contract through their tenant-scoped
// adapter rather than exposing a root key to Core business services.
func NewWalletAccountService(
	db database.Database,
	masterKey *hdkeychain.ExtendedKey,
	multiwallet contracts.WalletOperator,
) contracts.WalletAccountService {
	return &walletAccountService{db: db, masterKey: masterKey, multiwallet: multiwallet}
}

// Capabilities reports only operations that are closed end-to-end. Receiving
// is available for supported rails; Guest remains disabled until the generic
// spend/transfer path can consume account-role balances after restart.
func (s *walletAccountService) Capabilities(_ context.Context, railID string) (contracts.WalletCapabilities, error) {
	if s == nil || s.db == nil || s.masterKey == nil {
		return contracts.WalletCapabilities{}, fmt.Errorf("wallet account service is not configured")
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(strings.TrimSpace(railID)))
	if err != nil || !coinInfo.IsNative || (!isWalletUTXOChain(coinInfo.Chain) && !isWalletEVMChain(coinInfo.Chain)) {
		return contracts.WalletCapabilities{}, fmt.Errorf("wallet rail %q is not supported", railID)
	}
	capabilities := contracts.WalletCapabilities{Receive: true, Affiliate: true}
	if isWalletUTXOChain(coinInfo.Chain) {
		s.transferMu.Lock()
		defer s.transferMu.Unlock()
		if s.multiwallet == nil {
			return contracts.WalletCapabilities{}, fmt.Errorf("multiwallet not initialised for %s", coinInfo.Chain)
		}
		wallet, loaded := s.multiwallet.WalletForChain(coinInfo.Chain)
		_, supportsAddresses := wallet.(iwallet.UTXOAddressUtilities)
		_, supportsSweep := wallet.(iwallet.UTXOSweeper)
		_, supportsTransfer := wallet.(iwallet.UTXOTransferBuilder)
		if !loaded || !supportsAddresses {
			return contracts.WalletCapabilities{}, fmt.Errorf("wallet receiving adapter unavailable for %s", coinInfo.Chain)
		}
		capabilities.Watch = true
		if s.chainOps != nil && supportsSweep && supportsTransfer {
			capabilities.Spend = true
			capabilities.AutoTransfer = true
			capabilities.Guest = true
		}
	}
	return capabilities, nil
}

// ReserveAddress allocates or restores one UTXO receiving address. The cursor
// lock serializes allocation by tenant, rail, and role; the persisted reference
// makes retries and restarts return the original destination.
func (s *walletAccountService) ReserveAddress(
	ctx context.Context,
	railID string,
	role contracts.WalletAccountRole,
	referenceID string,
) (contracts.ReservedDestination, error) {
	if s == nil || s.db == nil || s.masterKey == nil {
		return contracts.ReservedDestination{}, fmt.Errorf("wallet account service is not configured")
	}
	railID = strings.TrimSpace(railID)
	referenceID = strings.TrimSpace(referenceID)
	if railID == "" || referenceID == "" {
		return contracts.ReservedDestination{}, fmt.Errorf("wallet address reservation requires rail and reference")
	}
	if !role.Valid() {
		return contracts.ReservedDestination{}, fmt.Errorf("unsupported wallet account role %q", role)
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(railID))
	if err != nil {
		return contracts.ReservedDestination{}, fmt.Errorf("parse wallet rail %q: %w", railID, err)
	}
	if !coinInfo.IsNative || (!isWalletUTXOChain(coinInfo.Chain) && !isWalletEVMChain(coinInfo.Chain)) {
		return contracts.ReservedDestination{}, fmt.Errorf("wallet rail %q is not a supported native receiving rail", railID)
	}
	s.reservationMu.Lock()
	defer s.reservationMu.Unlock()

	var result contracts.ReservedDestination
	for attempt := 0; attempt < 3; attempt++ {
		err = s.reserveAddress(ctx, railID, role, referenceID, coinInfo.Chain, &result)
		if !errors.Is(err, errWalletAddressCursorCreateRace) {
			break
		}
	}
	if err != nil {
		return contracts.ReservedDestination{}, err
	}
	return result, nil
}

func (s *walletAccountService) reserveAddress(
	ctx context.Context,
	railID string,
	role contracts.WalletAccountRole,
	referenceID string,
	chain iwallet.ChainType,
	result *contracts.ReservedDestination,
) error {
	return s.db.Update(func(tx database.Tx) error {
		cursor, err := s.lockCursor(ctx, tx, railID, role)
		if err != nil {
			return err
		}

		var existing models.WalletAddressReservation
		err = tx.Read().WithContext(ctx).
			Where("rail_id = ? AND account_role = ? AND reference_id = ?", railID, string(role), referenceID).
			First(&existing).Error
		if err == nil {
			*result = reservedDestinationFromModel(existing)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load wallet address reservation: %w", err)
		}

		address, err := s.deriveAddress(chain, role, cursor.NextIndex)
		if err != nil {
			return err
		}
		reservation := models.WalletAddressReservation{
			RailID: railID, AccountRole: string(role), ReferenceID: referenceID,
			AddressIndex: cursor.NextIndex, Address: address, Version: 1,
		}
		cursor.NextIndex++
		if err := tx.Save(cursor); err != nil {
			return fmt.Errorf("advance wallet address cursor: %w", err)
		}
		if err := tx.Create(&reservation); err != nil {
			return fmt.Errorf("persist wallet address reservation: %w", err)
		}
		*result = reservedDestinationFromModel(reservation)
		return nil
	})
}

func (s *walletAccountService) lockCursor(
	ctx context.Context,
	tx database.Tx,
	railID string,
	role contracts.WalletAccountRole,
) (*models.WalletAddressCursor, error) {
	cursor := models.WalletAddressCursor{RailID: railID, AccountRole: string(role)}
	err := tx.Read().WithContext(ctx).
		Where("rail_id = ? AND account_role = ?", railID, string(role)).First(&cursor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cursor = models.WalletAddressCursor{RailID: railID, AccountRole: string(role)}
		if err := tx.Create(&cursor); err != nil {
			return nil, fmt.Errorf("%w: %v", errWalletAddressCursorCreateRace, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("load wallet address cursor: %w", err)
	}
	if err := tx.Read().WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("rail_id = ? AND account_role = ?", railID, string(role)).First(&cursor).Error; err != nil {
		return nil, fmt.Errorf("lock wallet address cursor: %w", err)
	}
	return &cursor, nil
}

func (s *walletAccountService) deriveAddress(
	chainType iwallet.ChainType,
	role contracts.WalletAccountRole,
	index uint32,
) (string, error) {
	childKey, err := s.deriveChildKey(chainType, role, index)
	if err != nil {
		return "", err
	}
	pubKey, err := childKey.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("derive wallet public key: %w", err)
	}
	if isWalletEVMChain(chainType) {
		return ethcrypto.PubkeyToAddress(*pubKey.ToECDSA()).Hex(), nil
	}
	if s.multiwallet == nil {
		return "", fmt.Errorf("multiwallet not initialised for %s", chainType)
	}
	wallet, ok := s.multiwallet.WalletForChain(chainType)
	if !ok {
		return "", fmt.Errorf("wallet for %s not loaded", chainType)
	}
	utilities, ok := wallet.(iwallet.UTXOAddressUtilities)
	if !ok {
		return "", fmt.Errorf("wallet for %s does not implement UTXO address utilities", chainType)
	}
	address, _, err := utilities.DerivePaymentAddressFromPubKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("encode wallet address: %w", err)
	}
	return address, nil
}

func (s *walletAccountService) deriveChildKey(chainType iwallet.ChainType, role contracts.WalletAccountRole, index uint32) (*hdkeychain.ExtendedKey, error) {
	coinType, ok := iwallet.CanonicalBIP44CoinType(chainType)
	if !ok {
		return nil, fmt.Errorf("wallet rail %s has no BIP44 derivation", chainType)
	}
	coinKey, err := s.masterKey.Derive(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return nil, fmt.Errorf("derive wallet coin key: %w", err)
	}
	accountKey, err := coinKey.Derive(hdkeychain.HardenedKeyStart + walletAccountIndex(role))
	if err != nil {
		return nil, fmt.Errorf("derive wallet account key: %w", err)
	}
	changeKey, err := accountKey.Derive(0)
	if err != nil {
		return nil, fmt.Errorf("derive wallet change key: %w", err)
	}
	childKey, err := changeKey.Derive(index)
	if err != nil {
		return nil, fmt.Errorf("derive wallet address index %d: %w", index, err)
	}
	return childKey, nil
}

// SetChainOperations closes the runtime adapter after the UTXO monitor starts.
// Keeping this setter on the concrete implementation avoids exposing chain
// clients through the business-facing WalletAccountService contract.
func (s *walletAccountService) SetChainOperations(chainOps utxo.ChainOperations) {
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	s.chainOps = chainOps
}

// Transfer creates or resumes an idempotent wallet-domain transfer.
func (s *walletAccountService) Transfer(ctx context.Context, request contracts.WalletTransferRequest) (contracts.WalletTransfer, error) {
	if err := validateWalletTransferRequest(request); err != nil {
		return contracts.WalletTransfer{}, err
	}
	s.transferMu.Lock()
	defer s.transferMu.Unlock()

	transfer, err := s.ensureTransfer(ctx, request)
	if err != nil {
		return contracts.WalletTransfer{}, err
	}
	if transfer.RetryCount >= models.MaxWalletTransferRetries &&
		walletTransferRetryLimited(contracts.WalletTransferState(transfer.State)) {
		return walletTransferFromModel(transfer), fmt.Errorf("wallet transfer automatic retry limit reached")
	}
	err = s.processTransfer(ctx, &transfer)
	return walletTransferFromModel(transfer), err
}

// ReconcileTransfers resumes pre-broadcast transactions from their persisted
// raw bytes and refreshes submitted/confirmed transactions for reorgs.
func (s *walletAccountService) ReconcileTransfers(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("wallet account service is not configured")
	}
	s.transferMu.Lock()
	defer s.transferMu.Unlock()

	var transfers []models.WalletTransfer
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).
			Where("(state IN ? AND retry_count < ?) OR state IN ?", []string{
				string(contracts.WalletTransferPending), string(contracts.WalletTransferBuilt),
			}, models.MaxWalletTransferRetries, []string{
				string(contracts.WalletTransferSubmitted), string(contracts.WalletTransferConfirmed),
				string(contracts.WalletTransferReorged),
			}).
			Order("created_at ASC").Find(&transfers).Error
	})
	if err != nil {
		return fmt.Errorf("load wallet transfers: %w", err)
	}
	var firstErr error
	for i := range transfers {
		if err := s.processTransfer(ctx, &transfers[i]); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func validateWalletTransferRequest(request contracts.WalletTransferRequest) error {
	if strings.TrimSpace(request.RailID) == "" || strings.TrimSpace(request.ReferenceID) == "" ||
		strings.TrimSpace(request.Destination) == "" || strings.TrimSpace(request.IdempotencyKey) == "" {
		return fmt.Errorf("wallet transfer requires rail, reference, destination, and idempotency key")
	}
	if !request.Role.Valid() {
		return fmt.Errorf("unsupported wallet account role %q", request.Role)
	}
	if request.SweepAll == (request.Amount > 0) {
		return fmt.Errorf("wallet transfer requires exactly one of sweep-all or a positive amount")
	}
	if request.Amount > math.MaxInt64 {
		return fmt.Errorf("wallet transfer amount exceeds supported UTXO range")
	}
	return nil
}

func (s *walletAccountService) ensureTransfer(ctx context.Context, request contracts.WalletTransferRequest) (models.WalletTransfer, error) {
	request.RailID = strings.TrimSpace(request.RailID)
	request.ReferenceID = strings.TrimSpace(request.ReferenceID)
	request.Destination = strings.TrimSpace(request.Destination)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)

	var reservation models.WalletAddressReservation
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).Where(
			"rail_id = ? AND account_role = ? AND reference_id = ?",
			request.RailID, string(request.Role), request.ReferenceID,
		).First(&reservation).Error
	}); err != nil {
		return models.WalletTransfer{}, fmt.Errorf("load wallet transfer source reservation: %w", err)
	}

	idHash := sha256.Sum256([]byte(request.IdempotencyKey))
	created := models.WalletTransfer{
		ID: hex.EncodeToString(idHash[:]), IdempotencyKey: request.IdempotencyKey,
		RailID: request.RailID, AccountRole: string(request.Role), ReferenceID: request.ReferenceID,
		SourceAddress: reservation.Address, AddressIndex: reservation.AddressIndex,
		Destination: request.Destination, Amount: request.Amount, SweepAll: request.SweepAll,
		State: string(contracts.WalletTransferPending),
	}
	var result models.WalletTransfer
	err := s.db.Update(func(tx database.Tx) error {
		err := tx.Read().WithContext(ctx).Where("idempotency_key = ?", request.IdempotencyKey).First(&result).Error
		if err == nil {
			if !sameWalletTransferRequest(result, created) {
				return fmt.Errorf("wallet transfer idempotency key is already bound to a different request")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&created); err != nil {
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		// A concurrent process may have won the insert. Reload the durable
		// idempotency claim before surfacing the create error.
		var concurrent models.WalletTransfer
		loadErr := s.db.View(func(tx database.Tx) error {
			return tx.Read().WithContext(ctx).Where("idempotency_key = ?", request.IdempotencyKey).First(&concurrent).Error
		})
		if loadErr == nil {
			if !sameWalletTransferRequest(concurrent, created) {
				return models.WalletTransfer{}, fmt.Errorf("wallet transfer idempotency key is already bound to a different request")
			}
			return concurrent, nil
		}
		return models.WalletTransfer{}, fmt.Errorf("persist wallet transfer: %w", err)
	}
	return result, nil
}

func sameWalletTransferRequest(a, b models.WalletTransfer) bool {
	return a.RailID == b.RailID && a.AccountRole == b.AccountRole && a.ReferenceID == b.ReferenceID &&
		a.SourceAddress == b.SourceAddress && a.AddressIndex == b.AddressIndex &&
		a.Destination == b.Destination && a.Amount == b.Amount && a.SweepAll == b.SweepAll
}

func (s *walletAccountService) processTransfer(ctx context.Context, transfer *models.WalletTransfer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch contracts.WalletTransferState(transfer.State) {
	case contracts.WalletTransferPending:
		if err := s.buildTransfer(ctx, transfer); err != nil {
			return s.recordTransferError(transfer, err)
		}
		fallthrough
	case contracts.WalletTransferBuilt:
		if err := s.broadcastTransfer(transfer); err != nil {
			return s.recordTransferError(transfer, err)
		}
		fallthrough
	case contracts.WalletTransferSubmitted, contracts.WalletTransferConfirmed, contracts.WalletTransferReorged:
		if err := s.refreshTransferConfirmation(transfer); err != nil {
			return s.recordTransferError(transfer, err)
		}
		return nil
	default:
		return fmt.Errorf("wallet transfer %s has invalid state %q", transfer.ID, transfer.State)
	}
}

func (s *walletAccountService) buildTransfer(ctx context.Context, transfer *models.WalletTransfer) error {
	transfer.BuildAttempts++
	target := transfer
	candidate := *transfer
	transfer = &candidate
	chain, wallet, utilities, err := s.utxoTransferRuntime(transfer.RailID)
	if err != nil {
		return err
	}
	if _, err := utilities.AddressToScriptPubKey(transfer.Destination); err != nil {
		return fmt.Errorf("invalid wallet transfer destination: %w", err)
	}
	sourceScript, err := utilities.AddressToScriptPubKey(transfer.SourceAddress)
	if err != nil {
		return fmt.Errorf("invalid wallet transfer source: %w", err)
	}
	outputs, err := s.chainOps.ListUnspent(chain, sourceScript)
	if err != nil {
		return fmt.Errorf("list wallet transfer inputs: %w", err)
	}
	if len(outputs) == 0 {
		return fmt.Errorf("wallet transfer source has no spendable outputs")
	}

	var reserved []models.WalletTransferInput
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).Where("rail_id = ?", transfer.RailID).Find(&reserved).Error
	}); err != nil {
		return fmt.Errorf("load wallet transfer input reservations: %w", err)
	}
	reservedSet := make(map[string]struct{}, len(reserved))
	for _, input := range reserved {
		reservedSet[fmt.Sprintf("%s:%d", strings.ToLower(input.TxHash), input.OutputIndex)] = struct{}{}
	}
	inputs := make([]iwallet.SweepInput, 0, len(outputs))
	selected := make([]models.WalletTransferInput, 0, len(outputs))
	for _, output := range outputs {
		key := fmt.Sprintf("%s:%d", strings.ToLower(output.TxHash), output.OutputIndex)
		if _, exists := reservedSet[key]; exists {
			continue
		}
		if output.Value > math.MaxInt64 {
			return fmt.Errorf("wallet transfer input value exceeds supported range")
		}
		inputs = append(inputs, iwallet.SweepInput{TxHash: output.TxHash, OutputIndex: output.OutputIndex, Value: int64(output.Value)})
		selected = append(selected, models.WalletTransferInput{
			TransferID: transfer.ID, RailID: transfer.RailID, TxHash: output.TxHash,
			OutputIndex: output.OutputIndex, Value: output.Value,
		})
	}
	if len(inputs) == 0 {
		return fmt.Errorf("wallet transfer source has no unreserved outputs")
	}

	childKey, err := s.deriveChildKey(chain, contracts.WalletAccountRole(transfer.AccountRole), transfer.AddressIndex)
	if err != nil {
		return err
	}
	privateKey, err := childKey.ECPrivKey()
	if err != nil {
		return fmt.Errorf("derive wallet transfer signer: %w", err)
	}
	const feeTargetBlocks = 2
	feePerByte := int64(s.chainOps.GetFeeEstimate(chain, feeTargetBlocks))
	if feePerByte < 1 {
		feePerByte = 1
	}
	var rawTx []byte
	var txHash string
	if transfer.SweepAll {
		builder, ok := wallet.(iwallet.UTXOSweeper)
		if !ok {
			return fmt.Errorf("wallet sweep adapter unavailable for %s", chain)
		}
		rawTx, txHash, err = builder.BuildSweepTx(inputs, *privateKey, transfer.Destination, feePerByte)
	} else {
		builder, ok := wallet.(iwallet.UTXOTransferBuilder)
		if !ok {
			return fmt.Errorf("wallet transfer adapter unavailable for %s", chain)
		}
		rawTx, txHash, err = builder.BuildTransferTx(inputs, *privateKey, transfer.SourceAddress, transfer.Destination, int64(transfer.Amount), feePerByte)
	}
	if err != nil {
		return fmt.Errorf("build wallet transfer: %w", err)
	}
	if len(rawTx) == 0 || strings.TrimSpace(txHash) == "" {
		return fmt.Errorf("wallet transfer builder returned incomplete transaction")
	}

	transfer.RawTxHex = hex.EncodeToString(rawTx)
	transfer.TxHash = strings.TrimSpace(txHash)
	transfer.AttemptTxHashes = appendWalletTransferAttemptHash(transfer.AttemptTxHashes, transfer.TxHash)
	transfer.FeeTargetBlocks = feeTargetBlocks
	transfer.FeePerByte = uint64(feePerByte)
	transfer.State = string(contracts.WalletTransferBuilt)
	transfer.RetryCount = 0
	transfer.LastError = ""
	transfer.UpdatedAt = time.Now().UTC()
	if err := s.db.Update(func(tx database.Tx) error {
		for i := range selected {
			if err := tx.Create(&selected[i]); err != nil {
				return fmt.Errorf("reserve wallet transfer input: %w", err)
			}
		}
		return tx.Save(transfer)
	}); err != nil {
		return fmt.Errorf("persist built wallet transfer: %w", err)
	}
	*target = candidate
	return nil
}

func (s *walletAccountService) broadcastTransfer(transfer *models.WalletTransfer) error {
	transfer.BroadcastAttempts++
	chain, _, _, err := s.utxoTransferRuntime(transfer.RailID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(transfer.RawTxHex) == "" || strings.TrimSpace(transfer.TxHash) == "" {
		return fmt.Errorf("built wallet transfer is missing raw transaction or hash")
	}
	broadcastHash, err := s.chainOps.BroadcastTransaction(chain, transfer.RawTxHex)
	if err != nil {
		known, lookupErr := s.chainOps.GetTransaction(chain, transfer.TxHash)
		if lookupErr != nil || known == nil {
			return fmt.Errorf("broadcast wallet transfer: %w", err)
		}
	} else if !strings.EqualFold(strings.TrimSpace(broadcastHash), transfer.TxHash) {
		return fmt.Errorf("wallet transfer broadcast hash mismatch: built %s, broadcast %s", transfer.TxHash, broadcastHash)
	}
	transfer.State = string(contracts.WalletTransferSubmitted)
	transfer.RetryCount = 0
	transfer.LastError = ""
	transfer.UpdatedAt = time.Now().UTC()
	return s.db.Update(func(tx database.Tx) error { return tx.Save(transfer) })
}

func (s *walletAccountService) refreshTransferConfirmation(transfer *models.WalletTransfer) error {
	chain, _, _, err := s.utxoTransferRuntime(transfer.RailID)
	if err != nil {
		return err
	}
	previous := contracts.WalletTransferState(transfer.State)
	confirmations, confirmationErr := s.chainOps.GetTxConfirmations(chain, transfer.TxHash)
	if confirmationErr != nil {
		if previous != contracts.WalletTransferConfirmed && previous != contracts.WalletTransferReorged {
			return fmt.Errorf("check wallet transfer confirmation: %w", confirmationErr)
		}
		known, lookupErr := s.chainOps.GetTransaction(chain, transfer.TxHash)
		switch {
		case errors.Is(lookupErr, utxo.ErrTransactionNotFound):
			if previous == contracts.WalletTransferReorged {
				return s.recoverEvictedTransfer(transfer)
			}
			confirmations = 0
		case lookupErr != nil:
			return errors.Join(
				fmt.Errorf("check wallet transfer confirmation: %w", confirmationErr),
				fmt.Errorf("check reorged wallet transfer presence: %w", lookupErr),
			)
		case known == nil:
			return fmt.Errorf("check reorged wallet transfer presence: chain returned no transaction and no not-found result")
		default:
			return fmt.Errorf("check wallet transfer confirmation: %w", confirmationErr)
		}
	}
	if confirmations == 0 && previous == contracts.WalletTransferReorged {
		// A prior reconciliation already observed this transaction drop to
		// zero confirmations. If the chain no longer knows about it at all,
		// it will never reconfirm on its own; release its reserved inputs
		// and rebuild instead of polling a dead TxHash forever.
		known, lookupErr := s.chainOps.GetTransaction(chain, transfer.TxHash)
		if errors.Is(lookupErr, utxo.ErrTransactionNotFound) {
			return s.recoverEvictedTransfer(transfer)
		}
		if lookupErr != nil {
			return fmt.Errorf("check reorged wallet transfer presence: %w", lookupErr)
		}
		if known == nil {
			return fmt.Errorf("check reorged wallet transfer presence: chain returned no transaction and no not-found result")
		}
	}
	transfer.Confirmations = confirmations
	transfer.RetryCount = 0
	if confirmations > 0 {
		transfer.State = string(contracts.WalletTransferConfirmed)
	} else if previous == contracts.WalletTransferConfirmed {
		transfer.State = string(contracts.WalletTransferReorged)
	} else if previous == contracts.WalletTransferReorged {
		transfer.State = string(contracts.WalletTransferReorged)
	} else {
		transfer.State = string(contracts.WalletTransferSubmitted)
	}
	transfer.LastError = ""
	transfer.UpdatedAt = time.Now().UTC()
	return s.db.Update(func(tx database.Tx) error { return tx.Save(transfer) })
}

// recoverEvictedTransfer releases the inputs reserved for a reorged transfer
// whose transaction the chain no longer knows about and returns it to
// Pending so the next processTransfer call selects fresh unspent outputs and
// builds a new transaction. The idempotency key, source, destination, and
// amount are unchanged, so a duplicate Transfer call still resolves to the
// same logical transfer.
func (s *walletAccountService) recoverEvictedTransfer(transfer *models.WalletTransfer) error {
	transfer.State = string(contracts.WalletTransferPending)
	transfer.RawTxHex = ""
	transfer.TxHash = ""
	transfer.FeeTargetBlocks = 0
	transfer.FeePerByte = 0
	transfer.Confirmations = 0
	transfer.RetryCount = 0
	transfer.LastError = "transaction evicted after reorg; rebuilding"
	transfer.UpdatedAt = time.Now().UTC()
	return s.db.Update(func(tx database.Tx) error {
		if err := tx.Delete("transfer_id", transfer.ID, nil, &models.WalletTransferInput{}); err != nil {
			return fmt.Errorf("release reorged wallet transfer inputs: %w", err)
		}
		return tx.Save(transfer)
	})
}

func appendWalletTransferAttemptHash(existing []string, txHash string) []string {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return existing
	}
	for _, candidate := range existing {
		if strings.EqualFold(strings.TrimSpace(candidate), txHash) {
			return existing
		}
	}
	return append(existing, txHash)
}

func (s *walletAccountService) recordTransferError(transfer *models.WalletTransfer, cause error) error {
	if walletTransferRetryLimited(contracts.WalletTransferState(transfer.State)) {
		transfer.RetryCount++
	}
	transfer.LastError = cause.Error()
	transfer.UpdatedAt = time.Now().UTC()
	if err := s.db.Update(func(tx database.Tx) error { return tx.Save(transfer) }); err != nil {
		return errors.Join(cause, fmt.Errorf("persist wallet transfer failure: %w", err))
	}
	return cause
}

func walletTransferRetryLimited(state contracts.WalletTransferState) bool {
	return state == contracts.WalletTransferPending || state == contracts.WalletTransferBuilt
}

func (s *walletAccountService) utxoTransferRuntime(railID string) (iwallet.ChainType, iwallet.Wallet, iwallet.UTXOAddressUtilities, error) {
	if s == nil || s.db == nil || s.masterKey == nil || s.multiwallet == nil || s.chainOps == nil {
		return "", nil, nil, fmt.Errorf("wallet transfer infrastructure is not configured")
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(strings.TrimSpace(railID)))
	if err != nil || !coinInfo.IsNative || !isWalletUTXOChain(coinInfo.Chain) {
		return "", nil, nil, fmt.Errorf("wallet transfer rail %q is not supported", railID)
	}
	wallet, ok := s.multiwallet.WalletForChain(coinInfo.Chain)
	if !ok {
		return "", nil, nil, fmt.Errorf("wallet for %s not loaded", coinInfo.Chain)
	}
	utilities, ok := wallet.(iwallet.UTXOAddressUtilities)
	if !ok {
		return "", nil, nil, fmt.Errorf("wallet address adapter unavailable for %s", coinInfo.Chain)
	}
	return coinInfo.Chain, wallet, utilities, nil
}

func walletTransferFromModel(transfer models.WalletTransfer) contracts.WalletTransfer {
	return contracts.WalletTransfer{
		IdempotencyKey: transfer.IdempotencyKey,
		State:          contracts.WalletTransferState(transfer.State), TxHash: transfer.TxHash,
		Confirmations: transfer.Confirmations, LastError: transfer.LastError,
	}
}

func isWalletUTXOChain(chainType iwallet.ChainType) bool {
	switch chainType {
	case iwallet.ChainBitcoin, iwallet.ChainBitcoinCash, iwallet.ChainLitecoin:
		return true
	default:
		return false
	}
}

func isWalletEVMChain(chainType iwallet.ChainType) bool {
	switch chainType {
	case iwallet.ChainEthereum, iwallet.ChainBSC, iwallet.ChainPolygon, iwallet.ChainBase:
		return true
	default:
		return false
	}
}

func walletAccountIndex(role contracts.WalletAccountRole) uint32 {
	switch role {
	case contracts.AccountGuest:
		return 1
	case contracts.AccountAffiliate:
		return 2
	default:
		return 0
	}
}

func reservedDestinationFromModel(reservation models.WalletAddressReservation) contracts.ReservedDestination {
	return contracts.ReservedDestination{
		Destination: contracts.Destination{
			RailID: reservation.RailID, Address: reservation.Address,
			Tag: reservation.Tag, Version: reservation.Version,
		},
		Index: reservation.AddressIndex,
	}
}
