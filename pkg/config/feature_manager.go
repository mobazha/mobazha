package config

import (
	"fmt"
	"sync"
)

// Toggle 表示一个功能开关
type Toggle struct {
	name    Feature
	enabled bool
}

// FeatureManager 管理所有功能开关
type FeatureManager struct {
	toggles map[Feature]*Toggle
	mu      sync.RWMutex
}

// 全局 FeatureManager 实例
var (
	globalFeatureManager     *FeatureManager
	globalFeatureManagerOnce sync.Once
)

// GetGlobalFeatureManager 获取全局 FeatureManager 实例
func GetGlobalFeatureManager() *FeatureManager {
	globalFeatureManagerOnce.Do(func() {
		globalFeatureManager = NewFeatureManager()
	})
	return globalFeatureManager
}

// NewFeatureManager 创建一个新的功能开关管理器
func NewFeatureManager() *FeatureManager {
	fm := &FeatureManager{
		toggles: make(map[Feature]*Toggle),
	}
	fm.RegisterToggle(FeatureLocalEncryptedStorage, false)

	return fm
}

// RegisterToggle 注册一个新的功能开关
func (m *FeatureManager) RegisterToggle(name Feature, defaultEnabled bool) *Toggle {
	m.mu.Lock()
	defer m.mu.Unlock()

	if toggle, exists := m.toggles[name]; exists {
		return toggle
	}

	toggle := &Toggle{
		name:    name,
		enabled: defaultEnabled,
	}
	m.toggles[name] = toggle
	return toggle
}

// Enable 启用指定的功能
func (m *FeatureManager) Enable(name Feature) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if toggle, exists := m.toggles[name]; exists {
		toggle.enabled = true
		return nil
	}
	return fmt.Errorf("feature flag %s does not exist", name)
}

// IsEnabled 检查功能是否启用
func (m *FeatureManager) IsEnabled(name Feature) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if toggle, exists := m.toggles[name]; exists {
		return toggle.enabled
	}
	return false
}

// GetAllToggles 获取所有功能开关的状态
func (m *FeatureManager) GetAllToggles() map[Feature]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[Feature]bool)
	for name, toggle := range m.toggles {
		result[name] = toggle.enabled
	}
	return result
}
