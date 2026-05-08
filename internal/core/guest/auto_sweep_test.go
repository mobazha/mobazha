package guest

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha3.0/pkg/models"
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
func (d *sweepTestDB) Close() error                             { return nil }

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

func newSweepTestDB(t *testing.T) *sweepTestDB {
	t.Helper()
	db, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SweepTask{}))
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
