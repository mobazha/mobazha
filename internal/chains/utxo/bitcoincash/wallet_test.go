package bitcoincash

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	chainutxo "github.com/mobazha/mobazha3.0/internal/chains/utxo"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	testInvalidAddress = "abc"
	testBCHTestnetAddr = "qrk0e04s67l9mf20jvae6fznht04rej57sf8jz2nua"
)

var testBitcoinCashNativeCoin = func() iwallet.CoinType {
	coin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBitcoinCash)
	if err != nil {
		panic(err)
	}
	return coin
}()

func newTestWallet() (*BitcoinCashWallet, error) {
	w := &BitcoinCashWallet{
		testnet: true,
	}

	chainClient := base.NewMockChainClient()
	chainClient.SetEstimateFee(map[iwallet.FeeLevel]iwallet.EstimateFeeRes{
		iwallet.FlPriority:      {FeePerTx: iwallet.NewAmount(50), FeePerUnit: iwallet.NewAmount(50 * 1000)},
		iwallet.FlNormal:        {FeePerTx: iwallet.NewAmount(40), FeePerUnit: iwallet.NewAmount(40 * 1000)},
		iwallet.FlEconomic:      {FeePerTx: iwallet.NewAmount(30), FeePerUnit: iwallet.NewAmount(30 * 1000)},
		iwallet.FLSuperEconomic: {FeePerTx: iwallet.NewAmount(20), FeePerUnit: iwallet.NewAmount(20 * 1000)},
	})

	w.ChainClient = chainClient
	w.KeyStore = base.NewKeyStore()
	w.Logger = logging.MustGetLogger("bchtest")
	w.CoinType = testBitcoinCashNativeCoin
	w.Done = make(chan struct{})
	w.PostInitFunc = w.postInit
	w.NetConfig = config.DefaultNetConfig()

	key, err := hdkeychain.NewKeyFromString("tprv8ZgxMBicQKsPeghT19pungdFLMJM2hMs3EEn5WtgobD7wuQSFQu4VNaEJXH9HS3RhhLT4wgZ3hj31m3kafuxhL9vfGTRtBVLSog4zjxW3L1")
	if err != nil {
		return nil, err
	}

	if err := w.CreateWallet(*key, time.Now()); err != nil {
		return nil, err
	}

	if err := w.OpenWallet(); err != nil {
		return nil, err
	}
	return w, nil
}

func TestBitcoinCashWallet_ValidateAddress(t *testing.T) {
	tests := []struct {
		address iwallet.Address
		valid   bool
	}{
		{
			address: iwallet.NewAddress(testInvalidAddress, testBitcoinCashNativeCoin),
			valid:   false,
		},
		{
			address: iwallet.NewAddress(testBCHTestnetAddr, testBitcoinCashNativeCoin),
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
			t.Errorf("Test %d expected valid address got invalid", i)
		}
	}
}

func TestBitcoinCashWallet_EstimateEscrowFee(t *testing.T) {
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
			expected:  iwallet.NewAmount(9120),
		},
		{
			threshold: 1,
			nOuts:     1,
			level:     iwallet.FlNormal,
			expected:  iwallet.NewAmount(12160),
		},
		{
			threshold: 1,
			nOuts:     1,
			level:     iwallet.FlPriority,
			expected:  iwallet.NewAmount(15200),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlEconomic,
			expected:  iwallet.NewAmount(13140),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlNormal,
			expected:  iwallet.NewAmount(17520),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlPriority,
			expected:  iwallet.NewAmount(21900),
		},
	}

	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		fee, err := w.EstimateEscrowFee(1, test.threshold, test.nOuts, test.level)
		if err != nil {
			t.Errorf("Test %d: error %s", i, err)
		}
		if fee.Cmp(test.expected) != 0 {
			t.Errorf("Test %d: expected %s, got %s", i, test.expected, fee)
		}
	}
}

