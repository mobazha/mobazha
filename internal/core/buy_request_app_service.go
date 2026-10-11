package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/orders"
	opb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// maxOpenBuyRequestsPerBuyer stops one buyer from flooding the board. The ceiling
// is pinned by the record contract so a node and the index agree on what a board
// may hold: it is an anti-abuse limit, not a product limit the UI has to explain.
const maxOpenBuyRequestsPerBuyer = orders.BuyerRequestMaxOpenPerPeer

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
	// notifier carries the buyer's decision back to sellers on other nodes. It
	// is nil on a node without a p2p transport, where every seller is local and
	// already sees the decision on this board.
	notifier buyRequestNotifier
	// eventBus announces board changes so the search service can re-index a
	// request. It is nil in tests and on a node without a bus.
	eventBus events.Bus
}

func NewBuyRequestAppService(store contracts.BuyRequestStore) *BuyRequestAppService {
	return &BuyRequestAppService{store: store, now: time.Now}
}

// SetNotifier wires the transport that tells remote sellers what the buyer
// decided. It is set once, after the node's messenger exists.
func (s *BuyRequestAppService) SetNotifier(notifier buyRequestNotifier) {
	if s == nil {
		return
	}
	s.notifier = notifier
}

// SetEventBus wires the bus that carries board changes to the search-service
// sync. It is set once, when the node is assembled.
func (s *BuyRequestAppService) SetEventBus(bus events.Bus) {
	if s == nil {
		return
	}
	s.eventBus = bus
}

// announce tells the search-service sync that this request changed. A change is
// worth publishing only while the request is open: the board only lists open
// requests, and anything else is withdrawn instead.
func (s *BuyRequestAppService) announce(slug string, open bool) {
	if s.eventBus == nil || slug == "" {
		return
	}
	if open {
		s.eventBus.Emit(&events.BuyRequestChanged{Slug: slug})
		return
	}
	s.eventBus.Emit(&events.BuyRequestDeleted{Slug: slug})
}

// Record renders one of this node's requests as the signed public record.
//
// It is the single mapping from storage onto the wire, so the node, the index,
// and the web client all describe a request the same way. Only the node that owns
// a request can produce it, which is what keeps the board's single-writer rule
// intact.
func (s *BuyRequestAppService) Record(ctx context.Context, requestID string) (*opb.BuyerRequest, error) {
	request, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	record := &opb.BuyerRequest{
		RequestID: request.Slug,
		Revision:  uint32(max(request.Revision, 1)),
		PeerID:    request.BuyerPeerID,
		Title:     request.Title,
		Quantity:  uint32(request.Quantity),
		Budget: &opb.CurrencyValue{
			Currency: &opb.Currency{
				Code:         request.BudgetMaxCurrency,
				Divisibility: uint32(budgetDivisibility(request.BudgetMaxCurrency)),
			},
			Amount: request.BudgetMaxAmount,
		},
		AcceptedCurrencies: contracts.SplitBuyRequestCurrencies(request.AcceptedCurrencies),
		ShipTo:             request.DeliverTo,
		ProductType:        request.CategorySlug,
		Description:        request.Description,
		Status:             recordStatus(request.Status),
		CreatedAt:          timestamppb.New(request.CreatedAt.UTC()),
		ExpiresAt:          timestamppb.New(request.ExpiresAt.UTC()),
	}
	for _, image := range request.Images {
		record.Images = append(record.Images, &opb.Image{
			Filename: image.Filename,
			Original: image.Original,
			Large:    image.Large,
			Medium:   image.Medium,
			Small:    image.Small,
			Tiny:     image.Tiny,
		})
	}
	return record, nil
}

// recordStatus folds the board's internal states onto the two values a reader
// sees: a request that is no longer collecting answers is closed.
func recordStatus(status models.BuyRequestStatus) string {
	if status == models.BuyRequestStatusOpen {
		return orders.BuyerRequestStatusOpen
	}
	return orders.BuyerRequestStatusClosed
}

