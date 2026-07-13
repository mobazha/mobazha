// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/wire"
	gosolana "github.com/gagliardetto/solana-go"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedWalletSolanaKeyProvider struct {
	contracts.KeyProvider
	key gosolana.PrivateKey
}

func (p fixedWalletSolanaKeyProvider) SolanaMasterKey() (*gosolana.PrivateKey, error) {
	key := p.key
	return &key, nil
}

func newWalletAccountServiceForTest(t *testing.T, db database.Database) contracts.WalletAccountService {
	t.Helper()
	masterKey := testMasterKey(t)
	return NewWalletAccountService(db, masterKey, testMultiwallet(t, masterKey))
}

func migrateWalletAccountModels(t *testing.T, db database.Database) {
	t.Helper()
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.WalletAddressCursor{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.WalletAddressReservation{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.WalletTransfer{}); err != nil {
			return err
		}
		return tx.Migrate(&models.WalletTransferInput{})
	}))
}

func TestWalletAccountService_ReserveAddress_SolanaUsesTenantKey(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	migrateWalletAccountModels(t, db)
	key := gosolana.NewWallet().PrivateKey
	masterKey := testMasterKey(t)
	service := NewWalletAccountService(
		db, masterKey, testMultiwallet(t, masterKey), fixedWalletSolanaKeyProvider{key: key},
	)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainSolana)
	require.True(t, ok)

	capabilities, err := service.Capabilities(t.Context(), string(rail))
	require.NoError(t, err)
	require.True(t, capabilities.Receive)
	first, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "solana-payout-1")
	require.NoError(t, err)
	require.Equal(t, key.PublicKey().String(), first.Address)
	retry, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "solana-payout-1")
	require.NoError(t, err)
	require.Equal(t, first, retry)
}

type walletTransferChainOps struct {
	mu              sync.Mutex
	outputs         []utxo.UnspentOutput
	confirmations   int
	broadcastErr    error
	transactionErr  error
	known           bool
	broadcasts      int
	rawTxs          []string
	feeEstimateHook func()
}

func (o *walletTransferChainOps) GetFeeEstimate(iwallet.ChainType, int) uint64 {
	o.mu.Lock()
	hook := o.feeEstimateHook
	o.feeEstimateHook = nil
	o.mu.Unlock()
	if hook != nil {
		hook()
	}
	return 2
}
func (*walletTransferChainOps) IsHealthy(iwallet.ChainType) bool { return true }
func (*walletTransferChainOps) GetAddressTransactions(iwallet.ChainType, string, []byte) ([]iwallet.Transaction, error) {
	return nil, nil
}
func (o *walletTransferChainOps) ListUnspent(iwallet.ChainType, []byte) ([]utxo.UnspentOutput, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]utxo.UnspentOutput(nil), o.outputs...), nil
}
func (o *walletTransferChainOps) BroadcastTransaction(_ iwallet.ChainType, rawHex string) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.broadcasts++
	o.rawTxs = append(o.rawTxs, rawHex)
	if o.broadcastErr != nil {
		return "", o.broadcastErr
	}
	raw, err := hex.DecodeString(rawHex)
	if err != nil {
		return "", err
	}
	var tx wire.MsgTx
	if err := tx.Deserialize(bytes.NewReader(raw)); err != nil {
		return "", err
	}
	o.known = true
	return tx.TxHash().String(), nil
}
func (o *walletTransferChainOps) GetTransaction(iwallet.ChainType, string) (*iwallet.Transaction, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.transactionErr != nil {
		return nil, o.transactionErr
	}
	if !o.known {
		return nil, utxo.ErrTransactionNotFound
	}
	return &iwallet.Transaction{}, nil
}
func (o *walletTransferChainOps) GetTxConfirmations(iwallet.ChainType, string) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.confirmations, nil
}

