package bitcoincash

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	btchd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/txsort"
	"github.com/gcash/bchwallet/wallet/txrules"
	"github.com/gcash/bchwallet/wallet/txsizes"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/chains/database"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// Assert interfaces
var _ = iwallet.Wallet(&BitcoinCashWallet{})
var _ = iwallet.WalletCrypter(&BitcoinCashWallet{})
var _ = iwallet.UTXOEscrow(&BitcoinCashWallet{})
var _ = iwallet.UTXOEscrowWithTimeout(&BitcoinCashWallet{})
var _ = iwallet.UTXODirectPayment(&BitcoinCashWallet{})

// BitcoinCashWallet extends wallet base and implements the
// remaining functions for each interface.
type BitcoinCashWallet struct { // nolint
	base.WalletBase
	testnet bool
}

// NewBitcoinCashWallet returns a new BitcoinCashWallet.
// ChainClient is not created here — it will be injected later via SetChainClient()
// using the shared UTXOChainClient backed by Electrum/Mempool Monitor.
func NewBitcoinCashWallet(cfg *base.WalletConfig) (*BitcoinCashWallet, error) {
	w := &BitcoinCashWallet{
		testnet: cfg.Testnet,
	}
	w.Init()

	// ChainClient intentionally left nil — will be set by configureUTXOWallets()
	// via SetChainClient() with a shared UTXOChainClient (Electrum/Mempool).
	// Previously used Blockbook or BCHD clients, both replaced at runtime.
	w.DB = cfg.DB
	w.Logger = cfg.Logger
	nativeCoin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBitcoinCash)
	if err != nil {
		return nil, err
	}
	w.CoinType = nativeCoin
	w.Done = make(chan struct{})
	w.PostInitFunc = w.postInit
	w.NetConfig = cfg.NetConfig
	return w, nil
}

// ValidateAddress validates that the serialization of the address is correct
// for this coin and network. It returns an error if it isn't.
func (w *BitcoinCashWallet) ValidateAddress(addr iwallet.Address) error {
	_, err := w.getPayToAddrScript(addr.String())
	return err
}

func (w *BitcoinCashWallet) getPayToAddrScript(addr string) ([]byte, error) {
	address, err := bchutil.DecodeAddress(addr, w.params())
	if err != nil {
		return nil, err
	}
	return txscript.PayToAddrScript(address)
}

// IsDust returns whether the amount passed in is considered dust by network. This
// method is called when building payout transactions from the multisig to the various
// participants. If the amount that is supposed to be sent to a given party is below
// the dust threshold, openbazaar-go will not pay that party to avoid building a transaction
// that never confirms.
func (w *BitcoinCashWallet) IsDust(iaddr iwallet.Address, amount iwallet.Amount) bool {
	return txrules.IsDustAmount(bchutil.Amount(amount.Int64()), 25, txrules.DefaultRelayFeePerKb)
}

// EstimateEscrowFee estimates the fee to release the funds from escrow.
// this assumes only one input. If there are more inputs Mobazha will
// will add 50% of the returned fee for each additional input. This is a
// crude fee calculating but it simplifies things quite a bit.
func (w *BitcoinCashWallet) EstimateEscrowFee(threshold int, nOuts int, level iwallet.FeeLevel) (iwallet.Amount, error) {
	var (
		redeemScriptSize = 4 + (threshold+1)*34
	)

	// 8 additional bytes are for version and locktime
	size := 8 + wire.VarIntSerializeSize(1) +
		wire.VarIntSerializeSize(uint64(nOuts)) + 1 +
		threshold*66 + txsizes.P2PKHOutputSize*nOuts + redeemScriptSize

	fpb, err := w.GetFeePerByte(level)
	if err != nil {
		return iwallet.NewAmount(0), err
	}
	return fpb.Mul(iwallet.NewAmount(size)), nil
}