// budgetDivisibility reads a currency's divisibility from the registry, which is
// what makes a budget amount a number in the smallest unit rather than a decimal.
func budgetDivisibility(code string) uint32 {
	if definition, err := models.CurrencyDefinitions.Lookup(code); err == nil && definition != nil {
		if definition.Divisibility > 18 {
			// The record's divisibility is a promise about how many decimal
			// places the amount carries, and the index refuses anything past
			// eighteen. A currency that claims more is described with the
			// maximum instead of failing the push.
			return 18
		}
		return uint32(definition.Divisibility)
	}
	// The registry does not know this code — a token identifier rather than a
	// currency, usually. The record still has to state a divisibility, and eight
	// decimal places is what a currency like that has.
	return 8
}

// Store exposes the underlying persistence for callers that need it (tests,
// cross-service wiring), mirroring the other app services.
func (s *BuyRequestAppService) Store() contracts.BuyRequestStore { return s.store }

func (s *BuyRequestAppService) Create(ctx context.Context, buyerPeerID string, in contracts.CreateBuyRequestInput) (*models.BuyRequest, error) {
	buyerPeerID = strings.TrimSpace(buyerPeerID)
	if buyerPeerID == "" {
		return nil, fmt.Errorf("%w: buyer identity is required", database.ErrBuyRequestInvalid)
	}

	validated, err := validateRequest(recordFields{
		title:              in.Title,
		productType:        in.ProductType,
		quantity:           in.Quantity,
		budgetAmount:       in.BudgetAmount,
		budgetCurrency:     in.BudgetCurrency,
		acceptedCurrencies: in.AcceptedCurrencies,
		shipTo:             in.ShipTo,
		description:        in.Description,
		expiresAt:          in.ExpiresAt,
		images:             in.Images,
	}, s.now())
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
		Slug:               buildBuyRequestSlug(validated.title),
		Revision:           1,
		BuyerPeerID:        buyerPeerID,
		Title:              validated.title,
		CategorySlug:       validated.productType,
		Description:        validated.description,
		Quantity:           validated.quantity,
		BudgetMaxAmount:    validated.budgetAmount,
		BudgetMaxCurrency:  validated.budgetCurrency,
		AcceptedCurrencies: database.NormalizeAcceptedCurrencies(validated.currencies),
		DeliverTo:          validated.shipTo,
		Status:             models.BuyRequestStatusOpen,
		ExpiresAt:          validated.expiresAt,
	}
	if err := request.SetImages(validated.images); err != nil {
		return nil, fmt.Errorf("encode buy request images: %w", err)
	}
	if err := s.store.CreateRequest(ctx, request); err != nil {
		return nil, err
	}
	s.announce(request.Slug, true)
	return request, nil
}

// Update edits a request the buyer owns. The revision grows by one on every
// edit, which is what lets the index refuse an older copy: a reader must never
// see a request that its owner already replaced.
func (s *BuyRequestAppService) Update(ctx context.Context, buyerPeerID, requestID string, in contracts.UpdateBuyRequestInput) (*models.BuyRequest, error) {
	buyerPeerID = strings.TrimSpace(buyerPeerID)
	request, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if request.BuyerPeerID != buyerPeerID {
		return nil, database.ErrBuyRequestNotOwner
	}
	if request.Status != models.BuyRequestStatusOpen {
		return nil, fmt.Errorf("%w: status is %s", database.ErrBuyRequestClosed, request.Status)
	}

	validated, err := validateRequest(recordFields{
		title:              in.Title,
		productType:        in.ProductType,
		quantity:           in.Quantity,
		budgetAmount:       in.BudgetAmount,
		budgetCurrency:     in.BudgetCurrency,
		acceptedCurrencies: in.AcceptedCurrencies,
		shipTo:             in.ShipTo,
		description:        in.Description,
		expiresAt:          in.ExpiresAt,
		images:             in.Images,
	}, s.now())
	if err != nil {
		return nil, err
	}

	request.Revision++
	request.Title = validated.title
	request.CategorySlug = validated.productType
	request.Description = validated.description
	request.Quantity = validated.quantity
	request.BudgetMaxAmount = validated.budgetAmount
	request.BudgetMaxCurrency = validated.budgetCurrency
	request.AcceptedCurrencies = database.NormalizeAcceptedCurrencies(validated.currencies)
	request.DeliverTo = validated.shipTo
	request.ExpiresAt = validated.expiresAt
	if err := request.SetImages(validated.images); err != nil {
		return nil, fmt.Errorf("encode buy request images: %w", err)
	}
	if err := s.store.UpdateRequest(ctx, request); err != nil {
		return nil, err
	}
	s.announce(request.Slug, true)
	return request, nil
}