func TestWalletAccountService_ReserveAddress_IsIdempotentAndRoleSeparated(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)

	service := newWalletAccountServiceForTest(t, db)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)

	guest, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "guest-order-1")
	require.NoError(t, err)
	guestRetry, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "guest-order-1")
	require.NoError(t, err)
	main, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "main-default")
	require.NoError(t, err)
	affiliate, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountAffiliate, "default")
	require.NoError(t, err)

	assert.Equal(t, guest, guestRetry)
	assert.Equal(t, uint32(0), guest.Index)
	assert.Equal(t, uint32(0), main.Index)
	assert.Equal(t, uint32(0), affiliate.Index)
	assert.NotEqual(t, guest.Address, main.Address)
	assert.NotEqual(t, guest.Address, affiliate.Address)
	assert.NotEqual(t, main.Address, affiliate.Address)
	assert.Equal(t, uint32(1), guest.Version)

	var reservations []models.WalletAddressReservation
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Find(&reservations).Error
	}))
	assert.Len(t, reservations, 3)
}

func TestWalletAccountService_ReserveAddress_Ethereum(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)

	service := newWalletAccountServiceForTest(t, db)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainEthereum)
	require.True(t, ok)

	destination, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountAffiliate, "default")
	require.NoError(t, err)
	assert.Equal(t, string(rail), destination.RailID)
	assert.Regexp(t, `^0x[0-9a-fA-F]{40}$`, destination.Address)
}

func TestWalletAccountService_Capabilities_GuestFailsClosedUntilTransferExists(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	service := newWalletAccountServiceForTest(t, db)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)

	capabilities, err := service.Capabilities(t.Context(), string(rail))
	require.NoError(t, err)
	assert.True(t, capabilities.Receive)
	assert.True(t, capabilities.Watch)
	assert.True(t, capabilities.Affiliate)
	assert.False(t, capabilities.Spend)
	assert.False(t, capabilities.AutoTransfer)
	assert.False(t, capabilities.Guest)
}

func TestWalletAccountService_ReserveAddress_RestoresFromPersistedState(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainLitecoin)
	require.True(t, ok)

	firstService := newWalletAccountServiceForTest(t, db)
	first, err := firstService.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "guest-order-restore")
	require.NoError(t, err)

	restartedService := newWalletAccountServiceForTest(t, db)
	restored, err := restartedService.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "guest-order-restore")
	require.NoError(t, err)
	assert.Equal(t, first, restored)
}

func TestWalletAccountService_ReserveAddress_RejectsReferenceScopeConflict(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)

	service := newWalletAccountServiceForTest(t, db)
	bitcoinRail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	bitcoinCashRail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoinCash)
	require.True(t, ok)
	const referenceID = "guest-order-1"

	_, err = service.ReserveAddress(t.Context(), string(bitcoinRail), contracts.AccountGuest, referenceID)
	require.NoError(t, err)

	_, err = service.ReserveAddress(t.Context(), string(bitcoinCashRail), contracts.AccountGuest, referenceID)
	require.ErrorContains(t, err, "already bound")
	_, err = service.ReserveAddress(t.Context(), string(bitcoinRail), contracts.AccountMain, referenceID)
	require.ErrorContains(t, err, "already bound")
}

func TestWalletAccountService_ReserveAddress_IsTenantScoped(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)
	router, ok := db.(interface {
		ForTenant(string) (database.Database, error)
	})
	require.True(t, ok)
	tenantADB, err := router.ForTenant("tenant-a")
	require.NoError(t, err)
	tenantBDB, err := router.ForTenant("tenant-b")
	require.NoError(t, err)
	serviceA := newWalletAccountServiceForTest(t, tenantADB)
	serviceB := newWalletAccountServiceForTest(t, tenantBDB)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)

	_, err = serviceA.ReserveAddress(config.ContextWithTenantID(t.Context(), "tenant-a"), string(rail), contracts.AccountGuest, "order-42")
	require.NoError(t, err)
	second, err := serviceB.ReserveAddress(config.ContextWithTenantID(t.Context(), "tenant-b"), string(rail), contracts.AccountGuest, "order-42")
	require.NoError(t, err)
	retry, err := serviceB.ReserveAddress(config.ContextWithTenantID(t.Context(), "tenant-b"), string(rail), contracts.AccountGuest, "order-42")
	require.NoError(t, err)
	assert.Equal(t, second, retry)

	var tenantAReservations []models.WalletAddressReservation
	require.NoError(t, tenantADB.View(func(tx database.Tx) error {
		return tx.Read().Where("reference_id = ?", "order-42").Find(&tenantAReservations).Error
	}))
	var tenantBReservations []models.WalletAddressReservation
	require.NoError(t, tenantBDB.View(func(tx database.Tx) error {
		return tx.Read().Where("reference_id = ?", "order-42").Find(&tenantBReservations).Error
	}))
	require.Len(t, tenantAReservations, 1)
	require.Len(t, tenantBReservations, 1)
	assert.Equal(t, "tenant-a", tenantAReservations[0].TenantID)
	assert.Equal(t, "tenant-b", tenantBReservations[0].TenantID)
}

