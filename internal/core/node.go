package core

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha3.0/internal/channels"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/core/coreiface"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/netdb"
	"github.com/mobazha/mobazha3.0/internal/events"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/notifications"
	"github.com/mobazha/mobazha3.0/internal/orders"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/ipfs/kubo/core"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/op/go-logging"
)

// OpenBazaarNode holds all the components that make up a network node
// on the OpenBazaar network. It also exposes an exported API which can
// be used to control the node.
type OpenBazaarNode struct {
	nodeID string

	sharedManager *SharedManager

	// ipfsNode is the IPFS instance that powers this node.
	ipfsNode *core.IpfsNode

	// repo holds the database and public data directory.
	repo *repo.Repo

	// ethMasterKey represents an secp256k1 private key, the
	// public key of which is advertised by the node in its profile
	// and in listings to be used when building eth escrow transactions.
	ethMasterKey *btcec.PrivateKey

	// escrowMasterKey represents an secp256k1 private key, the
	// public key of which is advertised by the node in its profile
	// and in listings to be used when building escrow transactions.
	escrowMasterKey *btcec.PrivateKey

	// ratingMasterKey represents an secp256k1 private key that
	// we used to generate rating keys to sign ratings with.
	ratingMasterKey *btcec.PrivateKey

	// ipnsQuorum is the size of the IPNS quorum to use. Smaller quorums
	// resolve faster but run the risk of getting back older records.
	ipnsQuorum uint

	// ipnsResolver is the URL of a resolver that can be queried to resolve
	// IPNS records. If this is empty we will use the p2p network.
	ipnsResolver string

	// netDB is the endpoint that can be queried for profile
	// and listing data if it is not found in the p2p network.
	netDB *netdb.NetDB

	netConfig *config.NetConfig

	// messenger is the primary object used to send messages to other peers.
	// It ensures reliable delivery by persisting messages and retrying them.
	// Generally you should always send messages using this and not the
	// NetworkService as the later will only attempt to send direct messages
	// and will not retry.
	messenger *net.Messenger

	// networkService manages the sending and receiving of messages
	// on the OpenBazaar protocol.
	networkService *net.NetworkService

	// banManager holds a list of peers that have been banned by this node.
	banManager *net.BanManager

	// eventBus allows a subscriber to receive event notifications from the node.
	eventBus events.Bus

	// followerTracker tries to maintain connections to a minimum number of our
	// followers so that we can use them to push data for redundancy.
	followerTracker *FollowerTracker

	// multiwallet is a map of cyptocurrency wallets.
	multiwallet multiwallet.Multiwallet

	// orderProcessor is the engine we use for processing all orders.
	orderProcessor *orders.OrderProcessor

	// exchangeRates is a provider of exchange rate data for various currencies.
	exchangeRates *wallet.ExchangeRateProvider

	// notifier listens to events coming off the bus, marshals them to notifications
	// and sends them off to the websocket.
	notifier *notifications.Notifier

	// testnet is whether the this node is configured to use the test network.
	testnet bool

	// torOnly is whether the node is running in tor only mode.
	torOnly bool

	// publishActive is an atomic integer that represents the number of inflight
	// publishes.
	publishActive int32

	// publishChan is used to signal to the republish loop that a publish
	// has just completed and it should update it's last published time.
	publishChan chan pubCloser

	// ipfsOnlyMode signals that the node is running in IPFS only mode.
	ipfsOnlyMode bool

	// storeAndForwardServers is a list of string peerIDs of servers we use
	// as our store and forward nodes.
	storeAndForwardServers []string

	// boostrapPeers holds the peers we use to bootstrap the node.
	boostrapPeers []peer.ID

	// channels holds active chat channels
	channels map[string]*channels.Channel

	// shutdownTorFunc is used to shutdown the embedded Tor client.
	shutdownTorFunc func() error

	// initialBootstrapChan is closed after the initial IPFS bootstrap completes.
	initialBootstrapChan chan struct{}

	// shutdown is closed when the node is stopped. Any listening
	// goroutines can use this to terminate.
	shutdown chan struct{}
}

// IsDefaultNode returns whether this node is the default node.
func (n *OpenBazaarNode) IsDefaultNode() bool {
	return n.nodeID == repo.DefaultNodeID
}

