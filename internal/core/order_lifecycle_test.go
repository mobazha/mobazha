package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	adapters "github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/database/netdb"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/encoding/protojson"
)

// newMockUTXOAdapter creates a UTXOAutoConfirmAdapter wired to a MobazhaNode's
// callbacks. Used in tests to register ChainMock with the payment registry.
// Requires the node to have a fully initialized wallet (Mocknet nodes do).
func newMockUTXOAdapter(node *MobazhaNode) *adapters.UTXOAutoConfirmAdapter {
	return &adapters.UTXOAutoConfirmAdapter{
		Multiwallet:    node.multiwallet,
		Keys:           node.keyProvider,
		OnAutoConfirm:  node.handleCancelablePaymentForUTXO,
		GetPaymentInfo: node.Wallet().GetUTXOPaymentInfo,
	}
}

// newStubUTXOAdapter creates a minimal UTXOAutoConfirmAdapter for registry
// coverage tests that only verify chain registration and instruction dispatch,
// without requiring a live wallet or Multiwallet.
func newStubUTXOAdapter() *adapters.UTXOAutoConfirmAdapter {
	return &adapters.UTXOAutoConfirmAdapter{}
}

// setupMockNetDB creates a mock HTTP server that serves listing index data
// from the provided nodes' local databases, then creates and sets a NetDB
// instance on each node. This mirrors the production path (netDB → HTTP →
// mobazha.info) without any content-routing dependency.
//
// In production: node.netDB → HTTP GET /listingindex/{peerID} → mobazha.info API
// In test:       node.netDB → HTTP GET /listingindex/{peerID} → httptest.Server → node.GetMyListings()
func setupMockNetDB(t *testing.T, nodes []*MobazhaNode) {
	t.Helper()

	nodeMap := make(map[string]*MobazhaNode)
	for _, n := range nodes {
		nodeMap[n.peerID.String()] = n
	}

	var mu sync.Mutex
	listingStore := make(map[string]netdb.Listing) // CID → Listing

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")

		switch {
		case len(parts) == 2 && parts[0] == "listing-indexes" && r.Method == http.MethodGet:
			peerID := parts[1]
			node, ok := nodeMap[peerID]
			if !ok {
				http.Error(w, "peer not found", http.StatusNotFound)
				return
			}
			index, err := node.Listing().GetMyListings()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			serializedIndex, _ := json.Marshal(index)
			resp := netdb.ListingIndex{
				PeerID:          peerID,
				SerializedIndex: serializedIndex,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"data": resp})

		case len(parts) == 1 && parts[0] == "listing-indexes" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)

		case len(parts) == 1 && parts[0] == "listings" && r.Method == http.MethodPost:
			var listing netdb.Listing
			if err := json.NewDecoder(r.Body).Decode(&listing); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			listingStore[listing.CID] = listing
			mu.Unlock()
			w.WriteHeader(http.StatusOK)

		case len(parts) == 2 && parts[0] == "listings" && r.Method == http.MethodGet:
			cidStr := parts[1]
			mu.Lock()
			listing, ok := listingStore[cidStr]
			mu.Unlock()
			if !ok {
				// Fallback: async SetOwnListing may not have arrived yet;
				// search local nodes directly (simulates eventual consistency).
				for _, node := range nodeMap {
					index, err := node.Listing().GetMyListings()
					if err != nil {
						continue
					}
					for _, entry := range index {
						if entry.CID == cidStr {
							sl, err := node.Listing().GetMyListingBySlug(entry.Slug)
							if err != nil {
								continue
							}
							raw, err := protojson.Marshal(sl)
							if err != nil {
								continue
							}
							listing = netdb.Listing{CID: cidStr, SerializedListing: raw}
							ok = true
							break
						}
					}
					if ok {
						break
					}
				}
			}
			if !ok {
				http.Error(w, "listing not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"data": listing})

		case len(parts) == 2 && parts[0] == "profiles" && r.Method == http.MethodGet:
			peerID := parts[1]
			node, ok := nodeMap[peerID]
			if !ok {
				http.Error(w, "peer not found", http.StatusNotFound)
				return
			}
			var profile *models.Profile
			_ = node.repo.DB().View(func(tx database.Tx) error {
				p, err := tx.GetProfile()
				if err != nil {
					return err
				}
				profile = p
				return nil
			})
			if profile == nil {
				http.Error(w, "profile not found", http.StatusNotFound)
				return
			}
			serialized, _ := json.Marshal(profile)
			resp := netdb.Profile{PeerID: peerID, SerializedProfile: serialized}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"data": resp})

		case len(parts) == 1 && parts[0] == "profiles" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)

		case len(parts) == 2 && parts[0] == "rating-indexes" && r.Method == http.MethodGet:
			peerID := parts[1]
			node, ok := nodeMap[peerID]
			if !ok {
				http.Error(w, "peer not found", http.StatusNotFound)
				return
			}
			var index models.RatingIndex
			viewErr := node.repo.DB().View(func(tx database.Tx) error {
				idx, err := tx.GetRatingIndex()
				if err != nil {
					return err
				}
				index = idx
				return nil
			})
			if viewErr != nil {
				http.Error(w, viewErr.Error(), http.StatusInternalServerError)
				return
			}
			if index == nil {
				index = models.RatingIndex{}
			}
			serialized, _ := json.Marshal(index)
			resp := netdb.RatingIndex{PeerID: peerID, SerializedIndex: serialized}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"data": resp})

		case len(parts) == 1 && parts[0] == "rating-indexes" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	for _, node := range nodes {
		ndb, _ := netdb.NewNetDB(server.URL, node.peerID.String(), node.privKey)
		node.netDB = ndb
		// Patch netDB into existing service instances rather than re-creating
		// them, so cross-service references (e.g. OrderAppService.listings)
		// remain valid.
		if node.listingService != nil {
			node.listingService.netDB = ndb
		} else {
			node.initListingService()
		}
		if node.profileService != nil {
			node.profileService.netDB = ndb
		} else {
			node.initProfileService()
		}
		node.initRatingsService()
	}
}

// setupMockReceivingAccounts creates a mock ReceivingAccount for each node,
// using the MockWallet's current address as the receiving address. This is
// required because GetPayoutAddress relies on GetActiveReceivingAccount
// (no fallback to internal wallet). Other tests that exercise order
// completion, refund, or dispute flows should call this helper during setup.
func setupMockReceivingAccounts(t *testing.T, nodes []*MobazhaNode) {
	t.Helper()

	for _, node := range nodes {
		w, ok := node.Multiwallet().WalletForChain(iwallet.ChainMock)
		if !ok {
			t.Fatalf("setupMockReceivingAccounts: WalletForChain(ChainMock) not found")
		}
		mockWallet := w.(*wallet.MockWallet)
		addr, err := mockWallet.CurrentAddress()
		if err != nil {
			t.Fatalf("setupMockReceivingAccounts: CurrentAddress failed: %v", err)
		}
		account := &models.ReceivingAccount{
			Name:      "Mock Account",
			ChainType: iwallet.ChainMock,
			Address:   addr.String(),
			IsActive:  true,
		}
		_ = account.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
		err = node.repo.DB().Update(func(tx database.Tx) error {
			return tx.Save(account)
		})
		if err != nil {
			t.Fatalf("setupMockReceivingAccounts: Save failed: %v", err)
		}
	}
}

