PROJECT_NAME := trireme-kubernetes
VERSION_FILE := ./version/version.go
VERSION := 0.11
REVISION=$(shell git log -1 --pretty=format:"%H")
BUILD_NUMBER := latest
DOCKER_REGISTRY?=aporeto
DOCKER_IMAGE_NAME?=$(PROJECT_NAME)
DOCKER_IMAGE_TAG?=$(BUILD_NUMBER)

codegen:
	echo 'package version' > $(VERSION_FILE)
	echo '' >> $(VERSION_FILE)
	echo '// VERSION is the version of Trireme-Kubernetes' >> $(VERSION_FILE)
	echo 'const VERSION = "$(VERSION)"' >> $(VERSION_FILE)
	echo '' >> $(VERSION_FILE)
	echo '// REVISION is the revision of Trireme-Kubernetes' >> $(VERSION_FILE)
	echo 'const REVISION = "$(REVISION)"' >> $(VERSION_FILE)

build: codegen
	CGO_ENABLED=1 go build -a -installsuffix cgo

package:
	mv trireme-kubernetes docker/trireme-kubernetes

docker_build: build package
	docker \
		build \
		-t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) docker

docker_push: docker_build
	docker \
		push \
		$(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)