func TestWalletAccountService_ReserveAddress_ConcurrentReferencesDoNotReuseIndexes(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)

	service := newWalletAccountServiceForTest(t, db)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoinCash)
	require.True(t, ok)

	const requests = 12
	results := make(chan contracts.ReservedDestination, requests)
	errs := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reservation, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, fmt.Sprintf("guest-order-%d", i))
			if err != nil {
				errs <- err
				return
			}
			results <- reservation
		}(i)
	}
	wg.Wait()
	close(errs)
	close(results)
	for err := range errs {
		require.NoError(t, err)
	}

	indexes := make(map[uint32]struct{}, requests)
	addresses := make(map[string]struct{}, requests)
	for reservation := range results {
		indexes[reservation.Index] = struct{}{}
		addresses[reservation.Address] = struct{}{}
	}
	assert.Len(t, indexes, requests)
	assert.Len(t, addresses, requests)
}

func TestWalletAccountService_Transfer_IdempotentRestartConfirmationAndReorg(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)
	masterKey := testMasterKey(t)
	multiwallet := testMultiwallet(t, masterKey)
	service := NewWalletAccountService(db, masterKey, multiwallet).(*walletAccountService)
	chainOps := &walletTransferChainOps{outputs: []utxo.UnspentOutput{{
		TxHash: fmt.Sprintf("%064x", 1), OutputIndex: 0, Value: 1_000_000,
	}}}
	service.SetChainOperations(chainOps)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	_, err = service.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "guest-transfer-source")
	require.NoError(t, err)
	destination, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "main-transfer-destination")
	require.NoError(t, err)

	request := contracts.WalletTransferRequest{
		RailID: string(rail), Role: contracts.AccountGuest, ReferenceID: "guest-transfer-source",
		Destination: destination.Address, Amount: 100_000, IdempotencyKey: "guest-transfer-1",
	}
	first, err := service.Transfer(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, contracts.WalletTransferSubmitted, first.State)
	assert.NotEmpty(t, first.TxHash)
	assert.Equal(t, 1, chainOps.broadcasts)

	retry, err := service.Transfer(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, first.TxHash, retry.TxHash)
	assert.Equal(t, 1, chainOps.broadcasts, "submitted idempotent retry must not rebroadcast")

	var persisted models.WalletTransfer
	var inputs []models.WalletTransferInput
	require.NoError(t, db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("idempotency_key = ?", request.IdempotencyKey).First(&persisted).Error; err != nil {
			return err
		}
		return tx.Read().Where("transfer_id = ?", persisted.ID).Find(&inputs).Error
	}))
	assert.NotEmpty(t, persisted.RawTxHex)
	assert.Len(t, inputs, 1)
	assert.Equal(t, 2, persisted.FeeTargetBlocks)
	assert.Equal(t, uint64(2), persisted.FeePerByte)
	assert.Equal(t, 1, persisted.BuildAttempts)
	assert.Equal(t, 1, persisted.BroadcastAttempts)

	restarted := NewWalletAccountService(db, masterKey, multiwallet).(*walletAccountService)
	restarted.SetChainOperations(chainOps)
	chainOps.confirmations = 2
	require.NoError(t, restarted.ReconcileTransfers(t.Context()))
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("idempotency_key = ?", request.IdempotencyKey).First(&persisted).Error
	}))
	assert.Equal(t, string(contracts.WalletTransferConfirmed), persisted.State)
	assert.Equal(t, 2, persisted.Confirmations)

	chainOps.confirmations = 0
	require.NoError(t, restarted.ReconcileTransfers(t.Context()))
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("idempotency_key = ?", request.IdempotencyKey).First(&persisted).Error
	}))
	assert.Equal(t, string(contracts.WalletTransferReorged), persisted.State)
	firstTxHash := persisted.TxHash

	// A transient lookup failure is not proof of eviction. It must preserve
	// the signed bytes, hash, and input reservation so a temporary source
	// outage cannot cause a duplicate payout.
	chainOps.mu.Lock()
	chainOps.transactionErr = errors.New("temporary transaction lookup outage")
	chainOps.mu.Unlock()
	for i := 0; i < models.MaxWalletTransferRetries+1; i++ {
		require.ErrorContains(t, restarted.ReconcileTransfers(t.Context()), "temporary transaction lookup outage")
	}
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("idempotency_key = ?", request.IdempotencyKey).First(&persisted).Error
	}))
	assert.Equal(t, string(contracts.WalletTransferReorged), persisted.State)
	assert.Equal(t, firstTxHash, persisted.TxHash)
	assert.NotEmpty(t, persisted.RawTxHex)
	assert.Zero(t, persisted.RetryCount, "chain observation outages must not exhaust build/broadcast retries")

	// The reorged transaction is now permanently evicted (e.g. one of its
	// inputs was double-spent elsewhere), not merely unconfirmed. Polling the
	// same dead TxHash must not leave the transfer stuck forever: the next
	// reconciliation should release its reserved inputs and return it to
	// Pending so a later reconciliation rebuilds and rebroadcasts.
	chainOps.mu.Lock()
	chainOps.transactionErr = nil
	chainOps.known = false
	chainOps.mu.Unlock()
	require.NoError(t, restarted.ReconcileTransfers(t.Context()))
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("idempotency_key = ?", request.IdempotencyKey).First(&persisted).Error
	}))
	assert.Equal(t, string(contracts.WalletTransferPending), persisted.State)
	assert.Empty(t, persisted.RawTxHex)
	assert.Empty(t, persisted.TxHash)
	assert.Equal(t, []string{firstTxHash}, persisted.AttemptTxHashes)
	require.NoError(t, db.View(func(tx database.Tx) error {
		var remaining []models.WalletTransferInput
		if err := tx.Read().Where("transfer_id = ?", persisted.ID).Find(&remaining).Error; err != nil {
			return err
		}
		assert.Empty(t, remaining, "evicted transfer must release its reserved inputs")
		return nil
	}))

	chainOps.mu.Lock()
	chainOps.outputs = []utxo.UnspentOutput{{
		TxHash: fmt.Sprintf("%064x", 11), OutputIndex: 1, Value: 1_000_000,
	}}
	chainOps.mu.Unlock()
	require.NoError(t, restarted.ReconcileTransfers(t.Context()))
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("idempotency_key = ?", request.IdempotencyKey).First(&persisted).Error
	}))
	assert.Equal(t, string(contracts.WalletTransferSubmitted), persisted.State)
	assert.NotEmpty(t, persisted.TxHash)
	assert.NotEqual(t, firstTxHash, persisted.TxHash)
	assert.Equal(t, []string{firstTxHash, persisted.TxHash}, persisted.AttemptTxHashes)
	assert.Equal(t, 2, persisted.BuildAttempts)
	assert.Equal(t, 2, persisted.BroadcastAttempts)
	assert.Equal(t, 2, chainOps.broadcasts, "rebuild after eviction must rebroadcast")
}

