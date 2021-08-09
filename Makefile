# SPDX-License-Identifier: MIT

SHELL := /bin/bash

# The name of the executable (default is current directory name)
TARGET := $(shell echo $${PWD\#\#*/})

GOOS ?= $(shell go env GOOS)
VERSION ?= $(shell git describe --tags --exact-match || \
	     git describe --exact-match 2> /dev/null || \
             git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

SRCS = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

REGISTRY ?= docker.pkg.github.com/daimler/kosmoo

all: test build docker

build:
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-o kosmoo \
		./main.go

docker: build
	cp kosmoo kubernetes/
	docker build -t $(REGISTRY)/kosmoo:$(VERSION) kubernetes/
	rm kubernetes/kosmoo

push:
	docker push $(REGISTRY)/kosmoo:$(VERSION)

fmt:
	@gofmt -l -w $(SRCS)

test: vet fmtcheck spdxcheck lint

vet:
	go vet ./...

lint:
	@hack/check_golangci-lint.sh

fmtcheck:
	@gofmt -l -s $(SRCS) | read; if [ $$? == 0 ]; then echo "gofmt check failed for:"; gofmt -l -s $(SRCS); exit 1; fi

spdxcheck:
	@hack/check_spdx.sh

version:
	@echo $(VERSION)
