package guest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/models"
	pkgutxo "github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// sweepTestDB is a test DB with composite-PK aware Save to mirror production.
type sweepTestDB struct {
	gormDB *gorm.DB
}

func (d *sweepTestDB) View(fn func(database.Tx) error) error {
	return fn(&sweepTestTx{testTx: testTx{db: d.gormDB}})
}

func (d *sweepTestDB) Update(fn func(database.Tx) error) error {
	return d.gormDB.Transaction(func(tx *gorm.DB) error {
		return fn(&sweepTestTx{testTx: testTx{db: tx}})
	})
}

func (d *sweepTestDB) ComputePublicDataHash() (cid.Cid, error) { return cid.Undef, nil }
func (d *sweepTestDB) Close() error                            { return nil }

type sweepTestTx struct {
	testTx
}

func (t *sweepTestTx) Save(i interface{}) error {
	stmt := &gorm.Statement{DB: t.db}
	if err := stmt.Parse(i); err != nil {
		return t.db.Save(i).Error
	}
	if len(stmt.Schema.PrimaryFields) < 2 {
		return t.db.Save(i).Error
	}

	v := reflect.ValueOf(i)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		for _, f := range stmt.Schema.PrimaryFields {
			if f.DBName == "tenant_id" {
				continue
			}
			fv := v.FieldByName(f.Name)
			if !fv.IsValid() || fv.Kind() != reflect.Int {
				continue
			}
			if fv.Int() != 0 {
				continue
			}
			var maxID int
			t.db.Model(i).Select(fmt.Sprintf("COALESCE(MAX(%s), 0)", f.DBName)).Scan(&maxID)
			fv.SetInt(int64(maxID + 1))
		}
	}

	var cols []clause.Column
	for _, f := range stmt.Schema.PrimaryFields {
		cols = append(cols, clause.Column{Name: f.DBName})
	}
	return t.db.Clauses(clause.OnConflict{Columns: cols, UpdateAll: true}).Create(i).Error
}
func (t *sweepTestTx) Create(i interface{}) error { return t.db.Create(i).Error }

func seedSweepGuestOrder(t *testing.T, db *sweepTestDB, id int, order models.GuestOrder) {
	t.Helper()
	order.ID = id
	order.TenantID = testTenantID
	if order.ExpiresAt.IsZero() {
		order.ExpiresAt = time.Now().Add(time.Hour)
	}
	require.NoError(t, db.gormDB.Create(&order).Error)
}

func loadSweepGuestOrder(t *testing.T, db *sweepTestDB, token string) models.GuestOrder {
	t.Helper()
	var o models.GuestOrder
	require.NoError(t, db.gormDB.Where("order_token = ?", token).First(&o).Error)
	return o
}

func newSweepTestDB(t *testing.T) *sweepTestDB {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SweepTask{}, &models.SettlementAction{}))
	return &sweepTestDB{gormDB: db}
}

func newSweepSvc(db database.Database) *AutoSweepService {
	return NewAutoSweepService(db, nil, nil)
}

// ── CreateSweepTask ────────────────────────────────────────────────

func TestCreateSweepTask_InsertsPendingRow(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	order := &models.GuestOrder{
		OrderToken:     "gst_create_1",
		PaymentCoin:    "BTC",
		PaymentAddress: "bc1q_from",
		SweepToAddress: "bc1q_to",
		PaymentAmount:  "100000",
		AddressIndex:   7,
	}

	err := db.Update(func(tx database.Tx) error {
		return svc.CreateSweepTask(tx, order)
	})
	require.NoError(t, err)

	var tasks []models.SweepTask
	require.NoError(t, db.gormDB.Find(&tasks).Error)
	require.Len(t, tasks, 1)
	assert.Equal(t, "gst_create_1", tasks[0].OrderToken)
	assert.Equal(t, "BTC", tasks[0].ChainKey)
	assert.Equal(t, uint32(7), tasks[0].AddressIndex)
	assert.Equal(t, models.SweepStatusPending, tasks[0].Status)
	assert.Equal(t, 0, tasks[0].RetryCount)
}