func TestWalletAccountService_Transfer_RecoversPersistedBuildAfterBroadcastFailure(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)
	masterKey := testMasterKey(t)
	multiwallet := testMultiwallet(t, masterKey)
	service := NewWalletAccountService(db, masterKey, multiwallet).(*walletAccountService)
	chainOps := &walletTransferChainOps{
		outputs:      []utxo.UnspentOutput{{TxHash: fmt.Sprintf("%064x", 2), OutputIndex: 1, Value: 900_000}},
		broadcastErr: errors.New("temporary broadcast failure"),
	}
	service.SetChainOperations(chainOps)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	_, err = service.ReserveAddress(t.Context(), string(rail), contracts.AccountAffiliate, "default")
	require.NoError(t, err)
	destination, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "main")
	require.NoError(t, err)
	request := contracts.WalletTransferRequest{
		RailID: string(rail), Role: contracts.AccountAffiliate, ReferenceID: "default",
		Destination: destination.Address, SweepAll: true, IdempotencyKey: "affiliate-transfer-recovery",
	}

	failed, err := service.Transfer(t.Context(), request)
	require.Error(t, err)
	assert.Equal(t, contracts.WalletTransferBuilt, failed.State)
	require.Len(t, chainOps.rawTxs, 1)
	firstRaw := chainOps.rawTxs[0]

	chainOps.broadcastErr = nil
	restarted := NewWalletAccountService(db, masterKey, multiwallet).(*walletAccountService)
	restarted.SetChainOperations(chainOps)
	require.NoError(t, restarted.ReconcileTransfers(t.Context()))
	require.Len(t, chainOps.rawTxs, 2)
	assert.Equal(t, firstRaw, chainOps.rawTxs[1], "recovery must rebroadcast persisted bytes")

	result, err := restarted.Transfer(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, contracts.WalletTransferSubmitted, result.State)
	assert.Equal(t, 2, chainOps.broadcasts)
}

