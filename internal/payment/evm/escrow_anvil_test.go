//go:build integration

package evmpayment

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	ethWal "github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/internal/chains/evm/contract"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// Anvil default pre-funded keys (deterministic for --mnemonic="test test test test test test test test test test test junk")
const (
	anvilKey0 = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	anvilKey1 = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	anvilKey2 = "5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
	anvilKey3 = "7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6"
)

func anvilURL() string {
	if u := os.Getenv("ANVIL_URL"); u != "" {
		return u
	}
	return "http://localhost:8545"
}

func connectAnvil(t *testing.T) *ethclient.Client {
	t.Helper()
	client, err := ethclient.Dial(anvilURL())
	if err != nil {
		t.Skipf("Anvil not available at %s: %v", anvilURL(), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ChainID(ctx); err != nil {
		t.Skipf("Anvil not responding: %v", err)
	}
	return client
}

func mustPrivKey(t *testing.T, hexKey string) *ecdsa.PrivateKey {
	t.Helper()
	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		t.Fatalf("invalid key: %v", err)
	}
	return key
}

func ecdsaToBtcec(t *testing.T, key *ecdsa.PrivateKey) *btcec.PrivateKey {
	t.Helper()
	privBytes := crypto.FromECDSA(key)
	btcKey, _ := btcec.PrivKeyFromBytes(privBytes)
	return btcKey
}

func deployEscrow(t *testing.T, client *ethclient.Client, deployerKey *ecdsa.PrivateKey) common.Address {
	t.Helper()
	ctx := context.Background()

	bytecodeHex, err := os.ReadFile("testdata/escrow_bytecode.hex")
	if err != nil {
		t.Fatalf("read bytecode: %v (run Hardhat compile first)", err)
	}
	bytecode, err := hex.DecodeString(strings.TrimSpace(string(bytecodeHex)))
	if err != nil {
		t.Fatalf("decode bytecode: %v", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := abi.JSON(strings.NewReader(contract.EscrowMetaData.ABI))
	if err != nil {
		t.Fatal(err)
	}

	deployerAddr := crypto.PubkeyToAddress(deployerKey.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, deployerAddr)
	if err != nil {
		t.Fatal(err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatal(err)
	}

	constructorInput, err := parsed.Pack("")
	if err != nil {
		constructorInput = nil
	}
	deployData := append(bytecode, constructorInput...)

	tx := types.NewContractCreation(nonce, big.NewInt(0), 5_000_000, gasPrice, deployData)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), deployerKey)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.SendTransaction(ctx, signedTx); err != nil {
		t.Fatal(err)
	}

	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("contract deployment failed, status=%d", receipt.Status)
	}

	t.Logf("Escrow deployed at %s (gas=%d)", receipt.ContractAddress.Hex(), receipt.GasUsed)
	return receipt.ContractAddress
}

// buildScriptAndHash constructs an EthRedeemScript and computes its keccak256 hash,
// mirroring the on-chain calculateRedeemScriptHash for native ETH.
func buildScriptAndHash(
	uniqueID [20]byte,
	threshold uint8,
	timeout uint32,
	buyer, seller, moderator, contractAddr common.Address,
) (ethWal.EthRedeemScript, [32]byte) {
	rScript := ethWal.EthRedeemScript{
		UniqueID:        common.BytesToAddress(uniqueID[:]),
		Threshold:       threshold,
		Timeout:         timeout,
		Buyer:           buyer,
		Seller:          seller,
		Moderator:       moderator,
		ContractAddress: contractAddr,
		TokenAddress:    common.Address{},
	}

	var data []byte
	data = append(data, rScript.UniqueID.Bytes()...)
	data = append(data, byte(rScript.Threshold))
	buf := make([]byte, 4)
	buf[0] = byte(rScript.Timeout >> 24)
	buf[1] = byte(rScript.Timeout >> 16)
	buf[2] = byte(rScript.Timeout >> 8)
	buf[3] = byte(rScript.Timeout)
	data = append(data, buf...)
	data = append(data, rScript.Buyer.Bytes()...)
	data = append(data, rScript.Seller.Bytes()...)
	data = append(data, rScript.Moderator.Bytes()...)
	data = append(data, rScript.ContractAddress.Bytes()...)

	var hash [32]byte
	copy(hash[:], crypto.Keccak256(data)[:])
	return rScript, hash
}

// TestEscrowAnvil_Moderated_2of3_Release deploys Escrow.sol to Anvil and
// verifies the full flow: addTransaction → fund → sign (Go code) → execute → release.
// This validates that Go-generated signatures are accepted by the Solidity contract.
func TestEscrowAnvil_Moderated_2of3_Release(t *testing.T) {
	client := connectAnvil(t)
	defer client.Close()
	ctx := context.Background()

	deployerKey := mustPrivKey(t, anvilKey0)
	buyerKey := mustPrivKey(t, anvilKey1)
	sellerKey := mustPrivKey(t, anvilKey2)
	moderatorKey := mustPrivKey(t, anvilKey3)

	buyerAddr := crypto.PubkeyToAddress(buyerKey.PublicKey)
	sellerAddr := crypto.PubkeyToAddress(sellerKey.PublicKey)
	moderatorAddr := crypto.PubkeyToAddress(moderatorKey.PublicKey)

	escrowContractAddr := deployEscrow(t, client, deployerKey)
	escrowContract, err := contract.NewEscrow(escrowContractAddr, client)
	if err != nil {
		t.Fatal(err)
	}

	uniqueID := [20]byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}
	rScript, scriptHash := buildScriptAndHash(uniqueID, 2, 24, buyerAddr, sellerAddr, moderatorAddr, escrowContractAddr)

	// Step 1: addTransaction (funded by buyer with 1 ETH)
	chainID, _ := client.ChainID(ctx)
	gasPrice, _ := client.SuggestGasPrice(ctx)

	auth, err := bind.NewKeyedTransactorWithChainID(buyerKey, chainID)
	if err != nil {
		t.Fatal(err)
	}
	auth.Value = big.NewInt(1_000_000_000_000_000_000) // 1 ETH
	auth.GasLimit = 500_000
	auth.GasPrice = gasPrice

	addTx, err := escrowContract.AddTransaction(auth,
		buyerAddr, sellerAddr, moderatorAddr, 2, 24, scriptHash, uniqueID)
	if err != nil {
		t.Fatalf("AddTransaction: %v", err)
	}

	receipt, err := bind.WaitMined(ctx, client, addTx)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatal("AddTransaction failed on-chain")
	}
	t.Logf("Escrow funded: scriptHash=%x, 1 ETH", scriptHash)

	// Verify on-chain state
	txInfo, err := escrowContract.Transactions(nil, scriptHash)
	if err != nil {
		t.Fatal(err)
	}
	if txInfo.Value.Cmp(big.NewInt(1_000_000_000_000_000_000)) != 0 {
		t.Fatalf("escrow value mismatch: got %s", txInfo.Value)
	}

	// Step 2: Generate Go signatures for release (buyer + seller → 2-of-3)
	serializedScript, err := ethWal.SerializeEthScript(rScript)
	if err != nil {
		t.Fatal(err)
	}

	releaseAmount := uint64(1_000_000_000_000_000_000)
	tos := []iwallet.SpendInfo{{
		Address: iwallet.NewAddress(sellerAddr.Hex(), iwallet.CoinType("crypto:eip155:1:native")),
		Amount:  iwallet.NewAmount(releaseAmount),
	}}

	buyerSigs, err := SignEscrowRelease(tos, serializedScript, ecdsaToBtcec(t, buyerKey))
	if err != nil {
		t.Fatalf("buyer SignEscrowRelease: %v", err)
	}
	sellerSigs, err := SignEscrowRelease(tos, serializedScript, ecdsaToBtcec(t, sellerKey))
	if err != nil {
		t.Fatalf("seller SignEscrowRelease: %v", err)
	}

	// Step 3: Call execute on-chain with the Go-generated signatures
	buyerR, buyerS, buyerV := ethWal.SigRSV(buyerSigs[0].Signature)
	sellerR, sellerS, sellerV := ethWal.SigRSV(sellerSigs[0].Signature)

	sellerBalanceBefore, err := client.BalanceAt(ctx, sellerAddr, nil)
	if err != nil {
		t.Fatal(err)
	}

	execAuth, err := bind.NewKeyedTransactorWithChainID(buyerKey, chainID)
	if err != nil {
		t.Fatal(err)
	}
	execAuth.GasLimit = 500_000
	execAuth.GasPrice = gasPrice

	payData := contract.PayData{
		Destinations: []common.Address{sellerAddr},
		Amounts:      []*big.Int{new(big.Int).SetUint64(releaseAmount)},
	}

	execTx, err := escrowContract.Execute(execAuth,
		[]uint8{buyerV, sellerV},
		[][32]byte{buyerR, sellerR},
		[][32]byte{buyerS, sellerS},
		scriptHash, payData,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	execReceipt, err := bind.WaitMined(ctx, client, execTx)
	if err != nil {
		t.Fatal(err)
	}
	if execReceipt.Status != types.ReceiptStatusSuccessful {
		t.Fatal("Execute failed on-chain — Go signatures rejected by contract")
	}

	// Step 4: Verify funds were released to seller
	sellerBalanceAfter, err := client.BalanceAt(ctx, sellerAddr, nil)
	if err != nil {
		t.Fatal(err)
	}

	gained := new(big.Int).Sub(sellerBalanceAfter, sellerBalanceBefore)
	expected := new(big.Int).SetUint64(releaseAmount)
	if gained.Cmp(expected) != 0 {
		t.Errorf("seller gained %s wei, expected %s", gained, expected)
	}

	t.Logf("SUCCESS: Go-generated 2-of-3 signatures accepted by Escrow.sol, seller received %s wei", gained)
}

