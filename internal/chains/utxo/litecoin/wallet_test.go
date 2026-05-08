package litecoin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/ltcsuite/ltcd/chaincfg"
	"github.com/ltcsuite/ltcd/chaincfg/chainhash"
	"github.com/ltcsuite/ltcd/ltcutil"
	"github.com/ltcsuite/ltcd/txscript"
	"github.com/ltcsuite/ltcd/wire"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	testInvalidAddress = "abc"
	testLTCTestnetAddr = "tltc1q0wzfm6yz9gxght997y38mfvc9lj25hrj2lwdtq"
)

var testLitecoinNativeCoin = func() iwallet.CoinType {
	coin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainLitecoin)
	if err != nil {
		panic(err)
	}
	return coin
}()

func newTestWallet() (*LitecoinWallet, error) {
	w := &LitecoinWallet{
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
	w.CoinType = testLitecoinNativeCoin
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

func TestLitecoinWallet_ValidateAddress(t *testing.T) {
	tests := []struct {
		address iwallet.Address
		valid   bool
	}{
		{
			address: iwallet.NewAddress(testInvalidAddress, testLitecoinNativeCoin),
			valid:   false,
		},
		{
			address: iwallet.NewAddress(testLTCTestnetAddr, testLitecoinNativeCoin),
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

func TestLitecoinWallet_EstimateEscrowFee(t *testing.T) {
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
			expected:  iwallet.NewAmount(5490),
		},
		{
			threshold: 1,
			nOuts:     1,
			level:     iwallet.FlNormal,
			expected:  iwallet.NewAmount(7320),
		},
		{
			threshold: 1,
			nOuts:     1,
			level:     iwallet.FlPriority,
			expected:  iwallet.NewAmount(9150),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlEconomic,
			expected:  iwallet.NewAmount(9510),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlNormal,
			expected:  iwallet.NewAmount(12680),
		},
		{
			threshold: 2,
			nOuts:     2,
			level:     iwallet.FlPriority,
			expected:  iwallet.NewAmount(15850),
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

func TestLitecoinWallet_IsDust(t *testing.T) {
	tests := []struct {
		amount iwallet.Amount
		isDust bool
	}{
		{
			amount: iwallet.NewAmount(0),
			isDust: true,
		},
		{
			amount: iwallet.NewAmount(293),
			isDust: true,
		},
		{
			amount: iwallet.NewAmount(294),
			isDust: false,
		},
	}
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		isDust := w.IsDust(iwallet.NewAddress(testLTCTestnetAddr, testLitecoinNativeCoin), test.amount)
		if test.isDust != isDust {
			t.Errorf("Test %d expected %t got %t", i, test.isDust, isDust)
		}
	}
}

func TestLitecoinWallet_Multisig1of2(t *testing.T) {
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
	expectedAddr := "tltc1qv5plgrqexzju9jympkh2qjcalgn0qytp2erqls9xaumc3nkz7v8s3mrwd7"
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
				Address: iwallet.NewAddress(testLTCTestnetAddr, testLitecoinNativeCoin),
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
	expectedTxid := "062ceb683cb367e373938090694d3bfabc0a8358b732c586731700ee8d8dd30c"
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

	witnessProgram := sha256.Sum256(redeemScript)

	scriptAddr, err := ltcutil.NewAddressWitnessScriptHash(witnessProgram[:], w.params())
	if err != nil {
		t.Fatal(err)
	}

	fromScript, err := txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}

	var msgTx wire.MsgTx
	if err := msgTx.BtcDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.WitnessEncoding); err != nil {
		t.Fatal(err)
	}

	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestLitecoinWallet_Multisig2of3(t *testing.T) {
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
	expectedAddr := "tltc1q8tz3nc4wsuh07009rykkgeme9p3qf2nevfa8kjysj34dme6cuq0s6yq00l"
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
				Address: iwallet.NewAddress(testLTCTestnetAddr, testLitecoinNativeCoin),
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
	expectedTxid := "062ceb683cb367e373938090694d3bfabc0a8358b732c586731700ee8d8dd30c"
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

	witnessProgram := sha256.Sum256(redeemScript)

	scriptAddr, err := ltcutil.NewAddressWitnessScriptHash(witnessProgram[:], w1.params())
	if err != nil {
		t.Fatal(err)
	}

	fromScript, err := txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}

	var msgTx wire.MsgTx
	if err := msgTx.BtcDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.WitnessEncoding); err != nil {
		t.Fatal(err)
	}

	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestLitecoinWallet_Multisig2of3Timlocked(t *testing.T) {
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
	expectedAddr := "tltc1qxpskrwmxttvynhrckl4da3jweaz50y20j6n9qrpfdtefvhwgvyxqrqd4t6"
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
				Address: iwallet.NewAddress(testLTCTestnetAddr, testLitecoinNativeCoin),
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
	expectedTxid := "062ceb683cb367e373938090694d3bfabc0a8358b732c586731700ee8d8dd30c"
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

	witnessProgram := sha256.Sum256(redeemScript)

	scriptAddr, err := ltcutil.NewAddressWitnessScriptHash(witnessProgram[:], w1.params())
	if err != nil {
		t.Fatal(err)
	}

	fromScript, err := txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}

	var msgTx wire.MsgTx
	if err := msgTx.BtcDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.WitnessEncoding); err != nil {
		t.Fatal(err)
	}

	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestLitecoinWallet_ReleaseFundsAfterTimeout(t *testing.T) {
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
	expectedAddr := "tltc1qxpskrwmxttvynhrckl4da3jweaz50y20j6n9qrpfdtefvhwgvyxqrqd4t6"
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
				Address: iwallet.NewAddress(testLTCTestnetAddr, testLitecoinNativeCoin),
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
	expectedTxid := "50c373196dd5e8117e4e98dbc55bad4f53822ea88673260745c9017bb4847445"
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

	witnessProgram := sha256.Sum256(redeemScript)

	scriptAddr, err := ltcutil.NewAddressWitnessScriptHash(witnessProgram[:], w.params())
	if err != nil {
		t.Fatal(err)
	}

	fromScript, err := txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}

	var msgTx wire.MsgTx
	if err := msgTx.BtcDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.WitnessEncoding); err != nil {
		t.Fatal(err)
	}

	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestLitecoinWallet_BuildSweepTx(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, _ := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	privKey, pubKey := btcec.PrivKeyFromBytes(keyBytes)

	pubKeyHash := ltcutil.Hash160(pubKey.SerializeCompressed())
	witnessAddr, _ := ltcutil.NewAddressWitnessPubKeyHash(pubKeyHash, &chaincfg.TestNet4Params)
	scriptPubKey, _ := txscript.PayToAddrScript(witnessAddr)

	destPubKeyHash := ltcutil.Hash160([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14,
		0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21})
	destWitnessAddr, _ := ltcutil.NewAddressWitnessPubKeyHash(destPubKeyHash, &chaincfg.TestNet4Params)
	destAddr := destWitnessAddr.String()

	txidStr := "bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d"
	inputAmount := int64(1000000)

	inputs := []iwallet.SweepInput{
		{TxHash: txidStr, OutputIndex: 0, Value: inputAmount},
	}

	rawTx, txHash, err := w.BuildSweepTx(inputs, *privKey, destAddr, 2)
	if err != nil {
		t.Fatal(err)
	}
	if txHash == "" {
		t.Error("Expected non-empty txHash")
	}
	if len(rawTx) == 0 {
		t.Error("Expected non-empty rawTx")
	}

	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(rawTx)); err != nil {
		t.Fatal(err)
	}
	if len(msgTx.TxIn) != 1 {
		t.Errorf("Expected 1 input, got %d", len(msgTx.TxIn))
	}
	if len(msgTx.TxOut) != 1 {
		t.Errorf("Expected 1 output, got %d", len(msgTx.TxOut))
	}

	vm, err := txscript.NewEngine(scriptPubKey, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, inputAmount)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verification failed: %s", err)
	}

	fee := inputAmount - msgTx.TxOut[0].Value
	t.Logf("Transaction fee: %d litoshis (%.1f sat/vB)", fee, float64(fee)/float64(10+68+31))
}

