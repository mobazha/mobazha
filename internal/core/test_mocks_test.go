package core

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"sync"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/identity"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/media"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
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
func (m *mockMessenger) Start()                                            {}
func (m *mockMessenger) Stop()                                             {}

// ── mockNetworkService ──────────────────────────────────────────

type mockNetworkService struct{}

var _ contracts.NetworkService = (*mockNetworkService)(nil)

func (m *mockNetworkService) SendMessage(_ context.Context, _ peer.ID, _ *pb.Message) error {
	return nil
}
func (m *mockNetworkService) RegisterHandler(_ pb.Message_MessageType, _ func(peer.ID, *pb.Message) error) {
}
func (m *mockNetworkService) DeliverLocalMessage(_ peer.ID, _ *pb.Message) error { return nil }
func (m *mockNetworkService) Close()                                              {}

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
