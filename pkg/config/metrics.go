// Package config — Feature flag Prometheus metrics
//
// 两个 counter：
//
//   - feature_flag_evaluations_total{key,result,denied_at}
//     Resolver 顶层入口（IsEnabled / Evaluate / List）调用一次对应记录一次。
//     result="true"|"false"；denied_at=""|"platform_global"|"tenant"|"node_runtime"。
//     内部依赖评估（evaluate 递归）不会重复计数，避免 fan-out 噪声。
//
//   - feature_flag_changes_total{key,scope,new_value}
//     管理端 handler 成功变更 override 后显式调用 RecordFeatureChange。
//     scope 取自 ScopePlatformGlobal / ScopeTenant（Node 层无变更通道）。
//
// 指标通过 prometheus/client_golang 的 promauto 注册到默认 registry，
// 消费仓库只要挂了 promhttp.Handler 即可被 /metrics 暴露。
package config

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	featureFlagEvaluations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feature_flag_evaluations_total",
			Help: "Total number of feature flag evaluations (top-level Resolver entry points only).",
		},
		[]string{"key", "result", "denied_at"},
	)

	featureFlagChanges = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feature_flag_changes_total",
			Help: "Total number of feature flag override changes performed via management APIs.",
		},
		[]string{"key", "scope", "new_value"},
	)
)

// RecordFeatureEvaluation 记录一次 Resolver 评估结果。
//
// 仅在 Resolver 的公开入口（IsEnabled / Evaluate / List）调用，避免
// 依赖递归造成双计数。
func RecordFeatureEvaluation(eval Evaluation) {
	deniedAt := ""
	if eval.DeniedAtLayer != "" {
		deniedAt = string(eval.DeniedAtLayer)
	}
	featureFlagEvaluations.WithLabelValues(
		eval.Key,
		strconv.FormatBool(eval.Enabled),
		deniedAt,
	).Inc()
}

// RecordFeatureChange 记录一次 feature 管理端变更。
//
// 调用方：
//   - mobazha3.0  tenant 层 PUT handler（scope=ScopeTenant）
//   - hosting    platform 层 PATCH handler（scope=ScopePlatformGlobal）
//
// 变更失败时**不要**调用此函数（否则指标会虚高）。
func RecordFeatureChange(scope Scope, key string, newValue bool) {
	featureFlagChanges.WithLabelValues(
		key,
		string(scope),
		strconv.FormatBool(newValue),
	).Inc()
}
