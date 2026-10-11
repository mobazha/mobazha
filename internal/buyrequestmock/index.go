// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package buyrequestmock is a local stand-in for the hosted index's buyer
// request endpoints (RFC-0017, "Index interfaces").
//
// It exists so the node and the web client can exercise the real wire contract
// — signed publication, revision growth, withdrawal, listing, search, detail —
// without a development machine ever touching the production index. It applies
// the rules the hosted service is specified to apply, so a contract test against
// it is meaningful, and it is deliberately in-memory: restarting it throws the
// test data away.
package buyrequestmock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mobazha/mobazha/pkg/orders"
	opb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

// MaxPageSize mirrors the hosted service's list limit.
const MaxPageSize = 100

// publication is what a node posts to publish or update a request.
type publication struct {
	PeerID            string `json:"peerID"`
	SerializedRequest []byte `json:"serializedRequest"`
	Signature         []byte `json:"signature"`
}

// withdrawal is what a node posts to take a request off the board.
type withdrawal struct {
	PeerID    string `json:"peerID"`
	Revision  uint32 `json:"revision"`
	Signature []byte `json:"signature"`
}

// Config tunes the mock for a test.
type Config struct {
	// BannedPeers is the ban list the hosted service applies before showing or
	// storing anything from a node.
	BannedPeers []string
	// Now lets a test move the clock, so the expiry rules are testable.
	Now func() time.Time
	// Logger receives one line per accepted or refused call; nil is silent.
	Logger func(format string, args ...any)
}

// Index is an in-memory buyer-request index.
type Index struct {
	mu      sync.Mutex
	records map[string]*opb.BuyerRequest
	owners  map[string]string // requestID -> owning peer
	config  Config
}

// NewIndex builds an empty index.
func NewIndex(cfg Config) *Index {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Index{records: make(map[string]*opb.BuyerRequest), owners: make(map[string]string), config: cfg}
}

// Handler returns the four endpoints the hosted service is specified to expose.
func (i *Index) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /search/v1/netdb/buyer-requests", i.handlePublish)
	mux.HandleFunc("DELETE /search/v1/netdb/buyer-requests/{requestID}", i.handleWithdraw)
	mux.HandleFunc("GET /search/v1/buyer-requests", i.handleList)
	mux.HandleFunc("GET /search/v1/buyer-requests/{peerID}/{requestID}", i.handleDetail)
	return mux
}

func (i *Index) logf(format string, args ...any) {
	if i.config.Logger != nil {
		i.config.Logger(format, args...)
	}
}

func (i *Index) banned(peerID string) bool {
	for _, banned := range i.config.BannedPeers {
		if banned == peerID {
			return true
		}
	}
	return false
}

// openCount counts the records a node currently has listed, excluding one
// identifier so an update does not count against its own quota.
func (i *Index) openCount(peerID, exceptRequestID string) int {
	now := i.config.Now()
	count := 0
	for requestID, request := range i.records {
		if request.GetPeerID() != peerID || requestID == exceptRequestID {
			continue
		}
		if orders.IsBuyerRequestListed(request, now) {
			count++
		}
	}
	return count
}

func (i *Index) handlePublish(w http.ResponseWriter, r *http.Request) {
	var body publication
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be JSON")
		return
	}
	if i.banned(body.PeerID) {
		writeError(w, http.StatusForbidden, "banned", "this node is not allowed to publish")
		return
	}

	request, err := orders.VerifyBuyerRequest(body.PeerID, body.SerializedRequest, body.Signature)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	if err := orders.ValidateBuyerRequest(request, i.config.Now()); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if existing, ok := i.records[request.GetRequestID()]; ok {
		if request.GetRevision() <= existing.GetRevision() {
			writeError(w, http.StatusConflict, "CONFLICT",
				fmt.Sprintf("stored revision is %d", existing.GetRevision()))
			return
		}
	}
	if i.openCount(request.GetPeerID(), request.GetRequestID()) >= orders.BuyerRequestMaxOpenPerPeer {
		writeError(w, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("a node may keep at most %d requests open", orders.BuyerRequestMaxOpenPerPeer))
		return
	}

	i.records[request.GetRequestID()] = request
	i.owners[request.GetRequestID()] = request.GetPeerID()
	i.logf("ACCEPT publish %s revision=%d peer=%s", request.GetRequestID(), request.GetRevision(), request.GetPeerID())
	writeData(w, http.StatusOK, map[string]any{
		"requestID": request.GetRequestID(),
		"revision":  request.GetRevision(),
	})
}

