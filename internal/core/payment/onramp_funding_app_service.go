// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
)

// Typed errors for onramp funding orchestration.
var (
	// ErrOnrampAttemptNotFound: no payment attempt for the tenant/attempt id.
	ErrOnrampAttemptNotFound = errors.New("onramp funding: payment attempt not found")
	// ErrOnrampAttemptNotReady: the attempt has no frozen, payable funding
	// target yet (RFC-0009: terms freeze before anything is payable).
	ErrOnrampAttemptNotReady = errors.New("onramp funding: attempt funding target is not frozen/payable")
)

// OnrampFundingAppService orchestrates onramp purchases against frozen payment
// attempts (ADR-019): it enforces the frozen-terms gate, delegates to a
// reviewed OnrampProvider, and maintains the durable
// PaymentAttemptOnrampFundingSource history that the session projector reads.
// It never touches settlement: delivery lands at the frozen funding target and
// resolves through the ordinary on-chain observation pipeline.
type OnrampFundingAppService struct {
	db        database.Database
	providers contracts.OnrampProviderRegistry
}

// NewOnrampFundingAppService builds the app service.
func NewOnrampFundingAppService(db database.Database, providers contracts.OnrampProviderRegistry) *OnrampFundingAppService {
	return &OnrampFundingAppService{db: db, providers: providers}
}

// InitiateOnrampFundingRequest asks to fund one frozen attempt via an onramp
// purchase. IdempotencyKey defaults to "primary": a buyer who leaves and
// returns resumes the same purchase; a retry after a terminal failure supplies
// a new key.
type InitiateOnrampFundingRequest struct {
	TenantID  string
	OrderID   string
	AttemptID string

	Buyer        contracts.BuyerRef
	ProviderID   string
	FiatCurrency string
	ClientIP     string

	IdempotencyKey string

	// DeliverToBuyerWallet routes delivery to the buyer's embedded wallet for a
	// later forwarding step instead of directly to the funding target.
	DeliverToBuyerWallet bool
	BuyerWalletAddress   string
}

// Validate rejects requests missing identity or provider coordinates.
func (r InitiateOnrampFundingRequest) Validate() error {
	if strings.TrimSpace(r.OrderID) == "" || strings.TrimSpace(r.AttemptID) == "" {
		return fmt.Errorf("onramp funding: order and attempt are required")
	}
	if strings.TrimSpace(r.ProviderID) == "" || strings.TrimSpace(r.FiatCurrency) == "" {
		return fmt.Errorf("onramp funding: provider and fiat currency are required")
	}
	return r.Buyer.Validate()
}

