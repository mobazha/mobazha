package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

type APIError struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
}

const profileResolveConcurrency = 10

type profileDisplayInfo struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// resolveProfileDisplayInfo looks up display names and avatars for a set of
// peerIDs in parallel. Returns a best-effort map; lookup failures are silently
// skipped so a single unreachable peer never blocks the entire list response.
func resolveProfileDisplayInfo(ctx context.Context, profileSvc contracts.ProfileService, peerIDs []string) map[string]profileDisplayInfo {
	unique := make(map[string]struct{}, len(peerIDs))
	for _, id := range peerIDs {
		if id != "" {
			unique[id] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil
	}

	result := make(map[string]profileDisplayInfo, len(unique))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, profileResolveConcurrency)

	for idStr := range unique {
		pid, err := peer.Decode(idStr)
		if err != nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(peerIDStr string, peerID peer.ID) {
			defer wg.Done()
			defer func() { <-sem }()
			profile, err := profileSvc.GetProfile(ctx, peerID, nil, true)
			if err != nil || profile == nil {
				return
			}
			name := profile.Name
			if name == "" {
				name = profile.Handle
			}
			avatar := profile.AvatarHashes.Small
			if avatar == "" {
				avatar = profile.AvatarHashes.Tiny
			}
			if name != "" || avatar != "" {
				mu.Lock()
				result[peerIDStr] = profileDisplayInfo{Name: name, Avatar: avatar}
				mu.Unlock()
			}
		}(idStr, pid)
	}
	wg.Wait()
	return result
}

func ErrorResponse(w http.ResponseWriter, errorCode int, reason string) {
	log.Errorf("ErrorResponse, errorCode: %d, reason: %s ", errorCode, reason)
	responsePkg.Error(w, errorCode, responsePkg.HttpStatusToCode(errorCode), reason)
}

// orderActionErrorResponse maps order action errors to appropriate HTTP status codes.
// Relay-specific errors are mapped to 503/501 so clients know to fall back to the
// instructions + frontend wallet flow.
func orderActionErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, coreiface.ErrRelayNotAvailable):
		ErrorResponse(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, coreiface.ErrRelayChainNotSupported):
		ErrorResponse(w, http.StatusNotImplemented, err.Error())
	case errors.Is(err, models.ErrRefundAddressRequired):
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeRefundAddressRequired, err.Error())
	case errors.Is(err, coreiface.ErrBadRequest):
		ErrorResponse(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, coreiface.ErrNotFound):
		ErrorResponse(w, http.StatusNotFound, err.Error())
	default:
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

func latestSettlementSummary(order models.Order) (action, actionID, state, txHash string) {
	if len(order.SettlementActions) == 0 {
		return "", "", "", ""
	}
	latest := order.SettlementActions[0]
	return latest.Action, latest.ActionID, latest.State, latest.TxHash
}

func (g *Gateway) handlePOSTPurchase(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data models.Purchase
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	order := getOrderService(r)

	orderID, amount, err := order.PurchaseListing(r.Context(), &data)
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}
	type purchaseReturn struct {
		OrderID     string                `json:"orderID"`
		PricingCoin string                `json:"pricingCoin"`
		Amount      *models.CurrencyValue `json:"amount"`
	}
	ret := purchaseReturn{orderID.String(), data.PricingCoin, &amount}

	sanitizedJSONResponse(w, ret)
}

func (g *Gateway) handlePOSTCheckoutBreakdown(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data models.Purchase
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	order := getOrderService(r)

	orderTotals, err := order.EstimateOrderTotal(r.Context(), &data)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, orderTotals)
}

func (g *Gateway) handlePOSTEstimateTotal(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data models.Purchase
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	order := getOrderService(r)

	orderTotals, err := order.EstimateOrderTotal(r.Context(), &data)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, orderTotals.Total.String())
}

