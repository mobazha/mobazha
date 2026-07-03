package zcash

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	btc "github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/btcutil/txsort"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	mbwire "github.com/martinboehm/btcd/wire"
	"github.com/martinboehm/btcutil"
	"github.com/martinboehm/btcutil/chaincfg"
	"github.com/martinboehm/btcutil/txscript"
	"github.com/minio/blake2b-simd"
	"github.com/mobazha/mobazha/internal/chains/base"
	chainutxo "github.com/mobazha/mobazha/internal/chains/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

var (
	txHeaderBytes          = []byte{0x04, 0x00, 0x00, 0x80}
	txNVersionGroupIDBytes = []byte{0x85, 0x20, 0x2f, 0x89}

	hashPrevOutPersonalization  = []byte("ZcashPrevoutHash")
	hashSequencePersonalization = []byte("ZcashSequencHash")
	hashOutputsPersonalization  = []byte("ZcashOutputsHash")
	sigHashPersonalization      = []byte("ZcashSigHash")

	// MainNetParams are parser parameters for mainnet
	MainNetParams chaincfg.Params
	// TestNetParams are parser parameters for testnet
	TestNetParams chaincfg.Params
)

const (
	sigHashMask     = 0x1f
	blossomBranchID = 0x2BB40E60

	// MainnetMagic is mainnet network constant
	MainnetMagic mbwire.BitcoinNet = 0x6427e924
	// TestnetMagic is testnet network constant
	TestnetMagic mbwire.BitcoinNet = 0xbff91afa
)

// Assert interfaces
var _ = iwallet.Wallet(&ZCashWallet{})

var _ = iwallet.UTXOEscrow(&ZCashWallet{})
var _ = iwallet.UTXOSweeper(&ZCashWallet{})
var _ iwallet.UTXOAddressUtilities = (*ZCashWallet)(nil)

func init() {
	MainNetParams = chaincfg.MainNetParams
	MainNetParams.Net = MainnetMagic

	// Address encoding magics
	MainNetParams.AddressMagicLen = 2
	MainNetParams.PubKeyHashAddrID = []byte{0x1C, 0xB8} // base58 prefix: t1
	MainNetParams.ScriptHashAddrID = []byte{0x1C, 0xBD} // base58 prefix: t3

	TestNetParams = chaincfg.TestNet3Params
	TestNetParams.Net = TestnetMagic

	// Address encoding magics
	TestNetParams.AddressMagicLen = 2
	TestNetParams.PubKeyHashAddrID = []byte{0x1D, 0x25} // base58 prefix: tm
	TestNetParams.ScriptHashAddrID = []byte{0x1C, 0xBA} // base58 prefix: t2
}

// ZCashWallet extends wallet base and implements the
// remaining functions for each interface.
type ZCashWallet struct { // nolint
	base.WalletBase
	testnet bool
}

// NewZCashWallet returns a new ZCashWallet.
// ChainClient is not created here — it will be injected later via SetChainClient()
// using the shared UTXOChainClient backed by Electrum/Mempool Monitor.
func NewZCashWallet(cfg *base.WalletConfig) (*ZCashWallet, error) {
	w := &ZCashWallet{
		testnet: cfg.Testnet,
	}
	w.Init()

	// ChainClient intentionally left nil — will be set by configureUTXOWallets()
	// via SetChainClient() with a shared UTXOChainClient (Electrum/Mempool).

	w.KeyStore = cfg.KeyStore
	w.Logger = cfg.Logger
	nativeCoin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainZCash)
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
func (w *ZCashWallet) ValidateAddress(addr iwallet.Address) error {
	_, err := w.getPayToAddrScript(addr.String())
	return err
}

