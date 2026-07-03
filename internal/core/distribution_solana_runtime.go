package core

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	gosolana "github.com/gagliardetto/solana-go"
	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const maxManagedSolanaMessageSize = 4096

type distributionManagedSolanaSigner struct {
	keys contracts.KeyProvider
}

type distributionManagedSolanaSetupService struct {
	service *corepayment.PaymentAppService
}

type distributionManagedSolanaOrderSource struct {
	db database.Database
}

func (s distributionManagedSolanaOrderSource) LoadManagedSolanaOrder(ctx context.Context, orderID string) (*models.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.db == nil || strings.TrimSpace(orderID) == "" {
		return nil, fmt.Errorf("distribution Solana order source: database and order ID are required")
	}
	var order models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().WithContext(ctx).Where("id = ?", orderID).First(&order).Error
	}); err != nil {
		return nil, fmt.Errorf("distribution Solana order source: load order %s: %w", orderID, err)
	}
	return &order, nil
}

func (s distributionManagedSolanaSetupService) PrepareManagedSolanaSetup(
	ctx context.Context,
	params payment.PaymentSetupParams,
	config distribution.ManagedSolanaSetupConfig,
) (*distribution.ManagedSolanaSetupIntent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.service == nil {
		return nil, fmt.Errorf("distribution Solana setup: payment service unavailable")
	}
	coinInfo, err := payment.SettlementCoinInfoForCoin(params.CoinType)
	if err != nil {
		return nil, fmt.Errorf("distribution Solana setup: %w", err)
	}
	if coinInfo.Chain != iwallet.ChainSolana {
		return nil, fmt.Errorf("distribution Solana setup: coin %s is not on Solana", params.CoinType)
	}
	program, err := gosolana.PublicKeyFromBase58(strings.TrimSpace(config.ProgramAddress))
	if err != nil || program.IsZero() {
		return nil, fmt.Errorf("distribution Solana setup: valid program address is required")
	}
	payer, err := gosolana.PublicKeyFromBase58(strings.TrimSpace(params.PayerAddress))
	if err != nil || payer.IsZero() {
		return nil, fmt.Errorf("distribution Solana setup: valid payer address is required")
	}
	orderInfo, err := s.service.GetOrderInfo(models.OrderID(params.OrderID), params.CoinType)
	if err != nil {
		return nil, fmt.Errorf("distribution Solana setup: order projection: %w", err)
	}
	method, moderatorAddress, requiredSignatures, err := s.service.GetModeratorEscrowInfo(ctx, params.Moderator, params.CoinType)
	if err != nil {
		return nil, fmt.Errorf("distribution Solana setup: moderator projection: %w", err)
	}
	refundAddress := strings.TrimSpace(params.RefundAddress)
	if refundAddress == "" {
		refundAddress = orderInfo.BuyerAddress
	}
	now := time.Now().UTC()
	unlockTime := now.Add(time.Duration(orderInfo.UnlockHours) * time.Hour).Unix()
	platformAuthority := strings.TrimSpace(config.PlatformAuthority)
	if platformAuthority == "" {
		platformAuthority = params.PayerAddress
	}
	feeCollector := strings.TrimSpace(config.PlatformFeeCollector)
	if feeCollector == "" {
		feeCollector = platformAuthority
	}
	rentCollector := strings.TrimSpace(config.RentCollector)
	if rentCollector == "" {
		rentCollector = platformAuthority
	}
	escrowInfo := iwallet.EscrowInfo{
		ContractAddress: program.String(), PayerAddress: params.PayerAddress,
		PlatformAuthority: platformAuthority, BuyerAddress: orderInfo.BuyerAddress,
		RefundAddress: refundAddress, SellerAddress: orderInfo.VendorAddress,
		ModeratorAddress: moderatorAddress, PlatformFeeCollector: feeCollector,
		RentCollector: rentCollector, UniqueId: orderInfo.UniqueId,
		RequiredSignatures: uint8(requiredSignatures), UnlockHours: uint64(orderInfo.UnlockHours),
		UnlockTime: unlockTime, FundingDeadline: unlockTime,
		CoinType: params.CoinType, Amount: params.Amount, Testnet: config.Testnet,
	}
	paymentTokenAddress := "0x0000000000000000000000000000000000000000"
	if !coinInfo.IsNative {
		paymentTokenAddress = coinInfo.ContractAddress(config.Testnet)
	}
	paymentData := &models.PaymentData{
		OrderID: params.OrderID, Coin: params.CoinType, Method: method,
		SettlementSpec:  payment.NewSolanaEscrowSpec(method == pb.PaymentSent_MODERATED).ToPending(),
		ContractAddress: program.String(), PayerAddress: params.PayerAddress,
		Moderator: params.Moderator, ModeratorAddress: moderatorAddress,
		Amount: params.Amount, FromID: padManagedSolanaID(payer.Bytes()),
		UnlockHours: uint32(orderInfo.UnlockHours), UnlockTime: unlockTime, FundingDeadline: unlockTime,
		PlatformAddr: feeCollector, RentCollector: rentCollector,
		RefundAddress: refundAddress, PaymentTokenAddress: paymentTokenAddress,
	}
	return &distribution.ManagedSolanaSetupIntent{EscrowInfo: escrowInfo, PaymentData: paymentData}, nil
}