func TestWalletAccountService_Transfer_RejectsIdempotencyMutationAndReservedInputReuse(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)
	masterKey := testMasterKey(t)
	multiwallet := testMultiwallet(t, masterKey)
	service := NewWalletAccountService(db, masterKey, multiwallet).(*walletAccountService)
	chainOps := &walletTransferChainOps{outputs: []utxo.UnspentOutput{{
		TxHash: fmt.Sprintf("%064x", 3), OutputIndex: 0, Value: 1_000_000,
	}}}
	service.SetChainOperations(chainOps)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	_, err = service.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "source")
	require.NoError(t, err)
	destination, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "destination")
	require.NoError(t, err)
	request := contracts.WalletTransferRequest{
		RailID: string(rail), Role: contracts.AccountGuest, ReferenceID: "source",
		Destination: destination.Address, Amount: 100_000, IdempotencyKey: "transfer-key",
	}
	_, err = service.Transfer(t.Context(), request)
	require.NoError(t, err)

	mutated := request
	mutated.Amount++
	_, err = service.Transfer(t.Context(), mutated)
	require.ErrorContains(t, err, "different request")

	second := request
	second.IdempotencyKey = "transfer-key-2"
	_, err = service.Transfer(t.Context(), second)
	require.ErrorContains(t, err, "no unreserved outputs")
}

func TestWalletAccountService_Transfer_DoesNotPersistBuiltStateWithoutInputReservation(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)
	masterKey := testMasterKey(t)
	service := NewWalletAccountService(db, masterKey, testMultiwallet(t, masterKey)).(*walletAccountService)
	outpoint := utxo.UnspentOutput{TxHash: fmt.Sprintf("%064x", 4), OutputIndex: 0, Value: 1_000_000}
	chainOps := &walletTransferChainOps{outputs: []utxo.UnspentOutput{outpoint}}
	service.SetChainOperations(chainOps)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	_, err = service.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "atomic-source")
	require.NoError(t, err)
	destination, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "atomic-destination")
	require.NoError(t, err)

	chainOps.feeEstimateHook = func() {
		require.NoError(t, db.Update(func(tx database.Tx) error {
			return tx.Create(&models.WalletTransferInput{
				TransferID: "competing-transfer", RailID: string(rail), TxHash: outpoint.TxHash,
				OutputIndex: outpoint.OutputIndex, Value: outpoint.Value,
			})
		}))
	}
	request := contracts.WalletTransferRequest{
		RailID: string(rail), Role: contracts.AccountGuest, ReferenceID: "atomic-source",
		Destination: destination.Address, Amount: 100_000, IdempotencyKey: "atomic-reservation",
	}
	_, err = service.Transfer(t.Context(), request)
	require.ErrorContains(t, err, "persist built wallet transfer")

	var persisted models.WalletTransfer
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("idempotency_key = ?", request.IdempotencyKey).First(&persisted).Error
	}))
	assert.Equal(t, string(contracts.WalletTransferPending), persisted.State)
	assert.Empty(t, persisted.RawTxHex)
	assert.Empty(t, persisted.TxHash)
	assert.Equal(t, 0, chainOps.broadcasts)
}

func TestWalletAccountService_Capabilities_OpenGuestOnlyWithTransferRuntime(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	masterKey := testMasterKey(t)
	service := NewWalletAccountService(db, masterKey, testMultiwallet(t, masterKey)).(*walletAccountService)
	service.SetChainOperations(&walletTransferChainOps{})
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	capabilities, err := service.Capabilities(t.Context(), string(rail))
	require.NoError(t, err)
	assert.True(t, capabilities.Spend)
	assert.True(t, capabilities.AutoTransfer)
	assert.True(t, capabilities.Guest)
}

