package wallet_interface

import (
	"errors"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
)

var ErrInsufficientFunds = errors.New("insufficient funds")

// OrderFinishType maps to the OrderFinishType in Escrow contract
type OrderFinishType uint8

const (
	// Buyer has completed the order
	ORDER_FINISH_COMPLETE OrderFinishType = 0
	// Vendor cancel the order
	ORDER_FINISH_CANCEL OrderFinishType = 1
	// Vendor refunded the order
	ORDER_FINISH_REFUND OrderFinishType = 2
	// The winning party has accepted the dispute and it is now complete
	ORDER_FINISH_RESOLVED OrderFinishType = 3
	// For executeAndClaim in MGLRewards
	ORDER_FINISH_OTHER OrderFinishType = 4
)

// Tx represents a database transaction used for atomic updates. It is expected that
// wallets implementing the full interface will respect the transaction and only
// commit the particular change on Commit() and will roll back the database change
// on Rollback(). In the case of a cryptocurrency transaction this would imply that
// the transaction not be broadcasted until Commit() and Rollback() will prevent
// broadcast and restore the prior wallet state.
type Tx interface {
	// Commit commits all changes that have been made to wallet state.
	// Depending on the backend implementation this could be to a cache that
	// is periodically synced to persistent storage or directly to persistent
	// storage.  In any case, all transactions which are started after the commit
	// finishes will include all changes made by this transaction.
	Commit() error

	// Rollback undoes all changes that have been made to the wallet state.
	Rollback() error
}

type WalletLoader interface {
	// WalletExists should return whether the wallet exits or has been
	// initialized.
	WalletExists() bool

	// CreateWallet should initialize the wallet. This will be called by
	// Mobazha if WalletExists() returns false.
	//
	// The xPriv will be used to create a bip44 keychain. The xPriv is the
	// `account` level in the bip44 path. For example in the following
	// path the wallet should only derive the paths after `account` as
	// m, purpose', and coin_type' are kept private by Mobazha so this
	// wallet cannot derive keys from other wallets.
	//
	// m / purpose' / coin_type' / account' / change / address_index
	//
	// The birthday can be used determine where to sync state from if
	// appropriate.
	CreateWallet(xpriv hd.ExtendedKey, birthday time.Time) error

	// Open wallet will be called each time on Mobazha start. It
	// will also be called after CreateWallet().
	OpenWallet() error

	// CloseWallet will be called when Mobazha shuts down.
	CloseWallet() error
}

type Wallet interface {
	// WalletLoader must be implemented by this interface.
	WalletLoader

	// Begin returns a new database transaction. A transaction must only be used
	// once. After Commit() or Rollback() is called the transaction can be discarded.
	Begin() (Tx, error)

	// BlockchainInfo returns the best hash and height of the chain.
	BlockchainInfo() (BlockInfo, error)

	// CoinCategory returns the category of the coin
	CoinCategory() CoinCategory

	// IsTestnet returns whether the wallet is using testnet
	IsTestnet() bool

	// ValidateAddress validates that the serialization of the address is correct
	// for this coin and network. It returns an error if it isn't.
	ValidateAddress(addr Address) error

	// GetTransaction returns a transaction given it's ID. This is used by Mobazha to
	// request transactions paid to an order's payment address. This means we expect both
	// internal wallet transactions and transactions sending to or from a watched address
	// to be returned here.
	GetTransaction(id TransactionID, coinType CoinType) (*Transaction, error)
}

// Spender is an optional interface for wallets that support spending funds directly.
// In the new architecture, UTXO wallets are primarily used for signing and escrow,
// and do not implement Spender. Mock and EVM wallets may still implement it.
type Spender interface {
	// Spend is a request to send requested amount to the requested address. The
	// fee level is provided by the user. It's up to the implementation to decide
	// how best to use the fee level.
	//
	// The database Tx MUST be respected. When this function is called the wallet
	// state changes should be prepped and held in memory. If Rollback() is called
	// the state changes should be discarded. Only when Commit() is called should
	// the state changes be applied and the transaction broadcasted to the network.
	Spend(dbtx Tx, to Address, amt Amount, feeLevel FeeLevel, platformAddr Address, platformAmt Amount) (TransactionID, error)
}

