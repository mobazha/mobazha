package config

import (
	"log"
)

// InitFeatureManager 初始化全局 FeatureManager
func InitFeatureManager() {
	fm := GetGlobalFeatureManager()

	fm.RegisterToggle(FeatureBuildinWalletPaymentDisabled, true)

	allToggles := fm.GetAllToggles()
	log.Println("功能开关初始化完成，当前所有功能开关状态：")
	for feature, enabled := range allToggles {
		log.Printf("  - %s: %v", feature, enabled)
	}

	log.Println("功能开关管理器初始化完成")
}

// 在包初始化时自动初始化 FeatureManager
func init() {
	// 这里可以选择是否在包初始化时自动初始化
	// 如果希望手动控制初始化时机，可以注释掉下面的代码
	InitFeatureManager()
}
