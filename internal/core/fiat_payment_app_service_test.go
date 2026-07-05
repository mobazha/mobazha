package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	dbgorm "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

// --- Mock provider ---

type mockFiatProvider struct {
	createMu      sync.Mutex
	id            string
	parseErr      error
	parsedEvent   *contracts.WebhookEvent
	createResult  *contracts.FiatProviderSession
	createErr     error
	captureResult *contracts.PaymentResult
	captureErr    error
	getResult     *contracts.PaymentDetail
	getErr        error
	refundResult  *contracts.RefundResult
	refundErr     error
	refundCalls   []contracts.RefundParams
	cancelErr     error
	cancelCalls   []string
	createCalls   int
	createParams  []contracts.CreatePaymentParams
}

type mockWebhookProvider struct {
	*mockFiatProvider
	setupResult *contracts.WebhookSetupResult
	setupErr    error
}

func (m *mockWebhookProvider) SetupWebhook(context.Context, string) (*contracts.WebhookSetupResult, error) {
	return m.setupResult, m.setupErr
}

func (*mockWebhookProvider) CleanupWebhook(context.Context, string) error { return nil }

func (m *mockFiatProvider) ProviderID() string { return m.id }

func (m *mockFiatProvider) CreatePayment(_ context.Context, params contracts.CreatePaymentParams) (*contracts.FiatProviderSession, error) {
	m.createMu.Lock()
	defer m.createMu.Unlock()
	m.createCalls++
	m.createParams = append(m.createParams, params)
	return m.createResult, m.createErr
}

func (m *mockFiatProvider) CapturePayment(_ context.Context, _ string) (*contracts.PaymentResult, error) {
	return m.captureResult, m.captureErr
}

func (m *mockFiatProvider) GetPayment(_ context.Context, _ string) (*contracts.PaymentDetail, error) {
	return m.getResult, m.getErr
}

func (m *mockFiatProvider) RefundPayment(_ context.Context, params contracts.RefundParams) (*contracts.RefundResult, error) {
	m.refundCalls = append(m.refundCalls, params)
	return m.refundResult, m.refundErr
}

func (m *mockFiatProvider) CancelPayment(_ context.Context, paymentID string) error {
	m.cancelCalls = append(m.cancelCalls, paymentID)
	return m.cancelErr
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

type testProviderCredentialKeys struct{}

func (testProviderCredentialKeys) ProviderCredentialMasterKey(version uint64) ([]byte, error) {
	key := sha256.Sum256([]byte(fmt.Sprintf("test-provider-credential-key:%d", version)))
	return key[:], nil
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
	require.NoError(t, dbgorm.MigrateFiatModels(db))
	return db
}

func newFiatTestService(t *testing.T, reg contracts.FiatProviderRegistry) (*FiatPaymentAppService, database.Database) {
	t.Helper()
	db := newFiatTestDB(t)
	svc := NewFiatPaymentAppService(reg, db, "test-node", false)
	svc.SetProviderCredentialKeyProvider(testProviderCredentialKeys{})
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

	var binding models.PaymentProviderBinding
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().First(&binding).Error }))
	assert.Equal(t, models.PaymentProviderBindingActive, binding.State)
	assert.Equal(t, uint64(1), binding.ConfigurationGeneration)
	assert.Equal(t, "acct_test123", binding.ExternalAccountReference)
	assert.Equal(t, "tenant-config:stripe:1", binding.CredentialReference)
}

func TestFiatService_SaveProviderConfig_ConfigurationChangeCreatesNewBindingGeneration(t *testing.T) {
	svc, db := newFiatTestService(t, newMockFiatRegistry())
	initial := contracts.ProviderConfigInput{AccountID: "acct_v1", PublicKey: "pk", SecretKey: "sk_v1", WebhookSecret: "wh"}
	require.NoError(t, svc.SaveProviderConfig("stripe", initial))
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{}), "empty partial update must reuse the generation")
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{AccountID: "acct_v2", SecretKey: "sk_v2"}))

	var cfg models.FiatProviderConfig
	var bindings []models.PaymentProviderBinding
	require.NoError(t, db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("provider_id = ?", "stripe").First(&cfg).Error; err != nil {
			return err
		}
		return tx.Read().Where("provider_id = ?", "stripe").Order("configuration_generation ASC").Find(&bindings).Error
	}))
	assert.Equal(t, uint64(2), cfg.ConfigurationGeneration)
	require.Len(t, bindings, 2)
	assert.Equal(t, models.PaymentProviderBindingRetired, bindings[0].State)
	assert.Equal(t, "acct_v1", bindings[0].ExternalAccountReference)
	assert.Equal(t, models.PaymentProviderBindingActive, bindings[1].State)
	assert.Equal(t, "acct_v2", bindings[1].ExternalAccountReference)
	assert.NotEqual(t, bindings[0].BindingID, bindings[1].BindingID)
}

func TestFiatService_SaveProviderConfig_CredentialsAreEncryptedAndAppendOnly(t *testing.T) {
	svc, db := newFiatTestService(t, newMockFiatRegistry())
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_v1", PublicKey: "pk_v1", SecretKey: "sk_secret_v1", WebhookSecret: "wh_secret_v1",
	}))

	var first models.PaymentProviderCredential
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("credential_reference = ?", "tenant-config:stripe:1").First(&first).Error
	}))
	assert.NotContains(t, string(first.Ciphertext), "sk_secret_v1")
	assert.NotContains(t, string(first.Ciphertext), "wh_secret_v1")
	firstCiphertext := append([]byte(nil), first.Ciphertext...)

	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{}))
	var repeated models.PaymentProviderCredential
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("credential_reference = ?", "tenant-config:stripe:1").First(&repeated).Error
	}))
	assert.Equal(t, firstCiphertext, repeated.Ciphertext, "an existing credential generation must never be rewritten")

	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{SecretKey: "sk_secret_v2"}))
	var credentials []models.PaymentProviderCredential
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ?", "stripe").Order("configuration_generation ASC").Find(&credentials).Error
	}))
	require.Len(t, credentials, 2)
	assert.Equal(t, uint64(1), credentials[0].ConfigurationGeneration)
	assert.Equal(t, firstCiphertext, credentials[0].Ciphertext)
	assert.Equal(t, uint64(2), credentials[1].ConfigurationGeneration)
}

