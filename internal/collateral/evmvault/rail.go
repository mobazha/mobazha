// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package evmvault

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha/pkg/assetid"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
)

const (
	DefaultRailID = "evm-erc20-collateral-vault"
	VaultVersion  = "1.0.0"
	custodyModel  = "dedicated-erc20-vault"
	payloadType   = "mobazha-evm-collateral-vault/v1"
)

type Config struct {
	RailID          string
	AssetID         string
	ChainID         uint64
	VaultAddress    common.Address
	TokenAddress    common.Address
	OperatorAddress common.Address
	StartBlock      uint64
	Confirmations   uint64
}

type ObligationCommand struct {
	CollateralKey [32]byte
	Principal     common.Address
	Amount        *big.Int
	ExpiresAt     uint64
}

type FundingQuery struct {
	CollateralKey [32]byte
	FundingKey    [32]byte
	Principal     common.Address
	MinimumAmount *big.Int
}

type ExecutionCommand struct {
	Request       pkgcollateral.RailExecutionRequest
	CollateralKey [32]byte
	ActionKey     [32]byte
	Destination   common.Address
	Amount        *big.Int
}

// VaultClient owns EVM RPC, signing, receipt confirmation, and event decoding.
// Rail retains the domain mapping and verifies every returned projection.
type VaultClient interface {
	CheckReady(context.Context) error
	EnsureObligation(context.Context, ObligationCommand) (string, error)
	FundingCallData([32]byte, [32]byte, *big.Int) ([]byte, error)
	FundingStatus(context.Context, FundingQuery) (pkgcollateral.RailFundingStatus, error)
	SubmitExecution(context.Context, ExecutionCommand) (pkgcollateral.RailActionResult, error)
	ExecutionStatus(context.Context, ExecutionCommand) (pkgcollateral.RailActionResult, error)
}

type Rail struct {
	config Config
	client VaultClient
	now    func() time.Time
}

type fundingPayload struct {
	Type                string `json:"type"`
	ChainID             uint64 `json:"chainID"`
	VaultAddress        string `json:"vaultAddress"`
	TokenAddress        string `json:"tokenAddress"`
	PrincipalAddress    string `json:"principalAddress"`
	CollateralKey       string `json:"collateralKey"`
	FundingKey          string `json:"fundingKey"`
	CallData            string `json:"callData"`
	ApprovalSpender     string `json:"approvalSpender"`
	ApprovalAmount      string `json:"approvalAmount"`
	ObligationReference string `json:"obligationReference,omitempty"`
}

func NewRail(config Config, client VaultClient) (*Rail, error) {
	if client == nil {
		return nil, fmt.Errorf("EVM collateral vault client is required")
	}
	if strings.TrimSpace(config.RailID) == "" {
		config.RailID = DefaultRailID
	}
	if config.ChainID == 0 || config.VaultAddress == (common.Address{}) || config.TokenAddress == (common.Address{}) || config.OperatorAddress == (common.Address{}) {
		return nil, fmt.Errorf("EVM collateral rail chain, vault, token, and operator are required")
	}
	if config.Confirmations == 0 {
		return nil, fmt.Errorf("EVM collateral rail confirmations are required")
	}
	if config.StartBlock == 0 {
		return nil, fmt.Errorf("EVM collateral rail deployment start block is required")
	}
	id, err := assetid.Parse(config.AssetID)
	if err != nil {
		return nil, fmt.Errorf("EVM collateral rail asset: %w", err)
	}
	if config.AssetID != id.String() || id.Namespace != assetid.NamespaceEIP155 || id.Standard != assetid.StandardERC20 || id.ChainRef != strconv.FormatUint(config.ChainID, 10) || common.HexToAddress(id.AssetRef) != config.TokenAddress {
		return nil, fmt.Errorf("EVM collateral rail asset does not match chain and token")
	}
	return &Rail{config: config, client: client, now: time.Now}, nil
}

func (r *Rail) Descriptor() pkgcollateral.RailDescriptor {
	return pkgcollateral.RailDescriptor{
		ID: r.config.RailID, Version: VaultVersion, CustodyModel: custodyModel,
		Assets: []string{r.config.AssetID}, SupportsFundingTargets: true,
		SupportsFundingObserve: true, SupportsPrincipalRelease: true, SupportsClaimSlash: true,
		SupportsReconciliation: true, HasReceiptVerification: true,
	}
}

