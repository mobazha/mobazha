package core

import (
	"errors"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type sweepCapabilityWallet struct {
	iwallet.Wallet
}

func (sweepCapabilityWallet) BuildSweepTx(_ []iwallet.SweepInput, _ btcec.PrivateKey, _ string, _ int64) ([]byte, string, error) {
	return []byte{0x01}, "sweep", nil
}

func (sweepCapabilityWallet) BuildTransferTx(_ []iwallet.SweepInput, _ btcec.PrivateKey, _, _ string, _ int64, _ int64) ([]byte, string, error) {
	return []byte{0x01}, "transfer", nil
}

type nonSweepCapabilityWallet struct {
	iwallet.Wallet
}

type capabilityMultiwallet struct {
	chains  []iwallet.ChainType
	wallets map[iwallet.ChainType]iwallet.Wallet
}

func (m *capabilityMultiwallet) Start() error { return nil }
func (m *capabilityMultiwallet) Close() error { return nil }
func (m *capabilityMultiwallet) SupportedChains() []iwallet.ChainType {
	return append([]iwallet.ChainType(nil), m.chains...)
}
func (m *capabilityMultiwallet) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	wallet, ok := m.wallets[chain]
	return wallet, ok
}
func (m *capabilityMultiwallet) WalletForCurrencyCode(string) (iwallet.Wallet, error) {
	return nil, errors.New("not implemented")
}

func TestDetectGuestUTXOChainsUsesWalletSweepCapability(t *testing.T) {
	mw := &capabilityMultiwallet{
		chains: []iwallet.ChainType{
			iwallet.ChainBitcoin,
			iwallet.ChainBitcoinCash,
			iwallet.ChainLitecoin,
			iwallet.ChainZCash,
		},
		wallets: map[iwallet.ChainType]iwallet.Wallet{
			iwallet.ChainBitcoin:     sweepCapabilityWallet{},
			iwallet.ChainBitcoinCash: sweepCapabilityWallet{},
			iwallet.ChainLitecoin:    sweepCapabilityWallet{},
			iwallet.ChainZCash:       nonSweepCapabilityWallet{},
		},
	}
	node := &MobazhaNode{walletFields: walletFields{multiwallet: mw}}

	require.Equal(t, []iwallet.ChainType{
		iwallet.ChainBitcoin,
		iwallet.ChainBitcoinCash,
		iwallet.ChainLitecoin,
	}, node.detectGuestUTXOChains())
}
