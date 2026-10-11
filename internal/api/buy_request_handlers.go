// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/orders"
	"github.com/mobazha/mobazha/pkg/response"
)

// maxBuyRequestDays caps how far ahead a request may be scheduled. The ceiling
// comes from the record contract, so a buyer learns the limit from this node
// instead of from a push the index refuses.
var maxBuyRequestDays = int(orders.BuyerRequestMaxLifetime.Hours() / 24)

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

// buyRequestOwnerQueryParam names the node that owns a request. A request lives
// on the node its buyer published it on, so a reader anywhere else names that
// node and the call is relayed to it over the native p2p channel.
const buyRequestOwnerQueryParam = "peer"

// getBuyRequestRelay returns this node's cross-node transport for the board. It
// is absent on a node without a p2p stack, where the board is local.
func getBuyRequestRelay(r *http.Request) (contracts.BuyRequestRelay, bool) {
	provider, ok := getNodeService(r).(contracts.BuyRequestRelayProvider)
	if !ok {
		return nil, false
	}
	relay := provider.BuyRequestRelay()
	if relay == nil {
		return nil, false
	}
	return relay, true
}

// buyRequestOwner returns the peer that owns the request this call addresses,
// or "" when that is this node. Naming this node explicitly means the same as
// leaving it out, so a link copied here keeps working here.
func buyRequestOwner(r *http.Request) string {
	owner := strings.TrimSpace(r.URL.Query().Get(buyRequestOwnerQueryParam))
	if owner == "" {
		return ""
	}
	if identity := getNodeService(r).IdentityInfo(); identity != nil && identity.Identity().String() == owner {
		return ""
	}
	return owner
}

// writeRelayedData forwards an answer the owning node already rendered in the
// shape this API serves. It is passed through verbatim: this node does not own
// the request, so it must not re-encode an answer it did not produce.
func writeRelayedData(w http.ResponseWriter, status int, payload []byte) {
	response.StatusWithData(w, status, json.RawMessage(payload))
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
	case errors.Is(err, database.ErrBuyRequestOwnerUnreachable):
		// The request itself may be perfectly fine: its node is the one that
		// did not answer, so this is reported apart from a refusal.
		response.Error(w, http.StatusGatewayTimeout, response.CodeServiceUnavail, err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
	}
}