func TestCreateSweepTask_NormalizesCanonicalCoinType(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	order := &models.GuestOrder{
		OrderToken:     "gst_canonical",
		PaymentCoin:    "crypto:bip122:000000000019d6689c085ae165831e93:native",
		PaymentAddress: "bc1q_from",
		SweepToAddress: "bc1q_to",
		PaymentAmount:  "100000",
	}
	err := db.Update(func(tx database.Tx) error {
		return svc.CreateSweepTask(tx, order)
	})
	require.NoError(t, err)

	var tasks []models.SweepTask
	require.NoError(t, db.gormDB.Find(&tasks).Error)
	require.Len(t, tasks, 1)
	assert.Equal(t, "BTC", tasks[0].ChainKey,
		"canonical CoinType must be normalised to chain identifier")
}

func TestCreateSweepTask_UsesBIP44KeySource(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	err := db.Update(func(tx database.Tx) error {
		return svc.CreateSweepTask(tx, &models.GuestOrder{
			OrderToken:     "gst_key_source",
			PaymentCoin:    "BTC",
			PaymentAddress: "bc1q_from",
			SweepToAddress: "bc1q_to",
			PaymentAmount:  "100000",
		})
	})
	require.NoError(t, err)

	var task models.SweepTask
	require.NoError(t, db.gormDB.Where("order_token = ?", "gst_key_source").First(&task).Error)
	assert.Equal(t, models.SweepKeySourceBIP44, task.KeySource)
}

func TestCreateSweepTask_AffiliateFreezesSplitAction(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)
	err := db.Update(func(tx database.Tx) error {
		return svc.CreateSweepTask(tx, &models.GuestOrder{
			OrderToken: "gst_affiliate", PaymentCoin: "BTC", PaymentAddress: "from", SweepToAddress: "seller",
			PaymentAmount: "100000", AffiliatePayoutAddress: "promoter", AffiliatePayoutAmount: "5000",
		})
	})
	require.NoError(t, err)
	var task models.SweepTask
	require.NoError(t, db.gormDB.Where("order_token = ?", "gst_affiliate").First(&task).Error)
	assert.Equal(t, "promoter", task.AffiliatePayoutAddress)
	assert.Equal(t, "5000", task.AffiliatePayoutAmount)
	var action models.SettlementAction
	require.NoError(t, db.gormDB.Where("action_id = ?", guestSweepActionID("gst_affiliate")).First(&action).Error)
	assert.Equal(t, "submitting", action.State)
	wantCoin, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	assert.Equal(t, wantCoin.String(), action.SettlementCoin)
	lines := models.DecodeSettlementPayoutLines(action.PlannedLines)
	require.Len(t, lines, 1)
	assert.Equal(t, "affiliate", lines[0].Type)
	assert.Equal(t, wantCoin.String(), lines[0].Coin)
}

func TestCreateSweepTask_SolanaSkipsWhenNoSweepTo(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	order := &models.GuestOrder{
		OrderToken:     "gst_solana",
		PaymentCoin:    "SOL",
		PaymentAddress: "buyer_addr",
		SweepToAddress: "",
	}

	err := db.Update(func(tx database.Tx) error {
		return svc.CreateSweepTask(tx, order)
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.gormDB.Model(&models.SweepTask{}).Count(&count).Error)
	assert.Equal(t, int64(0), count,
		"Solana orders must not create sweep tasks")
}

func TestCreateSweepTask_MultipleOrdersGetUniqueIDs(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	for i := 0; i < 3; i++ {
		order := &models.GuestOrder{
			OrderToken:     fmt.Sprintf("gst_multi_%d", i),
			PaymentCoin:    "ETH",
			PaymentAddress: fmt.Sprintf("0xfrom_%d", i),
			SweepToAddress: "0xto",
			PaymentAmount:  "1000",
			AddressIndex:   uint32(i),
		}
		err := db.Update(func(tx database.Tx) error {
			return svc.CreateSweepTask(tx, order)
		})
		require.NoError(t, err)
	}

	var tasks []models.SweepTask
	require.NoError(t, db.gormDB.Order("id ASC").Find(&tasks).Error)
	require.Len(t, tasks, 3)
	assert.Equal(t, 1, tasks[0].ID)
	assert.Equal(t, 2, tasks[1].ID)
	assert.Equal(t, 3, tasks[2].ID)
}

// ── ProcessPendingSweeps ───────────────────────────────────────────

func TestProcessPendingSweeps_MarksPendingAsFailedWithRetry(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	pending := &models.SweepTask{
		ID:          1,
		OrderToken:  "gst_pending",
		ChainKey:    "BTC",
		FromAddress: "bc1q_a",
		ToAddress:   "bc1q_b",
		Amount:      "1000",
		Status:      models.SweepStatusPending,
	}
	pending.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(pending).Error)

	svc.ProcessPendingSweeps(context.Background())

	var got models.SweepTask
	require.NoError(t, db.gormDB.First(&got, "id = ?", 1).Error)
	assert.Equal(t, 1, got.RetryCount,
		"processSingleSweep must bump retry_count on failure")
	assert.Contains(t, got.LastError, "sweep infrastructure not initialised")
	assert.Equal(t, models.SweepStatusPending, got.Status)
}

