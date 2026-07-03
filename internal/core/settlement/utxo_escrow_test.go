package settlement

import (
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock UTXOMonitorService for verifyUTXOsOnChain tests ────────────────

type mockUTXOMonitor struct {
	watchedAddresses      map[string]*utxo.WatchedAddress
	addressTxs            map[string][]iwallet.Transaction
	addressTxsErr         error
	listUnspent           []utxo.UnspentOutput
	listUnspentErr        error
	listUnspentConfigured bool
}

func (m *mockUTXOMonitor) Start()                                     {}
func (m *mockUTXOMonitor) Stop()                                      {}
func (m *mockUTXOMonitor) WatchAddress(wa *utxo.WatchedAddress) error { return nil }
func (m *mockUTXOMonitor) UnwatchAddress(address string) error        { return nil }
func (m *mockUTXOMonitor) RegisterNodeCallback(string, func(iwallet.Transaction, *utxo.WatchedAddress)) error {
	return nil
}
func (m *mockUTXOMonitor) UnregisterNode(string) {}
func (m *mockUTXOMonitor) GetTransaction(iwallet.ChainType, string) (*iwallet.Transaction, error) {
	return nil, nil
}
func (m *mockUTXOMonitor) GetFeeEstimate(iwallet.ChainType, int) uint64 { return 10 }
func (m *mockUTXOMonitor) BroadcastTransaction(iwallet.ChainType, string) (string, error) {
	return "", nil
}
func (m *mockUTXOMonitor) IsHealthy(iwallet.ChainType) bool { return true }
func (m *mockUTXOMonitor) ListUnspent(iwallet.ChainType, []byte) ([]utxo.UnspentOutput, error) {
	if !m.listUnspentConfigured {
		return nil, errors.New("ListUnspent not configured")
	}
	if m.listUnspentErr != nil {
		return nil, m.listUnspentErr
	}
	return m.listUnspent, nil
}
func (m *mockUTXOMonitor) GetTxConfirmations(iwallet.ChainType, string) (int, error) {
	return 0, nil
}

func (m *mockUTXOMonitor) GetWatchedAddress(address string) *utxo.WatchedAddress {
	if m.watchedAddresses == nil {
		return nil
	}
	return m.watchedAddresses[address]
}

func (m *mockUTXOMonitor) GetAddressTransactions(_ iwallet.ChainType, address string, _ []byte) ([]iwallet.Transaction, error) {
	if m.addressTxsErr != nil {
		return nil, m.addressTxsErr
	}
	return m.addressTxs[address], nil
}

// canonical BTC coin type for verifyUTXOsOnChain tests
const testBTCCanonical = "crypto:bip122:000000000019d6689c085ae165831e93:native"

// ── Helper ──────────────────────────────────────────────────────────────

func newTestSettlementServiceForUTXO(monitor utxo.UTXOMonitorService) *SettlementService {
	return &SettlementService{
		nodeID:         "test-node",
		monitorService: monitor,
	}
}

// ── Tests ───────────────────────────────────────────────────────────────

func TestVerifyUTXOsOnChain_NoMonitor(t *testing.T) {
	svc := newTestSettlementServiceForUTXO(nil)
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", []iwallet.SpendInfo{{ID: []byte{0x01}}})
	assert.NoError(t, err)
}

func TestVerifyUTXOsOnChain_NonUTXOCoin(t *testing.T) {
	mon := &mockUTXOMonitor{}
	svc := newTestSettlementServiceForUTXO(mon)
	err := svc.verifyUTXOsOnChain("crypto:eip155:1:native", "0xaddr", []iwallet.SpendInfo{{ID: []byte{0x01}}})
	assert.NoError(t, err)
}

func TestVerifyUTXOsOnChain_AddressNotWatched(t *testing.T) {
	mon := &mockUTXOMonitor{watchedAddresses: map[string]*utxo.WatchedAddress{}}
	svc := newTestSettlementServiceForUTXO(mon)
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", []iwallet.SpendInfo{{ID: []byte{0x01}}})
	assert.NoError(t, err)
}

func TestVerifyUTXOsOnChain_ChainQueryFails(t *testing.T) {
	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"1addr": {Address: "1addr", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoin},
		},
		addressTxsErr: errors.New("network error"),
	}
	svc := newTestSettlementServiceForUTXO(mon)
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", []iwallet.SpendInfo{{ID: []byte{0x01}}})
	assert.NoError(t, err)
}

func TestVerifyUTXOsOnChain_UTXOsConfirmedUnspent(t *testing.T) {
	utxoID := []byte{0x01, 0x02}
	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"1addr": {Address: "1addr", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoin},
		},
		addressTxs: map[string][]iwallet.Transaction{
			"1addr": {
				{
					To: []iwallet.SpendInfo{
						{ID: utxoID, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(50000)},
					},
				},
			},
		},
	}
	svc := newTestSettlementServiceForUTXO(mon)

	expected := []iwallet.SpendInfo{{ID: utxoID, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(50000)}}
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", expected)
	assert.NoError(t, err)
}

func TestVerifyUTXOsOnChain_ListUnspentConfirmedUnspent(t *testing.T) {
	const txHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	utxoID, ok := payment.UTXOOutpointID(txHash, 1)
	require.True(t, ok)

	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"1addr": {Address: "1addr", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoin},
		},
		listUnspentConfigured: true,
		listUnspent: []utxo.UnspentOutput{{
			TxHash:      txHash,
			OutputIndex: 1,
			Value:       50000,
		}},
	}
	svc := newTestSettlementServiceForUTXO(mon)

	expected := []iwallet.SpendInfo{{ID: utxoID, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(50000)}}
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", expected)
	assert.NoError(t, err)
}

