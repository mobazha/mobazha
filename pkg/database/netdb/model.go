package netdb

import (
	"sync"
	"time"
)

// Nounce For empty http body with TrackingID signature for verification
type Nounce struct {
	PeerID string

	TrackingID string

	Sig []byte
}

type Profile struct {
	PeerID            string `gorm:"primaryKey"`
	SerializedProfile []byte

	Sig []byte `gorm:"-"`
}

type Followers struct {
	PeerID string `gorm:"primaryKey"`

	SerializedFollowers []byte

	Sig []byte `gorm:"-"`
}

type Following struct {
	PeerID string `gorm:"primaryKey"`

	SerializedFollowing []byte

	Sig []byte `gorm:"-"`
}

type ListingIndex struct {
	PeerID string `gorm:"primaryKey"`

	SerializedIndex []byte

	Sig []byte `gorm:"-"`
}

type Listing struct {
	CID               string `gorm:"primaryKey"`
	PeerID            string
	Slug              string
	SerializedListing []byte

	Sig []byte `gorm:"-"`
}

type RatingIndex struct {
	PeerID string `gorm:"primaryKey"`

	SerializedIndex []byte

	Sig []byte `gorm:"-"`
}

// StripeConfig 存储 Stripe 配置信息
type StripeConfig struct {
	PublicKey  string    `gorm:"column:public_key;type:varchar(255)"`
	SecretKey  string    `gorm:"column:secret_key;type:varchar(255)"`
	WebhookKey string    `gorm:"column:webhook_key;type:varchar(255)"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

// TableName 指定表名
func (StripeConfig) TableName() string {
	return "stripe_config"
}

// StripeConfigCache 管理 Stripe 配置缓存
type StripeConfigCache struct {
	config    *StripeConfig
	updatedAt time.Time
	mu        sync.RWMutex
}

func NewStripeConfigCache() *StripeConfigCache {
	return &StripeConfigCache{
		config:    nil,
		updatedAt: time.Time{},
		mu:        sync.RWMutex{},
	}
}

// GetConfig 获取缓存的配置
func (c *StripeConfigCache) GetConfig() *StripeConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查缓存是否过期（5分钟）
	if c.config != nil && time.Since(c.updatedAt) < time.Minute*5 {
		return c.config
	}
	return nil
}

// SetConfig 设置缓存的配置
func (c *StripeConfigCache) SetConfig(config *StripeConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	c.updatedAt = time.Now()
}
