package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/wallet"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	paypb "github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

// CryptoPaymentSetupService is the canonical crypto setup port used by
// PaymentSessionService. It keeps session-level metadata out of legacy chain DTOs.
type CryptoPaymentSetupService interface {
	GeneratePaymentSetup(ctx context.Context, params paypb.PaymentSetupParams) (*paypb.PaymentSetupResult, error)
}

// StandardOrderSettlementAuthorizationStartRequest is the minimal callback
// value needed to start the new non-actionable authorization ceremony without
// importing the Core node package into the payment package.
type StandardOrderSettlementAuthorizationStartRequest struct {
	OrderID                 string
	PaymentSelectionQuoteID string
	RailID                  string
	AmountAtomic            string
	ModeratorPeerID         string
}

type standardOrderSettlementAuthorizationStarter func(
	context.Context,
	StandardOrderSettlementAuthorizationStartRequest,
) error

// CryptoPaymentFacade wraps the canonical payment setup service to populate
// crypto funding targets (managed EVM, UTXO, monitored flows) behind
// PaymentSessionService.CreateSession.
type CryptoPaymentFacade struct {
	db                   database.Database
	projector            *PaymentSessionProjector
	orderSvc             contracts.OrderService
	setupSvc             CryptoPaymentSetupService
	exchange             contracts.ExchangeRateService
	storePolicy          contracts.StorePolicyService
	sellerPolicyResolver sellerStorePolicyResolver
	settlementStarter    standardOrderSettlementAuthorizationStarter
}

// SetStandardOrderSettlementAuthorizationStarter wires the fail-closed
// authorization cutover for eligible native UTXO standard orders.
func (c *CryptoPaymentFacade) SetStandardOrderSettlementAuthorizationStarter(
	starter func(context.Context, StandardOrderSettlementAuthorizationStartRequest) error,
) {
	if c != nil {
		c.settlementStarter = starter
	}
}

// NewCryptoPaymentFacade constructs CryptoPaymentFacade.
func NewCryptoPaymentFacade(
	db database.Database,
	orderSvc contracts.OrderService,
	setupSvc CryptoPaymentSetupService,
	exchange contracts.ExchangeRateService,
	storePolicy contracts.StorePolicyService,
) *CryptoPaymentFacade {
	return &CryptoPaymentFacade{
		db:          db,
		projector:   NewPaymentSessionProjector(db),
		orderSvc:    orderSvc,
		setupSvc:    setupSvc,
		exchange:    exchange,
		storePolicy: storePolicy,
		sellerPolicyResolver: dbSellerStorePolicyResolver{
			db: db,
		},
	}
}

