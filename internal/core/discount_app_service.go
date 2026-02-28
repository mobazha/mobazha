package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

const (
	maxDiscountsPerTenant = 500
	maxCodesPerDiscount   = 1000
	maxBulkGenerateCount  = 500
	generatedCodeLength   = 8
)

type DiscountAppService struct {
	store    contracts.DiscountStore
	tenantID string
}

func NewDiscountAppService(store contracts.DiscountStore, tenantID string) *DiscountAppService {
	return &DiscountAppService{
		store:    store,
		tenantID: tenantID,
	}
}

// Store returns the underlying DiscountStore for engine wiring (e.g., hosting
// constructs a DiscountEngine with the vendor's store for cross-tenant resolution).
func (s *DiscountAppService) Store() contracts.DiscountStore {
	return s.store
}

func (s *DiscountAppService) CreateDiscount(ctx context.Context, d *models.Discount) error {
	if err := s.validateDiscount(d); err != nil {
		return err
	}

	count, err := s.store.CountDiscounts(ctx)
	if err != nil {
		return fmt.Errorf("count discounts: %w", err)
	}
	if count >= maxDiscountsPerTenant {
		return fmt.Errorf("maximum discounts (%d) reached", maxDiscountsPerTenant)
	}

	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	d.TenantID = s.tenantID
	d.Status = s.computeInitialStatus(d)

	for i := range d.Codes {
		if d.Codes[i].ID == "" {
			d.Codes[i].ID = uuid.New().String()
		}
		d.Codes[i].DiscountID = d.ID
		d.Codes[i].CodeHash = s.hashCode(d.Codes[i].Code)
	}

	return s.store.CreateDiscount(ctx, d)
}

func (s *DiscountAppService) GetDiscount(ctx context.Context, id string) (*models.Discount, error) {
	return s.store.GetDiscount(ctx, id)
}

func (s *DiscountAppService) ListDiscounts(ctx context.Context, filter contracts.DiscountFilter) ([]models.Discount, int64, error) {
	return s.store.ListDiscounts(ctx, filter)
}

func (s *DiscountAppService) UpdateDiscount(ctx context.Context, d *models.Discount) error {
	if err := s.validateDiscount(d); err != nil {
		return err
	}
	d.Status = s.computeInitialStatus(d)
	return s.store.UpdateDiscount(ctx, d)
}

func (s *DiscountAppService) DeleteDiscount(ctx context.Context, id string) error {
	return s.store.SoftDeleteDiscount(ctx, id)
}

// AddCodes adds explicit codes to a discount.
func (s *DiscountAppService) AddCodes(ctx context.Context, discountID string, codes []models.DiscountCode) error {
	existing, err := s.store.ListCodes(ctx, discountID)
	if err != nil {
		return err
	}
	if len(existing)+len(codes) > maxCodesPerDiscount {
		return fmt.Errorf("maximum codes per discount (%d) would be exceeded", maxCodesPerDiscount)
	}

	for i := range codes {
		if codes[i].ID == "" {
			codes[i].ID = uuid.New().String()
		}
		codes[i].DiscountID = discountID
		codes[i].CodeHash = s.hashCode(codes[i].Code)
	}
	return s.store.CreateCodes(ctx, codes)
}

// GenerateCodes creates random unique codes with an optional prefix.
func (s *DiscountAppService) GenerateCodes(ctx context.Context, discountID string, count int, prefix string) ([]models.DiscountCode, error) {
	if count <= 0 || count > maxBulkGenerateCount {
		return nil, fmt.Errorf("count must be between 1 and %d", maxBulkGenerateCount)
	}

	existing, err := s.store.ListCodes(ctx, discountID)
	if err != nil {
		return nil, err
	}
	if len(existing)+count > maxCodesPerDiscount {
		return nil, fmt.Errorf("maximum codes per discount (%d) would be exceeded", maxCodesPerDiscount)
	}

	codes := make([]models.DiscountCode, 0, count)
	for i := 0; i < count; i++ {
		code := prefix + randomAlphanumeric(generatedCodeLength)
		codes = append(codes, models.DiscountCode{
			ID:         uuid.New().String(),
			DiscountID: discountID,
			Code:       code,
			CodeHash:   s.hashCode(code),
		})
	}
	if err := s.store.CreateCodes(ctx, codes); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *DiscountAppService) ListCodes(ctx context.Context, discountID string) ([]models.DiscountCode, error) {
	return s.store.ListCodes(ctx, discountID)
}

func (s *DiscountAppService) DeleteCode(ctx context.Context, codeID string) error {
	return s.store.DeleteCode(ctx, codeID)
}

func (s *DiscountAppService) ListRedemptions(ctx context.Context, discountID string, page, pageSize int) ([]models.DiscountRedemption, int64, error) {
	return s.store.ListRedemptions(ctx, discountID, page, pageSize)
}

