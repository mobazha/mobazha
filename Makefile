##
## Mobile
##

.PHONY: ios_framework
ios_framework: ## Build iOS Framework for mobile
	gomobile bind -target=ios/arm64,ios/amd64 -iosversion=10 -ldflags="-s -w" -tags notor github.com/OpenBazaar/openbazaar3.0/mobile

.PHONY: android_framework
android_framework: ## Build Android Framework for mobile
	gomobile bind -target=android/arm,android/arm64,android/amd64 -ldflags="-s -w" -tags notor github.com/cpacia/openbazaar3.0/mobile

##
## Protobuf compilation
##

.PHONY: protos
protos:
	cd net/pb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=./ *.proto
	cd orders/pb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=./ --proto_path=../../net/pb --proto_path=./ *.proto
	cd orders/pb && sed -i 's/OrderList/pb.OrderList/' orders.pb.go
	cd orders/pb && sed -i '11i\"github.com/cpacia/openbazaar3.0/net/pb"\' orders.pb.go
	cd orders/pb && sed -i 's/file_msg_proto_init()//' orders.pb.go
	cd orders/pb && gofmt -s -w orders.pb.go
	cd channels/pb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=./ *.proto

##
## Sample config file
##

sample-config:
	cd repo && go-bindata -pkg=repo sample-openbazaar.conf

##
## Docker
##
DOCKER_PROFILE ?= openbazaar
DOCKER_VERSION ?= $(shell git describe --tags --abbrev=0)
DOCKER_IMAGE_NAME ?= $(DOCKER_PROFILE)/server:$(DOCKER_VERSION)

.PHONY: docker
docker:
	docker build -t $(DOCKER_IMAGE_NAME) .

.PHONY: push_docker
push_docker:
	docker push $(DOCKER_IMAGE_NAME)