// Start gets the node up and running and listens for a signal interrupt.
func (n *OpenBazaarNode) Start() {
	// Check repo migration
	go func() {
		if err := n.checkRepoMigration(); err != nil {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "checkRepoMigration failed, %v", err)
		}
	}()

	go n.bootstrapIPFS()
	if !n.ipfsOnlyMode {
		n.publishHandler()
		go n.messenger.Start()
		go n.followerTracker.Start()

		go n.orderProcessor.Start()
		go n.syncMessages()
		go func() {
			n.multiwallet.Start()
			n.listenWalletEvents()

			if n.IsDefaultNode() {
				go n.SharedManager().Start()
			}
			n.orderProcessor.CheckForMorePayments(false)
		}()

		go n.notifier.Start()
		go n.OpenSavedChannels()
		if err := n.removeDisabledCoinsFromListings(); err != nil && !os.IsNotExist(err) {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "Error removing disabled coins from listings: %s", err)
		}
		if err := n.updateSNFServers(); err != nil {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "Error updating store and forward servers in profile: %s", err)
		}

		go n.listenPubsub()
	}

	// Add log to verify connection reuse
	go func() {
		conns := n.ipfsNode.PeerHost.Network().Conns()
		for _, conn := range conns {
			streams := conn.GetStreams()
			logger.LogWithIDf(log, n.nodeID, logging.DEBUG, "Connection to %s has %d streams",
				conn.RemotePeer(), len(streams))
		}
	}()
}

func (n *OpenBazaarNode) checkRepoMigration() error {
	version, err := n.repo.ReadVersion()
	if err != nil {
		return err
	}

	if version == 0 {
		logger.LogWithIDf(log, n.nodeID, logging.INFO, "Migrate repo from version 0")
		err = n.migrateRepoFromVersion0()
		if err != nil {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "Migration error: %v", err)
		}

		logger.LogWithIDf(log, n.nodeID, logging.INFO, "Migrate repo from version 1")
		err = n.migrateRepoFromVersion1()
		if err != nil {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "Migration error: %v", err)
		}
	} else if version == 1 {
		logger.LogWithIDf(log, n.nodeID, logging.INFO, "Migrate repo from version 1")
		err = n.migrateRepoFromVersion1()
		if err != nil {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "Migration error: %v", err)
		}
	} else if version == 2 || version == 3 {
		logger.LogWithIDf(log, n.nodeID, logging.INFO, "Migrate repo from version 2")
		err = n.migrateRepoWithListingsUpdate()
		if err != nil {
			logger.LogWithIDf(log, n.nodeID, logging.ERROR, "Migration error: %v", err)
		}
	}

	if version != repo.DefaultRepoVersion {
		if err := n.repo.WriteVersion(repo.DefaultRepoVersion); err != nil {
			return err
		}
	}
	return nil
}

// Do profile and listings migration with ETH pubKey adding
func (n *OpenBazaarNode) migrateRepoFromVersion0() error {
	done1 := make(chan struct{})
	myProfile, err := n.GetMyProfile()
	if err != nil {
		return fmt.Errorf("get my profile failed, %v", err)
	}
	err = n.SetProfile(myProfile, done1)
	if err != nil {
		return fmt.Errorf("update profile failed, %v", err)
	}

	select {
	case <-done1:
		break
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on profile update")
	}

	done2 := make(chan struct{})
	err = n.UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		listing.VendorID.Pubkeys.Eth = n.ethMasterKey.PubKey().SerializeCompressed()
		return true, nil
	}, done2)
	if err != nil {
		return fmt.Errorf("update listings failed, %v", err)
	}
	select {
	case <-done2:
		return nil
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on listing update")
	}
}

// Do profile and listings migration with MATIC currencies update
func (n *OpenBazaarNode) migrateRepoFromVersion1() error {
	done1 := make(chan struct{})
	myProfile, err := n.GetMyProfile()
	if err != nil {
		return fmt.Errorf("get my profile failed, %v", err)
	}
	myProfile.Currencies = []string{"MATIC", "MATICUSDT", "MATICUSDC"}
	err = n.SetProfile(myProfile, done1)
	if err != nil {
		return fmt.Errorf("update profile failed, %v", err)
	}

	select {
	case <-done1:
		break
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on profile update")
	}

	done2 := make(chan struct{})
	err = n.UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		listing.Metadata.AcceptedCurrencies = []string{"MATIC", "MATICUSDT", "MATICUSDC"}
		return true, nil
	}, done2)
	if err != nil {
		return fmt.Errorf("update listings failed, %v", err)
	}

	select {
	case <-done2:
		return nil
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on listing update")
	}
}

