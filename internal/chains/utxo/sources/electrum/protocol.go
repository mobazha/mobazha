package electrum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// Electrum JSON-RPC protocol implementation
// Reference: https://electrumx.readthedocs.io/en/latest/protocol-methods.html

// Request represents an Electrum JSON-RPC request
type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      uint64        `json:"id"`
}

// Response represents an Electrum JSON-RPC response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      uint64          `json:"id"`
}

// RPCError represents an Electrum JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("electrum error %d: %s", e.Code, e.Message)
}

// Notification represents an Electrum subscription notification
type Notification struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// ServerVersion response
type ServerVersion struct {
	ServerSoftware string `json:"server_software"`
	ProtocolMin    string `json:"protocol_min"`
	ProtocolMax    string `json:"protocol_max"`
}

// ScriptHashHistory represents transaction history for a scripthash
type ScriptHashHistory struct {
	TxHash string `json:"tx_hash"`
	Height int64  `json:"height"` // -1 for unconfirmed, 0 for unconfirmed with unconfirmed parents
	Fee    int64  `json:"fee,omitempty"`
}

// ScriptHashBalance represents the balance for a scripthash
type ScriptHashBalance struct {
	Confirmed   uint64 `json:"confirmed"`
	Unconfirmed int64  `json:"unconfirmed"` // Can be negative
}

// ScriptHashUnspent represents an unspent output for a scripthash
type ScriptHashUnspent struct {
	TxHash string `json:"tx_hash"`
	TxPos  uint32 `json:"tx_pos"`
	Height int64  `json:"height"`
	Value  uint64 `json:"value"`
}

// TransactionInfo represents transaction information
type TransactionInfo struct {
	Hex           string   `json:"hex"`
	Blockhash     string   `json:"blockhash,omitempty"`
	Confirmations int64    `json:"confirmations,omitempty"`
	Time          int64    `json:"time,omitempty"`
	Blocktime     int64    `json:"blocktime,omitempty"`
	Vin           []TxVin  `json:"vin,omitempty"`
	Vout          []TxVout `json:"vout,omitempty"`
}

// TxVin represents a transaction input
type TxVin struct {
	Txid      string `json:"txid"`
	Vout      uint32 `json:"vout"`
	ScriptSig struct {
		Asm string `json:"asm"`
		Hex string `json:"hex"`
	} `json:"scriptSig,omitempty"`
	Sequence    uint32   `json:"sequence"`
	Txinwitness []string `json:"txinwitness,omitempty"`
}

// TxVout represents a transaction output
type TxVout struct {
	Value        float64 `json:"value"`
	N            uint32  `json:"n"`
	ScriptPubKey struct {
		Asm       string   `json:"asm"`
		Hex       string   `json:"hex"`
		Address   string   `json:"address,omitempty"`
		Addresses []string `json:"addresses,omitempty"` // Legacy format
		Type      string   `json:"type"`
	} `json:"scriptPubKey"`
}

// BlockHeader represents a block header
type BlockHeader struct {
	Height            int64  `json:"height"`
	Hex               string `json:"hex"`
	PreviousBlockHash string `json:"prev_block_hash,omitempty"`
	Timestamp         int64  `json:"timestamp,omitempty"`
}

// RequestIDGenerator generates unique request IDs
type RequestIDGenerator struct {
	counter uint64
}

// Next returns the next request ID
func (g *RequestIDGenerator) Next() uint64 {
	return atomic.AddUint64(&g.counter, 1)
}

// NewRequest creates a new Electrum request
func NewRequest(id uint64, method string, params ...interface{}) *Request {
	if params == nil {
		params = []interface{}{}
	}
	return &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}
}

// AddressToScriptHash converts a Bitcoin address to Electrum scripthash format
// The scripthash is the sha256 hash of the scriptPubKey, reversed (little-endian)
func AddressToScriptHash(address string, scriptPubKey []byte) string {
	hash := sha256.Sum256(scriptPubKey)
	// Reverse the hash (to little-endian)
	reversed := make([]byte, 32)
	for i := 0; i < 32; i++ {
		reversed[i] = hash[31-i]
	}
	return hex.EncodeToString(reversed)
}