// recordFields is one set of raw record fields, before validation. Create and
// Update both go through validateRequest so an edit cannot be looser than a
// publication.
type recordFields struct {
	title              string
	productType        string
	description        string
	budgetAmount       string
	budgetCurrency     string
	acceptedCurrencies []string
	shipTo             string
	quantity           int
	expiresAt          time.Time
	images             []models.ImageHashes
}

// validatedFields is a record that satisfies every rule the contract states.
type validatedFields struct {
	title          string
	productType    string
	description    string
	budgetAmount   string
	budgetCurrency string
	currencies     []string
	shipTo         string
	quantity       int
	expiresAt      time.Time
	images         []models.ImageHashes
}

// validateRequest enforces the record contract's rules. The index enforces the
// same ones again, because it cannot trust a publisher; doing it here as well
// means a buyer sees the problem instead of a refused push.
func validateRequest(in recordFields, now time.Time) (validatedFields, error) {
	out := validatedFields{}

	out.title = strings.TrimSpace(in.title)
	if out.title == "" {
		return out, fmt.Errorf("%w: title is required", database.ErrBuyRequestInvalid)
	}
	if utf8.RuneCountInString(out.title) > orders.BuyerRequestMaxTitleLength {
		return out, fmt.Errorf("%w: title must be at most %d characters",
			database.ErrBuyRequestInvalid, orders.BuyerRequestMaxTitleLength)
	}
	out.description = strings.TrimSpace(in.description)
	if utf8.RuneCountInString(out.description) > orders.BuyerRequestMaxDescriptionLength {
		return out, fmt.Errorf("%w: description must be at most %d characters",
			database.ErrBuyRequestInvalid, orders.BuyerRequestMaxDescriptionLength)
	}
	// productType is optional. The board indexes it so a seller can filter by
	// the products they already publish, but a buyer who does not know the
	// taxonomy is not made to invent one.
	out.productType = strings.TrimSpace(in.productType)
	if utf8.RuneCountInString(out.productType) > orders.BuyerRequestMaxProductTypeLength {
		return out, fmt.Errorf("%w: productType must be at most %d characters",
			database.ErrBuyRequestInvalid, orders.BuyerRequestMaxProductTypeLength)
	}
	out.quantity = in.quantity
	if out.quantity < 1 || out.quantity > orders.BuyerRequestMaxQuantity {
		return out, fmt.Errorf("%w: quantity must be between 1 and %d",
			database.ErrBuyRequestInvalid, orders.BuyerRequestMaxQuantity)
	}

	// A budget is a whole number of the currency's smallest unit, never a
	// decimal: the same rule listing prices follow.
	out.budgetAmount = strings.TrimSpace(in.budgetAmount)
	if out.budgetAmount == "" {
		return out, fmt.Errorf("%w: budget.amount is required", database.ErrBuyRequestInvalid)
	}
	if !isPositiveInteger(out.budgetAmount) || len(out.budgetAmount) > orders.BuyerRequestMaxBudgetDigits {
		return out, fmt.Errorf("%w: budget.amount must be a positive integer in the "+
			"currency's smallest unit, at most %d digits",
			database.ErrBuyRequestInvalid, orders.BuyerRequestMaxBudgetDigits)
	}
	out.budgetCurrency = normalizeCode(in.budgetCurrency)
	if out.budgetCurrency == "" {
		return out, fmt.Errorf("%w: budget.currency is required", database.ErrBuyRequestInvalid)
	}

	out.currencies = normalizeCurrencies(in.acceptedCurrencies)
	if len(out.currencies) == 0 {
		// A buyer who states a budget but no accepted currency obviously means
		// the budget currency, so default to it instead of rejecting.
		out.currencies = []string{out.budgetCurrency}
	}

	// shipTo is optional: a buyer who will take the goods wherever they come
	// from is not asked to name a country. When it is given it must be valid.
	out.shipTo = normalizeCode(in.shipTo)
	if out.shipTo != "" && !isCountryCode(out.shipTo) {
		return out, fmt.Errorf("%w: shipTo must be a two-letter country code", database.ErrBuyRequestInvalid)
	}

	if in.expiresAt.IsZero() {
		return out, fmt.Errorf("%w: expiresAt is required", database.ErrBuyRequestInvalid)
	}
	out.expiresAt = in.expiresAt.UTC()
	if !out.expiresAt.After(now.UTC()) {
		return out, fmt.Errorf("%w: expiresAt must be in the future", database.ErrBuyRequestInvalid)
	}
	if out.expiresAt.Sub(now.UTC()) > orders.BuyerRequestMaxLifetime {
		return out, fmt.Errorf("%w: a request may stay open for at most %d days",
			database.ErrBuyRequestInvalid, int(orders.BuyerRequestMaxLifetime.Hours()/24))
	}

	images, err := normalizeBuyRequestImages(in.images)
	if err != nil {
		return out, err
	}
	out.images = images
	return out, nil
}