// UTXOEscrow is functions related to the Mobazha escrow system. This interface should
// be implemented but it's technically optional as some coins like ExternalPayment have a
// hard time implementing escrow. If it's not implemented then this coin will not
// be selectable for either escrow payments or offline payments.
type UTXOEscrow interface {
	// EstimateEscrowFee estimates the fee to release the funds from escrow.
	EstimateEscrowFee(nInputs int, threshold int, nOuts int, level FeeLevel) (Amount, error)

	// CreateMultisigAddress creates a new threshold multisig address using the
	// provided pubkeys and the threshold. The multisig address is returned along
	// with a byte slice. The byte slice will typically be the redeem script for
	// the address (in Bitcoin related coins). The slice will be saved in Mobazha
	// with the order and passed back into the wallet when signing the transaction.
	// In practice this does not need to be a redeem script so long as the wallet
	// knows how to sign the transaction when it sees it.
	//
	// This function should be deterministic as both buyer and vendor will be passing
	// in the same set of keys and expecting to get back the same address and redeem
	// script. If this is not the case the vendor will reject the order.
	//
	// Note that this is normally a 2 of 3 escrow in the normal case, however Mobazha
	// also uses 1 of 2 multisigs as a form of a "cancelable" address when sending to
	// a node that is offline. This allows the sender to cancel the payment if the vendor
	// never comes back online.
	CreateMultisigAddress(keys []btcec.PublicKey, chaincode []byte, threshold int) (Address, []byte, error)

	// SignMultisigTransaction should use the provided key to create a signature for
	// the multisig transaction. Since this a threshold signature this function will
	// separately by each party signing this transaction. The resulting signatures
	// will be shared between the relevant parties and one of them will aggregate
	// the signatures into a transaction for broadcast.
	//
	// For coins like bitcoin you may need to return one signature *per input* which is
	// why a slice of signatures is returned.
	SignMultisigTransaction(txn Transaction, key btcec.PrivateKey, redeemScript []byte) ([]EscrowSignature, error)

	// BuildAndSend should used the passed in signatures to build the transaction.
	// Note the signatures are a slice of slices. This is because coins like Bitcoin
	// may require one signature *per input*. In this case the outer slice is the
	// signatures from the different key holders and the inner slice is the keys
	// per input.
	// (TransactionID,
	// Note a database transaction is used here. Same rules of Commit() and
	// Rollback() apply.
	BuildAndSend(dbtx Tx, txn Transaction, signatures [][]EscrowSignature, redeemScript []byte, finishType OrderFinishType) (TransactionID, error)
}

// SweepInput describes one unspent output to spend during a sweep.
type SweepInput struct {
	TxHash      string // hex-encoded transaction hash
	OutputIndex uint32
	Value       int64 // amount in satoshi/litoshi
}

// UTXOSweeper builds and signs a sweep transaction that spends all provided
// inputs into a single output (minus fee). Each UTXO chain implements this
// with its own signing method (P2WPKH for BTC/LTC, P2PKH+ForkID for BCH,
// ZIP-243 for ZEC). The returned bytes are the fully-signed serialized
// transaction ready for broadcast.
type UTXOSweeper interface {
	BuildSweepTx(inputs []SweepInput, signingKey btcec.PrivateKey, destAddress string, feePerByte int64) (rawTx []byte, txHash string, err error)
}

// UTXOAddressUtilities provides chain-specific address derivation and parsing
// utilities. Implemented by all UTXO wallets (BTC/LTC/BCH/ZEC). This interface
// centralizes chain-specific logic (HRP, network params, encoding format) inside
// each wallet package, so callers (KeyDeriver, GuestPaymentMonitor) need not
// branch on chain type and need not depend on btcd-only chain params.
//
// Each chain uses its preferred derivation type:
//   - BTC, LTC: P2WPKH (native Segwit, bech32 with chain-specific HRP)
//   - BCH, ZEC: P2PKH (legacy, base58)
//
// All implementations are testnet-aware via the wallet's internal params().
type UTXOAddressUtilities interface {
	// DerivePaymentAddressFromPubKey derives the canonical payment address
	// for this chain from a public key, returning both the encoded address
	// (mainnet/testnet HRP per wallet config) and its scriptPubKey for
	// transaction signing/monitoring.
	DerivePaymentAddressFromPubKey(pubKey *btcec.PublicKey) (address string, scriptPubKey []byte, err error)

	// AddressToScriptPubKey decodes an encoded address (string) into its
	// scriptPubKey. The address must be valid for this chain on the wallet's
	// configured network (mainnet/testnet); returns an error otherwise.
	// Used by chain monitors to compute scripthashes for Electrum subscriptions.
	AddressToScriptPubKey(address string) ([]byte, error)
}

// UTXOEscrowWithTimeout is an optional interface to be implemented by wallets whos coins
// are capable of supporting time based release of funds from escrow.
type UTXOEscrowWithTimeout interface {
	// CreateMultisigWithTimeout is the same as CreateMultisigAddress but it adds
	// an additional timeout to the address. The address should have two ways to
	// release the funds:
	//  - m of n signatures are provided (or)
	//  - timeout has passed and a signature for timeoutKey is provided.
	CreateMultisigWithTimeout(keys []btcec.PublicKey, chaincode []byte, threshold int, timeout time.Duration, timeoutKey btcec.PublicKey) (Address, []byte, error)

	// ReleaseFundsAfterTimeout will release funds from the escrow. The signature will
	// be created using the timeoutKey.
	ReleaseFundsAfterTimeout(dbtx Tx, txn Transaction, timeoutKey btcec.PrivateKey, redeemScript []byte, finishType OrderFinishType) (TransactionID, error)
}
