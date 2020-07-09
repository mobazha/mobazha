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
P_TIMESTAMP = Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp
P_ANY = Mgoogle/protobuf/any.proto=github.com/golang/protobuf/ptypes/any

PKGMAP = $(P_TIMESTAMP),$(P_ANY)

.PHONY: protos
protos:
	cd net/pb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=$(PKGMAP):./ *.proto
	cd orders/pb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=$(PKGMAP):./ *.proto
	cd channels/pb && PATH=$(PATH):$(GOPATH)/bin protoc --go_out=$(PKGMAP):./ *.proto

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