// ingestPaymentToWallets builds a transaction from paymentData and synchronously
// adds it to the specified nodes' mock wallets. This simulates blockchain broadcast
// so that GetTransaction succeeds on the vendor side (required for PaymentVerified).
// Uses AddTransaction (synchronous) instead of IngestTransaction (async channel)
// to avoid race conditions where GetTransaction is called before the async
// processing goroutine stores the transaction.
// Call this BEFORE ProcessOrderPayment.
func ingestPaymentToWallets(t *testing.T, paymentData *models.PaymentData, nodes ...*MobazhaNode) {
	t.Helper()

	if err := paymentData.EnsureTransactionFields(); err != nil {
		t.Fatalf("ingestPaymentToWallets: EnsureTransactionFields: %v", err)
	}
	tx, err := paymentData.BuildTransaction()
	if err != nil {
		t.Fatalf("ingestPaymentToWallets: BuildTransaction: %v", err)
	}
	tx.Height = 1
	for _, node := range nodes {
		w, ok := node.Multiwallet().WalletForChain(iwallet.ChainMock)
		if !ok {
			t.Fatalf("ingestPaymentToWallets: WalletForChain(ChainMock) not found")
		}
		if err := w.(*wallet.MockWallet).AddTransaction(tx.ID, tx); err != nil {
			t.Fatalf("ingestPaymentToWallets: AddTransaction: %v", err)
		}
	}
}

// ── Common Test Helpers ──────────────────────────────────────────────────
//
// These helpers reduce boilerplate across order lifecycle tests.
// T0 helpers — used by T1-T5+ tests.

// setupProfiles creates minimal profiles for all nodes via the Profile facade,
// which publishes profiles through P2P (required for MODERATED flows where
// other nodes need to fetch profile data like escrow keys).
func setupProfiles(t *testing.T, nodes []*MobazhaNode) {
	t.Helper()
	for _, node := range nodes {
		done := make(chan struct{})
		if err := node.Profile().SetProfile(&models.Profile{
			Name: "Test Store " + node.Identity().String()[:8],
		}, done); err != nil {
			t.Fatalf("setupProfiles: %v", err)
		}
		select {
		case <-done:
		case <-time.After(time.Second * 10):
			t.Fatalf("setupProfiles: timeout for %s", node.Identity().String()[:8])
		}
	}
}

// setupModeratorNode configures a node as a moderator.
func setupModeratorNode(t *testing.T, node *MobazhaNode, currencies []string) {
	t.Helper()
	modInfo := &models.ModeratorInfo{
		AcceptedCurrencies: currencies,
		Fee: models.ModeratorFee{
			Percentage: 10,
			FeeType:    models.PercentageFee,
		},
	}
	done := make(chan struct{})
	if err := node.Profile().SetSelfAsModerator(context.Background(), modInfo, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout setting moderator")
	}
}

// createListingAndPurchase creates a listing on seller, buyer purchases it,
// and waits for both nodes to have the order. Returns orderID and paymentAmount.
func createListingAndPurchase(t *testing.T, seller, buyer *MobazhaNode) (models.OrderID, models.CurrencyValue) {
	t.Helper()
	listing := factory.NewPhysicalListing("tshirt")
	done := make(chan struct{})
	if err := seller.Listing().SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout saving listing")
	}
	index, err := seller.Listing().GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	orderSub, err := seller.eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}
	orderAck, err := buyer.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID
	orderID, paymentAmount, err := buyer.Order().PurchaseListing(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-orderSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for NewOrder")
	}
	select {
	case <-orderAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for order ACK")
	}
	return orderID, paymentAmount
}

// waitForEvent waits for an event with a descriptive message on timeout.
func waitForEvent(t *testing.T, sub events.Subscription, eventName string) {
	t.Helper()
	select {
	case <-sub.Out():
	case <-time.After(time.Second * 10):
		t.Fatalf("Timeout waiting for %s", eventName)
	}
}

// waitForDone waits for a done channel with timeout.
func waitForDone(t *testing.T, done chan struct{}, opName string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatalf("Timeout waiting for %s", opName)
	}
}

// ── Registry-Driven Order Lifecycle Tests ────────────────────────────────
//
// These tests validate the full order lifecycle with chain escrow dispatch via Registry
// once the registry is initialized. They serve two purposes:
//
// 1. Verify the registry setup doesn't interfere with normal order flows
// 2. Validate the complete happy path: Purchase → Payment → Confirm → Ship → Complete
//
// Note on CANCELABLE path: The full CANCELABLE → auto-confirm flow requires
// GetUTXOPaymentInfo which calls CalculateOrderTotalInCurrency. This function
// has a known issue with MOCK coin (pre-existing in cancel_test.go too).
// The registry dispatch logic is tested separately in
// TestOrderLifecycle_RegistryCoversAllProductionChains which validates
// strategy resolution, model types, and instruction behaviors per chain.

// TestOrderLifecycle_RegistryDriven_FullHappyPath tests the complete happy path:
//
//	Purchase → DIRECT Payment → Seller Confirm → Ship → Complete
//
// This test initializes the payment registry on the seller node (matching
// production setup) and verifies the full lifecycle completes correctly.
func TestOrderLifecycle_RegistryDriven_FullHappyPath(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	go network.StartWalletNetwork()

	sellerNode := network.Nodes()[0]
	buyerNode := network.Nodes()[1]

	// ── Mock NetDB Setup ────────────────────────────────────────
	// Set up a mock HTTP server to serve listing index data. This mirrors
	// the production path where netDB
	// queries mobazha.info for listing data.
	setupMockNetDB(t, network.Nodes())

	// ── Profile Setup ───────────────────────────────────────────
	// CompleteOrder requires a buyer profile (for rating processing).
	for _, node := range network.Nodes() {
		err := node.repo.DB().Update(func(tx database.Tx) error {
			return tx.SetProfile(&models.Profile{
				PeerID: node.Identity().String(),
				Name:   "Test Store " + node.Identity().String()[:8],
			})
		})
		if err != nil {
			t.Fatalf("Failed to set profile: %v", err)
		}
	}

	// ── Registry Setup ──────────────────────────────────────────
	// Initialize the payment registry on the seller node (same as production).
	// This verifies that registry initialization doesn't interfere with
	// the normal order processing pipeline.
	sellerNode.registerPaymentStrategies()
	sellerNode.paymentRegistry.Register(iwallet.ChainMock, newMockUTXOAdapter(sellerNode))

	// Verify registry is populated correctly
	strategy, err := sellerNode.paymentRegistry.ForCoin(iwallet.CtMock)
	if err != nil {
		t.Fatalf("ChainMock not registered in payment registry: %v", err)
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		t.Fatalf("Expected PaymentModelMonitored for ChainMock, got %s", strategy.Model())
	}

	// Start the cancelable payment monitor (same as production)
	sellerNode.startCancelablePaymentMonitor()

	// Start order processors for message handling
	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	// ── Receiving Account Setup ──────────────────────────────────
	setupMockReceivingAccounts(t, network.Nodes())

	// ── Step 1: Seller Creates Listing ────────────────────────────
	listing := factory.NewPhysicalListing("tshirt")
	done := make(chan struct{})
	if err := sellerNode.Listing().SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout saving listing")
	}

	index, err := sellerNode.Listing().GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	// ── Step 2: Buyer Purchases ──────────────────────────────────
	orderSub, err := sellerNode.eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}
	orderAck, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	orderID, paymentAmount, err := buyerNode.Order().PurchaseListing(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-orderSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for NewOrder event on seller")
	}
	select {
	case <-orderAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for order ACK on buyer")
	}

	// Verify both nodes saved the order
	var sellerOrder models.Order
	err = sellerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&sellerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if sellerOrder.SerializedOrderOpen == nil {
		t.Error("Seller failed to save order open")
	}

	// ── Step 3: Buyer Sends DIRECT Payment ──────────────────────
	// Use DIRECT payment method (same pattern as confirm_test.go)
	paymentData := &models.PaymentData{
		OrderID:       orderID.String(),
		TransactionID: "lifecycle-direct-tx",
		Method:        pb.PaymentSent_DIRECT,
		Amount:        paymentAmount.Amount.Uint64(),
		Coin:          iwallet.CtMock,
		ToAddress:     "mock-payment-addr",
	}
	// Ingest into both wallets so vendor GetTransaction succeeds (PaymentVerified)
	ingestPaymentToWallets(t, paymentData, buyerNode, sellerNode)

	if err := buyerNode.Order().ProcessOrderPayment(context.Background(), paymentData); err != nil {
		t.Fatal(err)
	}

	// Wait for payment to propagate
	time.Sleep(100 * time.Millisecond)

	// ── Step 4: Seller Confirms Order ────────────────────────────
	confirmSub, err := buyerNode.eventBus.Subscribe(&events.OrderConfirmation{})
	if err != nil {
		t.Fatal(err)
	}
	confirmAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done4 := make(chan struct{})
	if err := sellerNode.Order().ConfirmOrder(orderID, "", "mock-payout-addr", done4); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done4:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for ConfirmOrder")
	}
	select {
	case <-confirmSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for OrderConfirmation on buyer")
	}
	select {
	case <-confirmAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for confirm ACK on seller")
	}

	// ── Step 5: Seller Ships ─────────────────────────────────
	shipSub, err := buyerNode.eventBus.Subscribe(&events.OrderShipment{})
	if err != nil {
		t.Fatal(err)
	}
	shipAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	shipments := []models.Shipment{
		{
			ItemIndex: 0,
			PhysicalDelivery: &models.PhysicalDelivery{
				TrackingNumber: "TRACK-001",
				Shipper:        "UPS",
			},
		},
	}
	if err := sellerNode.Order().ShipOrder(orderID, shipments, done5); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done5:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for ShipOrder")
	}
	select {
	case <-shipSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for OrderShipment event on buyer")
	}
	select {
	case <-shipAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for shipment ACK on seller")
	}

	// ── Step 6: Buyer Completes ─────────────────────────────────
	completeSub, err := sellerNode.eventBus.Subscribe(&events.OrderCompletion{})
	if err != nil {
		t.Fatal(err)
	}
	completeAck, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done6 := make(chan struct{})
	ratings := []models.Rating{
		{
			Overall: 5,
			Review:  "Great product — full lifecycle with registry works!",
		},
	}
	if err := buyerNode.Order().CompleteOrder(orderID, iwallet.TransactionID(""), ratings, true, done6); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done6:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for CompleteOrder")
	}
	select {
	case <-completeSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for OrderCompletion event on seller")
	}
	select {
	case <-completeAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for complete ACK on buyer")
	}

	// ── Verify Final State ──────────────────────────────────────
	var buyerFinalOrder models.Order
	err = buyerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&buyerFinalOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if buyerFinalOrder.SerializedOrderOpen == nil {
		t.Error("Missing OrderOpen on buyer")
	}
	if buyerFinalOrder.SerializedPaymentSent == nil {
		t.Error("Missing PaymentSent on buyer")
	}
	if buyerFinalOrder.SerializedOrderConfirmation == nil {
		t.Error("Missing OrderConfirmation on buyer")
	}
	if buyerFinalOrder.SerializedOrderShipments == nil {
		t.Error("Missing OrderShipments on buyer")
	}
	if buyerFinalOrder.SerializedOrderComplete == nil {
		t.Error("Missing OrderComplete on buyer")
	}

	// Verify ratings
	complete, err := buyerFinalOrder.OrderCompleteMessage()
	if err != nil {
		t.Fatal(err)
	}
	if complete.Ratings[0].Overall != 5 {
		t.Errorf("Expected overall rating 5, got %d", complete.Ratings[0].Overall)
	}

	// Verify seller's order state
	var sellerFinalOrder models.Order
	err = sellerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&sellerFinalOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if sellerFinalOrder.SerializedOrderConfirmation == nil {
		t.Error("Missing OrderConfirmation on seller")
	}
	if sellerFinalOrder.SerializedOrderShipments == nil {
		t.Error("Missing OrderShipments on seller")
	}

	t.Log("✓ Full happy path with registry completed: Purchase → Payment → Confirm → Ship → Complete")
}

