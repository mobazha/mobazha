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

func (m *mockFiatProvider) RefundPayment(_ context.Context, _ contracts.RefundParams) (*contracts.RefundResult, error) {
	return nil, nil
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

// --- Tests: Webhook PaymentData enrichment ---

func TestFiatService_HandleWebhook_EnrichesPaymentData(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID: "evt_enrich", Type: contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_enrich", OrderID: "order_enrich",
		},
		getResult: &contracts.PaymentDetail{
			PaymentID: "pi_enrich",
			Amount:    4999,
			Currency:  "USD",
			PaymentMethod: contracts.PaymentMethodInfo{
				Type: "card", Brand: "visa", Last4: "4242",
			},
		},
	})

	svc, _ := newFiatTestService(t, reg)

	var handledEvent *contracts.WebhookEvent
	svc.SetWebhookHandler(func(_ context.Context, event *contracts.WebhookEvent) error {
		handledEvent = event
		return nil
	})

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err)
	require.NotNil(t, handledEvent)
	assert.Equal(t, int64(4999), handledEvent.Amount)
	assert.Equal(t, "USD", handledEvent.Currency)
	assert.Equal(t, "visa", handledEvent.PaymentMethod.Brand)
	assert.Equal(t, "4242", handledEvent.PaymentMethod.Last4)
}

func TestFiatService_HandleWebhook_EnrichmentFailure_StillProcesses(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID: "evt_enrich_fail", Type: contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_fail_detail", OrderID: "order_enrich_fail",
		},
		getErr: errors.New("stripe api unreachable"),
	})

	svc, _ := newFiatTestService(t, reg)

	var handledEvent *contracts.WebhookEvent
	svc.SetWebhookHandler(func(_ context.Context, event *contracts.WebhookEvent) error {
		handledEvent = event
		return nil
	})

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err, "webhook processing should succeed despite enrichment failure")
	require.NotNil(t, handledEvent)
	assert.Equal(t, "order_enrich_fail", handledEvent.OrderID)
	assert.Equal(t, int64(0), handledEvent.Amount, "amount should remain zero when enrichment fails")
	assert.Equal(t, "", handledEvent.Currency, "currency should remain empty when enrichment fails")
}

// --- Mock OrderRepo for webhook handler tests ---

type mockOrderRepo struct {
	orders       map[string]*models.Order // keyed by OrderID
	byPaymentTx  map[string]*models.Order // keyed by PaymentTransactionID
	savedOrders  []*models.Order          // track Save calls
	mergedMeta   map[string]map[string]string
	findByIDErr  error
	findByTxErr  error
	saveErr      error
	mergeMetaErr error
}

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{
		orders:      make(map[string]*models.Order),
		byPaymentTx: make(map[string]*models.Order),
		mergedMeta:  make(map[string]map[string]string),
	}
}

func (m *mockOrderRepo) addOrder(o *models.Order) {
	m.orders[string(o.ID)] = o
	if o.PaymentTransactionID != "" {
		m.byPaymentTx[o.PaymentTransactionID] = o
	}
}

func (m *mockOrderRepo) FindByID(_ context.Context, orderID string) (*models.Order, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	o, ok := m.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order %s not found", orderID)
	}
	return o, nil
}

func (m *mockOrderRepo) FindByPaymentTransactionID(_ context.Context, txID string) (*models.Order, error) {
	if m.findByTxErr != nil {
		return nil, m.findByTxErr
	}
	o, ok := m.byPaymentTx[txID]
	if !ok {
		return nil, fmt.Errorf("order with payment tx %s not found", txID)
	}
	return o, nil
}

func (m *mockOrderRepo) Save(_ context.Context, order *models.Order) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.savedOrders = append(m.savedOrders, order)
	m.orders[string(order.ID)] = order
	return nil
}

func (m *mockOrderRepo) MergeFiatMetadata(_ context.Context, orderID string, kv map[string]string) error {
	if m.mergeMetaErr != nil {
		return m.mergeMetaErr
	}
	existing, ok := m.mergedMeta[orderID]
	if !ok {
		existing = make(map[string]string)
	}
	for k, v := range kv {
		existing[k] = v
	}
	m.mergedMeta[orderID] = existing
	return nil
}

func (m *mockOrderRepo) SetPaymentTransactionID(_ context.Context, orderID string, txID string) error {
	o, ok := m.orders[orderID]
	if ok {
		o.PaymentTransactionID = txID
		m.byPaymentTx[txID] = o
	}
	return nil
}

func (m *mockOrderRepo) FindPurchases(_ context.Context, _ contracts.OrderFilter) ([]models.Order, int64, error) {
	return nil, 0, nil
}
func (m *mockOrderRepo) FindSales(_ context.Context, _ contracts.OrderFilter) ([]models.Order, int64, error) {
	return nil, 0, nil
}
func (m *mockOrderRepo) FindUnverifiedPaymentOrders(_ context.Context) ([]models.Order, error) {
	return nil, nil
}
func (m *mockOrderRepo) MarkAsRead(_ context.Context, _ string) error { return nil }
func (m *mockOrderRepo) UpdateState(_ context.Context, _ string, _ models.OrderState) error {
	return nil
}
func (m *mockOrderRepo) UpdateLastCheckTime(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (m *mockOrderRepo) ExpirePaymentVerification(_ context.Context, _ string, _ time.Time) error {
	return nil
}

// Compile-time interface check
var _ contracts.OrderRepo = (*mockOrderRepo)(nil)

// --- Tests: Webhook event handlers (S2-11) ---

func TestFiatService_HandleWebhook_PaymentFailed_NoAutoCancel(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:       "evt_pf_001",
			Type:          contracts.WebhookPaymentFailed,
			ProviderID:    "stripe",
			PaymentID:     "pi_failed_001",
			OrderID:       "order_pf_001",
			FailureReason: "card_declined",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	repo := newMockOrderRepo()
	repo.addOrder(&models.Order{ID: "order_pf_001", State: models.OrderState_AWAITING_PAYMENT})
	svc.SetOrderRepo(repo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err, "PaymentFailed should not return error")

	order := repo.orders["order_pf_001"]
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT, order.State,
		"order state should NOT change on PaymentFailed — buyer may retry")
	assert.Empty(t, repo.savedOrders, "no Save call expected for PaymentFailed")
}