// Electrum protocol method names
const (
	MethodServerVersion         = "server.version"
	MethodServerBanner          = "server.banner"
	MethodServerPing            = "server.ping"
	MethodBlockchainHeaders     = "blockchain.headers.subscribe"
	MethodBlockchainRelayfee    = "blockchain.relayfee"
	MethodBlockchainEstimatefee = "blockchain.estimatefee"
	MethodScripthashBalance     = "blockchain.scripthash.get_balance"
	MethodScripthashHistory     = "blockchain.scripthash.get_history"
	MethodScripthashMempool     = "blockchain.scripthash.get_mempool"
	MethodScripthashListunspent = "blockchain.scripthash.listunspent"
	MethodScripthashSubscribe   = "blockchain.scripthash.subscribe"
	MethodScripthashUnsubscribe = "blockchain.scripthash.unsubscribe"
	MethodTransactionBroadcast  = "blockchain.transaction.broadcast"
	MethodTransactionGet        = "blockchain.transaction.get"
	MethodTransactionGetMerkle  = "blockchain.transaction.get_merkle"
	MethodTransactionIdFromPos  = "blockchain.transaction.id_from_pos"
)

// Default Electrum servers for each chain (SSL/TLS port 50002)
// Servers are selected for global distribution and reliability
// Note: Some servers may not be reachable from certain regions,
// the client will try them in parallel and use the first successful one
var DefaultElectrumServers = map[string][]string{
	"BTC": {
		// High reliability servers
		"electrum.blockstream.info:50002", // Blockstream (US/EU)
		"electrum.acinq.co:50002",         // ACINQ (France)
		// BlueWallet servers (good global coverage)
		"electrum1.bluewallet.io:443",
		"electrum2.bluewallet.io:443",
		// Additional servers for geographic distribution
		"bolt.schulzemic.net:50002",
		"electrum.bitaroo.net:50002",  // Australia
		"fortress.qtornado.com:443",   // US
		"electrum.hodlister.co:50002", // Europe
		"btc.lastingcoin.net:50002",   // Asia
		"electrum.hsmiths.com:50002",  // US
	},
	"LTC": {
		"electrum-ltc.bysh.me:50002",
		"electrum.ltc.xurious.com:50002",
		"electrum.ltcwallet.org:50002",
		"electrum-ltc.petrkr.net:60002",
		"ltc.rentonisk.com:50002",
		"electrum.ltc.einfachmalansen.de:50002",
	},
	"BCH": {
		"bch.imaginary.cash:50002",
		"electrum.imaginary.cash:50002",
		"electron.jochen-hoenicke.de:51002",
		"bch.loping.net:50002",
		"electroncash.dk:50002",
		"bch.soul-dev.com:50002",
	},
	"ZEC": {
		// ZEC uses lightwalletd protocol, not standard Electrum
		// For Zcash, consider using lightwalletd servers or zcashd RPC
		// Leaving empty - configure via environment or config file
	},
}

// Default Electrum TESTNET servers for each chain
// Note: BTC now uses Testnet4 (the latest testnet, replacing testnet3)
var DefaultElectrumTestnetServers = map[string][]string{
	"BTC": {
		// Bitcoin Testnet4 (new testnet, launched 2024)
		// Note: Testnet4 Electrum server availability is limited
		// mempool.space provides good coverage for testnet4
		"electrum.blockstream.info:40002", // Blockstream testnet4 (if available)
		// Fallback to testnet3 servers (some may support testnet4)
		"testnet.aranguren.org:51002",
		"testnet.qtornado.com:51002",
		"tn.not.fyi:55002",
	},
	"LTC": {
		// Litecoin Testnet
		"electrum-ltc.bysh.me:51002",
		"electrum.ltc.xurious.com:51002",
	},
	"BCH": {
		// Bitcoin Cash Testnet (testnet3/testnet4)
		"tbch.imaginary.cash:50002",
		"testnet.imaginary.cash:50002",
	},
	"ZEC": {
		// ZEC testnet uses lightwalletd
	},
}

// GetDefaultServers returns the Electrum servers for a chain
// Set testnet=true for testnet servers, false for mainnet
func GetDefaultServers(chain string, testnet bool) []string {
	if testnet {
		if servers, ok := DefaultElectrumTestnetServers[chain]; ok {
			return servers
		}
		return nil
	}
	if servers, ok := DefaultElectrumServers[chain]; ok {
		return servers
	}
	return nil
}