func TestProcessPendingSweeps_SkipsMaxedOutTasks(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	maxed := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_maxed",
		ChainKey:   "BTC",
		Status:     models.SweepStatusPending,
		RetryCount: models.MaxSweepRetries,
	}
	maxed.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(maxed).Error)

	svc.ProcessPendingSweeps(context.Background())

	var got models.SweepTask
	require.NoError(t, db.gormDB.First(&got, "id = ?", 1).Error)
	assert.Equal(t, models.MaxSweepRetries, got.RetryCount,
		"tasks at MaxSweepRetries must not be retried further")
}

func TestProcessPendingSweeps_SkipsNonPendingStatus(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	confirmed := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_confirmed",
		ChainKey:   "BTC",
		Status:     models.SweepStatusConfirmed,
		RetryCount: 0,
		TxHash:     "0xabc",
	}
	confirmed.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(confirmed).Error)

	svc.ProcessPendingSweeps(context.Background())

	var got models.SweepTask
	require.NoError(t, db.gormDB.First(&got, "id = ?", 1).Error)
	assert.Equal(t, 0, got.RetryCount,
		"non-pending tasks must not be touched by ProcessPendingSweeps")
}

// ── ConfirmSweep ───────────────────────────────────────────────────

func TestConfirmSweep_TransitionsSubmittedToConfirmed(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	submitted := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_submit",
		ChainKey:   "BTC",
		Status:     models.SweepStatusSubmitted,
		TxHash:     "0xdeadbeef",
	}
	submitted.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(submitted).Error)

	require.NoError(t, svc.ConfirmSweep("gst_submit", "0xdeadbeef"))

	var got models.SweepTask
	require.NoError(t, db.gormDB.First(&got, "id = ?", 1).Error)
	assert.Equal(t, models.SweepStatusConfirmed, got.Status)
}

func TestConfirmSweep_AffiliateConfirmsObservedSettlementOutput(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)
	task := &models.SweepTask{
		ID: 1, OrderToken: "gst_affiliate_confirm", ChainKey: "BTC", Status: models.SweepStatusSubmitted,
		TxHash: "tx-split", AffiliatePayoutAddress: "promoter", AffiliatePayoutAmount: "5000",
	}
	task.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(task).Error)
	action := models.SettlementAction{
		ActionID: guestSweepActionID(task.OrderToken), OrderID: task.OrderToken, ActionKind: "guest_sweep",
		State: "submitted", TxHash: task.TxHash, SettlementCoin: "BTC",
		PlannedLines: models.EncodeSettlementPayoutLines([]models.SettlementPayoutLine{{Type: "affiliate", Amount: "5000", Address: "promoter", Coin: "BTC"}}),
	}
	action.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(&action).Error)
	observed := []models.SettlementPayoutLine{{Type: "affiliate", Amount: "5000", Address: "promoter", Coin: "BTC", TxHash: task.TxHash}}
	require.NoError(t, svc.ConfirmSweep(task.OrderToken, task.TxHash, observed))
	var got models.SettlementAction
	require.NoError(t, db.gormDB.Where("action_id = ?", action.ActionID).First(&got).Error)
	assert.Equal(t, "confirmed", got.State)
	assert.NotNil(t, got.ConfirmedAt)
	assert.Equal(t, observed, models.DecodeSettlementPayoutLines(got.ObservedLines))
}

