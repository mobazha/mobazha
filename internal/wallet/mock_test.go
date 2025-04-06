package wallet

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha3.0/internal/events"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
)

func TestMockWallet_Spend(t *testing.T) {
	w := NewMockWallet()

	testUtxoPrevHash := make([]byte, 32)
	rand.Read(testUtxoPrevHash)

	addr, err := w.CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	w.utxos[hex.EncodeToString(append(testUtxoPrevHash, []byte{0x00, 0x00, 0x00, 0x00}...))] = mockUtxo{
		outpoint: append(testUtxoPrevHash, []byte{0x00, 0x00, 0x00, 0x00}...),
		address:  addr,
		value:    iwallet.NewAmount(10000),
	}

	spendAddrBytes := make([]byte, 20)
	rand.Read(spendAddrBytes)
	spendAddr := iwallet.NewAddress(hex.EncodeToString(spendAddrBytes), iwallet.CtMock)

	dbtx, err := w.Begin()
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.Spend(dbtx, spendAddr, iwallet.NewAmount(20000), iwallet.FlNormal, iwallet.Address{}, iwallet.Amount{})
	if err == nil {
		t.Error("should have errored for insuffient funds")
	}

	dbtx, err = w.Begin()
	if err != nil {
		t.Fatal(err)
	}

	txid, err := w.Spend(dbtx, spendAddr, iwallet.NewAmount(9000), iwallet.FlNormal, iwallet.Address{}, iwallet.Amount{})
	if err != nil {
		t.Fatal(err)
	}

	if err := dbtx.Commit(); err != nil {
		t.Fatal(err)
	}

	txidBytes, err := hex.DecodeString(string(txid))
	if err != nil {
		t.Fatal(err)
	}
	outpoint := append(txidBytes, []byte{0x00, 0x00, 0x00, 0x01}...)
	changeUtxo, ok := w.utxos[hex.EncodeToString(outpoint)]
	if !ok {
		t.Error("wallet missing change utxo")
	}

	if !bytes.Equal(changeUtxo.outpoint, outpoint) {
		t.Errorf("Incorrect change utxo hash returned. Expected %s, got %s", hex.EncodeToString(outpoint), hex.EncodeToString(changeUtxo.outpoint))
	}

	txn, ok := w.transactions[txid]
	if !ok {
		t.Error("wallet missing transaction")
	}

	if txn.ID.String() != txid.String() {
		t.Errorf("Incorrect txn txid. Expected %s, got %s", txid.String(), txn.ID.String())
	}

	txns, err := w.Transactions(-1, iwallet.TransactionID(""))
	if err != nil {
		t.Fatal(err)
	}

	if len(txns) != 1 {
		t.Errorf("Expected 1 txn, got %d", len(txns))
	}

	if txns[0].ID.String() != txid.String() {
		t.Errorf("Incorrect txn txid. Expected %s, got %s", txid.String(), txn.ID.String())
	}
}

func TestMockWallet_SweepWallet(t *testing.T) {
	w := NewMockWallet()

	testUtxoPrevHash := make([]byte, 32)
	rand.Read(testUtxoPrevHash)

	addr, err := w.CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	w.utxos[hex.EncodeToString(append(testUtxoPrevHash, []byte{0x00, 0x00, 0x00, 0x00}...))] = mockUtxo{
		outpoint: append(testUtxoPrevHash, []byte{0x00, 0x00, 0x00, 0x00}...),
		address:  addr,
		value:    iwallet.NewAmount(10000),
	}
	w.utxos[hex.EncodeToString(append(testUtxoPrevHash, []byte{0x00, 0x00, 0x00, 0x01}...))] = mockUtxo{
		outpoint: append(testUtxoPrevHash, []byte{0x00, 0x00, 0x00, 0x01}...),
		address:  addr,
		value:    iwallet.NewAmount(10000),
	}

	spendAddrBytes := make([]byte, 20)
	rand.Read(spendAddrBytes)
	spendAddr := iwallet.NewAddress(hex.EncodeToString(spendAddrBytes), iwallet.CtMock)

	dbtx, err := w.Begin()
	if err != nil {
		t.Fatal(err)
	}

	txid, err := w.SweepWallet(dbtx, spendAddr, iwallet.FlNormal)
	if err != nil {
		t.Fatal(err)
	}

	if err := dbtx.Commit(); err != nil {
		t.Fatal(err)
	}

	if len(w.utxos) != 0 {
		t.Error("Failed to spend all utxos")
	}

	txn, ok := w.transactions[txid]
	if !ok {
		t.Error("wallet missing transaction")
	}

	if txn.ID.String() != txid.String() {
		t.Errorf("Incorrect txn txid. Expected %s, got %s", txid.String(), txn.ID.String())
	}

	txns, err := w.Transactions(-1, iwallet.TransactionID(""))
	if err != nil {
		t.Fatal(err)
	}

	if len(txns) != 1 {
		t.Errorf("Expected 1 txn, got %d", len(txns))
	}

	if txns[0].ID.String() != txid.String() {
		t.Errorf("Incorrect txn txid. Expected %s, got %s", txid.String(), txn.ID.String())
	}
}

