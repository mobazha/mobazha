package core

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/media"
	pb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/multiformats/go-multihash"
)

// ── mockContentStore ────────────────────────────────────────────

type mockContentStore struct{}

var _ contracts.ContentStore = (*mockContentStore)(nil)

func (m *mockContentStore) ComputeCID(data []byte) (cid.Cid, error) {
	return media.ComputeUnixFSCID(data)
}

// ── mockMessenger ───────────────────────────────────────────────

type mockMessenger struct {
	mu   sync.Mutex
	sent []sentMessage
}

type sentMessage struct {
	PeerID peer.ID
	Msg    *pb.Message
}

var _ contracts.Messenger = (*mockMessenger)(nil)

func (m *mockMessenger) ReliablySendMessage(_ database.Tx, p peer.ID, msg *pb.Message, done chan<- struct{}) error {
	m.mu.Lock()
	m.sent = append(m.sent, sentMessage{PeerID: p, Msg: msg})
	m.mu.Unlock()
	if done != nil {
		close(done)
	}
	return nil
}

func (m *mockMessenger) ProcessACK(_ database.Tx, _ *pb.AckMessage) error { return nil }
func (m *mockMessenger) SendACK(_ string, _ peer.ID)                      {}
func (m *mockMessenger) Start()                                           {}
func (m *mockMessenger) Stop()                                            {}

// ── mockNetworkService ──────────────────────────────────────────

type mockNetworkService struct{}

var _ contracts.NetworkService = (*mockNetworkService)(nil)

func (m *mockNetworkService) SendMessage(_ context.Context, _ peer.ID, _ *pb.Message) error {
	return nil
}
func (m *mockNetworkService) RegisterHandler(_ pb.Message_MessageType, _ func(peer.ID, *pb.Message) error) {
}
func (m *mockNetworkService) DeliverLocalMessage(_ peer.ID, _ *pb.Message) error { return nil }
func (m *mockNetworkService) Close()                                             {}

// ── mockSigner ──────────────────────────────────────────────────

type mockSigner struct {
	privKey ed25519.PrivateKey
	pubKey  ed25519.PublicKey
	pid     identity.PeerID
}

var _ contracts.Signer = (*mockSigner)(nil)

func newMockSigner() *mockSigner {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	kp, _ := identity.KeyPairFromPrivateKey(priv)
	pid, _ := identity.PeerIDFromPublicKey(kp.PubKey)
	return &mockSigner{privKey: priv, pubKey: pub, pid: pid}
}

func (m *mockSigner) Sign(message []byte) ([]byte, error) {
	return ed25519.Sign(m.privKey, message), nil
}

func (m *mockSigner) Verify(message []byte, signature []byte) (bool, error) {
	return ed25519.Verify(m.pubKey, message, signature), nil
}

func (m *mockSigner) PublicKey() (ed25519.PublicKey, error) {
	return m.pubKey, nil
}

func (m *mockSigner) PeerID() identity.PeerID {
	return m.pid
}

// ── mockKeyProvider ─────────────────────────────────────────────

type mockKeyProvider struct{}

var _ contracts.KeyProvider = (*mockKeyProvider)(nil)

func (m *mockKeyProvider) EVMMasterKey() (*btcec.PrivateKey, error) {
	k, _ := btcec.NewPrivateKey()
	return k, nil
}

func (m *mockKeyProvider) SolanaMasterKey() (*solana.PrivateKey, error) {
	pk := solana.NewWallet().PrivateKey
	return &pk, nil
}

func (m *mockKeyProvider) EscrowMasterKey() (*btcec.PrivateKey, error) {
	k, _ := btcec.NewPrivateKey()
	return k, nil
}

func (m *mockKeyProvider) RatingMasterKey() (*btcec.PrivateKey, error) {
	k, _ := btcec.NewPrivateKey()
	return k, nil
}

func (m *mockKeyProvider) TRONMasterKey() (*btcec.PrivateKey, error) {
	k, _ := btcec.NewPrivateKey()
	return k, nil
}

func (m *mockKeyProvider) DigitalContentMasterKey(version int) ([]byte, error) {
	return make([]byte, 32), nil
}

// ── mockWalletOperator ──────────────────────────────────────────
//
// A no-op contracts.WalletOperator used when tests need to satisfy
// the dependency surface (e.g., managed EVM adapter shadow registration in
// registerPaymentStrategies) without standing up a real multiwallet.
// Every wallet lookup returns errMockWalletUnsupported so any unintended
// payment path fails loudly rather than silently no-oping.