func TestLitecoinWallet_BuildSweepTx_FeeExceedsInput(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, _ := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	privKey, _ := btcec.PrivKeyFromBytes(keyBytes)

	inputs := []iwallet.SweepInput{
		{TxHash: "bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d", OutputIndex: 0, Value: 100},
	}

	destPubKeyHash := ltcutil.Hash160([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14,
		0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21})
	destWitnessAddr, _ := ltcutil.NewAddressWitnessPubKeyHash(destPubKeyHash, &chaincfg.TestNet4Params)

	_, _, err = w.BuildSweepTx(inputs, *privKey, destWitnessAddr.String(), 2)
	if err == nil {
		t.Error("Expected error when fee exceeds input")
	}
}

func TestLitecoinWallet_BuildSweepTx_NoInputs(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, _ := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	privKey, _ := btcec.PrivKeyFromBytes(keyBytes)

	destPubKeyHash := ltcutil.Hash160([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14,
		0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21})
	destWitnessAddr, _ := ltcutil.NewAddressWitnessPubKeyHash(destPubKeyHash, &chaincfg.TestNet4Params)

	_, _, err = w.BuildSweepTx(nil, *privKey, destWitnessAddr.String(), 2)
	if err == nil {
		t.Error("Expected error with no inputs")
	}
}

