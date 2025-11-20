package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

// LocalListingCrypto 本地商品加密服务
// 专门负责当前节点商品的加解密操作
// 设计理念：
//   - 只处理本地商品（当前节点发布的商品）
//   - peerID 在构造时固定，无需每次传递
//   - 加密功能直接集成（无额外抽象层）
type LocalListingCrypto struct {
	keyManager *KeyManager
	db         database.Database
	peerID     string // 当前节点 ID（构造时传入）
	algorithm  string // 加密算法，默认 "AES-256-GCM"
}

// NewLocalListingCrypto 创建本地商品加密服务
// peerID: 当前节点的 Peer ID（从 IPFS 节点获取）
func NewLocalListingCrypto(km *KeyManager, db database.Database, peerID string) *LocalListingCrypto {
	return &LocalListingCrypto{
		keyManager: km,
		db:         db,
		peerID:     peerID,
		algorithm:  "AES-256-GCM",
	}
}

// ============================================================================
// 商品加密流程
// ============================================================================

// EncryptListing 加密商品数据
// 注意：只加密数据并存储加密元数据，不存储 listing 本身
// 返回: masterKey（用于同步到 hosting）, encryptedData（用于上传到 IPFS）, error
func (lc *LocalListingCrypto) EncryptListing(
	signedListing *pb.SignedListing,
) (masterKey []byte, encryptedData []byte, err error) {
	slug := signedListing.Listing.Slug

	// 1. 序列化商品数据（JSON 格式）
	m := protojson.MarshalOptions{EmitUnpopulated: false}
	ser := m.Format(signedListing)
	var out bytes.Buffer
	json.Indent(&out, []byte(ser), "", "    ")
	plaintextData := out.Bytes()

	// 2. 生成或获取商品主密钥（使用构造时传入的 peerID）
	masterKey, err = lc.getOrGenerateListingKey(slug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get listing key: %w", err)
	}

	// 3. 加密商品数据
	encryptedData, err = lc.encryptData(plaintextData, masterKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt listing: %w", err)
	}

	// 4. 存储加密元数据
	if err := lc.storeEncryptionMeta(slug, true); err != nil {
		return nil, nil, fmt.Errorf("failed to store encryption metadata: %w", err)
	}

	// 5. 返回密钥和加密数据，由调用方上传到 IPFS
	// 注意：CID 由 ListingMetadata 或 IPNS 管理，不在加密元数据中存储
	return masterKey, encryptedData, nil
}

// getOrGenerateListingKey 获取或派生商品主密钥
// Phase 2.1: 使用基于 HKDF 的确定性派生方法
// 密钥从节点私钥派生，支持版本轮换
// 注意：密钥派生只依赖 slug 和 peerID，与商品是否公开无关
func (lc *LocalListingCrypto) getOrGenerateListingKey(slug string) ([]byte, error) {
	// 使用确定性派生方法（KeyManager 内部使用构造时的 peerID）
	masterKey, err := lc.keyManager.GetOrDeriveListingKey(slug)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	return masterKey, nil
}

// storeEncryptionMeta 存储加密元数据
// 只存储加密状态、算法和版本信息
// 注意：peerID 不存储在元数据中，本地数据库只有当前节点商品
func (lc *LocalListingCrypto) storeEncryptionMeta(slug string, isEncrypted bool) error {
	// 获取当前密钥版本
	version, err := lc.keyManager.GetCurrentKeyVersion(slug)
	if err != nil {
		version = 1 // 默认版本
	}

	meta := models.ListingEncryptionMeta{
		Slug:        slug,
		IsEncrypted: isEncrypted,
		Algorithm:   lc.algorithm,
		KeyVersion:  version,
	}

	return lc.db.Update(func(tx database.Tx) error {
		return tx.Save(&meta)
	})
}

// ============================================================================
// 商品解密流程
// ============================================================================

// DecryptListing 解密本地商品数据（自动派生密钥）
func (lc *LocalListingCrypto) DecryptListing(encryptedData []byte, slug string) ([]byte, error) {
	// 1. 获取商品主密钥（KeyManager 内部使用构造时的 peerID）
	masterKey, err := lc.keyManager.GetOrDeriveListingKey(slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get listing key: %w", err)
	}

	// 2. 解密数据
	plaintextData, err := lc.decryptData(encryptedData, masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt listing: %w", err)
	}

	return plaintextData, nil
}