func (i *Index) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	var body withdrawal
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be JSON")
		return
	}
	payload := orders.BuyerRequestWithdrawPayload(body.PeerID, requestID, body.Revision)
	if err := orders.VerifyBuyerRequestSignature(body.PeerID, payload, body.Signature); err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	owner, ok := i.owners[requestID]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "no such request")
		return
	}
	if owner != body.PeerID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "only the owning node may withdraw a request")
		return
	}
	if stored := i.records[requestID]; stored != nil && body.Revision < stored.GetRevision() {
		writeError(w, http.StatusConflict, "CONFLICT",
			fmt.Sprintf("stored revision is %d", stored.GetRevision()))
		return
	}
	delete(i.records, requestID)
	delete(i.owners, requestID)
	i.logf("ACCEPT withdraw %s by %s", requestID, body.PeerID)
	writeData(w, http.StatusOK, map[string]any{"requestID": requestID})
}

func (i *Index) handleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	now := i.config.Now()

	page := positiveInt(query.Get("p"), 1)
	pageSize := positiveInt(query.Get("pageSize"), 20)
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	needle := strings.ToLower(strings.TrimSpace(query.Get("q")))
	productType := strings.TrimSpace(query.Get("productType"))
	shipTo := strings.TrimSpace(query.Get("shipTo"))
	currency := strings.TrimSpace(query.Get("currency"))
	minBudget := strings.TrimSpace(query.Get("minBudget"))
	maxBudget := strings.TrimSpace(query.Get("maxBudget"))
	budgetCurrency := strings.TrimSpace(query.Get("budgetCurrency"))

	i.mu.Lock()
	matched := make([]*opb.BuyerRequest, 0, len(i.records))
	for _, request := range i.records {
		if i.banned(request.GetPeerID()) || !orders.IsBuyerRequestListed(request, now) {
			continue
		}
		if productType != "" && request.GetProductType() != productType {
			continue
		}
		if shipTo != "" && !strings.EqualFold(request.GetShipTo(), shipTo) {
			continue
		}
		if currency != "" && !containsFold(request.GetAcceptedCurrencies(), currency) {
			continue
		}
		if (minBudget != "" || maxBudget != "") && !budgetMatches(request, minBudget, maxBudget, budgetCurrency) {
			continue
		}
		if needle != "" && !recordMatches(request, needle) {
			continue
		}
		matched = append(matched, request)
	}
	i.mu.Unlock()

	switch query.Get("sortBy") {
	case "expires-asc":
		sort.SliceStable(matched, func(a, b int) bool {
			return matched[a].GetExpiresAt().AsTime().Before(matched[b].GetExpiresAt().AsTime())
		})
	default: // added-desc
		sort.SliceStable(matched, func(a, b int) bool {
			return matched[a].GetCreatedAt().AsTime().After(matched[b].GetCreatedAt().AsTime())
		})
	}

	total := len(matched)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	// Each item is wrapped like a listing hit, so a client can read a request
	// from the index the same way it reads anything else the index returns.
	items := make([]map[string]any, 0, end-start)
	for _, request := range matched[start:end] {
		items = append(items, map[string]any{"data": recordJSON(request), "type": "buyer_request"})
	}
	writeRaw(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]any{"total": total, "hasMore": end < total},
	})
}

func (i *Index) handleDetail(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	peerID := strings.TrimSpace(r.PathValue("peerID"))
	now := i.config.Now()

	i.mu.Lock()
	request := i.records[requestID]
	i.mu.Unlock()

	if request == nil || request.GetPeerID() != peerID || i.banned(peerID) ||
		!orders.IsBuyerRequestListed(request, now) {
		writeError(w, http.StatusNotFound, "not_found", "no such request")
		return
	}
	writeData(w, http.StatusOK, recordJSON(request))
}

func recordMatches(request *opb.BuyerRequest, needle string) bool {
	fields := []string{request.GetTitle(), request.GetDescription(), request.GetProductType()}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

// budgetMatches compares minor-unit amounts, and only when the caller names the
// currency the amounts are counted in.
func budgetMatches(request *opb.BuyerRequest, min, max, currency string) bool {
	budget := request.GetBudget()
	if budget == nil || budget.GetCurrency() == nil {
		return false
	}
	if currency != "" && !strings.EqualFold(budget.GetCurrency().GetCode(), currency) {
		return false
	}
	amount, err := strconv.ParseUint(strings.TrimSpace(budget.GetAmount()), 10, 64)
	if err != nil {
		return false
	}
	if min != "" {
		if lower, err := strconv.ParseUint(min, 10, 64); err != nil || amount < lower {
			return false
		}
	}
	if max != "" {
		if upper, err := strconv.ParseUint(max, 10, 64); err != nil || amount > upper {
			return false
		}
	}
	return true
}

func positiveInt(raw string, fallback int) int {
	if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && value > 0 {
		return value
	}
	return fallback
}

// recordJSON renders a stored record the way a node renders one, so a client
// reads the index and a node the same way.
func recordJSON(request *opb.BuyerRequest) json.RawMessage {
	marshaler := protojson.MarshalOptions{EmitUnpopulated: false}
	out, err := marshaler.Marshal(request)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(out)
}

func writeData(w http.ResponseWriter, status int, payload any) {
	writeRaw(w, status, map[string]any{"data": payload})
}

func writeRaw(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeRaw(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