// InitiateOrResume creates the attempt's onramp purchase or resumes the
// existing one. Idempotent on (attempt, idempotency key) at both layers. An
// awaiting-payment resume calls the same provider purchase again so adapters
// with short-lived, single-use action URLs can issue a fresh session without
// creating a second provider order.
func (s *OnrampFundingAppService) InitiateOrResume(ctx context.Context, req InitiateOnrampFundingRequest) (*payment.OnrampFundingSourceView, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey == "" {
		idemKey = "primary"
	}
	requestedFiatCurrency := normalizeFiatCurrency(req.FiatCurrency)

	// Frozen-terms gate: the attempt must expose a verified, immutable funding
	// target before any purchase is initiated.
	attempt, target, err := s.loadFrozenAttempt(req.TenantID, req.OrderID, req.AttemptID)
	if err != nil {
		return nil, err
	}

	settlementAmount, err := frozenSettlementAmount(target)
	if err != nil {
		return nil, err
	}

	// Resume: the durable provider/order binding wins. Awaiting-payment action
	// URLs are reissued because providers such as CDP make them short-lived and
	// single-use; later lifecycle states are status-polled instead.
	if existing := s.findByIdempotencyKey(req.TenantID, req.AttemptID, idemKey); existing != nil {
		if existing.Status != string(contracts.OnrampStatusCreated) &&
			existing.Status != string(contracts.OnrampStatusAwaitingPayment) {
			return s.refreshRecord(ctx, existing), nil
		}
		provider, err := s.providers.ForProvider(existing.ProviderID)
		if err != nil {
			return nil, err
		}
		fiatCurrency, err := s.pinResumeFiatCurrency(existing, requestedFiatCurrency)
		if err != nil {
			return nil, err
		}
		purchase, err := provider.InitiatePurchase(ctx, contracts.OnrampPurchaseRequest{
			Buyer:                req.Buyer,
			OrderID:              req.OrderID,
			AttemptID:            req.AttemptID,
			RailID:               attempt.Currency,
			SettlementAsset:      target.AssetID,
			SettlementAmount:     settlementAmount,
			FiatCurrency:         fiatCurrency,
			ClientIP:             req.ClientIP,
			DeliveryTarget:       target.Address,
			DeliverToBuyerWallet: existing.DeliverToBuyerWallet,
			BuyerWalletAddress:   existing.BuyerWalletAddress,
			IdempotencyKey:       idemKey,
		})
		if err != nil {
			return nil, err
		}
		if err := validateResumedPurchase(existing, purchase); err != nil {
			return nil, err
		}
		if err := s.persistResumedPurchase(existing, purchase); err != nil {
			return nil, err
		}
		view := onrampSourceView(existing)
		return &view, nil
	}

	provider, err := s.providers.ForProvider(req.ProviderID)
	if err != nil {
		return nil, err
	}
	// Capability gate (RFC-0012 Proposal 6): the rail must be offerable, and
	// direct-to-target delivery must be proven when requested.
	caps, err := provider.Capabilities(ctx, attempt.Currency)
	if err != nil {
		return nil, err
	}
	if !caps.Offerable {
		return nil, contracts.ErrOnrampCapabilityClosed
	}
	if !req.DeliverToBuyerWallet && !caps.DeliverToTarget {
		return nil, fmt.Errorf("%w: provider %s cannot deliver to the funding target on %s", contracts.ErrOnrampCapabilityClosed, req.ProviderID, attempt.Currency)
	}

	purchase, err := provider.InitiatePurchase(ctx, contracts.OnrampPurchaseRequest{
		Buyer:                req.Buyer,
		OrderID:              req.OrderID,
		AttemptID:            req.AttemptID,
		RailID:               attempt.Currency,
		SettlementAsset:      target.AssetID,
		SettlementAmount:     settlementAmount,
		FiatCurrency:         requestedFiatCurrency,
		ClientIP:             req.ClientIP,
		DeliveryTarget:       target.Address,
		DeliverToBuyerWallet: req.DeliverToBuyerWallet,
		BuyerWalletAddress:   req.BuyerWalletAddress,
		IdempotencyKey:       idemKey,
	})
	if err != nil {
		return nil, err
	}

	row := &models.PaymentAttemptOnrampFundingSource{
		TenantID:             req.TenantID,
		AttemptID:            req.AttemptID,
		OnrampOrderID:        purchase.OnrampOrderID,
		OrderID:              req.OrderID,
		ProviderID:           purchase.ProviderID,
		FiatCurrency:         requestedFiatCurrency,
		IdempotencyKey:       idemKey,
		DeliveryTarget:       purchase.DeliveryTarget,
		DeliverToBuyerWallet: purchase.DeliverToBuyerWallet,
		BuyerWalletAddress:   purchase.BuyerWalletAddress,
		BuyerActionURL:       purchase.BuyerActionURL,
		Disclosure:           purchase.Disclosure,
	}
	row.SetStatus(string(purchase.Status))

	// Table creation belongs to MigrateFiatModels at repo init; a missing
	// table here must fail loudly rather than be papered over per-call.
	if err := s.db.Update(func(tx database.Tx) error {
		return tx.Create(row)
	}); err != nil {
		// A concurrent initiate can lose the unique race (idempotency or
		// single-active index). The stored record is then authoritative.
		if existing := s.findByIdempotencyKey(req.TenantID, req.AttemptID, idemKey); existing != nil {
			view := onrampSourceView(existing)
			return &view, nil
		}
		return nil, err
	}
	view := onrampSourceView(row)
	return &view, nil
}

func normalizeFiatCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

