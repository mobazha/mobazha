// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package evmvault

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
)

const (
	obligationOpen    = uint8(1)
	obligationFunded  = uint8(2)
	obligationSlashed = uint8(4)
)

type Backend interface {
	bind.ContractBackend
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	BlockNumber(context.Context) (uint64, error)
	ChainID(context.Context) (*big.Int, error)
}

type TransactOptsFactory func(context.Context) (*bind.TransactOpts, error)

type BindingClient struct {
	config       Config
	backend      Backend
	contract     *CollateralVault
	newAuth      TransactOptsFactory
	pollInterval time.Duration
}

func NewBindingClient(config Config, backend Backend, newAuth TransactOptsFactory) (*BindingClient, error) {
	if backend == nil || newAuth == nil {
		return nil, fmt.Errorf("EVM collateral binding backend and signer are required")
	}
	contract, err := NewCollateralVault(config.VaultAddress, backend)
	if err != nil {
		return nil, err
	}
	return &BindingClient{
		config: config, backend: backend, contract: contract, newAuth: newAuth,
		pollInterval: time.Second,
	}, nil
}

func (c *BindingClient) CheckReady(ctx context.Context) error {
	if err := c.checkIdentity(ctx); err != nil {
		return err
	}
	call := &bind.CallOpts{Context: ctx}
	paused, err := c.contract.Paused(call)
	if err != nil {
		return fmt.Errorf("read EVM collateral vault pause state: %w", err)
	}
	if paused {
		return fmt.Errorf("EVM collateral vault is paused")
	}
	operatorRole, err := c.contract.OPERATORROLE(call)
	if err != nil {
		return fmt.Errorf("read EVM collateral operator role: %w", err)
	}
	authorized, err := c.contract.HasRole(call, operatorRole, c.config.OperatorAddress)
	if err != nil {
		return fmt.Errorf("read EVM collateral operator authorization: %w", err)
	}
	if !authorized {
		return fmt.Errorf("EVM collateral operator is not authorized")
	}
	return nil
}

func (c *BindingClient) checkIdentity(ctx context.Context) error {
	chainID, err := c.backend.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("read EVM collateral chain ID: %w", err)
	}
	if chainID.Sign() <= 0 || !chainID.IsUint64() || chainID.Uint64() != c.config.ChainID {
		return fmt.Errorf("EVM collateral chain ID mismatch")
	}
	code, err := c.backend.CodeAt(ctx, c.config.VaultAddress, nil)
	if err != nil {
		return fmt.Errorf("read EVM collateral vault code: %w", err)
	}
	if len(code) == 0 {
		return fmt.Errorf("EVM collateral vault is not deployed")
	}
	call := &bind.CallOpts{Context: ctx}
	version, err := c.contract.VERSION(call)
	if err != nil {
		return fmt.Errorf("read EVM collateral vault version: %w", err)
	}
	if version != VaultVersion {
		return fmt.Errorf("EVM collateral vault version mismatch")
	}
	token, err := c.contract.Asset(call)
	if err != nil {
		return fmt.Errorf("read EVM collateral vault asset: %w", err)
	}
	if token != c.config.TokenAddress {
		return fmt.Errorf("EVM collateral vault token mismatch")
	}
	return nil
}

func (c *BindingClient) EnsureObligation(ctx context.Context, command ObligationCommand) (string, error) {
	if obligation, err := c.contract.GetObligation(&bind.CallOpts{Context: ctx}, command.CollateralKey); err == nil {
		if err := verifyObligation(obligation, command, true); err != nil {
			return "", err
		}
		return "", nil
	}
	auth, err := c.transactOpts(ctx)
	if err != nil {
		return "", err
	}
	tx, err := c.contract.CreateObligation(auth, command.CollateralKey, command.Principal, command.Amount, command.ExpiresAt)
	if err != nil {
		return "", fmt.Errorf("submit EVM collateral obligation: %w", err)
	}
	if _, err := c.waitSuccessfulReceipt(ctx, tx.Hash()); err != nil {
		return "", err
	}
	obligation, err := c.contract.GetObligation(&bind.CallOpts{Context: ctx}, command.CollateralKey)
	if err != nil {
		return "", fmt.Errorf("read created EVM collateral obligation: %w", err)
	}
	if err := verifyObligation(obligation, command, true); err != nil {
		return "", err
	}
	return tx.Hash().Hex(), nil
}

func (c *BindingClient) FundingCallData(collateralKey, fundingKey [32]byte, amount *big.Int) ([]byte, error) {
	parsed, err := CollateralVaultMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return parsed.Pack("fund", collateralKey, fundingKey, amount)
}