// DecryptListingWithKey 使用提供的密钥解密商品数据（用于远程商品）
// 用途：解密从其他节点获取的加密商品，使用从 mobazha_hosting 获取的密钥
func (lc *LocalListingCrypto) DecryptListingWithKey(encryptedData []byte, masterKey []byte) ([]byte, error) {
	return lc.decryptData(encryptedData, masterKey)
}

// TryDecryptListingData 尝试解密商品数据（支持向后兼容）
// 如果数据未加密，直接返回原数据
func (lc *LocalListingCrypto) TryDecryptListingData(data []byte, slug string) ([]byte, error) {
	// 1. 检查是否有加密元数据
	var meta models.ListingEncryptionMeta
	err := lc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("slug = ?", slug).First(&meta).Error
	})

	if err != nil {
		// 没有加密元数据 → 旧的未加密商品
		return data, nil
	}

	if !meta.IsEncrypted {
		// 明确标记为未加密（迁移过程中的过渡状态）
		return data, nil
	}

	// 2. 尝试解密
	decryptedData, err := lc.DecryptListing(data, slug)
	if err != nil {
		// 3. 解密失败回退：可能元数据错误，数据实际未加密
		var testListing pb.SignedListing
		if unmarshalErr := protojson.Unmarshal(data, &testListing); unmarshalErr == nil {
			// 数据可以作为 JSON 解析，可能是未加密的
			// 更新元数据标记为未加密
			_ = lc.updateEncryptionMetaFlag(slug, false)
			return data, nil
		}
		return nil, fmt.Errorf("decryption failed and data is corrupted: %w", err)
	}

	return decryptedData, nil
}

// updateEncryptionMetaFlag 更新加密元数据的 is_encrypted 标志
func (lc *LocalListingCrypto) updateEncryptionMetaFlag(slug string, isEncrypted bool) error {
	return lc.db.Update(func(tx database.Tx) error {
		where := map[string]interface{}{
			"slug = ?": slug,
		}
		return tx.Update("is_encrypted", isEncrypted, where, &models.ListingEncryptionMeta{})
	})
}

// ============================================================================
// 元数据查询
// ============================================================================

// GetEncryptionMeta 获取商品的加密元数据
func (lc *LocalListingCrypto) GetEncryptionMeta(slug string) (*models.ListingEncryptionMeta, error) {
	var meta models.ListingEncryptionMeta
	err := lc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("slug = ?", slug).First(&meta).Error
	})
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// IsListingEncrypted 检查商品是否已加密
func (lc *LocalListingCrypto) IsListingEncrypted(slug string) bool {
	meta, err := lc.GetEncryptionMeta(slug)
	if err != nil {
		return false
	}
	return meta.IsEncrypted
}

// ============================================================================
// 密钥轮换
// ============================================================================

// RotateListingKey 轮换商品密钥
// 递增密钥版本号，下次加密时自动使用新密钥
// 注意：需要重新加密商品数据并上传到 IPFS
//
// 返回:
//   - 新的密钥版本号
//   - 新的主密钥
//   - 错误信息
func (lc *LocalListingCrypto) RotateListingKey(slug string) (int, []byte, error) {
	// 1. 轮换密钥版本
	newVersion, err := lc.keyManager.RotateListingKey(slug)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to rotate key: %w", err)
	}

	// 2. 派生新密钥（KeyManager 内部使用构造时的 peerID）
	newKey, err := lc.keyManager.DeriveListingMasterKey(slug, newVersion)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to derive new key: %w", err)
	}

	return newVersion, newKey, nil
}

// GetKeyVersion 获取商品的当前密钥版本
func (lc *LocalListingCrypto) GetKeyVersion(slug string) (int, error) {
	return lc.keyManager.GetCurrentKeyVersion(slug)
}

// ============================================================================
// 私有加密/解密方法（AES-256-GCM）
// ============================================================================

// encryptData 使用 AES-256-GCM 加密数据
// 私有方法，只在 ListingEncryptionService 内部使用
func (lc *LocalListingCrypto) encryptData(plaintext, key []byte) ([]byte, error) {
	// 1. 验证密钥长度（AES-256 需要 32 字节）
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(key))
	}

	// 2. 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// 3. 创建 GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 4. 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 5. 加密（nonce 会被添加到 ciphertext 前面）
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// decryptData 使用 AES-256-GCM 解密数据
// 私有方法，只在 ListingEncryptionService 内部使用
func (lc *LocalListingCrypto) decryptData(ciphertext, key []byte) ([]byte, error) {
	// 1. 验证密钥长度
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(key))
	}

	// 2. 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// 3. 创建 GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 4. 验证密文长度
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// 5. 提取 nonce 和实际密文
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 6. 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