func TestBitcoinCashWallet_EstimateEscrowFee_UsesRelayFeeFloor(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	chainClient, ok := w.ChainClient.(*base.MockChainClient)
	if !ok {
		t.Fatal("expected *base.MockChainClient")
	}
	chainClient.SetEstimateFee(map[iwallet.FeeLevel]iwallet.EstimateFeeRes{
		iwallet.FlNormal: {FeePerTx: iwallet.NewAmount(0), FeePerUnit: iwallet.NewAmount(0)},
	})

	fee, err := w.EstimateEscrowFee(1, 1, 1, iwallet.FlNormal)
	if err != nil {
		t.Fatal(err)
	}
	if want := iwallet.NewAmount(304); fee.Cmp(want) != 0 {
		t.Fatalf("expected BCH relay fee floor %s, got %s", want, fee)
	}
}

func TestBitcoinCashWallet_IsDust(t *testing.T) {
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
		isDust := w.IsDust(iwallet.NewAddress(testBCHTestnetAddr, testBitcoinCashNativeCoin), test.amount)
		if test.isDust != isDust {
			t.Errorf("Test %d expected %t got %t", i, test.isDust, isDust)
		}
	}
}

func TestBitcoinCashWallet_Multisig1of2(t *testing.T) {
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
	expectedAddr := "prlxr3xvattzez7y79k5yv4gtgrqlxthyc9dnv8mm4"
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

	var buf bytes.Buffer
	if err := op.Serialize(&buf); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:     buf.Bytes(),
				Amount: iwallet.NewAmount(1000000),
			},
		},
		To: []iwallet.SpendInfo{
			{
				Amount:  iwallet.NewAmount(900000),
				Address: iwallet.NewAddress(testBCHTestnetAddr, testBitcoinCashNativeCoin),
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
	expectedTxid := "0f103a079ca2b0252e47a557d4c2aeb908d5570ba0907ef52512ce4740c49bac"
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
	txBytes := chainClient.BroadcastedTxs[len(chainClient.BroadcastedTxs)-1]
	if got, wantMax := len(txBytes), chainutxo.EstimateP2SHSchnorrMultisigSpendRelaySize(1, 1, 1); got > wantMax {
		t.Fatalf("serialized tx size %d exceeds estimated size %d", got, wantMax)
	}

	scriptAddr, err := bchutil.NewAddressScriptHash(redeemScript, w.params())
	if err != nil {
		t.Fatal(err)
	}

	fromScript, err := txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}

	var msgTx wire.MsgTx
	if err := msgTx.BchDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		t.Fatal(err)
	}

	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, nil, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestBitcoinCashWallet_Multisig2of3(t *testing.T) {
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
	expectedAddr := "pzwwxvlrywdy0gkzaq6ttxkccfznxw3sqsf42jhhea"
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

	var buf bytes.Buffer
	if err := op.Serialize(&buf); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:     buf.Bytes(),
				Amount: iwallet.NewAmount(1000000),
			},
		},
		To: []iwallet.SpendInfo{
			{
				Amount:  iwallet.NewAmount(900000),
				Address: iwallet.NewAddress(testBCHTestnetAddr, testBitcoinCashNativeCoin),
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
	expectedTxid := "a0a6487eaa732903b5344fc864dd6c33a00b7df3ec87dc2c0e341151495c325a"
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
	txBytes := chainClient.BroadcastedTxs[len(chainClient.BroadcastedTxs)-1]

	scriptAddr, err := bchutil.NewAddressScriptHash(redeemScript, w1.params())
	if err != nil {
		t.Fatal(err)
	}

	fromScript, err := txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}

	var msgTx wire.MsgTx
	if err := msgTx.BchDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		t.Fatal(err)
	}

	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, nil, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestBitcoinCashWallet_Multisig2of3Timlocked(t *testing.T) {
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

	address, redeemScript, err := w1.CreateMultisigWithTimeout([]btcec.PublicKey{*key1.PubKey(), *key2.PubKey(), *key3.PubKey()}, []byte{}, 2, time.Hour*24, *key2.PubKey())
	if err != nil {
		t.Fatal(err)
	}
	expectedAddr := "pr62804de6uwc42w0ktf64znavkfaa0eyujm08xlwx"
	if address.String() != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, address)
	}
	expectedRedeemScript := "635221031f0ab385f3493b1e750f03ba590df5c7895415446d1c8aa60a7effc658ae183b2103c46f902f37e852dc7e8958bb440af7795fb323be6aaa3e99423076dc076315d02102567a15f95333dbed4ff2913e58f554d784cf7787650e44d6b7faf30c79e5b67953ae67029000b2752103c46f902f37e852dc7e8958bb440af7795fb323be6aaa3e99423076dc076315d0ac68"
	if hex.EncodeToString(redeemScript) != expectedRedeemScript {
		t.Errorf("Expected redeem script %s, got %s", expectedRedeemScript, hex.EncodeToString(redeemScript))
	}

	h, err := chainhash.NewHashFromStr("bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d")
	if err != nil {
		t.Fatal(err)
	}

	op := wire.NewOutPoint(h, 0)

	var buf bytes.Buffer
	if err := op.Serialize(&buf); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:     buf.Bytes(),
				Amount: iwallet.NewAmount(1000000),
			},
		},
		To: []iwallet.SpendInfo{
			{
				Amount:  iwallet.NewAmount(900000),
				Address: iwallet.NewAddress(testBCHTestnetAddr, testBitcoinCashNativeCoin),
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
	expectedTxid := "ea33ac8c7361268976c2e56a62136fd0dc819828de0243fe5535a2ff6e5c87e7"
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
	txBytes := chainClient.BroadcastedTxs[len(chainClient.BroadcastedTxs)-1]

	scriptAddr, err := bchutil.NewAddressScriptHash(redeemScript, w1.params())
	if err != nil {
		t.Fatal(err)
	}

	fromScript, err := txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}

	var msgTx wire.MsgTx
	if err := msgTx.BchDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		t.Fatal(err)
	}

	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, nil, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestBitcoinCashWallet_ReleaseFundsAfterTimeout(t *testing.T) {
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

	key3Bytes, err := hex.DecodeString("0404e6967fc6c638564d4c381e299636fd01fdbcaaaa28e540647c928b44d39b")
	if err != nil {
		t.Fatal(err)
	}

	key1, _ := btcec.PrivKeyFromBytes(key1Bytes)
	key2, _ := btcec.PrivKeyFromBytes(key2Bytes)
	key3, _ := btcec.PrivKeyFromBytes(key3Bytes)

	address, redeemScript, err := w.CreateMultisigWithTimeout([]btcec.PublicKey{*key1.PubKey(), *key2.PubKey(), *key3.PubKey()}, []byte{}, 2, time.Hour*24, *key2.PubKey())
	if err != nil {
		t.Fatal(err)
	}
	expectedAddr := "pr62804de6uwc42w0ktf64znavkfaa0eyujm08xlwx"
	if address.String() != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, address)
	}
	expectedRedeemScript := "635221031f0ab385f3493b1e750f03ba590df5c7895415446d1c8aa60a7effc658ae183b2103c46f902f37e852dc7e8958bb440af7795fb323be6aaa3e99423076dc076315d02102567a15f95333dbed4ff2913e58f554d784cf7787650e44d6b7faf30c79e5b67953ae67029000b2752103c46f902f37e852dc7e8958bb440af7795fb323be6aaa3e99423076dc076315d0ac68"
	if hex.EncodeToString(redeemScript) != expectedRedeemScript {
		t.Errorf("Expected redeem script %s, got %s", expectedRedeemScript, hex.EncodeToString(redeemScript))
	}

	h, err := chainhash.NewHashFromStr("bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d")
	if err != nil {
		t.Fatal(err)
	}

	op := wire.NewOutPoint(h, 0)

	var buf bytes.Buffer
	if err := op.Serialize(&buf); err != nil {
		t.Fatal(err)
	}

	tx := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:     buf.Bytes(),
				Amount: iwallet.NewAmount(1000000),
			},
		},
		To: []iwallet.SpendInfo{
			{
				Amount:  iwallet.NewAmount(900000),
				Address: iwallet.NewAddress(testBCHTestnetAddr, testBitcoinCashNativeCoin),
			},
		},
	}

	wtx, err := w.Begin()
	if err != nil {
		t.Fatal(err)
	}

	txid, err := w.ReleaseFundsAfterTimeout(wtx, tx, *key2, redeemScript, iwallet.ORDER_FINISH_COMPLETE)
	if err != nil {
		t.Fatal(err)
	}
	expectedTxid := "81b911ff25c1b3acb68d5754d59607510fe11693e4957427504f559233fd7c2b"
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
	txBytes := chainClient.BroadcastedTxs[len(chainClient.BroadcastedTxs)-1]

	scriptAddr, err := bchutil.NewAddressScriptHash(redeemScript, w.params())
	if err != nil {
		t.Fatal(err)
	}

	fromScript, err := txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}

	var msgTx wire.MsgTx
	if err := msgTx.BchDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		t.Fatal(err)
	}

	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, nil, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