// CreateSession provisions the crypto funding target on the order and returns
// the unified projection.
func (c *CryptoPaymentFacade) CreateSession(
	ctx context.Context,
	req contracts.CreatePaymentSessionRequest,
) (*paypb.PaymentSession, error) {
	coin := iwallet.CoinType(req.PaymentCoin)
	input, err := c.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("crypto facade: load order %s: %w", req.OrderID, err)
	}
	order := input.order
	if models.BuyerAwaitingPaymentReadiness(order) {
		view, err := c.UpdateCreateSessionRefundAddress(ctx, req)
		if err != nil {
			return nil, err
		}
		// ORDER_OPEN acknowledgement can race this request. If readiness became
		// ready while the refund address was saved, continue provisioning in the
		// same call; returning a ready session without a funding target violates
		// the PaymentSession contract and makes clients stop retrying.
		if view.PaymentReadiness.Status != paypb.PaymentReadinessReadyToPay || view.FundingTarget.Address != "" {
			return view, nil
		}
		input, err = c.projector.fetchProjectInput(req.OrderID)
		if err != nil {
			return nil, fmt.Errorf("crypto facade: re-load ready order %s: %w", req.OrderID, err)
		}
		order = input.order
	}
	orderOpen := input.orderOpen
	if orderOpen == nil {
		return nil, fmt.Errorf("crypto facade: order open unavailable for order %s", req.OrderID)
	}

	refundAddr, err := resolveCreateSessionRefundAddress(coin, req)
	if err != nil {
		return nil, err
	}

	moderator := strings.TrimSpace(req.Moderator)
	storePolicyRevision, err := c.validateStorePolicyModerator(ctx, req.OrderID, orderOpen, moderator)
	if err != nil {
		return nil, err
	}
	if c.settlementStarter != nil && standardOrderNativeUTXORail(coin) {
		if !standardOrderUTXOAuthorizationEligible(coin, orderOpen) {
			return nil, fmt.Errorf("crypto facade: cross-currency UTXO settlement authorization is not implemented")
		}
		setupParams, err := buildPaymentSetupParamsFromOrder(
			order, orderOpen, coin, req.PayerAddress, refundAddr, moderator,
			req.AuthorizedPaymentAmount, c.exchange,
		)
		if err != nil {
			return nil, fmt.Errorf("crypto facade: build settlement authorization params: %w", err)
		}
		if err := c.saveCreateSessionRefundAddress(ctx, coin, req.OrderID, refundAddr); err != nil {
			return nil, err
		}
		startRequest := standardOrderSettlementAuthorizationStartRequest(
			req, strconv.FormatUint(setupParams.Amount, 10), moderator,
		)
		if err := c.settlementStarter(ctx, startRequest); err != nil {
			return nil, fmt.Errorf("crypto facade: start settlement authorization: %w", err)
		}
		updated, err := c.projector.fetchProjectInput(req.OrderID)
		if err != nil {
			return nil, fmt.Errorf("crypto facade: re-load settlement authorization draft: %w", err)
		}
		return c.projector.Project(updated)
	}

	setupParams, err := buildPaymentSetupParamsFromOrder(order, orderOpen, coin,
		req.PayerAddress, refundAddr, moderator, req.AuthorizedPaymentAmount, c.exchange)
	if err != nil {
		return nil, fmt.Errorf("crypto facade: build escrow params: %w", err)
	}
	setupParams.StorePolicyRevision = storePolicyRevision

	if c.setupSvc == nil {
		return nil, ErrProvisioningNotImplemented
	}
	result, err := c.setupSvc.GeneratePaymentSetup(ctx, setupParams)
	if err != nil {
		if result != nil && result.PaymentData != nil && errors.Is(err, coreiface.ErrCoinSwitchRequiresConfirmation) {
			return nil, fmt.Errorf("%w", coreiface.ErrCoinSwitchRequiresConfirmation)
		}
		return nil, fmt.Errorf("crypto facade: generate instructions: %w", err)
	}

	if err := c.saveCreateSessionRefundAddress(ctx, coin, req.OrderID, refundAddr); err != nil {
		return nil, err
	}

	input2, err := c.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("crypto facade: re-load order %s: %w", req.OrderID, err)
	}
	return c.projector.Project(input2)
}

func standardOrderSettlementAuthorizationStartRequest(
	req contracts.CreatePaymentSessionRequest,
	amountAtomic string,
	moderatorPeerID string,
) StandardOrderSettlementAuthorizationStartRequest {
	return StandardOrderSettlementAuthorizationStartRequest{
		OrderID: req.OrderID, PaymentSelectionQuoteID: req.PaymentSelectionQuoteID,
		RailID: req.PaymentCoin, AmountAtomic: strings.TrimSpace(amountAtomic),
		ModeratorPeerID: strings.TrimSpace(moderatorPeerID),
	}
}

func standardOrderUTXOAuthorizationEligible(coin iwallet.CoinType, orderOpen *porderpb.OrderOpen) bool {
	if orderOpen == nil {
		return false
	}
	if !standardOrderNativeUTXORail(coin) {
		return false
	}
	paymentCurrency, err := coin.PricingCurrencyCode()
	return err == nil && strings.EqualFold(strings.TrimSpace(paymentCurrency), strings.TrimSpace(orderOpen.PricingCoin))
}

