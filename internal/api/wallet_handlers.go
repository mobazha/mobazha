package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/internal/core/coreiface"
	"github.com/mobazha/mobazha3.0/internal/models"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
	"github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type walletBalanceResponse struct {
	Confirmed   string           `json:"confirmed"`
	Unconfirmed string           `json:"unconfirmed"`
	Currency    *models.Currency `json:"currency"`
	Height      uint64           `json:"height"`
}

func (g *Gateway) handleGETBalance(w http.ResponseWriter, r *http.Request) {
	coinType := mux.Vars(r)["coinType"]

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	if coinType == "" {
		ret := make(map[string]interface{})

		for ct, wallet := range node.Multiwallet() {
			unconfirmed, confirmed, err := wallet.Balance()
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}

			info, err := wallet.BlockchainInfo()
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}

			def, err := models.CurrencyDefinitions.Lookup(ct.CurrencyCode())
			if err != nil {
				continue
			}

			ret[ct.CurrencyCode()] = walletBalanceResponse{
				Confirmed:   confirmed.String(),
				Unconfirmed: unconfirmed.String(),
				Currency:    def,
				Height:      info.Height,
			}
		}

		sanitizedJSONResponse(w, ret)
		return
	}

	mw := node.Multiwallet()
	wallet, err := mw.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	unconfirmed, confirmed, err := wallet.Balance()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	info, err := wallet.BlockchainInfo()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	def, _ := models.CurrencyDefinitions.Lookup(coinType)

	ret := walletBalanceResponse{
		Confirmed:   confirmed.String(),
		Unconfirmed: unconfirmed.String(),
		Currency:    def,
		Height:      info.Height,
	}

	sanitizedJSONResponse(w, ret)
}

type walletAddressResponse struct {
	Address string `json:"address"`
}

func (g *Gateway) handleGETAddress(w http.ResponseWriter, r *http.Request) {
	coinType := mux.Vars(r)["coinType"]

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	if coinType == "" {
		ret := make(map[string]string)

		for ct, wallet := range node.Multiwallet() {
			address, err := wallet.CurrentAddress()
			if err != nil {
				ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}

			ret[ct.CurrencyCode()] = address.String()
		}

		sanitizedJSONResponse(w, ret)
		return
	}

	mw := node.Multiwallet()
	wallet, err := mw.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	address, err := wallet.CurrentAddress()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	ret := walletAddressResponse{
		Address: address.String(),
	}

	sanitizedJSONResponse(w, ret)
}