func (c *BindingClient) FundingStatus(ctx context.Context, query FundingQuery) (pkgcollateral.RailFundingStatus, error) {
	if err := c.checkIdentity(ctx); err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	event, err := c.findFundingEvent(ctx, query)
	if err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	if event == nil {
		obligation, readErr := c.contract.GetObligation(&bind.CallOpts{Context: ctx}, query.CollateralKey)
		if readErr != nil {
			return pkgcollateral.RailFundingStatus{}, fmt.Errorf("read EVM collateral funding obligation: %w", readErr)
		}
		if obligation.State != obligationOpen {
			return pkgcollateral.RailFundingStatus{}, fmt.Errorf("EVM collateral funding event is unavailable from configured start block")
		}
		state := pkgcollateral.RailActionPending
		lastError := ""
		header, headerErr := c.backend.HeaderByNumber(ctx, nil)
		if headerErr != nil {
			return pkgcollateral.RailFundingStatus{}, headerErr
		}
		if obligation.State == obligationOpen && header.Time > obligation.ExpiresAt {
			state = pkgcollateral.RailActionFailed
			lastError = "collateral funding window expired"
		}
		return pkgcollateral.RailFundingStatus{
			State: state, AssetID: c.config.AssetID, Amount: query.MinimumAmount.String(), LastError: lastError,
		}, nil
	}
	if event.Amount.Cmp(query.MinimumAmount) < 0 || event.TotalBalance.Cmp(event.Amount) != 0 {
		return pkgcollateral.RailFundingStatus{}, fmt.Errorf("EVM collateral funding event amount mismatch")
	}
	confirmed, confirmations, observedAt, err := c.verifyEvent(ctx, event.Raw)
	if err != nil {
		return pkgcollateral.RailFundingStatus{}, err
	}
	state := pkgcollateral.RailActionPending
	if confirmed {
		state = pkgcollateral.RailActionConfirmed
	}
	return pkgcollateral.RailFundingStatus{
		State: state, Reference: event.Raw.TxHash.Hex(), AssetID: c.config.AssetID,
		Amount: event.Amount.String(), Confirmations: confirmations, ObservedAt: observedAt,
	}, nil
}

func (c *BindingClient) SubmitExecution(ctx context.Context, command ExecutionCommand) (pkgcollateral.RailActionResult, error) {
	current, err := c.ExecutionStatus(ctx, command)
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	if current.State == pkgcollateral.RailActionConfirmed {
		return current, nil
	}
	expectedDigest, err := executionDigest(command)
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	existing, err := c.contract.ActionDigests(&bind.CallOpts{Context: ctx}, command.ActionKey)
	if err != nil {
		return pkgcollateral.RailActionResult{}, fmt.Errorf("read EVM collateral action digest: %w", err)
	}
	if existing != ([32]byte{}) {
		if existing != expectedDigest {
			return pkgcollateral.RailActionResult{}, fmt.Errorf("EVM collateral action digest conflict")
		}
		return pkgcollateral.RailActionResult{}, fmt.Errorf("EVM collateral execution event is unavailable from configured start block")
	}
	obligation, err := c.contract.GetObligation(&bind.CallOpts{Context: ctx}, command.CollateralKey)
	if err != nil {
		return pkgcollateral.RailActionResult{}, fmt.Errorf("read EVM collateral execution obligation: %w", err)
	}
	if err := verifyExecutionObligation(obligation, command); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	auth, err := c.transactOpts(ctx)
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	var tx *types.Transaction
	if command.Request.Kind == pkgcollateral.ExecutionRelease {
		tx, err = c.contract.Release(auth, command.CollateralKey, command.ActionKey, command.Destination, command.Amount)
	} else {
		tx, err = c.contract.Slash(auth, command.CollateralKey, command.ActionKey, command.Destination, command.Amount)
	}
	if err != nil {
		return pkgcollateral.RailActionResult{}, fmt.Errorf("submit EVM collateral %s: %w", command.Request.Kind, err)
	}
	return pkgcollateral.RailActionResult{
		ActionID: command.Request.ActionID, State: pkgcollateral.RailActionPending, Reference: tx.Hash().Hex(),
	}, nil
}