func standardOrderNativeUTXORail(coin iwallet.CoinType) bool {
	coinInfo, err := iwallet.CoinInfoFromCoinType(coin)
	return err == nil && coinInfo.IsNative && coinInfo.Chain.IsUTXOChain()
}

// UpdateCreateSessionRefundAddress saves the refund address carried by a
// CreateSession request and returns a fresh payment session projection without
// provisioning or re-provisioning a funding target.
func (c *CryptoPaymentFacade) UpdateCreateSessionRefundAddress(ctx context.Context, req contracts.CreatePaymentSessionRequest) (*paypb.PaymentSession, error) {
	coin := iwallet.CoinType(req.PaymentCoin)
	refundAddr, err := resolveCreateSessionRefundAddress(coin, req)
	if err != nil {
		return nil, err
	}
	if err := c.saveCreateSessionRefundAddress(ctx, coin, req.OrderID, refundAddr); err != nil {
		return nil, err
	}
	input, err := c.projector.fetchProjectInput(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("crypto facade: re-load order %s: %w", req.OrderID, err)
	}
	return c.projector.Project(input)
}

func (c *CryptoPaymentFacade) saveCreateSessionRefundAddress(ctx context.Context, coin iwallet.CoinType, orderID string, refundAddr string) error {
	if refundAddr == "" {
		return nil
	}
	if c.orderSvc == nil {
		return nil
	}
	if err := c.orderSvc.SetOrderRefundAddressForPayment(ctx, orderID, coin, refundAddr); err != nil {
		if errors.Is(err, coreiface.ErrBadRequest) || errors.Is(err, models.ErrRefundAddressRequired) || errors.Is(err, models.ErrRefundAddressInvalid) {
			return err
		}
		return fmt.Errorf("crypto facade: save refund address: %w", err)
	}
	return nil
}

func (c *CryptoPaymentFacade) validateStorePolicyModerator(ctx context.Context, orderID string, orderOpen *porderpb.OrderOpen, moderatorID string) (uint64, error) {
	if moderatorID == "" {
		return 0, nil
	}
	moderatorPeerID, err := peer.Decode(moderatorID)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid moderator ID", coreiface.ErrBadRequest)
	}
	if resolver := c.resolvedSellerPolicyResolver(); resolver != nil {
		policy, ok, err := resolver.SellerStorePolicy(ctx, orderID, orderOpen)
		if err != nil {
			return 0, err
		}
		if ok {
			return validateStorePolicyContainsModerator(policy, moderatorPeerID.String())
		}
	}
	if c.storePolicy == nil {
		return 0, fmt.Errorf("%w: store policy service is not available", coreiface.ErrBadRequest)
	}
	policy, err := c.storePolicy.GetPolicy(ctx)
	if err != nil {
		return 0, fmt.Errorf("get store policy: %w", err)
	}
	return validateStorePolicyContainsModerator(policy, moderatorPeerID.String())
}

type sellerStorePolicyResolver interface {
	SellerStorePolicy(ctx context.Context, orderID string, orderOpen *porderpb.OrderOpen) (*models.StorePolicy, bool, error)
}

type rawDBProvider interface {
	RawDB() *gorm.DB
}

func (c *CryptoPaymentFacade) resolvedSellerPolicyResolver() sellerStorePolicyResolver {
	if c.sellerPolicyResolver != nil {
		return c.sellerPolicyResolver
	}
	if c.db == nil {
		return nil
	}
	return dbSellerStorePolicyResolver{db: c.db}
}

type dbSellerStorePolicyResolver struct {
	db database.Database
}