func TestFiatService_LoadProviderCredential_RejectsValidCiphertextSubstitution(t *testing.T) {
	svc, db := newFiatTestService(t, newMockFiatRegistry())
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_swap", PublicKey: "pk", SecretKey: "sk_v1", WebhookSecret: "wh_v1",
	}))
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{SecretKey: "sk_v2"}))

	var first, second models.PaymentProviderCredential
	require.NoError(t, db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("credential_reference = ?", "tenant-config:stripe:1").First(&first).Error; err != nil {
			return err
		}
		return tx.Read().Where("credential_reference = ?", "tenant-config:stripe:2").First(&second).Error
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(
			map[string]interface{}{"ciphertext": second.Ciphertext},
			map[string]interface{}{"credential_reference = ?": first.CredentialReference},
			&models.PaymentProviderCredential{},
		)
		return err
	}))

	err := db.View(func(tx database.Tx) error {
		_, err := svc.loadProviderCredentialTx(
			tx, first.CredentialReference, first.ProviderID, first.ExternalAccountReference,
			first.ConfigurationGeneration, first.ConfigurationFingerprint,
		)
		return err
	})
	require.ErrorContains(t, err, "failed integrity verification")
}

func TestFiatService_SetupWebhook_CreatesNewCredentialGeneration(t *testing.T) {
	svc, db := newFiatTestService(t, newMockFiatRegistry())
	provider := &mockWebhookProvider{
		mockFiatProvider: &mockFiatProvider{id: "stripe"},
		setupResult:      &contracts.WebhookSetupResult{WebhookID: "we_2", WebhookSecret: "wh_secret_v2"},
	}
	svc.providerFactory = func(string, providerCredentialMaterial, bool, *contracts.PlatformProviderOpts) (contracts.FiatPaymentProvider, error) {
		return provider, nil
	}
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_webhook", PublicKey: "pk", SecretKey: "sk", WebhookSecret: "wh_secret_v1",
	}))

	result, err := svc.SetupWebhook(context.Background(), "stripe", "https://example.test/webhook")
	require.NoError(t, err)
	assert.Equal(t, "we_2", result.WebhookID)

	var cfg models.FiatProviderConfig
	var credentialCount int64
	var current providerCredentialMaterial
	require.NoError(t, db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("provider_id = ?", "stripe").First(&cfg).Error; err != nil {
			return err
		}
		if err := tx.Read().Model(&models.PaymentProviderCredential{}).Where("provider_id = ?", "stripe").Count(&credentialCount).Error; err != nil {
			return err
		}
		var err error
		current, err = svc.loadProviderCredentialTx(tx, cfg.CredentialReference, cfg.ProviderID, cfg.AccountID, cfg.ConfigurationGeneration, cfg.ConfigurationFingerprint)
		return err
	}))
	assert.Equal(t, uint64(2), cfg.ConfigurationGeneration)
	assert.Equal(t, "tenant-config:stripe:2", cfg.CredentialReference)
	assert.Equal(t, int64(2), credentialCount)
	assert.Equal(t, "wh_secret_v2", current.WebhookSecret)
}

func TestFiatService_ProviderBinding_PlatformAccountRebindAdvancesGeneration(t *testing.T) {
	svc, db := newFiatTestService(t, newMockFiatRegistry())
	svc.markPlatformProvider("stripe")
	var first, repeated, second models.PaymentProviderBinding
	require.NoError(t, db.Update(func(tx database.Tx) error {
		var err error
		first, err = svc.ensureProviderBindingTx(tx, "stripe", "acct_platform_v1")
		if err != nil {
			return err
		}
		repeated, err = svc.ensureProviderBindingTx(tx, "stripe", "acct_platform_v1")
		if err != nil {
			return err
		}
		second, err = svc.ensureProviderBindingTx(tx, "stripe", "acct_platform_v2")
		return err
	}))
	assert.Equal(t, first.BindingID, repeated.BindingID)
	assert.Equal(t, uint64(1), first.ConfigurationGeneration)
	assert.Equal(t, uint64(2), second.ConfigurationGeneration)
	assert.NotEqual(t, first.BindingID, second.BindingID)

	var bindings []models.PaymentProviderBinding
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ?", "stripe").Order("configuration_generation ASC").Find(&bindings).Error
	}))
	require.Len(t, bindings, 2)
	assert.Equal(t, models.PaymentProviderBindingRetired, bindings[0].State)
	assert.Equal(t, models.PaymentProviderBindingActive, bindings[1].State)
}

func TestFiatService_deleteProviderConfig_UnregistersProvider(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, db := newFiatTestService(t, reg)

	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_del", PublicKey: "pk", SecretKey: "sk", WebhookSecret: "wh",
	}))
	assert.NotEmpty(t, reg.Registered())

	require.NoError(t, svc.deleteProviderConfig(context.Background(), "stripe"))
	assert.Empty(t, reg.Registered(), "provider should be unregistered after delete")
	var credentialCount int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.PaymentProviderCredential{}).Where("provider_id = ?", "stripe").Count(&credentialCount).Error
	}))
	assert.Equal(t, int64(1), credentialCount, "disconnect must retain historical credentials for in-flight recovery")
	var binding models.PaymentProviderBinding
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().First(&binding).Error }))
	assert.Equal(t, models.PaymentProviderBindingRetired, binding.State)
	assert.NotNil(t, binding.RetiredAt)

	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_del", PublicKey: "pk", SecretKey: "sk", WebhookSecret: "wh",
	}))
	var bindings []models.PaymentProviderBinding
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("provider_id = ?", "stripe").Order("configuration_generation ASC").Find(&bindings).Error
	}))
	require.Len(t, bindings, 2)
	assert.Equal(t, uint64(2), bindings[1].ConfigurationGeneration)
	assert.Equal(t, models.PaymentProviderBindingActive, bindings[1].State)
}