// TestOrderLifecycle_Cancelable_AutoConfirm tests the CANCELABLE payment lifecycle
// which is the primary production path for UTXO chains:
//
//	Purchase → GetUTXOPaymentInfo → ProcessOrderPayment(CANCELABLE)
//	         → CancelablePaymentReady event → registry dispatch → AutoConfirm
//	         → Ship → Complete
//
// This exercises the entire payment registry dispatch chain that DIRECT payment
// bypasses, validating the core value of the payment architecture refactoring.
func TestOrderLifecycle_Cancelable_AutoConfirm(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	go network.StartWalletNetwork()

	sellerNode := network.Nodes()[0]
	buyerNode := network.Nodes()[1]

	// ── Mock NetDB Setup ────────────────────────────────────────
	setupMockNetDB(t, network.Nodes())

	// ── Profile Setup ───────────────────────────────────────────
	for _, node := range network.Nodes() {
		err := node.repo.DB().Update(func(tx database.Tx) error {
			return tx.SetProfile(&models.Profile{
				PeerID: node.Identity().String(),
				Name:   "Test Store " + node.Identity().String()[:8],
			})
		})
		if err != nil {
			t.Fatalf("Failed to set profile: %v", err)
		}
	}

	// ── Registry Setup ──────────────────────────────────────────
	// Initialize the payment registry and register ChainMock
	sellerNode.registerPaymentStrategies()
	sellerNode.paymentRegistry.Register(iwallet.ChainMock, newMockUTXOAdapter(sellerNode))

	// Verify registry is populated
	strategy, err := sellerNode.paymentRegistry.ForCoin(iwallet.CtMock)
	if err != nil {
		t.Fatalf("ChainMock not registered: %v", err)
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		t.Fatalf("Expected PaymentModelMonitored, got %s", strategy.Model())
	}

	// Start the cancelable payment monitor (key for auto-confirm)
	sellerNode.startCancelablePaymentMonitor()

	// Start the order event monitor so OrderAutoConfirmRequest is handled
	sellerNode.orderService.StartPaymentEventMonitor()

	// Start order processors for message handling
	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	// ── Receiving Account Setup ──────────────────────────────────
	setupMockReceivingAccounts(t, network.Nodes())

	// ── Step 1: Seller Creates Listing ────────────────────────────
	listing := factory.NewPhysicalListing("tshirt")
	done := make(chan struct{})
	if err := sellerNode.Listing().SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout saving listing")
	}

	index, err := sellerNode.Listing().GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	// ── Step 2: Buyer Purchases ──────────────────────────────────
	orderSub, err := sellerNode.eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}
	orderAck, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	orderID, _, err := buyerNode.Order().PurchaseListing(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-orderSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for NewOrder event on seller")
	}
	select {
	case <-orderAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for order ACK on buyer")
	}

	// Verify both nodes saved the order
	var sellerOrder models.Order
	err = sellerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&sellerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if sellerOrder.SerializedOrderOpen == nil {
		t.Error("Seller failed to save order open")
	}

	// ── Step 3: Buyer Gets CANCELABLE Payment Info ───────────────
	// GetUTXOPaymentInfo generates CANCELABLE escrow address (1-of-2 multisig)
	paymentData, err := buyerNode.Wallet().GetUTXOPaymentInfo(
		context.Background(),
		orderID.String(),
		"", // empty moderator = CANCELABLE
		iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo failed: %v", err)
	}
	t.Logf("CANCELABLE payment: amount=%d, address=%s", paymentData.Amount, paymentData.ToAddress)

	// Ingest tx into both wallets so vendor GetTransaction succeeds (PaymentVerified)
	ingestPaymentToWallets(t, paymentData, buyerNode, sellerNode)

	// ── Step 4: Buyer Sends CANCELABLE Payment ───────────────────
	// Subscribe to OrderConfirmation BEFORE processing payment,
	// because auto-confirm happens asynchronously after payment processing.
	confirmSub, err := buyerNode.eventBus.Subscribe(&events.OrderConfirmation{})
	if err != nil {
		t.Fatal(err)
	}

	if err := buyerNode.Order().ProcessOrderPayment(context.Background(), paymentData); err != nil {
		t.Fatal(err)
	}

	// ── Step 5: Wait for Auto-Confirm ────────────────────────────
	// The seller's payment monitor receives CancelablePaymentReady event,
	// dispatches to utxoAutoConfirmAdapter.AutoConfirm, which calls
	// ConfirmOrder → releaseFromCancelableAddress → sends OrderConfirmation.
	select {
	case <-confirmSub.Out():
		t.Log("Auto-confirm triggered: OrderConfirmation received on buyer")
	case <-time.After(time.Second * 15):
		t.Fatal("Timeout waiting for auto-confirm OrderConfirmation on buyer")
	}

	// ── Step 6: Seller Ships ─────────────────────────────────
	shipSub, err := buyerNode.eventBus.Subscribe(&events.OrderShipment{})
	if err != nil {
		t.Fatal(err)
	}
	shipAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	shipments := []models.Shipment{
		{
			ItemIndex: 0,
			PhysicalDelivery: &models.PhysicalDelivery{
				TrackingNumber: "TRACK-CANCELABLE-001",
				Shipper:        "FedEx",
			},
		},
	}
	if err := sellerNode.Order().ShipOrder(orderID, shipments, done5); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done5:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for ShipOrder")
	}
	select {
	case <-shipSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for OrderShipment event on buyer")
	}
	select {
	case <-shipAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for shipment ACK on seller")
	}

	// ── Step 7: Buyer Completes ─────────────────────────────────
	completeSub, err := sellerNode.eventBus.Subscribe(&events.OrderCompletion{})
	if err != nil {
		t.Fatal(err)
	}
	completeAck, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done6 := make(chan struct{})
	ratings := []models.Rating{
		{
			Overall: 5,
			Review:  "CANCELABLE auto-confirm lifecycle works perfectly!",
		},
	}
	if err := buyerNode.Order().CompleteOrder(orderID, iwallet.TransactionID(""), ratings, true, done6); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done6:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for CompleteOrder")
	}
	select {
	case <-completeSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for OrderCompletion event on seller")
	}
	select {
	case <-completeAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for complete ACK on buyer")
	}

	// ── Verify Final State ──────────────────────────────────────
	var buyerFinalOrder models.Order
	err = buyerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&buyerFinalOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if buyerFinalOrder.SerializedOrderOpen == nil {
		t.Error("Missing OrderOpen on buyer")
	}
	if buyerFinalOrder.SerializedPaymentSent == nil {
		t.Error("Missing PaymentSent on buyer")
	}
	if buyerFinalOrder.SerializedOrderConfirmation == nil {
		t.Error("Missing OrderConfirmation on buyer (auto-confirm failed?)")
	}
	if buyerFinalOrder.SerializedOrderShipments == nil {
		t.Error("Missing OrderShipments on buyer")
	}
	if buyerFinalOrder.SerializedOrderComplete == nil {
		t.Error("Missing OrderComplete on buyer")
	}

	// Verify the payment was CANCELABLE
	paymentSentMsg, err := buyerFinalOrder.PaymentSentMessage()
	if err != nil {
		t.Fatal(err)
	}
	if paymentSentMsg.Method != pb.PaymentSent_CANCELABLE {
		t.Errorf("Expected CANCELABLE payment method, got %s", paymentSentMsg.Method)
	}

	// Verify seller's order state
	var sellerFinalOrder models.Order
	err = sellerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&sellerFinalOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if sellerFinalOrder.SerializedOrderConfirmation == nil {
		t.Error("Missing OrderConfirmation on seller")
	}
	if sellerFinalOrder.SerializedOrderShipments == nil {
		t.Error("Missing OrderShipments on seller")
	}

	t.Log("CANCELABLE auto-confirm lifecycle completed: Purchase -> CANCELABLE Payment -> AutoConfirm -> Ship -> Complete")
}