func (r dbSellerStorePolicyResolver) SellerStorePolicy(ctx context.Context, orderID string, orderOpen *porderpb.OrderOpen) (*models.StorePolicy, bool, error) {
	rawProvider, ok := r.db.(rawDBProvider)
	if !ok || rawProvider.RawDB() == nil {
		return nil, false, nil
	}
	raw := rawProvider.RawDB().WithContext(ctx)

	sellerTenantID, resolved, err := sellerTenantIDForStorePolicy(raw, orderID, orderOpen)
	if err != nil {
		return nil, true, err
	}
	if !resolved {
		return nil, false, nil
	}

	var policy models.StorePolicy
	err = raw.Where("tenant_id = ?", sellerTenantID).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, true, fmt.Errorf("%w: moderator is not in store policy", coreiface.ErrBadRequest)
	}
	if err != nil {
		return nil, true, fmt.Errorf("get seller store policy: %w", err)
	}
	if err := raw.
		Where("tenant_id = ?", sellerTenantID).
		Order("position ASC, created_at ASC").
		Find(&policy.Moderators).Error; err != nil {
		return nil, true, fmt.Errorf("get seller store moderators: %w", err)
	}
	return &policy, true, nil
}

func sellerTenantIDForStorePolicy(raw *gorm.DB, orderID string, orderOpen *porderpb.OrderOpen) (string, bool, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID != "" {
		var vendorOrder models.Order
		err := raw.
			Where("id = ? AND my_role = ?", orderID, string(models.RoleVendor)).
			First(&vendorOrder).Error
		if err == nil && vendorOrder.TenantID != "" {
			return vendorOrder.TenantID, true, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && !isTableLookupUnavailable(err) {
			return "", true, fmt.Errorf("get seller order for store policy: %w", err)
		}
	}

	vendorPeerID, ok, err := sellerPeerIDFromOrderOpen(orderOpen)
	if err != nil {
		return "", true, err
	}
	if !ok {
		return "", false, nil
	}

	var row struct {
		AccountID string `gorm:"column:account_id"`
	}
	err = raw.
		Table("account_peer_ids").
		Select("account_id").
		Where("peer_id = ?", vendorPeerID).
		Take(&row).Error
	if err == nil && strings.TrimSpace(row.AccountID) != "" {
		return strings.TrimSpace(row.AccountID), true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || isTableLookupUnavailable(err) {
		return "", false, nil
	}
	return "", true, fmt.Errorf("resolve seller tenant for store policy: %w", err)
}

func sellerPeerIDFromOrderOpen(orderOpen *porderpb.OrderOpen) (string, bool, error) {
	if orderOpen == nil {
		return "", false, nil
	}
	for _, listing := range orderOpen.GetListings() {
		if listing == nil || listing.GetListing() == nil || listing.GetListing().GetVendorID() == nil {
			continue
		}
		vendorPeerID := strings.TrimSpace(listing.GetListing().GetVendorID().GetPeerID())
		if vendorPeerID == "" {
			continue
		}
		pid, err := peer.Decode(vendorPeerID)
		if err != nil {
			return "", false, fmt.Errorf("%w: invalid vendor peer ID", coreiface.ErrBadRequest)
		}
		return pid.String(), true, nil
	}
	return "", false, nil
}

func isTableLookupUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "unknown table")
}

func validateStorePolicyContainsModerator(policy *models.StorePolicy, canonicalModeratorID string) (uint64, error) {
	if policy == nil {
		return 0, fmt.Errorf("%w: moderator is not in store policy", coreiface.ErrBadRequest)
	}
	for _, mod := range policy.Moderators {
		if mod.PeerID != canonicalModeratorID {
			continue
		}
		if !mod.Enabled {
			return 0, fmt.Errorf("%w: moderator is disabled in store policy", coreiface.ErrBadRequest)
		}
		return policy.Revision, nil
	}
	return 0, fmt.Errorf("%w: moderator is not in store policy", coreiface.ErrBadRequest)
}