func TestFiatService_deleteProviderConfig_KeepPlatformProviderRegistered(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestService(t, reg)

	require.NoError(t, svc.SaveProviderConfig("paypal", contracts.ProviderConfigInput{
		AccountID: "acct_del_pp", PublicKey: "client_id", SecretKey: "client_secret", WebhookSecret: "wh",
	}))
	svc.markPlatformProvider("paypal")

	require.NoError(t, svc.deleteProviderConfig(context.Background(), "paypal"))
	_, err := reg.ForProvider("paypal")
	require.NoError(t, err, "platform provider should stay registered after disconnect")
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
		createResult: &contracts.FiatProviderSession{SessionID: "sess_1"},
	})

	svc, _ := newFiatTestService(t, reg)

	_, err := svc.CreatePayment(context.Background(), "stripe", contracts.CreatePaymentParams{
		OrderID: "order_1", Amount: 1000, Currency: "USD",
	})
	assert.Error(t, err, "should fail when no receiving account")
}

func TestFiatService_CreatePayment_Success(t *testing.T) {
	reg := newMockFiatRegistry()
	provider := &mockFiatProvider{
		id: "stripe",
		createResult: &contracts.FiatProviderSession{
			SessionID: "sess_ok", CaptureMode: contracts.CaptureAutomatic,
			ExpiresAt: time.Now().Add(30 * time.Minute), Status: "requires_payment_method",
		},
	}
	reg.Register(provider)

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
	require.Len(t, provider.createParams, 1)
	assert.NotEmpty(t, provider.createParams[0].IdempotencyKey)
	assert.NotEmpty(t, provider.createParams[0].Metadata["mobazha_payment_attempt_id"])

	var attempt models.PaymentAttempt
	var route models.PaymentRouteBinding
	var binding models.PaymentProviderBinding
	require.NoError(t, db.View(func(tx database.Tx) error {
		if err := tx.Read().First(&attempt).Error; err != nil {
			return err
		}
		if err := tx.Read().First(&route).Error; err != nil {
			return err
		}
		return tx.Read().First(&binding).Error
	}))
	assert.Equal(t, models.PaymentAttemptExternalCreated, attempt.State)
	assert.Equal(t, "sess_ok", attempt.ExternalReference)
	assert.Equal(t, attempt.AttemptID, route.AttemptID)
	assert.Equal(t, "core.fiat.stripe", route.ContributionID)
	assert.Equal(t, "fiat:stripe:USD", route.AssetID)
	assert.Equal(t, binding.BindingID, route.ProviderBindingID)
	assert.Equal(t, binding.ExternalAccountReference, route.ExternalAccountReference)
	assert.Equal(t, provider.createParams[0].IdempotencyKey, attempt.IdempotencyKey)
	require.ErrorContains(t, svc.commitFiatPaymentAttempt(attempt.AttemptID, "sess_conflict"), "provider reference conflict")
}

func TestFiatService_CreatePayment_RetryUsesSameDurableAttemptAndProviderKey(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe", createResult: &contracts.FiatProviderSession{SessionID: "sess_retry"}}
	reg := newMockFiatRegistry()
	reg.Register(provider)
	svc, db := newFiatTestService(t, reg)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ReceivingAccount{ChainType: FiatChainType("stripe"), Address: "acct_retry", IsActive: true})
	}))
	params := contracts.CreatePaymentParams{OrderID: "order_retry", Amount: 2500, Currency: "USD"}

	first, err := svc.CreatePayment(context.Background(), "stripe", params)
	require.NoError(t, err)
	second, err := svc.CreatePayment(context.Background(), "stripe", params)
	require.NoError(t, err)
	assert.Equal(t, first.SessionID, second.SessionID)
	require.Len(t, provider.createParams, 2)
	assert.Equal(t, provider.createParams[0].IdempotencyKey, provider.createParams[1].IdempotencyKey)

	var attempts int64
	var routes int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		if err := tx.Read().Model(&models.PaymentAttempt{}).Count(&attempts).Error; err != nil {
			return err
		}
		return tx.Read().Model(&models.PaymentRouteBinding{}).Count(&routes).Error
	}))
	assert.Equal(t, int64(1), attempts)
	assert.Equal(t, int64(1), routes)
}

func TestFiatService_CreatePayment_ConcurrentRetryUsesOneDurableClaim(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe", createResult: &contracts.FiatProviderSession{SessionID: "sess_concurrent"}}
	reg := newMockFiatRegistry()
	reg.Register(provider)
	svc, db := newFiatTestService(t, reg)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ReceivingAccount{ChainType: FiatChainType("stripe"), Address: "acct_concurrent", IsActive: true})
	}))
	params := contracts.CreatePaymentParams{OrderID: "order_concurrent", Amount: 2500, Currency: "USD"}

	results := make(chan *contracts.FiatProviderSession, 2)
	errs := make(chan error, 2)
	var callers sync.WaitGroup
	callers.Add(2)
	for range 2 {
		go func() {
			defer callers.Done()
			result, err := svc.CreatePayment(context.Background(), "stripe", params)
			results <- result
			errs <- err
		}()
	}
	callers.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	for result := range results {
		require.NotNil(t, result)
		assert.Equal(t, "sess_concurrent", result.SessionID)
	}
	require.Len(t, provider.createParams, 2)
	assert.Equal(t, provider.createParams[0].IdempotencyKey, provider.createParams[1].IdempotencyKey)

	var attempts, routes int64
	require.NoError(t, db.View(func(tx database.Tx) error {
		if err := tx.Read().Model(&models.PaymentAttempt{}).Count(&attempts).Error; err != nil {
			return err
		}
		return tx.Read().Model(&models.PaymentRouteBinding{}).Count(&routes).Error
	}))
	assert.Equal(t, int64(1), attempts)
	assert.Equal(t, int64(1), routes)
}

