// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	dbgorm "github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	"github.com/stretchr/testify/require"
)

type managedRelayRetryRelayer struct {
	hash common.Hash
	err  error
	call *relay.EVMRelayRequest
}

func (s *managedRelayRetryRelayer) Execute(_ context.Context, call *relay.EVMRelayRequest) (*relay.EVMRelayResponse, error) {
	s.call = call
	if s.err != nil {
		return nil, s.err
	}
	return &relay.EVMRelayResponse{TxHash: s.hash.Hex()}, nil
}

func (*managedRelayRetryRelayer) GetSupportedChains() []string { return []string{"bsc"} }
func (*managedRelayRetryRelayer) IsAvailable() bool            { return true }
func (*managedRelayRetryRelayer) GetGasWalletAddress(context.Context, uint64) (string, error) {
	return "0x0000000000000000000000000000000000000001", nil
}
func (*managedRelayRetryRelayer) GetGasWalletStatus(context.Context, uint64) (*relay.EVMGasWalletStatus, error) {
	return &relay.EVMGasWalletStatus{Address: "0x0000000000000000000000000000000000000001", Balance: big.NewInt(1), Healthy: true}, nil
}
func (*managedRelayRetryRelayer) ChainTypeForID(uint64) (string, error) { return "bsc", nil }

type managedRelayConfirmationClient struct {
	receipts map[common.Hash]*types.Receipt
	known    map[common.Hash]bool
	head     uint64
}

type recordingManagedEscrowReceiptValidator struct {
	request payment.ManagedEscrowReceiptValidationRequest
	err     error
}

func (v *recordingManagedEscrowReceiptValidator) ValidateManagedEscrowReceipt(
	_ context.Context,
	request payment.ManagedEscrowReceiptValidationRequest,
) error {
	v.request = request
	return v.err
}

func (c managedRelayConfirmationClient) TransactionReceipt(_ context.Context, hash common.Hash) (*types.Receipt, error) {
	if receipt := c.receipts[hash]; receipt != nil {
		return receipt, nil
	}
	return nil, ethereum.NotFound
}

func (c managedRelayConfirmationClient) TransactionByHash(_ context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	if c.known[hash] {
		return types.NewTx(&types.LegacyTx{}), true, nil
	}
	return nil, false, ethereum.NotFound
}

func (c managedRelayConfirmationClient) HeaderByNumber(_ context.Context, _ *big.Int) (*types.Header, error) {
	return &types.Header{Number: big.NewInt(int64(c.head))}, nil
}

func TestRunManagedRelayConfirmationsOnce_InvalidTxHashMarksFailed(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	row := models.SettlementAction{
		ActionID:    "act-invalid-hash",
		OrderID:     "order-1",
		ActionKind:  "complete",
		ChainID:     56,
		State:       "submitted",
		TxHash:      "not-a-hash",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		LastError:   "",
		RelayTaskID: "task-1",
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&row)
	}))

	node := &MobazhaNode{
		identityFields: identityFields{nodeID: "test-node"},
		storageFields:  storageFields{db: db},
		walletFields:   walletFields{multiwallet: &mockWalletOperator{}},
	}

	node.runManagedRelayConfirmationsOnce(context.Background())

	var got models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", row.ActionID).First(&got).Error
	}))
	require.Equal(t, "failed", got.State)
	require.Equal(t, "relay projection missing valid tx hash", got.LastError)
}

func TestLoadPendingManagedRelayActions_SkipsNonEVMSettlementActions(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	evmRow := models.SettlementAction{
		ActionID:   "act-evm",
		OrderID:    "order-1",
		ActionKind: "confirm",
		ChainID:    56,
		State:      "submitted",
		TxHash:     "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	solanaRow := models.SettlementAction{
		ActionID:   "act-solana",
		OrderID:    "order-2",
		ActionKind: "confirm",
		ChainID:    0,
		State:      "submitted",
		TxHash:     "5j7sX3wYqW9p9hJmE2zZqY4q3YkX4tL8pQ9nS6uV1mN2",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Save(&evmRow); err != nil {
			return err
		}
		return tx.Save(&solanaRow)
	}))

	node := &MobazhaNode{storageFields: storageFields{db: db}}
	rows, err := node.loadPendingManagedRelayActions(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, evmRow.ActionID, rows[0].ActionID)
}

