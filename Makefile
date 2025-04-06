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
	cd internal/net/pb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=./ *.proto
	cd internal/orders/mbzpb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=./ --proto_path=../../net/mbzpb --proto_path=./ *.proto
	cd internal/orders/mbzpb && sed -i 's/OrderList/pb.OrderList/' orders.pb.go
	cd internal/orders/mbzpb && sed -i '11i\"github.com/mobazha/mobazha3.0/pkg/net/mbzpb"\' orders.pb.go
	cd internal/orders/mbzpb && sed -i 's/file_msg_proto_init()//' orders.pb.go
	cd internal/orders/mbzpb && gofmt -s -w orders.pb.go
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