func TestFiatService_CreatePayment_ProviderReferenceConflictRequiresReconciliation(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe", createResult: &contracts.FiatProviderSession{SessionID: "sess_first"}}
	reg := newMockFiatRegistry()
	reg.Register(provider)
	svc, db := newFiatTestService(t, reg)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ReceivingAccount{ChainType: FiatChainType("stripe"), Address: "acct_conflict", IsActive: true})
	}))
	params := contracts.CreatePaymentParams{OrderID: "order_conflict", Amount: 2500, Currency: "USD"}

	_, err := svc.CreatePayment(context.Background(), "stripe", params)
	require.NoError(t, err)
	provider.createResult = &contracts.FiatProviderSession{SessionID: "sess_duplicate"}
	_, err = svc.CreatePayment(context.Background(), "stripe", params)
	require.ErrorContains(t, err, "provider reference conflict")

	var attempt models.PaymentAttempt
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().First(&attempt).Error }))
	assert.Equal(t, models.PaymentAttemptReconcileRequired, attempt.State)
	assert.Equal(t, "sess_first", attempt.ExternalReference)
	assert.Contains(t, attempt.LastError, "sess_duplicate")
}

func TestFiatService_CreatePayment_NodeIdentityScopesProviderIdempotency(t *testing.T) {
	newService := func(nodeID string) (*FiatPaymentAppService, *mockFiatProvider) {
		provider := &mockFiatProvider{id: "stripe", createResult: &contracts.FiatProviderSession{SessionID: "sess_" + nodeID}}
		reg := newMockFiatRegistry()
		reg.Register(provider)
		db := newFiatTestDB(t)
		require.NoError(t, db.Update(func(tx database.Tx) error {
			return tx.Save(&models.ReceivingAccount{ChainType: FiatChainType("stripe"), Address: "acct_shared", IsActive: true})
		}))
		return NewFiatPaymentAppService(reg, db, nodeID, false), provider
	}
	firstService, firstProvider := newService("node-a")
	secondService, secondProvider := newService("node-b")
	params := contracts.CreatePaymentParams{OrderID: "order_shared", Amount: 2500, Currency: "USD"}

	_, err := firstService.CreatePayment(context.Background(), "stripe", params)
	require.NoError(t, err)
	_, err = secondService.CreatePayment(context.Background(), "stripe", params)
	require.NoError(t, err)
	require.Len(t, firstProvider.createParams, 1)
	require.Len(t, secondProvider.createParams, 1)
	assert.NotEqual(t, firstProvider.createParams[0].IdempotencyKey, secondProvider.createParams[0].IdempotencyKey)
}

func TestFiatService_CreatePayment_ProviderFailureLeavesReconcileClaim(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe", createErr: errors.New("ambiguous timeout")}
	reg := newMockFiatRegistry()
	reg.Register(provider)
	svc, db := newFiatTestService(t, reg)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ReceivingAccount{ChainType: FiatChainType("stripe"), Address: "acct_timeout", IsActive: true})
	}))

	_, err := svc.CreatePayment(context.Background(), "stripe", contracts.CreatePaymentParams{OrderID: "order_timeout", Amount: 2500, Currency: "USD"})
	require.ErrorContains(t, err, "ambiguous timeout")
	var attempt models.PaymentAttempt
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().First(&attempt).Error }))
	assert.Equal(t, models.PaymentAttemptReconcileRequired, attempt.State)
	assert.Contains(t, attempt.LastError, "ambiguous timeout")
}

func TestFiatService_ReconcilePaymentAttempt_ReplaysProviderCreateWithSameClaim(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe", createErr: errors.New("ambiguous timeout")}
	reg := newMockFiatRegistry()
	reg.Register(provider)
	svc, db := newFiatTestService(t, reg)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.ReceivingAccount{ChainType: FiatChainType("stripe"), Address: "acct_recover", IsActive: true})
	}))
	params := contracts.CreatePaymentParams{OrderID: "order_recover", Amount: 2500, Currency: "USD"}

	_, err := svc.CreatePayment(context.Background(), "stripe", params)
	require.ErrorContains(t, err, "ambiguous timeout")
	require.Len(t, provider.createParams, 1)
	originalKey := provider.createParams[0].IdempotencyKey
	provider.createErr = nil
	provider.createResult = &contracts.FiatProviderSession{SessionID: "sess_recovered"}

	svc.ReconcileFiatOrders(context.Background())
	require.Len(t, provider.createParams, 2)
	assert.Equal(t, originalKey, provider.createParams[1].IdempotencyKey)
	assert.Equal(t, "acct_recover", provider.createParams[1].SellerAccountID)
	var attempt models.PaymentAttempt
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().First(&attempt).Error }))
	assert.Equal(t, models.PaymentAttemptExternalCreated, attempt.State)
	assert.Equal(t, "sess_recovered", attempt.ExternalReference)
	assert.Empty(t, attempt.LastError)
}

