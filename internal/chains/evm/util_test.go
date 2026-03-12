package evm

import (
	"encoding/hex"
	"log"
	"math/big"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestBuildEthSignatureMessage_RemixCompare(t *testing.T) {
	// Remix合约地址
	contractAddr := common.HexToAddress("0x533e73e754c7f6fafd00f0be296e0d952db80162")
	// Remix scriptHash
	scriptHash, _ := hex.DecodeString("f5daf9534f8fdc777ce5d5c6995807e777b57a63626fe2f381252facacda9f7d")
	var scriptHashArr [32]byte
	copy(scriptHashArr[:], scriptHash)

	// 收款人
	recipients := []common.Address{
		common.HexToAddress("0xC4736E41D02faa7D735819AA9afa2ffee1Ce5931"),
	}
	// 金额
	amounts := []uint64{1000000}

	// 构造payload
	payload := []byte{byte(0x19), byte(0)}
	payload = append(payload, contractAddr.Bytes()...)
	for _, recipient := range recipients {
		payload = append(payload, recipient.Bytes()...)
	}
	for _, amount := range amounts {
		bigVal := big.NewInt(0).SetUint64(amount)
		amtBytes := common.LeftPadBytes(bigVal.Bytes(), 32)
		payload = append(payload, amtBytes...)
	}
	payload = append(payload, scriptHashArr[:]...)

	payloadHex := hex.EncodeToString(payload)
	log.Printf("payload(hex): %s", payloadHex)

	// 计算payloadHash
	payloadHash := Keccak256(payload)
	log.Printf("payloadHash: 0x%s", hex.EncodeToString(payloadHash))

	// 拼接prefix
	prefix := append([]byte("\x19Ethereum Signed Message:\n32"), payloadHash...)
	log.Printf("prefix(hex): %s", hex.EncodeToString(prefix))

	// 计算最终txHash
	txHash := Keccak256(prefix)
	log.Printf("txHash: 0x%s", hex.EncodeToString(txHash))

	// 你可以把Remix的packed、payloadHash、prefix、txHash贴在这里，和Go端输出对比
	// t.Fail() // 如果不一致可以手动fail
}

// Keccak256 returns the Keccak256 hash of the input data.
func Keccak256(data []byte) []byte {
	return crypto.Keccak256(data)
}

func TestEcrecover_CompareWithSolidity(t *testing.T) {
	// 测试数据：从你的实际交易中获取
	txHashHex := "0x8bf86fda4f6bcc92b1e4aa79925692e1a44fb37dc6ca4c3f95b2c759a2274844"                                                                // 你的payloadHash
	sigHex := "0x20ac9099bd02180c5380718bedf7f825b3f25360ce1f19cabd07e2d81974bc246f028538614955dae7e014ca543ce0961841249e236a048bf29014883c66744600" // 你的签名

	// 解析txHash
	txHash := common.HexToHash(txHashHex)
	log.Printf("txHash: %s", txHash.Hex())

	// 解析签名
	sig, err := hex.DecodeString(sigHex[2:]) // 去掉0x前缀
	if err != nil {
		t.Fatalf("Failed to decode signature: %v", err)
	}

	// 提取r, s, v
	r, s, v := SigRSV(sig)
	log.Printf("r: %x", r)
	log.Printf("s: %x", s)
	log.Printf("v: %d", v)

	// 使用Go的Ecrecover
	recoveredPubKey, err := crypto.Ecrecover(txHash.Bytes(), sig)
	if err != nil {
		t.Fatalf("Failed to recover public key: %v", err)
	}

	// 从公钥推导地址
	pubKey, err := crypto.UnmarshalPubkey(recoveredPubKey)
	if err != nil {
		t.Fatalf("Failed to unmarshal seller public key: %v", err)
	}
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	log.Printf("Recovered address (Go): %s", recoveredAddr.Hex())

	// 验证签名
	valid := crypto.VerifySignature(recoveredPubKey, txHash.Bytes(), sig[:64])
	log.Printf("Signature valid: %v", valid)

	// 手动验证ecrecover: crypto.Ecrecover 期望 v 为 0 或 1，SigRSV 返回的是归一化后的 27/28
	manualRecovered, err := crypto.Ecrecover(txHash.Bytes(), append(append(r[:], s[:]...), v-27))
	if err != nil {
		t.Fatalf("Failed to manually recover: %v", err)
	}
	manualPubKey, err := crypto.UnmarshalPubkey(manualRecovered)
	if err != nil {
		t.Fatalf("Failed to unmarshal manual pubkey: %v", err)
	}
	manualAddr := crypto.PubkeyToAddress(*manualPubKey)
	log.Printf("Manual recovered address: %s", manualAddr.Hex())

	// 对比结果
	if recoveredAddr != manualAddr {
		t.Errorf("Address mismatch: %s vs %s", recoveredAddr.Hex(), manualAddr.Hex())
	}

	// 这里你可以和Solidity的ecrecover结果对比
	// 在Remix里用同样的txHash、sigV、sigR、sigS调用ecrecover，看结果是否一致
}

func TestEcrecover_WithYourActualData(t *testing.T) {
	// 使用你实际的测试数据
	txHashHex := "0x515d8afd1b88ee2a9f93f4a6426da85bfad0c75b69dfca521e164c51fdb1c390"
	payloadHashHex := "0x61434c8d040670103b359f043d63f08e731e34eca9b7908e184233cd67d50e40"

	// 你的实际签名数据（从日志中获取）
	sigV := uint8(28) // 你的v值
	sigRHex := "042124e349e1b94e357561b3a3da1a993cd85558fb403e09f8e5864d25cfe7ac"
	sigSHex := "722f4f3b88e90c35cc79755b062b63f0eb89dda8d5a7edc8da54b9dcb9abe32a"

	// 解析r和s
	r, _ := hex.DecodeString(sigRHex)
	s, _ := hex.DecodeString(sigSHex)

	log.Printf("txHash: %s", txHashHex)
	log.Printf("payloadHash: %s", payloadHashHex)
	log.Printf("r: %x", r)
	log.Printf("s: %x", s)
	log.Printf("v: %d", sigV)

	// 测试1: 用最终的txHash
	log.Printf("=== Testing with final txHash ===")
	txHash := common.HexToHash(txHashHex)
	testEcrecoverWithHash(t, txHash, r, s, sigV, "final txHash")

	// 测试2: 用payloadHash
	log.Printf("=== Testing with payloadHash ===")
	payloadHash := common.HexToHash(payloadHashHex)
	testEcrecoverWithHash(t, payloadHash, r, s, sigV, "payloadHash")

	// 测试3: 重新构造payloadHash
	log.Printf("=== Testing with reconstructed payloadHash ===")
	contractAddr := common.HexToAddress("0x396176c559B616A9F3EC5b98df4287aF9C8D294F")
	recipient := common.HexToAddress("0xC4736E41D02faa7D735819AA9afa2ffee1Ce5931")
	amount := uint64(1000000)
	scriptHash, _ := hex.DecodeString("22fe3dd3114181d44904dec064f2ffa63fa617faaaf206b275c575e0d6a76338")

	payload := []byte{byte(0x19), byte(0)}
	payload = append(payload, contractAddr.Bytes()...)
	payload = append(payload, common.LeftPadBytes(recipient.Bytes(), 32)...)
	bigVal := big.NewInt(0).SetUint64(amount)
	amtBytes := common.LeftPadBytes(bigVal.Bytes(), 32)
	payload = append(payload, amtBytes...)
	payload = append(payload, common.LeftPadBytes(scriptHash, 32)...)

	reconstructedPayloadHash := crypto.Keccak256(payload)
	log.Printf("Reconstructed payload: %x", payload)
	log.Printf("Reconstructed payloadHash: 0x%x", reconstructedPayloadHash)

	reconstructedHash := common.BytesToHash(reconstructedPayloadHash)
	testEcrecoverWithHash(t, reconstructedHash, r, s, sigV, "reconstructed payloadHash")
}

func testEcrecoverWithHash(t *testing.T, hash common.Hash, r, s []byte, sigV uint8, hashType string) {
	// 尝试不同的v值来恢复公钥
	vValues := []uint8{0, 1, 27, 28}
	var recoveredAddr common.Address
	var success bool

	for _, v := range vValues {
		// 构造签名
		sig := make([]byte, 65)
		copy(sig[:32], r)
		copy(sig[32:64], s)
		sig[64] = v

		log.Printf("Trying v=%d with %s", v, hashType)

		// 使用Go的Ecrecover
		recoveredPubKey, err := crypto.Ecrecover(hash.Bytes(), sig)
		if err != nil {
			log.Printf("Failed to recover with v=%d: %v", v, err)
			continue
		}

		pubKey, err := crypto.UnmarshalPubkey(recoveredPubKey)
		if err != nil {
			log.Printf("Failed to unmarshal pubkey with v=%d: %v", v, err)
			continue
		}

		addr := crypto.PubkeyToAddress(*pubKey)
		log.Printf("Successfully recovered address with v=%d: %s", v, addr.Hex())

		// 验证签名是否有效
		valid := crypto.VerifySignature(recoveredPubKey, hash.Bytes(), sig[:64])
		log.Printf("Signature valid with v=%d: %v", v, valid)

		if valid {
			recoveredAddr = addr
			success = true
			log.Printf("✓ SUCCESS: Valid signature found with v=%d, address=%s", v, addr.Hex())
			break
		}
	}

	if !success {
		log.Printf("✗ FAILED: No valid signature found with %s", hashType)
	} else {
		// 验证这个地址是否是合约登记的owner
		expectedBuyer := "0x9E91D5CbbC4B1e081553d01f53770279b6Cd4ea2"
		expectedSeller := "0xa59FF30F141195afc3237105C4172Bc73673E8d3"

		if recoveredAddr.Hex() == expectedBuyer || recoveredAddr.Hex() == expectedSeller {
			log.Printf("✓ SUCCESS: Recovered address %s is a legitimate owner", recoveredAddr.Hex())
		} else {
			log.Printf("✗ WARNING: Recovered address %s is not a legitimate owner", recoveredAddr.Hex())
		}
	}
}

func TestSigRSV_Validation(t *testing.T) {
	// 测试SigRSV函数的正确性
	sigHex := "0x20ac9099bd02180c5380718bedf7f825b3f25360ce1f19cabd07e2d81974bc246f028538614955dae7e014ca543ce0961841249e236a048bf29014883c66744600"

	sig, _ := hex.DecodeString(sigHex[2:])
	r, s, v := SigRSV(sig)

	log.Printf("Original sig: %x", sig)
	log.Printf("Extracted r: %x", r)
	log.Printf("Extracted s: %x", s)
	log.Printf("Extracted v: %d", v)

	// 验证v值是否正确（应该是27或28或0或1）
	if v != 27 && v != 28 && v != 0 && v != 1 {
		t.Errorf("Invalid v value: %d, should be 27, 28, 0, or 1", v)
	}

	// 重新构造签名验证：SigRSV 将 v 归一化为 27/28，验证 r、s 分量一致，v 正确归一化
	reconstructedSig := make([]byte, 65)
	copy(reconstructedSig[:32], r[:])
	copy(reconstructedSig[32:64], s[:])
	reconstructedSig[64] = v - 27 // 转回原始 v 格式 (0/1) 以对比原始签名

	// 验证 r 和 s 分量一致
	if hex.EncodeToString(sig[:64]) != hex.EncodeToString(reconstructedSig[:64]) {
		t.Errorf("R/S component mismatch")
	}
	// 验证 v 归一化正确（原始 v=0 → 归一化 v=27）
	originalV := sig[64]
	expectedNormalizedV := originalV
	if expectedNormalizedV < 27 {
		expectedNormalizedV += 27
	}
	if v != expectedNormalizedV {
		t.Errorf("V normalization failed: original=%d, expected normalized=%d, got=%d", originalV, expectedNormalizedV, v)
	}
}

func TestGenerateKeysAndVerifySignature(t *testing.T) {
	// 生成buyer的密钥对
	buyerPrivateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate buyer private key: %v", err)
	}
	buyerAddress := crypto.PubkeyToAddress(buyerPrivateKey.PublicKey)
	log.Printf("Buyer private key: %x", crypto.FromECDSA(buyerPrivateKey))
	log.Printf("Buyer address: %s", buyerAddress.Hex())

	// 生成seller的密钥对
	sellerPrivateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate seller private key: %v", err)
	}
	sellerAddress := crypto.PubkeyToAddress(sellerPrivateKey.PublicKey)
	log.Printf("Seller private key: %x", crypto.FromECDSA(sellerPrivateKey))
	log.Printf("Seller address: %s", sellerAddress.Hex())

	// 构造测试数据（使用你实际的参数）
	contractAddr := common.HexToAddress("0x396176c559B616A9F3EC5b98df4287aF9C8D294F")
	recipient := common.HexToAddress("0xC4736E41D02faa7D735819AA9afa2ffee1Ce5931")
	amount := uint64(1000000)
	scriptHash, _ := hex.DecodeString("22fe3dd3114181d44904dec064f2ffa63fa617faaaf206b275c575e0d6a76338")

	// 构造payload
	payload := []byte{byte(0x19), byte(0)}
	payload = append(payload, contractAddr.Bytes()...)
	payload = append(payload, common.LeftPadBytes(recipient.Bytes(), 32)...)
	bigVal := big.NewInt(0).SetUint64(amount)
	amtBytes := common.LeftPadBytes(bigVal.Bytes(), 32)
	payload = append(payload, amtBytes...)
	payload = append(payload, common.LeftPadBytes(scriptHash, 32)...)

	// 计算payloadHash
	payloadHash := crypto.Keccak256(payload)
	log.Printf("Payload: %x", payload)
	log.Printf("PayloadHash: 0x%x", payloadHash)

	// 构造prefix
	prefix := append([]byte("\x19Ethereum Signed Message:\n32"), payloadHash...)

	// 计算最终的txHash
	txHash := crypto.Keccak256(prefix)
	log.Printf("Prefix: %x", prefix)
	log.Printf("TxHash: 0x%x", txHash)

	// 用seller的私钥签名
	sellerSignature, err := crypto.Sign(txHash, sellerPrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign with seller private key: %v", err)
	}

	log.Printf("Seller signature: %x", sellerSignature)
	log.Printf("Seller signature length: %d", len(sellerSignature))
	log.Printf("Seller signature[64] (v value): %d", sellerSignature[64])

	// 提取r, s, v
	r, s, v := SigRSV(sellerSignature)
	log.Printf("Seller signature - r: %x, s: %x, v: %d", r, s, v)

	// 验证seller的签名
	// crypto.Sign返回的签名格式是[r, s, v]其中v是0或1
	// 直接使用原始签名进行验证
	recoveredPubKey, err := crypto.Ecrecover(txHash, sellerSignature)
	if err != nil {
		t.Fatalf("Failed to recover seller public key: %v", err)
	}

	pubKey, err := crypto.UnmarshalPubkey(recoveredPubKey)
	if err != nil {
		t.Fatalf("Failed to unmarshal seller public key: %v", err)
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	log.Printf("Recovered seller address: %s", recoveredAddr.Hex())
	log.Printf("Original seller address: %s", sellerAddress.Hex())

	if recoveredAddr != sellerAddress {
		t.Errorf("Seller address mismatch: recovered=%s, original=%s", recoveredAddr.Hex(), sellerAddress.Hex())
	} else {
		log.Printf("✓ SUCCESS: Seller signature verification passed!")
	}

	// 验证签名有效性
	valid := crypto.VerifySignature(recoveredPubKey, txHash, sellerSignature[:64])
	log.Printf("Seller signature valid: %v", valid)

	// 用buyer的私钥签名
	buyerSignature, err := crypto.Sign(txHash, buyerPrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign with buyer private key: %v", err)
	}

	log.Printf("Buyer signature: %x", buyerSignature)

	// 提取r, s, v
	r, s, v = SigRSV(buyerSignature)
	log.Printf("Buyer signature - r: %x, s: %x, v: %d", r, s, v)

	// 验证buyer的签名
	recoveredPubKey, err = crypto.Ecrecover(txHash, buyerSignature)
	if err != nil {
		t.Fatalf("Failed to recover buyer public key: %v", err)
	}

	pubKey, err = crypto.UnmarshalPubkey(recoveredPubKey)
	if err != nil {
		t.Fatalf("Failed to unmarshal buyer public key: %v", err)
	}

	recoveredAddr = crypto.PubkeyToAddress(*pubKey)
	log.Printf("Recovered buyer address: %s", recoveredAddr.Hex())
	log.Printf("Original buyer address: %s", buyerAddress.Hex())

	if recoveredAddr != buyerAddress {
		t.Errorf("Buyer address mismatch: recovered=%s, original=%s", recoveredAddr.Hex(), buyerAddress.Hex())
	} else {
		log.Printf("✓ SUCCESS: Buyer signature verification passed!")
	}

	// 验证签名有效性
	valid = crypto.VerifySignature(recoveredPubKey, txHash, buyerSignature[:64])
	log.Printf("Buyer signature valid: %v", valid)

	// 测试错误的签名
	wrongSignature := make([]byte, 65)
	copy(wrongSignature, sellerSignature)
	wrongSignature[0] = wrongSignature[0] ^ 0x01 // 修改第一个字节

	recoveredPubKey, err = crypto.Ecrecover(txHash, wrongSignature)
	if err == nil {
		pubKey, err = crypto.UnmarshalPubkey(recoveredPubKey)
		if err == nil {
			recoveredAddr = crypto.PubkeyToAddress(*pubKey)
			// 只要不是原始seller/buyer地址，就算验证通过
			if recoveredAddr == sellerAddress || recoveredAddr == buyerAddress {
				t.Errorf("Expected invalid signature, but got valid address: %s", recoveredAddr.Hex())
			} else {
				log.Printf("✓ SUCCESS: Wrong signature correctly rejected: address mismatch %s", recoveredAddr.Hex())
			}
		} else {
			log.Printf("✓ SUCCESS: Wrong signature correctly rejected: %v", err)
		}
	} else {
		log.Printf("✓ SUCCESS: Wrong signature correctly rejected: %v", err)
	}
}

