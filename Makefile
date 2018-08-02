.PHONY: install test builder-image push

DOCKER_IMAGE ?= stakater/konfigurator

# Default value "dev"
DOCKER_TAG ?= dev

install: lint
	dep ensure -v

test:
	go test -v ./...

binary-image:
	operator-sdk build ${DOCKER_IMAGE}:${DOCKER_TAG}

lint:
	golangci-lint run --enable-all

push:
	docker push ${DOCKER_IMAGE}:${DOCKER_TAG}
