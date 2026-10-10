package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/response"
)

// maxBuyRequestDays caps how far ahead a request may be scheduled. A year is
// long enough for sourcing rare goods and short enough that a forgotten request
// does not sit on the board forever.
const maxBuyRequestDays = 365

func getBuyRequestService(r *http.Request) (contracts.BuyRequestService, bool) {
	provider, ok := getNodeService(r).(contracts.BuyRequestProvider)
	if !ok {
		return nil, false
	}
	svc := provider.BuyRequests()
	if svc == nil {
		return nil, false
	}
	return svc, true
}

// actingBuyRequestPeerID attributes a board action to a peer. On a standalone
// node the authenticated caller is the node itself, which is exactly the
// identity that owns the store.
func actingBuyRequestPeerID(r *http.Request) string {
	identity := getNodeService(r).IdentityInfo()
	if identity == nil {
		return ""
	}
	return identity.Identity().String()
}

func buyRequestErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrBuyRequestNotFound),
		errors.Is(err, database.ErrBuyRequestOfferNotFound),
		errors.Is(err, database.ErrBuyRequestCommentNotFound):
		response.Error(w, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, database.ErrBuyRequestInvalid):
		response.Error(w, http.StatusBadRequest, response.CodeValidation, err.Error())
	case errors.Is(err, database.ErrBuyRequestClosed),
		errors.Is(err, database.ErrBuyRequestExpired),
		errors.Is(err, database.ErrBuyRequestSelfOffer),
		errors.Is(err, database.ErrBuyRequestDuplicateOffer),
		errors.Is(err, database.ErrBuyRequestCurrencyRejected),
		errors.Is(err, database.ErrBuyRequestNotOwner),
		errors.Is(err, database.ErrBuyRequestLimitReached),
		errors.Is(err, database.ErrBuyRequestSlugTaken),
		errors.Is(err, database.ErrBuyRequestReplyToReply):
		response.Error(w, http.StatusConflict, response.CodeConflict, err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
	}
}

// --- wire types ---

type buyRequestParty struct {
	PeerID string `json:"peerID"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

type buyRequestAmount struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type buyRequestBudget struct {
	Max buyRequestAmount  `json:"max"`
	Min *buyRequestAmount `json:"min,omitempty"`
}

type buyRequest struct {
	Slug               string           `json:"slug"`
	BuyerID            buyRequestParty  `json:"buyerID"`
	Title              string           `json:"title"`
	CategorySlug       string           `json:"categorySlug"`
	Description        string           `json:"description,omitempty"`
	Quantity           int              `json:"quantity"`
	Unit               string           `json:"unit,omitempty"`
	Budget             buyRequestBudget `json:"budget"`
	AcceptedCurrencies []string         `json:"acceptedCurrencies"`
	DeliverTo          string           `json:"deliverTo"`
	Condition          string           `json:"condition"`
	Status             string           `json:"status"`
	AcceptedOfferID    string           `json:"acceptedOfferID,omitempty"`
	OfferCount         int              `json:"offerCount"`
	ExpiresAt          string           `json:"expiresAt"`
	CreatedAt          string           `json:"createdAt"`
}

type buyRequestOffer struct {
	ID           string           `json:"id"`
	RequestSlug  string           `json:"requestSlug"`
	SellerID     buyRequestParty  `json:"sellerID"`
	UnitPrice    buyRequestAmount `json:"unitPrice"`
	Quantity     int              `json:"quantity"`
	LeadTimeDays int              `json:"leadTimeDays"`
	ShipsFrom    string           `json:"shipsFrom"`
	// Escrow is not optional on Mobazha; the field is constant so a client can
	// never accidentally render an offer as "pay me directly".
	AcceptsEscrow bool   `json:"acceptsEscrow"`
	Note          string `json:"note,omitempty"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
}

type buyRequestDetail struct {
	Request *buyRequest        `json:"request"`
	Offers  []*buyRequestOffer `json:"offers"`
}

