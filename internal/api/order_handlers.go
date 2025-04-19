package api

import (
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

type APIError struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
}

func ErrorResponse(w http.ResponseWriter, errorCode int, reason string) {
	reason = strings.Replace(reason, `"`, `'`, -1)
	apiErr := APIError{false, reason}

	log.Errorf("ErrorResponse, errorCode: %d, reason: %s ", errorCode, reason)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(errorCode)
	sanitizedJSONResponse(w, apiErr)
}

func (g *Gateway) handlePOSTPurchase(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var data models.Purchase
	err := decoder.Decode(&data)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	orderID, amount, err := node.PurchaseListing(r.Context(), &data)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}
	type purchaseReturn struct {
		Amount  *models.CurrencyValue `json:"amount"`
		OrderID string                `json:"orderID"`
	}
	ret := purchaseReturn{&amount, orderID.String()}

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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	orderTotals, err := node.EstimateOrderTotal(r.Context(), &data)
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	orderTotals, err := node.EstimateOrderTotal(r.Context(), &data)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedStringResponse(w, orderTotals.Total.String())
}

func (g *Gateway) handleGETOrder(w http.ResponseWriter, r *http.Request) {
	_, orderID := path.Split(r.URL.Path)

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	order, err := node.GetOrder(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	unreadChatMsgCount, _ := node.GetChatMessagesUnreadCountByOrderID(order.ID)

	type OrderRespApi struct {
		Contract           *models.Order `json:"contract,omitempty"`
		State              string        `json:"state,omitempty"`
		Read               bool          `json:"read,omitempty"`
		UnreadChatMessages int64         `json:"unreadChatMessages,omitempty"`
		Funded             bool          `json:"funded,omitempty"`
		Completable        bool          `json:"completable,omitempty"`
	}

	isFunded, _ := order.IsFunded()

	ret := OrderRespApi{
		Contract:           order,
		State:              order.GetState().String(),
		UnreadChatMessages: unreadChatMsgCount,
		Funded:             isFunded,
		Completable:        order.CanComplete(),
	}

	sanitizedJSONResponse(w, ret)
}

// handleNotifyPayment 处理支付通知
func (g *Gateway) handleNotifyPayment(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	var req struct {
		OrderID     string `json:"orderId"`
		PaymentInfo struct {
			Method      string `json:"method"`
			Amount      string `json:"amount"`
			Coin        string `json:"coin"`
			FromAddress string `json:"fromAddress"`
			ToAddress   string `json:"toAddress"`
			Script      string `json:"script"`
			Moderator   string `json:"moderator"`
		} `json:"paymentInfo"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 创建 paymentData 消息
	paymentData := &models.PaymentData{
		OrderID:     req.OrderID,
		Method:      req.PaymentInfo.Method,
		Amount:      req.PaymentInfo.Amount,
		Coin:        req.PaymentInfo.Coin,
		FromAddress: req.PaymentInfo.FromAddress,
		ToAddress:   req.PaymentInfo.ToAddress,
		Script:      req.PaymentInfo.Script,
		Moderator:   req.PaymentInfo.Moderator,
		Timestamp:   time.Now(),
	}

	node.ProcessOrderPayment(r.Context(), paymentData)

	w.WriteHeader(http.StatusOK)
}

func (g *Gateway) getPurchasesImpl(w http.ResponseWriter, node coreiface.CoreIface, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
	orders, total, err := node.GetPurchases(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	type purchaseInfo struct {
		OrderID            string               `json:"orderID"`
		Slug               string               `json:"slug"`
		Timestamp          time.Time            `json:"timestamp"`
		Title              string               `json:"title"`
		Thumbnail          string               `json:"thumbnail"`
		Total              models.CurrencyValue `json:"total"`
		VendorID           string               `json:"vendorID"`
		VendorHandle       string               `json:"vendorHandle"`
		ShippingName       string               `json:"shippingName"`
		ShippingAddress    string               `json:"shippingAddress"`
		CoinType           string               `json:"coinType"`
		PaymentCoin        string               `json:"paymentCoin"`
		State              string               `json:"state"`
		Read               bool                 `json:"read"`
		Moderated          bool                 `json:"moderated"`
		UnreadChatMessages int                  `json:"unreadChatMessages"`
	}

	purchases := []purchaseInfo{}
	for _, order := range orders {
		orderOpen, err := order.OrderOpenMessage()
		if err != nil {
			log.Errorf("Failed to get OrderOpenMessage for order: %s", order.ID.String())
			continue
		}

		paymentSent, err := order.PaymentSentMessage()
		if err != nil {
			log.Errorf("Failed to get PaymentSentMessage for order: %s", order.ID.String())
			continue
		}

		var listingInfo *pb.Listing
		if len(orderOpen.Listings) > 0 && orderOpen.Listings[0] != nil {
			listingInfo = orderOpen.Listings[0].Listing
		} else {
			log.Errorf("Failed to get listing info for order: %s", order.ID.String())
			continue
		}

		info := purchaseInfo{
			OrderID:   order.ID.String(),
			Slug:      listingInfo.Slug,
			Timestamp: orderOpen.Timestamp.AsTime(),
			Title:     listingInfo.Item.Title,
			Thumbnail: listingInfo.Item.Images[0].Tiny,
			Total: *models.NewCurrencyValue(
				paymentSent.Amount,
				models.CurrencyDefinitions[paymentSent.Coin],
			),
			VendorID:           listingInfo.VendorID.PeerID,
			VendorHandle:       listingInfo.VendorID.Handle,
			ShippingName:       orderOpen.Shipping.ShipTo,
			ShippingAddress:    orderOpen.Shipping.Address,
			PaymentCoin:        paymentSent.Coin,
			State:              order.GetState().String(),
			Read:               order.Read,
			UnreadChatMessages: order.UnreadChatMessages,
			Moderated:          paymentSent.Method == pb.PaymentSent_MODERATED,
		}

		purchases = append(purchases, info)
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	g.getPurchasesImpl(w, node, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
}

func (g *Gateway) handleGETPurchases(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	g.getPurchasesImpl(w, node, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
}

func (g *Gateway) getSalesImpl(w http.ResponseWriter, node coreiface.CoreIface, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
	orders, total, err := node.GetSales(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	type saleInfo struct {
		OrderID            string               `json:"orderID"`
		Slug               string               `json:"slug"`
		Timestamp          time.Time            `json:"timestamp"`
		Title              string               `json:"title"`
		Thumbnail          string               `json:"thumbnail"`
		Total              models.CurrencyValue `json:"total"`
		BuyerID            string               `json:"buyerID"`
		BuyerHandle        string               `json:"buyerHandle"`
		ShippingName       string               `json:"shippingName"`
		ShippingAddress    string               `json:"shippingAddress"`
		CoinType           string               `json:"coinType"`
		PaymentCoin        string               `json:"paymentCoin"`
		State              string               `json:"state"`
		Read               bool                 `json:"read"`
		Moderated          bool                 `json:"moderated"`
		UnreadChatMessages int                  `json:"unreadChatMessages"`
	}

	sales := []saleInfo{}
	for _, order := range orders {
		orderOpen, err := order.OrderOpenMessage()
		if err != nil {
			log.Errorf("Failed to get OrderOpenMessage for order: %s", order.ID.String())
			continue
		}

		paymentSent, err := order.PaymentSentMessage()
		if err != nil {
			log.Errorf("Failed to get PaymentSentMessage for order: %s", order.ID.String())
			continue
		}

		var listingInfo *pb.Listing
		if len(orderOpen.Listings) > 0 && orderOpen.Listings[0] != nil {
			listingInfo = orderOpen.Listings[0].Listing
		} else {
			log.Errorf("Failed to get listing info for order: %s", order.ID.String())
			continue
		}

		info := saleInfo{
			OrderID:   order.ID.String(),
			Slug:      listingInfo.Slug,
			Timestamp: orderOpen.Timestamp.AsTime(),
			Title:     listingInfo.Item.Title,
			Thumbnail: listingInfo.Item.Images[0].Tiny,
			Total: *models.NewCurrencyValue(
				paymentSent.Amount,
				models.CurrencyDefinitions[paymentSent.Coin],
			),
			BuyerID:            orderOpen.BuyerID.PeerID,
			BuyerHandle:        orderOpen.BuyerID.Handle,
			ShippingName:       orderOpen.Shipping.ShipTo,
			ShippingAddress:    orderOpen.Shipping.Address,
			PaymentCoin:        paymentSent.Coin,
			State:              order.GetState().String(),
			Read:               order.Read,
			UnreadChatMessages: order.UnreadChatMessages,
			Moderated:          paymentSent.Method == pb.PaymentSent_MODERATED,
		}

		sales = append(sales, info)
	}

	type salesResponse struct {
		QueryCount int        `json:"queryCount"`
		Sales      []saleInfo `json:"sales"`
	}
	ret := salesResponse{int(total), sales}

	sanitizedJSONResponse(w, ret)
}

func (g *Gateway) handleGETSales(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	g.getSalesImpl(w, node, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
}

func (g *Gateway) handlePostSales(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var query TransactionQuery
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	g.getSalesImpl(w, node, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
}

func (g *Gateway) getCasesImpl(w http.ResponseWriter, node coreiface.CoreIface, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
	cases, total, err := node.GetCases(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
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
		BuyerHandle        string               `json:"buyerHandle"`
		VendorID           string               `json:"vendorID"`
		VendorHandle       string               `json:"vendorHandle"`
		CoinType           string               `json:"coinType"`
		PaymentCoin        string               `json:"paymentCoin"`
		BuyerOpened        bool                 `json:"buyerOpened"`
		State              string               `json:"state"`
		Read               bool                 `json:"read"`
		UnreadChatMessages int                  `json:"unreadChatMessages"`
	}

	casesInfo := []caseInfo{}
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
			BuyerID:            orderOpen.BuyerID.PeerID,
			BuyerHandle:        orderOpen.BuyerID.Handle,
			VendorID:           listingInfo.VendorID.PeerID,
			VendorHandle:       listingInfo.VendorID.Handle,
			PaymentCoin:        paymentSent.Coin,
			BuyerOpened:        disputeOpen.OpenedBy == pb.DisputeOpen_BUYER,
			Read:               aCase.Read,
			UnreadChatMessages: aCase.UnreadChatMessages,
			State:              aCase.GetState().String(),
		}

		casesInfo = append(casesInfo, info)
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	g.getCasesImpl(w, node, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
}

func (g *Gateway) handlePostCases(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var query TransactionQuery
	err := decoder.Decode(&query)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, wrapError(err))
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	g.getCasesImpl(w, node, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
}

func (g *Gateway) handleGetCase(w http.ResponseWriter, r *http.Request) {
	_, orderID := path.Split(r.URL.Path)

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	disputeCase, err := node.GetCase(orderID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
		return
	}

	sanitizedJSONResponse(w, disputeCase)
}

func (g *Gateway) handlePostSpendForOrder(w http.ResponseWriter, r *http.Request) {
	g.handlePOSTSpend(w, r)
}

func (g *Gateway) handlePOSTOrderCancel(w http.ResponseWriter, r *http.Request) {
	type orderCancel struct {
		OrderID string `json:"orderID"`
	}
	decoder := json.NewDecoder(r.Body)
	var cancel orderCancel
	err := decoder.Decode(&cancel)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	err = node.CancelOrder(models.OrderID(cancel.OrderID), done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handlePOSTOrderConfirmation(w http.ResponseWriter, r *http.Request) {
	type orderConf struct {
		OrderID string `json:"orderID"`
		Reject  bool   `json:"reject"`
	}
	decoder := json.NewDecoder(r.Body)
	var conf orderConf
	err := decoder.Decode(&conf)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	if !conf.Reject {
		err = node.ConfirmOrder(models.OrderID(conf.OrderID), done)
	} else {
		err = node.RejectOrder(models.OrderID(conf.OrderID), "", done)
	}
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handlePOSTOrderFulfillment(w http.ResponseWriter, r *http.Request) {
	type orderFulfillment struct {
		OrderID string `json:"orderID"`
		models.Fulfillment
	}

	decoder := json.NewDecoder(r.Body)
	var fulfillParam orderFulfillment
	err := decoder.Decode(&fulfillParam)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	fulFillment := models.Fulfillment{
		ItemIndex:              0,
		Note:                   fulfillParam.Note,
		PhysicalDelivery:       fulfillParam.PhysicalDelivery,
		DigitalDelivery:        fulfillParam.DigitalDelivery,
		CryptocurrencyDelivery: fulfillParam.CryptocurrencyDelivery,
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	err = node.FulfillOrder(models.OrderID(fulfillParam.OrderID), []models.Fulfillment{fulFillment}, done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handlePOSTOrderRefund(w http.ResponseWriter, r *http.Request) {
	type orderRefund struct {
		OrderID string `json:"orderID"`
	}
	decoder := json.NewDecoder(r.Body)
	var refundParam orderRefund
	err := decoder.Decode(&refundParam)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	err = node.RefundOrder(models.OrderID(refundParam.OrderID), done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handlePOSTOrderCompletion(w http.ResponseWriter, r *http.Request) {
	type orderCompletion struct {
		OrderID   string          `json:"orderID"`
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

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	done := make(chan struct{})
	err = node.CompleteOrder(models.OrderID(completeParam.OrderID), completeParam.Ratings, !completeParam.Anonymous, done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 10):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}