func TestWalletAccountService_Transfer_AllSupportedUTXORails(t *testing.T) {
	for i, chain := range []iwallet.ChainType{
		iwallet.ChainBitcoin, iwallet.ChainBitcoinCash, iwallet.ChainLitecoin,
	} {
		t.Run(string(chain), func(t *testing.T) {
			db, err := repo.MockDB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			migrateWalletAccountModels(t, db)
			masterKey := testMasterKey(t)
			service := NewWalletAccountService(db, masterKey, testMultiwallet(t, masterKey)).(*walletAccountService)
			chainOps := &walletTransferChainOps{outputs: []utxo.UnspentOutput{{
				TxHash: fmt.Sprintf("%064x", i+100), OutputIndex: 0, Value: 1_000_000,
			}}}
			service.SetChainOperations(chainOps)
			rail, ok := iwallet.CanonicalNativeCoinType(chain)
			require.True(t, ok)
			_, err = service.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "source")
			require.NoError(t, err)
			destination, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "destination")
			require.NoError(t, err)

			transfer, err := service.Transfer(t.Context(), contracts.WalletTransferRequest{
				RailID: string(rail), Role: contracts.AccountGuest, ReferenceID: "source",
				Destination: destination.Address, Amount: 100_000, IdempotencyKey: "all-rails-" + string(chain),
			})
			require.NoError(t, err)
			assert.Equal(t, contracts.WalletTransferSubmitted, transfer.State)
			assert.NotEmpty(t, transfer.TxHash)
			assert.Equal(t, 1, chainOps.broadcasts)
		})
	}
}

func TestWalletAccountService_ReserveAddress_NetworkIsolation(t *testing.T) {
	masterKey := testMasterKey(t)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)

	mainDB, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mainDB.Close()) })
	migrateWalletAccountModels(t, mainDB)
	mainService := NewWalletAccountService(mainDB, masterKey, testMultiwallet(t, masterKey))
	mainDestination, err := mainService.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "network-vector")
	require.NoError(t, err)

	testDB, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testDB.Close()) })
	migrateWalletAccountModels(t, testDB)
	mw, err := loadTestMultiwallet(masterKey, &repo.Config{LogLevel: "error"}, nil, true, t.TempDir())
	require.NoError(t, err)
	testService := NewWalletAccountService(testDB, masterKey, &mw)
	testDestination, err := testService.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "network-vector")
	require.NoError(t, err)

	assert.NotEqual(t, mainDestination.Address, testDestination.Address)
	assert.Contains(t, mainDestination.Address, "bc1")
	assert.Contains(t, testDestination.Address, "tb1")
}

func TestWalletAccountService_Transfer_EnforcesFiniteRetryLimit(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	migrateWalletAccountModels(t, db)
	masterKey := testMasterKey(t)
	service := NewWalletAccountService(db, masterKey, testMultiwallet(t, masterKey)).(*walletAccountService)
	chainOps := &walletTransferChainOps{
		outputs:      []utxo.UnspentOutput{{TxHash: fmt.Sprintf("%064x", 999), OutputIndex: 0, Value: 1_000_000}},
		broadcastErr: errors.New("broadcast unavailable"),
	}
	service.SetChainOperations(chainOps)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	_, err = service.ReserveAddress(t.Context(), string(rail), contracts.AccountGuest, "retry-source")
	require.NoError(t, err)
	destination, err := service.ReserveAddress(t.Context(), string(rail), contracts.AccountMain, "retry-destination")
	require.NoError(t, err)
	request := contracts.WalletTransferRequest{
		RailID: string(rail), Role: contracts.AccountGuest, ReferenceID: "retry-source",
		Destination: destination.Address, SweepAll: true, IdempotencyKey: "finite-retry",
	}
	for i := 0; i < models.MaxWalletTransferRetries; i++ {
		_, err = service.Transfer(t.Context(), request)
		require.ErrorContains(t, err, "broadcast wallet transfer")
	}
	assert.Equal(t, models.MaxWalletTransferRetries, chainOps.broadcasts)
	_, err = service.Transfer(t.Context(), request)
	require.ErrorContains(t, err, "automatic retry limit reached")
	assert.Equal(t, models.MaxWalletTransferRetries, chainOps.broadcasts)
}
