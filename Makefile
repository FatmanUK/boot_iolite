GO ?= go
PODMAN ?= podman
APP = iolite
IMAGE = ghcr.io/fatmanuk/$(APP)
V_TAG = $(shell git tag --list 'v*' | sort -V | tail -n1)
VERSION = $(if $(V_TAG),$(V_TAG),v0.0.0)
STAMP = $(shell date -u +%Y%m%dT%H%M%SZ)
LDFLAGS = -s -w
PKGDIRS = $(shell $(GO) list -f '{{.Dir}}' ./...)
ARGS ?=

.PHONY: all build test lint clean run image podman-build podman-push

all: lint test build

build:
	CGO_ENABLED=0 $(GO) build -ldflags="$(LDFLAGS)" -o $(APP) ./main

test:
	$(GO) test -race -cover ./...

lint:
	@out="$$(gofmt -l $(PKGDIRS))"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi
	$(GO) vet ./...
	$(GO) run ./tools/lint ./...

clean:
	rm -f $(APP)

run: build
	sudo ./$(APP) $(ARGS)

image:
	$(MAKE) -C image

podman-build: build
	$(PODMAN) build --format=oci \
		-t $(IMAGE):latest \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):$(STAMP) \
		--build-arg VERSION=$(VERSION) \
		--build-arg STAMP=$(STAMP) \
		.

podman-push: podman-build
	$(PODMAN) push $(IMAGE):latest
	$(PODMAN) push $(IMAGE):$(VERSION)
	$(PODMAN) push $(IMAGE):$(STAMP)
