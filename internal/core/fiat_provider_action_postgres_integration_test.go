//go:build integration

package core

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const providerActionPostgresDSNEnv = "MOBAZHA_TEST_POSTGRES_DSN"

var errSimulatedProviderActionCrash = errors.New("simulated process exit before provider action completion persisted")

type failNextUpdateDatabase struct {
	database.Database
	failNext atomic.Bool
}

func (db *failNextUpdateDatabase) Update(fn func(tx database.Tx) error) error {
	if db.failNext.Swap(false) {
		return errSimulatedProviderActionCrash
	}
	return db.Database.Update(fn)
}

type idempotentCaptureProvider struct {
	*mockFiatProvider
	mu          sync.Mutex
	results     map[string]*contracts.PaymentResult
	effects     int
	afterEffect func()
}

func newIdempotentCaptureProvider(providerID string) *idempotentCaptureProvider {
	return &idempotentCaptureProvider{
		mockFiatProvider: &mockFiatProvider{id: providerID},
		results:          make(map[string]*contracts.PaymentResult),
	}
}

func (p *idempotentCaptureProvider) CapturePayment(_ context.Context, params contracts.CapturePaymentParams) (*contracts.PaymentResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.captureCalls = append(p.captureCalls, params)
	if result, ok := p.results[params.IdempotencyKey]; ok {
		copy := *result
		return &copy, nil
	}
	result := &contracts.PaymentResult{PaymentID: params.SessionID, Status: "succeeded"}
	p.results[params.IdempotencyKey] = result
	p.effects++
	if p.afterEffect != nil {
		p.afterEffect()
	}
	copy := *result
	return &copy, nil
}

func (p *idempotentCaptureProvider) snapshot() (calls []contracts.CapturePaymentParams, effects int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]contracts.CapturePaymentParams(nil), p.captureCalls...), p.effects
}

func openProviderActionPostgresDatabases(t *testing.T, tenantID string) (database.Database, database.Database) {
	t.Helper()
	dsn := os.Getenv(providerActionPostgresDSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set", providerActionPostgresDSNEnv)
	}

	open := func() (*gorm.DB, database.Database) {
		shared, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := shared.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		tenantDB, err := dbstore.NewTenantDBWithPublicData(shared, tenantID, nil)
		require.NoError(t, err)
		return shared, tenantDB
	}

	_, first := open()
	_, second := open()
	require.NoError(t, first.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.ReceivingAccount{})
	}))
	require.NoError(t, dbgorm.MigrateFiatModels(first))
	return first, second
}

func newProviderActionIntegrationService(reg contracts.FiatProviderRegistry, db database.Database, nodeID string) *FiatPaymentAppService {
	service := NewFiatPaymentAppService(reg, db, nodeID, false)
	service.SetProviderCredentialKeyProvider(testProviderCredentialKeys{})
	return service
}

func TestFiatProviderAction_PostgresMultiInstance_OnlyOneWorkerCallsProvider(t *testing.T) {
	dbOne, dbTwo := openProviderActionPostgresDatabases(t, "provider-action-multi-instance")
	entered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce sync.Once
	provider := &mockFiatProvider{
		id:            "stripe",
		captureResult: &contracts.PaymentResult{PaymentID: "pi_postgres_lease", Status: "succeeded"},
		beforeCapture: func(contracts.CapturePaymentParams) {
			enteredOnce.Do(func() { close(entered) })
			<-release
		},
	}
	registry := newMockFiatRegistry()
	registry.Register(provider)
	workerOne := newProviderActionIntegrationService(registry, dbOne, "postgres-worker-one")
	workerTwo := newProviderActionIntegrationService(registry, dbTwo, "postgres-worker-two")
	seedProviderActionAttempt(t, workerOne, dbOne, "stripe", "order_postgres_lease", "pi_postgres_lease")
	action, err := workerOne.prepareProviderAction(
		"stripe", models.PaymentProviderActionCapture, "pi_postgres_lease", "", "",
		providerCaptureIntent{SessionID: "pi_postgres_lease"},
	)
	require.NoError(t, err)

	firstResult := make(chan error, 1)
	go func() {
		_, executeErr := workerOne.executeProviderAction(context.Background(), action)
		firstResult <- executeErr
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("first PostgreSQL worker did not enter provider capture")
	}

	_, err = workerTwo.executeProviderAction(context.Background(), action)
	require.ErrorIs(t, err, contracts.ErrActionInProgress)
	close(release)
	require.NoError(t, <-firstResult)

	provider.captureMu.Lock()
	captureCalls := len(provider.captureCalls)
	provider.captureMu.Unlock()
	assert.Equal(t, 1, captureCalls)
	var completed models.PaymentProviderAction
	require.NoError(t, dbTwo.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", action.ActionID).First(&completed).Error
	}))
	assert.Equal(t, models.PaymentProviderActionCompleted, completed.State)
	assert.Equal(t, 1, completed.Attempts)
	assert.Empty(t, completed.LeaseOwner)
}