func TestReconcileManagedRelayAction_SubmittingWithoutTxWaitsThenRetries(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	newHash := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	relayer := &managedRelayRetryRelayer{hash: common.HexToHash(newHash)}
	node := &MobazhaNode{
		identityFields: identityFields{nodeID: "test-node"},
		storageFields:  storageFields{db: db},
		walletFields:   walletFields{evmRelay: relayer},
	}
	fresh := models.SettlementAction{
		ActionID:   "act-submitting-fresh",
		OrderID:    "order-fresh",
		ActionKind: "confirm",
		ChainID:    56,
		To:         "0x1111111111111111111111111111111111111111",
		Data:       "0xdeadbeef",
		State:      "submitting",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	stale := fresh
	stale.ActionID = "act-submitting-stale"
	stale.OrderID = "order-stale"
	stale.CreatedAt = time.Now().Add(-managedRelayConfirmationTimeout).UTC()
	stale.UpdatedAt = time.Now().Add(-managedRelayConfirmationTimeout).UTC()
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Save(&fresh); err != nil {
			return err
		}
		return tx.Save(&stale)
	}))

	node.reconcileManagedRelayAction(context.Background(), fresh)
	var gotFresh models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", fresh.ActionID).First(&gotFresh).Error
	}))
	require.Equal(t, "submitting", gotFresh.State)
	require.Empty(t, gotFresh.TxHash)
	require.Empty(t, gotFresh.LastError)

	node.reconcileManagedRelayAction(context.Background(), stale)
	var gotStale models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", stale.ActionID).First(&gotStale).Error
	}))
	require.Equal(t, "submitted", gotStale.State)
	require.Equal(t, newHash, gotStale.TxHash)
	require.Equal(t, 1, gotStale.Attempts)
	require.Contains(t, gotStale.AttemptTxHashes, newHash)
}

func TestValidateManagedEscrowReceiptDelegatesToBoundValidator(t *testing.T) {
	validator := &recordingManagedEscrowReceiptValidator{}
	node := &MobazhaNode{walletFields: walletFields{managedEscrowReceiptValidator: validator}}
	row := models.SettlementAction{
		ActionID: "action-1", OrderID: "order-1", ActionKind: payment.ManagedEscrowGuestSettlementAction,
		ChainID: 56, To: "0x1111111111111111111111111111111111111111",
	}
	txHash := common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful}
	require.NoError(t, node.validateManagedEscrowReceipt(context.Background(), row, txHash, receipt))
	require.Equal(t, row.ActionID, validator.request.ActionID)
	require.Equal(t, row.OrderID, validator.request.OrderID)
	require.Equal(t, row.To, validator.request.EscrowAddress)
	require.Equal(t, txHash.Hex(), validator.request.TxHash)
	require.Same(t, receipt, validator.request.Receipt)

	validator.err = errors.New("provider receipt rejected")
	require.EqualError(t, node.validateManagedEscrowReceipt(context.Background(), row, txHash, receipt), "provider receipt rejected")
	require.EqualError(t, (&MobazhaNode{}).validateManagedEscrowReceipt(context.Background(), row, txHash, receipt), "managed escrow receipt validator is unavailable")
}

