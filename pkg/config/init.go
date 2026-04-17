// Package config — Feature flag bootstrap.
//
// FF-1 仅做：
//  1. 显式触发 features_defined.go 中的 var 求值（import by side-effect 已完成，此处主要是占位）。
//  2. 执行 ValidateRegistry()：依赖存在性 + 循环依赖检测；失败直接 log.Fatal。
//  3. 创建全局 FeatureManager 单例并打印当前 DefaultValue 快照，便于启动排障。
//
// 注意：为避免 init() 执行顺序不确定性（§13.6），校验不放在包初始化中，
// 而是由 main（节点 & hosting 启动脚本）显式调用 InitFeatureManager。
package config

import (
	"log"
)

// InitFeatureManager 校验 feature registry 并初始化全局 FeatureManager。
//
// 调用方：独立节点 main / hosting main 的启动序列中显式调用一次。
// 失败（依赖缺失 / 循环依赖）将通过 log.Fatalf 终止进程——这是配置错误，
// 不应允许节点以"部分失败"的方式启动。
func InitFeatureManager() {
	if err := ValidateRegistry(); err != nil {
		log.Fatalf("feature flag registry validation failed: %v", err)
	}

	fm := GetGlobalFeatureManager()

	snapshot := fm.Snapshot()
	log.Printf("feature flags initialized (%d registered):", len(snapshot))
	for key, enabled := range snapshot {
		log.Printf("  - %s: %v (default)", key, enabled)
	}
}