// ValidateCode checks if a discount code is valid for use.
// customerPeerID is optional — when provided, perCustomerLimit is enforced.
func (s *DiscountAppService) ValidateCode(ctx context.Context, code string, customerPeerID string) (*contracts.ValidateCodeResult, error) {
	codeHash := s.hashCode(code)
	dc, err := s.store.FindCodeByHash(ctx, codeHash)
	if err != nil {
		return nil, err
	}
	if dc == nil {
		return &contracts.ValidateCodeResult{Valid: false, Reason: "INVALID"}, nil
	}

	discount, err := s.store.GetDiscount(ctx, dc.DiscountID)
	if err != nil {
		return &contracts.ValidateCodeResult{Valid: false, Reason: "INVALID"}, nil
	}

	if reason := s.checkDiscountValidity(ctx, discount, dc, customerPeerID); reason != "" {
		return &contracts.ValidateCodeResult{Valid: false, Discount: discount, Code: dc, Reason: reason}, nil
	}

	return &contracts.ValidateCodeResult{Valid: true, Discount: discount, Code: dc}, nil
}

// GetApplicableDiscounts returns active automatic discounts (public endpoint).
func (s *DiscountAppService) GetApplicableDiscounts(ctx context.Context, productIDs []string) ([]models.Discount, error) {
	return s.store.GetApplicableDiscounts(ctx, productIDs)
}

// RecordRedemption atomically increments usage and records a redemption.
func (s *DiscountAppService) RecordRedemption(ctx context.Context, discountID string, codeID *string, orderID, customerPeerID, discountAmount, currency string) error {
	if err := s.store.IncrementUsageWithCheck(ctx, discountID, codeID); err != nil {
		return err
	}

	r := &models.DiscountRedemption{
		ID:             uuid.New().String(),
		DiscountID:     discountID,
		CodeID:         codeID,
		OrderID:        orderID,
		CustomerPeerID: customerPeerID,
		DiscountAmount: discountAmount,
		Currency:       currency,
		RedeemedAt:     time.Now(),
	}
	return s.store.CreateRedemption(ctx, r)
}

// --- internal helpers ---

func (s *DiscountAppService) validateDiscount(d *models.Discount) error {
	if strings.TrimSpace(d.Title) == "" {
		return errors.New("title is required")
	}
	if d.Method != models.DiscountMethodCode && d.Method != models.DiscountMethodAutomatic {
		return fmt.Errorf("invalid method: %s", d.Method)
	}

	switch d.ValueType {
	case models.DiscountValuePercentage:
		v, ok := new(big.Int).SetString(d.Value, 10)
		if !ok || v.Cmp(big.NewInt(1)) < 0 || v.Cmp(big.NewInt(99)) > 0 {
			return errors.New("percentage value must be between 1 and 99")
		}
	case models.DiscountValueFixed:
		v, ok := new(big.Int).SetString(d.Value, 10)
		if !ok || v.Cmp(big.NewInt(0)) <= 0 {
			return errors.New("fixed amount must be positive")
		}
		if d.Currency == "" {
			return errors.New("currency is required for fixed amount discounts")
		}
	case models.DiscountValueFreeShipping:
		// no value validation needed
	default:
		return fmt.Errorf("invalid value type: %s", d.ValueType)
	}

	if d.StartsAt.IsZero() {
		return errors.New("startsAt is required")
	}
	if d.EndsAt != nil && !d.EndsAt.IsZero() && d.EndsAt.Before(d.StartsAt) {
		return errors.New("endsAt must be after startsAt")
	}
	return nil
}

func (s *DiscountAppService) computeInitialStatus(d *models.Discount) models.DiscountStatus {
	if d.Status == models.DiscountStatusDraft || d.Status == models.DiscountStatusExpired {
		return d.Status
	}
	now := time.Now()
	if d.EndsAt != nil && !d.EndsAt.IsZero() && d.EndsAt.Before(now) {
		return models.DiscountStatusExpired
	}
	if d.StartsAt.After(now) {
		return models.DiscountStatusScheduled
	}
	return models.DiscountStatusActive
}

func (s *DiscountAppService) checkDiscountValidity(ctx context.Context, d *models.Discount, dc *models.DiscountCode, customerPeerID string) string {
	now := time.Now()

	if d.Status == models.DiscountStatusExpired {
		return "EXPIRED"
	}
	if d.Status == models.DiscountStatusDraft {
		return "INVALID"
	}
	if d.Status == models.DiscountStatusScheduled || d.StartsAt.After(now) {
		return "NOT_STARTED"
	}
	if d.EndsAt != nil && !d.EndsAt.IsZero() && d.EndsAt.Before(now) {
		return "EXPIRED"
	}
	if d.UsageLimit > 0 && d.UsageCount >= d.UsageLimit {
		return "USAGE_LIMIT_REACHED"
	}
	if dc != nil && dc.UsageLimit > 0 && dc.UsageCount >= dc.UsageLimit {
		return "USAGE_LIMIT_REACHED"
	}
	if customerPeerID != "" && d.PerCustomerLimit > 0 {
		count, err := s.store.CountCustomerRedemptions(ctx, d.ID, customerPeerID)
		if err == nil && count >= int64(d.PerCustomerLimit) {
			return "CUSTOMER_LIMIT_REACHED"
		}
	}
	return ""
}

func (s *DiscountAppService) hashCode(code string) string {
	input := s.tenantID + ":" + strings.ToLower(code)
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

func randomAlphanumeric(n int) string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = chars[b[i]%byte(len(chars))]
	}
	return string(b)
}