// pinResumeFiatCurrency returns the commercial currency frozen by the first
// initiate. The conditional update only exists for rows created before the
// fiat_currency column was introduced; it also makes the first legacy resume
// deterministic if two clients race with different locale defaults.
func (s *OnrampFundingAppService) pinResumeFiatCurrency(row *models.PaymentAttemptOnrampFundingSource, requested string) (string, error) {
	if stored := normalizeFiatCurrency(row.FiatCurrency); stored != "" {
		return stored, nil
	}
	var updated int64
	err := s.db.Update(func(tx database.Tx) error {
		var err error
		updated, err = tx.UpdateColumns(map[string]interface{}{
			"fiat_currency": requested,
		}, map[string]interface{}{
			"tenant_id = ?":       row.TenantID,
			"attempt_id = ?":      row.AttemptID,
			"onramp_order_id = ?": row.OnrampOrderID,
			"fiat_currency = ?":   "",
		}, &models.PaymentAttemptOnrampFundingSource{})
		return err
	})
	if err != nil {
		return "", fmt.Errorf("pin onramp fiat currency: %w", err)
	}
	if updated == 1 {
		row.FiatCurrency = requested
		return requested, nil
	}
	// Another legacy resume won the conditional update. Reload the durable row
	// instead of allowing this request's input to overwrite that decision.
	if durable := s.findByIdempotencyKey(row.TenantID, row.AttemptID, row.IdempotencyKey); durable != nil {
		if stored := normalizeFiatCurrency(durable.FiatCurrency); stored != "" {
			row.FiatCurrency = stored
			return stored, nil
		}
	}
	return "", fmt.Errorf("pin onramp fiat currency: durable record is unavailable")
}

// RefreshStatus polls the provider for the attempt's in-flight purchase and
// persists the transition. When nothing is in flight it returns the current
// selection (a delivered-to-wallet record pending forwarding, or nil).
func (s *OnrampFundingAppService) RefreshStatus(ctx context.Context, tenantID, attemptID string) (*payment.OnrampFundingSourceView, error) {
	rows := s.loadHistory(tenantID, attemptID)
	if len(rows) == 0 {
		return nil, nil
	}
	var active *models.PaymentAttemptOnrampFundingSource
	for i := range rows {
		if rows[i].IsActive() {
			if active == nil || rows[i].UpdatedAt.After(active.UpdatedAt) {
				active = &rows[i]
			}
		}
	}
	if active != nil {
		// Return the transition that was just polled even when it became
		// terminal. Otherwise direct-to-target delivered/failed transitions
		// collapse to null before clients can stop polling or render the result.
		return s.refreshRecord(ctx, active), nil
	}
	views := make([]payment.OnrampFundingSourceView, 0, len(rows))
	for i := range rows {
		views = append(views, onrampSourceView(&rows[i]))
	}
	if selected := payment.SelectOnrampFundingSource(views); selected != nil {
		return selected, nil
	}
	// Refresh is a lifecycle endpoint, not the session projection: once a
	// purchase exists, return its latest durable terminal record so another
	// client cannot consume the one transition and leave this caller with null.
	latest := onrampSourceView(&rows[0])
	return &latest, nil
}

// refreshRecord best-effort polls the provider and persists a status change.
// On any provider error the stored record stands (the projection stays
// truthful to the last durable fact). The write uses an explicit column update
// keyed on the full primary key: gorm's Save would degrade to INSERT whenever
// a composite-PK component is zero-valued — which tenant_id legitimately is in
// standalone deployments — and silently fail on conflict.
func (s *OnrampFundingAppService) refreshRecord(ctx context.Context, row *models.PaymentAttemptOnrampFundingSource) *payment.OnrampFundingSourceView {
	if row.IsActive() {
		if provider, err := s.providers.ForProvider(row.ProviderID); err == nil {
			if purchase, err := provider.PurchaseStatus(ctx, row.OnrampOrderID); err == nil &&
				string(purchase.Status) != row.Status {
				row.SetStatus(string(purchase.Status))
				row.UpdatedAt = time.Now().UTC()
				_ = s.db.Update(func(tx database.Tx) error {
					_, err := tx.UpdateColumns(map[string]interface{}{
						"status":     row.Status,
						"active":     row.Active,
						"updated_at": row.UpdatedAt,
					}, map[string]interface{}{
						"tenant_id = ?":       row.TenantID,
						"attempt_id = ?":      row.AttemptID,
						"onramp_order_id = ?": row.OnrampOrderID,
					}, &models.PaymentAttemptOnrampFundingSource{})
					return err
				})
			}
		}
	}
	view := onrampSourceView(row)
	return &view
}

// InitiateOrResumeForOrder implements contracts.OnrampFundingService: it
// resolves the order's tenant and current payable attempt, then delegates to
// InitiateOrResume.
func (s *OnrampFundingAppService) InitiateOrResumeForOrder(ctx context.Context, orderID string, req contracts.OnrampFundingInitiation) (*payment.OnrampFundingSourceView, error) {
	tenantID, attemptID, err := s.resolveOrderAttempt(orderID)
	if err != nil {
		return nil, err
	}
	return s.InitiateOrResume(ctx, InitiateOnrampFundingRequest{
		TenantID:             tenantID,
		OrderID:              orderID,
		AttemptID:            attemptID,
		Buyer:                req.Buyer,
		ProviderID:           req.ProviderID,
		FiatCurrency:         req.FiatCurrency,
		ClientIP:             req.ClientIP,
		IdempotencyKey:       req.IdempotencyKey,
		DeliverToBuyerWallet: req.DeliverToBuyerWallet,
		BuyerWalletAddress:   req.BuyerWalletAddress,
	})
}

