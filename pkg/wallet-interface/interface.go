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
	//
	// If the wallet does not implement WalletCrypter then pw will be
	// nil. Otherwise it should be used to encrypt the private keys.
	CreateWallet(xpriv hd.ExtendedKey, pw []byte, birthday time.Time) error

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

	// CurrentAddress is called when requesting this wallet's receiving
	// address. It is customary that the wallet return the first unused
	// address and only return a different address after funds have been
	// received on the address. This, however, is just a wallet implementation
	// detail.
	CurrentAddress() (Address, error)

	// ValidateAddress validates that the serialization of the address is correct
	// for this coin and network. It returns an error if it isn't.
	ValidateAddress(addr Address) error

	// Balance should return the confirmed and unconfirmed balance for the wallet.
	Balance() (unconfirmed Amount, confirmed Amount, err error)

	// GetTransaction returns a transaction given it's ID. This is used by Mobazha to
	// request transactions paid to an order's payment address. This means we expect both
	// internal wallet transactions and transactions sending to or from a watched address
	// to be returned here.
	GetTransaction(id TransactionID, coinType CoinType) (*Transaction, error)

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

// Escrow is functions related to the Mobazha escrow system. This interface should
// be implemented but it's technically optional as some coins like ExternalPayment have a
// hard time implementing escrow. If it's not implemented then this coin will not
// be selectable for either escrow payments or offline payments.
type Escrow interface {
	// EstimateEscrowFee estimates the fee to release the funds from escrow.
	// this assumes only one input. If there are more inputs Mobazha will
	// add 50% of the returned fee for each additional input. This is a
	// crude fee calculating but it simplifies things quite a bit.
	EstimateEscrowFee(threshold int, nOuts int, level FeeLevel) (Amount, error)

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

// EscrowWithTimeout is an optional interface to be implemented by wallets whos coins
// are capable of supporting time based release of funds from escrow.
type EscrowWithTimeout interface {
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

// WalletCrypter is an optional interface that the wallet may implement to allow
// for encrypting private keys. If this is implemented Mobazha will call these
// functions as specified below.
type WalletCrypter interface {
	// SetPassphase is called after creating the wallet. It gives the wallet
	// the opportunity to set up encryption of the private keys.
	SetPassphase(pw []byte) error

	// ChangePassphrase is called in response to user action requesting the
	// passphrase be changed. It is expected that this will return an error
	// if the old password is incorrect.
	ChangePassphrase(old, new []byte) error

	// RemovePassphrase is called in response to user action requesting the
	// passphrase be removed. It is expected that this will return an error
	// if the old password is incorrect.
	RemovePassphrase(pw []byte) error

	// Unlock is called just prior to calling Spend(). The wallet should
	// decrypt the private key and hold the decrypted key in memory for
	// the provided duration after which it should be purged from memory.
	// If the provided password is incorrect it should error.
	Unlock(pw []byte, howLong time.Duration) error
}