func TestFiatService_ReconcilePaymentAttempt_UsesHistoricalDirectCredential(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, db := newFiatTestService(t, reg)
	oldProvider := &mockFiatProvider{id: "stripe", createErr: errors.New("ambiguous timeout")}
	currentProvider := &mockFiatProvider{id: "stripe", createResult: &contracts.FiatProviderSession{SessionID: "current_must_not_be_used"}}
	svc.providerFactory = func(_ string, credential providerCredentialMaterial, _ bool, _ *contracts.PlatformProviderOpts) (contracts.FiatPaymentProvider, error) {
		if credential.SecretKey == "sk_old" {
			return oldProvider, nil
		}
		return currentProvider, nil
	}
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_old", PublicKey: "pk", SecretKey: "sk_old", WebhookSecret: "wh",
	}))
	_, err := svc.CreatePayment(context.Background(), "stripe", contracts.CreatePaymentParams{
		OrderID: "order_old_binding", Amount: 2500, Currency: "USD",
	})
	require.ErrorContains(t, err, "ambiguous timeout")
	require.Len(t, oldProvider.createParams, 1)

	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{AccountID: "acct_new", SecretKey: "sk_new"}))
	oldProvider.createErr = nil
	oldProvider.createResult = &contracts.FiatProviderSession{SessionID: "created_with_historical_credential"}
	svc.ReconcileFiatOrders(context.Background())
	require.Len(t, oldProvider.createParams, 2)
	assert.Equal(t, "acct_old", oldProvider.createParams[1].SellerAccountID)
	assert.Zero(t, currentProvider.createCalls, "reconciliation must not use current credentials for a historical direct binding")

	var attempt models.PaymentAttempt
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().First(&attempt).Error }))
	assert.Equal(t, models.PaymentAttemptExternalCreated, attempt.State)
	assert.Equal(t, "created_with_historical_credential", attempt.ExternalReference)
}

func TestFiatService_ReconcilePaymentAttempt_MissingHistoricalCredentialFailsClosed(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, db := newFiatTestService(t, reg)
	oldProvider := &mockFiatProvider{id: "stripe", createErr: errors.New("ambiguous timeout")}
	currentProvider := &mockFiatProvider{id: "stripe", createResult: &contracts.FiatProviderSession{SessionID: "must_not_be_created"}}
	svc.providerFactory = func(_ string, credential providerCredentialMaterial, _ bool, _ *contracts.PlatformProviderOpts) (contracts.FiatPaymentProvider, error) {
		if credential.SecretKey == "sk_old" {
			return oldProvider, nil
		}
		return currentProvider, nil
	}
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_old", PublicKey: "pk", SecretKey: "sk_old", WebhookSecret: "wh",
	}))
	_, err := svc.CreatePayment(context.Background(), "stripe", contracts.CreatePaymentParams{
		OrderID: "order_missing_credential", Amount: 2500, Currency: "USD",
	})
	require.ErrorContains(t, err, "ambiguous timeout")
	require.NoError(t, svc.SaveProviderConfig("stripe", contracts.ProviderConfigInput{SecretKey: "sk_new"}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Delete("credential_reference", "tenant-config:stripe:1", nil, &models.PaymentProviderCredential{})
	}))

	svc.ReconcileFiatOrders(context.Background())
	require.Len(t, oldProvider.createParams, 1)
	assert.Zero(t, currentProvider.createCalls)
	var attempt models.PaymentAttempt
	require.NoError(t, db.View(func(tx database.Tx) error { return tx.Read().First(&attempt).Error }))
	assert.Equal(t, models.PaymentAttemptReconcileRequired, attempt.State)
	assert.Contains(t, attempt.LastError, "route decision denied historical provider binding")
	assert.Contains(t, attempt.LastError, "credential reference tenant-config:stripe:1 is unavailable")
}

func TestFiatService_CreatePayment_RejectsManagedCollectibleBeforeProvider(t *testing.T) {
	provider := &mockFiatProvider{
		id:           "stripe",
		createResult: &contracts.FiatProviderSession{SessionID: "must-not-exist"},
	}
	reg := newMockFiatRegistry()
	reg.Register(provider)
	svc, db := newFiatTestService(t, reg)
	svc.AddProvisioningPolicy(corepayment.NewOrderExtensionProvisioningPolicy(
		func(corepayment.SessionProvisioningPolicyInput) (extensions.OrderExtension, bool, error) {
			return extensions.OrderExtension{}, false, corepayment.ErrRWAPaymentSessionUnsupported
		}, nil,
	))

	open := &pb.OrderOpen{
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{
			Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_RWA_TOKEN},
			Item: &pb.Listing_Item{
				Blockchain:    "SOL",
				TokenStandard: "metaplex_pnft",
			},
		}}},
		Items: []*pb.OrderOpen_Item{{OptionalFeatures: []string{
			models.CollectibleOptionalFeature(models.CollectibleFeatureFulfillment, models.CollectibleFulfillmentNFT),
			models.CollectibleOptionalFeature(models.CollectibleFeatureHubSlotID, "source-1"),
			models.CollectibleOptionalFeature(models.CollectibleFeatureCertNumber, "PSA-1"),
			models.CollectibleOptionalFeature(models.CollectibleFeatureHolderWallet, "11111111111111111111111111111111"),
		}}},
	}
	rawOpen, err := protojson.Marshal(open)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{ID: models.OrderID("source-fiat-order"), SerializedOrderOpen: rawOpen})
	}))

	_, err = svc.CreatePayment(context.Background(), "stripe", contracts.CreatePaymentParams{
		OrderID: "source-fiat-order", Amount: 2500, Currency: "USD",
	})
	require.ErrorIs(t, err, corepayment.ErrRWAPaymentSessionUnsupported)
	require.Zero(t, provider.createCalls, "provider must not receive a source-custody fiat request")
}

// --- Tests: LoadAndRegisterProviders ---