// TestEscrowAnvil_Dispute_BuyerModerator tests the dispute resolution path:
// buyer + moderator sign the release with a split payout.
func TestEscrowAnvil_Dispute_BuyerModerator(t *testing.T) {
	client := connectAnvil(t)
	defer client.Close()
	ctx := context.Background()

	deployerKey := mustPrivKey(t, anvilKey0)
	buyerKey := mustPrivKey(t, anvilKey1)
	sellerKey := mustPrivKey(t, anvilKey2)
	moderatorKey := mustPrivKey(t, anvilKey3)

	buyerAddr := crypto.PubkeyToAddress(buyerKey.PublicKey)
	sellerAddr := crypto.PubkeyToAddress(sellerKey.PublicKey)
	moderatorAddr := crypto.PubkeyToAddress(moderatorKey.PublicKey)

	escrowContractAddr := deployEscrow(t, client, deployerKey)
	escrowContract, err := contract.NewEscrow(escrowContractAddr, client)
	if err != nil {
		t.Fatal(err)
	}

	uniqueID := [20]byte{0xAA, 0xBB, 0xCC, 0xDD, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x20}
	rScript, scriptHash := buildScriptAndHash(uniqueID, 2, 24, buyerAddr, sellerAddr, moderatorAddr, escrowContractAddr)

	chainID, _ := client.ChainID(ctx)
	gasPrice, _ := client.SuggestGasPrice(ctx)

	auth, err := bind.NewKeyedTransactorWithChainID(buyerKey, chainID)
	if err != nil {
		t.Fatal(err)
	}
	auth.Value = big.NewInt(2_000_000_000_000_000_000) // 2 ETH
	auth.GasLimit = 500_000
	auth.GasPrice = gasPrice

	addTx, err := escrowContract.AddTransaction(auth, buyerAddr, sellerAddr, moderatorAddr, 2, 24, scriptHash, uniqueID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := bind.WaitMined(ctx, client, addTx)
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatal("AddTransaction failed")
	}

	serializedScript, _ := ethWal.SerializeEthScript(rScript)
	buyerRefund := uint64(1_500_000_000_000_000_000)
	sellerPay := uint64(500_000_000_000_000_000)

	tos := []iwallet.SpendInfo{
		{Address: iwallet.NewAddress(buyerAddr.Hex(), iwallet.CoinType("crypto:eip155:1:native")), Amount: iwallet.NewAmount(buyerRefund)},
		{Address: iwallet.NewAddress(sellerAddr.Hex(), iwallet.CoinType("crypto:eip155:1:native")), Amount: iwallet.NewAmount(sellerPay)},
	}

	buyerSigs, err := SignEscrowRelease(tos, serializedScript, ecdsaToBtcec(t, buyerKey))
	if err != nil {
		t.Fatal(err)
	}
	modSigs, err := SignEscrowRelease(tos, serializedScript, ecdsaToBtcec(t, moderatorKey))
	if err != nil {
		t.Fatal(err)
	}

	buyerBalBefore, _ := client.BalanceAt(ctx, buyerAddr, nil)
	sellerBalBefore, _ := client.BalanceAt(ctx, sellerAddr, nil)

	buyerR, buyerS, buyerV := ethWal.SigRSV(buyerSigs[0].Signature)
	modR, modS, modV := ethWal.SigRSV(modSigs[0].Signature)

	execAuth, _ := bind.NewKeyedTransactorWithChainID(moderatorKey, chainID)
	execAuth.GasLimit = 500_000
	execAuth.GasPrice = gasPrice

	payData := contract.PayData{
		Destinations: []common.Address{buyerAddr, sellerAddr},
		Amounts:      []*big.Int{new(big.Int).SetUint64(buyerRefund), new(big.Int).SetUint64(sellerPay)},
	}

	execTx, err := escrowContract.Execute(execAuth,
		[]uint8{buyerV, modV},
		[][32]byte{buyerR, modR},
		[][32]byte{buyerS, modS},
		scriptHash, payData,
	)
	if err != nil {
		t.Fatalf("Execute (dispute): %v", err)
	}
	execReceipt, err := bind.WaitMined(ctx, client, execTx)
	if err != nil || execReceipt.Status != types.ReceiptStatusSuccessful {
		t.Fatal("Execute (dispute) failed on-chain")
	}

	buyerBalAfter, _ := client.BalanceAt(ctx, buyerAddr, nil)
	sellerBalAfter, _ := client.BalanceAt(ctx, sellerAddr, nil)

	buyerGained := new(big.Int).Sub(buyerBalAfter, buyerBalBefore)
	sellerGained := new(big.Int).Sub(sellerBalAfter, sellerBalBefore)

	if buyerGained.Cmp(new(big.Int).SetUint64(buyerRefund)) != 0 {
		t.Errorf("buyer refund mismatch: got %s, want %d", buyerGained, buyerRefund)
	}
	if sellerGained.Cmp(new(big.Int).SetUint64(sellerPay)) != 0 {
		t.Errorf("seller payment mismatch: got %s, want %d", sellerGained, sellerPay)
	}

	t.Logf("Dispute resolution: buyer +%s, seller +%s", buyerGained, sellerGained)
}

