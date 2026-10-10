package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
)

// maxOpenBuyRequestsPerBuyer stops one buyer from flooding the board. The
// number is deliberately generous: it is an anti-abuse ceiling, not a product
// limit the UI has to explain.
const maxOpenBuyRequestsPerBuyer = 20

// BuyRequestAppService owns the wanted board rules.
//
// The three platform red lines are enforced here rather than in the UI:
//  1. every accepted offer becomes an escrow-backed order (see the order draft
//     the API returns from AcceptOffer),
//  2. an offer is binding — a seller may not re-offer on the same request,
//  3. disputes are handled by the moderator, so no direct-payment path exists.
type BuyRequestAppService struct {
	store contracts.BuyRequestStore
	now   func() time.Time
}

func NewBuyRequestAppService(store contracts.BuyRequestStore) *BuyRequestAppService {
	return &BuyRequestAppService{store: store, now: time.Now}
}

// Store exposes the underlying persistence for callers that need it (tests,
// cross-service wiring), mirroring the other app services.
func (s *BuyRequestAppService) Store() contracts.BuyRequestStore { return s.store }

func (s *BuyRequestAppService) Create(ctx context.Context, buyerPeerID string, in contracts.CreateBuyRequestInput) (*models.BuyRequest, error) {
	buyerPeerID = strings.TrimSpace(buyerPeerID)
	if buyerPeerID == "" {
		return nil, fmt.Errorf("%w: buyer identity is required", database.ErrBuyRequestInvalid)
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", database.ErrBuyRequestInvalid)
	}
	if in.Quantity < 1 {
		return nil, fmt.Errorf("%w: quantity must be at least 1", database.ErrBuyRequestInvalid)
	}

	maxAmount := strings.TrimSpace(in.BudgetMaxAmount)
	if maxAmount == "" {
		return nil, fmt.Errorf("%w: budgetMax.amount is required", database.ErrBuyRequestInvalid)
	}
	if amount, err := strconv.ParseFloat(maxAmount, 64); err != nil || amount <= 0 {
		return nil, fmt.Errorf("%w: budgetMax.amount must be a positive number", database.ErrBuyRequestInvalid)
	}

	maxCurrency := normalizeCode(in.BudgetMaxCurrency)
	if maxCurrency == "" {
		return nil, fmt.Errorf("%w: budgetMax.currency is required", database.ErrBuyRequestInvalid)
	}

	currencies := normalizeCurrencies(in.AcceptedCurrencies)
	if len(currencies) == 0 {
		// A buyer who states a budget but no accepted currency obviously means
		// the budget currency, so default to it instead of rejecting.
		currencies = []string{maxCurrency}
	}

	deliverTo := normalizeCode(in.DeliverTo)
	if deliverTo == "" {
		return nil, fmt.Errorf("%w: deliverTo is required", database.ErrBuyRequestInvalid)
	}

	if in.ExpiresAt.IsZero() {
		return nil, fmt.Errorf("%w: expiresAt is required", database.ErrBuyRequestInvalid)
	}
	expiresAt := in.ExpiresAt.UTC()
	if !expiresAt.After(s.now().UTC()) {
		return nil, fmt.Errorf("%w: expiresAt must be in the future", database.ErrBuyRequestInvalid)
	}

	condition, err := parseCondition(in.Condition)
	if err != nil {
		return nil, err
	}

	active, err := s.store.CountActiveByBuyer(ctx, buyerPeerID)
	if err != nil {
		return nil, fmt.Errorf("count active buy requests: %w", err)
	}
	if active >= maxOpenBuyRequestsPerBuyer {
		return nil, fmt.Errorf("%w: limit %d", database.ErrBuyRequestLimitReached, maxOpenBuyRequestsPerBuyer)
	}

	request := &models.BuyRequest{
		ID:                 uuid.New().String(),
		Slug:               buildBuyRequestSlug(title),
		BuyerPeerID:        buyerPeerID,
		Title:              title,
		CategorySlug:       strings.TrimSpace(in.CategorySlug),
		Description:        strings.TrimSpace(in.Description),
		Quantity:           in.Quantity,
		Unit:               strings.TrimSpace(in.Unit),
		BudgetMaxAmount:    maxAmount,
		BudgetMaxCurrency:  maxCurrency,
		BudgetMinAmount:    strings.TrimSpace(in.BudgetMinAmount),
		AcceptedCurrencies: database.NormalizeAcceptedCurrencies(currencies),
		DeliverTo:          deliverTo,
		Condition:          condition,
		Status:             models.BuyRequestStatusOpen,
		ExpiresAt:          expiresAt,
	}
	if err := s.store.CreateRequest(ctx, request); err != nil {
		return nil, err
	}
	return request, nil
}

