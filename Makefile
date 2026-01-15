.PHONY: build test clean ios_framework android_framework protos sample-config docker push_docker

build: ## 构建项目
	go build -o mobazha

test: ## 运行测试
	go test ./...

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

.DEFAULT_GOAL := help