// normalizeBuyRequestImages trims each hash set and drops entries that carry no
// hash at all: an empty entry says nothing, and rejecting it would force every
// client to special-case "no image". The count cap is a hard error instead of a
// silent truncation, because dropping pictures a seller deliberately attached
// would be worse than refusing the request.
func normalizeBuyRequestImages(in []models.ImageHashes) ([]models.ImageHashes, error) {
	out := make([]models.ImageHashes, 0, len(in))
	for _, img := range in {
		trimmed := models.ImageHashes{
			Tiny:     strings.TrimSpace(img.Tiny),
			Small:    strings.TrimSpace(img.Small),
			Medium:   strings.TrimSpace(img.Medium),
			Large:    strings.TrimSpace(img.Large),
			Original: strings.TrimSpace(img.Original),
			Filename: strings.TrimSpace(img.Filename),
		}
		if trimmed.Tiny == "" && trimmed.Small == "" && trimmed.Medium == "" &&
			trimmed.Large == "" && trimmed.Original == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) > models.MaxBuyRequestImages {
		return nil, fmt.Errorf("%w: at most %d images are allowed",
			database.ErrBuyRequestInvalid, models.MaxBuyRequestImages)
	}
	return out, nil
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
	s.announce(request.Slug, false)
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
	s.announceOfferOutcomes(ctx, request, offer.ID)
	// A locked request is off the open board: the deal has moved to the order.
	s.announce(request.Slug, false)
	return request, offer, nil
}

// announceOfferOutcomes tells the sellers what the buyer decided. Sellers on
// this node already see it on the board, so the notice is only sent to peers.
//
// It is deliberately best effort: the decision is already stored here, on the
// only node that owns the request, so a notice that cannot be delivered must
// not turn a successful acceptance into an error.
func (s *BuyRequestAppService) announceOfferOutcomes(ctx context.Context, request *models.BuyRequest, acceptedOfferID string) {
	if s.notifier == nil || request == nil {
		return
	}
	offers, err := s.store.ListOffers(ctx, request.Slug)
	if err != nil {
		log.Warningf("BuyRequest: cannot list offers to announce a decision on %s: %s", request.Slug, err)
		return
	}
	payload, err := json.Marshal(contracts.NewBuyRequestDetailDTO(request, offers))
	if err != nil {
		log.Warningf("BuyRequest: cannot encode the decision on %s: %s", request.Slug, err)
		return
	}
	for _, offer := range offers {
		if offer == nil {
			continue
		}
		accepted := offer.ID == acceptedOfferID
		if err := s.notifier.NotifyOfferOutcome(ctx, offer.SellerPeerID, request.Slug, accepted, payload); err != nil {
			// The seller will still see the decision when it next reads the
			// request, so a failed notice is worth a line but not an error.
			log.Warningf("BuyRequest: cannot tell %s about offer %s on %s: %s",
				offer.SellerPeerID, offer.ID, request.Slug, err)
		}
	}
}