// --- UTXOAddressUtilities tests ---

const (
	testLTCPubKeyHex      = "0330d54fd0dd420a6e5f8d3624f5f3482cae350f79d5f0753bf5beef9c2d91af3c"
	testLTCMainnetAddress = "ltc1qcr8te4kr609gcawutmrza0j4xv80jy8z4nqduv"
	testLTCTestnetAddress = "tltc1qcr8te4kr609gcawutmrza0j4xv80jy8zzpry0x"
)

func TestLitecoinWallet_DerivePaymentAddressFromPubKey_Mainnet(t *testing.T) {
	w := &LitecoinWallet{testnet: false}

	pubKeyBytes, _ := hex.DecodeString(testLTCPubKeyHex)
	pubKey, _ := btcec.ParsePubKey(pubKeyBytes)

	addr, scriptPubKey, err := w.DerivePaymentAddressFromPubKey(pubKey)
	if err != nil {
		t.Fatalf("DerivePaymentAddressFromPubKey: %v", err)
	}

	if addr != testLTCMainnetAddress {
		t.Errorf("address mismatch: got %s, want %s", addr, testLTCMainnetAddress)
	}
	if !strings.HasPrefix(addr, "ltc1") {
		t.Errorf("LTC mainnet address should start with ltc1: got %s", addr)
	}
	if len(scriptPubKey) != 22 {
		t.Errorf("expected 22-byte P2WPKH scriptPubKey, got %d bytes", len(scriptPubKey))
	}
	if scriptPubKey[0] != txscript.OP_0 {
		t.Errorf("scriptPubKey[0]=%x, want OP_0=%x", scriptPubKey[0], txscript.OP_0)
	}
}

func TestLitecoinWallet_DerivePaymentAddressFromPubKey_Testnet(t *testing.T) {
	w := &LitecoinWallet{testnet: true}

	pubKeyBytes, _ := hex.DecodeString(testLTCPubKeyHex)
	pubKey, _ := btcec.ParsePubKey(pubKeyBytes)

	addr, _, err := w.DerivePaymentAddressFromPubKey(pubKey)
	if err != nil {
		t.Fatalf("DerivePaymentAddressFromPubKey: %v", err)
	}
	if addr != testLTCTestnetAddress {
		t.Errorf("address mismatch: got %s, want %s", addr, testLTCTestnetAddress)
	}
	if !strings.HasPrefix(addr, "tltc1") {
		t.Errorf("LTC testnet address should start with tltc1: got %s", addr)
	}
}

func TestLitecoinWallet_DerivePaymentAddressFromPubKey_NilPubKey(t *testing.T) {
	w := &LitecoinWallet{testnet: true}
	if _, _, err := w.DerivePaymentAddressFromPubKey(nil); err == nil {
		t.Errorf("expected error for nil pubkey")
	}
}

func TestLitecoinWallet_AddressToScriptPubKey_RoundTrip(t *testing.T) {
	w := &LitecoinWallet{testnet: false}

	pubKeyBytes, _ := hex.DecodeString(testLTCPubKeyHex)
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

func TestLitecoinWallet_AddressToScriptPubKey_Invalid(t *testing.T) {
	w := &LitecoinWallet{testnet: true}
	if _, err := w.AddressToScriptPubKey(testInvalidAddress); err == nil {
		t.Errorf("expected error for invalid address")
	}
}
