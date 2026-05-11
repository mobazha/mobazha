package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

func fulfillmentWebhookBaseURL(r *http.Request, providerID string) string {
	scheme := "https"
	if r.TLS == nil {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else {
			scheme = "http"
		}
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	return fmt.Sprintf("%s://%s/v1/fulfillment/%s/webhooks", scheme, host, providerID)
}

// getSupplyChainService extracts the SupplyChainService from the request's
// NodeService via the SupplyChainProvider type assertion. Returns nil when
// the node does not support supply chain or the feature flag is off.
//
// Feature gating uses the per-node Resolver (SSOT) rather than the gateway's
// legacy *FeatureManager: the latter only reads DefaultValue and would
// silently 404 every fulfillment endpoint when hosting flips the platform
// flag at runtime. See pkg/config/feature_manager.go for context (TD-098).
func (g *Gateway) getSupplyChainService(r *http.Request) (contracts.SupplyChainService, bool) {
	ns := getNodeService(r)
	if fp, ok := ns.(contracts.FeaturesProvider); ok {
		if res := fp.Features(); res != nil &&
			!res.IsEnabled(r.Context(), pkgconfig.FeatureSupplyChainEnabled.Key) {
			return nil, false
		}
	}
	sc, ok := ns.(contracts.SupplyChainProvider)
	if !ok {
		return nil, false
	}
	svc := sc.SupplyChain()
	if svc == nil {
		return nil, false
	}
	return svc, true
}

func fulfillmentNotAvailable(w http.ResponseWriter) {
	responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented,
		"Fulfillment providers not available")
}

// ---------------------------------------------------------------------------
// Provider management
// ---------------------------------------------------------------------------

func (g *Gateway) handleGETFulfillmentProviders(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}
	connections, err := svc.ListConnections(r.Context())
	if err != nil {
		log.Warningf("Failed to list fulfillment providers: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to list fulfillment providers")
		return
	}
	if connections == nil {
		connections = []contracts.ProviderConnection{}
	}
	responsePkg.Success(w, connections)
}

func (g *Gateway) handlePOSTConnectFulfillmentProvider(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	var req contracts.ConnectProviderParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
			"Invalid request body")
		return
	}
	req.ProviderID = providerID
	req.WebhookBaseURL = fulfillmentWebhookBaseURL(r, providerID)

	conn, err := svc.ConnectProvider(r.Context(), req)
	if err != nil {
		log.Warningf("Failed to connect fulfillment provider %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeProviderError,
			"Failed to connect provider")
		return
	}
	responsePkg.Created(w, conn)
}

func (g *Gateway) handleDELETEDisconnectFulfillmentProvider(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	if err := svc.DisconnectProvider(r.Context(), providerID); err != nil {
		log.Warningf("Failed to disconnect fulfillment provider %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to disconnect provider")
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleGETFulfillmentProviderStatus(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	status, err := svc.GetProviderStatus(r.Context(), providerID)
	if err != nil {
		if isNotFound(err) {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound,
				"Provider not found")
			return
		}
		log.Warningf("Failed to get provider status %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to get provider status")
		return
	}
	responsePkg.Success(w, status)
}

// ---------------------------------------------------------------------------
// Catalog browsing
// ---------------------------------------------------------------------------

func (g *Gateway) handleGETFulfillmentCatalog(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	searchParam := r.URL.Query().Get("search")
	query := contracts.CatalogQuery{
		CategoryID: r.URL.Query().Get("categoryId"),
		Search:     searchParam,
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		query.Offset = parseInt(v, 0)
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		query.Limit = parseInt(v, 20)
	}

	page, err := svc.BrowseCatalog(r.Context(), providerID, query)
	if err != nil {
		log.Warningf("Failed to browse catalog %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to browse catalog")
		return
	}
	if searchParam != "" && !page.SearchSupported {
		w.Header().Set("X-Warning", "search parameter is not supported by this provider; results are unfiltered")
	}
	responsePkg.Success(w, page)
}

func (g *Gateway) handleGETFulfillmentCatalogProduct(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	product, err := svc.GetCatalogProduct(r.Context(), chi.URLParam(r, "providerID"), chi.URLParam(r, "productID"))
	if err != nil {
		log.Warningf("Failed to get catalog product: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to get catalog product")
		return
	}
	responsePkg.Success(w, product)
}

// ---------------------------------------------------------------------------
// Store sync products (designed in supplier dashboard)
// ---------------------------------------------------------------------------

func (g *Gateway) handleGETStoreSyncProducts(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	offset := parseInt(r.URL.Query().Get("offset"), 0)
	limit := parseInt(r.URL.Query().Get("limit"), 20)

	page, err := svc.BrowseStoreSyncProducts(r.Context(), providerID, offset, limit)
	if err != nil {
		if errors.Is(err, contracts.ErrFulfillmentNotImplemented) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented,
				"Store sync products not supported by this provider")
			return
		}
		log.Warningf("Failed to list store sync products for %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to list store sync products")
		return
	}
	responsePkg.Success(w, page)
}

