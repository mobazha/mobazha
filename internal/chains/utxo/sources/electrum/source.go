package electrum

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"

	pkgutxo "github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// Source implements utxo.PaymentSource using Electrum protocol
type Source struct {
	client    *Client
	chain     iwallet.ChainType
	coinType  iwallet.CoinType
	mu        sync.RWMutex
	healthy   bool
	lastCheck time.Time
}

// NewSource creates a new Electrum payment source
// Set testnet=true for testnet servers, false for mainnet
func NewSource(chain iwallet.ChainType, config *ClientConfig, testnet bool) *Source {
	if config == nil {
		config = DefaultClientConfig(string(chain), testnet)
	}
	config.Chain = string(chain)
	config.Testnet = testnet

	coinType, err := iwallet.RequireCanonicalNativeCoinType(chain)
	if err != nil {
		log.Errorf("Electrum source chain %s has no canonical native coin type: %v", chain, err)
	}

	return &Source{
		client:   NewClient(config),
		chain:    chain,
		coinType: coinType,
		healthy:  false,
	}
}

// Connect establishes connection to Electrum server
func (s *Source) Connect(ctx context.Context) error {
	err := s.client.Connect(ctx)
	s.mu.Lock()
	s.healthy = err == nil
	s.lastCheck = time.Now()
	s.mu.Unlock()
	return err
}

// Subscribe subscribes to payment notifications for an address
func (s *Source) Subscribe(ctx context.Context, address string, scriptPubKey []byte, callback func(tx *iwallet.Transaction)) error {
	if len(scriptPubKey) == 0 {
		return fmt.Errorf("scriptPubKey is required for Electrum subscription")
	}

	scriptHash := AddressToScriptHash(address, scriptPubKey)

	return s.client.Subscribe(ctx, scriptHash, func(params []interface{}) {
		// When we receive a notification, fetch the transaction history
		history, err := s.client.GetHistory(ctx, scriptHash)
		if err != nil {
			log.Warningf("Failed to get history after notification: %v", err)
			return
		}

		// Get the most recent transaction
		if len(history) > 0 {
			latest := history[len(history)-1]
			tx, err := s.fetchTransaction(ctx, latest.TxHash, latest.Height)
			if err != nil {
				log.Warningf("Failed to fetch transaction %s: %v", latest.TxHash, err)
				return
			}
			callback(tx)
		}
	})
}

// Unsubscribe unsubscribes from payment notifications
func (s *Source) Unsubscribe(ctx context.Context, address string) error {
	// Note: We'd need to track the scriptHash for each address
	// For now, this is a placeholder
	return nil
}

// GetTransactions gets transactions for an address
func (s *Source) GetTransactions(ctx context.Context, address string, scriptPubKey []byte) ([]*iwallet.Transaction, error) {
	if len(scriptPubKey) == 0 {
		return nil, fmt.Errorf("scriptPubKey is required")
	}

	scriptHash := AddressToScriptHash(address, scriptPubKey)

	history, err := s.client.GetHistory(ctx, scriptHash)
	if err != nil {
		s.markUnhealthy()
		return nil, err
	}

	s.markHealthy()

	txs := make([]*iwallet.Transaction, 0, len(history))
	for _, h := range history {
		tx, err := s.fetchTransaction(ctx, h.TxHash, h.Height)
		if err != nil {
			log.Warningf("Failed to fetch transaction %s: %v", h.TxHash, err)
			continue
		}
		txs = append(txs, tx)
	}

	return txs, nil
}

// GetTransaction gets a specific transaction by ID
func (s *Source) GetTransaction(ctx context.Context, txid string) (*iwallet.Transaction, error) {
	txInfo, err := s.client.GetTransaction(ctx, txid, true)
	if err != nil {
		// Don't mark unhealthy for "transaction not found" errors
		// Only mark unhealthy for connection errors (checked separately via IsConnected)
		return nil, err
	}

	s.markHealthy()

	var timestamp time.Time
	if txInfo.Time > 0 {
		timestamp = time.Unix(txInfo.Time, 0)
	} else {
		// For unconfirmed transactions, use current time as timestamp
		timestamp = time.Now()
	}

	tx := &iwallet.Transaction{
		ID:        iwallet.TransactionID(txid),
		Timestamp: timestamp,
	}

	// Parse outputs (vout) into To field
	tx.To = s.parseVouts(txid, txInfo.Vout)

	// Parse inputs (vin) into From field, fetching previous transactions to get addresses
	tx.From = s.parseVinsWithAddresses(ctx, txInfo.Vin)

	// Calculate total output value
	var totalValue uint64
	for _, to := range tx.To {
		totalValue += to.Amount.Uint64()
	}
	tx.Value = iwallet.NewAmount(totalValue)

	return tx, nil
}

