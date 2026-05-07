package zcash

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/martinboehm/btcutil/txscript"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	testInvalidAddress = "abc"
	testZECTestnetAddr = "tmJKrg3gS4sPS7gSJ4vT8dFeqkGtfnDW4gu"
)

var testZCashNativeCoin = func() iwallet.CoinType {
	coin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainZCash)
	if err != nil {
		panic(err)
	}
	return coin
}()

func newTestWallet() (*ZCashWallet, error) {
	w := &ZCashWallet{
		testnet: true,
	}

	chainClient := base.NewMockChainClient()
	chainClient.SetEstimateFee(map[iwallet.FeeLevel]iwallet.EstimateFeeRes{
		iwallet.FlPriority:      {FeePerTx: iwallet.NewAmount(50)},
		iwallet.FlNormal:        {FeePerTx: iwallet.NewAmount(40)},
		iwallet.FlEconomic:      {FeePerTx: iwallet.NewAmount(30)},
		iwallet.FLSuperEconomic: {FeePerTx: iwallet.NewAmount(20)},
	})

	w.ChainClient = chainClient
	w.KeyStore = base.NewKeyStore()
	w.Logger = logging.MustGetLogger("bchtest")
	w.CoinType = testZCashNativeCoin
	w.Done = make(chan struct{})
	w.PostInitFunc = w.postInit
	w.NetConfig = config.DefaultNetConfig()

	key, err := hdkeychain.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		return nil, err
	}

	if err := w.CreateWallet(*key, nil, time.Now()); err != nil {
		return nil, err
	}

	if err := w.OpenWallet(); err != nil {
		return nil, err
	}
	return w, nil
}

func TestZCashWallet_ValidateAddress(t *testing.T) {
	tests := []struct {
		address iwallet.Address
		valid   bool
	}{
		{
			address: iwallet.NewAddress(testInvalidAddress, testZCashNativeCoin),
			valid:   false,
		},
		{
			address: iwallet.NewAddress(testZECTestnetAddr, testZCashNativeCoin),
			valid:   true,
		},
	}
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		err := w.ValidateAddress(test.address)
		if !test.valid && err == nil {
			t.Errorf("Test %d expected invalid address got valid", i)
		}
		if test.valid && err != nil {
			fmt.Println(err)
			t.Errorf("Test %d expected valid address got invalid", i)
		}
	}
}

func TestZCashWallet_IsDust(t *testing.T) {
	tests := []struct {
		amount iwallet.Amount
		isDust bool
	}{
		{
			amount: iwallet.NewAmount(0),
			isDust: true,
		},
		{
			amount: iwallet.NewAmount(545),
			isDust: true,
		},
		{
			amount: iwallet.NewAmount(546),
			isDust: false,
		},
	}
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		isDust := w.IsDust(iwallet.NewAddress(testZECTestnetAddr, testZCashNativeCoin), test.amount)
		if test.isDust != isDust {
			t.Errorf("Test %d expected %t got %t", i, test.isDust, isDust)
		}
	}
}

func TestZCashWallet_EstimateEscrowFee(t *testing.T) {
	tests := []struct {
		threshold int
		nOuts     int
		level     iwallet.FeeLevel
		expected  iwallet.Amount
	}{
		{
			threshold: 1,
			nOuts:     1,
			level:     iwallet.FlEconomic,
			expected:  iwallet.NewAmount(5940),
		},
		{
			threshold: 1,
			nOuts:     1,
			level:     iwallet.FlNormal,
			expected:  iwallet.NewAmount(7920),
		},
		{
			threshold: 1,
			nOuts:     1,
			level:     iwallet.FlPriority,
			expected:  iwallet.NewAmount(9900),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlEconomic,
			expected:  iwallet.NewAmount(9960),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlNormal,
			expected:  iwallet.NewAmount(13280),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlPriority,
			expected:  iwallet.NewAmount(16600),
		},
	}

	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		fee, err := w.EstimateEscrowFee(test.threshold, test.nOuts, test.level)
		if err != nil {
			t.Errorf("Test %d: error %s", i, err)
		}
		if fee.Cmp(test.expected) != 0 {
			t.Errorf("Test %d: expected %s, got %s", i, test.expected, fee)
		}
	}
}

func TestZCashWallet_Multisig1of2(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	key1Bytes, err := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	if err != nil {
		t.Fatal(err)
	}

	key2Bytes, err := hex.DecodeString("c68ab7796c52952a062b4c875c758ae3831448240fb58c152cc58a224d6ad3b8")
	if err != nil {
		t.Fatal(err)
	}

	key1, _ := btcec.PrivKeyFromBytes(key1Bytes)
	key2, _ := btcec.PrivKeyFromBytes(key2Bytes)

	address, redeemScript, err := w.CreateMultisigAddress([]btcec.PublicKey{*key1.PubKey(), *key2.PubKey()}, []byte{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	expectedAddr := "t2VjrjNPjoPXDdgYM3PW3hTsh572EghfUQw"
	if address.String() != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, address)
	}
	expectedRedeemScript := "5121031f0ab385f3493b1e750f03ba590df5c7895415446d1c8aa60a7effc658ae183b2103c46f902f37e852dc7e8958bb440af7795fb323be6aaa3e99423076dc076315d052ae"
	if hex.EncodeToString(redeemScript) != expectedRedeemScript {
		t.Errorf("Expected redeem script %s, got %s", expectedRedeemScript, hex.EncodeToString(redeemScript))
	}

	h, err := chainhash.NewHashFromStr("bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d")
	if err != nil {
		t.Fatal(err)
	}

	op := wire.NewOutPoint(h, 0)

	tx := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:     serializeOutpoint(op),
				Amount: iwallet.NewAmount(1000000),
			},
		},
		To: []iwallet.SpendInfo{
			{
				Amount:  iwallet.NewAmount(900000),
				Address: iwallet.NewAddress(testZECTestnetAddr, testZCashNativeCoin),
			},
		},
	}

	sig, err := w.SignMultisigTransaction(tx, *key1, redeemScript)
	if err != nil {
		t.Fatal(err)
	}

	wtx, err := w.Begin()
	if err != nil {
		t.Fatal(err)
	}

	txid, err := w.BuildAndSend(wtx, tx, [][]iwallet.EscrowSignature{sig}, redeemScript, iwallet.ORDER_FINISH_COMPLETE)
	if err != nil {
		t.Fatal(err)
	}
	expectedTxid := "b6eed7a5099a65284f0b3cd64baa131f1caa275a903a1324fdd408923ec744bc"
	if txid.String() != expectedTxid {
		t.Errorf("Expected txid %s, got %s", expectedTxid, txid)
	}

	if err := wtx.Commit(); err != nil {
		t.Fatal(err)
	}

	chainClient, ok := w.ChainClient.(*base.MockChainClient)
	if !ok {
		t.Fatal("expected *base.MockChainClient")
	}
	if len(chainClient.BroadcastedTxs) == 0 {
		t.Fatal("Expected broadcasted transaction")
	}
}

func TestZCashWallet_Multisig2of3(t *testing.T) {
	w1, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}
	w2, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	key1Bytes, err := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	if err != nil {
		t.Fatal(err)
	}

	key2Bytes, err := hex.DecodeString("c68ab7796c52952a062b4c875c758ae3831448240fb58c152cc58a224d6ad3b8")
	if err != nil {
		t.Fatal(err)
	}

	key3Bytes, err := hex.DecodeString("0404e6967fc6c638564d4c381e299636fd01fdbcaaaa28e540647c928b44d39b")
	if err != nil {
		t.Fatal(err)
	}

	key1, _ := btcec.PrivKeyFromBytes(key1Bytes)
	key2, _ := btcec.PrivKeyFromBytes(key2Bytes)
	key3, _ := btcec.PrivKeyFromBytes(key3Bytes)

	address, redeemScript, err := w1.CreateMultisigAddress([]btcec.PublicKey{*key1.PubKey(), *key2.PubKey(), *key3.PubKey()}, []byte{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	expectedAddr := "t2LrMZoDJmjB4gafSnPabnwmXZ6BmKSBspv"
	if address.String() != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, address)
	}
	expectedRedeemScript := "5221031f0ab385f3493b1e750f03ba590df5c7895415446d1c8aa60a7effc658ae183b2103c46f902f37e852dc7e8958bb440af7795fb323be6aaa3e99423076dc076315d02102567a15f95333dbed4ff2913e58f554d784cf7787650e44d6b7faf30c79e5b67953ae"
	if hex.EncodeToString(redeemScript) != expectedRedeemScript {
		t.Errorf("Expected redeem script %s, got %s", expectedRedeemScript, hex.EncodeToString(redeemScript))
	}

	h, err := chainhash.NewHashFromStr("bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d")
	if err != nil {
		t.Fatal(err)
	}

	op := wire.NewOutPoint(h, 0)

	tx := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:     serializeOutpoint(op),
				Amount: iwallet.NewAmount(1000000),
			},
		},
		To: []iwallet.SpendInfo{
			{
				Amount:  iwallet.NewAmount(900000),
				Address: iwallet.NewAddress(testZECTestnetAddr, testZCashNativeCoin),
			},
		},
	}

	sig1, err := w1.SignMultisigTransaction(tx, *key1, redeemScript)
	if err != nil {
		t.Fatal(err)
	}

	sig2, err := w2.SignMultisigTransaction(tx, *key2, redeemScript)
	if err != nil {
		t.Fatal(err)
	}

	wtx, err := w1.Begin()
	if err != nil {
		t.Fatal(err)
	}

	txid, err := w1.BuildAndSend(wtx, tx, [][]iwallet.EscrowSignature{sig1, sig2}, redeemScript, iwallet.ORDER_FINISH_COMPLETE)
	if err != nil {
		t.Fatal(err)
	}
	expectedTxid := "36fe83d892459ca5c1e728a0e8fc3481f31775b6a97598f7d9c9b6510f860da7"
	if txid.String() != expectedTxid {
		t.Errorf("Expected txid %s, got %s", expectedTxid, txid)
	}

	if err := wtx.Commit(); err != nil {
		t.Fatal(err)
	}

	chainClient, ok := w1.ChainClient.(*base.MockChainClient)
	if !ok {
		t.Fatal("expected *base.MockChainClient")
	}
	if len(chainClient.BroadcastedTxs) == 0 {
		t.Fatal("Expected broadcasted transaction")
	}
}

