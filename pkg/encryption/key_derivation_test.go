package encryption

import (
	"bytes"
	"testing"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

// TestDeriveListingMasterKey 测试密钥派生功能
func TestDeriveListingMasterKey(t *testing.T) {
	// 生成测试用的节点私钥
	privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerID := "QmTestPeer"
	km := &KeyManager{
		nodePrivKey: privKey,
		peerID:      peerID,
	}

	slug := "test-product"

	t.Run("Derive key with version 1", func(t *testing.T) {
		key1, err := km.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive key: %v", err)
		}

		if len(key1) != 32 {
			t.Errorf("Expected key length 32, got %d", len(key1))
		}
	})

	t.Run("Deterministic - same inputs produce same key", func(t *testing.T) {
		key1, err := km.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive key1: %v", err)
		}

		key2, err := km.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive key2: %v", err)
		}

		if !bytes.Equal(key1, key2) {
			t.Error("Same inputs should produce identical keys (deterministic)")
		}
	})

	t.Run("Different versions produce different keys", func(t *testing.T) {
		keyV1, err := km.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive keyV1: %v", err)
		}

		keyV2, err := km.DeriveListingMasterKey(slug, 2)
		if err != nil {
			t.Fatalf("Failed to derive keyV2: %v", err)
		}

		if bytes.Equal(keyV1, keyV2) {
			t.Error("Different versions should produce different keys")
		}
	})

	t.Run("Different slugs produce different keys", func(t *testing.T) {
		keyA, err := km.DeriveListingMasterKey("product-a", 1)
		if err != nil {
			t.Fatalf("Failed to derive keyA: %v", err)
		}

		keyB, err := km.DeriveListingMasterKey("product-b", 1)
		if err != nil {
			t.Fatalf("Failed to derive keyB: %v", err)
		}

		if bytes.Equal(keyA, keyB) {
			t.Error("Different slugs should produce different keys")
		}
	})

	t.Run("Different peerIDs produce different keys", func(t *testing.T) {
		// 创建两个不同 peerID 的 KeyManager
		kmPeer1 := &KeyManager{
			nodePrivKey: privKey,
			peerID:      "QmPeer1",
		}
		kmPeer2 := &KeyManager{
			nodePrivKey: privKey,
			peerID:      "QmPeer2",
		}

		keyPeer1, err := kmPeer1.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive keyPeer1: %v", err)
		}

		keyPeer2, err := kmPeer2.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive keyPeer2: %v", err)
		}

		if bytes.Equal(keyPeer1, keyPeer2) {
			t.Error("Different peerIDs should produce different keys")
		}
	})

	t.Run("Different node keys produce different keys", func(t *testing.T) {
		// 创建另一个节点私钥
		privKey2, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			t.Fatalf("Failed to generate second key pair: %v", err)
		}

		km2 := &KeyManager{
			nodePrivKey: privKey2,
			peerID:      peerID,
		}

		keyNode1, err := km.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive keyNode1: %v", err)
		}

		keyNode2, err := km2.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive keyNode2: %v", err)
		}

		if bytes.Equal(keyNode1, keyNode2) {
			t.Error("Different node keys should produce different listing keys")
		}
	})
}

// TestKeyRotation 测试密钥轮换功能
func TestKeyRotation(t *testing.T) {
	privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerID := "QmTestPeer"
	km := &KeyManager{
		nodePrivKey: privKey,
		peerID:      peerID,
	}

	slug := "test-product"

	t.Run("Version increment produces new key", func(t *testing.T) {
		// 模拟密钥轮换：从版本 1 到版本 2
		keyV1, err := km.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to derive v1 key: %v", err)
		}

		// 轮换后使用版本 2
		keyV2, err := km.DeriveListingMasterKey(slug, 2)
		if err != nil {
			t.Fatalf("Failed to derive v2 key: %v", err)
		}

		if bytes.Equal(keyV1, keyV2) {
			t.Error("Key rotation should produce a different key")
		}

		// 验证版本 1 的密钥仍然可以重新派生（用于解密历史数据）
		keyV1Again, err := km.DeriveListingMasterKey(slug, 1)
		if err != nil {
			t.Fatalf("Failed to re-derive v1 key: %v", err)
		}

		if !bytes.Equal(keyV1, keyV1Again) {
			t.Error("Old version keys should still be derivable (for decrypting old data)")
		}
	})
}

// TestKeyDerivationConsistency 测试密钥派生的一致性（用于灾难恢复验证）
func TestKeyDerivationConsistency(t *testing.T) {
	// 生成固定的种子（模拟已知的节点私钥）
	// Ed25519 私钥需要 64 字节（32 字节种子 + 32 字节公钥）
	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = byte(i)
	}

	privKey, err := crypto.UnmarshalEd25519PrivateKey(seed)
	if err != nil {
		t.Fatalf("Failed to unmarshal key: %v", err)
	}

	peerID := "QmRecoveryTest"
	km := &KeyManager{
		nodePrivKey: privKey,
		peerID:      peerID,
	}

	slug := "disaster-recovery-test"

	// 派生密钥
	key1, err := km.DeriveListingMasterKey(slug, 1)
	if err != nil {
		t.Fatalf("Failed to derive key: %v", err)
	}

	// 模拟灾难恢复：使用相同的节点私钥重新创建 KeyManager
	privKey2, err := crypto.UnmarshalEd25519PrivateKey(seed)
	if err != nil {
		t.Fatalf("Failed to unmarshal key again: %v", err)
	}

	km2 := &KeyManager{
		nodePrivKey: privKey2,
		peerID:      peerID,
	}

	// 重新派生密钥
	key2, err := km2.DeriveListingMasterKey(slug, 1)
	if err != nil {
		t.Fatalf("Failed to derive key after recovery: %v", err)
	}

	// 验证密钥完全相同
	if !bytes.Equal(key1, key2) {
		t.Error("Disaster recovery should produce identical keys")
	}

	t.Logf("✅ Disaster recovery test passed: keys are identical")
}

// BenchmarkDeriveListingMasterKey 性能基准测试
func BenchmarkDeriveListingMasterKey(b *testing.B) {
	privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		b.Fatalf("Failed to generate key pair: %v", err)
	}

	peerID := "QmBenchmarkPeer"
	km := &KeyManager{
		nodePrivKey: privKey,
		peerID:      peerID,
	}

	slug := "benchmark-product"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := km.DeriveListingMasterKey(slug, 1)
		if err != nil {
			b.Fatalf("Failed to derive key: %v", err)
		}
	}
}