// GetFeePerByte returns the current fee per byte for the given fee level. There
// are three fee levels ― priority, normal, and economic.
//
// The returned value should be in the coin's base unit (for example: satoshis).
func (w *BitcoinCashWallet) GetFeePerByte(feeLevel iwallet.FeeLevel) (iwallet.Amount, error) {
	resp, err := w.ChainClient.EstimateFee(1)
	if err != nil {
		return iwallet.NewAmount(0), err
	}
	return resp[feeLevel].FeePerTx, nil
}

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
func (w *BitcoinCashWallet) CreateMultisigAddress(keys []btcec.PublicKey, chaincode []byte, threshold int) (iwallet.Address, []byte, error) {
	if len(keys) < threshold {
		return iwallet.Address{}, nil, fmt.Errorf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", threshold, len(keys))
	}

	if len(keys) > 8 {
		return iwallet.Address{}, nil, fmt.Errorf("unable to generate multisig script with " +
			"more than 8 public keys")
	}

	builder := txscript.NewScriptBuilder()
	builder.AddInt64(int64(threshold))
	for _, key := range keys {
		builder.AddData(key.SerializeCompressed())
	}
	builder.AddInt64(int64(len(keys)))
	builder.AddOp(txscript.OP_CHECKMULTISIG)

	redeemScript, err := builder.Script()
	if err != nil {
		return iwallet.Address{}, nil, err
	}
	addr, err := bchutil.NewAddressScriptHash(redeemScript, w.params())
	if err != nil {
		return iwallet.Address{}, nil, err
	}
	return iwallet.NewAddress(addr.String(), w.CoinType), redeemScript, nil
}