// buyRequestListPayload is the success payload for both board listings. It
// carries its own pagination instead of relying on the response envelope's
// meta, because the huma bridge re-wraps the payload and would drop meta.
type buyRequestListPayload struct {
	Requests []*buyRequest `json:"requests"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

// multiplyAmount computes unitPrice * quantity exactly, without routing money
// through a float.
func multiplyAmount(amount string, quantity int) (string, error) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(amount))
	if !ok {
		return "", fmt.Errorf("invalid amount %q", amount)
	}
	if quantity > 0 {
		rat.Mul(rat, new(big.Rat).SetInt64(int64(quantity)))
	}
	out := strings.TrimRight(rat.FloatString(8), "0")
	out = strings.TrimRight(out, ".")
	if out == "" {
		out = "0"
	}
	return out, nil
}

type buyRequestOrderDraft struct {
	Slug            string           `json:"slug"`
	VendorID        buyRequestParty  `json:"vendorID"`
	Title           string           `json:"title"`
	Quantity        int              `json:"quantity"`
	UnitPrice       buyRequestAmount `json:"unitPrice"`
	Subtotal        buyRequestAmount `json:"subtotal"`
	Currency        string           `json:"currency"`
	ShipsFrom       string           `json:"shipsFrom"`
	DeliverTo       string           `json:"deliverTo"`
	AcceptedOfferID string           `json:"acceptedOfferID"`
}

type acceptBuyRequestOffer struct {
	Request    *buyRequest          `json:"request"`
	OrderDraft buyRequestOrderDraft `json:"orderDraft"`
}

// buyRequestComment is one message in a request's thread. Replies are nested so
// a client renders a thread without rebuilding the tree itself.
type buyRequestComment struct {
	ID          string          `json:"id"`
	RequestSlug string          `json:"requestSlug"`
	AuthorID    buyRequestParty `json:"authorID"`
	// Empty for a top-level comment.
	ParentCommentID string               `json:"parentCommentID,omitempty"`
	Body            string               `json:"body"`
	CreatedAt       string               `json:"createdAt"`
	Replies         []*buyRequestComment `json:"replies,omitempty"`
}

type buyRequestCommentThread struct {
	Comments []*buyRequestComment `json:"comments"`
	Total    int                  `json:"total"`
}

type createBuyRequestCommentBody struct {
	Body            string `json:"body"`
	ParentCommentID string `json:"parentCommentID"`
}

func toBuyRequestCommentDTO(m *models.BuyRequestComment) *buyRequestComment {
	if m == nil {
		return nil
	}
	return &buyRequestComment{
		ID:              m.ID,
		RequestSlug:     m.RequestSlug,
		AuthorID:        toBuyRequestParty(m.AuthorPeerID),
		ParentCommentID: m.ParentCommentID,
		Body:            m.Body,
		CreatedAt:       m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// groupBuyRequestComments turns the flat, oldest-first list the store returns
// into one level of threads. A reply whose parent is missing is surfaced as a
// top-level comment rather than dropped — silently losing a message is worse
// than a slightly odd indentation.
func groupBuyRequestComments(flat []*models.BuyRequestComment) []*buyRequestComment {
	roots := make([]*buyRequestComment, 0, len(flat))
	byID := make(map[string]*buyRequestComment, len(flat))

	for _, m := range flat {
		dto := toBuyRequestCommentDTO(m)
		byID[dto.ID] = dto
		if dto.ParentCommentID == "" {
			roots = append(roots, dto)
		}
	}
	for _, m := range flat {
		dto := byID[m.ID]
		if dto.ParentCommentID == "" {
			continue
		}
		parent, ok := byID[dto.ParentCommentID]
		if !ok {
			roots = append(roots, dto)
			continue
		}
		parent.Replies = append(parent.Replies, dto)
	}
	return roots
}

func (g *Gateway) handleListBuyRequestComments(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	comments, err := svc.ListComments(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Success(w, buyRequestCommentThread{
		Comments: groupBuyRequestComments(comments),
		Total:    len(comments),
	})
}

func (g *Gateway) handleCreateBuyRequestComment(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	var body createBuyRequestCommentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	comment, err := svc.CreateComment(r.Context(), actingBuyRequestPeerID(r), chi.URLParam(r, "slug"),
		contracts.CreateBuyRequestCommentInput{
			Body:            body.Body,
			ParentCommentID: body.ParentCommentID,
		})
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Created(w, toBuyRequestCommentDTO(comment))
}

func toBuyRequestParty(peerID string) buyRequestParty {
	return buyRequestParty{PeerID: peerID}
}

func toBuyRequestDTO(m *models.BuyRequest) *buyRequest {
	if m == nil {
		return nil
	}
	out := &buyRequest{
		Slug:               m.Slug,
		BuyerID:            toBuyRequestParty(m.BuyerPeerID),
		Title:              m.Title,
		CategorySlug:       m.CategorySlug,
		Description:        m.Description,
		Quantity:           m.Quantity,
		Unit:               m.Unit,
		Budget:             buyRequestBudget{Max: buyRequestAmount{Amount: m.BudgetMaxAmount, Currency: m.BudgetMaxCurrency}},
		AcceptedCurrencies: splitCurrencies(m.AcceptedCurrencies),
		DeliverTo:          m.DeliverTo,
		Condition:          string(m.Condition),
		Status:             string(m.Status),
		AcceptedOfferID:    m.AcceptedOfferID,
		OfferCount:         m.OfferCount,
		ExpiresAt:          m.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt:          m.CreatedAt.UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(m.BudgetMinAmount) != "" {
		min := buyRequestAmount{Amount: m.BudgetMinAmount, Currency: m.BudgetMaxCurrency}
		out.Budget.Min = &min
	}
	return out
}

func toBuyRequestOfferDTO(m *models.BuyRequestOffer) *buyRequestOffer {
	if m == nil {
		return nil
	}
	return &buyRequestOffer{
		ID:            m.ID,
		RequestSlug:   m.RequestSlug,
		SellerID:      toBuyRequestParty(m.SellerPeerID),
		UnitPrice:     buyRequestAmount{Amount: m.UnitPriceAmount, Currency: m.UnitPriceCurrency},
		Quantity:      m.Quantity,
		LeadTimeDays:  m.LeadTimeDays,
		ShipsFrom:     m.ShipsFrom,
		AcceptsEscrow: true,
		Note:          m.Note,
		Status:        string(m.Status),
		CreatedAt:     m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// splitCurrencies turns the stored ",USD,USDT," form back into a list.
func splitCurrencies(stored string) []string {
	out := make([]string, 0, 4)
	for _, part := range strings.Split(stored, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// --- request bodies ---

type createBuyRequestBody struct {
	Title              string            `json:"title"`
	CategorySlug       string            `json:"categorySlug"`
	Quantity           int               `json:"quantity"`
	Unit               string            `json:"unit"`
	BudgetMax          *buyRequestAmount `json:"budgetMax"`
	BudgetMin          *buyRequestAmount `json:"budgetMin"`
	BudgetMaxAmount    string            `json:"budgetMaxAmount"`
	BudgetMaxCurrency  string            `json:"budgetMaxCurrency"`
	BudgetMinAmount    string            `json:"budgetMinAmount"`
	AcceptedCurrencies []string          `json:"acceptedCurrencies"`
	DeliverTo          string            `json:"deliverTo"`
	Description        string            `json:"description"`
	Condition          string            `json:"condition"`
	// ExpiresAt takes an RFC3339 timestamp; ExpiresInDays is the friendlier
	// "keep this up for N days" form the UI uses.
	ExpiresAt     string `json:"expiresAt"`
	ExpiresInDays int    `json:"expiresInDays"`
}

type createBuyRequestOfferBody struct {
	UnitPrice         *buyRequestAmount `json:"unitPrice"`
	UnitPriceAmount   string            `json:"unitPriceAmount"`
	UnitPriceCurrency string            `json:"unitPriceCurrency"`
	Quantity          int               `json:"quantity"`
	LeadTimeDays      int               `json:"leadTimeDays"`
	ShipsFrom         string            `json:"shipsFrom"`
	Note              string            `json:"note"`
}

type acceptBuyRequestOfferBody struct {
	OfferID string `json:"offerID"`
}

func resolveExpiry(body createBuyRequestBody) (time.Time, error) {
	if strings.TrimSpace(body.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(body.ExpiresAt))
		if err != nil {
			return time.Time{}, err
		}
		return parsed, nil
	}
	if body.ExpiresInDays <= 0 || body.ExpiresInDays > maxBuyRequestDays {
		return time.Time{}, errors.New("expiresAt or expiresInDays is required")
	}
	return time.Now().UTC().AddDate(0, 0, body.ExpiresInDays), nil
}

// --- handlers ---

func (g *Gateway) handleListBuyRequests(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 20)
	filter := contracts.BuyRequestFilter{
		CategorySlug: r.URL.Query().Get("categorySlug"),
		Currency:     r.URL.Query().Get("currency"),
		DeliverTo:    r.URL.Query().Get("deliverTo"),
		Status:       r.URL.Query().Get("status"),
		Page:         page,
		PageSize:     pageSize,
	}

	requests, total, err := svc.List(r.Context(), filter)
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	dtos := make([]*buyRequest, 0, len(requests))
	for _, m := range requests {
		dtos = append(dtos, toBuyRequestDTO(m))
	}
	response.Success(w, buyRequestListPayload{Requests: dtos, Total: total, Page: page, PageSize: pageSize})
}

func (g *Gateway) handleGetBuyRequest(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	request, offers, err := svc.Get(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	offerDTOs := make([]*buyRequestOffer, 0, len(offers))
	for _, o := range offers {
		offerDTOs = append(offerDTOs, toBuyRequestOfferDTO(o))
	}
	response.Success(w, buyRequestDetail{Request: toBuyRequestDTO(request), Offers: offerDTOs})
}

func (g *Gateway) handleCreateBuyRequest(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	var body createBuyRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	expiresAt, err := resolveExpiry(body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidation, err.Error())
		return
	}

	in := contracts.CreateBuyRequestInput{
		Title:              body.Title,
		CategorySlug:       body.CategorySlug,
		Quantity:           body.Quantity,
		Unit:               body.Unit,
		BudgetMaxAmount:    body.BudgetMaxAmount,
		BudgetMaxCurrency:  body.BudgetMaxCurrency,
		BudgetMinAmount:    body.BudgetMinAmount,
		AcceptedCurrencies: body.AcceptedCurrencies,
		DeliverTo:          body.DeliverTo,
		Description:        body.Description,
		Condition:          body.Condition,
		ExpiresAt:          expiresAt,
	}
	if body.BudgetMax != nil {
		in.BudgetMaxAmount = body.BudgetMax.Amount
		in.BudgetMaxCurrency = body.BudgetMax.Currency
	}
	if body.BudgetMin != nil {
		in.BudgetMinAmount = body.BudgetMin.Amount
	}

	created, err := svc.Create(r.Context(), actingBuyRequestPeerID(r), in)
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Created(w, toBuyRequestDTO(created))
}

func (g *Gateway) handleListMyBuyRequests(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 20)
	requests, total, err := svc.ListMine(r.Context(), actingBuyRequestPeerID(r), page, pageSize)
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	dtos := make([]*buyRequest, 0, len(requests))
	for _, m := range requests {
		dtos = append(dtos, toBuyRequestDTO(m))
	}
	response.Success(w, buyRequestListPayload{Requests: dtos, Total: total, Page: page, PageSize: pageSize})
}

func (g *Gateway) handleCancelBuyRequest(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	request, err := svc.Cancel(r.Context(), actingBuyRequestPeerID(r), chi.URLParam(r, "slug"))
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Success(w, toBuyRequestDTO(request))
}

func (g *Gateway) handleCreateBuyRequestOffer(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	var body createBuyRequestOfferBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	in := contracts.CreateBuyRequestOfferInput{
		UnitPriceAmount:   body.UnitPriceAmount,
		UnitPriceCurrency: body.UnitPriceCurrency,
		Quantity:          body.Quantity,
		LeadTimeDays:      body.LeadTimeDays,
		ShipsFrom:         body.ShipsFrom,
		Note:              body.Note,
	}
	if body.UnitPrice != nil {
		in.UnitPriceAmount = body.UnitPrice.Amount
		in.UnitPriceCurrency = body.UnitPrice.Currency
	}

	offer, err := svc.SubmitOffer(r.Context(), actingBuyRequestPeerID(r), chi.URLParam(r, "slug"), in)
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Created(w, toBuyRequestOfferDTO(offer))
}

func (g *Gateway) handleAcceptBuyRequestOffer(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	var body acceptBuyRequestOfferBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	slugParam := chi.URLParam(r, "slug")
	request, offer, err := svc.AcceptOffer(r.Context(), actingBuyRequestPeerID(r), slugParam, body.OfferID)
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}

	subtotal, _ := multiplyAmount(offer.UnitPriceAmount, offer.Quantity)
	response.Success(w, acceptBuyRequestOffer{
		Request: toBuyRequestDTO(request),
		OrderDraft: buyRequestOrderDraft{
			Slug:            request.Slug,
			VendorID:        toBuyRequestParty(offer.SellerPeerID),
			Title:           request.Title,
			Quantity:        offer.Quantity,
			UnitPrice:       buyRequestAmount{Amount: offer.UnitPriceAmount, Currency: offer.UnitPriceCurrency},
			Subtotal:        buyRequestAmount{Amount: subtotal, Currency: offer.UnitPriceCurrency},
			Currency:        offer.UnitPriceCurrency,
			ShipsFrom:       offer.ShipsFrom,
			DeliverTo:       request.DeliverTo,
			AcceptedOfferID: offer.ID,
		},
	})
}