func (w *ZCashWallet) getPayToAddrScript(addr string) ([]byte, error) {
	address, err := btcutil.DecodeAddress(addr, w.params())
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
func (w *ZCashWallet) IsDust(iaddr iwallet.Address, amount iwallet.Amount) bool {
	// Check for dust
	script, err := w.getPayToAddrScript(iaddr.String())
	if err != nil {
		return true
	}

	output := wire.NewTxOut(amount.Int64(), script)
	return txrules.IsDustOutput(output, txrules.DefaultRelayFeePerKb)
}

// EstimateEscrowFee estimates the fee to release the funds from escrow.
// this assumes only one input. If there are more inputs Mobazha will
// will add 50% of the returned fee for each additional input. This is a
// crude fee calculating but it simplifies things quite a bit.
func (w *ZCashWallet) EstimateEscrowFee(nInputs int, threshold int, nOuts int, level iwallet.FeeLevel) (iwallet.Amount, error) {
	size := chainutxo.EstimateP2SHECDSAMultisigSpendRelaySize(nInputs, threshold, nOuts)

	resp, err := w.ChainClient.EstimateFee(size)
	if err != nil {
		return iwallet.NewAmount(0), err
	}
	return resp[level].FeePerTx, nil
}

// GetFeePerByte returns the current fee per byte for the given fee level. There
// are three fee levels ― priority, normal, and economic.
//
// The returned value should be in the coin's base unit (for example: satoshis).
func (w *ZCashWallet) GetFeePerByte(feeLevel iwallet.FeeLevel) (iwallet.Amount, error) {
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
func (w *ZCashWallet) CreateMultisigAddress(keys []btcec.PublicKey, chaincode []byte, threshold int) (iwallet.Address, []byte, error) {
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
	addr, err := btcutil.NewAddressScriptHash(redeemScript, w.params())
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
func (w *ZCashWallet) SignMultisigTransaction(txn iwallet.Transaction, key btcec.PrivateKey, redeemScript []byte) ([]iwallet.EscrowSignature, error) {
	var sigs []iwallet.EscrowSignature
	tx := wire.NewMsgTx(1)
	for _, from := range txn.From {
		op, err := derializeOutpoint(from.ID)
		if err != nil {
			return nil, err
		}

		input := wire.NewTxIn(op, nil, nil)
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

	blockchainInfo, err := w.BlockchainInfo()
	if err != nil {
		return nil, err
	}
	for i := range tx.TxIn {
		sig, err := rawTxInSignature(tx, i, redeemScript, txscript.SigHashAll, &key, txn.From[i].Amount.Int64(), blockchainInfo.Height)
		if err != nil {
			return nil, err
		}
		bs := iwallet.EscrowSignature{Index: i, Signature: sig[:len(sig)-1]}
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
func (w *ZCashWallet) BuildAndSend(wtx iwallet.Tx, txn iwallet.Transaction, signatures [][]iwallet.EscrowSignature, redeemScript []byte, finishType iwallet.OrderFinishType) (iwallet.TransactionID, error) {
	tx := wire.NewMsgTx(1)
	for _, from := range txn.From {
		op, err := derializeOutpoint(from.ID)
		if err != nil {
			return iwallet.TransactionID(""), err
		}
		input := wire.NewTxIn(op, nil, nil)
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

	for _, sig := range signatures {
		if len(sig) != len(tx.TxIn) {
			return iwallet.TransactionID(""), errors.New("incorrect number of signatures")
		}
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	for i := range tx.TxIn {
		var sigs [][]byte
		for _, escrowSigs := range signatures {
			for _, sig := range escrowSigs {
				if sig.Index == i {
					sigs = append(sigs, append(sig.Signature, byte(txscript.SigHashAll)))
					break
				}
			}
		}

		builder := txscript.NewScriptBuilder()
		builder.AddOp(txscript.OP_0)
		for _, sig := range sigs {
			builder.AddData(sig)
		}

		builder.AddData(redeemScript)
		scriptSig, err := builder.Script()
		if err != nil {
			return iwallet.TransactionID(""), err
		}
		tx.TxIn[i].SignatureScript = scriptSig
	}

	txid := iwallet.TransactionID(tx.TxHash().String())

	buf, err := serializeVersion4Transaction(tx, 0)
	if err != nil {
		return txid, err
	}

	wbtx, ok := wtx.(*base.DBTx)
	if !ok {
		return txid, errors.New("tx is not expected type")
	}

	wbtx.OnCommit = func() error {
		return w.ChainClient.Broadcast(buf)
	}

	return txid, nil
}

func (w *ZCashWallet) params() *chaincfg.Params {
	if w.testnet {
		if !chaincfg.IsRegistered(&TestNetParams) {
			chaincfg.Register(&TestNetParams)
		}
		return &TestNetParams
	}
	if !chaincfg.IsRegistered(&MainNetParams) {
		chaincfg.Register(&MainNetParams)
	}
	return &MainNetParams
}

func (w *ZCashWallet) postInit(masterKey *hdkeychain.ExtendedKey) error {
	return nil
}

func derializeOutpoint(ser []byte) (*wire.OutPoint, error) {
	h, err := chainhash.NewHash(ser[:32])
	if err != nil {
		return nil, err
	}
	return wire.NewOutPoint(h, binary.LittleEndian.Uint32(ser[32:])), nil
}

func serializeOutpoint(op *wire.OutPoint) []byte {
	i := make([]byte, 4)
	binary.LittleEndian.PutUint32(i, op.Index)
	return append(op.Hash[:], i...)
}

// rawTxInSignature returns the serialized ECDSA signature for the input idx of
// the given transaction, with hashType appended to it.
func rawTxInSignature(tx *wire.MsgTx, idx int, prevScriptBytes []byte,
	hashType txscript.SigHashType, key *btcec.PrivateKey, amt int64, currentHeight uint64) ([]byte, error) {

	hash, err := calcSignatureHash(prevScriptBytes, hashType, tx, idx, amt, 0, currentHeight)
	if err != nil {
		return nil, err
	}
	signature := ecdsa.Sign(key, hash)

	return append(signature.Serialize(), byte(hashType)), nil
}

func calcSignatureHash(prevScriptBytes []byte, hashType txscript.SigHashType, tx *wire.MsgTx, idx int, amt int64, expiry uint32, currentHeight uint64) ([]byte, error) {

	// As a sanity check, ensure the passed input index for the transaction
	// is valid.
	if idx > len(tx.TxIn)-1 {
		return nil, fmt.Errorf("idx %d but %d txins", idx, len(tx.TxIn))
	}

	// We'll utilize this buffer throughout to incrementally calculate
	// the signature hash for this transaction.
	var sigHash bytes.Buffer

	// Write header
	_, err := sigHash.Write(txHeaderBytes)
	if err != nil {
		return nil, err
	}

	// Write group ID
	_, err = sigHash.Write(txNVersionGroupIDBytes)
	if err != nil {
		return nil, err
	}

	// Next write out the possibly pre-calculated hashes for the sequence
	// numbers of all inputs, and the hashes of the previous outs for all
	// outputs.
	var zeroHash chainhash.Hash

	// If anyone can pay isn't active, then we can use the cached
	// hashPrevOuts, otherwise we just write zeroes for the prev outs.
	if hashType&txscript.SigHashAnyOneCanPay == 0 {
		sigHash.Write(calcHashPrevOuts(tx))
	} else {
		sigHash.Write(zeroHash[:])
	}

	// If the sighash isn't anyone can pay, single, or none, the use the
	// cached hash sequences, otherwise write all zeroes for the
	// hashSequence.
	if hashType&txscript.SigHashAnyOneCanPay == 0 &&
		hashType&sigHashMask != txscript.SigHashSingle &&
		hashType&sigHashMask != txscript.SigHashNone {
		sigHash.Write(calcHashSequence(tx))
	} else {
		sigHash.Write(zeroHash[:])
	}

	// If the current signature mode isn't single, or none, then we can
	// re-use the pre-generated hashoutputs sighash fragment. Otherwise,
	// we'll serialize and add only the target output index to the signature
	// pre-image.
	if hashType&sigHashMask != txscript.SigHashSingle &&
		hashType&sigHashMask != txscript.SigHashNone {
		sigHash.Write(calcHashOutputs(tx))
	} else if hashType&sigHashMask == txscript.SigHashSingle && idx < len(tx.TxOut) {
		var b bytes.Buffer
		wire.WriteTxOut(&b, 0, 0, tx.TxOut[idx])
		sigHash.Write(chainhash.DoubleHashB(b.Bytes()))
	} else {
		sigHash.Write(zeroHash[:])
	}

	// Write hash JoinSplits
	sigHash.Write(make([]byte, 32))

	// Write hash ShieldedSpends
	sigHash.Write(make([]byte, 32))

	// Write hash ShieldedOutputs
	sigHash.Write(make([]byte, 32))

	// Write out the transaction's locktime, and the sig hash
	// type.
	var bLockTime [4]byte
	binary.LittleEndian.PutUint32(bLockTime[:], tx.LockTime)
	sigHash.Write(bLockTime[:])

	// Write expiry
	var bExpiryTime [4]byte
	binary.LittleEndian.PutUint32(bExpiryTime[:], expiry)
	sigHash.Write(bExpiryTime[:])

	// Write valueblance
	sigHash.Write(make([]byte, 8))

	// Write the hash type
	var bHashType [4]byte
	binary.LittleEndian.PutUint32(bHashType[:], uint32(hashType))
	sigHash.Write(bHashType[:])

	// Next, write the outpoint being spent.
	sigHash.Write(tx.TxIn[idx].PreviousOutPoint.Hash[:])
	var bIndex [4]byte
	binary.LittleEndian.PutUint32(bIndex[:], tx.TxIn[idx].PreviousOutPoint.Index)
	sigHash.Write(bIndex[:])

	// Write the previous script bytes
	wire.WriteVarBytes(&sigHash, 0, prevScriptBytes)

	// Next, add the input amount, and sequence number of the input being
	// signed.
	var bAmount [8]byte
	binary.LittleEndian.PutUint64(bAmount[:], uint64(amt))
	sigHash.Write(bAmount[:])
	var bSequence [4]byte
	binary.LittleEndian.PutUint32(bSequence[:], tx.TxIn[idx].Sequence)
	sigHash.Write(bSequence[:])

	branchID := selectBranchID(currentHeight)
	leBranchID := make([]byte, 4)
	binary.LittleEndian.PutUint32(leBranchID, branchID)
	bl, _ := blake2b.New(&blake2b.Config{
		Size:   32,
		Person: append(sigHashPersonalization, leBranchID...),
	})
	bl.Write(sigHash.Bytes())
	h := bl.Sum(nil)
	return h[:], nil
}

// serializeVersion4Transaction serializes a wire.MsgTx into the zcash version four
// wire transaction format.
func serializeVersion4Transaction(tx *wire.MsgTx, expiryHeight uint32) ([]byte, error) {
	var buf bytes.Buffer

	// Write header
	_, err := buf.Write(txHeaderBytes)
	if err != nil {
		return nil, err
	}

	// Write group ID
	_, err = buf.Write(txNVersionGroupIDBytes)
	if err != nil {
		return nil, err
	}

	// Write varint input count
	count := uint64(len(tx.TxIn))
	err = wire.WriteVarInt(&buf, wire.ProtocolVersion, count)
	if err != nil {
		return nil, err
	}

	// Write inputs
	for _, ti := range tx.TxIn {
		// Write outpoint hash
		_, err := buf.Write(ti.PreviousOutPoint.Hash[:])
		if err != nil {
			return nil, err
		}
		// Write outpoint index
		index := make([]byte, 4)
		binary.LittleEndian.PutUint32(index, ti.PreviousOutPoint.Index)
		_, err = buf.Write(index)
		if err != nil {
			return nil, err
		}
		// Write sigscript
		err = wire.WriteVarBytes(&buf, wire.ProtocolVersion, ti.SignatureScript)
		if err != nil {
			return nil, err
		}
		// Write sequence
		sequence := make([]byte, 4)
		binary.LittleEndian.PutUint32(sequence, ti.Sequence)
		_, err = buf.Write(sequence)
		if err != nil {
			return nil, err
		}
	}
	// Write varint output count
	count = uint64(len(tx.TxOut))
	err = wire.WriteVarInt(&buf, wire.ProtocolVersion, count)
	if err != nil {
		return nil, err
	}
	// Write outputs
	for _, to := range tx.TxOut {
		// Write value
		val := make([]byte, 8)
		binary.LittleEndian.PutUint64(val, uint64(to.Value))
		_, err = buf.Write(val)
		if err != nil {
			return nil, err
		}
		// Write pkScript
		err = wire.WriteVarBytes(&buf, wire.ProtocolVersion, to.PkScript)
		if err != nil {
			return nil, err
		}
	}
	// Write nLocktime
	nLockTime := make([]byte, 4)
	binary.LittleEndian.PutUint32(nLockTime, tx.LockTime)
	_, err = buf.Write(nLockTime)
	if err != nil {
		return nil, err
	}

	// Write nExpiryHeight
	expiry := make([]byte, 4)
	binary.LittleEndian.PutUint32(expiry, expiryHeight)
	_, err = buf.Write(expiry)
	if err != nil {
		return nil, err
	}

	// Write nil value balance
	_, err = buf.Write(make([]byte, 8))
	if err != nil {
		return nil, err
	}

	// Write nil value vShieldedSpend
	_, err = buf.Write(make([]byte, 1))
	if err != nil {
		return nil, err
	}

	// Write nil value vShieldedOutput
	_, err = buf.Write(make([]byte, 1))
	if err != nil {
		return nil, err
	}

	// Write nil value vJoinSplit
	_, err = buf.Write(make([]byte, 1))
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func calcHashPrevOuts(tx *wire.MsgTx) []byte {
	var b bytes.Buffer
	for _, in := range tx.TxIn {
		// First write out the 32-byte transaction ID one of whose
		// outputs are being referenced by this input.
		b.Write(in.PreviousOutPoint.Hash[:])

		// Next, we'll encode the index of the referenced output as a
		// little endian integer.
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], in.PreviousOutPoint.Index)
		b.Write(buf[:])
	}
	bl, _ := blake2b.New(&blake2b.Config{
		Size:   32,
		Person: hashPrevOutPersonalization,
	})
	bl.Write(b.Bytes())
	h := bl.Sum(nil)
	return h[:]
}

func calcHashSequence(tx *wire.MsgTx) []byte {
	var b bytes.Buffer
	for _, in := range tx.TxIn {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], in.Sequence)
		b.Write(buf[:])
	}
	bl, _ := blake2b.New(&blake2b.Config{
		Size:   32,
		Person: hashSequencePersonalization,
	})
	bl.Write(b.Bytes())
	h := bl.Sum(nil)
	return h[:]
}

func calcHashOutputs(tx *wire.MsgTx) []byte {
	var b bytes.Buffer
	for _, out := range tx.TxOut {
		wire.WriteTxOut(&b, 0, 0, out)
	}
	bl, _ := blake2b.New(&blake2b.Config{
		Size:   32,
		Person: hashOutputsPersonalization,
	})
	bl.Write(b.Bytes())
	h := bl.Sum(nil)
	return h[:]
}

// https://github.com/zcash/zcash/blob/99ad6fdc3a549ab510422820eea5e5ce9f60a5fd/src/chainparams.cpp#L143
func selectBranchID(currentHeight uint64) uint32 {
	if currentHeight < 2726400 {
		// "NU5"
		return 0xc2d6d0b4
	}
	// "NU6"
	return 0xc8e71055
}

// BuildSweepTx builds and signs a P2PKH sweep transaction using ZCash's
// ZIP-243 signature hash that spends all provided inputs to a single destination.
func (w *ZCashWallet) BuildSweepTx(inputs []iwallet.SweepInput, signingKey btcec.PrivateKey, destAddress string, feePerByte int64) ([]byte, string, error) {
	if len(inputs) == 0 {
		return nil, "", errors.New("no inputs provided")
	}

	tx := wire.NewMsgTx(1)
	var totalInput int64

	for _, inp := range inputs {
		hash, err := chainhash.NewHashFromStr(inp.TxHash)
		if err != nil {
			return nil, "", fmt.Errorf("invalid txid %s: %w", inp.TxHash, err)
		}
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(hash, inp.OutputIndex), nil, nil))
		totalInput += inp.Value
	}
	if totalInput == 0 {
		return nil, "", errors.New("total input is zero")
	}

	// P2PKH: ~148 bytes per input, ~34 bytes for output + 10 overhead
	estimatedSize := int64(10 + len(inputs)*148 + 34)
	fee := estimatedSize * feePerByte
	if fee >= totalInput {
		return nil, "", fmt.Errorf("fee (%d) exceeds total input (%d)", fee, totalInput)
	}

	destScript, err := w.getPayToAddrScript(destAddress)
	if err != nil {
		return nil, "", fmt.Errorf("decode dest address: %w", err)
	}
	tx.AddTxOut(wire.NewTxOut(totalInput-fee, destScript))

	pubKey := signingKey.PubKey()
	pubKeyHash := btc.Hash160(pubKey.SerializeCompressed())
	prevScript, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_DUP).
		AddOp(txscript.OP_HASH160).
		AddData(pubKeyHash).
		AddOp(txscript.OP_EQUALVERIFY).
		AddOp(txscript.OP_CHECKSIG).
		Script()
	if err != nil {
		return nil, "", fmt.Errorf("build P2PKH script: %w", err)
	}

	chainInfo, _ := w.BlockchainInfo()
	currentHeight := chainInfo.Height

	for i, inp := range inputs {
		sig, sigErr := rawTxInSignature(tx, i, prevScript, txscript.SigHashAll, &signingKey, inp.Value, currentHeight)
		if sigErr != nil {
			return nil, "", fmt.Errorf("sign input %d: %w", i, sigErr)
		}

		builder := txscript.NewScriptBuilder()
		builder.AddData(sig)
		builder.AddData(pubKey.SerializeCompressed())
		sigScript, buildErr := builder.Script()
		if buildErr != nil {
			return nil, "", fmt.Errorf("build sig script input %d: %w", i, buildErr)
		}
		tx.TxIn[i].SignatureScript = sigScript
	}

	txBytes, err := serializeVersion4Transaction(tx, 0)
	if err != nil {
		return nil, "", fmt.Errorf("serialize tx: %w", err)
	}

	h := chainhash.DoubleHashH(txBytes)
	return txBytes, h.String(), nil
}

// DerivePaymentAddressFromPubKey derives a P2PKH transparent (t-address) payment
// address from a single public key. ZCash transparent addresses use the legacy
// base58check format with chain-specific 2-byte prefixes (`t1` mainnet, `tm`
// testnet for P2PKH).
func (w *ZCashWallet) DerivePaymentAddressFromPubKey(pubKey *btcec.PublicKey) (string, []byte, error) {
	if pubKey == nil {
		return "", nil, errors.New("public key cannot be nil")
	}

	pubKeyHash := btc.Hash160(pubKey.SerializeCompressed())

	addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, w.params())
	if err != nil {
		return "", nil, err
	}

	scriptPubKey, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return "", nil, err
	}

	return addr.EncodeAddress(), scriptPubKey, nil
}

// AddressToScriptPubKey decodes an encoded ZCash transparent address into its
// scriptPubKey.
func (w *ZCashWallet) AddressToScriptPubKey(address string) ([]byte, error) {
	return w.getPayToAddrScript(address)
}