func TestFiatService_LoadAndRegisterProviders(t *testing.T) {
	db := newFiatTestDB(t)
	seed := NewFiatPaymentAppService(newMockFiatRegistry(), db, "test-node", false)
	seed.SetProviderCredentialKeyProvider(testProviderCredentialKeys{})
	require.NoError(t, seed.SaveProviderConfig("stripe", contracts.ProviderConfigInput{
		AccountID: "acct_pre", PublicKey: "pk_pre", SecretKey: "sk_pre", WebhookSecret: "whsec_pre",
	}))
	reg := newMockFiatRegistry()
	svc := NewFiatPaymentAppService(reg, db, "test-node", false)
	svc.SetProviderCredentialKeyProvider(testProviderCredentialKeys{})
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

func TestFiatService_HandleWebhook_PaymentFailed_MarksVerificationFailedWhenAwaitingVerification(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:       "evt_pf_002",
			Type:          contracts.WebhookPaymentFailed,
			ProviderID:    "stripe",
			PaymentID:     "pi_failed_002",
			OrderID:       "order_pf_002",
			FailureReason: "card_declined",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	repo := newMockOrderRepo()
	repo.addOrder(&models.Order{
		ID:                    "order_pf_002",
		State:                 models.OrderState_AWAITING_PAYMENT_VERIFICATION,
		SerializedPaymentSent: []byte{1},
	})
	svc.SetOrderRepo(repo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err)

	require.Len(t, repo.savedOrders, 1, "verification failure should be persisted once")
	saved := repo.savedOrders[0]
	assert.Equal(t, models.OrderState_AWAITING_PAYMENT_VERIFICATION, saved.State,
		"state should stay awaiting verification; only verification result changes")
	assert.True(t, saved.IsPaymentVerificationFailed(), "payment verification should be failed")
	assert.Equal(t, "provider_failed", saved.PaymentVerificationFailureReason)
	assert.NotNil(t, saved.PaymentVerificationFailedAt)
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
	repo.addOrder(&models.Order{ID: "order_rf_001", State: models.OrderState_SHIPPED})
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
	repo.addOrder(&models.Order{ID: "order_do_001", State: models.OrderState_SHIPPED})
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
	repo.addOrder(&models.Order{ID: "order_dr_lost", State: models.OrderState_SHIPPED})
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
	repo.addOrder(&models.Order{ID: "order_dr_sf", State: models.OrderState_SHIPPED})
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

// --- S3-4: Webhook race condition handling tests ---

func TestFiatService_WebhookRace_OrderNotCreated_ReturnsRetryableError(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:   "evt_race_notfound",
			Type:      contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_race_notfound",
			OrderID:   "order_not_yet_created",
		},
	})

	svc, _ := newFiatTestService(t, reg)
	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		t.Fatal("webhook handler should NOT be called when order not found")
		return nil
	})

	orderRepo := newMockOrderRepo()
	orderRepo.findByIDErr = gorm.ErrRecordNotFound
	svc.SetOrderRepo(orderRepo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.Error(t, err)

	var retryErr *contracts.RetryableError
	require.True(t, errors.As(err, &retryErr), "error should be RetryableError")
	assert.Equal(t, 30*time.Second, retryErr.RetryAfter)
	assert.Contains(t, retryErr.Error(), "not yet created")
}

func TestFiatService_WebhookRace_OrderCanceled_AutoRefund(t *testing.T) {
	provider := &mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:   "evt_race_canceled",
			Type:      contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_race_canceled",
			OrderID:   "order_already_canceled",
		},
		refundResult: &contracts.RefundResult{RefundID: "re_auto_123"},
	}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, _ := newFiatTestService(t, reg)
	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		t.Fatal("webhook handler should NOT be called for canceled orders")
		return nil
	})

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "order_already_canceled",
		State: models.OrderState_CANCELED,
	})
	svc.SetOrderRepo(orderRepo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err, "canceled order webhook should succeed (auto-refund)")

	require.Len(t, provider.refundCalls, 1, "should call RefundPayment once")
	assert.Equal(t, "pi_race_canceled", provider.refundCalls[0].PaymentID)
	assert.Equal(t, "order_canceled", provider.refundCalls[0].Reason)
}

func TestFiatService_WebhookRace_AutoRefundFails_NoRetryLoop(t *testing.T) {
	provider := &mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:   "evt_race_refund_fail",
			Type:      contracts.WebhookPaymentSucceeded,
			PaymentID: "pi_refund_fail",
			OrderID:   "order_canceled_refund_fail",
		},
		refundErr: errors.New("stripe refund API unreachable"),
	}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, _ := newFiatTestService(t, reg)
	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		t.Fatal("webhook handler should NOT be called for canceled orders")
		return nil
	})

	orderRepo := newMockOrderRepo()
	orderRepo.addOrder(&models.Order{
		ID:    "order_canceled_refund_fail",
		State: models.OrderState_CANCELED,
	})
	svc.SetOrderRepo(orderRepo)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err, "should return nil even when auto-refund fails, to avoid retry loop")

	require.Len(t, provider.refundCalls, 1, "should attempt refund once")
}

// --- Tests: DisconnectProvider ---

func newFiatTestDBWithOrders(t *testing.T) database.Database {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, dbgorm.MigrateFiatModels(db))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		return tx.Migrate(&models.ReceivingAccount{})
	}))
	return db
}

func newFiatTestServiceWithOrders(t *testing.T, reg contracts.FiatProviderRegistry) (*FiatPaymentAppService, database.Database) {
	t.Helper()
	db := newFiatTestDBWithOrders(t)
	svc := NewFiatPaymentAppService(reg, db, "test-node", false)
	svc.SetProviderCredentialKeyProvider(testProviderCredentialKeys{})
	return svc, db
}

func seedFiatOrder(t *testing.T, db database.Database, order *models.Order, state models.OrderState) {
	t.Helper()
	order.SetFSMState(state)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

func TestDisconnectProvider_NoActiveOrders_Success(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe"}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID: "completed-order",
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				PaymentTransactionID: "pi_completed_123",
				FiatMetadata:         []byte(`{"fiat_provider":"stripe"}`),
			},
		},
	}, models.OrderState_COMPLETED)

	svc.SetOrderRepo(newMockOrderRepo())

	ra := &models.ReceivingAccount{
		ChainType: FiatChainType("stripe"),
		Address:   "acct_test",
		IsActive:  true,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(ra)
	}))
	cfg := &models.FiatProviderConfig{
		ProviderID: "stripe",
		IsActive:   true,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(cfg)
	}))

	err := svc.DisconnectProvider(context.Background(), "stripe")
	require.NoError(t, err)

	var count int64
	_ = db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.ReceivingAccount{}).
			Where("chain_type = ?", FiatChainType("stripe")).Count(&count).Error
	})
	assert.Equal(t, int64(0), count, "ReceivingAccount should be deleted")
}

