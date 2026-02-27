package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDiscountStore is a simple in-memory mock for unit testing DiscountAppService.
type mockDiscountStore struct {
	discounts   map[string]*models.Discount
	codes       map[string]*models.DiscountCode
	redemptions []models.DiscountRedemption

	createErr error
	getErr    error
}

func newMockStore() *mockDiscountStore {
	return &mockDiscountStore{
		discounts: make(map[string]*models.Discount),
		codes:     make(map[string]*models.DiscountCode),
	}
}

func (m *mockDiscountStore) CreateDiscount(_ context.Context, d *models.Discount) error {
	if m.createErr != nil {
		return m.createErr
	}
	cp := *d
	m.discounts[d.ID] = &cp
	for i := range d.Codes {
		c := d.Codes[i]
		m.codes[c.ID] = &c
	}
	return nil
}

func (m *mockDiscountStore) GetDiscount(_ context.Context, id string) (*models.Discount, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	d, ok := m.discounts[id]
	if !ok {
		return nil, errors.New("discount not found")
	}
	cp := *d
	var codeCopies []models.DiscountCode
	for _, c := range m.codes {
		if c.DiscountID == id {
			codeCopies = append(codeCopies, *c)
		}
	}
	cp.Codes = codeCopies
	return &cp, nil
}

