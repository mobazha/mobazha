package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

var (
	// ErrReceivingAccountNameInUse is returned when the display name is taken on the same chain.
	ErrReceivingAccountNameInUse = errors.New("name already used by another account on this network")
	// ErrReceivingAccountAddressInUse is returned when the address belongs to another account.
	ErrReceivingAccountAddressInUse = errors.New("address already used by another account")
)

// ReceivingAccount 表示用户的收款账户信息
type ReceivingAccount struct {
	TenantMixin
	ID                       int               `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Name                     string            `gorm:"type:text" json:"name"`           // 账户名称
	ChainType                iwallet.ChainType `gorm:"index" json:"chainType"`          // 区块链网络类型
	Address                  string            `gorm:"index" json:"address"`            // 用户的收款钱包地址, 对于Stripe, 则是StripeAccountID
	SerializedActiveTokens   []byte            `json:"-"`
	SerializedInactiveTokens []byte            `json:"-"`
	Source                   string            `json:"source,omitempty"`                // 来源
	IsActive                 bool              `json:"isActive"`                        // 是否激活
	Status                   string            `json:"status,omitempty"`                // 账户状态, 对于Stripe, 则是StripeStatus
	CreatedAt                time.Time         `gorm:"autoCreateTime" json:"createdAt"` // 创建时间
	UpdatedAt                time.Time         `gorm:"autoUpdateTime" json:"updatedAt"` // 更新时间
}

// ActiveTokens 返回已激活的代币列表
func (ra *ReceivingAccount) ActiveTokens() ([]string, error) {
	var tokens []string
	if ra.SerializedActiveTokens != nil {
		if err := json.Unmarshal(ra.SerializedActiveTokens, &tokens); err != nil {
			return nil, fmt.Errorf("反序列化激活代币列表失败: %v", err)
		}
	}
	return tokens, nil
}

// InactiveTokens 返回未激活的代币列表
func (ra *ReceivingAccount) InactiveTokens() ([]string, error) {
	var tokens []string
	if ra.SerializedInactiveTokens != nil {
		if err := json.Unmarshal(ra.SerializedInactiveTokens, &tokens); err != nil {
			return nil, fmt.Errorf("反序列化未激活代币列表失败: %v", err)
		}
	}
	return tokens, nil
}

// SetActiveTokens 设置已激活的代币列表
// 对于原生代币，使用NATIVE_SYMBOL
func (ra *ReceivingAccount) SetActiveTokens(tokens []string) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("序列化激活代币列表失败: %v", err)
	}
	ra.SerializedActiveTokens = data
	return nil
}

// SetInactiveTokens 设置未激活的代币列表
func (ra *ReceivingAccount) SetInactiveTokens(tokens []string) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("序列化未激活代币列表失败: %v", err)
	}
	ra.SerializedInactiveTokens = data
	return nil
}

func (ra *ReceivingAccount) AcceptedCurrencies() []string {
	tokens, err := ra.ActiveTokens()
	if err != nil {
		return []string{}
	}

	var currencies []string
	for _, token := range tokens {
		if token == iwallet.NATIVE_SYMBOL || strings.EqualFold(token, string(ra.ChainType)) {
			currencies = append(currencies, string(ra.ChainType))
		} else {
			currencies = append(currencies, string(ra.ChainType)+token)
		}
	}
	return currencies
}

// receivingAccountJSON 用于 JSON 序列化的结构体
type receivingAccountJSON struct {
	ID             int               `json:"id"`
	Name           string            `json:"name"`
	ChainType      iwallet.ChainType `json:"chainType"`
	Address        string            `json:"address"`
	ActiveTokens   []string          `json:"activeTokens"`
	InactiveTokens []string          `json:"inactiveTokens"`
	Source         string            `json:"source,omitempty"`
	IsActive       bool              `json:"isActive"`
	Status         string            `json:"status,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

// MarshalJSON 实现 JSON 序列化
func (ra *ReceivingAccount) MarshalJSON() ([]byte, error) {
	activeTokens, err := ra.ActiveTokens()
	if err != nil {
		return nil, err
	}

	inactiveTokens, err := ra.InactiveTokens()
	if err != nil {
		return nil, err
	}

	raJSON := receivingAccountJSON{
		ID:             ra.ID,
		Name:           ra.Name,
		ChainType:      ra.ChainType,
		Address:        ra.Address,
		ActiveTokens:   activeTokens,
		InactiveTokens: inactiveTokens,
		Source:         ra.Source,
		IsActive:       ra.IsActive,
		Status:         ra.Status,
		CreatedAt:      ra.CreatedAt,
		UpdatedAt:      ra.UpdatedAt,
	}

	return json.Marshal(raJSON)
}

// UnmarshalJSON 实现 JSON 反序列化
func (ra *ReceivingAccount) UnmarshalJSON(b []byte) error {
	var raJSON receivingAccountJSON
	if err := json.Unmarshal(b, &raJSON); err != nil {
		return err
	}

	ra.ID = raJSON.ID
	ra.Name = raJSON.Name
	ra.ChainType = raJSON.ChainType
	ra.Address = raJSON.Address

	if err := ra.SetActiveTokens(raJSON.ActiveTokens); err != nil {
		return err
	}

	if err := ra.SetInactiveTokens(raJSON.InactiveTokens); err != nil {
		return err
	}

	ra.Source = raJSON.Source
	ra.IsActive = raJSON.IsActive
	ra.Status = raJSON.Status
	ra.CreatedAt = raJSON.CreatedAt
	ra.UpdatedAt = raJSON.UpdatedAt
	return nil
}

// NormalizeActiveTokens replaces token entries that match the chain type
// with the canonical NATIVE_SYMBOL, preventing duplicates like "BTCBTC".
func (ra *ReceivingAccount) NormalizeActiveTokens() error {
	tokens, err := ra.ActiveTokens()
	if err != nil {
		return err
	}
	normalized := false
	for i, token := range tokens {
		if strings.EqualFold(token, string(ra.ChainType)) && token != iwallet.NATIVE_SYMBOL {
			tokens[i] = iwallet.NATIVE_SYMBOL
			normalized = true
		}
	}
	if normalized {
		return ra.SetActiveTokens(tokens)
	}
	return nil
}

// Validate 验证收款账户的必要字段
func (ra *ReceivingAccount) Validate() error {
	if ra.ChainType == "" {
		return errors.New("chain type cannot be empty")
	}
	if ra.Address == "" {
		return errors.New("address cannot be empty")
	}
	if ra.Name == "" {
		return errors.New("name cannot be empty")
	}
	return nil
}

// ValidateForUpdate 验证更新收款账户的必要字段
func (ra *ReceivingAccount) ValidateForUpdate() error {
	if ra.ID == 0 {
		return errors.New("account ID cannot be empty")
	}
	return ra.Validate()
}
