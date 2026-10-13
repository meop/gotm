MODULE = github.com/meop/gotm
BINARY = gotm
VERSION ?= $(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build build-all test lint install clean-build

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/gotm

build-all:
	$(foreach PLATFORM,$(PLATFORMS),\
		$(eval OS=$(word 1,$(subst /, ,$(PLATFORM)))) \
		$(eval ARCH=$(word 2,$(subst /, ,$(PLATFORM)))) \
		GOOS=$(OS) GOARCH=$(ARCH) go build $(LDFLAGS) \
			-o dist/$(BINARY)-$(VERSION)-$(OS)-$(ARCH)$(if $(filter windows,$(OS)),.exe,) \
			./cmd/gotm ;)

test:
	go test ./...

lint:
	golangci-lint run ./...

install:
	go install $(LDFLAGS) ./cmd/gotm

clean-build:
	rm -rf dist/ $(BINARY)