func (s *Source) fetchTransaction(ctx context.Context, txid string, height int64) (*iwallet.Transaction, error) {
	txInfo, err := s.client.GetTransaction(ctx, txid, true)
	if err != nil {
		return nil, err
	}

	var h uint64
	if height > 0 {
		h = uint64(height)
	}

	tx := &iwallet.Transaction{
		ID:     iwallet.TransactionID(txid),
		Height: h,
	}

	if txInfo.Time > 0 {
		tx.Timestamp = time.Unix(txInfo.Time, 0)
	} else {
		// For unconfirmed transactions, use current time as timestamp
		tx.Timestamp = time.Now()
	}

	// Parse outputs (vout) into To field
	tx.To = s.parseVouts(txid, txInfo.Vout)

	// Parse inputs (vin) into From field, fetching previous transactions to get addresses
	tx.From = s.parseVinsWithAddresses(ctx, txInfo.Vin)

	// Calculate total output value
	var totalValue uint64
	for _, to := range tx.To {
		totalValue += to.Amount.Uint64()
	}
	tx.Value = iwallet.NewAmount(totalValue)

	return tx, nil
}

// parseVouts converts Electrum vouts to iwallet.SpendInfo slice
func (s *Source) parseVouts(txid string, vouts []TxVout) []iwallet.SpendInfo {
	var result []iwallet.SpendInfo
	for _, vout := range vouts {
		// Get address from vout
		address := vout.ScriptPubKey.Address
		if address == "" && len(vout.ScriptPubKey.Addresses) > 0 {
			address = vout.ScriptPubKey.Addresses[0]
		}

		// Skip outputs without addresses (e.g., OP_RETURN)
		if address == "" {
			continue
		}

		// Convert BTC value to satoshis
		satoshis := uint64(math.Round(vout.Value * 1e8))

		// Create outpoint ID: txid:vout (serialized)
		outpointID := makeOutpointID(txid, vout.N)
		if outpointID == nil {
			// Skip outputs with invalid txid
			continue
		}

		result = append(result, iwallet.SpendInfo{
			ID:      outpointID,
			Address: iwallet.NewAddress(address, s.getCoinType()),
			Amount:  iwallet.NewAmount(satoshis),
		})
	}
	return result
}

// parseVinsWithAddresses converts Electrum vins to iwallet.SpendInfo slice,
// fetching previous transactions to populate input addresses.
// This is useful for extracting buyer's refund address from payment transactions.
func (s *Source) parseVinsWithAddresses(ctx context.Context, vins []TxVin) []iwallet.SpendInfo {
	var result []iwallet.SpendInfo
	for _, vin := range vins {
		// Skip coinbase inputs
		if vin.Txid == "" {
			continue
		}

		// Create outpoint ID from the input's previous output
		outpointID := makeOutpointID(vin.Txid, vin.Vout)
		if outpointID == nil {
			continue
		}

		spendInfo := iwallet.SpendInfo{
			ID: outpointID,
		}

		// Fetch previous transaction to get address and amount
		prevTx, err := s.client.GetTransaction(ctx, vin.Txid, true)
		if err != nil {
			log.Warningf("Failed to fetch previous transaction %s: %v", vin.Txid, err)
			result = append(result, spendInfo)
			continue
		}

		// Find the output at vin.Vout
		if int(vin.Vout) < len(prevTx.Vout) {
			prevOut := prevTx.Vout[vin.Vout]
			address := prevOut.ScriptPubKey.Address
			if address == "" && len(prevOut.ScriptPubKey.Addresses) > 0 {
				address = prevOut.ScriptPubKey.Addresses[0]
			}
			if address != "" {
				spendInfo.Address = iwallet.NewAddress(address, s.getCoinType())
				satoshis := uint64(math.Round(prevOut.Value * 1e8))
				spendInfo.Amount = iwallet.NewAmount(satoshis)
			}
		}

		result = append(result, spendInfo)
	}
	return result
}