func (r *Rail) PrepareFunding(ctx context.Context, request pkgcollateral.FundingTargetRequest) (pkgcollateral.FundingTarget, error) {
	now := r.now().UTC()
	if err := request.Validate(now); err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	if request.AssetID != r.config.AssetID {
		return pkgcollateral.FundingTarget{}, fmt.Errorf("EVM collateral funding asset mismatch")
	}
	principal, err := canonicalAddress(request.PrincipalDestination)
	if err != nil {
		return pkgcollateral.FundingTarget{}, fmt.Errorf("EVM collateral principal destination: %w", err)
	}
	amount, err := positiveAmount(request.Amount)
	if err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	if request.ExpiresAt.Unix() <= 0 {
		return pkgcollateral.FundingTarget{}, fmt.Errorf("EVM collateral funding expiry is unsupported")
	}
	if err := r.client.CheckReady(ctx); err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	collateralKey := domainKey("collateral", request.TenantID, request.CollateralID)
	fundingKey := domainKey("funding", request.TenantID, request.CollateralID, request.IdempotencyKey)
	reference, err := r.client.EnsureObligation(ctx, ObligationCommand{
		CollateralKey: collateralKey, Principal: principal, Amount: amount,
		ExpiresAt: uint64(request.ExpiresAt.Unix()),
	})
	if err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	callData, err := r.client.FundingCallData(collateralKey, fundingKey, amount)
	if err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	payload, err := json.Marshal(fundingPayload{
		Type: payloadType, ChainID: r.config.ChainID, VaultAddress: r.config.VaultAddress.Hex(),
		TokenAddress: r.config.TokenAddress.Hex(), PrincipalAddress: principal.Hex(),
		CollateralKey: hexKey(collateralKey), FundingKey: hexKey(fundingKey), CallData: "0x" + hex.EncodeToString(callData),
		ApprovalSpender: r.config.VaultAddress.Hex(), ApprovalAmount: request.Amount, ObligationReference: reference,
	})
	if err != nil {
		return pkgcollateral.FundingTarget{}, err
	}
	return pkgcollateral.FundingTarget{
		RailID: r.config.RailID, TenantID: request.TenantID, CollateralID: request.CollateralID,
		PrincipalDestination: request.PrincipalDestination, IdempotencyKey: request.IdempotencyKey, AssetID: request.AssetID,
		Amount: request.Amount, Destination: r.config.VaultAddress.Hex(), Payload: payload, ExpiresAt: request.ExpiresAt,
	}, nil
}

func (r *Rail) FundingStatus(ctx context.Context, target pkgcollateral.FundingTarget) (pkgcollateral.RailFundingStatus, error) {
	payload, amount, err := r.validateFundingTarget(target)
	if err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	status, err := r.client.FundingStatus(ctx, FundingQuery{
		CollateralKey: mustHexKey(payload.CollateralKey), FundingKey: mustHexKey(payload.FundingKey),
		Principal: common.HexToAddress(payload.PrincipalAddress), MinimumAmount: amount,
	})
	if err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	if status.AssetID == "" {
		status.AssetID = r.config.AssetID
	}
	if status.AssetID != r.config.AssetID {
		return pkgcollateral.RailFundingStatus{}, fmt.Errorf("EVM collateral funding status asset mismatch")
	}
	return status, status.Validate()
}

func (r *Rail) SubmitExecution(ctx context.Context, request pkgcollateral.RailExecutionRequest) (pkgcollateral.RailActionResult, error) {
	command, err := r.executionCommand(request)
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	if err := r.client.CheckReady(ctx); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	result, err := r.client.SubmitExecution(ctx, command)
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	return result, validateActionResult(request.ActionID, result)
}

func (r *Rail) ExecutionStatus(ctx context.Context, request pkgcollateral.RailExecutionRequest) (pkgcollateral.RailActionResult, error) {
	command, err := r.executionCommand(request)
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	result, err := r.client.ExecutionStatus(ctx, command)
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	return result, validateActionResult(request.ActionID, result)
}

func (r *Rail) executionCommand(request pkgcollateral.RailExecutionRequest) (ExecutionCommand, error) {
	if err := request.Validate(); err != nil {
		return ExecutionCommand{}, err
	}
	if request.AssetID != r.config.AssetID {
		return ExecutionCommand{}, fmt.Errorf("EVM collateral execution asset mismatch")
	}
	destination, err := canonicalAddress(request.Destination)
	if err != nil {
		return ExecutionCommand{}, fmt.Errorf("EVM collateral execution destination: %w", err)
	}
	amount, err := positiveAmount(request.Amount)
	if err != nil {
		return ExecutionCommand{}, err
	}
	return ExecutionCommand{
		Request: request, CollateralKey: domainKey("collateral", request.TenantID, request.CollateralID),
		ActionKey: executionKey(request), Destination: destination, Amount: amount,
	}, nil
}

