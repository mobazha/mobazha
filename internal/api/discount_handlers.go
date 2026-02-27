package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

const maxPageSize = 100

func getDiscountService(r *http.Request) (contracts.DiscountService, bool) {
	dp, ok := getNodeService(r).(contracts.DiscountProvider)
	if !ok {
		return nil, false
	}
	svc := dp.Discount()
	if svc == nil {
		return nil, false
	}
	return svc, true
}

func (g *Gateway) handleCreateDiscount(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	var d models.Discount
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if err := svc.CreateDiscount(r.Context(), &d); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Created(w, d)
}

func (g *Gateway) handleListDiscounts(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	filter := contracts.DiscountFilter{
		Page:     intQueryParam(r, "page", 1),
		PageSize: intQueryParam(r, "pageSize", 20),
	}
	if s := r.URL.Query().Get("status"); s != "" {
		status := models.DiscountStatus(s)
		filter.Status = &status
	}
	if m := r.URL.Query().Get("method"); m != "" {
		method := models.DiscountMethod(m)
		filter.Method = &method
	}
	if q := r.URL.Query().Get("q"); q != "" {
		filter.SearchTerm = q
	}

	discounts, total, err := svc.ListDiscounts(r.Context(), filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.List(w, discounts, response.Meta{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	})
}

func (g *Gateway) handleGetDiscount(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	id := mux.Vars(r)["discountID"]
	d, err := svc.GetDiscount(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "Discount not found")
		return
	}
	response.Success(w, d)
}

func (g *Gateway) handleUpdateDiscount(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	id := mux.Vars(r)["discountID"]
	existing, err := svc.GetDiscount(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "Discount not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(existing); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}
	existing.ID = id

	if err := svc.UpdateDiscount(r.Context(), existing); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(w, existing)
}

func (g *Gateway) handleDeleteDiscount(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	id := mux.Vars(r)["discountID"]
	if err := svc.DeleteDiscount(r.Context(), id); err != nil {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, err.Error())
		return
	}
	response.NoContent(w)
}

func (g *Gateway) handleAddDiscountCodes(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	discountID := mux.Vars(r)["discountID"]

	var req struct {
		Codes    []models.DiscountCode `json:"codes"`
		Generate *struct {
			Count  int    `json:"count"`
			Prefix string `json:"prefix"`
		} `json:"generate,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if req.Generate != nil {
		codes, err := svc.GenerateCodes(r.Context(), discountID, req.Generate.Count, req.Generate.Prefix)
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
			return
		}
		response.Created(w, codes)
		return
	}

	if len(req.Codes) == 0 {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "codes or generate is required")
		return
	}
	if err := svc.AddCodes(r.Context(), discountID, req.Codes); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Created(w, req.Codes)
}

func (g *Gateway) handleListDiscountCodes(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	discountID := mux.Vars(r)["discountID"]
	codes, err := svc.ListCodes(r.Context(), discountID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.Success(w, codes)
}

func (g *Gateway) handleDeleteDiscountCode(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	codeID := mux.Vars(r)["codeID"]
	if err := svc.DeleteCode(r.Context(), codeID); err != nil {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, err.Error())
		return
	}
	response.NoContent(w)
}

func (g *Gateway) handleListDiscountRedemptions(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	discountID := mux.Vars(r)["discountID"]
	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 20)

	redemptions, total, err := svc.ListRedemptions(r.Context(), discountID, page, pageSize)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.List(w, redemptions, response.Meta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

// handleValidateDiscount is public: validates a discount code for the storefront.
// customerPeerID for per-customer limit checking is deferred to P2.
func (g *Gateway) handleValidateDiscount(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "code is required")
		return
	}

	result, err := svc.ValidateCode(r.Context(), req.Code, "")
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	if !result.Valid {
		response.Error(w, http.StatusBadRequest, response.CodeValidation, result.Reason)
		return
	}

	resp := map[string]interface{}{
		"valid":     true,
		"title":     result.Discount.Title,
		"valueType": result.Discount.ValueType,
		"value":     result.Discount.Value,
	}
	if result.Discount.MaxDiscountAmount != nil {
		resp["maxDiscountAmount"] = *result.Discount.MaxDiscountAmount
	}
	response.Success(w, resp)
}

// handleGetApplicableDiscounts is public: returns active automatic discounts for buyers.
func (g *Gateway) handleGetApplicableDiscounts(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDiscountService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Discounts not available")
		return
	}

	var productIDs []string
	if slug := r.URL.Query().Get("listingSlug"); slug != "" {
		productIDs = []string{slug}
	}

	discounts, err := svc.GetApplicableDiscounts(r.Context(), productIDs)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	type discountSummary struct {
		Title           string `json:"title"`
		ValueType       string `json:"valueType"`
		Value           string `json:"value,omitempty"`
		Currency        string `json:"currency,omitempty"`
		MinPurchaseType string `json:"minPurchaseType,omitempty"`
		MinAmount       string `json:"minAmount,omitempty"`
	}

	summaries := make([]discountSummary, 0, len(discounts))
	for _, d := range discounts {
		s := discountSummary{
			Title:     d.Title,
			ValueType: string(d.ValueType),
			Value:     d.Value,
			Currency:  d.Currency,
		}
		if d.MinPurchaseType != models.DiscountMinPurchaseNone {
			s.MinPurchaseType = string(d.MinPurchaseType)
			if d.MinAmount != nil {
				s.MinAmount = *d.MinAmount
			}
		}
		summaries = append(summaries, s)
	}
	response.Success(w, summaries)
}

func intQueryParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return defaultVal
	}
	if key == "pageSize" && v > maxPageSize {
		return maxPageSize
	}
	return v
}