func TestVerifyUTXOsOnChain_ListUnspentMissingBlocksRelease(t *testing.T) {
	const txHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	utxoID, ok := payment.UTXOOutpointID(txHash, 1)
	require.True(t, ok)

	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"1addr": {Address: "1addr", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoin},
		},
		listUnspentConfigured: true,
		listUnspent: []utxo.UnspentOutput{{
			TxHash:      txHash,
			OutputIndex: 0,
			Value:       50000,
		}},
	}
	svc := newTestSettlementServiceForUTXO(mon)

	expected := []iwallet.SpendInfo{{ID: utxoID, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(50000)}}
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", expected)
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrUTXOAlreadySpent)
}

func TestVerifyUTXOsOnChain_UTXOAddressPrefixDifference(t *testing.T) {
	utxoID := []byte{0x01, 0x02}
	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"pp0cn2dcd83": {Address: "pp0cn2dcd83", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoinCash},
		},
		addressTxs: map[string][]iwallet.Transaction{
			"pp0cn2dcd83": {
				{
					To: []iwallet.SpendInfo{
						{ID: utxoID, Address: iwallet.NewAddress("bitcoincash:pp0cn2dcd83", "BCH"), Amount: iwallet.NewAmount(50000)},
					},
				},
			},
		},
	}
	svc := newTestSettlementServiceForUTXO(mon)

	expected := []iwallet.SpendInfo{{ID: utxoID, Address: iwallet.NewAddress("pp0cn2dcd83", "BCH"), Amount: iwallet.NewAmount(50000)}}
	err := svc.verifyUTXOsOnChain("crypto:bitcoincash:mainnet:native", "pp0cn2dcd83", expected)
	assert.NoError(t, err)
}

func TestVerifyUTXOsOnChain_UTXONotFoundButNoSpendObservedIsBestEffort(t *testing.T) {
	utxoID := []byte{0x01, 0x02}
	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"1addr": {Address: "1addr", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoin},
		},
		addressTxs: map[string][]iwallet.Transaction{
			"1addr": {
				{
					To: []iwallet.SpendInfo{
						{ID: []byte{0x03}, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(50000)},
					},
				},
			},
		},
	}
	svc := newTestSettlementServiceForUTXO(mon)

	expected := []iwallet.SpendInfo{{ID: utxoID, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(50000)}}
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", expected)
	assert.NoError(t, err)
}

func TestVerifyUTXOsOnChain_UTXOAlreadySpent(t *testing.T) {
	utxoID := []byte{0x01, 0x02}
	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"1addr": {Address: "1addr", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoin},
		},
		addressTxs: map[string][]iwallet.Transaction{
			"1addr": {
				{
					To: []iwallet.SpendInfo{
						{ID: utxoID, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(50000)},
					},
				},
				{
					From: []iwallet.SpendInfo{
						{ID: utxoID},
					},
					To: []iwallet.SpendInfo{
						{ID: []byte{0x03}, Address: iwallet.NewAddress("other", "BTC"), Amount: iwallet.NewAmount(49000)},
					},
				},
			},
		},
	}
	svc := newTestSettlementServiceForUTXO(mon)

	expected := []iwallet.SpendInfo{{ID: utxoID, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(50000)}}
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", expected)
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrUTXOAlreadySpent)
}

func TestVerifyUTXOsOnChain_MultipleUTXOs_PartialSpent(t *testing.T) {
	utxoA := []byte{0x0a}
	utxoB := []byte{0x0b}
	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"1addr": {Address: "1addr", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoin},
		},
		addressTxs: map[string][]iwallet.Transaction{
			"1addr": {
				{
					To: []iwallet.SpendInfo{
						{ID: utxoA, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(30000)},
						{ID: utxoB, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(20000)},
					},
				},
				{
					From: []iwallet.SpendInfo{{ID: utxoA}},
					To: []iwallet.SpendInfo{
						{ID: []byte{0x0c}, Address: iwallet.NewAddress("other", "BTC"), Amount: iwallet.NewAmount(29000)},
					},
				},
			},
		},
	}
	svc := newTestSettlementServiceForUTXO(mon)

	expected := []iwallet.SpendInfo{
		{ID: utxoA, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(30000)},
		{ID: utxoB, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(20000)},
	}
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", expected)
	require.Error(t, err)
	assert.ErrorIs(t, err, contracts.ErrUTXOAlreadySpent)
	assert.Contains(t, err.Error(), "0a")
}

func TestVerifyUTXOsOnChain_AllUnspent(t *testing.T) {
	utxoA := []byte{0x0a}
	utxoB := []byte{0x0b}
	mon := &mockUTXOMonitor{
		watchedAddresses: map[string]*utxo.WatchedAddress{
			"1addr": {Address: "1addr", ScriptPubKey: []byte{0xaa}, ChainType: iwallet.ChainBitcoin},
		},
		addressTxs: map[string][]iwallet.Transaction{
			"1addr": {
				{
					To: []iwallet.SpendInfo{
						{ID: utxoA, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(30000)},
						{ID: utxoB, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(20000)},
					},
				},
			},
		},
	}
	svc := newTestSettlementServiceForUTXO(mon)

	expected := []iwallet.SpendInfo{
		{ID: utxoA, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(30000)},
		{ID: utxoB, Address: iwallet.NewAddress("1addr", "BTC"), Amount: iwallet.NewAmount(20000)},
	}
	err := svc.verifyUTXOsOnChain(testBTCCanonical, "1addr", expected)
	assert.NoError(t, err)
}