func TestFiatService_HandleWebhook_RefundCreated_TransitionsToRefunded(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:    "evt_rf_001",
			Type:       contracts.WebhookRefundCreated,
			ProviderID: "stripe",
			PaymentID:  "pi_refund_001",
			OrderID:    "order_rf_001",
			RefundID:   "re_001",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	repo := newMockOrderRepo()
	repo.addOrder(&models.Order{ID: "order_rf_001", State: models.OrderState_FULFILLED})
	svc.SetOrderRepo(repo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err)

	require.Len(t, repo.savedOrders, 1, "should Save order once")
	assert.Equal(t, models.OrderState_REFUNDED, repo.savedOrders[0].State,
		"order should transition to REFUNDED")
}

func TestFiatService_HandleWebhook_RefundCreated_AlreadyRefunded_Idempotent(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:    "evt_rf_dup",
			Type:       contracts.WebhookRefundCreated,
			ProviderID: "stripe",
			PaymentID:  "pi_refund_dup",
			OrderID:    "order_rf_dup",
			RefundID:   "re_dup",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	repo := newMockOrderRepo()
	repo.addOrder(&models.Order{ID: "order_rf_dup", State: models.OrderState_REFUNDED})
	svc.SetOrderRepo(repo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err)

	assert.Empty(t, repo.savedOrders, "should NOT Save when already REFUNDED (idempotent)")
}

func TestFiatService_HandleWebhook_DisputeOpened_StoresMetadata(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:       "evt_do_001",
			Type:          contracts.WebhookDisputeOpened,
			ProviderID:    "stripe",
			PaymentID:     "pi_dispute_001",
			OrderID:       "order_do_001",
			DisputeID:     "dp_001",
			DisputeReason: "product_not_received",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	repo := newMockOrderRepo()
	repo.addOrder(&models.Order{ID: "order_do_001", State: models.OrderState_FULFILLED})
	svc.SetOrderRepo(repo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err)

	meta, ok := repo.mergedMeta["order_do_001"]
	require.True(t, ok, "MergeFiatMetadata should have been called")
	assert.Equal(t, "opened", meta["fiat_dispute_status"])
	assert.Equal(t, "dp_001", meta["fiat_dispute_id"])
	assert.Equal(t, "product_not_received", meta["fiat_dispute_reason"])
	assert.NotEmpty(t, meta["fiat_dispute_opened_at"])
	assert.Empty(t, repo.savedOrders, "DisputeOpened should not change order state")
}

func TestFiatService_HandleWebhook_DisputeResolved_Lost_TransitionsToRefunded(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:        "evt_dr_lost",
			Type:           contracts.WebhookDisputeResolved,
			ProviderID:     "stripe",
			PaymentID:      "pi_dispute_lost",
			OrderID:        "order_dr_lost",
			DisputeID:      "dp_lost",
			DisputeOutcome: "lost",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	repo := newMockOrderRepo()
	repo.addOrder(&models.Order{ID: "order_dr_lost", State: models.OrderState_FULFILLED})
	svc.SetOrderRepo(repo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err)

	meta, ok := repo.mergedMeta["order_dr_lost"]
	require.True(t, ok, "MergeFiatMetadata should have been called")
	assert.Equal(t, "resolved", meta["fiat_dispute_status"])
	assert.Equal(t, "lost", meta["fiat_dispute_outcome"])
	assert.NotEmpty(t, meta["fiat_dispute_resolved_at"])

	require.Len(t, repo.savedOrders, 1, "should Save order when dispute lost")
	assert.Equal(t, models.OrderState_REFUNDED, repo.savedOrders[0].State,
		"dispute lost → REFUNDED")
}

func TestFiatService_HandleWebhook_DisputeResolved_Lost_SaveFails_ReturnsError(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:        "evt_dr_save_fail",
			Type:           contracts.WebhookDisputeResolved,
			ProviderID:     "stripe",
			PaymentID:      "pi_dispute_sf",
			OrderID:        "order_dr_sf",
			DisputeID:      "dp_sf",
			DisputeOutcome: "lost",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	repo := newMockOrderRepo()
	repo.addOrder(&models.Order{ID: "order_dr_sf", State: models.OrderState_FULFILLED})
	repo.saveErr = errors.New("db connection lost")
	svc.SetOrderRepo(repo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.Error(t, err, "should propagate Save error so event is NOT marked processed")
	assert.Contains(t, err.Error(), "REFUNDED sync failed")
}

func TestFiatService_HandleWebhook_AccountUpdated_LogsOnly(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:    "evt_au_001",
			Type:       contracts.WebhookAccountUpdated,
			ProviderID: "stripe",
			AccountID:  "acct_test_001",
			WebhookAccountStatus: &contracts.WebhookAccountStatus{
				ChargesEnabled: true,
				PayoutsEnabled: false,
			},
		},
	})

	svc, _ := newFiatTestService(t, reg)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err, "AccountUpdated should not return error")
}