func (c *BindingClient) ExecutionStatus(ctx context.Context, command ExecutionCommand) (pkgcollateral.RailActionResult, error) {
	if err := c.checkIdentity(ctx); err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	var raw *types.Log
	if command.Request.Kind == pkgcollateral.ExecutionRelease {
		iterator, err := c.contract.FilterCollateralReleased(&bind.FilterOpts{Start: c.config.StartBlock, Context: ctx}, [][32]byte{command.CollateralKey}, [][32]byte{command.ActionKey}, []common.Address{command.Destination})
		if err != nil {
			return pkgcollateral.RailActionResult{}, err
		}
		defer iterator.Close()
		for iterator.Next() {
			event := iterator.Event
			if event.Amount.Cmp(command.Amount) != 0 {
				return pkgcollateral.RailActionResult{}, fmt.Errorf("EVM collateral release event amount mismatch")
			}
			if raw != nil {
				return pkgcollateral.RailActionResult{}, fmt.Errorf("duplicate EVM collateral release event")
			}
			copyLog := event.Raw
			raw = &copyLog
		}
		if err := iterator.Error(); err != nil {
			return pkgcollateral.RailActionResult{}, err
		}
	} else {
		iterator, err := c.contract.FilterCollateralSlashed(&bind.FilterOpts{Start: c.config.StartBlock, Context: ctx}, [][32]byte{command.CollateralKey}, [][32]byte{command.ActionKey}, []common.Address{command.Destination})
		if err != nil {
			return pkgcollateral.RailActionResult{}, err
		}
		defer iterator.Close()
		for iterator.Next() {
			event := iterator.Event
			if event.Amount.Cmp(command.Amount) != 0 {
				return pkgcollateral.RailActionResult{}, fmt.Errorf("EVM collateral slash event amount mismatch")
			}
			if raw != nil {
				return pkgcollateral.RailActionResult{}, fmt.Errorf("duplicate EVM collateral slash event")
			}
			copyLog := event.Raw
			raw = &copyLog
		}
		if err := iterator.Error(); err != nil {
			return pkgcollateral.RailActionResult{}, err
		}
	}
	if raw == nil {
		expectedDigest, err := executionDigest(command)
		if err != nil {
			return pkgcollateral.RailActionResult{}, err
		}
		existing, err := c.contract.ActionDigests(&bind.CallOpts{Context: ctx}, command.ActionKey)
		if err != nil {
			return pkgcollateral.RailActionResult{}, fmt.Errorf("read EVM collateral action digest: %w", err)
		}
		if existing != ([32]byte{}) {
			if existing != expectedDigest {
				return pkgcollateral.RailActionResult{}, fmt.Errorf("EVM collateral action digest conflict")
			}
			return pkgcollateral.RailActionResult{}, fmt.Errorf("EVM collateral execution event is unavailable from configured start block")
		}
		return pkgcollateral.RailActionResult{ActionID: command.Request.ActionID, State: pkgcollateral.RailActionPending}, nil
	}
	confirmed, _, observedAt, err := c.verifyEvent(ctx, *raw)
	if err != nil {
		return pkgcollateral.RailActionResult{}, err
	}
	state := pkgcollateral.RailActionPending
	if confirmed {
		state = pkgcollateral.RailActionConfirmed
	}
	return pkgcollateral.RailActionResult{
		ActionID: command.Request.ActionID, State: state, Reference: raw.TxHash.Hex(), ObservedAt: observedAt,
	}, nil
}

func (c *BindingClient) findFundingEvent(ctx context.Context, query FundingQuery) (*CollateralVaultCollateralFunded, error) {
	iterator, err := c.contract.FilterCollateralFunded(&bind.FilterOpts{Start: c.config.StartBlock, Context: ctx}, [][32]byte{query.CollateralKey}, [][32]byte{query.FundingKey}, []common.Address{query.Principal})
	if err != nil {
		return nil, err
	}
	defer iterator.Close()
	var found *CollateralVaultCollateralFunded
	for iterator.Next() {
		if found != nil {
			return nil, fmt.Errorf("duplicate EVM collateral funding event")
		}
		copyEvent := *iterator.Event
		found = &copyEvent
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	return found, nil
}

func (c *BindingClient) verifyEvent(ctx context.Context, event types.Log) (bool, uint64, time.Time, error) {
	if event.Removed || event.Address != c.config.VaultAddress {
		return false, 0, time.Time{}, fmt.Errorf("EVM collateral event is not canonical")
	}
	receipt, err := c.backend.TransactionReceipt(ctx, event.TxHash)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("read EVM collateral receipt: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockHash != event.BlockHash || receipt.BlockNumber == nil || receipt.BlockNumber.Uint64() != event.BlockNumber || !receiptContainsLog(receipt, event) {
		return false, 0, time.Time{}, fmt.Errorf("EVM collateral receipt verification failed")
	}
	header, err := c.backend.HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("read EVM collateral receipt block: %w", err)
	}
	if header == nil || header.Number == nil || header.Number.Cmp(receipt.BlockNumber) != 0 || header.Hash() != receipt.BlockHash {
		return false, 0, time.Time{}, fmt.Errorf("EVM collateral receipt block is not canonical")
	}
	latest, err := c.backend.BlockNumber(ctx)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("read EVM collateral confirmation head: %w", err)
	}
	if latest < event.BlockNumber {
		return false, 0, time.Time{}, fmt.Errorf("EVM collateral confirmation head precedes receipt")
	}
	confirmations := latest - event.BlockNumber + 1
	confirmed := confirmations >= c.config.Confirmations
	return confirmed, confirmations, time.Unix(int64(header.Time), 0).UTC(), nil
}