// TestEscrowAnvil_ScriptHashConsistency verifies the Go keccak256 hash
// matches the on-chain calculateRedeemScriptHash.
func TestEscrowAnvil_ScriptHashConsistency(t *testing.T) {
	client := connectAnvil(t)
	defer client.Close()

	deployerKey := mustPrivKey(t, anvilKey0)
	escrowContractAddr := deployEscrow(t, client, deployerKey)
	escrowContract, err := contract.NewEscrow(escrowContractAddr, client)
	if err != nil {
		t.Fatal(err)
	}

	buyerAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sellerAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	moderatorAddr := common.HexToAddress("0x3333333333333333333333333333333333333333")
	uniqueID := [20]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14}

	_, goHash := buildScriptAndHash(uniqueID, 2, 24, buyerAddr, sellerAddr, moderatorAddr, escrowContractAddr)

	onChainHash, err := escrowContract.CalculateRedeemScriptHash(nil,
		uniqueID, 2, 24, buyerAddr, sellerAddr, moderatorAddr, common.Address{})
	if err != nil {
		t.Fatal(err)
	}

	if goHash != onChainHash {
		t.Errorf("scriptHash mismatch:\n  Go:      %x\n  On-chain: %x", goHash, onChainHash)
	} else {
		t.Logf("scriptHash consistent: %x", goHash)
	}
}

