package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
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
	BuyerRefundAddress      string
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
	storePolicy          contracts.StorePolicyService
	sellerPolicyResolver sellerStorePolicyResolver
	settlementStarter    standardOrderSettlementAuthorizationStarter
	settlementEligible   func(iwallet.CoinType) bool
	quoteBoundEligible   func(iwallet.CoinType) bool
}

// SetQuoteBoundSettlementAuthorizationEligibility wires the rail-scoped v2
// writer capability. It is evaluated separately from v1 route capability.
func (c *CryptoPaymentFacade) SetQuoteBoundSettlementAuthorizationEligibility(
	eligible func(iwallet.CoinType) bool,
) {
	if c != nil {
		c.quoteBoundEligible = eligible
	}
}

// SetStandardOrderSettlementAuthorizationEligibility wires the runtime
// capability check used before the facade cuts a rail over to authorization.
func (c *CryptoPaymentFacade) SetStandardOrderSettlementAuthorizationEligibility(
	eligible func(iwallet.CoinType) bool,
) {
	if c != nil {
		c.settlementEligible = eligible
	}
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
	storePolicy contracts.StorePolicyService,
) *CryptoPaymentFacade {
	return &CryptoPaymentFacade{
		db:          db,
		projector:   NewPaymentSessionProjector(db),
		orderSvc:    orderSvc,
		setupSvc:    setupSvc,
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
	crossCurrency := !coin.MatchesPricingCurrency(orderOpen.PricingCoin)
	v1Eligible := c.settlementEligible != nil && c.settlementEligible(coin) &&
		standardOrderSettlementAuthorizationV1Eligible(coin, orderOpen)
	v2Eligible := c.quoteBoundEligible != nil && c.quoteBoundEligible(coin) &&
		standardOrderSettlementAuthorizationV2Eligible(coin, orderOpen, req.PaymentSelectionQuoteID)
	if crossCurrency {
		if strings.TrimSpace(req.PaymentSelectionQuoteID) == "" || strings.TrimSpace(req.AuthorizedPaymentAmount) == "" {
			return nil, fmt.Errorf("%w: cross-currency crypto payment requires a resolved quote", ErrDealPaymentSelectionQuoteInvalid)
		}
		if !v2Eligible {
			return nil, fmt.Errorf("%w: quote-bound settlement authorization is unavailable for rail %s", ErrProvisioningNotImplemented, coin)
		}
		if c.settlementStarter == nil {
			return nil, fmt.Errorf("%w: quote-bound settlement authorization starter is unavailable", ErrProvisioningNotImplemented)
		}
		if !standardOrderSettlementAuthorizationEconomicEligible(order) {
			return nil, fmt.Errorf("%w: quote-bound settlement authorization does not admit the order payment costs", ErrDealPaymentSelectionQuoteInvalid)
		}
	}
	if c.settlementStarter != nil && (v1Eligible || v2Eligible) {
		if standardOrderSettlementAuthorizationEconomicEligible(order) {
			setupParams, err := buildPaymentSetupParamsFromOrder(
				order, orderOpen, coin, req.PayerAddress, refundAddr, moderator,
				req.AuthorizedPaymentAmount,
			)
			if err != nil {
				return nil, fmt.Errorf("crypto facade: build settlement authorization params: %w", err)
			}
			if err := c.saveCreateSessionRefundAddress(ctx, coin, req.OrderID, refundAddr); err != nil {
				return nil, err
			}
			startRequest := standardOrderSettlementAuthorizationStartRequest(
				req, strconv.FormatUint(setupParams.Amount, 10), moderator, refundAddr,
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
	}

	setupParams, err := buildPaymentSetupParamsFromOrder(order, orderOpen, coin,
		req.PayerAddress, refundAddr, moderator, req.AuthorizedPaymentAmount)
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
	buyerRefundAddress string,
) StandardOrderSettlementAuthorizationStartRequest {
	return StandardOrderSettlementAuthorizationStartRequest{
		OrderID: req.OrderID, PaymentSelectionQuoteID: req.PaymentSelectionQuoteID,
		RailID: req.PaymentCoin, AmountAtomic: strings.TrimSpace(amountAtomic),
		ModeratorPeerID:    strings.TrimSpace(moderatorPeerID),
		BuyerRefundAddress: strings.TrimSpace(buyerRefundAddress),
	}
}

func standardOrderUTXOAuthorizationEligible(coin iwallet.CoinType, orderOpen *porderpb.OrderOpen) bool {
	return standardOrderNativeUTXORail(coin) && standardOrderSettlementAuthorizationV1Eligible(coin, orderOpen)
}

// standardOrderSettlementAuthorizationV1Eligible is an explicit protocol
// version gate, not a product rule. Quote-bound cross-currency attempts use v2.
func standardOrderSettlementAuthorizationV1Eligible(coin iwallet.CoinType, orderOpen *porderpb.OrderOpen) bool {
	if orderOpen == nil {
		return false
	}
	return coin.MatchesPricingCurrency(orderOpen.PricingCoin)
}

func standardOrderSettlementAuthorizationV2Eligible(
	coin iwallet.CoinType,
	orderOpen *porderpb.OrderOpen,
	quoteID string,
) bool {
	return orderOpen != nil && strings.TrimSpace(quoteID) != "" &&
		!coin.MatchesPricingCurrency(orderOpen.PricingCoin)
}

func standardOrderSettlementAuthorizationEconomicEligible(order *models.Order) bool {
	if order == nil {
		return false
	}
	cancelFee := strings.TrimSpace(order.CancelFeeAmount)
	return cancelFee == "" || cancelFee == "0"
}

func (c *CryptoPaymentFacade) abandonUnsupportedSettlementAuthorizationDraft(
	ctx context.Context, order *models.Order, attempt *models.PaymentAttempt,
) (bool, error) {
	if c == nil || order == nil || attempt == nil || standardOrderSettlementAuthorizationEconomicEligible(order) {
		return false, nil
	}
	rawProvider, ok := c.db.(rawDBProvider)
	if !ok || rawProvider.RawDB() == nil {
		return false, fmt.Errorf("crypto facade: raw database is unavailable for draft recovery")
	}
	tenantID := strings.TrimSpace(order.TenantID)
	if tenantID == "" {
		tenantID = database.StandaloneTenantID
	}
	return paymentintent.AbandonCryptoPaymentAttemptDraft(
		rawProvider.RawDB().WithContext(ctx), tenantID, order.ID.String(), attempt.AttemptID,
	)
}

func (c *CryptoPaymentFacade) expireQuoteBoundSettlementAuthorizationDraft(
	ctx context.Context,
	order *models.Order,
	attempt *models.PaymentAttempt,
	now time.Time,
) (bool, error) {
	if c == nil || order == nil || attempt == nil || attempt.State != models.PaymentAttemptAuthorizationDraft {
		return false, nil
	}
	basis, err := attempt.GetFundingBasis()
	if err != nil {
		return false, fmt.Errorf("crypto facade: load quote-bound settlement funding basis: %w", err)
	}
	if basis == nil || basis.ExpiresAtUnix > now.UTC().Unix() {
		return false, nil
	}
	rawProvider, ok := c.db.(rawDBProvider)
	if !ok || rawProvider.RawDB() == nil {
		return false, fmt.Errorf("crypto facade: raw database is unavailable for quote-bound draft expiry")
	}
	tenantID := strings.TrimSpace(order.TenantID)
	if tenantID == "" {
		tenantID = database.StandaloneTenantID
	}
	return paymentintent.ExpireCryptoPaymentAttemptDraft(
		rawProvider.RawDB().WithContext(ctx), tenantID, order.ID.String(), attempt.AttemptID,
	)
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
) (paypb.PaymentSetupParams, error) {
	if order == nil {
		return paypb.PaymentSetupParams{}, errors.New("order is nil")
	}
	orderAmount := iwallet.NewAmount(orderOpen.Amount)

	var amt uint64
	if strings.TrimSpace(authorizedPaymentAmount) != "" {
		quoted, ok := new(big.Int).SetString(strings.TrimSpace(authorizedPaymentAmount), 10)
		if !ok || quoted.Sign() <= 0 || !quoted.IsUint64() {
			return paypb.PaymentSetupParams{}, errors.New("authorized payment amount is invalid")
		}
		amt = quoted.Uint64()
	} else if !coin.MatchesPricingCurrency(orderOpen.PricingCoin) {
		return paypb.PaymentSetupParams{}, fmt.Errorf(
			"%w: cross-currency crypto payment requires a resolved quote amount",
			ErrDealPaymentSelectionQuoteInvalid,
		)
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