func (c *BindingClient) waitSuccessfulReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()
	for {
		receipt, err := c.backend.TransactionReceipt(ctx, hash)
		if err == nil {
			if receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil {
				return nil, fmt.Errorf("EVM collateral transaction reverted")
			}
			header, headerErr := c.backend.HeaderByNumber(ctx, receipt.BlockNumber)
			if headerErr != nil {
				return nil, fmt.Errorf("read EVM collateral transaction block: %w", headerErr)
			}
			if header == nil || header.Number == nil || header.Number.Cmp(receipt.BlockNumber) != 0 || header.Hash() != receipt.BlockHash {
				return nil, fmt.Errorf("EVM collateral transaction block is not canonical")
			}
			latest, headErr := c.backend.BlockNumber(ctx)
			if headErr != nil {
				return nil, headErr
			}
			if latest >= receipt.BlockNumber.Uint64() && latest-receipt.BlockNumber.Uint64()+1 >= c.config.Confirmations {
				return receipt, nil
			}
		} else if !errors.Is(err, ethereum.NotFound) {
			return nil, fmt.Errorf("read EVM collateral transaction receipt: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *BindingClient) transactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	auth, err := c.newAuth(ctx)
	if err != nil {
		return nil, err
	}
	if auth == nil || auth.From != c.config.OperatorAddress {
		return nil, fmt.Errorf("EVM collateral signer does not match operator")
	}
	copyAuth := *auth
	copyAuth.Context = ctx
	return &copyAuth, nil
}

func verifyObligation(obligation CollateralVaultObligation, command ObligationCommand, requireOpen bool) error {
	if obligation.Principal != command.Principal || obligation.RequiredAmount == nil || obligation.RequiredAmount.Cmp(command.Amount) != 0 || obligation.ExpiresAt != command.ExpiresAt {
		return fmt.Errorf("EVM collateral obligation binding mismatch")
	}
	if requireOpen && obligation.State != obligationOpen {
		return fmt.Errorf("EVM collateral obligation is not open")
	}
	return nil
}

func verifyExecutionObligation(obligation CollateralVaultObligation, command ExecutionCommand) error {
	if obligation.Balance == nil || obligation.Balance.Sign() <= 0 {
		return fmt.Errorf("EVM collateral obligation has no balance")
	}
	if obligation.State != obligationFunded && obligation.State != obligationSlashed {
		return fmt.Errorf("EVM collateral obligation is not executable")
	}
	if command.Request.Kind == pkgcollateral.ExecutionRelease {
		if obligation.Principal != command.Destination || obligation.Balance.Cmp(command.Amount) != 0 {
			return fmt.Errorf("EVM collateral release does not match principal residual balance")
		}
	} else {
		if command.Amount.Cmp(obligation.Balance) > 0 {
			return fmt.Errorf("EVM collateral slash exceeds obligation balance")
		}
		remaining := new(big.Int).Sub(obligation.Balance, command.Amount)
		if remaining.Sign() > 0 && (obligation.RequiredAmount == nil || remaining.Cmp(obligation.RequiredAmount) < 0) {
			return fmt.Errorf("EVM collateral slash would leave an underfunded residual")
		}
	}
	return nil
}

func executionDigest(command ExecutionCommand) ([32]byte, error) {
	stringType, err := abi.NewType("string", "", nil)
	if err != nil {
		return [32]byte{}, err
	}
	bytes32Type, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		return [32]byte{}, err
	}
	addressType, err := abi.NewType("address", "", nil)
	if err != nil {
		return [32]byte{}, err
	}
	uintType, err := abi.NewType("uint256", "", nil)
	if err != nil {
		return [32]byte{}, err
	}
	kind := "RELEASE"
	if command.Request.Kind == pkgcollateral.ExecutionSlash {
		kind = "SLASH"
	}
	encoded, err := (abi.Arguments{{Type: stringType}, {Type: bytes32Type}, {Type: bytes32Type}, {Type: addressType}, {Type: uintType}}).
		Pack(kind, command.CollateralKey, command.ActionKey, command.Destination, command.Amount)
	if err != nil {
		return [32]byte{}, err
	}
	return crypto.Keccak256Hash(encoded), nil
}

func receiptContainsLog(receipt *types.Receipt, expected types.Log) bool {
	for _, entry := range receipt.Logs {
		if entry.Address == expected.Address && entry.Index == expected.Index && entry.TxHash == expected.TxHash && bytes.Equal(entry.Data, expected.Data) && equalTopics(entry.Topics, expected.Topics) {
			return true
		}
	}
	return false
}

func equalTopics(left, right []common.Hash) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

var _ VaultClient = (*BindingClient)(nil)