// Do listings migration about signature due to new fields added
func (n *OpenBazaarNode) migrateRepoWithListingsUpdate() error {
	done := make(chan struct{})
	err := n.UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		// do nothing, just update the signature using existing flow
		return true, nil
	}, done)
	if err != nil {
		return fmt.Errorf("update listings failed, %v", err)
	}

	select {
	case <-done:
		return nil
	case <-time.After(time.Second * 300):
		return errors.New("timeout waiting on listing update")
	}
}

// Stop cleanly shutsdown the OpenBazaarNode and signals to any
// listening goroutines that it's time to stop.
func (n *OpenBazaarNode) Stop(force bool) error {
	if atomic.LoadInt32(&n.publishActive) > 0 && !force {
		return coreiface.ErrPublishingActive
	}

	if !n.ipfsOnlyMode {
		n.messenger.Stop()
		n.networkService.Close()
		n.orderProcessor.Stop()
		n.followerTracker.Close()
		n.multiwallet.Close()
		if n.IsDefaultNode() {
			n.SharedManager().Stop()
		}
		if n.notifier != nil {
			n.notifier.Stop()
		}
		for _, channel := range n.channels {
			channel.Close()
		}
	}
	if n.shutdownTorFunc != nil {
		n.shutdownTorFunc()
	}
	close(n.shutdown)
	n.repo.Close()

	stop := make(chan struct{})
	go func() {
		n.ipfsNode.Context().Done()
		n.ipfsNode.Close()
		time.AfterFunc(time.Second, func() {
			n.eventBus.Emit(&events.IPFSShutdown{})
		})
		close(stop)
	}()
	select {
	case <-time.After(time.Second * 2):
		return coreiface.ErrIPFSDelayedShutdown
	case <-stop:

	}
	return nil
}

// UsingTestnet returns whether or not this node is running on
// the test network.
func (n *OpenBazaarNode) UsingTestnet() bool {
	return n.testnet
}

// UsingTorMode returns whether or not this node is using the tor
// network exclusively. Dual stack returns false for this.
func (n *OpenBazaarNode) UsingTorMode() bool {
	return n.torOnly
}

// DestroyNode shutsdown the node and deletes the entire data directory.
// This should only be used during testing as destroying a live node will
// result in data loss.
func (n *OpenBazaarNode) DestroyNode() {
	n.Stop(true)
	n.repo.DestroyRepo()
}

// IPFSNode returns the underlying IPFS node instance.
func (n *OpenBazaarNode) IPFSNode() *core.IpfsNode {
	return n.ipfsNode
}

// Multiwallet returns the underlying Multiwallet instance.
func (n *OpenBazaarNode) Multiwallet() multiwallet.Multiwallet {
	return n.multiwallet
}

// DB returns the node's database.
func (n *OpenBazaarNode) DB() database.Database {
	return n.repo.DB()
}

// ExchangeRates returns the node's exchange rate provider.
func (n *OpenBazaarNode) ExchangeRates() *wallet.ExchangeRateProvider {
	return n.exchangeRates
}

// GetNodeID returns the user ID for this node.
func (n *OpenBazaarNode) GetNodeID() string {
	return n.nodeID
}

func (n *OpenBazaarNode) SharedManager() *SharedManager {
	return n.sharedManager
}

// Identity returns the peer ID for this node.
func (n *OpenBazaarNode) Identity() peer.ID {
	return n.ipfsNode.Identity
}

// SubscribeEvent returns a subscription to the provided event. The event argument
// may be an interface slice.
func (n *OpenBazaarNode) SubscribeEvent(event interface{}) (events.Subscription, error) {
	return n.eventBus.Subscribe(event)
}

// EventBus returns the node's event bus.
func (n *OpenBazaarNode) EventBus() events.Bus {
	return n.eventBus
}
