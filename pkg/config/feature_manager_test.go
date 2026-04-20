package config

import (
	"testing"
)

// TestFeatureManager_IsEnabled — FF-1 语义：委托 Registry.DefaultValue。
func TestFeatureManager_IsEnabled(t *testing.T) {
	fm := NewFeatureManager()

	if !fm.IsEnabled(FeatureWalletBuiltinDisabled) {
		t.Errorf("FeatureWalletBuiltinDisabled default should be true")
	}
	if fm.IsEnabled(FeaturePrivacyLocalEncryptedStorageEnabled) {
		t.Errorf("FeaturePrivacyLocalEncryptedStorageEnabled default should be false")
	}
	if fm.IsEnabled(FeatureGuestCheckoutEnabled) {
		t.Errorf("FeatureGuestCheckoutEnabled default should be false")
	}
}

// TestFeatureManager_Nil_Safety — 传入 nil 不应 panic。
func TestFeatureManager_Nil_Safety(t *testing.T) {
	fm := NewFeatureManager()
	if fm.IsEnabled(nil) {
		t.Errorf("IsEnabled(nil) should return false")
	}
}

// TestFeatureManager_Snapshot — 返回全部已注册 feature 的默认状态。
func TestFeatureManager_Snapshot(t *testing.T) {
	fm := NewFeatureManager()
	snap := fm.Snapshot()

	// 至少应包含已定义的三个 feature
	required := []string{
		FeatureWalletBuiltinDisabled.Key,
		FeaturePrivacyLocalEncryptedStorageEnabled.Key,
		FeatureGuestCheckoutEnabled.Key,
	}
	for _, key := range required {
		if _, ok := snap[key]; !ok {
			t.Errorf("Snapshot missing feature %q", key)
		}
	}
}

// TestFeatureManager_Global — 全局单例可重入获取。
func TestFeatureManager_Global(t *testing.T) {
	a := GetGlobalFeatureManager()
	b := GetGlobalFeatureManager()
	if a != b {
		t.Errorf("GetGlobalFeatureManager should return the same instance")
	}
}
