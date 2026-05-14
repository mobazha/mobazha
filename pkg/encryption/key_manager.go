package encryption

import (
	"crypto/sha256"
	"fmt"
	"io"
	"sync"
	"time"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"golang.org/x/crypto/hkdf"
)

// KeyManager 管理商品加密密钥和产品组密钥
// 说明：KeyManager 绑定到当前节点，所有密钥派生都使用当前节点的私钥和 peerID
type KeyManager struct {
	db          database.Database
	nodePrivKey crypto.PrivKey
	peerID      string // 当前节点的 Peer ID（构造时传入）

	// 密钥缓存（提高性能）
	listingKeyCache sync.Map // map[string][]byte: "slug" -> masterKey

	cacheTTL time.Duration
}

// NewKeyManager 创建新的密钥管理器
// 参数:
//   - db: 数据库实例
//   - nodePrivKey: 当前节点的私钥（用于密钥派生）
//   - peerID: 当前节点的 Peer ID（用于密钥派生的 info 参数）
func NewKeyManager(db database.Database, nodePrivKey crypto.PrivKey, peerID string) *KeyManager {
	return &KeyManager{
		db:          db,
		nodePrivKey: nodePrivKey,
		peerID:      peerID,
		cacheTTL:    5 * time.Minute, // 密钥缓存 5 分钟
	}
}

// ============================================================================
// 商品主密钥管理
// ============================================================================

// DeriveListingMasterKey 从节点私钥派生商品主密钥（推荐方法）
// 使用 HKDF (HMAC-based Key Derivation Function) 确保安全性和确定性
// 支持密钥版本，实现密钥轮换功能
//
// 参数:
//   - slug: 商品唯一标识
//   - version: 密钥版本号（1, 2, 3...），用于支持密钥轮换
//
// 返回:
//   - 32字节的 AES-256 密钥
//   - 错误信息（如果派生失败）
//
// 特性:
//   - 确定性：相同输入产生相同输出
//   - 与节点身份强关联：基于节点私钥派生
//   - 支持灾难恢复：只要有节点私钥和版本号就能恢复密钥
//   - 支持密钥轮换：通过递增版本号实现
//
// 注意：peerID 使用构造时传入的当前节点 ID
func (km *KeyManager) DeriveListingMasterKey(slug string, version int) ([]byte, error) {
	// 1. 获取节点私钥的原始字节
	nodeKeyBytes, err := km.nodePrivKey.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get node private key: %w", err)
	}

	// 2. 构造派生上下文（info）
	// 包含 peerID、slug 和版本号，确保不同商品和不同版本有不同的密钥
	info := []byte(fmt.Sprintf("listing:%s:%s:v%d", km.peerID, slug, version))

	// 3. 固定 salt（应用级常量）
	// salt 提供额外的安全性，防止彩虹表攻击
	salt := []byte("mobazha-phase2-listing-encryption-v1")

	// 4. 使用 HKDF 派生密钥
	kdf := hkdf.New(sha256.New, nodeKeyBytes, salt, info)

	// 5. 读取 32 字节（AES-256）
	masterKey := make([]byte, 32)
	if _, err := io.ReadFull(kdf, masterKey); err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	return masterKey, nil
}

// GetCurrentKeyVersion 获取商品的当前密钥版本号
// 从 ListingEncryptionMeta 表中读取，如果不存在则返回默认版本 1
// 注意：本地数据库只存储当前节点的商品，无需 peerID 参数
func (km *KeyManager) GetCurrentKeyVersion(slug string) (int, error) {
	var meta models.ListingEncryptionMeta
	err := km.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("slug = ?", slug).
			First(&meta).Error
	})

	if err != nil {
		// 商品不存在或首次创建，返回默认版本 1
		return 1, nil
	}

	// 版本号至少为 1
	if meta.KeyVersion < 1 {
		return 1, nil
	}

	return meta.KeyVersion, nil
}

// GetOrDeriveListingKey 获取或派生商品主密钥（推荐使用）
// 自动获取当前密钥版本并派生对应的密钥
// 支持密钥缓存以提高性能
//
// 注意：peerID 使用构造时传入的当前节点 ID
func (km *KeyManager) GetOrDeriveListingKey(slug string) ([]byte, error) {
	// 1. 优先从缓存获取
	cacheKey := slug
	if cached, ok := km.listingKeyCache.Load(cacheKey); ok {
		return cached.([]byte), nil
	}

	// 2. 获取当前密钥版本
	version, err := km.GetCurrentKeyVersion(slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get key version: %w", err)
	}

	// 3. 派生密钥
	masterKey, err := km.DeriveListingMasterKey(slug, version)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	// 4. 更新缓存
	km.listingKeyCache.Store(cacheKey, masterKey)

	return masterKey, nil
}

// RotateListingKey 轮换商品密钥
// 递增版本号，使下次获取密钥时自动使用新版本
// 注意：调用此方法后需要重新加密商品数据并重新上传
//
// 返回:
//   - 新的密钥版本号
//   - 错误信息（如果更新失败）
func (km *KeyManager) RotateListingKey(slug string) (int, error) {
	// 1. 获取当前版本
	currentVersion, err := km.GetCurrentKeyVersion(slug)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	// 2. 递增版本号
	newVersion := currentVersion + 1

	// 3. 更新数据库中的版本号
	err = km.db.Update(func(tx database.Tx) error {
		where := map[string]interface{}{
			"slug = ?": slug,
		}
		return tx.Update("key_version", newVersion, where, &models.ListingEncryptionMeta{})
	})

	if err != nil {
		return 0, fmt.Errorf("failed to update key version: %w", err)
	}

	// 4. 清除缓存，强制下次重新派生
	km.ClearListingKeyCache(slug)

	return newVersion, nil
}

// ============================================================================
// 缓存管理
// ============================================================================

// ClearCache 清除所有密钥缓存
func (km *KeyManager) ClearCache() {
	km.listingKeyCache = sync.Map{}
}

// ClearListingKeyCache 清除特定商品的密钥缓存
func (km *KeyManager) ClearListingKeyCache(slug string) {
	km.listingKeyCache.Delete(slug)
}