func TestMockWallet_CreateMultisigAddress(t *testing.T) {
	var (
		w1 = NewMockWallet()
		w2 = NewMockWallet()
		w3 = NewMockWallet()
	)

	k1, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	k3, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	addr1, rs1, err := w1.CreateMultisigAddress([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey(), *k3.PubKey()}, []byte{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	addr2, rs2, err := w2.CreateMultisigAddress([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey(), *k3.PubKey()}, []byte{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	addr3, rs3, err := w3.CreateMultisigAddress([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey(), *k3.PubKey()}, []byte{}, 2)
	if err != nil {
		t.Fatal(err)
	}

	if addr1.String() != addr2.String() {
		t.Errorf("Addresses are not equal. %s, %s", addr1.String(), addr2.String())
	}

	if addr2.String() != addr3.String() {
		t.Errorf("Addresses are not equal. %s, %s", addr2.String(), addr3.String())
	}

	if !bytes.Equal(rs1, rs2) {
		t.Errorf("Redeem scripts are not equal. %v, %v", rs1, rs2)
	}

	if !bytes.Equal(rs2, rs3) {
		t.Errorf("Redeem scripts are not equal. %v, %v", rs2, rs3)
	}

	addr1, rs1, err = w1.CreateMultisigWithTimeout([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey(), *k3.PubKey()}, []byte{}, 2, time.Hour, *k1.PubKey())
	if err != nil {
		t.Fatal(err)
	}
	addr2, rs2, err = w2.CreateMultisigWithTimeout([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey(), *k3.PubKey()}, []byte{}, 2, time.Hour, *k1.PubKey())
	if err != nil {
		t.Fatal(err)
	}
	addr3, rs3, err = w3.CreateMultisigWithTimeout([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey(), *k3.PubKey()}, []byte{}, 2, time.Hour, *k1.PubKey())
	if err != nil {
		t.Fatal(err)
	}

	if addr1.String() != addr2.String() {
		t.Errorf("Addresses are not equal. %s, %s", addr1.String(), addr2.String())
	}

	if addr2.String() != addr3.String() {
		t.Errorf("Addresses are not equal. %s, %s", addr2.String(), addr3.String())
	}

	if !bytes.Equal(rs1, rs2) {
		t.Errorf("Redeem scripts are not equal. %v, %v", rs1, rs2)
	}

	if !bytes.Equal(rs2, rs3) {
		t.Errorf("Redeem scripts are not equal. %v, %v", rs2, rs3)
	}
}

func TestMockWallet_SignMultisigTransaction(t *testing.T) {
	var (
		w1 = NewMockWallet()
		w2 = NewMockWallet()
	)

	k1, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	k3, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	addr, rs, err := w1.CreateMultisigAddress([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey(), *k3.PubKey()}, []byte{}, 2)
	if err != nil {
		t.Fatal(err)
	}

	outAddr, err := w1.CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	outpoint := make([]byte, 36)
	rand.Read(outpoint)
	txn := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:      outpoint,
				Amount:  iwallet.NewAmount(10000),
				Address: addr,
			},
		},
		To: []iwallet.SpendInfo{
			{
				Address: outAddr,
				Amount:  iwallet.NewAmount(9000),
			},
		},
	}

	sig1, err := w1.SignMultisigTransaction(txn, *k1, rs)
	if err != nil {
		t.Fatal(err)
	}
	sig2, err := w2.SignMultisigTransaction(txn, *k2, rs)
	if err != nil {
		t.Fatal(err)
	}

	dbtx, err := w1.Begin()
	if err != nil {
		t.Fatal(err)
	}

	_, err = w1.BuildAndSend(dbtx, txn, [][]iwallet.EscrowSignature{sig1, sig2}, rs, iwallet.ORDER_FINISH_COMPLETE)
	if err != nil {
		t.Fatal(err)
	}

	if err := dbtx.Commit(); err != nil {
		t.Fatal(err)
	}

	txs, err := w1.Transactions(-1, iwallet.TransactionID(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 {
		t.Error("Failed to record transaction")
	}
}

func TestMockWallet_SignMultisigAfterTimeout(t *testing.T) {
	var (
		w1 = NewMockWallet()
	)

	k1, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	k3, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	addr, rs, err := w1.CreateMultisigWithTimeout([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey(), *k3.PubKey()}, []byte{}, 2, time.Nanosecond, *k2.PubKey())
	if err != nil {
		t.Fatal(err)
	}

	outAddr, err := w1.CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	outpoint := make([]byte, 36)
	rand.Read(outpoint)
	txn := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:      outpoint,
				Amount:  iwallet.NewAmount(10000),
				Address: addr,
			},
		},
		To: []iwallet.SpendInfo{
			{
				Address: outAddr,
				Amount:  iwallet.NewAmount(9000),
			},
		},
	}

	dbtx, err := w1.Begin()
	if err != nil {
		t.Fatal(err)
	}

	_, err = w1.ReleaseFundsAfterTimeout(dbtx, txn, *k2, rs, iwallet.ORDER_FINISH_COMPLETE)
	if err != nil {
		t.Fatal(err)
	}

	if err := dbtx.Commit(); err != nil {
		t.Fatal(err)
	}

	txs, err := w1.Transactions(-1, iwallet.TransactionID(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 {
		t.Error("Failed to record transaction")
	}
}

func TestMockWallet_1of2(t *testing.T) {
	w1 := NewMockWallet()

	k1, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	addr, rs, err := w1.CreateMultisigAddress([]btcec.PublicKey{*k1.PubKey(), *k2.PubKey()}, []byte{}, 1)
	if err != nil {
		t.Fatal(err)
	}

	outAddr, err := w1.CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	outpoint := make([]byte, 36)
	rand.Read(outpoint)
	txn := iwallet.Transaction{
		From: []iwallet.SpendInfo{
			{
				ID:      outpoint,
				Amount:  iwallet.NewAmount(10000),
				Address: addr,
			},
		},
		To: []iwallet.SpendInfo{
			{
				Address: outAddr,
				Amount:  iwallet.NewAmount(9000),
			},
		},
	}

	sig1, err := w1.SignMultisigTransaction(txn, *k1, rs)
	if err != nil {
		t.Fatal(err)
	}

	dbtx, err := w1.Begin()
	if err != nil {
		t.Fatal(err)
	}

	_, err = w1.BuildAndSend(dbtx, txn, [][]iwallet.EscrowSignature{sig1}, rs, iwallet.ORDER_FINISH_COMPLETE)
	if err != nil {
		t.Fatal(err)
	}

	if err := dbtx.Commit(); err != nil {
		t.Fatal(err)
	}

	txs, err := w1.Transactions(-1, iwallet.TransactionID(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 {
		t.Error("Failed to record transaction")
	}
}

func TestMockWalletNetwork(t *testing.T) {
	network := NewMockWalletNetwork(3)
	network.Start()

	for _, wallet := range network.Wallets() {
		wallet.SetEventBus(events.NewBus())
	}

	sub, err := network.Wallets()[0].bus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	addr, err := network.Wallets()[0].NewAddress()
	if err != nil {
		t.Fatal(err)
	}

	// Generate coins and send to wallet 0.
	if err := network.GenerateToAddress(addr, iwallet.NewAmount(100000)); err != nil {
		t.Fatal(err)
	}

	<-sub.Out()

	if len(network.Wallets()[0].utxos) != 1 {
		t.Error("Failed to record new utxo")
	}

	if len(network.Wallets()[0].transactions) != 1 {
		t.Error("Failed to record new txn")
	}

	unconfirmed, confirmed, err := network.Wallets()[0].Balance()
	if err != nil {
		t.Fatal(err)
	}

	if confirmed.Cmp(iwallet.NewAmount(0)) != 0 {
		t.Error("Confirmed balance is not zero")
	}

	if unconfirmed.Cmp(iwallet.NewAmount(100000)) != 0 {
		t.Errorf("Incorrect unconfirmed balance. Expected %d, got %v", 100000, confirmed)
	}

	if len(network.Wallets()[1].utxos) != 0 {
		t.Error("Incorrectly recorded utxo")
	}

	if len(network.Wallets()[1].transactions) != 0 {
		t.Error("Incorrectly recorded txn")
	}

	if len(network.Wallets()[2].utxos) != 0 {
		t.Error("Incorrectly recorded utxo")
	}

	if len(network.Wallets()[2].transactions) != 0 {
		t.Error("Incorrectly recorded txn")
	}

	blockSub, err := network.Wallets()[0].bus.Subscribe(&events.BlockReceived{})
	if err != nil {
		t.Fatal(err)
	}

	// Generate block and check the transaction confirms.
	network.GenerateBlock()

	<-blockSub.Out()

	unconfirmed, confirmed, err = network.Wallets()[0].Balance()
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Cmp(iwallet.NewAmount(100000)) != 0 {
		t.Errorf("Incorrect confirmed balance. Expected %d, got %v", 100000, confirmed)
	}

	if unconfirmed.Cmp(iwallet.NewAmount(0)) != 0 {
		t.Error("Unconfirmed balance is not zero")
	}

	sub2, err := network.Wallets()[2].bus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	// Wallet 0 send coins to wallet 2.
	addr2, err := network.Wallets()[2].CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}

	dbtx, err := network.Wallets()[0].Begin()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := network.Wallets()[0].Spend(dbtx, addr2, iwallet.NewAmount(90000), iwallet.FlPriority, iwallet.Address{}, iwallet.Amount{}); err != nil {
		t.Fatal(err)
	}

	if err := dbtx.Commit(); err != nil {
		t.Fatal(err)
	}

	<-sub2.Out()

	if len(network.Wallets()[2].utxos) != 1 {
		t.Error("Failed to record new utxo")
	}

	if len(network.Wallets()[2].transactions) != 1 {
		t.Error("Failed to record new txn")
	}

	unconfirmed, confirmed, err = network.Wallets()[2].Balance()
	if err != nil {
		t.Fatal(err)
	}

	if confirmed.Cmp(iwallet.NewAmount(0)) != 0 {
		t.Error("Confirmed balance is not zero")
	}

	if unconfirmed.Cmp(iwallet.NewAmount(90000)) != 0 {
		t.Errorf("Incorrect unconfirmed balance. Expected %d, got %v", 90000, unconfirmed)
	}

	if len(network.Wallets()[0].utxos) != 1 {
		t.Error("Failed to record new utxo")
	}

	txs, err := network.Wallets()[0].Transactions(-1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 2 {
		t.Error("Failed to record new txn")
	}

	unconfirmed, confirmed, err = network.Wallets()[0].Balance()
	if err != nil {
		t.Fatal(err)
	}

	if confirmed.Cmp(iwallet.NewAmount(0)) != 0 {
		t.Error("Confirmed balance is not zero")
	}

	if unconfirmed.Cmp(iwallet.NewAmount(9250)) != 0 {
		t.Errorf("Incorrect unconfirmed balance. Expected %d, got %v", 9250, unconfirmed)
	}
}