// --- UTXOAddressUtilities tests ---

const (
	testBCHPubKeyHex      = "0330d54fd0dd420a6e5f8d3624f5f3482cae350f79d5f0753bf5beef9c2d91af3c"
	testBCHMainnetAddress = "qrqva0xkc0fu4rr4m30vvt4725esa7gsug52ypqews"
	testBCHTestnetAddress = "qrqva0xkc0fu4rr4m30vvt4725esa7gsugscqxzwfv"
)

func TestBitcoinCashWallet_DerivePaymentAddressFromPubKey_Mainnet(t *testing.T) {
	w := &BitcoinCashWallet{testnet: false}

	pubKeyBytes, _ := hex.DecodeString(testBCHPubKeyHex)
	pubKey, _ := btcec.ParsePubKey(pubKeyBytes)

	addr, scriptPubKey, err := w.DerivePaymentAddressFromPubKey(pubKey)
	if err != nil {
		t.Fatalf("DerivePaymentAddressFromPubKey: %v", err)
	}
	if addr != testBCHMainnetAddress {
		t.Errorf("address mismatch: got %s, want %s", addr, testBCHMainnetAddress)
	}
	if len(scriptPubKey) != 25 {
		t.Errorf("expected 25-byte P2PKH scriptPubKey, got %d bytes", len(scriptPubKey))
	}
}