func (g *Gateway) handleGETStoreSyncProduct(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	product, err := svc.GetStoreSyncProduct(r.Context(), chi.URLParam(r, "providerID"), chi.URLParam(r, "syncProductID"))
	if err != nil {
		if errors.Is(err, contracts.ErrFulfillmentNotImplemented) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented,
				"Store sync products not supported by this provider")
			return
		}
		log.Warningf("Failed to get store sync product: %v", err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to get store sync product")
		return
	}
	responsePkg.Success(w, product)
}

// ---------------------------------------------------------------------------
// Fulfillment Locations
// ---------------------------------------------------------------------------

func (g *Gateway) handleGETFulfillmentLocations(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}
	locs, err := svc.ListLocations(r.Context())
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to list locations")
		return
	}
	responsePkg.Success(w, locs)
}

func (g *Gateway) handleGETFulfillmentLocation(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}
	locationID := chi.URLParam(r, "locationID")
	loc, err := svc.GetLocation(r.Context(), locationID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound,
				"Location not found")
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to get location")
		return
	}
	responsePkg.Success(w, loc)
}

// ---------------------------------------------------------------------------
// Product import & sync
// ---------------------------------------------------------------------------

func (g *Gateway) handlePOSTImportFulfillmentProduct(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	var req contracts.ImportProductParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
			"Invalid request body")
		return
	}
	req.ProviderID = providerID

	result, err := svc.ImportProduct(r.Context(), req)
	if err != nil {
		if errors.Is(err, contracts.ErrFulfillmentNotImplemented) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented,
				"Product import is not yet available (planned for FF-1.4a)")
			return
		}
		log.Warningf("Failed to import product from %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to import product")
		return
	}
	responsePkg.Created(w, result)
}

func (g *Gateway) handleGETSyncedProducts(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	products, err := svc.ListSyncedProducts(r.Context(), providerID)
	if err != nil {
		log.Warningf("Failed to list synced products for %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to list synced products")
		return
	}
	if products == nil {
		products = []contracts.SyncedProduct{}
	}
	responsePkg.Success(w, products)
}

func (g *Gateway) handleDELETESyncedProduct(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	mappingID := chi.URLParam(r, "mappingID")
	if mappingID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
			"Mapping ID is required")
		return
	}

	if err := svc.UnlinkSyncedProduct(r.Context(), providerID, mappingID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound,
				"Synced product mapping not found")
			return
		}
		log.Warningf("Failed to unlink synced product %s: %v", mappingID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to unlink synced product")
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTSyncProduct(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	slug := chi.URLParam(r, "slug")
	status, err := svc.SyncProduct(r.Context(), slug)
	if err != nil {
		if errors.Is(err, contracts.ErrFulfillmentNotImplemented) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented,
				"Product sync is not yet available")
			return
		}
		if strings.Contains(err.Error(), "not found") {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, err.Error())
			return
		}
		log.Warningf("Failed to sync product %s: %v", slug, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to sync product")
		return
	}
	responsePkg.Success(w, status)
}

