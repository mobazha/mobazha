package models

import "time"

// ListingEncryptionMeta 商品加密元数据
// 说明：每个商品只有一条记录，主键为 Slug
// 用途：记录商品加密状态、使用的算法、密钥版本等基本信息
//
// 设计要点：
//   - 本地数据库只存储当前节点的商品，无需 peer_id 字段
//   - 使用 HKDF 从节点私钥派生商品密钥，无需存储实际密钥
//   - key_version 支持密钥轮换（递增版本号即可生成新密钥）
//   - is_encrypted 用于向后兼容未加密的旧数据
//   - CID 不在此表存储（由 ListingMetadata 或 IPNS 动态获取）
//
// 密钥派生公式：
//
//	masterKey = HKDF(nodePrivKey, salt, info="listing:peerID:slug:vN")
//	其中 peerID 从节点上下文获取，N 为 key_version
type ListingEncryptionMeta struct {
	Slug        string    `gorm:"primary_key"`           // 商品唯一标识
	IsEncrypted bool      `gorm:"default:true"`          // 是否加密（兼容旧数据）
	Algorithm   string    `gorm:"default:'AES-256-GCM'"` // 加密算法
	KeyVersion  int       `gorm:"default:1"`             // 密钥版本号（支持密钥轮换）
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (ListingEncryptionMeta) TableName() string {
	return "listing_encryption_meta"
}