func TestConfirmSweep_ErrorsWhenNoSubmittedTask(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	pending := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_nosubmit",
		ChainKey:   "BTC",
		Status:     models.SweepStatusPending,
	}
	pending.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(pending).Error)

	err := svc.ConfirmSweep("gst_nosubmit", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sweep task not found")
}

// ── ClaimSweepTask ─────────────────────────────────────────────────

func TestClaimSweepTask_SucceedsForPending(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	task := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_claim",
		ChainKey:   "LTC",
		Status:     models.SweepStatusPending,
	}
	task.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(task).Error)

	claimed, err := svc.ClaimSweepTask(1)
	require.NoError(t, err)
	assert.True(t, claimed)

	var got models.SweepTask
	require.NoError(t, db.gormDB.First(&got, "id = ?", 1).Error)
	assert.Equal(t, models.SweepStatusProcessing, got.Status)
}

func TestClaimSweepTask_FailsForNonPending(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	task := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_processing",
		ChainKey:   "LTC",
		Status:     models.SweepStatusProcessing,
	}
	task.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(task).Error)

	claimed, err := svc.ClaimSweepTask(1)
	require.NoError(t, err)
	assert.False(t, claimed, "should not claim a task that's already processing")
}

// ── RecoverStaleTasks ──────────────────────────────────────────────

func TestRecoverStaleTasks_TransitionsStaleProcessingToPending(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	stale := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_stale",
		ChainKey:   "LTC",
		Status:     models.SweepStatusProcessing,
	}
	stale.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(stale).Error)
	require.NoError(t, db.gormDB.Exec(
		"UPDATE sweep_tasks SET updated_at = datetime('now', '-15 minutes') WHERE id = 1",
	).Error)

	svc.RecoverStaleTasks()

	var got models.SweepTask
	require.NoError(t, db.gormDB.First(&got, "id = ?", 1).Error)
	assert.Equal(t, models.SweepStatusPending, got.Status,
		"stale processing task must be recovered to pending")
}

func TestRecoverStaleTasks_IgnoresRecentProcessing(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	fresh := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_fresh",
		ChainKey:   "LTC",
		Status:     models.SweepStatusProcessing,
	}
	fresh.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(fresh).Error)

	svc.RecoverStaleTasks()

	var got models.SweepTask
	require.NoError(t, db.gormDB.First(&got, "id = ?", 1).Error)
	assert.Equal(t, models.SweepStatusProcessing, got.Status,
		"recently-claimed task must not be recovered")
}

func TestRecoverStaleTasks_PromotesAffiliateActionAfterRecoveredBroadcast(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)
	task := &models.SweepTask{
		ID: 1, OrderToken: "gst_affiliate_stale", ChainKey: "LTC",
		AffiliatePayoutAddress: "promoter", AffiliatePayoutAmount: "5000",
		Status: models.SweepStatusProcessing,
	}
	task.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(task).Error)
	require.NoError(t, db.gormDB.Create(&models.SettlementAction{
		ActionID: guestSweepActionID(task.OrderToken), OrderID: task.OrderToken,
		ActionKind: "guest_sweep", State: "submitting", SettlementCoin: "LTC",
	}).Error)
	require.NoError(t, db.gormDB.Exec(
		"UPDATE sweep_tasks SET updated_at = datetime('now', '-15 minutes') WHERE id = 1",
	).Error)
	svc.recordBroadcast(task.ID, "recovered_tx")

	svc.RecoverStaleTasks()

	var action models.SettlementAction
	require.NoError(t, db.gormDB.First(&action, "action_id = ?", guestSweepActionID(task.OrderToken)).Error)
	assert.Equal(t, "submitted", action.State)
	assert.Equal(t, "recovered_tx", action.TxHash)
}

// ── ConfirmSweep with txHash mismatch ──────────────────────────────

func TestConfirmSweep_ErrorsOnTxHashMismatch(t *testing.T) {
	db := newSweepTestDB(t)
	svc := newSweepSvc(db)

	submitted := &models.SweepTask{
		ID:         1,
		OrderToken: "gst_mismatch",
		ChainKey:   "LTC",
		Status:     models.SweepStatusSubmitted,
		TxHash:     "actual_hash_abc",
	}
	submitted.TenantID = testTenantID
	require.NoError(t, db.gormDB.Create(submitted).Error)

	err := svc.ConfirmSweep("gst_mismatch", "wrong_hash")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sweep task not found")

	var got models.SweepTask
	require.NoError(t, db.gormDB.First(&got, "id = ?", 1).Error)
	assert.Equal(t, models.SweepStatusSubmitted, got.Status)
}