type mockWalletOperator struct{}

var _ contracts.WalletOperator = (*mockWalletOperator)(nil)

var errMockWalletUnsupported = errors.New("mockWalletOperator: wallet not supported")

func (m *mockWalletOperator) WalletForCurrencyCode(_ string) (iwallet.Wallet, error) {
	return nil, errMockWalletUnsupported
}
func (m *mockWalletOperator) WalletForChain(_ iwallet.ChainType) (iwallet.Wallet, bool) {
	return nil, false
}
func (m *mockWalletOperator) SupportedChains() []iwallet.ChainType { return nil }
func (m *mockWalletOperator) Start() error                         { return nil }
func (m *mockWalletOperator) Close() error                         { return nil }

type mockWalletOperatorWithChainWallets struct {
	wallets map[iwallet.ChainType]iwallet.Wallet
}

var _ contracts.WalletOperator = (*mockWalletOperatorWithChainWallets)(nil)

func (m *mockWalletOperatorWithChainWallets) WalletForCurrencyCode(code string) (iwallet.Wallet, error) {
	return nil, errMockWalletUnsupported
}
func (m *mockWalletOperatorWithChainWallets) WalletForChain(chain iwallet.ChainType) (iwallet.Wallet, bool) {
	wallet, ok := m.wallets[chain]
	return wallet, ok
}
func (m *mockWalletOperatorWithChainWallets) SupportedChains() []iwallet.ChainType {
	out := make([]iwallet.ChainType, 0, len(m.wallets))
	for chain := range m.wallets {
		out = append(out, chain)
	}
	return out
}
func (m *mockWalletOperatorWithChainWallets) Start() error { return nil }
func (m *mockWalletOperatorWithChainWallets) Close() error { return nil }

type mockEVMWallet struct {
	chain       iwallet.ChainType
	coin        iwallet.CoinType
	testnet     bool
	chainClient iwallet.ChainClient
}

var _ iwallet.Wallet = (*mockEVMWallet)(nil)

func newMockEVMWallet(chain iwallet.ChainType, client iwallet.ChainClient) *mockEVMWallet {
	return newMockEVMWalletWithTestnet(chain, client, false)
}

func newMockEVMWalletWithTestnet(chain iwallet.ChainType, client iwallet.ChainClient, testnet bool) *mockEVMWallet {
	coin, err := iwallet.RequireCanonicalNativeCoinType(chain)
	if err != nil {
		panic(err)
	}
	return &mockEVMWallet{
		chain:       chain,
		coin:        coin,
		testnet:     testnet,
		chainClient: client,
	}
}

func (m *mockEVMWallet) WalletExists() bool { return true }
func (m *mockEVMWallet) CreateWallet(_ hdkeychain.ExtendedKey, _ time.Time) error {
	return nil
}
func (m *mockEVMWallet) OpenWallet() error  { return nil }
func (m *mockEVMWallet) CloseWallet() error { return nil }
func (m *mockEVMWallet) Begin() (iwallet.Tx, error) {
	return nil, errMockWalletUnsupported
}
func (m *mockEVMWallet) BlockchainInfo() (iwallet.BlockInfo, error) {
	return iwallet.BlockInfo{}, nil
}
func (m *mockEVMWallet) CoinCategory() iwallet.CoinCategory { return iwallet.CoinCategoryEthereum }
func (m *mockEVMWallet) IsTestnet() bool                    { return m.testnet }
func (m *mockEVMWallet) ValidateAddress(_ iwallet.Address) error {
	return nil
}
func (m *mockEVMWallet) GetTransaction(id iwallet.TransactionID, coinType iwallet.CoinType) (*iwallet.Transaction, error) {
	return m.chainClient.GetTransaction(id, coinType)
}
func (m *mockEVMWallet) GetChainClient() iwallet.ChainClient { return m.chainClient }

// ── helpers ─────────────────────────────────────────────────────

func testCID() cid.Cid {
	mh, _ := multihash.Sum([]byte("test"), multihash.SHA2_256, -1)
	return cid.NewCidV1(cid.Raw, mh)
}

func noopPublish(done chan<- struct{}) {
	if done != nil {
		close(done)
	}
}
