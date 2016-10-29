
include domingo.mk

PROJECT_NAME := trireme-kubernetes

ci: domingo_contained_build

init: domingo_init
test: domingo_test
release: build domingo_docker_build domingo_docker_push

build:
	rm -rf ${GOPATH}/src/k8s.io/kubernetes/vendor/github.com/golang/glog
	CGO_ENABLED=1 go build -a -installsuffix cgo
	make package

package:
	mkdir -p docker/app
	cp -a trireme-kubernetes  docker/app

clean:
