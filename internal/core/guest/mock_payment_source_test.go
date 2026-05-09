package guest

import (
	"context"
	"sync"

	pkgutxo "github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// mockPaymentSource implements pkgutxo.PaymentSource for testing UTXO payment detection.
type mockPaymentSource struct {
	mu          sync.Mutex
	subscribeCB map[string]func(tx *iwallet.Transaction)
	chainType   iwallet.ChainType
}

func newMockPaymentSource(chain iwallet.ChainType) *mockPaymentSource {
	return &mockPaymentSource{
		subscribeCB: make(map[string]func(tx *iwallet.Transaction)),
		chainType:   chain,
	}
}

func (m *mockPaymentSource) Subscribe(_ context.Context, addr string, _ []byte, cb func(*iwallet.Transaction)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribeCB[addr] = cb
	return nil
}

func (m *mockPaymentSource) Unsubscribe(_ context.Context, addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscribeCB, addr)
	return nil
}

func (m *mockPaymentSource) GetTransactions(_ context.Context, _ string, _ []byte) ([]*iwallet.Transaction, error) {
	return nil, nil
}

func (m *mockPaymentSource) GetTransaction(_ context.Context, _ string) (*iwallet.Transaction, error) {
	return nil, nil
}

func (m *mockPaymentSource) IsHealthy() bool { return true }

func (m *mockPaymentSource) Chain() iwallet.ChainType { return m.chainType }

func (m *mockPaymentSource) Close() error { return nil }

func (m *mockPaymentSource) ListUnspent(_ context.Context, _ []byte) ([]pkgutxo.UnspentOutput, error) {
	return nil, nil
}

func (m *mockPaymentSource) GetTxConfirmations(_ context.Context, _ string) (int, error) {
	return 3, nil
}

// SimulatePayment triggers the subscription callback for the given address.
func (m *mockPaymentSource) SimulatePayment(addr string, tx *iwallet.Transaction) {
	m.mu.Lock()
	cb, ok := m.subscribeCB[addr]
	m.mu.Unlock()
	if ok {
		cb(tx)
	}
}