func TestDisconnectProvider_ActiveOrders_ReturnsError(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe"}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID: "shipped-order",
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				PaymentTransactionID: "pi_ship_123",
				FiatMetadata:         []byte(`{"fiat_provider":"stripe"}`),
			},
		},
	}, models.OrderState_AWAITING_SHIPMENT)

	svc.SetOrderRepo(newMockOrderRepo())

	err := svc.DisconnectProvider(context.Background(), "stripe")
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrActiveOrdersExist)
}

func TestDisconnectProvider_AwaitingPaymentVerification_BlocksDisconnect(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe"}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID: "awaiting-verification-order",
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				PaymentTransactionID: "pi_verify_123",
				FiatMetadata:         []byte(`{"fiat_provider":"stripe","fiat_session_id":"pi_verify_123"}`),
			},
		},
	}, models.OrderState_AWAITING_PAYMENT_VERIFICATION)

	svc.SetOrderRepo(newMockOrderRepo())

	err := svc.DisconnectProvider(context.Background(), "stripe")
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrActiveOrdersExist)
}

func TestDisconnectProvider_AwaitingPayment_CancelsSession(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe"}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID: "awaiting-payment-order",
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				FiatMetadata: []byte(`{"fiat_provider":"stripe","fiat_session_id":"pi_session_abc"}`),
			},
		},
	}, models.OrderState_AWAITING_PAYMENT)

	svc.SetOrderRepo(newMockOrderRepo())

	ra := &models.ReceivingAccount{
		ChainType: FiatChainType("stripe"),
		Address:   "acct_test",
		IsActive:  true,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(ra)
	}))
	cfg := &models.FiatProviderConfig{
		ProviderID: "stripe",
		IsActive:   true,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(cfg)
	}))

	err := svc.DisconnectProvider(context.Background(), "stripe")
	require.NoError(t, err)

	require.Len(t, provider.cancelCalls, 1)
	assert.Equal(t, "pi_session_abc", provider.cancelCalls[0])
}

func TestDisconnectProvider_CryptoOrderDoesNotBlock(t *testing.T) {
	provider := &mockFiatProvider{id: "stripe"}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID: "crypto-order",
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				PaymentTransactionID: "0xabc123",
			},
		},
	}, models.OrderState_AWAITING_SHIPMENT)

	svc.SetOrderRepo(newMockOrderRepo())

	ra := &models.ReceivingAccount{
		ChainType: FiatChainType("stripe"),
		Address:   "acct_test",
		IsActive:  true,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(ra)
	}))
	cfg := &models.FiatProviderConfig{
		ProviderID: "stripe",
		IsActive:   true,
	}
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(cfg)
	}))

	err := svc.DisconnectProvider(context.Background(), "stripe")
	require.NoError(t, err, "crypto orders should not block fiat provider disconnect")
}

// --- Tests: CleanupProcessedEvents ---

func TestFiatService_CleanupProcessedEvents_DeletesOldRecords(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, db := newFiatTestService(t, reg)

	old := &models.ProcessedFiatEvent{
		EventID:     "evt_old",
		ProviderID:  "stripe",
		ProcessedAt: time.Now().Add(-48 * time.Hour),
	}
	recent := &models.ProcessedFiatEvent{
		EventID:     "evt_recent",
		ProviderID:  "stripe",
		ProcessedAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(old) }))
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(recent) }))

	deleted, err := svc.CleanupProcessedEvents(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	var remaining []models.ProcessedFiatEvent
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Find(&remaining).Error
	}))
	require.Len(t, remaining, 1)
	assert.Equal(t, "evt_recent", remaining[0].EventID)
}

func TestFiatService_CleanupProcessedEvents_NoOldRecords(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, db := newFiatTestService(t, reg)

	recent := &models.ProcessedFiatEvent{
		EventID:     "evt_fresh",
		ProviderID:  "paypal",
		ProcessedAt: time.Now().Add(-30 * time.Minute),
	}
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(recent) }))

	deleted, err := svc.CleanupProcessedEvents(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
}

func TestFiatService_CleanupProcessedEvents_EmptyTable(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestService(t, reg)

	deleted, err := svc.CleanupProcessedEvents(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
}

// --- Tests: Payment Canceled webhook (P3) ---

func TestFiatService_HandleWebhook_PaymentCanceled_LogsOnly(t *testing.T) {
	reg := newMockFiatRegistry()
	reg.Register(&mockFiatProvider{
		id: "stripe",
		parsedEvent: &contracts.WebhookEvent{
			EventID:    "evt_pc_001",
			Type:       contracts.WebhookPaymentCanceled,
			ProviderID: "stripe",
			PaymentID:  "pi_canceled_001",
			OrderID:    "order_pc_001",
		},
	})

	svc, _ := newFiatTestService(t, reg)

	err := svc.HandleWebhook(context.Background(), "stripe", []byte("p"), nil)
	require.NoError(t, err, "PaymentCanceled should not return error")
}

// --- Tests: Fiat Reconciliation (P0) ---

func TestFiatService_ReconcileFiatOrders_SucceededPayment_TriggersHandler(t *testing.T) {
	provider := &mockFiatProvider{
		id: "stripe",
		getResult: &contracts.PaymentDetail{
			PaymentID: "pi_reconcile_001",
			Status:    "succeeded",
			Amount:    5000,
			Currency:  "USD",
			PaymentMethod: contracts.PaymentMethodInfo{
				Type: "card", Brand: "visa", Last4: "4242",
			},
		},
	}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID:   "order_reconcile_001",
		Open: true,
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				FiatMetadata: []byte(`{"fiat_provider":"stripe","fiat_session_id":"pi_reconcile_001"}`),
			},
		},
	}, models.OrderState_AWAITING_PAYMENT)

	var handledEvent *contracts.WebhookEvent
	svc.SetWebhookHandler(func(_ context.Context, event *contracts.WebhookEvent) error {
		handledEvent = event
		return nil
	})

	svc.ReconcileFiatOrders(context.Background())

	require.NotNil(t, handledEvent, "webhook handler should be called for succeeded payment")
	assert.Equal(t, "order_reconcile_001", handledEvent.OrderID)
	assert.Equal(t, "pi_reconcile_001", handledEvent.PaymentID)
	assert.Equal(t, int64(5000), handledEvent.Amount)
	assert.Equal(t, contracts.WebhookPaymentSucceeded, handledEvent.Type)
}