// SignMultisigTransaction should use the provided key to create a signature for
// the multisig transaction. Since this a threshold signature this function will
// separately by each party signing this transaction. The resulting signatures
// will be shared between the relevant parties and one of them will aggregate
// the signatures into a transaction for broadcast.
//
// For coins like bitcoin you may need to return one signature *per input* which is
// why a slice of signatures is returned.
func (w *BitcoinCashWallet) SignMultisigTransaction(txn iwallet.Transaction, key btcec.PrivateKey, redeemScript []byte) ([]iwallet.EscrowSignature, error) {
	var sigs []iwallet.EscrowSignature
	tx := wire.NewMsgTx(1)
	for _, from := range txn.From {
		op := wire.OutPoint{}
		if err := op.Deserialize(bytes.NewReader(from.ID)); err != nil {
			return nil, err
		}

		input := wire.NewTxIn(&op, nil)
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, to := range txn.To {
		scriptPubkey, err := w.getPayToAddrScript(to.Address.String())
		if err != nil {
			return nil, err
		}
		output := wire.NewTxOut(to.Amount.Int64(), scriptPubkey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	privKey, _ := bchec.PrivKeyFromBytes(bchec.S256(), key.Serialize())

	for i := range tx.TxIn {
		sig, err := txscript.RawTxInSchnorrSignature(tx, i, redeemScript, txscript.SigHashAll, privKey, txn.From[i].Amount.Int64())
		if err != nil {
			return nil, err
		}
		bs := iwallet.EscrowSignature{Index: i, Signature: sig[:64]}
		sigs = append(sigs, bs)
	}
	return sigs, nil
}

// BuildAndSend should used the passed in signatures to build the transaction.
// Note the signatures are a slice of slices. This is because coins like Bitcoin
// may require one signature *per input*. In this case the outer slice is the
// signatures from the different key holders and the inner slice is the keys
// per input.
//
// Note a database transaction is used here. Same rules of Commit() and
// Rollback() apply.
func (w *BitcoinCashWallet) BuildAndSend(wtx iwallet.Tx, txn iwallet.Transaction, signatures [][]iwallet.EscrowSignature, redeemScript []byte, finishType iwallet.OrderFinishType) (iwallet.TransactionID, error) {
	tx := wire.NewMsgTx(1)
	for _, from := range txn.From {
		op := wire.OutPoint{}
		if err := op.Deserialize(bytes.NewReader(from.ID)); err != nil {
			return iwallet.TransactionID(""), err
		}
		input := wire.NewTxIn(&op, nil)
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, to := range txn.To {
		scriptPubkey, err := w.getPayToAddrScript(to.Address.String())
		if err != nil {
			return iwallet.TransactionID(""), err
		}
		output := wire.NewTxOut(to.Amount.Int64(), scriptPubkey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	// Check if time locked
	var timeLocked bool
	if redeemScript[0] == txscript.OP_IF {
		timeLocked = true
	}

	elems, err := txscript.ExtractDataElements(redeemScript)
	if err != nil {
		return iwallet.TransactionID(""), err
	}

	var pubkeys []*bchec.PublicKey
	for _, elem := range elems {
		pubkey, err := bchec.ParsePubKey(elem, bchec.S256())
		if err == nil {
			pubkeys = append(pubkeys, pubkey)
		}
	}

	if len(pubkeys) > 8 {
		return iwallet.TransactionID(""), errors.New("too many pubkeys in redeem script")
	}

	for i := range tx.TxIn {
		// The primary challenge for us here is matching signatures with public keys from
		// the redeem script. The Bitcoin Cash schnorr signature specification requires
		// that we enumerate the indexes of the public keys for which we are providing a
		// signature. To do this we will validate the signature against the public keys
		// to figure out the key index.
		var (
			parsedSigs []*bchec.Signature
			escrowSigs []iwallet.EscrowSignature
		)

		for _, indexSig := range signatures {
			for _, sig := range indexSig {
				if sig.Index == i {
					escrowSigs = append(escrowSigs, sig)
					break
				}
			}
		}

		for _, sig := range escrowSigs {
			parsedSig, err := bchec.ParseSchnorrSignature(sig.Signature)
			if err != nil {
				return iwallet.TransactionID(""), err
			}
			parsedSigs = append(parsedSigs, parsedSig)
		}

		pubkeyIndexes := make([]int, 0, len(parsedSigs))

		sigHash, err := txscript.CalcSignatureHash(redeemScript, txscript.NewTxSigHashes(tx), txscript.SigHashAll|txscript.SigHashForkID, tx, i, txn.From[i].Amount.Int64(), true)
		if err != nil {
			return iwallet.TransactionID(""), err
		}

		for _, parsedSig := range parsedSigs {
			for i, key := range pubkeys {
				if parsedSig.Verify(sigHash, key) {
					pubkeyIndexes = append(pubkeyIndexes, i)
					break
				}
			}
		}

		if len(pubkeyIndexes) != len(parsedSigs) {
			return iwallet.TransactionID(""), errors.New("signatures do not match public keys")
		}

		var (
			dummy = make([]byte, 1)
			mask  = 0x80
		)
		for _, idx := range pubkeyIndexes {
			dummy[0] |= byte(mask >> uint(8-(idx+1)))
		}

		builder := txscript.NewScriptBuilder()
		builder.AddData(dummy)
		for _, sig := range escrowSigs {
			builder.AddData(append(sig.Signature, byte(txscript.SigHashAll|txscript.SigHashForkID)))
		}

		if timeLocked {
			builder.AddOp(txscript.OP_1)
		}

		builder.AddData(redeemScript)
		scriptSig, err := builder.Script()
		if err != nil {
			return iwallet.TransactionID(""), err
		}
		tx.TxIn[i].SignatureScript = scriptSig
	}

	txid := iwallet.TransactionID(tx.TxHash().String())

	var buf bytes.Buffer
	if err := tx.BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		return txid, err
	}

	wbtx, ok := wtx.(*base.DBTx)
	if !ok {
		return txid, errors.New("tx is not expected type")
	}

	wbtx.OnCommit = func() error {
		return w.DB.Update(func(dbtx database.Tx) error {
			err := dbtx.Save(&database.UnconfirmedTransaction{
				Timestamp: time.Now(),
				Coin:      w.CoinType.String(),
				TxBytes:   buf.Bytes(),
				Txid:      tx.TxHash().String(),
			})
			if err != nil {
				return err
			}
			return w.ChainClient.Broadcast(buf.Bytes())
		})
	}

	return txid, nil
}

// CreateMultisigWithTimeout is the same as CreateMultisigAddress but it adds
// an additional timeout to the address. The address should have two ways to
// release the funds:
//   - m of n signatures are provided (or)
//   - timeout has passed and a signature for timeoutKey is provided.
func (w *BitcoinCashWallet) CreateMultisigWithTimeout(keys []btcec.PublicKey, chaincode []byte, threshold int, timeout time.Duration, timeoutKey btcec.PublicKey) (iwallet.Address, []byte, error) {
	if len(keys) < threshold {
		return iwallet.Address{}, nil, fmt.Errorf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", threshold, len(keys))
	}
	if len(keys) > 8 {
		return iwallet.Address{}, nil, fmt.Errorf("unable to generate multisig script with " +
			"more than 8 public keys")
	}

	builder := txscript.NewScriptBuilder()
	sequenceLock := blockchain.LockTimeToSequence(false, uint32(timeout.Hours()*6))
	builder.AddOp(txscript.OP_IF)
	builder.AddInt64(int64(threshold))
	for _, key := range keys {
		builder.AddData(key.SerializeCompressed())
	}
	builder.AddInt64(int64(len(keys)))
	builder.AddOp(txscript.OP_CHECKMULTISIG)
	builder.AddOp(txscript.OP_ELSE).
		AddInt64(int64(sequenceLock)).
		AddOp(txscript.OP_CHECKSEQUENCEVERIFY).
		AddOp(txscript.OP_DROP).
		AddData(timeoutKey.SerializeCompressed()).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ENDIF)

	redeemScript, err := builder.Script()
	if err != nil {
		return iwallet.Address{}, nil, err
	}
	addr, err := bchutil.NewAddressScriptHash(redeemScript, w.params())
	if err != nil {
		return iwallet.Address{}, nil, err
	}
	return iwallet.NewAddress(addr.String(), w.CoinType), redeemScript, nil
}

// ReleaseFundsAfterTimeout will release funds from the escrow. The signature will
// be created using the timeoutKey.
func (w *BitcoinCashWallet) ReleaseFundsAfterTimeout(wtx iwallet.Tx, txn iwallet.Transaction, timeoutKey btcec.PrivateKey, redeemScript []byte, finishType iwallet.OrderFinishType) (iwallet.TransactionID, error) {
	tx := wire.NewMsgTx(2)
	for _, from := range txn.From {
		op := wire.OutPoint{}
		if err := op.Deserialize(bytes.NewReader(from.ID)); err != nil {
			return iwallet.TransactionID(""), err
		}
		input := wire.NewTxIn(&op, nil)
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, to := range txn.To {
		scriptPubkey, err := w.getPayToAddrScript(to.Address.String())
		if err != nil {
			return iwallet.TransactionID(""), err
		}
		output := wire.NewTxOut(to.Amount.Int64(), scriptPubkey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	privKey, _ := bchec.PrivKeyFromBytes(bchec.S256(), timeoutKey.Serialize())

	locktime, err := lockTimeFromRedeemScript(redeemScript)
	if err != nil {
		return iwallet.TransactionID(""), err
	}
	for i := range tx.TxIn {
		tx.TxIn[i].Sequence = locktime
	}

	for i := range tx.TxIn {
		sig, err := txscript.RawTxInSchnorrSignature(tx, i, redeemScript, txscript.SigHashAll, privKey, txn.From[i].Amount.Int64())
		if err != nil {
			return iwallet.TransactionID(""), err
		}
		builder := txscript.NewScriptBuilder()
		builder.AddData(sig)
		builder.AddOp(txscript.OP_0)
		builder.AddData(redeemScript)
		scriptSig, err := builder.Script()
		if err != nil {
			return iwallet.TransactionID(""), err
		}
		tx.TxIn[i].SignatureScript = scriptSig
	}

	txid := iwallet.TransactionID(tx.TxHash().String())

	var buf bytes.Buffer
	if err := tx.BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		return txid, err
	}

	wbtx, ok := wtx.(*base.DBTx)
	if !ok {
		return txid, errors.New("tx is not expected type")
	}

	wbtx.OnCommit = func() error {
		return w.DB.Update(func(dbtx database.Tx) error {
			err := dbtx.Save(&database.UnconfirmedTransaction{
				Timestamp: time.Now(),
				Coin:      w.CoinType.String(),
				TxBytes:   buf.Bytes(),
				Txid:      tx.TxHash().String(),
			})
			if err != nil {
				return err
			}
			return w.ChainClient.Broadcast(buf.Bytes())
		})
	}

	return txid, nil
}

func (w *BitcoinCashWallet) params() *chaincfg.Params {
	if w.testnet {
		return &chaincfg.TestNet3Params
	}
	return &chaincfg.MainNetParams
}

func (w *BitcoinCashWallet) postInit(masterKey *btchd.ExtendedKey) error {
	return nil
}

func lockTimeFromRedeemScript(redeemScript []byte) (uint32, error) {
	if len(redeemScript) < 113 {
		return 0, errors.New("redeem script invalid length")
	}
	if redeemScript[106] != 103 {
		return 0, errors.New("invalid redeem script")
	}
	if redeemScript[107] == 0 {
		return 0, nil
	}
	if 81 <= redeemScript[107] && redeemScript[107] <= 96 {
		return uint32((redeemScript[107] - 81) + 1), nil
	}
	var v []byte
	op := redeemScript[107]
	if 1 <= op && op <= 75 {
		for i := 0; i < int(op); i++ {
			v = append(v, []byte{redeemScript[108+i]}...)
		}
	} else {
		return 0, errors.New("too many bytes pushed for sequence")
	}
	var result int64
	for i, val := range v {
		result |= int64(val) << uint8(8*i)
	}

	return uint32(result), nil
}

// SpendFromDerivedAddress spends funds from an HD-derived address (identified by utxo)
// to multiple outputs using a single private key.
// Note: DIRECT payment mode has been removed. This method is retained for potential future use.
//
// Network fee handling: In UTXO model, fee = inputs - outputs.
// The caller must pre-calculate outputs to leave the desired fee as the difference.
// This function does NOT calculate or deduct fees - it uses exact output amounts provided.
func (w *BitcoinCashWallet) SpendFromDerivedAddress(wtx iwallet.Tx, utxo iwallet.UTXO, outputs []iwallet.SpendInfo, signingKey btcec.PrivateKey, _ iwallet.FeeLevel) (iwallet.TransactionID, error) {
	// Build the transaction
	tx := wire.NewMsgTx(wire.TxVersion)

	// Add the input
	txidHash, err := chainhash.NewHashFromStr(string(utxo.TxID))
	if err != nil {
		return iwallet.TransactionID(""), fmt.Errorf("invalid txid: %w", err)
	}
	outpoint := wire.NewOutPoint(txidHash, utxo.OutputIndex)
	txIn := wire.NewTxIn(outpoint, nil)
	tx.TxIn = append(tx.TxIn, txIn)

	// Calculate total output amount
	var totalOutputAmt int64
	for _, out := range outputs {
		totalOutputAmt += out.Amount.Int64()
	}

	// Verify: outputs must be less than input (difference becomes network fee)
	inputAmount := utxo.Amount.Int64()
	implicitFee := inputAmount - totalOutputAmt
	if implicitFee < 0 {
		return iwallet.TransactionID(""), fmt.Errorf("outputs exceed input: input=%d, outputs=%d", inputAmount, totalOutputAmt)
	}
	if implicitFee == 0 {
		return iwallet.TransactionID(""), fmt.Errorf("zero fee transaction not allowed")
	}

	// Add outputs
	for _, out := range outputs {
		if out.Amount.Int64() <= 0 {
			continue
		}
		scriptPubKey, err := w.getPayToAddrScript(out.Address.String())
		if err != nil {
			return iwallet.TransactionID(""), fmt.Errorf("failed to get scriptPubKey for %s: %w", out.Address, err)
		}
		txOut := wire.NewTxOut(out.Amount.Int64(), scriptPubKey)
		tx.TxOut = append(tx.TxOut, txOut)
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	// Sign the input using BCH's SigHashForkID
	pubKey := signingKey.PubKey()
	pubKeyHash := bchutil.Hash160(pubKey.SerializeCompressed())

	// Create the P2PKH script for signing
	sigScript, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_DUP).
		AddOp(txscript.OP_HASH160).
		AddData(pubKeyHash).
		AddOp(txscript.OP_EQUALVERIFY).
		AddOp(txscript.OP_CHECKSIG).
		Script()
	if err != nil {
		return iwallet.TransactionID(""), fmt.Errorf("failed to create sig script: %w", err)
	}

	// Calculate signature hash with ForkID
	sigHashes := txscript.NewTxSigHashes(tx)
	sigHash, err := txscript.CalcSignatureHash(sigScript, sigHashes, txscript.SigHashAll|txscript.SigHashForkID, tx, 0, inputAmount, true)
	if err != nil {
		return iwallet.TransactionID(""), fmt.Errorf("failed to calculate sig hash: %w", err)
	}

	// Convert to bchec key and sign
	bchPrivKey, _ := bchec.PrivKeyFromBytes(bchec.S256(), signingKey.Serialize())
	sig, err := bchPrivKey.SignECDSA(sigHash)
	if err != nil {
		return iwallet.TransactionID(""), fmt.Errorf("failed to sign: %w", err)
	}

	// Build the signature script (P2PKH)
	builder := txscript.NewScriptBuilder()
	builder.AddData(append(sig.Serialize(), byte(txscript.SigHashAll|txscript.SigHashForkID)))
	builder.AddData(pubKey.SerializeCompressed())
	sigScriptFinal, err := builder.Script()
	if err != nil {
		return iwallet.TransactionID(""), fmt.Errorf("failed to build sig script: %w", err)
	}

	tx.TxIn[0].SignatureScript = sigScriptFinal

	txid := iwallet.TransactionID(tx.TxHash().String())

	// Serialize the transaction
	var buf bytes.Buffer
	if err := tx.BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		return txid, fmt.Errorf("failed to encode transaction: %w", err)
	}

	// Broadcast via OnCommit callback
	wbtx, ok := wtx.(*base.DBTx)
	if !ok {
		return txid, errors.New("tx is not expected type")
	}

	wbtx.OnCommit = func() error {
		return w.DB.Update(func(dbtx database.Tx) error {
			err := dbtx.Save(&database.UnconfirmedTransaction{
				Timestamp: time.Now(),
				Coin:      w.CoinType.String(),
				TxBytes:   buf.Bytes(),
				Txid:      tx.TxHash().String(),
			})
			if err != nil {
				return err
			}
			return w.ChainClient.Broadcast(buf.Bytes())
		})
	}

	return txid, nil
}