type affiliateSweepTestKeyProvider struct {
	escrow *btcec.PrivateKey
}

var _ contracts.KeyProvider = (*affiliateSweepTestKeyProvider)(nil)

func (p *affiliateSweepTestKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error) {
	return nil, errors.New("not used")
}
func (p *affiliateSweepTestKeyProvider) SolanaMasterKey() (*solana.PrivateKey, error) {
	return nil, errors.New("not used")
}
func (p *affiliateSweepTestKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error) {
	return p.escrow, nil
}
func (p *affiliateSweepTestKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error) {
	return nil, errors.New("not used")
}
func (p *affiliateSweepTestKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error) {
	return nil, errors.New("not used")
}
func (p *affiliateSweepTestKeyProvider) DigitalContentMasterKey(int) ([]byte, error) {
	return nil, errors.New("not used")
}

type affiliateSweepTestPayouts struct{}

var _ contracts.EscrowOperations = (*affiliateSweepTestPayouts)(nil)

func (p *affiliateSweepTestPayouts) GetPayoutAddress(coin string) (iwallet.Address, error) {
	return iwallet.NewAddress("seller-"+coin, iwallet.CoinType(coin)), nil
}
func (p *affiliateSweepTestPayouts) ReleaseCancelableFunds(*models.Order, string) (iwallet.TransactionID, string, error) {
	return "", "", errors.New("not used")
}
func (p *affiliateSweepTestPayouts) ReleaseFromCancelableAddressWithParams(*models.Order, contracts.ReleaseFromCancelableParams) (iwallet.Tx, *iwallet.Transaction, error) {
	return nil, nil, errors.New("not used")
}
func (p *affiliateSweepTestPayouts) RelayInstructions(string, iwallet.CoinType, any) (string, error) {
	return "", errors.New("not used")
}
func (p *affiliateSweepTestPayouts) CancelPartialPayment(string) (string, uint64, error) {
	return "", 0, errors.New("not used")
}

type affiliateSweepTestWallet struct {
	iwallet.Wallet
	chain       iwallet.ChainType
	signingKeys [][]byte
}

func (w *affiliateSweepTestWallet) DerivePaymentAddressFromPubKey(*btcec.PublicKey) (string, []byte, error) {
	return "affiliate-" + string(w.chain), []byte{byte(len(w.chain))}, nil
}
func (w *affiliateSweepTestWallet) AddressToScriptPubKey(string) ([]byte, error) {
	return []byte{byte(len(w.chain))}, nil
}
func (w *affiliateSweepTestWallet) BuildSweepTx(_ []iwallet.SweepInput, key btcec.PrivateKey, _ string, _ int64) ([]byte, string, error) {
	w.signingKeys = append(w.signingKeys, key.Serialize())
	return []byte{0x01, 0x02}, "built-" + string(w.chain), nil
}

type affiliateSweepTestWallets struct {
	wallets map[iwallet.ChainType]*affiliateSweepTestWallet
}

var _ contracts.WalletOperator = (*affiliateSweepTestWallets)(nil)

func (w *affiliateSweepTestWallets) WalletForCurrencyCode(string) (iwallet.Wallet, error) {
	return nil, errors.New("not used")
}
func (w *affiliateSweepTestWallets) SupportedChains() []iwallet.ChainType {
	return []iwallet.ChainType{iwallet.ChainBitcoin, iwallet.ChainBitcoinCash, iwallet.ChainLitecoin}
}
func (w *affiliateSweepTestWallets) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	wallet, ok := w.wallets[chain]
	return wallet, ok
}
func (w *affiliateSweepTestWallets) Start() error { return nil }
func (w *affiliateSweepTestWallets) Close() error { return nil }

type affiliateSweepTestChainOps struct {
	unspent map[iwallet.ChainType][]pkgutxo.UnspentOutput
}