func TestFiatService_ReconcileFiatOrders_AwaitingVerificationState_SkippedByReconciliation(t *testing.T) {
	provider := &mockFiatProvider{
		id: "stripe",
		getResult: &contracts.PaymentDetail{
			PaymentID: "pi_verify_reconcile_001",
			Status:    "succeeded",
			Amount:    4200,
			Currency:  "USD",
			PaymentMethod: contracts.PaymentMethodInfo{
				Type: "card", Brand: "mastercard", Last4: "4444",
			},
		},
	}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID:   "order_reconcile_verify_001",
		Open: true,
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				FiatMetadata: []byte(`{"fiat_provider":"stripe","fiat_session_id":"pi_verify_reconcile_001"}`),
			},
		},
	}, models.OrderState_AWAITING_PAYMENT_VERIFICATION)

	var handledEvent *contracts.WebhookEvent
	svc.SetWebhookHandler(func(_ context.Context, event *contracts.WebhookEvent) error {
		handledEvent = event
		return nil
	})

	svc.ReconcileFiatOrders(context.Background())

	assert.Nil(t, handledEvent, "AWAITING_PAYMENT_VERIFICATION orders should be handled by PaymentVerificationLoop, not reconciliation")
}

func TestFiatService_ReconcileFiatOrders_UsesResolvedPaymentID(t *testing.T) {
	provider := &mockFiatProvider{
		id: "paypal",
		getResult: &contracts.PaymentDetail{
			PaymentID: "CAP-RECON-001",
			Status:    "succeeded",
			Amount:    5000,
			Currency:  "USD",
			PaymentMethod: contracts.PaymentMethodInfo{
				Type: "paypal", Brand: "paypal",
			},
		},
	}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID:   "order_reconcile_capture",
		Open: true,
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				FiatMetadata: []byte(`{"fiat_provider":"paypal","fiat_session_id":"ORDER-RECON-001"}`),
			},
		},
	}, models.OrderState_AWAITING_PAYMENT)

	var handledEvent *contracts.WebhookEvent
	svc.SetWebhookHandler(func(_ context.Context, event *contracts.WebhookEvent) error {
		handledEvent = event
		return nil
	})

	svc.ReconcileFiatOrders(context.Background())

	require.NotNil(t, handledEvent)
	assert.Equal(t, "CAP-RECON-001", handledEvent.PaymentID)
	assert.Equal(t, "order_reconcile_capture", handledEvent.OrderID)
	assert.Equal(t, "paypal", handledEvent.ProviderID)
}

func TestFiatService_ReconcileFiatOrders_PendingPayment_NoAction(t *testing.T) {
	provider := &mockFiatProvider{
		id: "stripe",
		getResult: &contracts.PaymentDetail{
			PaymentID: "pi_pending_001",
			Status:    "requires_payment_method",
			Amount:    3000,
			Currency:  "USD",
		},
	}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID:   "order_pending_recon",
		Open: true,
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				FiatMetadata: []byte(`{"fiat_provider":"stripe","fiat_session_id":"pi_pending_001"}`),
			},
		},
	}, models.OrderState_AWAITING_PAYMENT)

	var handlerCalled bool
	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		handlerCalled = true
		return nil
	})

	svc.ReconcileFiatOrders(context.Background())

	assert.False(t, handlerCalled, "webhook handler should NOT be called for non-succeeded payment")
}

func TestFiatService_ReconcileFiatOrders_NoOrders_NoAction(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestServiceWithOrders(t, reg)

	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		t.Fatal("webhook handler should not be called when no orders exist")
		return nil
	})

	svc.ReconcileFiatOrders(context.Background())
}

func TestFiatService_ReconcileFiatOrders_ProviderAPIError_ContinuesOthers(t *testing.T) {
	provider := &mockFiatProvider{
		id:     "stripe",
		getErr: errors.New("stripe API timeout"),
	}
	reg := newMockFiatRegistry()
	reg.Register(provider)

	svc, db := newFiatTestServiceWithOrders(t, reg)

	seedFiatOrder(t, db, &models.Order{
		ID:   "order_api_err",
		Open: true,
		OrderPaymentState: models.OrderPaymentState{
			FiatPaymentState: models.FiatPaymentState{
				FiatMetadata: []byte(`{"fiat_provider":"stripe","fiat_session_id":"pi_api_err"}`),
			},
		},
	}, models.OrderState_AWAITING_PAYMENT)

	svc.SetWebhookHandler(func(_ context.Context, _ *contracts.WebhookEvent) error {
		t.Fatal("webhook handler should not be called on API error")
		return nil
	})

	svc.ReconcileFiatOrders(context.Background())
}

func TestFiatService_ReconcileFiatOrders_NoWebhookHandler_NoAction(t *testing.T) {
	reg := newMockFiatRegistry()
	svc, _ := newFiatTestServiceWithOrders(t, reg)

	svc.ReconcileFiatOrders(context.Background())
}