// TestOrderLifecycle_Cancelable_BuyerCancel tests the buyer cancel path:
//
//	Purchase → GetUTXOPaymentInfo → ProcessOrderPayment(CANCELABLE)
//	         → Buyer CancelOrder → funds released back to buyer
//
// This verifies the reverse flow where a buyer cancels before seller confirms.
// The cancelable payment monitor is intentionally NOT started so that auto-confirm
// does not race with the cancel operation.
func TestOrderLifecycle_Cancelable_BuyerCancel(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	go network.StartWalletNetwork()

	sellerNode := network.Nodes()[0]
	buyerNode := network.Nodes()[1]

	// ── Mock NetDB Setup ────────────────────────────────────────
	setupMockNetDB(t, network.Nodes())

	// ── Profile Setup ───────────────────────────────────────────
	for _, node := range network.Nodes() {
		err := node.repo.DB().Update(func(tx database.Tx) error {
			return tx.SetProfile(&models.Profile{
				PeerID: node.Identity().String(),
				Name:   "Test Store " + node.Identity().String()[:8],
			})
		})
		if err != nil {
			t.Fatalf("Failed to set profile: %v", err)
		}
	}

	// NOTE: Intentionally NOT starting cancelable payment monitor.
	// We want the buyer to cancel before any auto-confirm fires.

	// Start order processors for message handling
	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	// ── Receiving Account Setup ──────────────────────────────────
	setupMockReceivingAccounts(t, network.Nodes())

	// ── Step 1: Seller Creates Listing ────────────────────────────
	listing := factory.NewPhysicalListing("tshirt")
	done := make(chan struct{})
	if err := sellerNode.Listing().SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout saving listing")
	}

	index, err := sellerNode.Listing().GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	// ── Step 2: Buyer Purchases ──────────────────────────────────
	orderSub, err := sellerNode.eventBus.Subscribe(&events.NewOrder{})
	if err != nil {
		t.Fatal(err)
	}
	orderAck, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	purchase := factory.NewPurchase()
	purchase.Items[0].ListingHash = index[0].CID

	orderID, _, err := buyerNode.Order().PurchaseListing(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-orderSub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for NewOrder event on seller")
	}
	select {
	case <-orderAck.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for order ACK on buyer")
	}

	// ── Step 3: Buyer Gets CANCELABLE Payment Info ───────────────
	paymentData, err := buyerNode.Wallet().GetUTXOPaymentInfo(
		context.Background(),
		orderID.String(),
		"", // empty moderator = CANCELABLE
		iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo failed: %v", err)
	}
	t.Logf("CANCELABLE payment: amount=%d, address=%s", paymentData.Amount, paymentData.ToAddress)

	// Ensure TransactionID/FromID are populated before building
	if err := paymentData.EnsureTransactionFields(); err != nil {
		t.Fatalf("EnsureTransactionFields failed: %v", err)
	}

	// Ingest transaction into buyer's wallet so CancelOrder can release funds
	tx, err := paymentData.BuildTransaction()
	if err != nil {
		t.Fatalf("BuildTransaction failed: %v", err)
	}
	bw, _ := buyerNode.Multiwallet().WalletForChain(iwallet.ChainMock)
	buyerWal := bw.(*wallet.MockWallet)
	buyerWal.IngestTransaction(tx)

	// ── Step 4: Buyer Sends CANCELABLE Payment ───────────────────
	if err := buyerNode.Order().ProcessOrderPayment(context.Background(), paymentData); err != nil {
		t.Fatal(err)
	}

	// Give time for payment message to propagate to seller
	time.Sleep(500 * time.Millisecond)

	// Verify payment was recorded
	var buyerOrder models.Order
	err = buyerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&buyerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if buyerOrder.SerializedPaymentSent == nil {
		t.Fatal("Buyer failed to save PaymentSent")
	}

	paymentSentMsg, err := buyerOrder.PaymentSentMessage()
	if err != nil {
		t.Fatal(err)
	}
	if paymentSentMsg.Method != pb.PaymentSent_CANCELABLE {
		t.Fatalf("Expected CANCELABLE payment, got %s", paymentSentMsg.Method)
	}

	// ── Step 5: Buyer Cancels Order ──────────────────────────────
	doneCh := make(chan struct{})
	if err := buyerNode.Order().CancelOrder(orderID, "", doneCh); err != nil {
		t.Fatal(err)
	}
	select {
	case <-doneCh:
		t.Log("CancelOrder completed successfully")
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for CancelOrder")
	}

	// ── Verify Cancel State ─────────────────────────────────────
	var buyerFinalOrder models.Order
	err = buyerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&buyerFinalOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if buyerFinalOrder.SerializedOrderCancel == nil {
		t.Error("Buyer node failed to save OrderCancel")
	}

	// Verify transaction was recorded
	txs, err := buyerFinalOrder.GetTransactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) < 1 {
		t.Errorf("Expected at least 1 transaction, got %d", len(txs))
	}

	// Check seller received the cancel message (retry with polling)
	var sellerFinalOrder models.Order
	cancelReceived := false
	for i := 0; i < 15; i++ {
		time.Sleep(500 * time.Millisecond)
		err = sellerNode.repo.DB().View(func(tx database.Tx) error {
			return tx.Read().Where("id = ?", orderID.String()).Last(&sellerFinalOrder).Error
		})
		if err != nil {
			t.Fatal(err)
		}
		if sellerFinalOrder.SerializedOrderCancel != nil {
			cancelReceived = true
			t.Log("Seller node received OrderCancel message")
			break
		}
	}
	if !cancelReceived {
		t.Error("Seller did not receive OrderCancel message within timeout")
	}

	t.Log("CANCELABLE buyer cancel lifecycle completed: Purchase -> CANCELABLE Payment -> BuyerCancel -> funds released")
}