// frozenSettlementAmount converts the canonical frozen atomic amount into the
// human-readable decimal required by hosted onramp APIs.
func frozenSettlementAmount(target *models.PaymentAttemptFundingTarget) (string, error) {
	raw := strings.TrimSpace(target.AmountAtomic)
	amount, ok := new(big.Int).SetString(raw, 10)
	if !ok || amount.Sign() <= 0 {
		return "", fmt.Errorf("onramp funding: invalid frozen atomic amount %q", target.AmountAtomic)
	}
	if _, ok := payment.SessionAmountDecimals(target.AssetID); !ok {
		return "", fmt.Errorf("onramp funding: unknown settlement asset divisibility for %s", target.AssetID)
	}
	return payment.FormatSessionAmount(raw, target.AssetID), nil
}

func validateResumedPurchase(existing *models.PaymentAttemptOnrampFundingSource, purchase contracts.OnrampPurchase) error {
	if purchase.ProviderID != existing.ProviderID || purchase.OnrampOrderID != existing.OnrampOrderID {
		return fmt.Errorf("onramp funding: provider violated resume idempotency")
	}
	if purchase.DeliveryTarget != existing.DeliveryTarget ||
		purchase.DeliverToBuyerWallet != existing.DeliverToBuyerWallet ||
		purchase.BuyerWalletAddress != existing.BuyerWalletAddress {
		return fmt.Errorf("onramp funding: provider changed the frozen delivery binding on resume")
	}
	return nil
}

func (s *OnrampFundingAppService) persistResumedPurchase(row *models.PaymentAttemptOnrampFundingSource, purchase contracts.OnrampPurchase) error {
	updates := map[string]interface{}{}
	if strings.TrimSpace(purchase.BuyerActionURL) != "" && purchase.BuyerActionURL != row.BuyerActionURL {
		row.BuyerActionURL = purchase.BuyerActionURL
		updates["buyer_action_url"] = row.BuyerActionURL
	}
	if strings.TrimSpace(purchase.Disclosure) != "" && purchase.Disclosure != row.Disclosure {
		row.Disclosure = purchase.Disclosure
		updates["disclosure"] = row.Disclosure
	}
	if len(updates) == 0 {
		return nil
	}
	row.UpdatedAt = time.Now().UTC()
	updates["updated_at"] = row.UpdatedAt
	return s.db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(updates, map[string]interface{}{
			"tenant_id = ?":       row.TenantID,
			"attempt_id = ?":      row.AttemptID,
			"onramp_order_id = ?": row.OnrampOrderID,
		}, &models.PaymentAttemptOnrampFundingSource{})
		return err
	})
}

// RefreshForOrder implements contracts.OnrampFundingService.
func (s *OnrampFundingAppService) RefreshForOrder(ctx context.Context, orderID string) (*payment.OnrampFundingSourceView, error) {
	tenantID, attemptID, err := s.resolveOrderAttempt(orderID)
	if err != nil {
		return nil, err
	}
	return s.RefreshStatus(ctx, tenantID, attemptID)
}

// ListProvidersForOrder implements contracts.OnrampFundingService: the
// discovery surface behind the buyer-visible onramp affordance. Providers are
// filtered through their own fail-closed Capabilities for the attempt's
// frozen rail, so an unproven rail yields an empty list, never an error.
func (s *OnrampFundingAppService) ListProvidersForOrder(ctx context.Context, orderID string) ([]payment.OnrampProviderOption, error) {
	tenantID, attemptID, err := s.resolveOrderAttempt(orderID)
	if err != nil {
		return nil, err
	}
	attempt, _, err := s.loadFrozenAttempt(tenantID, orderID, attemptID)
	if err != nil {
		return nil, err
	}
	options := make([]payment.OnrampProviderOption, 0)
	for _, id := range s.providers.Registered() {
		provider, err := s.providers.ForProvider(id)
		if err != nil {
			continue // unregistered between listing and lookup: skip, fail closed
		}
		caps, err := provider.Capabilities(ctx, attempt.Currency)
		if err != nil || !caps.Offerable {
			continue
		}
		options = append(options, payment.OnrampProviderOption{
			ProviderID:      id,
			RailID:          attempt.Currency,
			DeliverToTarget: caps.DeliverToTarget,
			FiatCurrencies:  caps.FiatCurrencies,
		})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ProviderID < options[j].ProviderID })
	return options, nil
}