func resolveCreateSessionRefundAddress(coin iwallet.CoinType, req contracts.CreatePaymentSessionRequest) (string, error) {
	explicit := strings.TrimSpace(req.RefundAddress)
	if req.PayFromCustodial {
		if explicit == "" {
			return "", fmt.Errorf("%w: %w: refund address is required when paying from an exchange or custodial wallet", coreiface.ErrBadRequest, models.ErrRefundAddressRequired)
		}
		if err := models.ValidateRefundAddress(coin, explicit); err != nil {
			return "", fmt.Errorf("%w: %w", coreiface.ErrBadRequest, err)
		}
		return explicit, nil
	}
	if explicit != "" {
		if err := models.ValidateRefundAddress(coin, explicit); err != nil {
			return "", fmt.Errorf("%w: %w", coreiface.ErrBadRequest, err)
		}
		return explicit, nil
	}
	// Client-signed escrow setup may still forward payerAddress as refund.
	if payer := strings.TrimSpace(req.PayerAddress); payer != "" {
		if err := models.ValidateRefundAddress(coin, payer); err != nil {
			return "", fmt.Errorf("%w: %w", coreiface.ErrBadRequest, err)
		}
		return payer, nil
	}
	return "", nil
}

func buildPaymentSetupParamsFromOrder(
	order *models.Order,
	orderOpen *porderpb.OrderOpen,
	coin iwallet.CoinType,
	payerAddress, refundAddress, moderator string,
	authorizedPaymentAmount string,
	ex contracts.ExchangeRateService,
) (paypb.PaymentSetupParams, error) {
	if order == nil {
		return paypb.PaymentSetupParams{}, errors.New("order is nil")
	}
	orderAmount := iwallet.NewAmount(orderOpen.Amount)
	pricingCoin := strings.ToUpper(orderOpen.PricingCoin)
	paymentCoinCode, err := coin.PricingCurrencyCode()
	if err != nil {
		return paypb.PaymentSetupParams{}, fmt.Errorf("coin type pricing code: %w", err)
	}

	var amt uint64
	if strings.TrimSpace(authorizedPaymentAmount) != "" {
		quoted, ok := new(big.Int).SetString(strings.TrimSpace(authorizedPaymentAmount), 10)
		if !ok || quoted.Sign() <= 0 || !quoted.IsUint64() {
			return paypb.PaymentSetupParams{}, errors.New("authorized payment amount is invalid")
		}
		amt = quoted.Uint64()
	} else if pricingCoin != "" && pricingCoin != paymentCoinCode {
		// Cross-currency order: pricing coin differs from payment coin.
		// ExchangeRateService is required; its adapter returns an informative error
		// (not a panic) when the underlying provider is nil, so errors propagate
		// cleanly to the caller as ErrExchangeRateUnavailable-wrapped messages.
		if ex == nil {
			return paypb.PaymentSetupParams{}, fmt.Errorf(
				"%w: order priced in %s but payment coin is %s",
				ErrExchangeRateUnavailable, pricingCoin, paymentCoinCode)
		}
		pricingCurrency, err := models.CurrencyDefinitions.Lookup(pricingCoin)
		if err != nil {
			return paypb.PaymentSetupParams{}, fmt.Errorf("unknown pricing currency %q: %w", pricingCoin, err)
		}
		paymentCurrency, err := models.CurrencyDefinitions.Lookup(paymentCoinCode)
		if err != nil {
			return paypb.PaymentSetupParams{}, fmt.Errorf("unknown payment currency %q: %w", paymentCoinCode, err)
		}
		converted, err := wallet.ConvertCurrencyAmount(
			&models.CurrencyValue{Amount: orderAmount, Currency: pricingCurrency},
			paymentCurrency,
			ex,
		)
		if err != nil {
			return paypb.PaymentSetupParams{}, fmt.Errorf("convert payment amount: %w", err)
		}
		amt = converted.Uint64()
	} else {
		amt = orderAmount.Uint64()
	}

	return paypb.PaymentSetupParams{
		OrderID:       order.ID.String(),
		PayerAddress:  payerAddress,
		RefundAddress: refundAddress,
		Moderator:     moderator,
		CoinType:      coin,
		Amount:        amt,
		OrderData:     order,
	}, nil
}