// TestOrderLifecycle_RegistryCoversAllProductionChains verifies that the
// payment registry covers all production chains and that ChainMock integrates
// correctly alongside them.
func TestOrderLifecycle_RegistryCoversAllProductionChains(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-lifecycle-registry"}}
	n.registerPaymentStrategies()

	// Register ChainMock with a stub adapter (no wallet needed for registry coverage)
	n.paymentRegistry.Register(iwallet.ChainMock, newStubUTXOAdapter())

	// Verify all expected chains are registered
	chains := n.paymentRegistry.Chains()
	expectedChains := []iwallet.ChainType{
		iwallet.ChainBitcoin, iwallet.ChainBitcoinCash, iwallet.ChainLitecoin, iwallet.ChainZCash,
		iwallet.ChainBSC, iwallet.ChainEthereum, iwallet.ChainPolygon, iwallet.ChainBase,
		iwallet.ChainSolana, iwallet.ChainMock,
	}
	chainSet := make(map[iwallet.ChainType]bool)
	for _, c := range chains {
		chainSet[c] = true
	}
	for _, expected := range expectedChains {
		if !chainSet[expected] {
			t.Errorf("Expected chain %s to be registered, but it was not", expected)
		}
	}

	// Verify ChainMock resolves correctly
	strategy, err := n.paymentRegistry.ForCoin(iwallet.CtMock)
	if err != nil {
		t.Fatalf("ForCoin(MCK) failed: %v", err)
	}
	if strategy.Model() != payment.PaymentModelMonitored {
		t.Errorf("ChainMock model = %s, want %s", strategy.Model(), payment.PaymentModelMonitored)
	}

	// Verify instruction methods return nil for UTXO (backend-handled)
	result, err := strategy.GetConfirmInstructions(context.Background(), payment.InstructionParams{})
	if err != nil {
		t.Fatalf("GetConfirmInstructions failed: %v", err)
	}
	if result.Instructions != nil {
		t.Error("UTXO GetConfirmInstructions should return nil Instructions (backend-handled)")
	}

	result, err = strategy.GetCancelInstructions(context.Background(), payment.InstructionParams{})
	if err != nil {
		t.Fatalf("GetCancelInstructions failed: %v", err)
	}
	if result.Instructions != nil {
		t.Error("UTXO GetCancelInstructions should return nil Instructions")
	}

	result, err = strategy.GetCompleteInstructions(context.Background(), payment.InstructionParams{})
	if err != nil {
		t.Fatalf("GetCompleteInstructions failed: %v", err)
	}
	if result.Instructions != nil {
		t.Error("UTXO GetCompleteInstructions should return nil Instructions")
	}

	result, err = strategy.GetDisputeReleaseInstructions(context.Background(), payment.InstructionParams{})
	if err != nil {
		t.Fatalf("GetDisputeReleaseInstructions failed: %v", err)
	}
	if result.Instructions != nil {
		t.Error("UTXO GetDisputeReleaseInstructions should return nil Instructions")
	}

	// ── Verify EVM chains use PaymentModelClientSigned ──────────
	testEvmChains := []iwallet.ChainType{
		iwallet.ChainBSC, iwallet.ChainEthereum, iwallet.ChainPolygon,
		iwallet.ChainBase,
	}
	for _, chain := range testEvmChains {
		evmStrategy, err := n.paymentRegistry.ForChain(chain)
		if err != nil {
			t.Errorf("ForChain(%s) failed for EVM chain: %v", chain, err)
			continue
		}
		if evmStrategy.Model() != payment.PaymentModelClientSigned {
			t.Errorf("EVM chain %s: model = %s, want %s",
				chain, evmStrategy.Model(), payment.PaymentModelClientSigned)
		}
	}
	t.Log("✓ EVM chains (BSC/ETH/MATIC/BASE) use PaymentModelClientSigned")

	// ── Verify Solana uses PaymentModelClientSigned ──────────────
	solStrategy, err := n.paymentRegistry.ForChain(iwallet.ChainSolana)
	if err != nil {
		t.Fatalf("ForChain(SOL) failed: %v", err)
	}
	if solStrategy.Model() != payment.PaymentModelClientSigned {
		t.Errorf("Solana: model = %s, want %s",
			solStrategy.Model(), payment.PaymentModelClientSigned)
	}
	t.Log("✓ Solana uses PaymentModelClientSigned")

	// ── Verify UTXO chains use PaymentModelMonitored ────────────
	testUtxoChains := []iwallet.ChainType{
		iwallet.ChainBitcoin, iwallet.ChainBitcoinCash,
		iwallet.ChainLitecoin, iwallet.ChainZCash,
	}
	for _, chain := range testUtxoChains {
		utxoStrat, err := n.paymentRegistry.ForChain(chain)
		if err != nil {
			t.Errorf("ForChain(%s) failed for UTXO chain: %v", chain, err)
			continue
		}
		if utxoStrat.Model() != payment.PaymentModelMonitored {
			t.Errorf("UTXO chain %s: model = %s, want %s",
				chain, utxoStrat.Model(), payment.PaymentModelMonitored)
		}
	}
	t.Log("✓ UTXO chains (BTC/BCH/LTC/ZEC) use PaymentModelMonitored")

	// ── Summary: Model semantics table ──────────────────────────
	t.Log("Model semantics verified:")
	t.Log("  UTXO (BTC/BCH/LTC/ZEC/Mock): Monitored — backend auto-confirms, instructions=nil")
	t.Log("  EVM (BSC/ETH/MATIC/BASE): ClientSigned — frontend signs, instructions!=nil")
	t.Log("  Solana (SOL): ClientSigned — frontend signs, instructions!=nil")
}