func (g *Gateway) handleListBuyRequestComments(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	if owner := buyRequestOwner(r); owner != "" {
		relay, ok := getBuyRequestRelay(r)
		if !ok {
			response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
			return
		}
		payload, relayErr := relay.AskComments(r.Context(), owner, chi.URLParam(r, "slug"))
		if relayErr != nil {
			buyRequestErrorResponse(w, relayErr)
			return
		}
		writeRelayedData(w, http.StatusOK, payload)
		return
	}

	comments, err := svc.ListComments(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Success(w, contracts.NewBuyRequestCommentThreadDTO(comments))
}

func (g *Gateway) handleCreateBuyRequestComment(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	var body contracts.CreateBuyRequestCommentRequest
	if owner := buyRequestOwner(r); owner != "" {
		relay, ok := getBuyRequestRelay(r)
		if !ok {
			response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
			return
		}
		// The body is relayed as it arrived: it is the owning node that
		// decides whether this is a well-formed comment.
		raw, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
			return
		}
		payload, relayErr := relay.RelayComment(r.Context(), owner, chi.URLParam(r, "slug"), raw)
		if relayErr != nil {
			buyRequestErrorResponse(w, relayErr)
			return
		}
		writeRelayedData(w, http.StatusCreated, payload)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	comment, err := svc.CreateComment(r.Context(), actingBuyRequestPeerID(r), chi.URLParam(r, "slug"),
		body.ToInput())
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Created(w, contracts.NewBuyRequestCommentDTO(comment))
}

// --- request bodies ---

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

// createBuyRequestBody is the body a buyer posts, and the same body an edit
// sends. The field names are the record contract's (RFC-0017 §"Records"), so a
// client that speaks the record speaks this API; the older names are still
// accepted so a released client keeps working.
type createBuyRequestBody struct {
	Title       string `json:"title"`
	ProductType string `json:"productType"`
	// CategorySlug is the previous name for ProductType.
	CategorySlug string `json:"categorySlug"`
	Quantity     int    `json:"quantity"`
	// Budget is the ceiling: an amount in the currency's smallest unit plus the
	// currency it is counted in.
	Budget *contracts.BuyRequestAmountDTO `json:"budget"`
	// BudgetMax and BudgetMaxAmount/BudgetMaxCurrency are the previous forms of
	// the same two values.
	BudgetMax          *contracts.BuyRequestAmountDTO `json:"budgetMax"`
	BudgetMaxAmount    string                         `json:"budgetMaxAmount"`
	BudgetMaxCurrency  string                         `json:"budgetMaxCurrency"`
	AcceptedCurrencies []string                       `json:"acceptedCurrencies"`
	ShipTo             string                         `json:"shipTo"`
	// DeliverTo is the previous name for ShipTo.
	DeliverTo   string `json:"deliverTo"`
	Description string `json:"description"`
	// ExpiresAt takes an RFC3339 timestamp; ExpiresInDays is the friendlier
	// "keep this up for N days" form the UI uses. Saying neither keeps the
	// request up for the default week.
	ExpiresAt     string `json:"expiresAt"`
	ExpiresInDays int    `json:"expiresInDays"`
	// Images is the hash set returned by POST /v1/media/product-images.
	Images []models.ImageHashes `json:"images"`
}

// budget reads the ceiling from whichever form the body used.
func (b createBuyRequestBody) budgetCeiling() contracts.BuyRequestAmountDTO {
	out := contracts.BuyRequestAmountDTO{Amount: b.BudgetMaxAmount, Currency: b.BudgetMaxCurrency}
	if b.BudgetMax != nil {
		out.Amount = firstNonEmpty(b.BudgetMax.Amount, out.Amount)
		out.Currency = firstNonEmpty(b.BudgetMax.Currency, out.Currency)
	}
	if b.Budget != nil {
		out.Amount = firstNonEmpty(b.Budget.Amount, out.Amount)
		out.Currency = firstNonEmpty(b.Budget.Currency, out.Currency)
	}
	return out
}

// toInput normalises the body into the app service input.
func (b createBuyRequestBody) toInput(expiresAt time.Time) contracts.CreateBuyRequestInput {
	budget := b.budgetCeiling()
	return contracts.CreateBuyRequestInput{
		Title:              b.Title,
		ProductType:        firstNonEmpty(b.ProductType, b.CategorySlug),
		Quantity:           b.Quantity,
		BudgetAmount:       budget.Amount,
		BudgetCurrency:     budget.Currency,
		AcceptedCurrencies: b.AcceptedCurrencies,
		ShipTo:             firstNonEmpty(b.ShipTo, b.DeliverTo),
		Description:        b.Description,
		ExpiresAt:          expiresAt,
		Images:             b.Images,
	}
}

// toUpdateInput is the same normalisation for an edit.
func (b createBuyRequestBody) toUpdateInput(expiresAt time.Time) contracts.UpdateBuyRequestInput {
	budget := b.budgetCeiling()
	return contracts.UpdateBuyRequestInput{
		Title:              b.Title,
		ProductType:        firstNonEmpty(b.ProductType, b.CategorySlug),
		Quantity:           b.Quantity,
		BudgetAmount:       budget.Amount,
		BudgetCurrency:     budget.Currency,
		AcceptedCurrencies: b.AcceptedCurrencies,
		ShipTo:             firstNonEmpty(b.ShipTo, b.DeliverTo),
		Description:        b.Description,
		ExpiresAt:          expiresAt,
		Images:             b.Images,
	}
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
	days := body.ExpiresInDays
	if days == 0 {
		// A buyer who does not say keeps the request up for the default week.
		days = int(orders.BuyerRequestDefaultLifetime.Hours() / 24)
	}
	if days < 1 || days > maxBuyRequestDays {
		return time.Time{}, fmt.Errorf("expiresInDays must be between 1 and %d", maxBuyRequestDays)
	}
	return time.Now().UTC().AddDate(0, 0, days), nil
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
		ProductType: firstNonEmpty(r.URL.Query().Get("productType"), r.URL.Query().Get("categorySlug")),
		Currency:    r.URL.Query().Get("currency"),
		ShipTo:      firstNonEmpty(r.URL.Query().Get("shipTo"), r.URL.Query().Get("deliverTo")),
		Status:      r.URL.Query().Get("status"),
		Page:        page,
		PageSize:    pageSize,
	}

	requests, total, err := svc.List(r.Context(), filter)
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	dtos := make([]*contracts.BuyRequestDTO, 0, len(requests))
	for _, m := range requests {
		dtos = append(dtos, contracts.NewBuyRequestDTO(m))
	}
	response.Success(w, contracts.BuyRequestListDTO{Requests: dtos, Total: total, Page: page, PageSize: pageSize})
}