// makeOutpointID creates a serialized outpoint ID (txid:vout)
// Returns nil if txid is invalid or cannot be decoded
func makeOutpointID(txid string, vout uint32) []byte {
	if txid == "" {
		return nil
	}
	txidBytes, err := hex.DecodeString(txid)
	if err != nil {
		log.Warningf("Failed to decode txid %s: %v", txid, err)
		return nil
	}
	if len(txidBytes) != 32 {
		log.Warningf("Invalid txid length %d (expected 32): %s", len(txidBytes), txid)
		return nil
	}
	// Reverse txid bytes (Bitcoin uses little-endian internally)
	for i, j := 0, len(txidBytes)-1; i < j; i, j = i+1, j-1 {
		txidBytes[i], txidBytes[j] = txidBytes[j], txidBytes[i]
	}
	outpoint := make([]byte, 36)
	copy(outpoint[:32], txidBytes)
	binary.LittleEndian.PutUint32(outpoint[32:], vout)
	return outpoint
}

// getCoinType returns the iwallet.CoinType for this source's chain
func (s *Source) getCoinType() iwallet.CoinType {
	return s.coinType
}

// IsHealthy returns true if the source is healthy
func (s *Source) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if health check is stale (more than 5 minutes old)
	if time.Since(s.lastCheck) > 5*time.Minute {
		// Perform a health check in background
		go s.healthCheck()
	}

	return s.healthy && s.client.IsConnected()
}

func (s *Source) healthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.client.Ping(ctx)
	s.mu.Lock()
	s.healthy = err == nil
	s.lastCheck = time.Now()
	s.mu.Unlock()
}

func (s *Source) markHealthy() {
	s.mu.Lock()
	s.healthy = true
	s.lastCheck = time.Now()
	s.mu.Unlock()
}

func (s *Source) markUnhealthy() {
	s.mu.Lock()
	s.healthy = false
	s.lastCheck = time.Now()
	s.mu.Unlock()
}

// Chain returns the chain type this source supports
func (s *Source) Chain() iwallet.ChainType {
	return s.chain
}

// Close closes the source
func (s *Source) Close() error {
	return s.client.Close()
}

// GetClient returns the underlying Electrum client
func (s *Source) GetClient() *Client {
	return s.client
}

// BroadcastTransaction broadcasts a raw transaction
func (s *Source) BroadcastTransaction(ctx context.Context, txHex string) (string, error) {
	return s.client.BroadcastTransaction(ctx, txHex)
}

// EstimateFee estimates the fee for confirmation in numBlocks
// Returns fee in satoshis per byte
func (s *Source) EstimateFee(ctx context.Context, numBlocks int) (uint64, error) {
	fee, err := s.client.EstimateFee(ctx, numBlocks)
	if err != nil || fee < 0 {
		relayFee, relayErr := s.client.GetRelayFee(ctx)
		if relayErr != nil {
			if err != nil {
				return 0, err
			}
			return 0, fmt.Errorf("server cannot estimate fee (returned %f)", fee)
		}
		return feeBTCPerKBToSatPerVB(relayFee), nil
	}

	satPerVB := feeBTCPerKBToSatPerVB(fee)
	if relayFee, err := s.client.GetRelayFee(ctx); err == nil {
		if relaySatPerVB := feeBTCPerKBToSatPerVB(relayFee); relaySatPerVB > satPerVB {
			satPerVB = relaySatPerVB
		}
	}
	return satPerVB, nil
}

func feeBTCPerKBToSatPerVB(fee float64) uint64 {
	satPerVB := uint64(math.Ceil(fee * 1e5))
	if satPerVB < 1 {
		satPerVB = 1
	}
	return satPerVB
}

// ListUnspent returns unspent outputs for a scriptPubKey. The Electrum-specific
// scripthash conversion is handled internally, keeping the public interface
// chain-agnostic.
func (s *Source) ListUnspent(ctx context.Context, scriptPubKey []byte) ([]pkgutxo.UnspentOutput, error) {
	scriptHash := AddressToScriptHash("", scriptPubKey)

	items, err := s.client.ListUnspent(ctx, scriptHash)
	if err != nil {
		s.markUnhealthy()
		return nil, err
	}

	s.markHealthy()

	out := make([]pkgutxo.UnspentOutput, len(items))
	for i, item := range items {
		out[i] = pkgutxo.UnspentOutput{
			TxHash:      item.TxHash,
			OutputIndex: item.TxPos,
			Height:      item.Height,
			Value:       item.Value,
		}
	}
	return out, nil
}

// GetTxConfirmations returns the confirmation count for a transaction by
// fetching verbose transaction info from the Electrum server.
func (s *Source) GetTxConfirmations(ctx context.Context, txHash string) (int, error) {
	txInfo, err := s.client.GetTransaction(ctx, txHash, true)
	if err != nil {
		return 0, err
	}
	confs := int(txInfo.Confirmations)
	if confs < 0 {
		confs = 0
	}
	return confs, nil
}