func TestResubmitDroppedManagedRelayAction_UpdatesHashAndAttempt(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	newHash := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	relayer := &managedRelayRetryRelayer{hash: common.HexToHash(newHash)}
	node := &MobazhaNode{
		identityFields: identityFields{nodeID: "test-node"},
		storageFields:  storageFields{db: db},
		walletFields:   walletFields{evmRelay: relayer},
	}
	row := models.SettlementAction{
		ActionID:    "act-retry",
		OrderID:     "order-1",
		ActionKind:  "confirm",
		ChainID:     56,
		To:          "0x1111111111111111111111111111111111111111",
		Data:        "0xdeadbeef",
		State:       "submitted",
		TxHash:      "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RelayTaskID: "task-original",
		Attempts:    1,
		CreatedAt:   time.Now().Add(-managedRelayConfirmationTimeout).UTC(),
		UpdatedAt:   time.Now().Add(-managedRelayConfirmationTimeout).UTC(),
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&row)
	}))

	retried, reason := node.resubmitDroppedManagedRelayAction(context.Background(), row)
	require.True(t, retried)
	require.Empty(t, reason)
	require.NotNil(t, relayer.call)
	require.Equal(t, "bsc", relayer.call.ChainType)
	require.Equal(t, common.HexToAddress(row.To).Hex(), relayer.call.To)
	require.Equal(t, "0xdeadbeef", relayer.call.Data)
	require.Equal(t, row.OrderID, relayer.call.OrderID)
	require.Equal(t, row.ActionID, relayer.call.ClientActionID)

	var got models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", row.ActionID).First(&got).Error
	}))
	require.Equal(t, "submitted", got.State)
	require.Equal(t, newHash, got.TxHash)
	require.Equal(t, "task-original", got.RelayTaskID)
	require.Equal(t, 2, got.Attempts)
	require.Contains(t, got.AttemptTxHashes, row.TxHash)
	require.Contains(t, got.AttemptTxHashes, newHash)
	require.Empty(t, got.LastError)
}

func TestResubmitDroppedManagedRelayAction_SubmitErrorDefersRetry(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	relayer := &managedRelayRetryRelayer{err: errors.New("relay temporarily unavailable")}
	node := &MobazhaNode{
		identityFields: identityFields{nodeID: "test-node"},
		storageFields:  storageFields{db: db},
		walletFields:   walletFields{evmRelay: relayer},
	}
	row := models.SettlementAction{
		ActionID:   "act-retry-error",
		OrderID:    "order-1",
		ActionKind: "confirm",
		ChainID:    56,
		To:         "0x1111111111111111111111111111111111111111",
		Data:       "0xdeadbeef",
		State:      "submitted",
		TxHash:     "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Attempts:   1,
		CreatedAt:  time.Now().Add(-managedRelayConfirmationTimeout).UTC(),
		UpdatedAt:  time.Now().Add(-managedRelayConfirmationTimeout).UTC(),
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&row)
	}))

	retried, reason := node.resubmitDroppedManagedRelayAction(context.Background(), row)
	require.True(t, retried)
	require.Empty(t, reason)

	var got models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", row.ActionID).First(&got).Error
	}))
	require.Equal(t, "submitted", got.State)
	require.Equal(t, row.TxHash, got.TxHash)
	require.Equal(t, 2, got.Attempts)
	require.Contains(t, got.AttemptTxHashes, row.TxHash)
	require.Equal(t, "confirmation wait timed out; relay retry failed", got.LastError)
}

func TestReconcileManagedRelayAttempts_PrefersSuccessfulPreviousHash(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, dbgorm.MigrateSettlementActionModels(db))

	oldHash := common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	newHash := common.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	row := models.SettlementAction{
		ActionID:        "act-late-original",
		OrderID:         "order-1",
		ActionKind:      "confirm",
		ChainID:         56,
		State:           "submitted",
		TxHash:          newHash.Hex(),
		AttemptTxHashes: oldHash.Hex() + "\n" + newHash.Hex(),
		Attempts:        2,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&row)
	}))
	client := managedRelayConfirmationClient{
		head: 12,
		receipts: map[common.Hash]*types.Receipt{
			oldHash: {Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(11)},
			newHash: {Status: types.ReceiptStatusFailed, BlockNumber: big.NewInt(12)},
		},
	}
	node := &MobazhaNode{
		identityFields: identityFields{nodeID: "test-node"},
		storageFields:  storageFields{db: db},
	}

	result, ok := node.reconcileManagedRelayAttempts(context.Background(), client, row)
	require.True(t, ok)
	require.Equal(t, oldHash, result.hash)
	node.applyManagedRelayReceipt(context.Background(), client, row, result.hash, result.receipt)

	var got models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", row.ActionID).First(&got).Error
	}))
	require.Equal(t, "confirmed", got.State)
	require.Equal(t, oldHash.Hex(), got.TxHash)
	require.Equal(t, 2, got.Confirmations)
}
