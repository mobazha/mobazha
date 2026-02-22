package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"gorm.io/gorm"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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

// orderActionErrorResponse maps order action errors to appropriate HTTP status codes.
// Relay-specific errors are mapped to 503/501 so clients know to fall back to the
// instructions + frontend wallet flow.
func orderActionErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, coreiface.ErrRelayNotAvailable):
		ErrorResponse(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, coreiface.ErrRelayChainNotSupported):
		ErrorResponse(w, http.StatusNotImplemented, err.Error())
	case errors.Is(err, coreiface.ErrBadRequest):
		ErrorResponse(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, coreiface.ErrNotFound):
		ErrorResponse(w, http.StatusNotFound, err.Error())
	default:
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
	}
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
		ErrorResponse(w, http.StatusInternalServerError, wrapError(err))
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

	unreadChatMsgCount, _ := getChatService(r).GetChatMessagesUnreadCountByOrderID(order.ID)

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
		State:              order.State.String(),
		UnreadChatMessages: unreadChatMsgCount,
		Funded:             isFunded,
		Completable:        order.CanComplete(),
	}

	sanitizedJSONResponse(w, ret)
}

// handlePOSTPayment 处理支付结果通知
func (g *Gateway) handlePOSTPayment(w http.ResponseWriter, r *http.Request) {
	orderSvc := getOrderService(r)

	var req struct {
		PaymentData *models.PaymentData `json:"paymentData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "decode request body failed: "+err.Error(), http.StatusBadRequest)
		return
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

func (g *Gateway) getPurchasesImpl(w http.ResponseWriter, orderSvc contracts.OrderService, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
	orders, total, err := orderSvc.GetPurchases(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
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
		paymentCoin := ""
		isModerated := false
		if err == nil {
			isModerated = paymentSent.Method == pb.PaymentSent_MODERATED
			paymentCoin = paymentSent.Coin
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
				orderOpen.Amount,
				models.CurrencyDefinitions[orderOpen.PricingCoin],
			),
			VendorID:           listingInfo.VendorID.PeerID,
			VendorHandle:       listingInfo.VendorID.Handle,
			ShippingName:       orderOpen.Shipping.ShipTo,
			ShippingAddress:    orderOpen.Shipping.Address,
			PaymentCoin:        paymentCoin,
			State:              order.State.String(),
			Read:               order.Read,
			UnreadChatMessages: order.UnreadChatMessages,
			Moderated:          isModerated,
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

	orderSvc := getOrderService(r)

	g.getPurchasesImpl(w, orderSvc, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
}

func (g *Gateway) handleGETPurchases(w http.ResponseWriter, r *http.Request) {
	orderStates, searchTerm, sortByAscending, sortByRead, limit, err := parseSearchTerms(r.URL.Query())
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)

	g.getPurchasesImpl(w, orderSvc, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
}

func (g *Gateway) getSalesImpl(w http.ResponseWriter, orderSvc contracts.OrderService, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
	orders, total, err := orderSvc.GetSales(stateFilters, searchTerm, sortByAscending, sortByRead, limit, exclude)
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
		isModerated := false
		paymentCoin := ""
		if err == nil {
			isModerated = paymentSent.Method == pb.PaymentSent_MODERATED
			paymentCoin = paymentSent.Coin
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
				orderOpen.Amount,
				models.CurrencyDefinitions[orderOpen.PricingCoin],
			),
			BuyerID:            orderOpen.BuyerID.PeerID,
			BuyerHandle:        orderOpen.BuyerID.Handle,
			ShippingName:       orderOpen.Shipping.ShipTo,
			ShippingAddress:    orderOpen.Shipping.Address,
			PaymentCoin:        paymentCoin,
			State:              order.State.String(),
			Read:               order.Read,
			UnreadChatMessages: order.UnreadChatMessages,
			Moderated:          isModerated,
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

	orderSvc := getOrderService(r)

	g.getSalesImpl(w, orderSvc, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
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

	g.getSalesImpl(w, orderSvc, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
}

func (g *Gateway) getCasesImpl(w http.ResponseWriter, orderSvc contracts.OrderService, stateFilters []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) {
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

	orderSvc := getOrderService(r)

	g.getCasesImpl(w, orderSvc, orderStates, searchTerm, sortByAscending, sortByRead, limit, nil)
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

	g.getCasesImpl(w, orderSvc, convertOrderStates(query.OrderStates), query.SearchTerm, query.SortByAscending, query.SortByRead, query.Limit, query.Exclude)
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
	type orderCancel struct {
		OrderID       string `json:"orderID"`
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

	done := make(chan struct{})

	if cancelParam.TransactionID != "" {
		err = orderSvc.CancelOrder(models.OrderID(cancelParam.OrderID), iwallet.TransactionID(cancelParam.TransactionID), done)
	} else {
		err = orderSvc.CancelOrderViaRelay(models.OrderID(cancelParam.OrderID), done)
	}
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 15):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handleGETOrderConfirmationInstructions(w http.ResponseWriter, r *http.Request) {
	type Params struct {
		OrderID          string `json:"orderID"`
		Reject           bool   `json:"reject"`
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

	orderSvc := getOrderService(r)

	order, err := orderSvc.GetOrder(args.OrderID)
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
		json.NewEncoder(w).Encode(response)
		return
	}

	var coinType iwallet.CoinType
	var instructions any
	if args.Reject {
		coinType, instructions, err = orderSvc.GetRefundOrderInstructions(models.OrderID(args.OrderID), args.InitiatorAddress)
	} else {
		coinType, instructions, err = orderSvc.GetConfirmOrderInstructions(models.OrderID(args.OrderID), args.InitiatorAddress, args.PayoutAddress)
	}
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	type ConfirmationResponse struct {
		PaymentChain    iwallet.ChainType `json:"paymentChain"`
		HasInstructions bool              `json:"hasInstructions"`
		Instructions    any               `json:"instructions"`
	}

	if instructions == nil {
		response := ConfirmationResponse{
			HasInstructions: false,
		}
		json.NewEncoder(w).Encode(response)
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
	json.NewEncoder(w).Encode(response)
}

func (g *Gateway) handlePOSTOrderConfirmation(w http.ResponseWriter, r *http.Request) {
	type orderConf struct {
		OrderID       string `json:"orderID"`
		TransactionID string `json:"transactionID"`
		PayoutAddress string `json:"payoutAddress"`
		Reject        bool   `json:"reject"`
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

	done := make(chan struct{})
	if !conf.Reject {
		err = orderSvc.ConfirmOrder(models.OrderID(conf.OrderID), iwallet.TransactionID(conf.TransactionID), conf.PayoutAddress, done)
	} else {
		if conf.TransactionID != "" {
			err = orderSvc.RejectOrder(models.OrderID(conf.OrderID), iwallet.TransactionID(conf.TransactionID), conf.Reason, done)
		} else {
			err = orderSvc.RejectOrderViaRelay(models.OrderID(conf.OrderID), conf.Reason, done)
		}
	}
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 15):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handlePOSTOrderFulfillment(w http.ResponseWriter, r *http.Request) {
	type orderFulfillment struct {
		OrderID                string                         `json:"orderID"`
		ItemIndex              int                            `json:"itemIndex"`
		Note                   string                         `json:"note"`
		PhysicalDelivery       *models.PhysicalDelivery       `json:"physicalDelivery"`
		DigitalDelivery        *models.DigitalDelivery        `json:"digitalDelivery"`
		CryptocurrencyDelivery *models.CryptocurrencyDelivery `json:"cryptocurrencyDelivery"`
		ReceivingAccountID     *int                           `json:"receivingAccountID,omitempty"`
	}

	decoder := json.NewDecoder(r.Body)
	var fulfillParam orderFulfillment
	err := decoder.Decode(&fulfillParam)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// 如果 JSON 反序列化时未提供 ReceivingAccountID，使用默认值 -1
	receivingAccountID := -1
	if fulfillParam.ReceivingAccountID != nil {
		receivingAccountID = *fulfillParam.ReceivingAccountID
	}

	fulFillment := models.Fulfillment{
		ItemIndex:              0,
		Note:                   fulfillParam.Note,
		PhysicalDelivery:       fulfillParam.PhysicalDelivery,
		DigitalDelivery:        fulfillParam.DigitalDelivery,
		CryptocurrencyDelivery: fulfillParam.CryptocurrencyDelivery,
	}

	orderSvc := getOrderService(r)

	if receivingAccountID >= 0 {
		receivingAccount, err := getWalletService(r).GetReceivingAccountByID(receivingAccountID)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "收款账户不存在或无效")
			return
		}
		fulFillment.ReceivingAccountAddress = receivingAccount.Address
	}

	done := make(chan struct{})
	err = orderSvc.FulfillOrder(models.OrderID(fulfillParam.OrderID), []models.Fulfillment{fulFillment}, done)
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

func (g *Gateway) handleGETOrderRefundInstructions(w http.ResponseWriter, r *http.Request) {
	g.handleOrderInstructions(w, r, func(orderSvc contracts.OrderService, orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error) {
		return orderSvc.GetRefundOrderInstructions(orderID, initiatorAddress)
	})
}

func (g *Gateway) handlePOSTOrderRefund(w http.ResponseWriter, r *http.Request) {
	type orderRefund struct {
		OrderID       string `json:"orderID"`
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

	done := make(chan struct{})

	if refundParam.TransactionID != "" {
		err = orderSvc.RefundOrder(models.OrderID(refundParam.OrderID), iwallet.TransactionID(refundParam.TransactionID), done)
	} else {
		err = orderSvc.RefundOrderViaRelay(models.OrderID(refundParam.OrderID), done)
	}
	if err != nil {
		orderActionErrorResponse(w, err)
		return
	}

	select {
	case <-done:
		sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 15):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handleGETOrderCompleteInstructions(w http.ResponseWriter, r *http.Request) {
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
	type Params struct {
		OrderID          string `json:"orderID"`
		InitiatorAddress string `json:"initiatorAddress"`
	}

	decoder := json.NewDecoder(r.Body)
	var args Params
	if err := decoder.Decode(&args); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orderSvc := getOrderService(r)
	coinType, instructions, err := getInstructions(orderSvc, models.OrderID(args.OrderID), args.InitiatorAddress)
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
		json.NewEncoder(w).Encode(GenericResponse{HasInstructions: false})
		return
	}

	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	json.NewEncoder(w).Encode(GenericResponse{
		PaymentChain:    coinInfo.Chain,
		HasInstructions: true,
		Instructions:    instructions,
	})
}

func (g *Gateway) handlePOSTOrderCompletion(w http.ResponseWriter, r *http.Request) {
	type orderCompletion struct {
		OrderID   string          `json:"orderID"`
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

	done := make(chan struct{})
	err = orderSvc.CompleteOrder(models.OrderID(completeParam.OrderID), iwallet.TransactionID(completeParam.TxID), completeParam.Ratings, !completeParam.Anonymous, done)
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

// func RegisterOrderHandlers(r *mux.Router, g *Gateway) {
// 	r.HandleFunc("/v1/ordercancel", g.handlePOSTOrderCancel).Methods("POST")
// 	r.HandleFunc("/v1/orderconfirmation", g.handlePOSTOrderConfirmation).Methods("POST")
// 	r.HandleFunc("/v1/orderfulfillment", g.handlePOSTOrderFulfillment).Methods("POST")
// 	r.HandleFunc("/v1/order/confirm/instructions", g.handleGetConfirmOrderInstructions).Methods("GET")
// 	r.HandleFunc("/v1/order/reject/instructions", g.handleGetRefundOrderInstructions).Methods("GET")
// }

// PaymentRemainingResponse represents the response for payment remaining endpoint
// PaymentTransactionInfo represents a transaction in the payment history
type PaymentTransactionInfo struct {
	TxID      string `json:"txid"`
	Amount    uint64 `json:"amount"`
	Height    uint64 `json:"height"`
	Timestamp string `json:"timestamp,omitempty"`
}

// PaymentRemainingResponse - Frontend generates PaymentURI/QRCode using paymentAddress + remainingAmount + coin
type PaymentRemainingResponse struct {
	OrderID           string                   `json:"orderID"`
	ExpectedAmount    uint64                   `json:"expectedAmount"`
	PaidAmount        uint64                   `json:"paidAmount"`
	RemainingAmount   uint64                   `json:"remainingAmount"`
	Coin              string                   `json:"coin"`
	PaymentAddress    string                   `json:"paymentAddress"`
	Transactions      []PaymentTransactionInfo `json:"transactions,omitempty"`
	HasPartialPayment bool                     `json:"hasPartialPayment"`
}

// handleGETPaymentRemaining returns the remaining payment amount for an order
// GET /v1/order/{orderID}/payment/remaining
func (g *Gateway) handleGETPaymentRemaining(w http.ResponseWriter, r *http.Request) {
	orderID := mux.Vars(r)["orderID"]
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
				if to.Address.String() == order.PaymentAddress {
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
	orderID := mux.Vars(r)["orderID"]
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDELETEPaymentWatch stops watching a payment address for an order
// DELETE /v1/order/{orderID}/payment/watch
// Called when buyer closes payment UI
func (g *Gateway) handleDELETEPaymentWatch(w http.ResponseWriter, r *http.Request) {
	orderID := mux.Vars(r)["orderID"]
	if orderID == "" {
		ErrorResponse(w, http.StatusBadRequest, "missing orderID")
		return
	}

	wallet := getWalletService(r)

	if err := wallet.StopWatchingPayment(orderID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