func buildTestTx() (*wire.MsgTx, []byte, error) {
	expected, err := hex.DecodeString(`0400008085202f8901a8c685478265f4c14dada651969c45a65e1aeb8cd6791f2f5bb6a1d9952104d9010000006b483045022100a61e5d557568c2ddc1d9b03a7173c6ce7c996c4daecab007ac8f34bee01e6b9702204d38fdc0bcf2728a69fde78462a10fb45a9baa27873e6a5fc45fb5c76764202a01210365ffea3efa3908918a8b8627724af852fc9b86d7375b103ab0543cf418bcaa7ffeffffff02005a6202000000001976a9148132712c3ff19f3a151234616777420a6d7ef22688ac8b959800000000001976a9145453e4698f02a38abdaa521cd1ff2dee6fac187188ac29b0040048b004000000000000000000000000`)
	if err != nil {
		return nil, nil, err
	}

	tx := wire.NewMsgTx(1)

	inHash, err := hex.DecodeString("a8c685478265f4c14dada651969c45a65e1aeb8cd6791f2f5bb6a1d9952104d9")
	if err != nil {
		return nil, nil, err
	}
	prevHash, err := chainhash.NewHash(inHash)
	if err != nil {
		return nil, nil, err
	}
	op := wire.NewOutPoint(prevHash, 1)

	scriptSig, err := hex.DecodeString("483045022100a61e5d557568c2ddc1d9b03a7173c6ce7c996c4daecab007ac8f34bee01e6b9702204d38fdc0bcf2728a69fde78462a10fb45a9baa27873e6a5fc45fb5c76764202a01210365ffea3efa3908918a8b8627724af852fc9b86d7375b103ab0543cf418bcaa7f")
	if err != nil {
		return nil, nil, err
	}
	txIn := wire.NewTxIn(op, scriptSig, nil)
	txIn.Sequence = 4294967294

	tx.TxIn = []*wire.TxIn{txIn}

	pkScirpt0, err := hex.DecodeString("76a9148132712c3ff19f3a151234616777420a6d7ef22688ac")
	if err != nil {
		return nil, nil, err
	}
	out0 := wire.NewTxOut(40000000, pkScirpt0)

	pkScirpt1, err := hex.DecodeString("76a9145453e4698f02a38abdaa521cd1ff2dee6fac187188ac")
	if err != nil {
		return nil, nil, err
	}
	out1 := wire.NewTxOut(9999755, pkScirpt1)
	tx.TxOut = []*wire.TxOut{out0, out1}

	tx.LockTime = 307241
	return tx, expected, nil
}

func TestSerializeVersion4Transaction(t *testing.T) {
	tx, expected, err := buildTestTx()
	if err != nil {
		t.Fatal(err)
	}

	serialized, err := serializeVersion4Transaction(tx, 307272)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(serialized, expected) {
		t.Fatal("Failed to serialize transaction correctly")
	}
}

func TestCalcSignatureHash(t *testing.T) {
	tx, _, err := buildTestTx()
	if err != nil {
		t.Fatal(err)
	}

	prevScript, err := hex.DecodeString("76a914507173527b4c3318a2aecd793bf1cfed705950cf88ac")
	if err != nil {
		t.Fatal(err)
	}
	sigHash, err := calcSignatureHash(prevScript, txscript.SigHashAll, tx, 0, 50000000, 307272, 100000000000)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := hex.DecodeString("3cc713fbd86a0ed7f32a9c84dfe2782c5507bf03d039420e2444bdfaee5a42c2")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sigHash, expected) {
		t.Fatal("Failed to calculate correct sig hash")
	}
}