func (o *affiliateSweepTestChainOps) GetTransaction(iwallet.ChainType, string) (*iwallet.Transaction, error) {
	return nil, errors.New("not used")
}
func (o *affiliateSweepTestChainOps) GetFeeEstimate(iwallet.ChainType, int) uint64 { return 2 }
func (o *affiliateSweepTestChainOps) BroadcastTransaction(chain iwallet.ChainType, _ string) (string, error) {
	return "affiliate-sweep-" + string(chain), nil
}
func (o *affiliateSweepTestChainOps) GetAddressTransactions(iwallet.ChainType, string, []byte) ([]iwallet.Transaction, error) {
	return nil, nil
}
func (o *affiliateSweepTestChainOps) IsHealthy(iwallet.ChainType) bool { return true }
func (o *affiliateSweepTestChainOps) ListUnspent(chain iwallet.ChainType, _ []byte) ([]pkgutxo.UnspentOutput, error) {
	return o.unspent[chain], nil
}
func (o *affiliateSweepTestChainOps) GetTxConfirmations(iwallet.ChainType, string) (int, error) {
	return 0, nil
}

func newAffiliateSweepTestWallets() *affiliateSweepTestWallets {
	wallets := make(map[iwallet.ChainType]*affiliateSweepTestWallet)
	for _, chain := range []iwallet.ChainType{iwallet.ChainBitcoin, iwallet.ChainBitcoinCash, iwallet.ChainLitecoin} {
		wallets[chain] = &affiliateSweepTestWallet{chain: chain}
	}
	return &affiliateSweepTestWallets{wallets: wallets}
}

func TestStartAffiliateUTXOSweeps_ConfirmedDepositCreatesEscrowKeyTask(t *testing.T) {
	db := newSweepTestDB(t)
	escrowKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	wallets := newAffiliateSweepTestWallets()
	ops := &affiliateSweepTestChainOps{unspent: map[iwallet.ChainType][]pkgutxo.UnspentOutput{}}
	svc := NewAutoSweepService(db, nil, nil)
	svc.SetChainOps(ops)
	svc.SetMultiwallet(wallets)
	svc.SetAffiliateSweepRuntime(&affiliateSweepTestKeyProvider{escrow: escrowKey}, &affiliateSweepTestPayouts{})
	monitor := pkgutxo.NewMonitor(nil)

	require.NoError(t, svc.StartAffiliateUTXOSweeps(context.Background(), "affiliate-node", monitor))
	ltcAddress := "affiliate-" + string(iwallet.ChainLitecoin)
	watch := monitor.GetWatchedAddress(ltcAddress)
	require.NotNil(t, watch)

	watch.OnPayment(&iwallet.Transaction{Height: 0}, pkgutxo.PaymentStatusNormal)
	var count int64
	require.NoError(t, db.gormDB.Model(&models.SweepTask{}).Count(&count).Error)
	assert.Zero(t, count)

	ops.unspent[iwallet.ChainLitecoin] = []pkgutxo.UnspentOutput{{
		TxHash: "affiliate-deposit", OutputIndex: 0, Height: 1, Value: 100_000,
	}}
	watch.OnPayment(&iwallet.Transaction{Height: 1}, pkgutxo.PaymentStatusNormal)
	require.NoError(t, db.gormDB.Model(&models.SweepTask{}).Count(&count).Error)
	require.Equal(t, int64(1), count)

	var task models.SweepTask
	require.NoError(t, db.gormDB.First(&task).Error)
	assert.Equal(t, models.SweepKeySourceAffiliateEscrow, task.KeySource)
	assert.Equal(t, "affiliate:LTC", task.OrderToken)
	ltcPayoutCoin, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainLitecoin)
	require.True(t, ok)
	assert.Equal(t, "seller-"+ltcPayoutCoin.String(), task.ToAddress)
	assert.Equal(t, "100000", task.Amount)

	svc.ProcessPendingSweeps(context.Background())
	require.Len(t, wallets.wallets[iwallet.ChainLitecoin].signingKeys, 1)
	assert.Equal(t, escrowKey.Serialize(), wallets.wallets[iwallet.ChainLitecoin].signingKeys[0])
	require.NoError(t, db.gormDB.First(&task).Error)
	assert.Equal(t, models.SweepStatusSubmitted, task.Status)
}
