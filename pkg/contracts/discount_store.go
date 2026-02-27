package contracts

import (
	"context"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// ValidateCodeResult holds the result of validating a discount code.
type ValidateCodeResult struct {
	Valid    bool                  `json:"valid"`
	Discount *models.Discount     `json:"discount,omitempty"`
	Code     *models.DiscountCode `json:"code,omitempty"`
	Reason   string               `json:"reason,omitempty"`
}

// DiscountService is the business-level interface for discount operations.
// Implemented by DiscountAppService in internal/core/.
type DiscountService interface {
	CreateDiscount(ctx context.Context, d *models.Discount) error
	GetDiscount(ctx context.Context, id string) (*models.Discount, error)
	ListDiscounts(ctx context.Context, filter DiscountFilter) ([]models.Discount, int64, error)
	UpdateDiscount(ctx context.Context, d *models.Discount) error
	DeleteDiscount(ctx context.Context, id string) error

	AddCodes(ctx context.Context, discountID string, codes []models.DiscountCode) error
	GenerateCodes(ctx context.Context, discountID string, count int, prefix string) ([]models.DiscountCode, error)
	ListCodes(ctx context.Context, discountID string) ([]models.DiscountCode, error)
	DeleteCode(ctx context.Context, codeID string) error

	ValidateCode(ctx context.Context, code string, customerPeerID string) (*ValidateCodeResult, error)
	GetApplicableDiscounts(ctx context.Context, productIDs []string) ([]models.Discount, error)
	RecordRedemption(ctx context.Context, discountID string, codeID *string, orderID, customerPeerID, discountAmount, currency string) error

	ListRedemptions(ctx context.Context, discountID string, page, pageSize int) ([]models.DiscountRedemption, int64, error)
}

// DiscountFilter holds query parameters for listing discounts.
type DiscountFilter struct {
	Status        *models.DiscountStatus
	Method        *models.DiscountMethod
	SearchTerm    string
	Page          int
	PageSize      int
	IncludeExpired bool
}

// DiscountStore abstracts discount persistence for both standalone and SaaS modes.
// Implementations handle tenant scoping internally (database.Tx injects tenantID on writes,
// read queries are pre-scoped). Callers need not pass tenantID explicitly.
type DiscountStore interface {
	// --- Discount CRUD ---

	// CreateDiscount inserts a new discount (and its codes if provided).
	CreateDiscount(ctx context.Context, d *models.Discount) error

	// GetDiscount loads a single discount by ID (excludes soft-deleted).
	GetDiscount(ctx context.Context, id string) (*models.Discount, error)

	// ListDiscounts returns discounts matching the filter, with total count for pagination.
	ListDiscounts(ctx context.Context, filter DiscountFilter) ([]models.Discount, int64, error)

	// UpdateDiscount performs a full update on the discount (excluding codes and usage_count).
	UpdateDiscount(ctx context.Context, d *models.Discount) error

	// SoftDeleteDiscount sets deleted_at on the discount. Codes and redemptions are preserved
	// for historical reference.
	SoftDeleteDiscount(ctx context.Context, id string) error

	// --- DiscountCode management ---

	// CreateCodes adds codes to an existing discount. Each code's codeHash must be unique
	// within the tenant (enforced by UNIQUE INDEX on code_hash).
	CreateCodes(ctx context.Context, codes []models.DiscountCode) error

	// ListCodes returns all codes for a discount.
	ListCodes(ctx context.Context, discountID string) ([]models.DiscountCode, error)

	// DeleteCode removes a single code by ID.
	DeleteCode(ctx context.Context, codeID string) error

	// FindCodeByHash looks up a code by its tenant-scoped hash.
	// Hash = SHA256(tenantID + ":" + lowercase(code)).
	FindCodeByHash(ctx context.Context, codeHash string) (*models.DiscountCode, error)

	// --- Usage tracking ---

	// IncrementUsageWithCheck atomically increments usage_count on both the discount and
	// the code (if non-nil). Returns ErrUsageLimitReached if the limit would be exceeded.
	// Uses a single UPDATE ... WHERE usage_count < usage_limit (or usage_limit = 0) pattern.
	IncrementUsageWithCheck(ctx context.Context, discountID string, codeID *string) error

	// CountCustomerRedemptions returns how many times a customer has redeemed a specific discount.
	CountCustomerRedemptions(ctx context.Context, discountID, customerPeerID string) (int64, error)

	// CreateRedemption records a discount usage.
	CreateRedemption(ctx context.Context, r *models.DiscountRedemption) error

	// ListRedemptions returns redemption records for a discount, ordered by redeemed_at desc.
	ListRedemptions(ctx context.Context, discountID string, page, pageSize int) ([]models.DiscountRedemption, int64, error)

	// --- Query helpers ---

	// GetApplicableDiscounts returns active automatic discounts that could apply to the given
	// product IDs. Used by the buyer-facing /v1/discounts/applicable endpoint and DiscountEngine.
	GetApplicableDiscounts(ctx context.Context, productIDs []string) ([]models.Discount, error)

	// CountDiscounts returns the total number of non-deleted discounts for quota enforcement.
	CountDiscounts(ctx context.Context) (int64, error)
}