// TestOrderLifecycle_Moderated_FullHappyPath tests the full moderated order lifecycle:
//
//	Purchase → MODERATED Payment → Seller Confirm → Ship → Complete
//
// This is the 3-node escrow path: Seller (node[0]), Buyer (node[1]), Moderator (node[2]).
// Uses 2-of-3 multisig escrow so that either buyer+seller or moderator+one-party
// can release funds. This test follows the happy path where no dispute is needed.
func TestOrderLifecycle_Moderated_FullHappyPath(t *testing.T) {
	network, err := NewMocknet(3)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	go network.StartWalletNetwork()

	sellerNode := network.Nodes()[0]
	buyerNode := network.Nodes()[1]
	moderatorNode := network.Nodes()[2]

	// ── Setup ───────────────────────────────────────────────────
	setupMockNetDB(t, network.Nodes())
	setupMockReceivingAccounts(t, network.Nodes())
	setupProfiles(t, network.Nodes())
	setupModeratorNode(t, moderatorNode, []string{"MCK"})

	sellerNode.registerPaymentStrategies()
	sellerNode.paymentRegistry.Register(iwallet.ChainMock, newMockUTXOAdapter(sellerNode))

	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	// ── Step 1+2: Create Listing & Buyer Purchases ──────────────
	orderID, _ := createListingAndPurchase(t, sellerNode, buyerNode)

	// ── Step 3: Buyer Gets MODERATED Payment Info ───────────────
	moderatorPeerID := moderatorNode.Identity().String()
	paymentData, err := buyerNode.Wallet().GetUTXOPaymentInfo(
		context.Background(),
		orderID.String(),
		moderatorPeerID,
		iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo (MODERATED) failed: %v", err)
	}
	t.Logf("MODERATED payment: amount=%d, address=%s", paymentData.Amount, paymentData.ToAddress)

	// ── Step 4: Ingest Payment & Process ────────────────────────
	fundingSub, err := sellerNode.eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}
	paymentRecvSub, err := buyerNode.eventBus.Subscribe(&events.OrderPaymentReceived{})
	if err != nil {
		t.Fatal(err)
	}
	ratingSigAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	ingestPaymentToWallets(t, paymentData, sellerNode, buyerNode)
	if err := buyerNode.Order().ProcessOrderPayment(context.Background(), paymentData); err != nil {
		t.Fatal(err)
	}

	waitForEvent(t, fundingSub, "OrderFunded on seller")
	waitForEvent(t, paymentRecvSub, "OrderPaymentReceived on buyer")
	waitForEvent(t, ratingSigAck, "MessageACK (rating sig) on seller")

	// ── Step 5: Seller Confirms Order ───────────────────────────
	confirmSub, err := buyerNode.eventBus.Subscribe(&events.OrderConfirmation{})
	if err != nil {
		t.Fatal(err)
	}
	confirmAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	if err := sellerNode.Order().ConfirmOrder(orderID, "", "mock-payout-addr", done5); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done5, "ConfirmOrder")
	waitForEvent(t, confirmSub, "OrderConfirmation on buyer")
	waitForEvent(t, confirmAck, "MessageACK (confirm) on seller")

	// ── Step 6: Seller Ships ─────────────────────────────────
	shipSub, err := buyerNode.eventBus.Subscribe(&events.OrderShipment{})
	if err != nil {
		t.Fatal(err)
	}
	shipAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done6 := make(chan struct{})
	shipments := []models.Shipment{
		{
			ItemIndex: 0,
			PhysicalDelivery: &models.PhysicalDelivery{
				TrackingNumber: "TRACK-MOD-001",
				Shipper:        "DHL",
			},
		},
	}
	if err := sellerNode.Order().ShipOrder(orderID, shipments, done6); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done6, "ShipOrder")
	waitForEvent(t, shipSub, "OrderShipment on buyer")
	waitForEvent(t, shipAck, "MessageACK (shipment) on seller")

	// ── Step 7: Buyer Completes ─────────────────────────────────
	completeSub, err := sellerNode.eventBus.Subscribe(&events.OrderCompletion{})
	if err != nil {
		t.Fatal(err)
	}
	completeAck, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done7 := make(chan struct{})
	ratings := []models.Rating{
		{
			Overall: 5,
			Review:  "Moderated order completed perfectly!",
		},
	}
	if err := buyerNode.Order().CompleteOrder(orderID, iwallet.TransactionID(""), ratings, true, done7); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done7, "CompleteOrder")
	waitForEvent(t, completeSub, "OrderCompletion on seller")
	waitForEvent(t, completeAck, "MessageACK (complete) on buyer")

	// ── Verify Final State ──────────────────────────────────────
	var buyerOrder models.Order
	err = buyerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&buyerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if buyerOrder.SerializedOrderOpen == nil {
		t.Error("Missing OrderOpen on buyer")
	}
	if buyerOrder.SerializedPaymentSent == nil {
		t.Error("Missing PaymentSent on buyer")
	}
	if buyerOrder.SerializedOrderConfirmation == nil {
		t.Error("Missing OrderConfirmation on buyer")
	}
	if buyerOrder.SerializedOrderShipments == nil {
		t.Error("Missing OrderShipments on buyer")
	}
	if buyerOrder.SerializedOrderComplete == nil {
		t.Error("Missing OrderComplete on buyer")
	}

	// Verify MODERATED payment method and moderator field
	paymentSentMsg, err := buyerOrder.PaymentSentMessage()
	if err != nil {
		t.Fatal(err)
	}
	if paymentSentMsg.Method != pb.PaymentSent_MODERATED {
		t.Errorf("Expected MODERATED payment method, got %s", paymentSentMsg.Method)
	}
	if paymentSentMsg.Moderator != moderatorPeerID {
		t.Errorf("Expected moderator %s, got %s", moderatorPeerID, paymentSentMsg.Moderator)
	}

	// Verify ratings
	complete, err := buyerOrder.OrderCompleteMessage()
	if err != nil {
		t.Fatal(err)
	}
	if complete.Ratings[0].Overall != 5 {
		t.Errorf("Expected overall rating 5, got %d", complete.Ratings[0].Overall)
	}

	// Verify seller's order state
	var sellerOrder models.Order
	err = sellerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&sellerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if sellerOrder.SerializedOrderOpen == nil {
		t.Error("Missing OrderOpen on seller")
	}
	if sellerOrder.SerializedOrderConfirmation == nil {
		t.Error("Missing OrderConfirmation on seller")
	}
	if sellerOrder.SerializedOrderShipments == nil {
		t.Error("Missing OrderShipments on seller")
	}

	t.Log("Moderated happy path completed: Purchase -> MODERATED Payment -> Confirm -> Ship -> Complete")
}