func (m *mockDiscountStore) ListDiscounts(_ context.Context, _ contracts.DiscountFilter) ([]models.Discount, int64, error) {
	var result []models.Discount
	for _, d := range m.discounts {
		if d.DeletedAt == nil {
			result = append(result, *d)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockDiscountStore) UpdateDiscount(_ context.Context, d *models.Discount) error {
	cp := *d
	m.discounts[d.ID] = &cp
	return nil
}

func (m *mockDiscountStore) SoftDeleteDiscount(_ context.Context, id string) error {
	d, ok := m.discounts[id]
	if !ok {
		return errors.New("not found")
	}
	now := time.Now()
	d.DeletedAt = &now
	return nil
}

func (m *mockDiscountStore) CreateCodes(_ context.Context, codes []models.DiscountCode) error {
	for i := range codes {
		c := codes[i]
		m.codes[c.ID] = &c
	}
	return nil
}

func (m *mockDiscountStore) ListCodes(_ context.Context, discountID string) ([]models.DiscountCode, error) {
	var result []models.DiscountCode
	for _, c := range m.codes {
		if c.DiscountID == discountID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *mockDiscountStore) DeleteCode(_ context.Context, codeID string) error {
	delete(m.codes, codeID)
	return nil
}

func (m *mockDiscountStore) FindCodeByHash(_ context.Context, codeHash string) (*models.DiscountCode, error) {
	for _, c := range m.codes {
		if c.CodeHash == codeHash {
			cp := *c
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockDiscountStore) IncrementUsageWithCheck(_ context.Context, discountID string, codeID *string) error {
	d, ok := m.discounts[discountID]
	if !ok {
		return errors.New("not found")
	}
	if d.UsageLimit > 0 && d.UsageCount >= d.UsageLimit {
		return errors.New("usage limit reached")
	}
	d.UsageCount++

	if codeID != nil && *codeID != "" {
		c, ok := m.codes[*codeID]
		if !ok {
			return errors.New("code not found")
		}
		if c.UsageLimit > 0 && c.UsageCount >= c.UsageLimit {
			return errors.New("usage limit reached")
		}
		c.UsageCount++
	}
	return nil
}

func (m *mockDiscountStore) CountCustomerRedemptions(_ context.Context, discountID, customerPeerID string) (int64, error) {
	var count int64
	for _, r := range m.redemptions {
		if r.DiscountID == discountID && r.CustomerPeerID == customerPeerID {
			count++
		}
	}
	return count, nil
}

func (m *mockDiscountStore) CreateRedemption(_ context.Context, r *models.DiscountRedemption) error {
	m.redemptions = append(m.redemptions, *r)
	return nil
}

func (m *mockDiscountStore) ListRedemptions(_ context.Context, discountID string, _, _ int) ([]models.DiscountRedemption, int64, error) {
	var result []models.DiscountRedemption
	for _, r := range m.redemptions {
		if r.DiscountID == discountID {
			result = append(result, r)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockDiscountStore) GetApplicableDiscounts(_ context.Context, _ []string) ([]models.Discount, error) {
	var result []models.Discount
	for _, d := range m.discounts {
		if d.Method == models.DiscountMethodAutomatic && d.DeletedAt == nil {
			result = append(result, *d)
		}
	}
	return result, nil
}

func (m *mockDiscountStore) CountDiscounts(_ context.Context) (int64, error) {
	var count int64
	for _, d := range m.discounts {
		if d.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func TestDiscountAppService_CreateDiscount_ValidPercentage(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "tenant1")
	ctx := context.Background()

	d := &models.Discount{
		Title:     "10% Off",
		Method:    models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage,
		Value:     "10",
		StartsAt:  time.Now(),
	}
	require.NoError(t, svc.CreateDiscount(ctx, d))
	assert.NotEmpty(t, d.ID)
	assert.Equal(t, "tenant1", d.TenantID)
}

func TestDiscountAppService_CreateDiscount_InvalidPercentage(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")
	ctx := context.Background()

	tests := []struct {
		name  string
		value string
	}{
		{"zero", "0"},
		{"hundred", "100"},
		{"negative", "-5"},
		{"non-numeric", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &models.Discount{
				Title: "Bad", Method: models.DiscountMethodCode,
				ValueType: models.DiscountValuePercentage, Value: tt.value,
				StartsAt: time.Now(),
			}
			err := svc.CreateDiscount(ctx, d)
			assert.Error(t, err)
		})
	}
}

func TestDiscountAppService_CreateDiscount_FixedAmountNeedsCurrency(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")
	ctx := context.Background()

	d := &models.Discount{
		Title: "500 off", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValueFixed, Value: "500",
		StartsAt: time.Now(),
	}
	err := svc.CreateDiscount(ctx, d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency")

	d.Currency = "USD"
	require.NoError(t, svc.CreateDiscount(ctx, d))
}

func TestDiscountAppService_CreateDiscount_TitleRequired(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")

	d := &models.Discount{
		Title: "", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage, Value: "10",
		StartsAt: time.Now(),
	}
	err := svc.CreateDiscount(context.Background(), d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestDiscountAppService_CreateDiscount_EndsAtMustBeAfterStartsAt(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")

	now := time.Now()
	endsAt := now.Add(-time.Hour)
	d := &models.Discount{
		Title: "Bad Date", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage, Value: "10",
		StartsAt: now, EndsAt: &endsAt,
	}
	err := svc.CreateDiscount(context.Background(), d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endsAt")
}

func TestDiscountAppService_ComputeStatus_Scheduled(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")

	future := time.Now().Add(24 * time.Hour)
	d := &models.Discount{
		Title: "Future", Method: models.DiscountMethodAutomatic,
		ValueType: models.DiscountValuePercentage, Value: "5",
		StartsAt: future,
	}
	require.NoError(t, svc.CreateDiscount(context.Background(), d))
	assert.Equal(t, models.DiscountStatusScheduled, d.Status)
}

func TestDiscountAppService_ComputeStatus_Active(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")

	d := &models.Discount{
		Title: "Active", Method: models.DiscountMethodAutomatic,
		ValueType: models.DiscountValuePercentage, Value: "5",
		StartsAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, svc.CreateDiscount(context.Background(), d))
	assert.Equal(t, models.DiscountStatusActive, d.Status)
}

func TestDiscountAppService_ValidateCode_Valid(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "tenant1")
	ctx := context.Background()

	d := &models.Discount{
		Title: "10% Off", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage, Value: "10",
		StartsAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, svc.CreateDiscount(ctx, d))

	codes := []models.DiscountCode{
		{Code: "SAVE10"},
	}
	require.NoError(t, svc.AddCodes(ctx, d.ID, codes))

	result, err := svc.ValidateCode(ctx, "SAVE10", "")
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.NotNil(t, result.Discount)
}

func TestDiscountAppService_ValidateCode_CaseInsensitive(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "tenant1")
	ctx := context.Background()

	d := &models.Discount{
		Title: "Discount", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage, Value: "5",
		StartsAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, svc.CreateDiscount(ctx, d))

	codes := []models.DiscountCode{{Code: "MyCode"}}
	require.NoError(t, svc.AddCodes(ctx, d.ID, codes))

	result, err := svc.ValidateCode(ctx, "mycode", "")
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestDiscountAppService_ValidateCode_Invalid(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")

	result, err := svc.ValidateCode(context.Background(), "NOPE", "")
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Equal(t, "INVALID", result.Reason)
}

func TestDiscountAppService_ValidateCode_Expired(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")
	ctx := context.Background()

	endedAt := time.Now().Add(-time.Hour)
	d := &models.Discount{
		Title: "Expired", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage, Value: "10",
		StartsAt: time.Now().Add(-24 * time.Hour), EndsAt: &endedAt,
		Status: models.DiscountStatusExpired,
	}
	require.NoError(t, svc.CreateDiscount(ctx, d))

	codes := []models.DiscountCode{{Code: "EXPIRED"}}
	require.NoError(t, svc.AddCodes(ctx, d.ID, codes))

	result, err := svc.ValidateCode(ctx, "EXPIRED", "")
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Equal(t, "EXPIRED", result.Reason)
}

func TestDiscountAppService_GenerateCodes(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")
	ctx := context.Background()

	d := &models.Discount{
		Title: "Promo", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage, Value: "15",
		StartsAt: time.Now(),
	}
	require.NoError(t, svc.CreateDiscount(ctx, d))

	codes, err := svc.GenerateCodes(ctx, d.ID, 5, "PROMO")
	require.NoError(t, err)
	assert.Len(t, codes, 5)
	for _, c := range codes {
		assert.NotEmpty(t, c.Code)
		assert.True(t, len(c.Code) > 4)
		assert.NotEmpty(t, c.CodeHash)
	}
}

func TestDiscountAppService_RecordRedemption(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")
	ctx := context.Background()

	d := &models.Discount{
		Title: "Promo", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage, Value: "10",
		StartsAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, svc.CreateDiscount(ctx, d))

	err := svc.RecordRedemption(ctx, d.ID, nil, "order123", "buyer1", "500", "USD")
	require.NoError(t, err)

	redemptions, total, err := svc.ListRedemptions(ctx, d.ID, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "order123", redemptions[0].OrderID)
}

func TestDiscountAppService_TenantQuota(t *testing.T) {
	store := newMockStore()
	svc := NewDiscountAppService(store, "t1")
	ctx := context.Background()

	for i := 0; i < maxDiscountsPerTenant; i++ {
		d := &models.Discount{
			Title: "D", Method: models.DiscountMethodCode,
			ValueType: models.DiscountValuePercentage, Value: "10",
			StartsAt: time.Now(),
		}
		require.NoError(t, svc.CreateDiscount(ctx, d))
	}

	d := &models.Discount{
		Title: "Over Limit", Method: models.DiscountMethodCode,
		ValueType: models.DiscountValuePercentage, Value: "10",
		StartsAt: time.Now(),
	}
	err := svc.CreateDiscount(ctx, d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum")
}