// closeExpired flips open requests past their deadline to cancelled and reports
// ListComments returns a request's comment thread. The request must exist, but
// comments stay readable after it closes — the thread is the record of how a
// deal was discussed.
func (s *BuyRequestAppService) ListComments(ctx context.Context, slugStr string) ([]*models.BuyRequestComment, error) {
	if _, err := s.store.GetRequest(ctx, slugStr); err != nil {
		return nil, err
	}
	return s.store.ListComments(ctx, slugStr)
}

// CreateComment posts to a request's thread. Threading is one level deep: a
// comment either starts a thread or answers one that did.
func (s *BuyRequestAppService) CreateComment(ctx context.Context, authorPeerID, slugStr string, in contracts.CreateBuyRequestCommentInput) (*models.BuyRequestComment, error) {
	authorPeerID = strings.TrimSpace(authorPeerID)
	if authorPeerID == "" {
		return nil, fmt.Errorf("%w: author identity is required", database.ErrBuyRequestInvalid)
	}

	body := strings.TrimSpace(in.Body)
	if body == "" {
		return nil, fmt.Errorf("%w: comment body is required", database.ErrBuyRequestInvalid)
	}
	if utf8.RuneCountInString(body) > models.MaxBuyRequestCommentLength {
		return nil, fmt.Errorf("%w: comment must be at most %d characters",
			database.ErrBuyRequestInvalid, models.MaxBuyRequestCommentLength)
	}

	request, err := s.store.GetRequest(ctx, slugStr)
	if err != nil {
		return nil, err
	}
	// An open request past its deadline is expired even if the lazy sweep has
	// not run yet, so report that before falling through to the status check.
	if request.Status == models.BuyRequestStatusOpen && !request.ExpiresAt.After(s.now().UTC()) {
		return nil, database.ErrBuyRequestExpired
	}
	// Once the buyer has walked away the thread becomes a record, so it stops
	// accepting new messages. A locked request is mid-deal and stays open for
	// the two parties to keep talking.
	if request.Status != models.BuyRequestStatusOpen && request.Status != models.BuyRequestStatusLocked {
		return nil, fmt.Errorf("%w: status is %s", database.ErrBuyRequestClosed, request.Status)
	}

	parentID := strings.TrimSpace(in.ParentCommentID)
	if parentID != "" {
		parent, err := s.store.GetComment(ctx, parentID)
		if err != nil {
			return nil, err
		}
		if parent.RequestSlug != request.Slug {
			return nil, database.ErrBuyRequestCommentNotFound
		}
		if parent.ParentCommentID != "" {
			return nil, database.ErrBuyRequestReplyToReply
		}
	}

	comment := &models.BuyRequestComment{
		ID:              uuid.New().String(),
		RequestSlug:     request.Slug,
		AuthorPeerID:    authorPeerID,
		ParentCommentID: parentID,
		Body:            body,
	}
	if err := s.store.CreateComment(ctx, comment); err != nil {
		return nil, err
	}
	return comment, nil
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
		s.announce(r.Slug, false)
		closed++
	}
	return closed
}

// isPositiveInteger reports whether a string is a run of digits that is not all
// zeroes. A budget counts the currency's smallest unit and may be far larger
// than a 64-bit integer, so it is never parsed into one.
func isPositiveInteger(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return strings.Trim(value, "0") != ""
}

// isCountryCode accepts the two-letter country goods must reach.
func isCountryCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
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