func (g *Gateway) handleGetBuyRequest(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	if owner := buyRequestOwner(r); owner != "" {
		relay, ok := getBuyRequestRelay(r)
		if !ok {
			response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
			return
		}
		payload, relayErr := relay.AskRequest(r.Context(), owner, chi.URLParam(r, "slug"))
		if relayErr != nil {
			buyRequestErrorResponse(w, relayErr)
			return
		}
		writeRelayedData(w, http.StatusOK, payload)
		return
	}

	request, offers, err := svc.Get(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	offerDTOs := make([]*contracts.BuyRequestOfferDTO, 0, len(offers))
	for _, o := range offers {
		offerDTOs = append(offerDTOs, contracts.NewBuyRequestOfferDTO(o))
	}
	response.Success(w, contracts.BuyRequestDetailDTO{Request: contracts.NewBuyRequestDTO(request), Offers: offerDTOs})
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

	created, err := svc.Create(r.Context(), actingBuyRequestPeerID(r), body.toInput(expiresAt))
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Created(w, contracts.NewBuyRequestDTO(created))
}

// handleUpdateBuyRequest edits a request its buyer owns. Every edit moves the
// record's revision forward, which is what lets the index refuse a copy that
// has already been replaced.
func (g *Gateway) handleUpdateBuyRequest(w http.ResponseWriter, r *http.Request) {
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

	updated, err := svc.Update(r.Context(), actingBuyRequestPeerID(r), chi.URLParam(r, "slug"),
		body.toUpdateInput(expiresAt))
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Success(w, contracts.NewBuyRequestDTO(updated))
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
	dtos := make([]*contracts.BuyRequestDTO, 0, len(requests))
	for _, m := range requests {
		dtos = append(dtos, contracts.NewBuyRequestDTO(m))
	}
	response.Success(w, contracts.BuyRequestListDTO{Requests: dtos, Total: total, Page: page, PageSize: pageSize})
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
	response.Success(w, contracts.NewBuyRequestDTO(request))
}

func (g *Gateway) handleCreateBuyRequestOffer(w http.ResponseWriter, r *http.Request) {
	svc, ok := getBuyRequestService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
		return
	}

	var body contracts.CreateBuyRequestOfferRequest
	if owner := buyRequestOwner(r); owner != "" {
		relay, ok := getBuyRequestRelay(r)
		if !ok {
			response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Buy requests not available")
			return
		}
		// The offer is relayed as it arrived: the owning node validates it
		// against the request it holds and stores it under the peer this node
		// is authenticated as, so a seller cannot offer as somebody else.
		raw, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
			return
		}
		payload, relayErr := relay.RelayOffer(r.Context(), owner, chi.URLParam(r, "slug"), raw)
		if relayErr != nil {
			buyRequestErrorResponse(w, relayErr)
			return
		}
		writeRelayedData(w, http.StatusCreated, payload)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	in := body.ToInput()

	offer, err := svc.SubmitOffer(r.Context(), actingBuyRequestPeerID(r), chi.URLParam(r, "slug"), in)
	if err != nil {
		buyRequestErrorResponse(w, err)
		return
	}
	response.Created(w, contracts.NewBuyRequestOfferDTO(offer))
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
	response.Success(w, contracts.AcceptBuyRequestOfferDTO{
		Request: contracts.NewBuyRequestDTO(request),
		OrderDraft: contracts.BuyRequestOrderDraftDTO{
			Slug:            request.Slug,
			VendorID:        contracts.NewBuyRequestPartyDTO(offer.SellerPeerID),
			Title:           request.Title,
			Quantity:        offer.Quantity,
			UnitPrice:       contracts.BuyRequestAmountDTO{Amount: offer.UnitPriceAmount, Currency: offer.UnitPriceCurrency},
			Subtotal:        contracts.BuyRequestAmountDTO{Amount: subtotal, Currency: offer.UnitPriceCurrency},
			Currency:        offer.UnitPriceCurrency,
			ShipsFrom:       offer.ShipsFrom,
			DeliverTo:       request.DeliverTo,
			AcceptedOfferID: offer.ID,
		},
	})
}