func TestBitcoinCashWallet_DerivePaymentAddressFromPubKey_Testnet(t *testing.T) {
	w := &BitcoinCashWallet{testnet: true}

	pubKeyBytes, _ := hex.DecodeString(testBCHPubKeyHex)
	pubKey, _ := btcec.ParsePubKey(pubKeyBytes)

	addr, _, err := w.DerivePaymentAddressFromPubKey(pubKey)
	if err != nil {
		t.Fatalf("DerivePaymentAddressFromPubKey: %v", err)
	}
	if addr != testBCHTestnetAddress {
		t.Errorf("address mismatch: got %s, want %s", addr, testBCHTestnetAddress)
	}
}

func TestBitcoinCashWallet_DerivePaymentAddressFromPubKey_NilPubKey(t *testing.T) {
	w := &BitcoinCashWallet{testnet: true}
	if _, _, err := w.DerivePaymentAddressFromPubKey(nil); err == nil {
		t.Errorf("expected error for nil pubkey")
	}
}

func TestBitcoinCashWallet_AddressToScriptPubKey_RoundTrip(t *testing.T) {
	w := &BitcoinCashWallet{testnet: false}

	pubKeyBytes, _ := hex.DecodeString(testBCHPubKeyHex)
	pubKey, _ := btcec.ParsePubKey(pubKeyBytes)

	addr, expected, err := w.DerivePaymentAddressFromPubKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}

	got, err := w.AddressToScriptPubKey(addr)
	if err != nil {
		t.Fatalf("AddressToScriptPubKey: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Errorf("scriptPubKey mismatch: derive=%x, decoded=%x", expected, got)
	}
}

func TestBitcoinCashWallet_AddressToScriptPubKey_Invalid(t *testing.T) {
	w := &BitcoinCashWallet{testnet: true}
	if _, err := w.AddressToScriptPubKey(testInvalidAddress); err == nil {
		t.Errorf("expected error for invalid address")
	}
}
