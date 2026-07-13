package core

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	gosolana "github.com/gagliardetto/solana-go"
	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const maxManagedSolanaMessageSize = 4096

type distributionManagedSolanaSigner struct {
	keys       contracts.KeyProvider
	settlement contracts.SettlementSigner
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
	var publicKey gosolana.PublicKey
	var signature []byte
	if request.AttemptScope != nil {
		signer, ok := s.settlement.(contracts.SolanaSettlementSigner)
		if !ok {
			return "", nil, fmt.Errorf("distribution Solana signer: attempt-scoped signer unavailable")
		}
		scope := request.AttemptScope
		scopeCoin, scopeErr := iwallet.CoinInfoFromCoinType(iwallet.CoinType(scope.KeyRef.RailID))
		if scopeErr != nil || scopeCoin.Chain != iwallet.ChainSolana {
			return "", nil, fmt.Errorf("distribution Solana signer: attempt rail is not Solana")
		}
		publicKeyBytes, signed, signErr := signer.SignSolanaMessage(ctx, contracts.SolanaMessageSettlementSignRequest{
			KeyRef: scope.KeyRef, OrderID: request.OrderID, AttemptID: scope.AttemptID,
			Action: request.ActionKind, Sequence: scope.Sequence, TermsHash: scope.TermsHash,
			ProgramAddress: request.ProgramAddress, EscrowAddress: request.EscrowAddress,
			Message: append([]byte(nil), request.Message...),
		})
		if signErr != nil {
			return "", nil, fmt.Errorf("distribution Solana signer: %w", signErr)
		}
		if len(publicKeyBytes) != gosolana.PublicKeyLength {
			return "", nil, fmt.Errorf("distribution Solana signer: invalid attempt public key")
		}
		publicKey = gosolana.PublicKeyFromBytes(publicKeyBytes)
		signature = signed
	} else {
		key, err := s.key(ctx)
		if err != nil {
			return "", nil, err
		}
		publicKey = key.PublicKey()
		signature = ed25519.Sign(ed25519.PrivateKey(*key), request.Message)
	}
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
