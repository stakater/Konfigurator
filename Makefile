.PHONY: install test builder-image push lint fetch-dependencies

DOCKER_IMAGE ?= stakater/konfigurator

# Default value "dev"
DOCKER_TAG ?= dev

install:  fetch-dependencies

fetch-dependencies:
	dep ensure -v

test:
	go test -v ./...

binary-image:
	operator-sdk build ${DOCKER_IMAGE}:${DOCKER_TAG}

lint:
	golangci-lint run --enable-all --skip-dirs vendor

push:
	docker push ${DOCKER_IMAGE}:${DOCKER_TAG}