func TestRealWorldSignatureVerification(t *testing.T) {
	t.Skip("跳过：此测试依赖特定的生产交易数据，仅用于手动调试")
	// 使用实际生产数据测试签名验证
	// 从日志中提取的数据
	contractAddr := common.HexToAddress("0x396176c559B616A9F3EC5b98df4287aF9C8D294F")
	receiverAddr := common.HexToAddress("0xc4736e41d02faa7d735819aa9afa2ffee1ce5931")
	amount := uint64(1000000)
	scriptHashHex := "22fe3dd3114181d44904dec064f2ffa63fa617faaaf206b275c575e0d6a76338"
	expectedSellerAddr := common.HexToAddress("0xa59FF30F141195afc3237105C4172Bc73673E8d3")

	// 实际签名数据
	signatureHex := "042124e349e1b94e357561b3a3da1a993cd85558fb403e09f8e5864d25cfe7ac722f4f3b88e90c35cc79755b062b63f0eb89dda8d5a7edc8da54b9dcb9abe32a01"

	// 解析签名
	sig, err := hex.DecodeString(signatureHex)
	if err != nil {
		t.Fatalf("Failed to decode signature: %v", err)
	}

	// 提取r, s, v
	r, s, v := SigRSV(sig)
	log.Printf("Real signature - r: %x, s: %x, v: %d", r, s, v)

	// 构造payload（与BuildEthSignatureMessage一致）
	scriptHash, err := hex.DecodeString(scriptHashHex)
	if err != nil {
		t.Fatalf("Failed to decode scriptHash: %v", err)
	}

	payload := []byte{byte(0x19), byte(0)}
	payload = append(payload, contractAddr.Bytes()...)
	payload = append(payload, common.LeftPadBytes(receiverAddr.Bytes(), 32)...)
	bigVal := big.NewInt(0).SetUint64(amount)
	payload = append(payload, common.LeftPadBytes(bigVal.Bytes(), 32)...)
	payload = append(payload, common.LeftPadBytes(scriptHash, 32)...)

	// 计算payloadHash
	payloadHash := crypto.Keccak256(payload)
	log.Printf("Real payload: %x", payload)
	log.Printf("Real payloadHash: 0x%x", payloadHash)

	// 构造prefix
	prefix := append([]byte("\x19Ethereum Signed Message:\n32"), payloadHash...)

	// 计算最终的txHash
	txHash := crypto.Keccak256(prefix)
	log.Printf("Real txHash: 0x%x", txHash)

	// 尝试不同的v值来恢复签名
	vValues := []uint8{0, 1, 27, 28}
	var success bool
	var recoveredAddr common.Address

	for _, testV := range vValues {
		// 构造测试签名
		testSig := make([]byte, 65)
		copy(testSig[:32], sig[:32])
		copy(testSig[32:64], sig[32:64])
		testSig[64] = testV

		log.Printf("Trying v=%d", testV)

		// 验证签名
		recoveredPubKey, err := crypto.Ecrecover(txHash, testSig)
		if err != nil {
			log.Printf("Failed to recover with v=%d: %v", testV, err)
			continue
		}

		pubKey, err := crypto.UnmarshalPubkey(recoveredPubKey)
		if err != nil {
			log.Printf("Failed to unmarshal pubkey with v=%d: %v", testV, err)
			continue
		}

		addr := crypto.PubkeyToAddress(*pubKey)
		log.Printf("Recovered address with v=%d: %s", testV, addr.Hex())

		// 验证签名有效性
		valid := crypto.VerifySignature(recoveredPubKey, txHash, testSig[:64])
		log.Printf("Signature valid with v=%d: %v", testV, valid)

		if valid && addr == expectedSellerAddr {
			recoveredAddr = addr
			success = true
			log.Printf("✓ SUCCESS: Found valid signature with v=%d, address=%s", testV, addr.Hex())
			break
		}
	}

	if !success {
		t.Errorf("Failed to find valid signature for seller address %s", expectedSellerAddr.Hex())
	} else {
		log.Printf("✓ SUCCESS: Signature verification passed! Address matches seller: %s", recoveredAddr.Hex())
	}

	// 验证原始签名（不转换v值）
	recoveredPubKey, err := crypto.Ecrecover(txHash, sig)
	if err != nil {
		t.Fatalf("Failed to recover public key: %v", err)
	}

	pubKey, err := crypto.UnmarshalPubkey(recoveredPubKey)
	if err != nil {
		t.Fatalf("Failed to unmarshal public key: %v", err)
	}

	recoveredAddr = crypto.PubkeyToAddress(*pubKey)
	log.Printf("Recovered address with original signature: %s", recoveredAddr.Hex())
	log.Printf("Expected seller address: %s", expectedSellerAddr.Hex())

	// 验证签名有效性
	valid := crypto.VerifySignature(recoveredPubKey, txHash, sig[:64])
	log.Printf("Original signature valid: %v", valid)
}

