package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock provider ---

type mockFiatProvider struct {
	id            string
	parseErr      error
	parsedEvent   *contracts.WebhookEvent
	createResult  *contracts.PaymentSession
	createErr     error
	captureResult *contracts.PaymentResult
	captureErr    error
	getResult     *contracts.PaymentDetail
	getErr        error
}

func (m *mockFiatProvider) ProviderID() string { return m.id }

func (m *mockFiatProvider) CreatePayment(_ context.Context, _ contracts.CreatePaymentParams) (*contracts.PaymentSession, error) {
	return m.createResult, m.createErr
}

func (m *mockFiatProvider) CapturePayment(_ context.Context, _ string) (*contracts.PaymentResult, error) {
	return m.captureResult, m.captureErr
}

func (m *mockFiatProvider) GetPayment(_ context.Context, _ string) (*contracts.PaymentDetail, error) {
	return m.getResult, m.getErr
}

func (m *mockFiatProvider) ParseWebhook(_ context.Context, _ []byte, _ map[string]string) (*contracts.WebhookEvent, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}
	return m.parsedEvent, nil
}

// --- Mock registry ---

type mockFiatRegistry struct {
	mu        sync.RWMutex
	providers map[string]contracts.FiatPaymentProvider
}

func newMockFiatRegistry() *mockFiatRegistry {
	return &mockFiatRegistry{providers: make(map[string]contracts.FiatPaymentProvider)}
}

func (r *mockFiatRegistry) Register(p contracts.FiatPaymentProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ProviderID()] = p
}

func (r *mockFiatRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, id)
}

func (r *mockFiatRegistry) ForProvider(id string) (contracts.FiatPaymentProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider %q not registered", id)
	}
	return p, nil
}

func (r *mockFiatRegistry) Registered() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

// --- Test helpers ---

func newFiatTestDB(t *testing.T) database.Database {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, database.MigrateFiatModels(db))
	return db
}

func newFiatTestService(t *testing.T, reg contracts.FiatProviderRegistry) (*FiatPaymentAppService, database.Database) {
	t.Helper()
	db := newFiatTestDB(t)
	svc := NewFiatPaymentAppService(reg, db, "test-node")
	return svc, db
}

// --- Tests: Webhook handling ---

func TestFiatService_HandleWebhook_NoHandler_ReturnsError(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID: "evt_001", Type: contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_123", OrderID: "order_abc",
		},
	})

	svc, _ := newFiatTestService(t, reg)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("payload"), nil)
	assert.Error(t, err, "should fail when webhook handler is nil")
	assert.Contains(t, err.Error(), "no webhook handler registered")
}

func TestFiatService_HandleWebhook_Success(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID: "evt_002", Type: contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_456", OrderID: "order_def",
		},
	})

	svc, _ := newFiatTestService(t, reg)

	var handledEvent *contracts.WebhookEvent
	svc.SetWebhookHandler(func(_ context.Context, event *contracts.WebhookEvent) error {
		handledEvent = event
		return nil
	})

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("payload"), nil)
	require.NoError(t, err)
	require.NotNil(t, handledEvent)
	assert.Equal(t, "order_def", handledEvent.OrderID)
}

func TestFiatService_HandleWebhook_Idempotency(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID: "evt_003", Type: contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_789", OrderID: "order_ghi",
		},
	})

	svc, _ := newFiatTestService(t, reg)

	callCount := 0
	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		callCount++
		return nil
	})

	require.NoError(t, svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil))
	require.NoError(t, svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil))
	assert.Equal(t, 1, callCount, "handler should be called exactly once due to idempotency")
}

func TestFiatService_HandleWebhook_HandlerError_NotMarkedProcessed(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID: "evt_004", Type: contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_err", OrderID: "order_err",
		},
	})

	svc, _ := newFiatTestService(t, reg)

	callCount := 0
	handlerErr := errors.New("order processing failed")
	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		callCount++
		if callCount == 1 {
			return handlerErr
		}
		return nil
	})

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	assert.ErrorIs(t, err, handlerErr)

	err = svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err, "retry should succeed")
	assert.Equal(t, 2, callCount, "handler should be called again after failure")
}

func TestFiatService_HandleWebhook_ParseError(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id:       "stripe",
		parseErr: fmt.Errorf("%w: bad sig", contracts.ErrWebhookSignature),
	})

	svc, _ := newFiatTestService(t, reg)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("bad"), nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrWebhookSignature)
}

func TestFiatService_HandleWebhook_UnknownProvider(t *testing.T) {
	svc, _ := newFiatTestService(t, newMockFiatRegistry())
	err := svc.HandleWebhook(context.Background(), "paypal", nil, nil)
	assert.Error(t, err)
}

