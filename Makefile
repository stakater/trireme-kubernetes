
include domingo.mk

PROJECT_NAME := kubernetes-integration

ci: domingo_contained_build

init: install_pkg_config domingo_init
test: domingo_test
release: build domingo_docker_build domingo_docker_push

install_pkg_config:
	apt-get update
	apt-get install -y libnetfilter-queue-dev

build:
	rm -rf ${GOPATH}/src/k8s.io/kubernetes/vendor/github.com/golang/glog
	CGO_ENABLED=1 go build -a -installsuffix cgo
	make package

package:
	mkdir -p docker/app
	cp -a kubernetes-integration  docker/app

clean:
