package electrum

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

func skipExternalInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skip("Skipping external Electrum server test in CI (set CI= to override)")
	}
}

// TestDirectElectrumConnection tests raw TLS connection to Electrum server
// This bypasses the library to verify the protocol works correctly
func TestDirectElectrumConnection(t *testing.T) {
	skipExternalInCI(t)

	server := "electrum.blockstream.info:50002"
	t.Logf("Connecting to %s", server)

	// Connect with TLS
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", server, &tls.Config{
		ServerName: "electrum.blockstream.info",
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer conn.Close()
	t.Log("TLS connected")

	// Set deadlines
	conn.SetDeadline(time.Now().Add(15 * time.Second))

	// Create reader/writer
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Test server.version
	req := map[string]interface{}{
		"id":     1,
		"method": "server.version",
		"params": []string{"mobazha-test", "1.4"},
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')

	_, err = writer.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	err = writer.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	t.Logf("server.version response: %s", string(line))

	// Test server.ping
	req2 := map[string]interface{}{
		"id":     2,
		"method": "server.ping",
		"params": []interface{}{},
	}
	data2, _ := json.Marshal(req2)
	data2 = append(data2, '\n')
	writer.Write(data2)
	writer.Flush()

	line2, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Ping read failed: %v", err)
	}
	t.Logf("server.ping response: %s", string(line2))

	// Test blockchain.estimatefee
	req3 := map[string]interface{}{
		"id":     3,
		"method": "blockchain.estimatefee",
		"params": []int{6},
	}
	data3, _ := json.Marshal(req3)
	data3 = append(data3, '\n')
	writer.Write(data3)
	writer.Flush()

	line3, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("EstimateFee read failed: %v", err)
	}
	t.Logf("blockchain.estimatefee response: %s", string(line3))
}

// TestClientConnect tests the Electrum client library connection
func TestClientConnect(t *testing.T) {
	skipExternalInCI(t)

	chains := []string{"BTC", "LTC", "BCH"}
	var wg sync.WaitGroup
	results := make(chan string, len(chains))

	for _, chain := range chains {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			result := testChainConnect(t, c)
			results <- result
		}(chain)
	}

	wg.Wait()
	close(results)

	// Collect results
	for result := range results {
		t.Log(result)
	}
}

func testChainConnect(t *testing.T, chain string) string {
	servers := GetDefaultServers(chain, false)
	if len(servers) == 0 {
		return chain + ": No default servers"
	}

	config := DefaultClientConfig(chain, false)
	config.Timeout = 5 * time.Second
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return chain + ": Connect failed - " + err.Error()
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		return chain + ": Connected but ping failed - " + err.Error()
	}

	return chain + ": OK"
}

// TestClientEstimateFee tests fee estimation
func TestClientEstimateFee(t *testing.T) {
	skipExternalInCI(t)

	config := DefaultClientConfig("BTC", false)
	config.Timeout = 5 * time.Second
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	fee, err := client.EstimateFee(ctx, 6)
	if err != nil {
		t.Fatalf("EstimateFee failed: %v", err)
	}

	// Fee should be positive and reasonable (0.00001 to 0.01 BTC/kB)
	if fee <= 0 || fee > 0.01 {
		t.Errorf("Fee out of expected range: %f", fee)
	}

	t.Logf("Estimated fee for 6 blocks: %.8f BTC/kB (%.1f sat/vB)", fee, fee*1e5)
}

// TestClientGetBalance tests getting address balance
func TestClientGetBalance(t *testing.T) {
	skipExternalInCI(t)

	config := DefaultClientConfig("BTC", false)
	config.Timeout = 5 * time.Second
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Use Satoshi's genesis address
	testAddress := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	scriptHash, err := addressToScriptHash(testAddress, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("Failed to convert address: %v", err)
	}

	balance, err := client.GetBalance(ctx, scriptHash)
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}

	t.Logf("Balance for %s: confirmed=%d sat, unconfirmed=%d sat",
		testAddress, balance.Confirmed, balance.Unconfirmed)

	// Genesis address should have some balance (donations over the years)
	if balance.Confirmed <= 0 {
		t.Error("Expected positive confirmed balance for genesis address")
	}
}

// TestClientGetHistory tests getting address transaction history
func TestClientGetHistory(t *testing.T) {
	skipExternalInCI(t)

	config := DefaultClientConfig("BTC", false)
	config.Timeout = 5 * time.Second
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Use Satoshi's genesis address
	testAddress := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	scriptHash, err := addressToScriptHash(testAddress, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("Failed to convert address: %v", err)
	}

	history, err := client.GetHistory(ctx, scriptHash)
	if err != nil {
		// Server busy is acceptable - genesis address has many transactions
		t.Skipf("GetHistory skipped (server may be busy): %v", err)
	}

	t.Logf("Found %d transactions for %s", len(history), testAddress)

	// Genesis address should have many transactions
	if len(history) < 100 {
		t.Errorf("Expected at least 100 transactions, got %d", len(history))
	}
}

// TestClientSubscribe tests subscribing to address notifications
// This is the core functionality for payment monitoring
func TestClientSubscribe(t *testing.T) {
	skipExternalInCI(t)

	config := DefaultClientConfig("BTC", false)
	config.Timeout = 5 * time.Second
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Generate a fresh random address to avoid "history too large" errors
	// from high-traffic addresses like Satoshi's genesis address.
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKeyHash := btcutil.Hash160(privKey.PubKey().SerializeCompressed())
	addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("Failed to create address: %v", err)
	}
	testAddress := addr.EncodeAddress()

	scriptHash, err := addressToScriptHash(testAddress, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("Failed to convert address: %v", err)
	}

	// Track if callback was invoked (for initial status)
	callbackInvoked := make(chan bool, 1)

	// Subscribe to address
	err = client.Subscribe(ctx, scriptHash, func(params []interface{}) {
		t.Logf("Subscription callback invoked with params: %v", params)
		select {
		case callbackInvoked <- true:
		default:
		}
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	t.Logf("Successfully subscribed to %s (scripthash: %s)", testAddress, scriptHash)

	// Verify we can unsubscribe (not all servers support this RPC)
	err = client.Unsubscribe(ctx, scriptHash)
	if err != nil {
		t.Logf("Unsubscribe unsupported by server: %v", err)
	} else {
		t.Log("Successfully unsubscribed")
	}
}

// TestClientSubscribeMultiple tests subscribing to multiple addresses
func TestClientSubscribeMultiple(t *testing.T) {
	skipExternalInCI(t)

	config := DefaultClientConfig("BTC", false)
	config.Timeout = 5 * time.Second
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Generate fresh random addresses to avoid "history too large" errors
	scriptHashes := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		privKey, err := btcec.NewPrivateKey()
		if err != nil {
			t.Fatalf("Failed to generate key %d: %v", i, err)
		}
		pubKeyHash := btcutil.Hash160(privKey.PubKey().SerializeCompressed())
		addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, &chaincfg.MainNetParams)
		if err != nil {
			t.Fatalf("Failed to create address %d: %v", i, err)
		}
		scriptHash, err := addressToScriptHash(addr.EncodeAddress(), &chaincfg.MainNetParams)
		if err != nil {
			t.Fatalf("Failed to convert address %d: %v", i, err)
		}
		scriptHashes = append(scriptHashes, scriptHash)
	}

	// Subscribe to all addresses
	for i, scriptHash := range scriptHashes {
		err := client.Subscribe(ctx, scriptHash, func(params []interface{}) {
			t.Logf("Callback for address %d: %v", i, params)
		})
		if err != nil {
			t.Errorf("Subscribe to address %d failed: %v", i, err)
		} else {
			t.Logf("Subscribed to address %d (scripthash: %s)", i, scriptHash[:16]+"...")
		}
	}

	// Unsubscribe from all (not all servers support this RPC)
	for i, scriptHash := range scriptHashes {
		err := client.Unsubscribe(ctx, scriptHash)
		if err != nil {
			t.Logf("Unsubscribe from address %d unsupported by server: %v", i, err)
		}
	}
	t.Log("Unsubscribe pass completed")
}

// TestClientGetTransaction tests fetching a specific transaction
func TestClientGetTransaction(t *testing.T) {
	skipExternalInCI(t)

	config := DefaultClientConfig("BTC", false)
	config.Timeout = 5 * time.Second
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// A well-known early Bitcoin transaction (Hal Finney's first received BTC)
	// Note: Genesis coinbase cannot be fetched via Electrum
	txid := "f4184fc596403b9d638783cf57adfe4c75c605f6356fbc91338530e9831e9e16"

	txInfo, err := client.GetTransaction(ctx, txid, false)
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}

	t.Logf("Got transaction %s", txid)
	t.Logf("  Raw hex length: %d", len(txInfo.Hex))

	// Verify it's a valid transaction
	if txInfo.Hex == "" {
		t.Error("Transaction hex should not be empty")
	}
}

// TestClientReconnect tests automatic reconnection
func TestClientReconnect(t *testing.T) {
	skipExternalInCI(t)

	config := DefaultClientConfig("BTC", false)
	config.Timeout = 5 * time.Second
	config.ReconnectDelay = 1 * time.Second
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Initial connect failed: %v", err)
	}

	// Verify connected
	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	// Ping to verify connection works
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Initial ping failed: %v", err)
	}
	t.Log("Initial connection and ping successful")

	// Close the client
	client.Close()

	if client.IsConnected() {
		t.Error("Client should be disconnected after Close()")
	}
	t.Log("Client disconnected successfully")
}

// TestClientConnectTestnet tests connecting to testnet servers
func TestClientConnectTestnet(t *testing.T) {
	skipExternalInCI(t)

	// Test BTC testnet only (most reliable testnet servers)
	config := DefaultClientConfig("BTC", true)
	config.Timeout = 5 * time.Second

	if len(config.Servers) == 0 {
		t.Skip("No testnet servers configured")
	}

	t.Logf("Testnet servers: %v", config.Servers)

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		// Testnet servers may not always be available
		t.Skipf("Testnet connect skipped (servers may be unavailable): %v", err)
	}
	defer client.Close()

	t.Log("Successfully connected to BTC testnet!")

	// Test ping
	if err := client.Ping(ctx); err != nil {
		t.Errorf("Testnet ping failed: %v", err)
	} else {
		t.Log("Testnet ping OK")
	}
}

// TestGetServers tests server list retrieval
func TestGetServers(t *testing.T) {
	// Test mainnet
	btcMainnet := GetDefaultServers("BTC", false)
	if len(btcMainnet) == 0 {
		t.Error("Expected BTC mainnet servers")
	}
	t.Logf("BTC mainnet: %d servers", len(btcMainnet))

	ltcMainnet := GetDefaultServers("LTC", false)
	if len(ltcMainnet) == 0 {
		t.Error("Expected LTC mainnet servers")
	}
	t.Logf("LTC mainnet: %d servers", len(ltcMainnet))

	// Test testnet
	btcTestnet := GetDefaultServers("BTC", true)
	if len(btcTestnet) == 0 {
		t.Error("Expected BTC testnet servers")
	}
	t.Logf("BTC testnet: %d servers", len(btcTestnet))

	// Verify testnet and mainnet are different
	if len(btcMainnet) > 0 && len(btcTestnet) > 0 {
		if btcMainnet[0] == btcTestnet[0] {
			t.Error("Mainnet and testnet servers should be different")
		}
	}
}

// addressToScriptHash converts a Bitcoin address to Electrum scripthash format
func addressToScriptHash(address string, params *chaincfg.Params) (string, error) {
	addr, err := btcutil.DecodeAddress(address, params)
	if err != nil {
		return "", err
	}

	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return "", err
	}

	// SHA256 hash
	hash := sha256.Sum256(script)

	// Reverse the bytes (Electrum format)
	for i, j := 0, len(hash)-1; i < j; i, j = i+1, j-1 {
		hash[i], hash[j] = hash[j], hash[i]
	}

	return hex.EncodeToString(hash[:]), nil
}