// ---------------------------------------------------------------------------
// Order fulfillment status
// ---------------------------------------------------------------------------

func (g *Gateway) handleGETFulfillmentOrderStatus(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	orderID := chi.URLParam(r, "orderID")
	fo, err := svc.GetFulfillmentStatus(r.Context(), orderID)
	if err != nil {
		if isNotFound(err) {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound,
				"No fulfillment record for this order")
			return
		}
		log.Warningf("Failed to get fulfillment status for order %s: %v", orderID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to get fulfillment status")
		return
	}
	responsePkg.Success(w, fo)
}

// ---------------------------------------------------------------------------
// Shipping estimation
// ---------------------------------------------------------------------------

func (g *Gateway) handlePOSTEstimateShipping(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	var req contracts.ShippingEstimateParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
			"Invalid request body")
		return
	}

	estimates, err := svc.EstimateShipping(r.Context(), providerID, req)
	if err != nil {
		log.Warningf("Failed to estimate shipping for %s: %v", providerID, err)
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"Failed to estimate shipping")
		return
	}
	responsePkg.Success(w, estimates)
}

// ---------------------------------------------------------------------------
// Webhook
// ---------------------------------------------------------------------------

func (g *Gateway) handlePOSTFulfillmentWebhook(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	providerID := chi.URLParam(r, "providerID")
	webhookSecret := chi.URLParam(r, "webhookSecret")

	if webhookSecret == "" || !svc.ValidateWebhookSecret(r.Context(), providerID, webhookSecret) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	if err := svc.HandleProviderWebhook(r.Context(), providerID, body, headers); err != nil {
		log.Warningf("Fulfillment webhook error (%s): %v", providerID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Alerts (M6)
// ---------------------------------------------------------------------------

func (g *Gateway) handleGETFulfillmentAlerts(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	dismissed := r.URL.Query().Get("dismissed") == "true"
	limit := parseInt(r.URL.Query().Get("limit"), 50)

	alerts, err := svc.ListAlerts(r.Context(), dismissed, limit)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to list alerts")
		return
	}
	if alerts == nil {
		alerts = []contracts.SupplyChainAlert{}
	}
	responsePkg.Success(w, alerts)
}

func (g *Gateway) handleDELETEFulfillmentAlert(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	alertID := chi.URLParam(r, "alertID")
	if alertID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "alertID is required")
		return
	}

	if err := svc.DismissAlert(r.Context(), alertID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "Alert not found")
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to dismiss alert")
		return
	}
	responsePkg.NoContent(w)
}

// ---------------------------------------------------------------------------
// Auto-Action Rules (M6)
// ---------------------------------------------------------------------------

func (g *Gateway) handleGETFulfillmentRules(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	rules, err := svc.ListRules(r.Context())
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to list rules")
		return
	}
	if rules == nil {
		rules = []contracts.AutoActionRule{}
	}
	responsePkg.Success(w, rules)
}

func (g *Gateway) handlePOSTFulfillmentRule(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	var rule contracts.AutoActionRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Invalid request body")
		return
	}
	if rule.Trigger == "" || rule.Action == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "trigger and action are required")
		return
	}

	if err := svc.CreateRule(r.Context(), &rule); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to create rule")
		return
	}
	responsePkg.Created(w, rule)
}

func (g *Gateway) handleDELETEFulfillmentRule(w http.ResponseWriter, r *http.Request) {
	svc, ok := g.getSupplyChainService(r)
	if !ok {
		fulfillmentNotAvailable(w)
		return
	}

	ruleID := chi.URLParam(r, "ruleID")
	if ruleID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "ruleID is required")
		return
	}

	if err := svc.DeleteRule(r.Context(), ruleID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "Rule not found")
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to delete rule")
		return
	}
	responsePkg.NoContent(w)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func isNotFound(err error) bool {
	return err != nil && (err == contracts.ErrFulfillmentProviderNotFound ||
		err == contracts.ErrFulfillmentOrderNotFound)
}

func parseInt(s string, defaultVal int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}