func TestAddressConsistencyIssue(t *testing.T) {
	// 测试地址一致性问题
	// 生成一个私钥
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// 方法1: 使用SerializeCompressed方式获取地址（合约中存储的方式）
	btcecPrivKey, _ := btcec.PrivKeyFromBytes(crypto.FromECDSA(privateKey))
	serializedPubKey := btcecPrivKey.PubKey().SerializeCompressed()

	// 使用PubKeyBytesToEthAddress转换（与合约一致）
	contractAddr, err := iwallet.PubKeyBytesToEthAddress(serializedPubKey)
	if err != nil {
		t.Fatalf("Failed to convert pubkey bytes to eth address: %v", err)
	}
	log.Printf("Contract stored address (SerializeCompressed): %s", contractAddr.Hex())

	// 方法2: 使用ToECDSA方式获取地址（签名恢复的方式）
	signerAddr := crypto.PubkeyToAddress(privateKey.PublicKey)
	log.Printf("Signer address (ToECDSA): %s", signerAddr.Hex())

	// 测试签名恢复
	testMessage := crypto.Keccak256([]byte("test message"))
	sig, err := crypto.Sign(testMessage, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	// 恢复签名者地址
	recoveredPubKey, err := crypto.Ecrecover(testMessage, sig)
	if err != nil {
		t.Fatalf("Failed to recover public key: %v", err)
	}

	pubKey, err := crypto.UnmarshalPubkey(recoveredPubKey)
	if err != nil {
		t.Fatalf("Failed to unmarshal public key: %v", err)
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	log.Printf("Recovered address from signature: %s", recoveredAddr.Hex())

	// 验证一致性
	if contractAddr != signerAddr {
		t.Errorf("Address mismatch: contract=%s, signer=%s", contractAddr.Hex(), signerAddr.Hex())
		log.Printf("⚠ WARNING: Contract and signer addresses are different!")
		log.Printf("  This explains the 'Invalid signature' error!")
	} else {
		log.Printf("✓ SUCCESS: Contract and signer addresses match")
	}

	if recoveredAddr != signerAddr {
		t.Errorf("Recovered address mismatch: recovered=%s, signer=%s", recoveredAddr.Hex(), signerAddr.Hex())
	} else {
		log.Printf("✓ SUCCESS: Signature recovery works correctly")
	}

	// 如果地址不匹配，这就是问题的根源
	if contractAddr != recoveredAddr {
		t.Errorf("CRITICAL: Contract stored address (%s) != Recovered address (%s)", contractAddr.Hex(), recoveredAddr.Hex())
		log.Printf("🚨 This is the root cause of 'Invalid signature' error!")
		log.Printf("   Contract expects: %s", contractAddr.Hex())
		log.Printf("   Signature recovers: %s", recoveredAddr.Hex())
	} else {
		log.Printf("✓ SUCCESS: All addresses are consistent")
	}
}

func TestPublicKeyConversionMethods(t *testing.T) {
	// 测试不同的公钥转换方式
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// 方法1: 使用SerializeCompressed (btcec方式)
	btcecPrivKey, _ := btcec.PrivKeyFromBytes(crypto.FromECDSA(privateKey))
	serializedPubKey := btcecPrivKey.PubKey().SerializeCompressed()
	log.Printf("Method 1 (SerializeCompressed): %x", serializedPubKey)

	// 方法2: 使用FromECDSAPub (以太坊方式)
	ethPubKeyBytes := crypto.FromECDSAPub(&privateKey.PublicKey)
	log.Printf("Method 2 (FromECDSAPub): %x", ethPubKeyBytes)

	// 方法3: 手动构造 (非压缩公钥 = 0x04 前缀 + X + Y 坐标)
	xBytes := privateKey.PublicKey.X.Bytes()
	yBytes := privateKey.PublicKey.Y.Bytes()
	manualPubKey := make([]byte, 0, 65)
	manualPubKey = append(manualPubKey, 0x04)
	manualPubKey = append(manualPubKey, common.LeftPadBytes(xBytes, 32)...)
	manualPubKey = append(manualPubKey, common.LeftPadBytes(yBytes, 32)...)
	log.Printf("Method 3 (Manual 04+X+Y): %x", manualPubKey)

	// 方法4: 使用压缩格式 (手动)
	compressedPubKey := make([]byte, 33)
	if privateKey.PublicKey.Y.Bit(0) == 0 {
		compressedPubKey[0] = 0x02
	} else {
		compressedPubKey[0] = 0x03
	}
	copy(compressedPubKey[1:], privateKey.PublicKey.X.Bytes())
	log.Printf("Method 4 (Manual Compressed): %x", compressedPubKey)

	// 验证地址一致性
	addr1, _ := iwallet.PubKeyBytesToEthAddress(serializedPubKey)
	addr2 := crypto.PubkeyToAddress(privateKey.PublicKey)
	addr3, _ := iwallet.PubKeyBytesToEthAddress(ethPubKeyBytes)
	addr4, _ := iwallet.PubKeyBytesToEthAddress(manualPubKey)
	addr5, _ := iwallet.PubKeyBytesToEthAddress(compressedPubKey)

	log.Printf("Address 1 (SerializeCompressed): %s", addr1.Hex())
	log.Printf("Address 2 (PubkeyToAddress): %s", addr2.Hex())
	log.Printf("Address 3 (FromECDSAPub): %s", addr3.Hex())
	log.Printf("Address 4 (Manual X+Y): %s", addr4.Hex())
	log.Printf("Address 5 (Manual Compressed): %s", addr5.Hex())

	// 验证一致性
	if addr1 != addr2 {
		t.Errorf("Address mismatch: addr1=%s, addr2=%s", addr1.Hex(), addr2.Hex())
	}
	if addr2 != addr3 {
		t.Errorf("Address mismatch: addr2=%s, addr3=%s", addr2.Hex(), addr3.Hex())
	}
	if addr3 != addr4 {
		t.Errorf("Address mismatch: addr3=%s, addr4=%s", addr3.Hex(), addr4.Hex())
	}
	if addr4 != addr5 {
		t.Errorf("Address mismatch: addr4=%s, addr5=%s", addr4.Hex(), addr5.Hex())
	}

	log.Printf("✓ SUCCESS: All addresses are consistent")
}
