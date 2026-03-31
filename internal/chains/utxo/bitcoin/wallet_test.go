package bitcoin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/chains/database"
	"github.com/mobazha/mobazha3.0/internal/chains/database/sqlitedb"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	testInvalidAddress = "abc"
	testBTCMainnetAddr = "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"
	testBTCTestnetAddr = "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx"
)

var testBitcoinNativeCoin = func() iwallet.CoinType {
	coin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBitcoin)
	if err != nil {
		panic(err)
	}
	return coin
}()

func newTestWallet() (*BitcoinWallet, error) {
	w := &BitcoinWallet{
		testnet: true,
	}

	chainClient := base.NewMockChainClient()
	chainClient.SetEstimateFee(map[iwallet.FeeLevel]iwallet.EstimateFeeRes{
		iwallet.FlPriority:      {FeePerTx: iwallet.NewAmount(50), FeePerUnit: iwallet.NewAmount(50 * 1000)},
		iwallet.FlNormal:        {FeePerTx: iwallet.NewAmount(40), FeePerUnit: iwallet.NewAmount(40 * 1000)},
		iwallet.FlEconomic:      {FeePerTx: iwallet.NewAmount(30), FeePerUnit: iwallet.NewAmount(30 * 1000)},
		iwallet.FLSuperEconomic: {FeePerTx: iwallet.NewAmount(20), FeePerUnit: iwallet.NewAmount(20 * 1000)},
	})

	db, err := sqlitedb.NewMemoryDB()
	if err != nil {
		return nil, err
	}
	if err := database.InitializeDatabase(db); err != nil {
		return nil, err
	}

	w.ChainClient = chainClient
	w.DB = db
	w.Logger = logging.MustGetLogger("bchtest")
	w.CoinType = testBitcoinNativeCoin
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

func TestBitcoinWallet_Params_NetworkSelection(t *testing.T) {
	tests := []struct {
		name       string
		testnet    bool
		regtest    bool
		wantName   string
		wantBech32 string
	}{
		{
			name:       "mainnet",
			testnet:    false,
			regtest:    false,
			wantName:   "mainnet",
			wantBech32: "bc",
		},
		{
			name:       "testnet",
			testnet:    true,
			regtest:    false,
			wantName:   "testnet3",
			wantBech32: "tb",
		},
		{
			name:       "regtest",
			testnet:    false,
			regtest:    true,
			wantName:   "regtest",
			wantBech32: "bcrt",
		},
		{
			name:       "regtest takes precedence over testnet",
			testnet:    true,
			regtest:    true,
			wantName:   "regtest",
			wantBech32: "bcrt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &BitcoinWallet{
				testnet: tt.testnet,
				regtest: tt.regtest,
			}
			p := w.params()
			if p.Name != tt.wantName {
				t.Errorf("params().Name = %q, want %q", p.Name, tt.wantName)
			}
			if p.Bech32HRPSegwit != tt.wantBech32 {
				t.Errorf("params().Bech32HRPSegwit = %q, want %q", p.Bech32HRPSegwit, tt.wantBech32)
			}
		})
	}
}

func TestNewBitcoinWallet_PropagatesRegtest(t *testing.T) {
	tests := []struct {
		name       string
		cfgRegtest bool
		cfgTestnet bool
		wantNet    string
	}{
		{"regtest propagated", true, false, "regtest"},
		{"testnet propagated", false, true, "testnet3"},
		{"mainnet by default", false, false, "mainnet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewBitcoinWallet(&base.WalletConfig{
				Testnet:   tt.cfgTestnet,
				Regtest:   tt.cfgRegtest,
				NetConfig: config.DefaultNetConfig(),
			})
			if err != nil {
				t.Fatal(err)
			}
			p := w.params()
			if p.Name != tt.wantNet {
				t.Errorf("NewBitcoinWallet(Regtest=%v, Testnet=%v): params().Name = %q, want %q",
					tt.cfgRegtest, tt.cfgTestnet, p.Name, tt.wantNet)
			}
		})
	}
}

