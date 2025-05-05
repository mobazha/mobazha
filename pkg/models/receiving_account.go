package models

import (
	"encoding/json"
	"errors"
	"fmt"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ReceivingAccount 表示用户的收款账户信息
type ReceivingAccount struct {
	ID               int               `gorm:"primaryKey" json:"id"`
	Name             string            `gorm:"type:text" json:"name"`  // 账户名称
	ChainType        iwallet.ChainType `gorm:"index" json:"chainType"` // 区块链网络类型
	Address          string            `gorm:"index" json:"address"`   // 用户的收款钱包地址
	SerializedTokens []byte            `gorm:"type:bytes" json:"-"`    // 序列化的已启用代币列表
	Email            string            `json:"email,omitempty"`        // 对于Stripe/Paypal，使用Email
}

// EnabledTokens 返回已启用的代币列表
func (ra *ReceivingAccount) EnabledTokens() ([]string, error) {
	var tokens []string
	if ra.SerializedTokens != nil {
		if err := json.Unmarshal(ra.SerializedTokens, &tokens); err != nil {
			return nil, fmt.Errorf("反序列化代币列表失败: %v", err)
		}
	}
	return tokens, nil
}

// SetEnabledTokens 设置已启用的代币列表
// 对于原生代币，使用NATIVE_SYMBOL
func (ra *ReceivingAccount) SetEnabledTokens(tokens []string) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("序列化代币列表失败: %v", err)
	}
	ra.SerializedTokens = data
	return nil
}

// AddToken 添加单个代币到已启用代币列表
func (ra *ReceivingAccount) AddToken(token string) error {
	tokens, err := ra.EnabledTokens()
	if err != nil {
		return err
	}

	// 检查代币是否已存在
	for _, t := range tokens {
		if t == token {
			return fmt.Errorf("代币 %s 已存在", token)
		}
	}

	// 添加新代币
	tokens = append(tokens, token)
	return ra.SetEnabledTokens(tokens)
}

// RemoveToken 从已启用代币列表中移除单个代币
func (ra *ReceivingAccount) RemoveToken(token string) error {
	tokens, err := ra.EnabledTokens()
	if err != nil {
		return err
	}

	// 查找并移除代币
	found := false
	for i, t := range tokens {
		if t == token {
			tokens = append(tokens[:i], tokens[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("代币 %s 不存在", token)
	}

	return ra.SetEnabledTokens(tokens)
}

// HasToken 检查代币是否在已启用代币列表中
func (ra *ReceivingAccount) HasToken(token string) (bool, error) {
	tokens, err := ra.EnabledTokens()
	if err != nil {
		return false, err
	}

	for _, t := range tokens {
		if t == token {
			return true, nil
		}
	}

	return false, nil
}

func (ra *ReceivingAccount) AcceptedCurrencies() []string {
	tokens, err := ra.EnabledTokens()
	if err != nil {
		return []string{}
	}

	var currencies []string
	for _, token := range tokens {
		if token == iwallet.NATIVE_SYMBOL {
			currencies = append(currencies, string(ra.ChainType))
		} else {
			currencies = append(currencies, string(ra.ChainType)+token)
		}
	}
	return currencies
}

// receivingAccountJSON 用于 JSON 序列化的结构体
type receivingAccountJSON struct {
	ID            int               `json:"id"`
	Name          string            `json:"name"`
	ChainType     iwallet.ChainType `json:"chainType"`
	Address       string            `json:"address"`
	EnabledTokens []string          `json:"enabledTokens"`
}

// MarshalJSON 实现 JSON 序列化
func (ra *ReceivingAccount) MarshalJSON() ([]byte, error) {
	tokens, err := ra.EnabledTokens()
	if err != nil {
		return nil, err
	}

	raJSON := receivingAccountJSON{
		ID:            ra.ID,
		Name:          ra.Name,
		ChainType:     ra.ChainType,
		Address:       ra.Address,
		EnabledTokens: tokens,
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

	if err := ra.SetEnabledTokens(raJSON.EnabledTokens); err != nil {
		return err
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