// TestOrderLifecycle_Moderated_Dispute_FullResolution tests the 3-node dispute flow:
//
//	Purchase → MODERATED Payment → Confirm → Buyer Disputes → Moderator Resolves (60/40) → Buyer Releases
func TestOrderLifecycle_Moderated_Dispute_FullResolution(t *testing.T) {
	network, err := NewMocknet(3)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	go network.StartWalletNetwork()

	sellerNode := network.Nodes()[0]
	buyerNode := network.Nodes()[1]
	moderatorNode := network.Nodes()[2]

	setupMockNetDB(t, network.Nodes())
	setupMockReceivingAccounts(t, network.Nodes())
	setupProfiles(t, network.Nodes())
	setupModeratorNode(t, moderatorNode, []string{"MCK"})

	sellerNode.registerPaymentStrategies()
	sellerNode.paymentRegistry.Register(iwallet.ChainMock, newMockUTXOAdapter(sellerNode))

	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	// ── Steps 1-2: Purchase ─────────────────────────────────────
	orderID, _ := createListingAndPurchase(t, sellerNode, buyerNode)

	// ── Step 3: MODERATED Payment ───────────────────────────────
	moderatorPeerID := moderatorNode.Identity().String()
	paymentData, err := buyerNode.Wallet().GetUTXOPaymentInfo(
		context.Background(), orderID.String(), moderatorPeerID, iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo (MODERATED) failed: %v", err)
	}

	fundingSub, err := sellerNode.eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}
	paymentRecvSub, err := buyerNode.eventBus.Subscribe(&events.OrderPaymentReceived{})
	if err != nil {
		t.Fatal(err)
	}
	ratingSigAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	ingestPaymentToWallets(t, paymentData, sellerNode, buyerNode)
	if err := buyerNode.Order().ProcessOrderPayment(context.Background(), paymentData); err != nil {
		t.Fatal(err)
	}

	waitForEvent(t, fundingSub, "OrderFunded on seller")
	waitForEvent(t, paymentRecvSub, "OrderPaymentReceived on buyer")
	waitForEvent(t, ratingSigAck, "MessageACK (rating sig) on seller")

	// ── Step 4: Seller Confirms ─────────────────────────────────
	confirmSub, err := buyerNode.eventBus.Subscribe(&events.OrderConfirmation{})
	if err != nil {
		t.Fatal(err)
	}
	confirmAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done4 := make(chan struct{})
	if err := sellerNode.Order().ConfirmOrder(orderID, "", "mock-payout-addr", done4); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done4, "ConfirmOrder")
	waitForEvent(t, confirmSub, "OrderConfirmation on buyer")
	waitForEvent(t, confirmAck, "MessageACK (confirm) on seller")

	// ── Step 5: Buyer Opens Dispute ─────────────────────────────
	caseOpenSub, err := moderatorNode.eventBus.Subscribe(&events.CaseOpen{})
	if err != nil {
		t.Fatal(err)
	}
	caseUpdateSub, err := moderatorNode.eventBus.Subscribe(&events.CaseUpdate{})
	if err != nil {
		t.Fatal(err)
	}
	caseUpdateAckSub, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}
	disputeOpenSub, err := sellerNode.eventBus.Subscribe(&events.DisputeOpen{})
	if err != nil {
		t.Fatal(err)
	}
	disputeOpenAckMod, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}
	disputeOpenAckVendor, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done5 := make(chan struct{})
	if err := buyerNode.Order().OpenDispute(orderID, "Item not as described", nil, done5); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done5, "OpenDispute")
	waitForEvent(t, disputeOpenSub, "DisputeOpen on seller")
	waitForEvent(t, caseOpenSub, "CaseOpen on moderator")
	waitForEvent(t, caseUpdateSub, "CaseUpdate on moderator")
	waitForEvent(t, disputeOpenAckMod, "MessageACK (dispute→mod)")
	waitForEvent(t, disputeOpenAckVendor, "MessageACK (dispute→vendor)")
	waitForEvent(t, caseUpdateAckSub, "MessageACK (caseUpdate→seller)")

	// Verify dispute state on all 3 nodes
	var buyerOrder models.Order
	err = buyerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&buyerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if buyerOrder.SerializedDisputeOpen == nil {
		t.Error("Buyer dispute open is nil")
	}

	var sellerOrder models.Order
	err = sellerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&sellerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if sellerOrder.SerializedDisputeOpen == nil {
		t.Error("Seller dispute open is nil")
	}
	if sellerOrder.SerializedDisputeUpdate == nil {
		t.Error("Seller dispute update is nil")
	}

	var modCase models.Case
	err = moderatorNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&modCase).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if modCase.SerializedDisputeOpen == nil {
		t.Error("Moderator dispute open is nil")
	}
	if modCase.SerializedBuyerContract == nil {
		t.Error("Moderator buyer contract is nil")
	}
	if modCase.SerializedVendorContract == nil {
		t.Error("Moderator vendor contract is nil")
	}

	// ── Step 6: Moderator Closes Dispute (60/40) ────────────────
	disputeCloseBuyer, err := buyerNode.eventBus.Subscribe(&events.DisputeClose{})
	if err != nil {
		t.Fatal(err)
	}
	disputeCloseSeller, err := sellerNode.eventBus.Subscribe(&events.DisputeClose{})
	if err != nil {
		t.Fatal(err)
	}
	disputeCloseAck, err := moderatorNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done6 := make(chan struct{})
	if err := moderatorNode.Order().CloseDispute(orderID, 60, 40, "Buyer gets 60%, seller gets 40%", done6); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done6, "CloseDispute")
	waitForEvent(t, disputeCloseBuyer, "DisputeClose on buyer")
	waitForEvent(t, disputeCloseSeller, "DisputeClose on seller")
	waitForEvent(t, disputeCloseAck, "MessageACK (disputeClose) on moderator")

	// Verify dispute resolution data
	err = sellerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&sellerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	disputeClose, err := sellerOrder.DisputeClosedMessage()
	if err != nil {
		t.Fatal("Failed to get DisputeClosedMessage")
	}
	if len(disputeClose.ReleaseInfo.Outpoints) == 0 {
		t.Error("No outpoint in release info")
	}
	if len(disputeClose.ReleaseInfo.EscrowSignatures) == 0 {
		t.Error("No moderator signature in release info")
	}

	// ── Step 7: Buyer Releases Funds ────────────────────────────
	disputeAcceptSub, err := sellerNode.eventBus.Subscribe(&events.DisputeAccepted{})
	if err != nil {
		t.Fatal(err)
	}
	disputeAcceptAck, err := buyerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done7 := make(chan struct{})
	if err := buyerNode.Order().ReleaseFunds(orderID, iwallet.TransactionID(""), done7); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done7, "ReleaseFunds")
	waitForEvent(t, disputeAcceptSub, "DisputeAccepted on seller")
	waitForEvent(t, disputeAcceptAck, "MessageACK (release) on buyer")

	network.WalletNetwork().GenerateBlock()

	releaseInfo := disputeClose.ReleaseInfo
	if releaseInfo.VendorAmount == "" || releaseInfo.VendorAmount == "0" {
		t.Error("Expected non-zero vendor amount")
	}
	if releaseInfo.BuyerAmount == "" || releaseInfo.BuyerAmount == "0" {
		t.Error("Expected non-zero buyer amount")
	}
	if releaseInfo.ModeratorAmount == "" || releaseInfo.ModeratorAmount == "0" {
		t.Error("Expected non-zero moderator amount")
	}

	buyerAmt := iwallet.NewAmount(releaseInfo.BuyerAmount)
	vendorAmt := iwallet.NewAmount(releaseInfo.VendorAmount)
	modAmt := iwallet.NewAmount(releaseInfo.ModeratorAmount)
	fee := iwallet.NewAmount(releaseInfo.TransactionFee)
	totalDistributed := buyerAmt.Add(vendorAmt).Add(modAmt).Add(fee)
	if totalDistributed.Cmp(iwallet.NewAmount(0)) <= 0 {
		t.Error("Total distributed amount should be positive")
	}

	t.Logf("Dispute resolution (60/40): buyer=%s, vendor=%s, moderator=%s, fee=%s",
		releaseInfo.BuyerAmount, releaseInfo.VendorAmount, releaseInfo.ModeratorAmount, releaseInfo.TransactionFee)
	t.Log("Moderated dispute completed: Purchase -> Payment -> Confirm -> Dispute -> Resolve(60/40) -> Release")
}

// TestOrderLifecycle_SellerDecline_AfterCancelablePayment tests seller declining
// a funded CANCELABLE order, which should trigger an automatic refund.
func TestOrderLifecycle_SellerDecline_AfterCancelablePayment(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	go network.StartWalletNetwork()

	sellerNode := network.Nodes()[0]
	buyerNode := network.Nodes()[1]

	setupMockNetDB(t, network.Nodes())
	setupMockReceivingAccounts(t, network.Nodes())

	sellerNode.registerPaymentStrategies()
	sellerNode.paymentRegistry.Register(iwallet.ChainMock, newMockUTXOAdapter(sellerNode))

	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	// ── Step 1: Purchase ────────────────────────────────────────
	orderID, _ := createListingAndPurchase(t, sellerNode, buyerNode)

	// ── Step 2: CANCELABLE Payment ──────────────────────────────
	paymentData, err := buyerNode.Wallet().GetUTXOPaymentInfo(
		context.Background(), orderID.String(), "", iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo (CANCELABLE) failed: %v", err)
	}

	buyerWallet, err := buyerNode.multiwallet.WalletForCurrencyCode(iwallet.CtMock.String())
	if err != nil {
		t.Fatal(err)
	}
	refundAddr, err := buyerWallet.(*wallet.MockWallet).CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}
	paymentData.PayerAddress = refundAddr.String()

	fundingSub, err := sellerNode.eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}
	paymentRecvSub, err := buyerNode.eventBus.Subscribe(&events.OrderPaymentReceived{})
	if err != nil {
		t.Fatal(err)
	}

	ingestPaymentToWallets(t, paymentData, sellerNode, buyerNode)
	if err := buyerNode.Order().ProcessOrderPayment(context.Background(), paymentData); err != nil {
		t.Fatal(err)
	}

	waitForEvent(t, fundingSub, "OrderFunded on seller")
	waitForEvent(t, paymentRecvSub, "OrderPaymentReceived on buyer")

	// ── Step 3: Seller Declines (funded — buyer releases CANCELABLE escrow inline) ──
	declineSub, err := buyerNode.eventBus.Subscribe(&events.OrderDeclined{})
	if err != nil {
		t.Fatal(err)
	}
	declineAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	if err := sellerNode.Order().DeclineOrder(orderID, iwallet.TransactionID(""), "Damaged item", done); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done, "DeclineOrder")
	waitForEvent(t, declineSub, "OrderDeclined on buyer")
	waitForEvent(t, declineAck, "MessageACK (decline) on seller")

	// ── Verify ──────────────────────────────────────────────────
	var buyerOrder models.Order
	err = buyerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&buyerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if buyerOrder.SerializedOrderDecline == nil {
		t.Error("Buyer failed to save order decline")
	}
	if buyerOrder.Open {
		t.Error("Order should be closed after decline")
	}

	t.Log("Seller decline (CANCELABLE funded) completed: Purchase -> Payment -> Decline")
}