func (s distributionManagedSolanaSetupService) CommitManagedSolanaSetup(
	ctx context.Context,
	intent distribution.ManagedSolanaSetupIntent,
	result distribution.ManagedSolanaSetupBuildResult,
) (*models.PaymentData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.service == nil || intent.PaymentData == nil {
		return nil, fmt.Errorf("distribution Solana setup: payment service and intent are required")
	}
	escrow, err := gosolana.PublicKeyFromBase58(strings.TrimSpace(result.EscrowAddress))
	if err != nil || escrow.IsZero() {
		return nil, fmt.Errorf("distribution Solana setup: valid derived escrow address is required")
	}
	paymentData := *intent.PaymentData
	paymentData.ToAddress = escrow.String()
	paymentData.ToID = padManagedSolanaID(escrow.Bytes())
	paymentData.Script = hex.EncodeToString(result.Script)
	if err := s.service.CommitManagedSolanaSetup(&paymentData); err != nil {
		return nil, fmt.Errorf("distribution Solana setup: commit: %w", err)
	}
	return &paymentData, nil
}

func padManagedSolanaID(value []byte) []byte {
	out := make([]byte, 36)
	copy(out, value)
	return out
}

func (s distributionManagedSolanaSigner) ManagedSolanaPublicKey(ctx context.Context) (string, error) {
	key, err := s.key(ctx)
	if err != nil {
		return "", err
	}
	return key.PublicKey().String(), nil
}

func (s distributionManagedSolanaSigner) SignManagedSolanaMessage(
	ctx context.Context,
	request distribution.ManagedSolanaSignRequest,
) (string, []byte, error) {
	if err := validateManagedSolanaSignRequest(request); err != nil {
		return "", nil, err
	}
	key, err := s.key(ctx)
	if err != nil {
		return "", nil, err
	}
	publicKey := key.PublicKey()
	authorized := false
	seen := make(map[gosolana.PublicKey]struct{}, len(request.AuthorizedSigners))
	for _, raw := range request.AuthorizedSigners {
		owner, err := gosolana.PublicKeyFromBase58(strings.TrimSpace(raw))
		if err != nil || owner.IsZero() {
			return "", nil, fmt.Errorf("distribution Solana signer: invalid authorized signer %q", raw)
		}
		if _, exists := seen[owner]; exists {
			return "", nil, fmt.Errorf("distribution Solana signer: duplicate authorized signer %s", owner)
		}
		seen[owner] = struct{}{}
		authorized = authorized || owner.Equals(publicKey)
	}
	if !authorized {
		return "", nil, fmt.Errorf("distribution Solana signer: local owner %s is outside the authorized owner set", publicKey)
	}
	signature := ed25519.Sign(ed25519.PrivateKey(*key), request.Message)
	return publicKey.String(), signature, nil
}

func (s distributionManagedSolanaSigner) key(ctx context.Context) (*gosolana.PrivateKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.keys == nil {
		return nil, fmt.Errorf("distribution Solana signer: key provider unavailable")
	}
	key, err := s.keys.SolanaMasterKey()
	if err != nil {
		return nil, fmt.Errorf("distribution Solana signer: load Solana key: %w", err)
	}
	if key == nil {
		return nil, fmt.Errorf("distribution Solana signer: Solana key unavailable")
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("distribution Solana signer: validate Solana key: %w", err)
	}
	return key, nil
}

func validateManagedSolanaSignRequest(request distribution.ManagedSolanaSignRequest) error {
	if request.Chain != iwallet.ChainSolana {
		return fmt.Errorf("distribution Solana signer: chain must be %s", iwallet.ChainSolana)
	}
	if request.Purpose != distribution.ManagedSolanaSignAnchorSettlement {
		return fmt.Errorf("distribution Solana signer: unsupported purpose %q", request.Purpose)
	}
	if strings.TrimSpace(request.OrderID) == "" || strings.TrimSpace(request.CorrelationID) == "" {
		return fmt.Errorf("distribution Solana signer: order and correlation IDs are required")
	}
	switch strings.TrimSpace(request.ActionKind) {
	case "confirm", "cancel", "seller_decline_refund", "complete", "dispute_release":
	default:
		return fmt.Errorf("distribution Solana signer: unsupported action %q", request.ActionKind)
	}
	program, err := gosolana.PublicKeyFromBase58(strings.TrimSpace(request.ProgramAddress))
	if err != nil || program.IsZero() {
		return fmt.Errorf("distribution Solana signer: valid program address is required")
	}
	escrow, err := gosolana.PublicKeyFromBase58(strings.TrimSpace(request.EscrowAddress))
	if err != nil || escrow.IsZero() {
		return fmt.Errorf("distribution Solana signer: valid escrow address is required")
	}
	if len(request.AuthorizedSigners) == 0 {
		return fmt.Errorf("distribution Solana signer: authorized signer set is required")
	}
	if len(request.Message) == 0 || len(request.Message) > maxManagedSolanaMessageSize {
		return fmt.Errorf("distribution Solana signer: message length must be between 1 and %d bytes", maxManagedSolanaMessageSize)
	}
	return nil
}

var (
	_ distribution.ManagedSolanaSigner       = distributionManagedSolanaSigner{}
	_ distribution.ManagedSolanaSetupService = distributionManagedSolanaSetupService{}
	_ distribution.ManagedSolanaOrderSource  = distributionManagedSolanaOrderSource{}
)