// resolveOrderAttempt resolves an order to its tenant and its current payable
// (funding_target_ready) attempt — the newest one when several exist.
func (s *OnrampFundingAppService) resolveOrderAttempt(orderID string) (tenantID, attemptID string, err error) {
	if strings.TrimSpace(orderID) == "" {
		return "", "", ErrOnrampAttemptNotFound
	}
	var order models.Order
	orderFound := false
	var attempt models.PaymentAttempt
	attemptFound := false
	dbErr := s.db.View(func(tx database.Tx) error {
		res := tx.Read().Where("id = ?", orderID).Limit(1).Find(&order)
		if res.Error != nil {
			return res.Error
		}
		orderFound = res.RowsAffected > 0
		if !orderFound || !tx.Read().Migrator().HasTable(&models.PaymentAttempt{}) {
			return nil
		}
		res = tx.Read().
			Where("tenant_id = ? AND order_id = ? AND state = ?", order.TenantID, orderID, models.PaymentAttemptFundingTargetReady).
			Order("updated_at DESC").Limit(1).Find(&attempt)
		if res.Error != nil {
			return res.Error
		}
		attemptFound = res.RowsAffected > 0
		return nil
	})
	if dbErr != nil {
		return "", "", dbErr
	}
	if !orderFound {
		return "", "", ErrOnrampAttemptNotFound
	}
	if !attemptFound {
		return "", "", ErrOnrampAttemptNotReady
	}
	return order.TenantID, attempt.AttemptID, nil
}

var _ contracts.OnrampFundingService = (*OnrampFundingAppService)(nil)

// loadFrozenAttempt loads the attempt and enforces the frozen-target gate.
func (s *OnrampFundingAppService) loadFrozenAttempt(tenantID, orderID, attemptID string) (*models.PaymentAttempt, *models.PaymentAttemptFundingTarget, error) {
	var attempt models.PaymentAttempt
	found := false
	err := s.db.View(func(tx database.Tx) error {
		if !tx.Read().Migrator().HasTable(&models.PaymentAttempt{}) {
			return nil
		}
		res := tx.Read().Where("tenant_id = ? AND attempt_id = ? AND order_id = ?", tenantID, attemptID, orderID).
			Limit(1).Find(&attempt)
		found = res.Error == nil && res.RowsAffected > 0
		return res.Error
	})
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, ErrOnrampAttemptNotFound
	}
	if attempt.State != models.PaymentAttemptFundingTargetReady {
		return nil, nil, ErrOnrampAttemptNotReady
	}
	target, err := attempt.GetFundingTarget()
	if err != nil {
		return nil, nil, err
	}
	if target == nil || strings.TrimSpace(target.Address) == "" {
		return nil, nil, ErrOnrampAttemptNotReady
	}
	return &attempt, target, nil
}

// findByIdempotencyKey returns the stored record for the resume key, if any.
func (s *OnrampFundingAppService) findByIdempotencyKey(tenantID, attemptID, idemKey string) *models.PaymentAttemptOnrampFundingSource {
	var row models.PaymentAttemptOnrampFundingSource
	found := false
	_ = s.db.View(func(tx database.Tx) error {
		if !tx.Read().Migrator().HasTable(&models.PaymentAttemptOnrampFundingSource{}) {
			return nil
		}
		res := tx.Read().Where("tenant_id = ? AND attempt_id = ? AND idempotency_key = ?", tenantID, attemptID, idemKey).
			Limit(1).Find(&row)
		found = res.Error == nil && res.RowsAffected > 0
		return res.Error
	})
	if !found {
		return nil
	}
	return &row
}

// loadHistory returns the attempt's full purchase history.
func (s *OnrampFundingAppService) loadHistory(tenantID, attemptID string) []models.PaymentAttemptOnrampFundingSource {
	var rows []models.PaymentAttemptOnrampFundingSource
	_ = s.db.View(func(tx database.Tx) error {
		if !tx.Read().Migrator().HasTable(&models.PaymentAttemptOnrampFundingSource{}) {
			return nil
		}
		return tx.Read().Where("tenant_id = ? AND attempt_id = ?", tenantID, attemptID).
			Order("updated_at DESC").Find(&rows).Error
	})
	return rows
}