// TestOrderLifecycle_CancelableConfirm_RefundBlocked verifies that a confirmed
// CANCELABLE order cannot be automatically refunded, since funds have already
// been released from the escrow to the seller's wallet.
func TestOrderLifecycle_CancelableConfirm_RefundBlocked(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer network.TearDown()

	go network.StartWalletNetwork()

	sellerNode := network.Nodes()[0]
	buyerNode := network.Nodes()[1]

	setupMockNetDB(t, network.Nodes())
	setupMockReceivingAccounts(t, network.Nodes())

	sellerNode.registerPaymentStrategies()
	sellerNode.paymentRegistry.Register(iwallet.ChainMock, newMockUTXOAdapter(sellerNode))

	for _, node := range network.Nodes() {
		go node.orderProcessor.Start()
	}

	// ── Steps 1-2: Purchase ─────────────────────────────────────
	orderID, _ := createListingAndPurchase(t, sellerNode, buyerNode)

	// ── Step 3: CANCELABLE Payment ──────────────────────────────
	paymentData, err := buyerNode.Wallet().GetUTXOPaymentInfo(
		context.Background(), orderID.String(), "", iwallet.CtMock,
	)
	if err != nil {
		t.Fatalf("GetUTXOPaymentInfo (CANCELABLE) failed: %v", err)
	}

	buyerWallet, err := buyerNode.multiwallet.WalletForCurrencyCode(iwallet.CtMock.String())
	if err != nil {
		t.Fatal(err)
	}
	refundAddr, err := buyerWallet.(*wallet.MockWallet).CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}
	paymentData.PayerAddress = refundAddr.String()

	fundingSub, err := sellerNode.eventBus.Subscribe(&events.OrderFunded{})
	if err != nil {
		t.Fatal(err)
	}
	paymentRecvSub, err := buyerNode.eventBus.Subscribe(&events.OrderPaymentReceived{})
	if err != nil {
		t.Fatal(err)
	}
	ratingSigAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	ingestPaymentToWallets(t, paymentData, sellerNode, buyerNode)
	if err := buyerNode.Order().ProcessOrderPayment(context.Background(), paymentData); err != nil {
		t.Fatal(err)
	}

	waitForEvent(t, fundingSub, "OrderFunded on seller")
	waitForEvent(t, paymentRecvSub, "OrderPaymentReceived on buyer")
	waitForEvent(t, ratingSigAck, "MessageACK (rating sig) on seller")

	// ── Step 4: Seller Confirms (releases escrow) ───────────────
	sellerWallet, err := sellerNode.multiwallet.WalletForCurrencyCode(iwallet.CtMock.String())
	if err != nil {
		t.Fatal(err)
	}
	payoutAddr, err := sellerWallet.(*wallet.MockWallet).CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	confirmSub, err := buyerNode.eventBus.Subscribe(&events.OrderConfirmation{})
	if err != nil {
		t.Fatal(err)
	}
	confirmAck, err := sellerNode.eventBus.Subscribe(&events.MessageACK{})
	if err != nil {
		t.Fatal(err)
	}

	done4 := make(chan struct{})
	if err := sellerNode.Order().ConfirmOrder(orderID, "", payoutAddr.String(), done4); err != nil {
		t.Fatal(err)
	}
	waitForDone(t, done4, "ConfirmOrder")
	waitForEvent(t, confirmSub, "OrderConfirmation on buyer")
	waitForEvent(t, confirmAck, "MessageACK (confirm) on seller")

	// ── Step 5: RefundOrder should fail (escrow already released) ──
	done5 := make(chan struct{})
	err = sellerNode.Order().RefundOrder(orderID, "", done5)
	if err == nil {
		t.Fatal("Expected error when refunding confirmed CANCELABLE order, got nil")
	}
	t.Logf("RefundOrder correctly rejected: %v", err)

	// ── Verify order is still in confirmed state ────────────────
	var sellerOrder models.Order
	err = sellerNode.repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Last(&sellerOrder).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if sellerOrder.SerializedOrderConfirmation == nil {
		t.Error("Order should still have confirmation")
	}

	t.Log("CANCELABLE confirm + refund-blocked: Purchase -> Payment -> Confirm -> RefundBlocked")
}

// TestOrderLifecycle_ClientSigned_InstructionMatrix validates all chains' instruction
// behavior in a table-driven test. Pure memory — no Mocknet needed.
func TestOrderLifecycle_ClientSigned_InstructionMatrix(t *testing.T) {
	n := &MobazhaNode{identityFields: identityFields{nodeID: "test-instruction-matrix"}}
	n.registerPaymentStrategies()
	n.paymentRegistry.Register(iwallet.ChainMock, newStubUTXOAdapter())

	type chainTest struct {
		chain    iwallet.ChainType
		model    payment.PaymentModel
		hasInstr bool
	}

	chains := []chainTest{
		{iwallet.ChainBitcoin, payment.PaymentModelMonitored, false},
		{iwallet.ChainLitecoin, payment.PaymentModelMonitored, false},
		{iwallet.ChainBitcoinCash, payment.PaymentModelMonitored, false},
		{iwallet.ChainZCash, payment.PaymentModelMonitored, false},
		{iwallet.ChainMock, payment.PaymentModelMonitored, false},
		{iwallet.ChainBSC, payment.PaymentModelClientSigned, true},
		{iwallet.ChainEthereum, payment.PaymentModelClientSigned, true},
		{iwallet.ChainPolygon, payment.PaymentModelClientSigned, true},
		{iwallet.ChainBase, payment.PaymentModelClientSigned, true},
		{iwallet.ChainSolana, payment.PaymentModelClientSigned, true},
	}

	ctx := context.Background()

	for _, tc := range chains {
		t.Run(string(tc.chain), func(t *testing.T) {
			strategy, err := n.paymentRegistry.ForChain(tc.chain)
			if err != nil {
				t.Fatalf("ForChain(%s) failed: %v", tc.chain, err)
			}
			if strategy.Model() != tc.model {
				t.Errorf("Model = %s, want %s", strategy.Model(), tc.model)
			}

			if !tc.hasInstr {
				params := payment.InstructionParams{}
				methods := map[string]func(context.Context, payment.InstructionParams) (*payment.InstructionResult, error){
					"Confirm":        strategy.GetConfirmInstructions,
					"Cancel":         strategy.GetCancelInstructions,
					"Complete":       strategy.GetCompleteInstructions,
					"DisputeRelease": strategy.GetDisputeReleaseInstructions,
				}
				for name, fn := range methods {
					result, err := fn(ctx, params)
					if err != nil {
						t.Errorf("%s should not error for UTXO chain, got: %v", name, err)
						continue
					}
					if result.Instructions != nil {
						t.Errorf("%s: UTXO should return nil instructions, got non-nil", name)
					}
				}
			}
		})
	}
}