func (g *Gateway) handleGETOrder(w http.ResponseWriter, r *http.Request) {
	_, orderID := path.Split(r.URL.Path)

	orderSvc := getOrderService(r)

	order, err := orderSvc.GetOrder(orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ErrorResponse(w, http.StatusNotFound, "order not found")
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	// Legacy P2P chat unread count removed; Matrix order-scoped unread is client/Matrix room metadata.
	unreadChatMsgCount := int64(0)

	type paymentStateResp struct {
		VerificationStatus        string            `json:"verificationStatus"`
		VerificationFailureReason string            `json:"verificationFailureReason,omitempty"`
		VerificationFailedAt      *time.Time        `json:"verificationFailedAt,omitempty"`
		FiatMetadata              map[string]string `json:"fiatMetadata,omitempty"`
		// Progress carries the running "you've paid X of Y" card derived
		// from OrderPaymentState.TotalReceived + OrderOpen.Amount. The
		// AggregatingVerifier refreshes TotalReceived on every pass
		// (Phase managed EVM v0.3.0 §4.2), so the dashboard can render a
		// live progress bar even before the order flips to "verified"
		// and again afterwards if a late deposit lands. Omitted when
		// there is no OrderOpen or no positive expected amount.
		Progress *models.PaymentProgressInfo `json:"progress,omitempty"`
	}

	type OrderRespApi struct {
		Contract            *models.Order                     `json:"contract,omitempty"`
		State               string                            `json:"state,omitempty"`
		Read                bool                              `json:"read,omitempty"`
		SettlementActions   []models.SettlementActionSnapshot `json:"settlementActions,omitempty"`
		UnreadChatMessages  int64                             `json:"unreadChatMessages,omitempty"`
		Funded              bool                              `json:"funded,omitempty"`
		Completable         bool                              `json:"completable,omitempty"`
		PaidAt              *time.Time                        `json:"paidAt,omitempty"`
		ShippedAt           *time.Time                        `json:"shippedAt,omitempty"`
		CompletedAt         *time.Time                        `json:"completedAt,omitempty"`
		LastStateChangeAt   *time.Time                        `json:"lastStateChangeAt,omitempty"`
		PaymentState        paymentStateResp                  `json:"paymentState"`
		Protection          *models.OrderProtectionInfo       `json:"protection,omitempty"`
		AfterSaleDispute    *models.AfterSaleDispute          `json:"afterSaleDispute,omitempty"`
		PricingBreakdown    *orderPricingBreakdownResp        `json:"pricingBreakdown,omitempty"`
		SettlementBreakdown *orderSettlementBreakdownResp     `json:"settlementBreakdown,omitempty"`
	}

	isFunded, _ := order.IsFunded()
	fiatMeta, _ := order.GetFiatMetadata()
	if len(fiatMeta) == 0 {
		fiatMeta = nil
	}

	ret := OrderRespApi{
		Contract:           order,
		State:              order.State.String(),
		SettlementActions:  order.SettlementActions,
		UnreadChatMessages: unreadChatMsgCount,
		Funded:             isFunded,
		Completable:        order.CanComplete(),
		PaidAt:             order.PaidAt,
		ShippedAt:          order.ShippedAt,
		CompletedAt:        order.CompletedAt,
		LastStateChangeAt:  order.LastStateChangeAt,
		PaymentState: paymentStateResp{
			VerificationStatus: string(order.CurrentPaymentVerificationStatus()),
			FiatMetadata:       fiatMeta,
			Progress:           order.ComputePaymentProgress(),
		},
		Protection:          order.ComputeProtection(time.Now()),
		PricingBreakdown:    buildOrderPricingBreakdown(order, getExchangeRateService(r)),
		SettlementBreakdown: buildOrderSettlementBreakdown(order),
	}
	if order.AfterSaleDispute.Reason != "" || order.AfterSaleDispute.Description != "" || order.AfterSaleDispute.OpenedAt != nil {
		dispute := order.AfterSaleDispute
		ret.AfterSaleDispute = &dispute
	}
	if order.IsPaymentVerificationFailed() {
		ret.PaymentState.VerificationFailureReason = order.PaymentVerificationFailureReason
		ret.PaymentState.VerificationFailedAt = order.PaymentVerificationFailedAt
	}

	sanitizedJSONResponse(w, ret)
}

// handlePOSTPayment 处理支付结果通知
func (g *Gateway) handlePOSTPayment(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	orderSvc := getOrderService(r)

	var req struct {
		PaymentData *models.PaymentData `json:"paymentData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "decode request body failed: "+err.Error())
		return
	}

	if req.PaymentData != nil {
		req.PaymentData.OrderID = orderID
	}

	err := orderSvc.ProcessOrderPayment(r.Context(), req.PaymentData)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	// 返回标准成功响应
	response := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}
	sanitizedJSONResponse(w, response)
}

func (g *Gateway) getPurchasesImpl(w http.ResponseWriter, ctx context.Context, orderSvc contracts.OrderService, profileSvc contracts.ProfileService, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
	orders, total, err := orderSvc.GetPurchases(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	type purchaseInfo struct {
		OrderID            string                      `json:"orderID"`
		Slug               string                      `json:"slug"`
		Timestamp          time.Time                   `json:"timestamp"`
		Title              string                      `json:"title"`
		Thumbnail          string                      `json:"thumbnail"`
		Total              models.CurrencyValue        `json:"total"`
		VendorID           string                      `json:"vendorID"`
		VendorName         string                      `json:"vendorName"`
		VendorAvatar       string                      `json:"vendorAvatar"`
		ShippingName       string                      `json:"shippingName"`
		ShippingAddress    string                      `json:"shippingAddress"`
		CoinType           string                      `json:"coinType"`
		PaymentCoin        string                      `json:"paymentCoin"`
		State              string                      `json:"state"`
		Read               bool                        `json:"read"`
		Moderated          bool                        `json:"moderated"`
		PaymentEscrowType  string                      `json:"paymentEscrowType,omitempty"`
		SettlementAction   string                      `json:"settlementAction,omitempty"`
		SettlementActionID string                      `json:"settlementActionId,omitempty"`
		SettlementState    string                      `json:"settlementState,omitempty"`
		SettlementTxHash   string                      `json:"settlementTxHash,omitempty"`
		UnreadChatMessages int                         `json:"unreadChatMessages"`
		Protection         *models.OrderProtectionInfo `json:"protection,omitempty"`
		ContractType       string                      `json:"contractType,omitempty"`
	}

	purchases := []purchaseInfo{}
	vendorIDs := make([]string, 0, len(orders))
	now := time.Now()
	for _, order := range orders {
		orderOpen, err := order.OrderOpenMessage()
		if err != nil {
			log.Errorf("Failed to get OrderOpenMessage for order: %s", order.ID.String())
			continue
		}

		paymentSent, err := order.PaymentSentMessage()
		paymentCoin := ""
		paymentEscrowType := ""
		isModerated := false
		if err == nil {
			isModerated = paymentSent.GetSettlementSpec() != nil && paymentSent.GetSettlementSpec().GetMethod() == pb.PaymentSent_MODERATED
			paymentCoin = paymentSent.Coin
			if paymentSent.GetSettlementSpec() != nil {
				paymentEscrowType = paymentSent.GetSettlementSpec().GetEscrowType()
			}
		}

		var listingInfo *pb.Listing
		if len(orderOpen.Listings) > 0 && orderOpen.Listings[0] != nil {
			listingInfo = orderOpen.Listings[0].Listing
		} else {
			log.Errorf("Failed to get listing info for order: %s", order.ID.String())
			continue
		}

		vendorID := listingInfo.VendorID.PeerID
		vendorIDs = append(vendorIDs, vendorID)

		info := purchaseInfo{
			OrderID:   order.ID.String(),
			Slug:      listingInfo.Slug,
			Timestamp: orderOpen.Timestamp.AsTime(),
			Title:     listingInfo.Item.Title,
			Thumbnail: listingInfo.Item.Images[0].Tiny,
			Total: *models.NewCurrencyValue(
				orderOpen.Amount,
				models.CurrencyDefinitions[orderOpen.PricingCoin],
			),
			VendorID:           vendorID,
			VendorName:         listingInfo.VendorID.DisplayName(),
			VendorAvatar:       listingInfo.VendorID.DisplayAvatar(),
			ShippingName:       orderOpen.Shipping.ShipTo,
			ShippingAddress:    orderOpen.Shipping.Address,
			PaymentCoin:        paymentCoin,
			PaymentEscrowType:  paymentEscrowType,
			State:              order.State.String(),
			Read:               order.Read,
			UnreadChatMessages: order.UnreadChatMessages,
			Moderated:          isModerated,
			Protection:         order.ComputeProtection(now),
		}
		if listingInfo.Metadata != nil {
			info.ContractType = listingInfo.Metadata.ContractType.String()
		}
		info.SettlementAction, info.SettlementActionID, info.SettlementState, info.SettlementTxHash = latestSettlementSummary(order)

		purchases = append(purchases, info)
	}

	if profileSvc != nil && len(vendorIDs) > 0 {
		infoMap := resolveProfileDisplayInfo(ctx, profileSvc, vendorIDs)
		for i := range purchases {
			if info, ok := infoMap[purchases[i].VendorID]; ok {
				purchases[i].VendorName = info.Name
				purchases[i].VendorAvatar = info.Avatar
			}
		}
	}

	type purchasesResponse struct {
		QueryCount int            `json:"queryCount"`
		Purchases  []purchaseInfo `json:"purchases"`
	}
	ret := purchasesResponse{int(total), purchases}

	sanitizedJSONResponse(w, ret)
}

func (g *Gateway) handlePOSTPurchases(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var query TransactionQuery
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	orderSvc := getOrderService(r)
	profileSvc := getProfileService(r)

	g.getPurchasesImpl(w, r.Context(), orderSvc, profileSvc, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
}

func (g *Gateway) handleGETPurchases(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)
	profileSvc := getProfileService(r)

	g.getPurchasesImpl(w, r.Context(), orderSvc, profileSvc, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
}

func (g *Gateway) getSalesImpl(w http.ResponseWriter, ctx context.Context, orderSvc contracts.OrderService, profileSvc contracts.ProfileService, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
	orders, total, err := orderSvc.GetSales(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	type saleInfo struct {
		OrderID            string                      `json:"orderID"`
		Slug               string                      `json:"slug"`
		Timestamp          time.Time                   `json:"timestamp"`
		Title              string                      `json:"title"`
		Thumbnail          string                      `json:"thumbnail"`
		Total              models.CurrencyValue        `json:"total"`
		BuyerID            string                      `json:"buyerID"`
		BuyerName          string                      `json:"buyerName"`
		BuyerAvatar        string                      `json:"buyerAvatar"`
		ShippingName       string                      `json:"shippingName"`
		ShippingAddress    string                      `json:"shippingAddress"`
		CoinType           string                      `json:"coinType"`
		PaymentCoin        string                      `json:"paymentCoin"`
		State              string                      `json:"state"`
		Read               bool                        `json:"read"`
		Moderated          bool                        `json:"moderated"`
		PaymentEscrowType  string                      `json:"paymentEscrowType,omitempty"`
		SettlementAction   string                      `json:"settlementAction,omitempty"`
		SettlementActionID string                      `json:"settlementActionId,omitempty"`
		SettlementState    string                      `json:"settlementState,omitempty"`
		SettlementTxHash   string                      `json:"settlementTxHash,omitempty"`
		UnreadChatMessages int                         `json:"unreadChatMessages"`
		Protection         *models.OrderProtectionInfo `json:"protection,omitempty"`
		ContractType       string                      `json:"contractType,omitempty"`
	}

	sales := []saleInfo{}
	buyerIDs := make([]string, 0, len(orders))
	now := time.Now()
	for _, order := range orders {
		orderOpen, err := order.OrderOpenMessage()
		if err != nil {
			log.Errorf("Failed to get OrderOpenMessage for order: %s", order.ID.String())
			continue
		}

		paymentSent, err := order.PaymentSentMessage()
		isModerated := false
		paymentCoin := ""
		paymentEscrowType := ""
		if err == nil {
			isModerated = paymentSent.GetSettlementSpec() != nil && paymentSent.GetSettlementSpec().GetMethod() == pb.PaymentSent_MODERATED
			paymentCoin = paymentSent.Coin
			if paymentSent.GetSettlementSpec() != nil {
				paymentEscrowType = paymentSent.GetSettlementSpec().GetEscrowType()
			}
		}

		var listingInfo *pb.Listing
		if len(orderOpen.Listings) > 0 && orderOpen.Listings[0] != nil {
			listingInfo = orderOpen.Listings[0].Listing
		} else {
			log.Errorf("Failed to get listing info for order: %s", order.ID.String())
			continue
		}

		buyerID := orderOpen.BuyerID.PeerID
		buyerIDs = append(buyerIDs, buyerID)

		info := saleInfo{
			OrderID:   order.ID.String(),
			Slug:      listingInfo.Slug,
			Timestamp: orderOpen.Timestamp.AsTime(),
			Title:     listingInfo.Item.Title,
			Thumbnail: listingInfo.Item.Images[0].Tiny,
			Total: *models.NewCurrencyValue(
				orderOpen.Amount,
				models.CurrencyDefinitions[orderOpen.PricingCoin],
			),
			BuyerID:            buyerID,
			BuyerName:          orderOpen.BuyerID.DisplayName(),
			BuyerAvatar:        orderOpen.BuyerID.DisplayAvatar(),
			ShippingName:       orderOpen.Shipping.ShipTo,
			ShippingAddress:    orderOpen.Shipping.Address,
			PaymentCoin:        paymentCoin,
			PaymentEscrowType:  paymentEscrowType,
			State:              order.State.String(),
			Read:               order.Read,
			UnreadChatMessages: order.UnreadChatMessages,
			Moderated:          isModerated,
			Protection:         order.ComputeProtection(now),
		}
		if listingInfo.Metadata != nil {
			info.ContractType = listingInfo.Metadata.ContractType.String()
		}
		info.SettlementAction, info.SettlementActionID, info.SettlementState, info.SettlementTxHash = latestSettlementSummary(order)

		sales = append(sales, info)
	}

	if profileSvc != nil && len(buyerIDs) > 0 {
		infoMap := resolveProfileDisplayInfo(ctx, profileSvc, buyerIDs)
		for i := range sales {
			if info, ok := infoMap[sales[i].BuyerID]; ok {
				sales[i].BuyerName = info.Name
				sales[i].BuyerAvatar = info.Avatar
			}
		}
	}

	type salesResponse struct {
		QueryCount int        `json:"queryCount"`
		Sales      []saleInfo `json:"sales"`
	}
	ret := salesResponse{int(total), sales}

	sanitizedJSONResponse(w, ret)
}

func (g *Gateway) handleGETSales(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	saleType := q.Get("type")

	if saleType == "all" || saleType == "guest" {
		g.handleUnifiedSales(w, r, saleType)
		return
	}

	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(q)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)
	profileSvc := getProfileService(r)

	g.getSalesImpl(w, r.Context(), orderSvc, profileSvc, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
}

func (g *Gateway) handleUnifiedSales(w http.ResponseWriter, r *http.Request, saleType string) {
	provider, ok := getNodeService(r).(contracts.UnifiedOrderViewProvider)
	if !ok || provider.UnifiedOrders() == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "unified order view not available")
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if pageSize <= 0 {
		pageSize = 20
	}

	filter := contracts.OrderListFilter{
		View:     saleType,
		State:    q.Get("state"),
		Page:     page,
		PageSize: pageSize,
	}

	orders, meta, err := provider.UnifiedOrders().ListOrders(r.Context(), filter)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}

	responsePkg.List(w, orders, responsePkg.Meta{
		Total:    meta.Total,
		Page:     meta.Page,
		PageSize: meta.PageSize,
	})
}

func (g *Gateway) handlePostSales(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var query TransactionQuery
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	orderSvc := getOrderService(r)
	profileSvc := getProfileService(r)

	g.getSalesImpl(w, r.Context(), orderSvc, profileSvc, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
}

func (g *Gateway) getCasesImpl(w http.ResponseWriter, ctx context.Context, orderSvc contracts.OrderService, profileSvc contracts.ProfileService, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
	cases, total, err := orderSvc.GetCases(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	type caseInfo struct {
		CaseID             string               `json:"caseID"`
		Slug               string               `json:"slug"`
		Timestamp          time.Time            `json:"timestamp"`
		Title              string               `json:"title"`
		Thumbnail          string               `json:"thumbnail"`
		Total              models.CurrencyValue `json:"total"`
		BuyerID            string               `json:"buyerID"`
		BuyerName          string               `json:"buyerName"`
		BuyerAvatar        string               `json:"buyerAvatar"`
		VendorID           string               `json:"vendorID"`
		VendorName         string               `json:"vendorName"`
		VendorAvatar       string               `json:"vendorAvatar"`
		CoinType           string               `json:"coinType"`
		PaymentCoin        string               `json:"paymentCoin"`
		BuyerOpened        bool                 `json:"buyerOpened"`
		State              string               `json:"state"`
		Read               bool                 `json:"read"`
		UnreadChatMessages int                  `json:"unreadChatMessages"`
	}

	casesInfo := []caseInfo{}
	allPeerIDs := make([]string, 0, len(cases)*2)
	for _, aCase := range cases {
		disputeOpen, err := aCase.DisuteOpenMessage()
		if err != nil {
			log.Errorf("Failed to get dispute open info from case: %s", aCase.ID.String())
			continue
		}

		contract, _ := aCase.BuyerContract()
		if contract == nil {
			contract, err = aCase.VendorContract()
			if contract == nil || err != nil {
				log.Errorf("Failed to get contract from case: %s, %v", aCase.ID.String(), err)
				continue
			}
		}

		orderOpen := contract.OrderOpen
		paymentSent := contract.PaymentSent

		var listingInfo *pb.Listing
		if len(orderOpen.Listings) > 0 && orderOpen.Listings[0] != nil {
			listingInfo = orderOpen.Listings[0].Listing
		} else {
			log.Errorf("Failed to get listing info for case: %s", aCase.ID.String())
			continue
		}

		buyerID := orderOpen.BuyerID.PeerID
		vendorID := listingInfo.VendorID.PeerID
		allPeerIDs = append(allPeerIDs, buyerID, vendorID)

		info := caseInfo{
			CaseID:    aCase.ID.String(),
			Slug:      listingInfo.Slug,
			Timestamp: orderOpen.Timestamp.AsTime(),
			Title:     listingInfo.Item.Title,
			Thumbnail: listingInfo.Item.Images[0].Tiny,
			Total: *models.NewCurrencyValue(
				paymentSent.Amount,
				models.CurrencyDefinitions[paymentSent.Coin],
			),
			BuyerID:            buyerID,
			BuyerName:          orderOpen.BuyerID.DisplayName(),
			BuyerAvatar:        orderOpen.BuyerID.DisplayAvatar(),
			VendorID:           vendorID,
			VendorName:         listingInfo.VendorID.DisplayName(),
			VendorAvatar:       listingInfo.VendorID.DisplayAvatar(),
			PaymentCoin:        paymentSent.Coin,
			BuyerOpened:        disputeOpen.OpenedBy == pb.DisputeOpen_BUYER,
			Read:               aCase.Read,
			UnreadChatMessages: aCase.UnreadChatMessages,
			State:              aCase.GetState().String(),
		}

		casesInfo = append(casesInfo, info)
	}

	if profileSvc != nil && len(allPeerIDs) > 0 {
		infoMap := resolveProfileDisplayInfo(ctx, profileSvc, allPeerIDs)
		for i := range casesInfo {
			if info, ok := infoMap[casesInfo[i].BuyerID]; ok {
				casesInfo[i].BuyerName = info.Name
				casesInfo[i].BuyerAvatar = info.Avatar
			}
			if info, ok := infoMap[casesInfo[i].VendorID]; ok {
				casesInfo[i].VendorName = info.Name
				casesInfo[i].VendorAvatar = info.Avatar
			}
		}
	}

	type casesResponse struct {
		QueryCount int        `json:"queryCount"`
		Cases      []caseInfo `json:"cases"`
	}
	ret := casesResponse{int(total), casesInfo}

	sanitizedJSONResponse(w, ret)
}

func (g *Gateway) handleGETCases(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)
	profileSvc := getProfileService(r)

	g.getCasesImpl(w, r.Context(), orderSvc, profileSvc, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
}

func (g *Gateway) handlePostCases(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var query TransactionQuery
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	orderSvc := getOrderService(r)
	profileSvc := getProfileService(r)

	g.getCasesImpl(w, r.Context(), orderSvc, profileSvc, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
}

func (g *Gateway) handleGetCase(w http.ResponseWriter, r *http.Request) {
	_, orderID := path.Split(r.URL.Path)

	orderSvc := getOrderService(r)

	disputeCase, err := orderSvc.GetCase(orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ErrorResponse(w, http.StatusNotFound, "case not found")
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	sanitizedJSONResponse(w, disputeCase)
}

func (g *Gateway) handlePostSpendForOrder(w http.ResponseWriter, r *http.Request) {
	g.handlePOSTSpend(w, r)
}

func (g *Gateway) handleGETOrderCancelInstructions(w http.ResponseWriter, r *http.Request) {
	g.handleOrderInstructions(w, r, func(orderSvc contracts.OrderService, orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error) {
		return orderSvc.GetRefundOrderInstructions(orderID, initiatorAddress)
	})
}

func (g *Gateway) handlePOSTOrderCancel(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type orderCancel struct {
		TransactionID string `json:"transactionID"`
	}
	decoder := json.NewDecoder(r.Body)
	var cancelParam orderCancel
	err := decoder.Decode(&cancelParam)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)

	if cancelParam.TransactionID != "" {
		err = orderSvc.CancelOrder(models.OrderID(orderID), iwallet.TransactionID(cancelParam.TransactionID), nil)
	} else {
		err = orderSvc.CancelOrderViaRelay(models.OrderID(orderID), nil)
	}
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	sanitizedStringResponse(w, `{}`)
}

// handleGETOrderConfirmationInstructions serves the legacy instructions surface
// for client-signed confirm / decline flows. backend-managed EVM callers must use
// /v1/orders/{orderID}/settlement-actions/{action} instead.
func (g *Gateway) handleGETOrderConfirmationInstructions(w http.ResponseWriter, r *http.Request) {
	type Params struct {
		Decline          bool   `json:"decline"`
		InitiatorAddress string `json:"initiatorAddress"`
		PayoutAddress    string `json:"payoutAddress"`
	}
	decoder := json.NewDecoder(r.Body)
	var args Params
	err := decoder.Decode(&args)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	orderSvc := getOrderService(r)

	order, err := orderSvc.GetOrder(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}
	if orderOpen.Listings[0].Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN {
		response := struct {
			HasInstructions bool `json:"hasInstructions"`
		}{
			HasInstructions: false,
		}
		responsePkg.Success(w, response)
		return
	}

	var coinType iwallet.CoinType
	var instructions any
	if args.Decline {
		coinType, instructions, err = orderSvc.GetRefundOrderInstructions(models.OrderID(orderID), args.InitiatorAddress)
	} else {
		coinType, instructions, err = orderSvc.GetConfirmOrderInstructions(models.OrderID(orderID), args.InitiatorAddress, args.PayoutAddress)
	}
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	type ConfirmationResponse struct {
		PaymentChain    iwallet.ChainType `json:"paymentChain"`
		HasInstructions bool              `json:"hasInstructions"`
		Instructions    any               `json:"instructions"`
	}

	if instructions == nil {
		// Legacy endpoint compatibility: UTXO and other backend-handled routes
		// still answer "hasInstructions=false" here. backend-managed EVM does not
		// fall through to this branch because core now rejects that misuse with
		// ErrBadRequest and points callers at settlement-actions.
		response := ConfirmationResponse{
			HasInstructions: false,
		}
		responsePkg.Success(w, response)
		return
	}

	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回响应
	response := ConfirmationResponse{
		PaymentChain:    coinInfo.Chain,
		HasInstructions: true,
		Instructions:    instructions,
	}
	responsePkg.Success(w, response)
}

func (g *Gateway) handlePOSTOrderConfirmation(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type orderConf struct {
		TransactionID string `json:"transactionID"`
		PayoutAddress string `json:"payoutAddress"`
		Decline       bool   `json:"decline"`
		Reason        string `json:"reason"`
	}
	decoder := json.NewDecoder(r.Body)
	var conf orderConf
	err := decoder.Decode(&conf)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)

	if !conf.Decline {
		err = orderSvc.ConfirmOrder(models.OrderID(orderID), iwallet.TransactionID(conf.TransactionID), conf.PayoutAddress, nil)
	} else {
		if conf.TransactionID != "" {
			err = orderSvc.DeclineOrder(models.OrderID(orderID), iwallet.TransactionID(conf.TransactionID), conf.Reason, nil)
		} else {
			err = orderSvc.DeclineOrderViaRelay(models.OrderID(orderID), conf.Reason, nil)
		}
	}
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	sanitizedStringResponse(w, `{}`)
}

func (g *Gateway) handlePOSTOrderShipment(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type orderShipment struct {
		ItemIndex              int                            `json:"itemIndex"`
		Note                   string                         `json:"note"`
		PhysicalDelivery       *models.PhysicalDelivery       `json:"physicalDelivery"`
		DigitalDelivery        *models.DigitalDelivery        `json:"digitalDelivery"`
		CryptocurrencyDelivery *models.CryptocurrencyDelivery `json:"cryptocurrencyDelivery"`
	}
	type orderShipmentRequest struct {
		Shipments          []orderShipment `json:"shipments"`
		ReceivingAccountID *int            `json:"receivingAccountID,omitempty"`
	}

	decoder := json.NewDecoder(r.Body)
	var shipParam orderShipmentRequest
	err := decoder.Decode(&shipParam)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	receivingAccountID := -1
	if shipParam.ReceivingAccountID != nil {
		receivingAccountID = *shipParam.ReceivingAccountID
	}
	if len(shipParam.Shipments) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "shipments is required")
		return
	}

	buildShipment := func(param orderShipment) models.Shipment {
		return models.Shipment{
			ItemIndex:              param.ItemIndex,
			Note:                   param.Note,
			PhysicalDelivery:       param.PhysicalDelivery,
			DigitalDelivery:        param.DigitalDelivery,
			CryptocurrencyDelivery: param.CryptocurrencyDelivery,
		}
	}

	shipments := make([]models.Shipment, 0, len(shipParam.Shipments))
	for _, param := range shipParam.Shipments {
		shipments = append(shipments, buildShipment(param))
	}

	if receivingAccountID >= 0 {
		receivingAccount, err := getReceivingAccountService(r).GetByID(receivingAccountID)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "收款账户不存在或无效")
			return
		}
		// ReceivingAccountAddress is order-level payout input for escrow
		// release. The core order service currently reads shipments[0] when
		// building ReleaseInfo, so do not imply per-shipment split payout here.
		shipments[0].ReceivingAccountAddress = receivingAccount.Address
	}

	orderSvc := getOrderService(r)

	err = orderSvc.ShipOrder(models.OrderID(orderID), shipments, nil)
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	sanitizedStringResponse(w, `{}`)
}

func (g *Gateway) handleGETOrderRefundInstructions(w http.ResponseWriter, r *http.Request) {
	g.handleOrderInstructions(w, r, func(orderSvc contracts.OrderService, orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error) {
		return orderSvc.GetRefundOrderInstructions(orderID, initiatorAddress)
	})
}

func (g *Gateway) handlePOSTOrderRefund(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type orderRefund struct {
		TransactionID string `json:"transactionID"`
	}
	decoder := json.NewDecoder(r.Body)
	var refundParam orderRefund
	err := decoder.Decode(&refundParam)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)

	if refundParam.TransactionID != "" {
		err = orderSvc.RefundOrder(models.OrderID(orderID), iwallet.TransactionID(refundParam.TransactionID), nil)
	} else {
		err = orderSvc.RefundOrderViaRelay(models.OrderID(orderID), nil)
	}
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	sanitizedStringResponse(w, `{}`)
}

func (g *Gateway) handleGETOrderCompleteInstructions(w http.ResponseWriter, r *http.Request) {
	// Legacy instructions surface for client-signed moderated completion.
	// backend-managed moderated completion stays on the backend-owned completion
	// path and therefore does not use this endpoint as its primary contract.
	g.handleOrderInstructions(w, r, func(orderSvc contracts.OrderService, orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error) {
		return orderSvc.GetCompleteOrderInstructions(orderID, initiatorAddress)
	})
}

// handleOrderInstructions 是取消/退款/完成三类订单指令获取接口的通用实现
func (g *Gateway) handleOrderInstructions(
	w http.ResponseWriter,
	r *http.Request,
	getInstructions func(contracts.OrderService, models.OrderID, string) (iwallet.CoinType, any, error),
) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type Params struct {
		InitiatorAddress string `json:"initiatorAddress"`
	}

	decoder := json.NewDecoder(r.Body)
	var args Params
	if err := decoder.Decode(&args); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)
	coinType, instructions, err := getInstructions(orderSvc, models.OrderID(orderID), args.InitiatorAddress)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	type GenericResponse struct {
		PaymentChain    iwallet.ChainType `json:"paymentChain"`
		HasInstructions bool              `json:"hasInstructions"`
		Instructions    any               `json:"instructions"`
	}

	if instructions == nil {
		responsePkg.Success(w, GenericResponse{HasInstructions: false})
		return
	}

	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	responsePkg.Success(w, GenericResponse{
		PaymentChain:    coinInfo.Chain,
		HasInstructions: true,
		Instructions:    instructions,
	})
}

func (g *Gateway) handlePOSTOrderCompletion(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type orderCompletion struct {
		TxID      string          `json:"txID"`
		Ratings   []models.Rating `json:"ratings"`
		Anonymous bool            `json:"anonymous"`
	}

	decoder := json.NewDecoder(r.Body)
	var completeParam orderCompletion
	err := decoder.Decode(&completeParam)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)

	err = orderSvc.CompleteOrder(models.OrderID(orderID), iwallet.TransactionID(completeParam.TxID), completeParam.Ratings, !completeParam.Anonymous, nil)
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	sanitizedStringResponse(w, `{}`)
}

func (g *Gateway) handlePOSTOrderRate(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	type rateRequest struct {
		Ratings   []models.Rating `json:"ratings"`
		Anonymous bool            `json:"anonymous"`
	}

	decoder := json.NewDecoder(r.Body)
	var req rateRequest
	if err := decoder.Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.Ratings) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "ratings cannot be empty")
		return
	}

	orderSvc := getOrderService(r)

	err := orderSvc.RateOrder(models.OrderID(orderID), req.Ratings, !req.Anonymous, nil)
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	sanitizedStringResponse(w, `{}`)
}

func (g *Gateway) handlePOSTExtendProtection(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	orderSvc := getOrderService(r)

	info, err := orderSvc.ExtendProtection(models.OrderID(orderID))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			ErrorResponse(w, http.StatusNotFound, "order not found")
		default:
			ErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	sanitizedJSONResponse(w, info)
}

// func RegisterOrderHandlers(r *mux.Router, g *Gateway) {
// 	r.Post("/v1/ordercancel", g.handlePOSTOrderCancel)
// 	r.Post("/v1/orderconfirmation", g.handlePOSTOrderConfirmation)
// 	r.Post("/v1/ordershipment", g.handlePOSTOrderShipment)
// 	r.Get("/v1/order/confirm/instructions", g.handleGetConfirmOrderInstructions)
// 	r.Get("/v1/order/decline/instructions", g.handleGetRefundOrderInstructions)
// }

// PaymentRemainingResponse represents the response for payment remaining endpoint
// PaymentTransactionInfo represents a transaction in the payment history
type PaymentTransactionInfo struct {
	TxID      string `json:"txid"`
	Amount    uint64 `json:"amount,string"`
	Height    uint64 `json:"height"`
	Timestamp string `json:"timestamp,omitempty"`
}

// PaymentRemainingResponse - Frontend generates PaymentURI/QRCode using paymentAddress + remainingAmount + coin
type PaymentRemainingResponse struct {
	OrderID           string                   `json:"orderID"`
	ExpectedAmount    uint64                   `json:"expectedAmount,string"`
	PaidAmount        uint64                   `json:"paidAmount,string"`
	RemainingAmount   uint64                   `json:"remainingAmount,string"`
	Coin              string                   `json:"coin"`
	PaymentAddress    string                   `json:"paymentAddress"`
	Transactions      []PaymentTransactionInfo `json:"transactions,omitempty"`
	HasPartialPayment bool                     `json:"hasPartialPayment"`
}

// handleGETPaymentRemaining returns the remaining payment amount for an order
// GET /v1/order/{orderID}/payment/remaining
func (g *Gateway) handleGETPaymentRemaining(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	orderSvc := getOrderService(r)

	order, err := orderSvc.GetOrder(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	pendingInfo, _ := order.GetPendingPaymentInfo()
	if pendingInfo == nil || pendingInfo.Amount == 0 || order.PaymentAddress == "" {
		ErrorResponse(w, http.StatusBadRequest, "no pending payment for this order")
		return
	}

	paidAmount, err := getWalletService(r).GetTotalPaidToAddress(order)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to calculate paid amount")
		return
	}

	remainingAmount := uint64(0)
	if pendingInfo.Amount > paidAmount {
		remainingAmount = pendingInfo.Amount - paidAmount
	}

	// Get existing transactions
	var txInfos []PaymentTransactionInfo
	txs, err := order.GetTransactions()
	if err == nil && len(txs) > 0 {
		for _, tx := range txs {
			// Calculate amount paid to payment address in this transaction
			var txAmount uint64
			for _, to := range tx.To {
				if payment.SameUTXOAddress(to.Address.String(), order.PaymentAddress) {
					txAmount = to.Amount.Uint64()
					break
				}
			}
			if txAmount > 0 {
				txInfos = append(txInfos, PaymentTransactionInfo{
					TxID:      tx.ID.String(),
					Amount:    txAmount,
					Height:    tx.Height,
					Timestamp: tx.Timestamp.Format("2006-01-02 15:04:05"),
				})
			}
		}
	}

	response := PaymentRemainingResponse{
		OrderID:           orderID,
		ExpectedAmount:    pendingInfo.Amount,
		PaidAmount:        paidAmount,
		RemainingAmount:   remainingAmount,
		Coin:              pendingInfo.Coin,
		PaymentAddress:    order.PaymentAddress,
		Transactions:      txInfos,
		HasPartialPayment: paidAmount > 0 && remainingAmount > 0,
	}

	responsePkg.Success(w, response)
}

// CancelPartialPaymentRequest represents the request for canceling partial payment
type CancelPartialPaymentRequest struct {
	OrderID string `json:"orderID"`
}

// CancelPartialPaymentResponse represents the response for cancel partial payment
type CancelPartialPaymentResponse struct {
	Success        bool   `json:"success"`
	TransactionID  string `json:"transactionID,omitempty"`
	RefundedAmount uint64 `json:"refundedAmount,omitempty"`
	Message        string `json:"message,omitempty"`
}

// handlePOSTCancelPartialPayment cancels partial payment and refunds to buyer
// POST /v1/order/{orderID}/payment/cancel-partial
func (g *Gateway) handlePOSTCancelPartialPayment(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	orderSvc := getOrderService(r)

	order, err := orderSvc.GetOrder(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "order not found")
		return
	}

	if order.PaymentAddress == "" {
		ErrorResponse(w, http.StatusBadRequest, "no payment address for this order")
		return
	}

	if _, err := order.PaymentSentMessage(); err == nil {
		ErrorResponse(w, http.StatusBadRequest, "payment already sent, cannot cancel partial payment")
		return
	}

	txid, refundedAmount, err := getWalletService(r).CancelPartialPayment(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := CancelPartialPaymentResponse{
		Success:        true,
		TransactionID:  txid,
		RefundedAmount: refundedAmount,
	}

	responsePkg.Success(w, response)
}

// handleDELETEPaymentWatch stops watching a payment address for an order
// DELETE /v1/order/{orderID}/payment/watch
// Called when buyer closes payment UI
func (g *Gateway) handleDELETEPaymentWatch(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	wallet := getWalletService(r)

	if err := wallet.StopWatchingPayment(orderID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	responsePkg.Success(w, map[string]bool{"success": true})
}
