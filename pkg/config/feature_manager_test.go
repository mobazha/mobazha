package config

import (
	"testing"
)

func TestGlobalFeatureManager(t *testing.T) {
	// 初始化全局 FeatureManager
	InitFeatureManager()

	// 获取全局 FeatureManager 实例
	fm := GetGlobalFeatureManager()

	// 测试获取所有功能开关状态
	allToggles := fm.GetAllToggles()
	if len(allToggles) < 1 {
		t.Errorf("功能开关数量不正确，期望至少 1 个，实际 %d", len(allToggles))
	}
}