func TestFiatService_HandleWebhook_NoOrderID_ReturnsError(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID: "evt_no_order", Type: contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_noorder", OrderID: "",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error { return nil })

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	assert.Error(t, err, "should fail when OrderID is empty")
}

func TestFiatService_HandleWebhook_NonPaymentEvent(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID: "evt_dispute", Type: contracts.WebhookDisputeOpened,
			PaymentID: "pi_dispute",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	assert.NoError(t, err, "non-critical events should not error")
}

// --- Tests: Provider config management ---

func TestFiatService_SaveProviderConfig_CreatesReceivingAccount(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, db := newFiatTestService(t, reg)

	input := contracts.ProviderConfigInput{
		AccountID:     "acct_test123",
		PublicKey:     "pk_test_xxx",
		SecretKey:     "sk_test_xxx",
		WebhookSecret: "whsec_xxx",
	}

	require.NoError(t, svc.SaveProviderConfig("stripe", input))

	var ra models.ReceivingAccount
	err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ?", iwallet.ChainType("fiat:stripe")).First(&ra).Error
	})
	require.NoError(t, err, "ReceivingAccount should be created")
	assert.Equal(t, "acct_test123", ra.Address)

	assert.Contains(t, reg.Registered(), "stripe", "provider should be registered")
}

func TestFiatService_DeleteProviderConfig_UnregistersProvider(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestService(t, reg)

	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_del", PublicKey: "pk", SecretKey: "sk", WebhookSecret: "wh",
	}))
	assert.NotEmpty(t, reg.Registered())

	require.NoError(t, svc.DeleteProviderConfig("stripe"))
	assert.Empty(t, reg.Registered(), "provider should be unregistered after delete")
}

func TestFiatService_GetProviderConfig_Masked(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestService(t, reg)

	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_mask", PublicKey: "pk_test_longkey123",
		SecretKey: "sk_test_longkey456", WebhookSecret: "whsec_longkey789",
	}))

	view, err := svc.GetProviderConfig("stripe")
	require.NoError(t, err)
	assert.Equal(t, "stripe", view.ProviderID)
	assert.NotEqual(t, "sk_test_longkey456", view.SecretKey, "secret should be masked")
}

func TestFiatService_GetProviderConfig_NotFound(t *testing.T) {
	svc, _ := newFiatTestService(t, newMockFiatRegistry())

	_, err := svc.GetProviderConfig("paypal")
	assert.ErrorIs(t, err, contracts.ErrProviderNotFound)
}

// --- Tests: CreatePayment ---

func TestFiatService_CreatePayment_NoAccount(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id:           "stripe",
		createResult: &contracts.PaymentSession{SessionID: "sess_1"},
	})

	svc, _ := newFiatTestService(t, reg)

	_, err := svc.CreatePayment(context.Background(), "stripe", contracts.CreatePaymentParams{
		OrderID: "order_1", Amount: 1000, Currency: "USD",
	})
	assert.Error(t, err, "should fail when no receiving account")
}

func TestFiatService_CreatePayment_Success(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		createResult: &contracts.PaymentSession{
			SessionID: "sess_ok", CaptureMode: contracts.CaptureAutomatic,
			ExpiresAt: time.Now().Add(30 * time.Minute), Status: "requires_payment_method",
		},
	})

	svc, db := newFiatTestService(t, reg)

	// Directly create ReceivingAccount instead of using SaveProviderConfig
	// (which would register a real Stripe provider, overriding our mock)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ReceivingAccount{
			ChainType: iwallet.ChainType("fiat:stripe"),
			Address:   "acct_ok",
			IsActive:  true,
		})
	}))

	session, err := svc.CreatePayment(context.Background(), "stripe", contracts.CreatePaymentParams{
		OrderID: "order_2", Amount: 2500, Currency: "USD",
	})
	require.NoError(t, err)
	assert.Equal(t, "sess_ok", session.SessionID)
}

// --- Tests: LoadAndRegisterProviders ---

func TestFiatService_LoadAndRegisterProviders(t *testing.T) {
	reg := newMockFiatRegistry()
	db := newFiatTestDB(t)

	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.FiatProviderConfig{
			ProviderID: "stripe", AccountID: "acct_pre",
			PublicKey: "pk_pre", SecretKey: "sk_pre",
			WebhookSecret: "whsec_pre", IsActive: true,
		})
	}))

	svc := NewFiatPaymentAppService(reg, db, "test-node")
	svc.LoadAndRegisterProviders()

	ids := reg.Registered()
	assert.Equal(t, []string{"stripe"}, ids, "should auto-register from existing config")
}

func TestFiatService_EnabledProviders(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{id: "stripe"})
	svc, _ := newFiatTestService(t, reg)

	providers, err := svc.EnabledProviders(context.Background())
	require.NoError(t, err)
	require.Len(t, providers, 1)
	assert.Equal(t, "stripe", providers[0].ProviderID)
	assert.Equal(t, "not_connected", providers[0].Status)
}