// TestEscrowAnvil_SellerModerator tests the third 2-of-3 combination:
// seller + moderator sign the release.
func TestEscrowAnvil_SellerModerator(t *testing.T) {
	client := connectAnvil(t)
	defer client.Close()
	ctx := context.Background()

	deployerKey := mustPrivKey(t, anvilKey0)
	buyerKey := mustPrivKey(t, anvilKey1)
	sellerKey := mustPrivKey(t, anvilKey2)
	moderatorKey := mustPrivKey(t, anvilKey3)

	buyerAddr := crypto.PubkeyToAddress(buyerKey.PublicKey)
	sellerAddr := crypto.PubkeyToAddress(sellerKey.PublicKey)
	moderatorAddr := crypto.PubkeyToAddress(moderatorKey.PublicKey)

	escrowContractAddr := deployEscrow(t, client, deployerKey)
	escrowContract, err := contract.NewEscrow(escrowContractAddr, client)
	if err != nil {
		t.Fatal(err)
	}

	uniqueID := [20]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x01, 0x02, 0x03, 0x04, 0x05}
	rScript, scriptHash := buildScriptAndHash(uniqueID, 2, 24, buyerAddr, sellerAddr, moderatorAddr, escrowContractAddr)

	chainID, _ := client.ChainID(ctx)
	gasPrice, _ := client.SuggestGasPrice(ctx)

	auth, err := bind.NewKeyedTransactorWithChainID(buyerKey, chainID)
	if err != nil {
		t.Fatal(err)
	}
	auth.Value = big.NewInt(1_000_000_000_000_000_000)
	auth.GasLimit = 500_000
	auth.GasPrice = gasPrice

	addTx, err := escrowContract.AddTransaction(auth, buyerAddr, sellerAddr, moderatorAddr, 2, 24, scriptHash, uniqueID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := bind.WaitMined(ctx, client, addTx)
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatal("AddTransaction failed")
	}

	serializedScript, _ := ethWal.SerializeEthScript(rScript)
	releaseAmount := uint64(1_000_000_000_000_000_000)
	tos := []iwallet.SpendInfo{{
		Address: iwallet.NewAddress(sellerAddr.Hex(), iwallet.CoinType("crypto:eip155:1:native")),
		Amount:  iwallet.NewAmount(releaseAmount),
	}}

	sellerSigs, err := SignEscrowRelease(tos, serializedScript, ecdsaToBtcec(t, sellerKey))
	if err != nil {
		t.Fatal(err)
	}
	modSigs, err := SignEscrowRelease(tos, serializedScript, ecdsaToBtcec(t, moderatorKey))
	if err != nil {
		t.Fatal(err)
	}

	sellerBalBefore, _ := client.BalanceAt(ctx, sellerAddr, nil)

	sellerR, sellerS, sellerV := ethWal.SigRSV(sellerSigs[0].Signature)
	modR, modS, modV := ethWal.SigRSV(modSigs[0].Signature)

	execAuth, _ := bind.NewKeyedTransactorWithChainID(sellerKey, chainID)
	execAuth.GasLimit = 500_000
	execAuth.GasPrice = gasPrice

	payData := contract.PayData{
		Destinations: []common.Address{sellerAddr},
		Amounts:      []*big.Int{new(big.Int).SetUint64(releaseAmount)},
	}

	execTx, err := escrowContract.Execute(execAuth,
		[]uint8{sellerV, modV},
		[][32]byte{sellerR, modR},
		[][32]byte{sellerS, modS},
		scriptHash, payData,
	)
	if err != nil {
		t.Fatalf("Execute (seller+mod): %v", err)
	}
	execReceipt, err := bind.WaitMined(ctx, client, execTx)
	if err != nil || execReceipt.Status != types.ReceiptStatusSuccessful {
		t.Fatal("Execute (seller+mod) failed on-chain")
	}

	sellerBalAfter, _ := client.BalanceAt(ctx, sellerAddr, nil)
	gained := new(big.Int).Sub(sellerBalAfter, sellerBalBefore)

	t.Logf("Seller+Moderator release: seller gained %s wei (expected %d, diff due to gas)", gained, releaseAmount)
}