func TestFiatProviderAction_PostgresCrashAfterProviderSuccess_ReconcilesWithoutDuplicateEffect(t *testing.T) {
	dbOne, dbTwo := openProviderActionPostgresDatabases(t, "provider-action-crash-recovery")
	failingDB := &failNextUpdateDatabase{Database: dbOne}
	provider := newIdempotentCaptureProvider("stripe")
	provider.afterEffect = func() { failingDB.failNext.Store(true) }
	registry := newMockFiatRegistry()
	registry.Register(provider)
	crashingWorker := newProviderActionIntegrationService(registry, failingDB, "postgres-crashing-worker")
	recoveryWorker := newProviderActionIntegrationService(registry, dbTwo, "postgres-recovery-worker")
	seedProviderActionAttempt(t, crashingWorker, failingDB, "stripe", "order_postgres_crash", "pi_postgres_crash")
	action, err := crashingWorker.prepareProviderAction(
		"stripe", models.PaymentProviderActionCapture, "pi_postgres_crash", "", "",
		providerCaptureIntent{SessionID: "pi_postgres_crash"},
	)
	require.NoError(t, err)

	_, err = crashingWorker.executeProviderAction(context.Background(), action)
	require.ErrorIs(t, err, errSimulatedProviderActionCrash)
	calls, effects := provider.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, 1, effects, "the provider-side effect must have happened before the simulated crash")
	assert.NotEmpty(t, calls[0].IdempotencyKey)

	_, err = recoveryWorker.executeProviderAction(context.Background(), action)
	require.ErrorIs(t, err, contracts.ErrActionInProgress)
	calls, effects = provider.snapshot()
	require.Len(t, calls, 1, "a live lease must prevent immediate replay")
	assert.Equal(t, 1, effects)

	expiredAt := time.Now().UTC().Add(-time.Second)
	require.NoError(t, dbOne.Update(func(tx database.Tx) error {
		updated, updateErr := tx.UpdateColumns(
			map[string]interface{}{"lease_expires_at": expiredAt},
			map[string]interface{}{"action_id = ?": action.ActionID},
			&models.PaymentProviderAction{},
		)
		if updateErr != nil {
			return updateErr
		}
		if updated != 1 {
			return errors.New("provider action lease was not expired")
		}
		return nil
	}))
	recoveryWorker.reconcileProviderActions(context.Background())

	calls, effects = provider.snapshot()
	require.Len(t, calls, 2, "reconciliation must replay the ambiguous provider request")
	assert.Equal(t, calls[0].IdempotencyKey, calls[1].IdempotencyKey)
	assert.Equal(t, 1, effects, "provider idempotency must collapse the replay into one external effect")
	var completed models.PaymentProviderAction
	require.NoError(t, dbTwo.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", action.ActionID).First(&completed).Error
	}))
	assert.Equal(t, models.PaymentProviderActionCompleted, completed.State)
	assert.Equal(t, 1, completed.Attempts)
	assert.Empty(t, completed.LeaseOwner)
	assert.Nil(t, completed.LeaseExpiresAt)
	require.NotNil(t, completed.CompletedAt)
}
