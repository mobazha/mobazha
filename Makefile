.PHONY: build build-private_distribution embed-private_distribution-frontend build-private_distribution-image smoke-private_distribution smoke-private_distribution-da test test-invariants test-e2e-evm test-libolm clean ios_framework android_framework protos sample-config docker push_docker openapi

SYSTEM_GO := /usr/local/go/bin/go
GO ?= $(if $(wildcard $(SYSTEM_GO)),$(SYSTEM_GO),go)
GO_TEST_TAGS ?= goolm

build: ## 构建项目
	bash ./scripts/with-libolm-env.sh $(GO) build -o mobazha

build-private_distribution: ## 构建 PrivateDistribution 精简版（CGO-free，隐私模式）
	CGO_ENABLED=0 $(GO) build -tags "private_distribution purego_sqlite embed_frontend" -ldflags "-s -w" -o mobazha-private_distribution .

embed-private_distribution-frontend: ## 构建 PrivateDistribution SPA + Brotli 嵌入（Tor / Managed pool）
	bash ./scripts/embed-private_distribution-frontend.sh

build-private_distribution-image: embed-private_distribution-frontend build-private_distribution ## PrivateDistribution 前端嵌入 + 二进制（本地 Docker 前置）

test: ## 运行测试
	$(GO) test -tags '$(GO_TEST_TAGS)' ./...

# Phase EVM-ManagedEscrow SP1-B — DelegateCall 架构守护测试。
# 覆盖 PENDING_RELAY_DESIGN §3.5 + §11.4 的三大不变量：
#   Invariant 1: MultiSendCallOnly 拒绝 inner DELEGATECALL（含 0..255 全字节穷举）
#   Invariant 2: outer DELEGATECALL 仅允许 canonical MultiSendCallOnly（含 trojan-target 目录）
#   Invariant 3: 所有 Ready 链的 MSCO 地址被 IsCanonicalMultiSendCallOnly 识别
# 以及 adapter 层 biconditional：CALL ⇔ recipient / DELEGATECALL ⇔ MSCO。
# 任何破坏这些性质的改动会在 CI 立即失败。
test-invariants: ## 运行 DelegateCall 架构守护测试（pkg/managedescrow + adapters）
	$(GO) test -tags '$(GO_TEST_TAGS)' -run 'Invariant|Inv2|DelegateCall|MultiSendCallOnly|AlwaysUsesMultiSend' \
		./pkg/managedescrow/... ./internal/payment/adapters/...

# Phase EVM-ManagedEscrow SP1-C — Anvil/Sepolia E2E smoke suite。
# 在 go-ethereum simulated.Backend 上注入 ManagedEscrow v1.4.1 deployed bytecode，
# 跑「真实 EVM 执行」回归。场景包括（v2 smoke 套件）：
#   场景 1: predicted-on-paper == deployed-on-chain proxy address
#   场景 2-4: 待补
# build tag e2e_evm 隔离，默认 test 套不会拉 simulated 后端的 ~50MB 依赖。
# bytecode fixture 见 pkg/managedescrow/testdata/bytecode/，刷新用 scripts/fetch-managed_escrow-bytecode.sh。
test-e2e-evm: ## 运行 SP1-C EVM E2E smoke 套件（需要 e2e_evm build tag）
	$(GO) test -tags '$(GO_TEST_TAGS) e2e_evm' -run 'E2E' \
		./pkg/managedescrow/... ./internal/payment/adapters/...

smoke-private_distribution: build-private_distribution ## 构建 private_distribution 并运行网络隔离 smoke test
	./scripts/private_distribution-network-smoke.sh ./mobazha-private_distribution 20

smoke-private_distribution-da: build-private_distribution ## 构建 private_distribution 并运行 digital-assets 写入端点 smoke test (TD-104 回归)
	./scripts/private_distribution-digital-assets-smoke.sh ./mobazha-private_distribution

test-libolm: ## 使用 libolm(cgo) 运行测试
	bash ./scripts/with-libolm-env.sh $(GO) test ./...

clean: ## 清理构建文件
	rm -f mobazha
	rm -rf dist/

##
## Mobile
##

ios_framework: ## Build iOS Framework for mobile
# https://github.com/libp2p/go-libp2p-connmgr/issues/98
# https://github.com/libp2p/go-libp2p/pull/1666
	gomobile bind -target=ios -iosversion=10 -ldflags="-s -w" -tags "nowatchdog notor" github.com/mobazha/mobazha3.0/mobile

android_framework: ## Build Android Framework for mobile
	gomobile bind -target=android/arm,android/arm64,android/amd64 -ldflags="-s -w" -tags notor github.com/mobazha/mobazha3.0/mobile

##
## Protobuf compilation
##

protos:
	# 生成 net/mbzpb 消息定义
	cd pkg/net/mbzpb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=./ *.proto
	# 生成 orders/mbzpb 订单定义
	cd pkg/orders/mbzpb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=./ --proto_path=../../net/mbzpb --proto_path=./ *.proto
	# 修复 OrderList 引用 (OrderList 定义在 net/mbzpb 中)
	cd pkg/orders/mbzpb && sed -i '' 's/\*OrderList/\*mbzpb.OrderList/g' orders.pb.go
	# 添加导入 (在 sync 导入后添加)
	cd pkg/orders/mbzpb && sed -i '' 's|sync "sync"|sync "sync"\n\n\t"github.com/mobazha/mobazha3.0/pkg/net/mbzpb"|' orders.pb.go
	# 移除无效的 file_msg_proto_init 调用
	cd pkg/orders/mbzpb && sed -i '' 's/file_msg_proto_init()//' orders.pb.go
	# 格式化代码
	cd pkg/orders/mbzpb && gofmt -s -w orders.pb.go
	# 生成 channels/pb
	cd internal/channels/pb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=./ *.proto

##
## Sample config file
##

sample-config:
	cd internal/repo && go-bindata -pkg=repo sample-mobazha.conf

##
## Docker
##
DOCKER_PROFILE ?= mobazha
DOCKER_VERSION ?= $(shell git describe --tags --abbrev=0)
DOCKER_IMAGE_NAME ?= $(DOCKER_PROFILE)/server:$(DOCKER_VERSION)

docker:
	docker build -t $(DOCKER_IMAGE_NAME) .

push_docker:
	docker push $(DOCKER_IMAGE_NAME)

help: ## 显示帮助信息
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Generate OpenAPI 3.1 spec from huma operations (AH-1.6).
openapi:
	$(GO) run ./cmd/gen-openapi/main.go

.DEFAULT_GOAL := help