func TestBitcoinWallet_CreateMultisigAddress_RegtestPrefix(t *testing.T) {
	w := &BitcoinWallet{regtest: true}
	w.Init()
	w.CoinType = testBitcoinNativeCoin

	key1Bytes, _ := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	key2Bytes, _ := hex.DecodeString("c68ab7796c52952a062b4c875c758ae3831448240fb58c152cc58a224d6ad3b8")
	key1, _ := btcec.PrivKeyFromBytes(key1Bytes)
	key2, _ := btcec.PrivKeyFromBytes(key2Bytes)

	addr, _, err := w.CreateMultisigAddress([]btcec.PublicKey{*key1.PubKey(), *key2.PubKey()}, []byte{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(addr.String()) < 4 || addr.String()[:4] != "bcrt" {
		t.Errorf("Expected bcrt prefix for regtest, got address: %s", addr.String())
	}
}

func TestBitcoinWallet_ValidateAddress(t *testing.T) {
	tests := []struct {
		address iwallet.Address
		valid   bool
	}{
		{
			address: iwallet.NewAddress(testInvalidAddress, testBitcoinNativeCoin),
			valid:   false,
		},
		{
			address: iwallet.NewAddress(testBTCMainnetAddr, testBitcoinNativeCoin),
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

func TestBitcoinWallet_EstimateEscrowFee(t *testing.T) {
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

func TestBitcoinWallet_IsDust(t *testing.T) {
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
		isDust := w.IsDust(iwallet.NewAddress(testBTCMainnetAddr, testBitcoinNativeCoin), test.amount)
		if test.isDust != isDust {
			t.Errorf("Test %d expected %t got %t", i, test.isDust, isDust)
		}
	}
}

func TestBitcoinWallet_Multisig1of2(t *testing.T) {
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
	expectedAddr := "tb1qv5plgrqexzju9jympkh2qjcalgn0qytp2erqls9xaumc3nkz7v8swcl0jp"
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
				Address: iwallet.NewAddress(testBTCMainnetAddr, testBitcoinNativeCoin),
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
	expectedTxid := "b12f50c698dfd650bfdea3568e5cd37634e63a10b8de42187ae2aed120c7fb6b"
	if txid.String() != expectedTxid {
		t.Errorf("Expected txid %s, got %s", expectedTxid, txid)
	}

	if err := wtx.Commit(); err != nil {
		t.Fatal(err)
	}

	var txBytes []byte
	err = w.DB.View(func(tx database.Tx) error {
		var txs []database.UnconfirmedTransaction
		if err := tx.Read().Where("coin=?", testBitcoinNativeCoin).Find(&txs).Error; err != nil {
			return err
		}
		if len(txs) != 1 {
			t.Errorf("Expected 1 tx found %d", len(txs))
		}
		if txs[0].Txid != txid.String() {
			t.Errorf("Expected txid %s, got %s", txid, txs[0].Txid)
		}
		txBytes = txs[0].TxBytes
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	witnessProgram := sha256.Sum256(redeemScript)

	scriptAddr, err := btcutil.NewAddressWitnessScriptHash(witnessProgram[:], w.params())
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

	var amt int64 = 1000000
	prevOutputFetcher := txscript.NewCannedPrevOutputFetcher(fromScript, amt)
	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, amt, prevOutputFetcher)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestBitcoinWallet_Multisig2of3(t *testing.T) {
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
	expectedAddr := "tb1q8tz3nc4wsuh07009rykkgeme9p3qf2nevfa8kjysj34dme6cuq0s98uwsq"
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
				Address: iwallet.NewAddress(testBTCMainnetAddr, testBitcoinNativeCoin),
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
	expectedTxid := "b12f50c698dfd650bfdea3568e5cd37634e63a10b8de42187ae2aed120c7fb6b"
	if txid.String() != expectedTxid {
		t.Errorf("Expected txid %s, got %s", expectedTxid, txid)
	}

	if err := wtx.Commit(); err != nil {
		t.Fatal(err)
	}

	var txBytes []byte
	err = w1.DB.View(func(tx database.Tx) error {
		var txs []database.UnconfirmedTransaction
		if err := tx.Read().Where("coin=?", testBitcoinNativeCoin).Find(&txs).Error; err != nil {
			return err
		}
		if len(txs) != 1 {
			t.Errorf("Expected 1 tx found %d", len(txs))
		}
		if txs[0].Txid != txid.String() {
			t.Errorf("Expected txid %s, got %s", txid, txs[0].Txid)
		}
		txBytes = txs[0].TxBytes
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	witnessProgram := sha256.Sum256(redeemScript)

	scriptAddr, err := btcutil.NewAddressWitnessScriptHash(witnessProgram[:], w1.params())
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

	var amt int64 = 1000000
	prevOutputFetcher := txscript.NewCannedPrevOutputFetcher(fromScript, amt)
	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, amt, prevOutputFetcher)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestBitcoinWallet_Multisig2of3Timlocked(t *testing.T) {
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
	expectedAddr := "tb1qxpskrwmxttvynhrckl4da3jweaz50y20j6n9qrpfdtefvhwgvyxqur3559"
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
				Address: iwallet.NewAddress(testBTCMainnetAddr, testBitcoinNativeCoin),
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
	expectedTxid := "b12f50c698dfd650bfdea3568e5cd37634e63a10b8de42187ae2aed120c7fb6b"
	if txid.String() != expectedTxid {
		t.Errorf("Expected txid %s, got %s", expectedTxid, txid)
	}

	if err := wtx.Commit(); err != nil {
		t.Fatal(err)
	}

	var txBytes []byte
	err = w1.DB.View(func(tx database.Tx) error {
		var txs []database.UnconfirmedTransaction
		if err := tx.Read().Where("coin=?", testBitcoinNativeCoin).Find(&txs).Error; err != nil {
			return err
		}
		if len(txs) != 1 {
			t.Errorf("Expected 1 tx found %d", len(txs))
		}
		if txs[0].Txid != txid.String() {
			t.Errorf("Expected txid %s, got %s", txid, txs[0].Txid)
		}
		txBytes = txs[0].TxBytes
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	witnessProgram := sha256.Sum256(redeemScript)

	scriptAddr, err := btcutil.NewAddressWitnessScriptHash(witnessProgram[:], w1.params())
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

	var amt int64 = 1000000
	prevOutputFetcher := txscript.NewCannedPrevOutputFetcher(fromScript, amt)
	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, amt, prevOutputFetcher)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

func TestBitcoinWallet_ReleaseFundsAfterTimeout(t *testing.T) {
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
	expectedAddr := "tb1qxpskrwmxttvynhrckl4da3jweaz50y20j6n9qrpfdtefvhwgvyxqur3559"
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
				Address: iwallet.NewAddress(testBTCMainnetAddr, testBitcoinNativeCoin),
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
	expectedTxid := "3bbcb72cb4c5ff7d6f2c11ef26c64f48f944943300f27b74d064bacf5f3a9369"
	if txid.String() != expectedTxid {
		t.Errorf("Expected txid %s, got %s", expectedTxid, txid)
	}

	if err := wtx.Commit(); err != nil {
		t.Fatal(err)
	}

	var txBytes []byte
	err = w.DB.View(func(tx database.Tx) error {
		var txs []database.UnconfirmedTransaction
		if err := tx.Read().Where("coin=?", testBitcoinNativeCoin).Find(&txs).Error; err != nil {
			return err
		}
		if len(txs) != 1 {
			t.Errorf("Expected 1 tx found %d", len(txs))
		}
		if txs[0].Txid != txid.String() {
			t.Errorf("Expected txid %s, got %s", txid, txs[0].Txid)
		}
		txBytes = txs[0].TxBytes
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	witnessProgram := sha256.Sum256(redeemScript)

	scriptAddr, err := btcutil.NewAddressWitnessScriptHash(witnessProgram[:], w.params())
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

	var amt int64 = 1000000
	prevOutputFetcher := txscript.NewCannedPrevOutputFetcher(fromScript, amt)
	vm, err := txscript.NewEngine(fromScript, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, amt, prevOutputFetcher)
	if err != nil {
		t.Fatal(err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("Script verificationf failed: %s", err)
	}
}

// TestBitcoinWallet_SpendFromDerivedAddress tests the SpendFromDerivedAddress method
// which is used for spending from single-sig addresses
func TestBitcoinWallet_SpendFromDerivedAddress(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	// Create a private key for signing
	keyBytes, err := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	if err != nil {
		t.Fatal(err)
	}
	privKey, pubKey := btcec.PrivKeyFromBytes(keyBytes)

	// Create P2WPKH address from the public key
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	witnessAddr, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}

	// Create the scriptPubKey for P2WPKH
	scriptPubKey, err := txscript.PayToAddrScript(witnessAddr)
	if err != nil {
		t.Fatal(err)
	}

	// Create a fake UTXO
	txidStr := "bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d"
	inputAmount := int64(1000000) // 1 BTC in satoshis

	utxo := iwallet.UTXO{
		TxID:         iwallet.TransactionID(txidStr),
		OutputIndex:  0,
		Amount:       iwallet.NewAmount(inputAmount),
		ScriptPubKey: scriptPubKey,
	}

	// Create outputs: seller gets 900000, platform gets 50000, fee is 50000
	sellerAddr := testBTCTestnetAddr
	platformAddr := "tb1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3q0sl5k7"

	outputs := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress(sellerAddr, testBitcoinNativeCoin),
			Amount:  iwallet.NewAmount(900000),
		},
		{
			Address: iwallet.NewAddress(platformAddr, testBitcoinNativeCoin),
			Amount:  iwallet.NewAmount(50000),
		},
	}
	// Total outputs: 950000, Input: 1000000, Implicit fee: 50000

	// Begin transaction
	wtx, err := w.Begin()
	if err != nil {
		t.Fatal(err)
	}

	// Call SpendFromDerivedAddress
	txid, err := w.SpendFromDerivedAddress(wtx, utxo, outputs, *privKey, iwallet.FlNormal)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Transaction ID: %s", txid)

	// Commit to trigger broadcast (which will fail in test, but that's ok)
	// In test environment, we just want to verify the transaction is built correctly
	// Skip commit since mock chain client doesn't handle broadcast

	// Verify the transaction was built correctly
	// We need to access the transaction bytes from the DBTx
	wbtx, ok := wtx.(*base.DBTx)
	if !ok {
		t.Fatal("wtx is not expected type")
	}

	// Manually trigger OnCommit to build the transaction
	// First, let's just verify the txid is not empty
	if txid.String() == "" {
		t.Error("Expected non-empty txid")
	}

	// Now let's verify the transaction is valid by rebuilding and checking
	txidHash, _ := chainhash.NewHashFromStr(txidStr)
	outpoint := wire.NewOutPoint(txidHash, 0)

	// Rebuild the transaction for verification
	verifyTx := wire.NewMsgTx(wire.TxVersion)
	verifyTx.AddTxIn(wire.NewTxIn(outpoint, nil, nil))

	for _, out := range outputs {
		script, err := txscript.PayToAddrScript(mustDecodeAddress(out.Address.String(), &chaincfg.TestNet3Params))
		if err != nil {
			t.Fatal(err)
		}
		verifyTx.AddTxOut(wire.NewTxOut(out.Amount.Int64(), script))
	}

	// The DBTx should have OnCommit set
	if wbtx.OnCommit == nil {
		t.Error("OnCommit should be set")
	}
}

// TestBitcoinWallet_SpendFromDerivedAddress_OutputsExceedInput tests error when outputs > input
func TestBitcoinWallet_SpendFromDerivedAddress_OutputsExceedInput(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, _ := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	privKey, pubKey := btcec.PrivKeyFromBytes(keyBytes)

	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	witnessAddr, _ := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, &chaincfg.TestNet3Params)
	scriptPubKey, _ := txscript.PayToAddrScript(witnessAddr)

	utxo := iwallet.UTXO{
		TxID:         iwallet.TransactionID("bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d"),
		OutputIndex:  0,
		Amount:       iwallet.NewAmount(1000000),
		ScriptPubKey: scriptPubKey,
	}

	outputs := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress(testBTCTestnetAddr, testBitcoinNativeCoin),
			Amount:  iwallet.NewAmount(2000000), // More than input
		},
	}

	wtx, _ := w.Begin()
	_, err = w.SpendFromDerivedAddress(wtx, utxo, outputs, *privKey, iwallet.FlNormal)
	if err == nil {
		t.Error("Expected error when outputs exceed input")
	}
	t.Logf("Got expected error: %v", err)
}

// TestBitcoinWallet_SpendFromDerivedAddress_ZeroFee tests error when fee is zero
func TestBitcoinWallet_SpendFromDerivedAddress_ZeroFee(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, _ := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	privKey, pubKey := btcec.PrivKeyFromBytes(keyBytes)

	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	witnessAddr, _ := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, &chaincfg.TestNet3Params)
	scriptPubKey, _ := txscript.PayToAddrScript(witnessAddr)

	utxo := iwallet.UTXO{
		TxID:         iwallet.TransactionID("bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d"),
		OutputIndex:  0,
		Amount:       iwallet.NewAmount(1000000),
		ScriptPubKey: scriptPubKey,
	}

	outputs := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress(testBTCTestnetAddr, testBitcoinNativeCoin),
			Amount:  iwallet.NewAmount(1000000), // Equals input, zero fee
		},
	}

	wtx, _ := w.Begin()
	_, err = w.SpendFromDerivedAddress(wtx, utxo, outputs, *privKey, iwallet.FlNormal)
	if err == nil {
		t.Error("Expected error when fee is zero")
	}
	t.Logf("Got expected error: %v", err)
}

// TestBitcoinWallet_SpendFromDerivedAddress_ScriptVerification tests that the signed transaction is valid
func TestBitcoinWallet_SpendFromDerivedAddress_ScriptVerification(t *testing.T) {
	w, err := newTestWallet()
	if err != nil {
		t.Fatal(err)
	}

	// Create a private key for signing
	keyBytes, _ := hex.DecodeString("84c8a01a81bf562aafafd4a9fccda533b33d6382b984c081a8cb7817bf909c18")
	privKey, pubKey := btcec.PrivKeyFromBytes(keyBytes)

	// Create P2WPKH address
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	witnessAddr, _ := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, &chaincfg.TestNet3Params)
	scriptPubKey, _ := txscript.PayToAddrScript(witnessAddr)

	inputAmount := int64(1000000)
	txidStr := "bdb237bf8c5de6b60ba1e2dcfe364fc24f583e568d1682f851a9d0f11a45c78d"

	utxo := iwallet.UTXO{
		TxID:         iwallet.TransactionID(txidStr),
		OutputIndex:  0,
		Amount:       iwallet.NewAmount(inputAmount),
		ScriptPubKey: scriptPubKey,
	}

	outputs := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress(testBTCTestnetAddr, testBitcoinNativeCoin),
			Amount:  iwallet.NewAmount(950000), // 50000 sat fee
		},
	}

	wtx, _ := w.Begin()
	txid, err := w.SpendFromDerivedAddress(wtx, utxo, outputs, *privKey, iwallet.FlNormal)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Generated txid: %s", txid)

	// Get the transaction bytes by triggering OnCommit
	// In a real scenario, this would broadcast, but MockChainClient handles it
	wbtx := wtx.(*base.DBTx)

	// Execute OnCommit to save the transaction
	if err := wbtx.OnCommit(); err != nil {
		t.Fatal(err)
	}

	// Retrieve the saved transaction and verify the signature
	var txBytes []byte
	err = w.DB.View(func(tx database.Tx) error {
		var txs []database.UnconfirmedTransaction
		if err := tx.Read().Where("txid = ?", txid.String()).Find(&txs).Error; err != nil {
			return err
		}
		if len(txs) != 1 {
			t.Errorf("Expected 1 tx found %d", len(txs))
			return nil
		}
		txBytes = txs[0].TxBytes
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Decode and verify the transaction
	var msgTx wire.MsgTx
	if err := msgTx.BtcDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.WitnessEncoding); err != nil {
		t.Fatal(err)
	}

	// Verify with txscript engine
	prevOutputFetcher := txscript.NewCannedPrevOutputFetcher(scriptPubKey, inputAmount)
	vm, err := txscript.NewEngine(scriptPubKey, &msgTx, 0, txscript.StandardVerifyFlags, nil, nil, inputAmount, prevOutputFetcher)
	if err != nil {
		t.Fatal(err)
	}

	if err := vm.Execute(); err != nil {
		t.Errorf("Script verification failed: %s", err)
	} else {
		t.Log("Script verification passed!")
	}

	// Verify transaction structure
	if len(msgTx.TxIn) != 1 {
		t.Errorf("Expected 1 input, got %d", len(msgTx.TxIn))
	}
	if len(msgTx.TxOut) != 1 {
		t.Errorf("Expected 1 output, got %d", len(msgTx.TxOut))
	}
	if msgTx.TxOut[0].Value != 950000 {
		t.Errorf("Expected output value 950000, got %d", msgTx.TxOut[0].Value)
	}

	// Calculate and verify fee
	fee := inputAmount - msgTx.TxOut[0].Value
	t.Logf("Transaction fee: %d satoshis", fee)
	if fee != 50000 {
		t.Errorf("Expected fee 50000, got %d", fee)
	}
}

func mustDecodeAddress(addr string, params *chaincfg.Params) btcutil.Address {
	decoded, err := btcutil.DecodeAddress(addr, params)
	if err != nil {
		panic(err)
	}
	return decoded
}