// List returns the public board. Requests whose deadline has passed are closed
// first so a caller never sees a stale "open" row.
func (s *BuyRequestAppService) List(ctx context.Context, f contracts.BuyRequestFilter) ([]*models.BuyRequest, int64, error) {
	if strings.TrimSpace(f.Status) == "" {
		f.Status = string(models.BuyRequestStatusOpen)
	}
	requests, total, err := s.store.ListRequests(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	if s.closeExpired(ctx, requests) > 0 {
		// Rows just left the board. Re-read so the page and the total agree —
		// otherwise a caller can see total=1 with an empty list and conclude
		// its pagination is broken.
		requests, total, err = s.store.ListRequests(ctx, f)
		if err != nil {
			return nil, 0, err
		}
	}
	return requests, total, nil
}

func (s *BuyRequestAppService) ListMine(ctx context.Context, buyerPeerID string, page, pageSize int) ([]*models.BuyRequest, int64, error) {
	buyerPeerID = strings.TrimSpace(buyerPeerID)
	if buyerPeerID == "" {
		return nil, 0, fmt.Errorf("%w: buyer identity is required", database.ErrBuyRequestInvalid)
	}
	requests, total, err := s.store.ListRequestsByBuyer(ctx, buyerPeerID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	// Deliberately no re-read here: "my requests" is unfiltered, so the buyer
	// should still see the row, just with its updated status.
	s.closeExpired(ctx, requests)
	return requests, total, nil
}

// Get returns a request together with its offers. Offers are public by design:
// a wanted board only works when sellers can see the going rate.
func (s *BuyRequestAppService) Get(ctx context.Context, slugStr string) (*models.BuyRequest, []*models.BuyRequestOffer, error) {
	request, err := s.store.GetRequest(ctx, slugStr)
	if err != nil {
		return nil, nil, err
	}
	s.closeExpired(ctx, []*models.BuyRequest{request})
	offers, err := s.store.ListOffers(ctx, request.Slug)
	if err != nil {
		return nil, nil, err
	}
	request.OfferCount = len(offers)
	return request, offers, nil
}

// Cancel closes an open request. Only the buyer may do it, and only while the
// question is still open — once an offer is accepted the deal is already being
// escrowed and must be resolved through the order instead.
func (s *BuyRequestAppService) Cancel(ctx context.Context, buyerPeerID, slugStr string) (*models.BuyRequest, error) {
	request, err := s.store.GetRequest(ctx, slugStr)
	if err != nil {
		return nil, err
	}
	if request.BuyerPeerID != strings.TrimSpace(buyerPeerID) {
		return nil, database.ErrBuyRequestNotOwner
	}
	if request.Status != models.BuyRequestStatusOpen {
		return nil, fmt.Errorf("%w: status is %s", database.ErrBuyRequestClosed, request.Status)
	}
	request.Status = models.BuyRequestStatusCancelled
	if err := s.store.UpdateRequest(ctx, request); err != nil {
		return nil, err
	}
	return request, nil
}

func (s *BuyRequestAppService) SubmitOffer(ctx context.Context, sellerPeerID, slugStr string, in contracts.CreateBuyRequestOfferInput) (*models.BuyRequestOffer, error) {
	sellerPeerID = strings.TrimSpace(sellerPeerID)
	if sellerPeerID == "" {
		return nil, fmt.Errorf("%w: seller identity is required", database.ErrBuyRequestInvalid)
	}

	request, err := s.store.GetRequest(ctx, slugStr)
	if err != nil {
		return nil, err
	}
	// Report the deadline explicitly, even when the lazy sweep already closed
	// the request: "expired" tells a seller to wait for a repost, while the
	// generic "closed" tells them nothing useful.
	if !request.ExpiresAt.After(s.now().UTC()) {
		return nil, database.ErrBuyRequestExpired
	}
	if request.Status != models.BuyRequestStatusOpen {
		return nil, fmt.Errorf("%w: status is %s", database.ErrBuyRequestClosed, request.Status)
	}
	if request.BuyerPeerID == sellerPeerID {
		return nil, database.ErrBuyRequestSelfOffer
	}

	unitPrice := strings.TrimSpace(in.UnitPriceAmount)
	if unitPrice == "" {
		return nil, fmt.Errorf("%w: unitPrice.amount is required", database.ErrBuyRequestInvalid)
	}
	if amount, err := strconv.ParseFloat(unitPrice, 64); err != nil || amount <= 0 {
		return nil, fmt.Errorf("%w: unitPrice.amount must be a positive number", database.ErrBuyRequestInvalid)
	}

	currency := normalizeCode(in.UnitPriceCurrency)
	if currency == "" {
		return nil, fmt.Errorf("%w: unitPrice.currency is required", database.ErrBuyRequestInvalid)
	}
	if !currencyAccepted(request, currency) {
		return nil, fmt.Errorf("%w: %s", database.ErrBuyRequestCurrencyRejected, currency)
	}

	if in.Quantity < 1 {
		return nil, fmt.Errorf("%w: quantity must be at least 1", database.ErrBuyRequestInvalid)
	}
	if in.LeadTimeDays < 0 {
		return nil, fmt.Errorf("%w: leadTimeDays must not be negative", database.ErrBuyRequestInvalid)
	}

	// An offer is binding, so a seller gets exactly one shot per request.
	duplicate, err := s.store.HasOfferFromSeller(ctx, request.Slug, sellerPeerID)
	if err != nil {
		return nil, fmt.Errorf("check existing offer: %w", err)
	}
	if duplicate {
		return nil, database.ErrBuyRequestDuplicateOffer
	}

	offer := &models.BuyRequestOffer{
		ID:                uuid.New().String(),
		RequestSlug:       request.Slug,
		SellerPeerID:      sellerPeerID,
		UnitPriceAmount:   unitPrice,
		UnitPriceCurrency: currency,
		Quantity:          in.Quantity,
		LeadTimeDays:      in.LeadTimeDays,
		ShipsFrom:         normalizeCode(in.ShipsFrom),
		Note:              strings.TrimSpace(in.Note),
		Status:            models.BuyRequestOfferSubmitted,
	}
	if err := s.store.CreateOffer(ctx, offer); err != nil {
		return nil, err
	}
	return offer, nil
}

// AcceptOffer locks the request onto one offer and rejects the rest. The
// returned pair is what the API turns into an escrow order draft.
func (s *BuyRequestAppService) AcceptOffer(ctx context.Context, buyerPeerID, slugStr, offerID string) (*models.BuyRequest, *models.BuyRequestOffer, error) {
	request, err := s.store.GetRequest(ctx, slugStr)
	if err != nil {
		return nil, nil, err
	}
	if request.BuyerPeerID != strings.TrimSpace(buyerPeerID) {
		return nil, nil, database.ErrBuyRequestNotOwner
	}
	if !request.ExpiresAt.After(s.now().UTC()) {
		return nil, nil, database.ErrBuyRequestExpired
	}
	if request.Status != models.BuyRequestStatusOpen {
		return nil, nil, fmt.Errorf("%w: status is %s", database.ErrBuyRequestClosed, request.Status)
	}

	offerID = strings.TrimSpace(offerID)
	if offerID == "" {
		return nil, nil, fmt.Errorf("%w: offerID is required", database.ErrBuyRequestInvalid)
	}
	offer, err := s.store.GetOffer(ctx, offerID)
	if err != nil {
		return nil, nil, err
	}
	if offer.RequestSlug != request.Slug {
		return nil, nil, database.ErrBuyRequestOfferNotFound
	}
	if offer.Status != models.BuyRequestOfferSubmitted {
		return nil, nil, fmt.Errorf("%w: offer status is %s", database.ErrBuyRequestClosed, offer.Status)
	}

	request.Status = models.BuyRequestStatusLocked
	request.AcceptedOfferID = offer.ID
	if err := s.store.UpdateRequest(ctx, request); err != nil {
		return nil, nil, err
	}

	offer.Status = models.BuyRequestOfferAccepted
	if err := s.store.UpdateOffer(ctx, offer); err != nil {
		return nil, nil, err
	}
	if err := s.store.RejectPendingOffers(ctx, request.Slug, offer.ID); err != nil {
		return nil, nil, err
	}
	return request, offer, nil
}

// closeExpired flips open requests past their deadline to cancelled and reports
// how many it closed. It is a lazy sweep on read: the board has no scheduler,
// and a request nobody reads does not need its state fixed.
func (s *BuyRequestAppService) closeExpired(ctx context.Context, requests []*models.BuyRequest) int {
	now := s.now().UTC()
	closed := 0
	for _, r := range requests {
		if r == nil || r.Status != models.BuyRequestStatusOpen {
			continue
		}
		if r.ExpiresAt.After(now) {
			continue
		}
		r.Status = models.BuyRequestStatusCancelled
		if err := s.store.UpdateRequest(ctx, r); err != nil {
			// Best effort: a failed sweep must not break the read.
			continue
		}
		closed++
	}
	return closed
}

func parseCondition(raw string) (models.BuyRequestCondition, error) {
	switch models.BuyRequestCondition(strings.ToLower(strings.TrimSpace(raw))) {
	case "", models.BuyRequestConditionAny:
		return models.BuyRequestConditionAny, nil
	case models.BuyRequestConditionNew:
		return models.BuyRequestConditionNew, nil
	case models.BuyRequestConditionUsed:
		return models.BuyRequestConditionUsed, nil
	default:
		return "", fmt.Errorf("%w: condition must be one of any, new, used", database.ErrBuyRequestInvalid)
	}
}

func normalizeCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func normalizeCurrencies(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, c := range in {
		c = normalizeCode(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func currencyAccepted(request *models.BuyRequest, currency string) bool {
	return strings.Contains(request.AcceptedCurrencies, ","+currency+",")
}

// buildBuyRequestSlug makes a readable, collision-free identifier. The short
// random tail avoids a retry loop against the unique index.
func buildBuyRequestSlug(title string) string {
	base := strings.Trim(slug.Make(title), "-")
	if base == "" {
		base = "wanted"
	}
	if len(base) > 60 {
		base = strings.Trim(base[:60], "-")
	}
	return base + "-" + uuid.New().String()[:8]
}

var _ contracts.BuyRequestService = (*BuyRequestAppService)(nil)