type walletTransactionResponse struct {
	Txid          string    `json:"txid"`
	Value         string    `json:"value"`
	Address       string    `json:"address"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	Confirmations uint64    `json:"confirmations"`
	Height        uint64    `json:"height"`
	Memo          string    `json:"memo"`
	OrderID       string    `json:"orderID"`
	Thumbnail     string    `json:"thumbnail"`
}

func (g *Gateway) handleGETTransactions(w http.ResponseWriter, r *http.Request) {
	var (
		coinType = mux.Vars(r)["coinType"]
		limitStr = r.URL.Query().Get("limit")
		offsetID = r.URL.Query().Get("offsetID")
		limit    = 1000
		err      error
	)
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	mw := node.Multiwallet()
	wallet, err := mw.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	def, err := models.CurrencyDefinitions.Lookup(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	chainInfo, err := wallet.BlockchainInfo()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	txs, err := wallet.Transactions(limit, iwallet.TransactionID(offsetID))
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	confirmedThreshold := uint64(time.Hour / def.BlockInterval)
	ret := make([]walletTransactionResponse, 0, len(txs))
	for _, tx := range txs {
		var (
			confirmations = uint64(0)
			status        string
		)
		if tx.Height > 0 {
			confirmations = (chainInfo.Height - tx.Height) + 1
		}
		if confirmations == 0 {
			status = "UNCONFIRMED"
		} else if confirmations < confirmedThreshold {
			status = "PENDING"
		} else if confirmations >= confirmedThreshold {
			status = "CONFIRMED"
		}
		metadata, err := node.GetTransactionMetadata(tx.ID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		ret = append(ret, walletTransactionResponse{
			Txid:          tx.ID.String(),
			Value:         tx.Value.String(),
			Height:        tx.Height,
			Timestamp:     tx.Timestamp,
			Confirmations: confirmations,
			Status:        status,
			Memo:          metadata.Memo,
			Thumbnail:     metadata.Thumbnail,
			OrderID:       metadata.OrderID.String(),
			Address:       metadata.PaymentAddress,
		})
	}

	type txWithCount struct {
		Transactions []walletTransactionResponse `json:"transactions"`
		Count        int                         `json:"count"`
	}
	txns := txWithCount{ret, len(ret)}

	sanitizedJSONResponse(w, txns)
}

func (g *Gateway) handleGETCurrencies(w http.ResponseWriter, r *http.Request) {
	sanitizedJSONResponse(w, models.CurrencyDefinitions)
}

func (g *Gateway) handleGETMnemonic(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	ret, err := node.GetMnemonic()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, struct {
		Mnemonic string `json:"mnemonic"`
	}{
		Mnemonic: ret,
	})
}

func (g *Gateway) handleGETEstimateFee(w http.ResponseWriter, r *http.Request) {
	var (
		coinType = mux.Vars(r)["coinType"]
		fl       = r.URL.Query().Get("feeLevel")
		amt      = r.URL.Query().Get("amount")
		err      error
	)

	amount, ok := new(big.Int).SetString(amt, 10) //strconv.Atoi(amt)
	if !ok {
		ErrorResponse(w, http.StatusBadRequest, "invalid amount")
		return
	}

	var feeLevel iwallet.FeeLevel
	switch strings.ToUpper(fl) {
	case "PRIORITY":
		feeLevel = iwallet.FlPriority
	case "NORMAL":
		feeLevel = iwallet.FlNormal
	case "ECONOMIC":
		feeLevel = iwallet.FlEconomic
	case "SUPER_ECONOMIC":
		feeLevel = iwallet.FLSuperEconomic
	default:
		customFee, err := strconv.Atoi(fl)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid custom fee")
			return
		}
		feeLevel = iwallet.FeeLevel(customFee)
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	mw := node.Multiwallet()
	wallet, err := mw.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	fee, err := wallet.EstimateSpendFee(iwallet.NewAmount(amount), feeLevel, iwallet.Address{}, iwallet.Amount{})
	if err != nil {
		switch {
		case err == iwallet.ErrInsufficientFunds:
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		default:
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	def, _ := models.CurrencyDefinitions.Lookup(coinType)

	sanitizedJSONResponse(w, struct {
		Currency *models.Currency `json:"currency"`
		Amount   iwallet.Amount   `json:"amount"`
	}{
		Currency: def,
		Amount:   fee,
	})
}

func (g *Gateway) handlePOSTSpend(w http.ResponseWriter, r *http.Request) {
	type Spend struct {
		CoinType string `json:"coinType"`
		Address  string `json:"address"`
		Amount   string `json:"amount"`
		FeeLevel string `json:"feeLevel"`
		Memo     string `json:"memo"`
		OrderID  string `json:"orderID"`
	}

	var spendData Spend
	if err := json.NewDecoder(r.Body).Decode(&spendData); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	var (
		thumbnail string
		memo      = spendData.Memo
	)

	var orderOpen *mbzpb.OrderOpen
	if spendData.OrderID != "" {
		order, err := node.GetOrder(spendData.OrderID)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, coreiface.ErrNotFound.Error())
		}

		orderOpen, err = order.OrderOpenMessage()
		if err == nil {
			thumbnail = orderOpen.Listings[0].Listing.Item.Images[0].Tiny
			if memo == "" {
				memo = orderOpen.Listings[0].Listing.Item.Title
			}
		}
	}

	mw := node.Multiwallet()
	wallet, err := mw.WalletForCurrencyCode(spendData.CoinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	addr := iwallet.NewAddress(spendData.Address, iwallet.CoinType(spendData.CoinType))
	amt := iwallet.NewAmount(spendData.Amount)
	if amt.Cmp(iwallet.NewAmount(0)) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "cannot send zero amount")
		return
	}

	var feeLevel iwallet.FeeLevel
	switch strings.ToUpper(spendData.FeeLevel) {
	case "PRIORITY":
		feeLevel = iwallet.FlPriority
	case "NORMAL":
		feeLevel = iwallet.FlNormal
	case "ECONOMIC":
		feeLevel = iwallet.FlEconomic
	case "SUPER_ECONOMIC":
		feeLevel = iwallet.FLSuperEconomic
	default:
		customFee, err := strconv.Atoi(spendData.FeeLevel)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid custom fee")
			return
		}
		feeLevel = iwallet.FeeLevel(customFee)
	}

	wtx, err := wallet.Begin()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	platformAddr := iwallet.Address{}
	platformAmt := iwallet.Amount{}
	if orderOpen != nil && len(orderOpen.Payment.PlatformAmount) > 0 {
		platformAddr = iwallet.NewAddress(orderOpen.Payment.PlatformAddr, iwallet.CoinType(spendData.CoinType))
		platformAmt = iwallet.NewAmount(orderOpen.Payment.PlatformAmount)
	}

	txid, err := wallet.Spend(wtx, addr, amt, feeLevel, platformAddr, platformAmt)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := wtx.Commit(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	md := models.TransactionMetadata{
		Txid:           txid,
		PaymentAddress: addr.String(),
		Memo:           memo,
		OrderID:        models.OrderID(spendData.OrderID),
		Thumbnail:      thumbnail,
	}

	if err := node.SaveTransactionMetadata(&md); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	unconfirmed, confirmed, err := wallet.Balance()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	def, _ := models.CurrencyDefinitions.Lookup(spendData.CoinType)

	sanitizedJSONResponse(w, struct {
		Amount             string           `json:"amount"`
		ConfirmedBalance   string           `json:"confirmedBalance"`
		UnconfirmedBalance string           `json:"unconfirmedBalance"`
		Currency           *models.Currency `json:"currency"`
		Memo               string           `json:"memo"`
		OrderID            string           `json:"orderID"`
		Timestamp          time.Time        `json:"timestamp"`
		Txid               string           `json:"txid"`
	}{
		Amount:             spendData.Amount,
		ConfirmedBalance:   confirmed.String(),
		UnconfirmedBalance: unconfirmed.String(),
		Currency:           def,
		Memo:               spendData.Memo,
		OrderID:            spendData.OrderID,
		Timestamp:          time.Now(),
		Txid:               txid.String(),
	})
}

// Trick: return fees of 1 coin with divisibility for each coin type
func (g *Gateway) handleGETFees(w http.ResponseWriter, r *http.Request) {
	type fees struct {
		Priority      *models.CurrencyValue `json:"priority"`
		Normal        *models.CurrencyValue `json:"normal"`
		Economic      *models.CurrencyValue `json:"economic"`
		SuperEconomic *models.CurrencyValue `json:"superEconomic"`
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	ret := make(map[string]interface{})
	for ct, wallet := range node.Multiwallet() {
		def, err := models.CurrencyDefinitions.Lookup(ct.CurrencyCode())
		if err != nil {
			continue
		}

		priority, err := wallet.GetFeePerByte(iwallet.FlPriority)
		if err != nil {
			continue
		}
		normal, err := wallet.GetFeePerByte(iwallet.FlNormal)
		if err != nil {
			continue
		}
		economic, err := wallet.GetFeePerByte(iwallet.FlEconomic)
		if err != nil {
			continue
		}
		superEconomic, err := wallet.GetFeePerByte(iwallet.FLSuperEconomic)
		if err != nil {
			continue
		}

		ret[ct.CurrencyCode()] = fees{
			Priority:      &models.CurrencyValue{Currency: def, Amount: priority},
			Normal:        &models.CurrencyValue{Currency: def, Amount: normal},
			Economic:      &models.CurrencyValue{Currency: def, Amount: economic},
			SuperEconomic: &models.CurrencyValue{Currency: def, Amount: superEconomic},
		}
	}
	sanitizedJSONResponse(w, ret)
}

func (g *Gateway) handlePOSTResyncOrders(w http.ResponseWriter, r *http.Request) {
	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	mw := node.Multiwallet()

	for _, coinType := range []string{"BTC", "BCH", "LTC", "BNB", "USDT", "USDC", "MBZ"} {
		wallet, err := mw.WalletForCurrencyCode(coinType)
		if err != nil {
			log.Errorf("Error get wallet for coin: %s", coinType)
			continue
		}

		scanner, _ := wallet.(iwallet.WalletScanner)
		startTime := time.Date(2023, time.Month(1), 0, 0, 0, 0, 0, time.UTC)
		if err := scanner.RescanTransactions(startTime, nil); err != nil {
			log.Errorf("Error starting rescan job: %s", err)
		}
	}

	node.CheckOrdersForMorePayments()

	sanitizedStringResponse(w, `{}`)
}

func (g *Gateway) handlePOSTResyncBlockchain(w http.ResponseWriter, r *http.Request) {
	coinType := mux.Vars(r)["coinType"]

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	mw := node.Multiwallet()
	wallet, err := mw.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	scanner, ok := wallet.(iwallet.WalletScanner)
	if !ok {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Coin sync for %s is not supported", coinType))
	}

	done := make(chan struct{})
	startTime := time.Date(2023, time.Month(1), 0, 0, 0, 0, 0, time.UTC)
	err = scanner.RescanTransactions(startTime, done)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	select {
	case <-done:
	sanitizedStringResponse(w, `{}`)
		return
	case <-time.After(time.Second * 60):
		ErrorResponse(w, http.StatusInternalServerError, "timeout waiting on channel")
		return
	}
}

func (g *Gateway) handleGETWalletStatus(w http.ResponseWriter, r *http.Request) {
	type status struct {
		Height   uint32 `json:"height"`
		BestHash string `json:"bestHash"`
	}

	coinType := mux.Vars(r)["coinType"]

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	mw := node.Multiwallet()

	if coinType == "" {
		ret := make(map[string]interface{})
		for ct, wallet := range mw {
			blockInfo, err := wallet.BlockchainInfo()
			if err == nil {
				ret[ct.CurrencyCode()] = status{uint32(blockInfo.Height), blockInfo.BlockID.String()}
			}
		}

		sanitizedJSONResponse(w, ret)
		return
	}

	wallet, err := mw.WalletForCurrencyCode(coinType)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	blockInfo, err := wallet.BlockchainInfo()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
	}
	st := status{uint32(blockInfo.Height), blockInfo.BlockID.String()}

	sanitizedJSONResponse(w, st)
}

func (g *Gateway) handleUpdateWalletStatus(w http.ResponseWriter, r *http.Request) {
	coinType := mux.Vars(r)["coinType"]

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	var coins []iwallet.CoinType
	if coinType == "" {
		mw := node.Multiwallet()
		for ct := range mw {
			coins = append(coins, ct)
		}
	} else {
		coins = append(coins, iwallet.CoinType(coinType))
	}

	node.UpdateWalletStatus(coins)
}

// GetReceivingAccounts 获取用户的收款账户信息
func (g *Gateway) GetReceivingAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "只允许GET请求")
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 从数据库获取用户的收款账户信息
	receivingAccounts, err := node.GetReceivingAccounts()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回收款账户信息
	resp := struct {
		ReceivingAccounts []models.ReceivingAccount `json:"receivingAccounts"`
	}{
		ReceivingAccounts: receivingAccounts,
	}

	sanitizedJSONResponse(w, resp)
}

// UpdateReceivingAccounts 更新用户的收款账户信息
func (g *Gateway) UpdateReceivingAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		ErrorResponse(w, http.StatusMethodNotAllowed, "只允许PUT请求")
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 解析请求体
	var req struct {
		ReceivingAccounts []models.ReceivingAccount `json:"receivingAccounts"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// 更新收款账户信息
	err = node.UpdateReceivingAccounts(req.ReceivingAccounts)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回成功响应
	resp := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	sanitizedJSONResponse(w, resp)
}

// GetStripeConnectURL 获取Stripe OAuth连接URL
func (g *Gateway) GetStripeConnectURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "只允许GET请求")
		return
	}

	node := r.Context().Value(nodeContextKey).(coreiface.CoreIface)

	// 获取Stripe连接URL
	// 这里需要使用Stripe API生成OAuth URL
	// 实际实现中需要使用Stripe SDK
	stripeConnectURL, err := node.GetStripeConnectURL()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回URL
	resp := struct {
		URL string `json:"url"`
	}{
		URL: stripeConnectURL,
	}

	sanitizedJSONResponse(w, resp)
}