func (r *Rail) validateFundingTarget(target pkgcollateral.FundingTarget) (fundingPayload, *big.Int, error) {
	if target.RailID != r.config.RailID || target.AssetID != r.config.AssetID || target.Destination != r.config.VaultAddress.Hex() {
		return fundingPayload{}, nil, fmt.Errorf("EVM collateral funding target binding mismatch")
	}
	amount, err := positiveAmount(target.Amount)
	if err != nil {
		return fundingPayload{}, nil, err
	}
	var payload fundingPayload
	if err := json.Unmarshal(target.Payload, &payload); err != nil {
		return fundingPayload{}, nil, fmt.Errorf("decode EVM collateral funding target: %w", err)
	}
	principal, err := canonicalAddress(payload.PrincipalAddress)
	if err != nil {
		return fundingPayload{}, nil, err
	}
	if payload.Type != payloadType || payload.ChainID != r.config.ChainID || payload.VaultAddress != r.config.VaultAddress.Hex() || payload.TokenAddress != r.config.TokenAddress.Hex() || payload.ApprovalSpender != r.config.VaultAddress.Hex() || payload.ApprovalAmount != target.Amount || principal.Hex() != payload.PrincipalAddress || payload.PrincipalAddress != target.PrincipalDestination {
		return fundingPayload{}, nil, fmt.Errorf("EVM collateral funding payload binding mismatch")
	}
	collateralKey, err := parseHexKey(payload.CollateralKey)
	if err != nil {
		return fundingPayload{}, nil, err
	}
	fundingKey, err := parseHexKey(payload.FundingKey)
	if err != nil {
		return fundingPayload{}, nil, err
	}
	if collateralKey != domainKey("collateral", target.TenantID, target.CollateralID) || fundingKey != domainKey("funding", target.TenantID, target.CollateralID, target.IdempotencyKey) {
		return fundingPayload{}, nil, fmt.Errorf("EVM collateral funding key binding mismatch")
	}
	callData, err := hex.DecodeString(strings.TrimPrefix(payload.CallData, "0x"))
	if err != nil || !strings.HasPrefix(payload.CallData, "0x") {
		return fundingPayload{}, nil, fmt.Errorf("EVM collateral funding calldata is invalid")
	}
	expectedCallData, err := r.client.FundingCallData(collateralKey, fundingKey, amount)
	if err != nil {
		return fundingPayload{}, nil, err
	}
	if !bytes.Equal(callData, expectedCallData) {
		return fundingPayload{}, nil, fmt.Errorf("EVM collateral funding calldata binding mismatch")
	}
	return payload, amount, nil
}

func validateActionResult(actionID string, result pkgcollateral.RailActionResult) error {
	if err := result.Validate(); err != nil {
		return err
	}
	if result.ActionID != actionID {
		return fmt.Errorf("EVM collateral execution result action mismatch")
	}
	return nil
}

func positiveAmount(value string) (*big.Int, error) {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok || amount.Sign() <= 0 || amount.String() != value {
		return nil, fmt.Errorf("EVM collateral amount must be canonical positive base units")
	}
	return amount, nil
}

func canonicalAddress(value string) (common.Address, error) {
	if !common.IsHexAddress(value) {
		return common.Address{}, fmt.Errorf("invalid address")
	}
	address := common.HexToAddress(value)
	if address == (common.Address{}) {
		return common.Address{}, fmt.Errorf("zero address is not allowed")
	}
	if value != address.Hex() {
		return common.Address{}, fmt.Errorf("address must use canonical checksum form")
	}
	return address, nil
}

func domainKey(kind string, values ...string) [32]byte {
	var encoded bytes.Buffer
	writeDomainField := func(value string) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		encoded.Write(size[:])
		encoded.WriteString(value)
	}
	writeDomainField("mobazha:collateral:v1")
	writeDomainField(kind)
	for _, value := range values {
		writeDomainField(value)
	}
	return crypto.Keccak256Hash(encoded.Bytes())
}

func executionKey(request pkgcollateral.RailExecutionRequest) [32]byte {
	return domainKey(
		"execution", request.ActionID, request.TenantID, request.CollateralID, request.ClaimID,
		string(request.Kind), request.AssetID, request.Amount, request.Destination,
		strconv.FormatUint(request.ExpectedRevision, 10), request.IdempotencyKey,
	)
}

func hexKey(value [32]byte) string { return common.BytesToHash(value[:]).Hex() }

func parseHexKey(value string) ([32]byte, error) {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return [32]byte{}, fmt.Errorf("EVM collateral key is invalid")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, fmt.Errorf("EVM collateral key is invalid")
	}
	var result [32]byte
	copy(result[:], decoded)
	return result, nil
}

func mustHexKey(value string) [32]byte {
	key, _ := parseHexKey(value)
	return key
}

var _ pkgcollateral.Rail = (*Rail)(nil)
