# Variables
GO = go
PODMAN = podman
APP_BINARY = iolite
IMAGE_REPO = ghcr.io/fatmanuk
IMAGE_NAME = $(IMAGE_REPO)/iolite
V_TAG = $(shell git tag --list 'v*' | tail -n1)
VERSION = $(if $(V_TAG),$(V_TAG),v0.0.0)

BUILD_DATE = $(shell date +%Y%m%d)
BUILD_TIMESTAMP = $(shell date +%Y%m%dT%H%M00Z)
CKSUM_SCRIPT = import hashlib; print(hashlib.sha1(open('./$(APP_BINARY)','rb').read()).hexdigest())

BUILD_ENV=CGO_ENABLED=0
BUILD_LDFLAGS=-s -w -X main.APP_NAME=$(APP_BINARY) -X main.VERSION=$(VERSION)

# Default target
all: clean podman-build

.PHONY: clean test lint mod-download build

clean:
	rm ./$(APP_BINARY)

# Run the app locally using go run
run:
	$(GO) run ./main

# Build static, stripped binaries for minimal final images
mod-download:
	$(GO) mod download

build: mod-download
	$(BUILD_ENV) $(GO) build \
		-ldflags="$(BUILD_LDFLAGS)" \
		-o ./$(APP_BINARY) \
		./main

# Run unit tests
test:
	GOTOOLCHAIN=go1.25.0+auto $(GO) test -cover -race -v ./...

# Lint code
lint:
	$(GO) fmt ./...

# Build the container image using podman
podman-build: build
	$(PODMAN) build \
		--compress=true \
		--layers=true \
		--format=oci \
		-t $(IMAGE_NAME):latest \
		-t $(IMAGE_NAME):$(BUILD_DATE) \
		-t $(IMAGE_NAME):$(BUILD_TIMESTAMP) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):$(shell python3 -c "$(CKSUM_SCRIPT)") \
		--build-arg APP_BINARY=$(APP_BINARY) \
		--build-arg BUILD_TIMESTAMP=$(BUILD_TIMESTAMP) \
		--build-arg VERSION=$(VERSION) \
		.

# Run the container locally using podman
podman-run: podman-build
	$(PODMAN) run \
		--rm -it \
		$(IMAGE_NAME):latest

# Push to the image repo
podman-push: podman-build
	$(PODMAN) push \
		$(IMAGE_NAME):latest
	$(PODMAN) push \
		$(IMAGE_NAME):$(BUILD_DATE)
	$(PODMAN) push \
		$(IMAGE_NAME):$(BUILD_TIMESTAMP)
	$(PODMAN) push \
		$(IMAGE_NAME):$(VERSION)
	$(PODMAN) push \
		$(IMAGE_NAME):$(shell python3 -c "$(CKSUM_SCRIPT)")
