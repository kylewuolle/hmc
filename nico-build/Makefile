# Builds and pushes the NICo (NVIDIA Infra Controller) container image.
#
# Each architecture uses separate upstream Dockerfiles, so multi-arch is handled
# by building arch-tagged images independently and combining them into a manifest:
#   registry/nico/nico:v0.7.0-amd64 )
#   registry/nico/nico:v0.7.0-arm64  } --> registry/nico/nico:v0.7.0  (manifest)
#
# Single-arch build and push (typical CI job, runs once per arch):
#   make build-push ARCH=amd64 IMAGE_TAG=v0.7.0
#   make build-push ARCH=arm64 IMAGE_TAG=v0.7.0
#
# Create the multi-arch manifest (runs once after both arch jobs complete):
#   make manifest IMAGE_TAG=v0.7.0
#
# Use an existing source checkout instead of cloning:
#   make build-push ARCH=amd64 REPO_DIR=/path/to/infra-controller-core

SHELL        := /bin/bash
.DEFAULT_GOAL := help

# BuildKit is required — the release Dockerfiles use --mount=type=cache.
export DOCKER_BUILDKIT := 1

## ---------------------------------------------------------------------------
## Registry / image
## ---------------------------------------------------------------------------

REGISTRY   ?= 127.0.0.1:5001
IMAGE_REPO ?= nico/nico

## ---------------------------------------------------------------------------
## Architecture  (amd64 | arm64)
## ---------------------------------------------------------------------------

ARCH ?= amd64

ifeq ($(ARCH),amd64)
  _BUILD_CONTAINER_DEFAULT   := nico-buildcontainer-x86_64
  _RUNTIME_CONTAINER_DEFAULT := nico-runtime-container-x86_64
  _BUILD_DOCKERFILE   := dev/docker/Dockerfile.build-container-x86_64
  _RUNTIME_DOCKERFILE := dev/docker/Dockerfile.runtime-container-x86_64
  _RELEASE_DOCKERFILE := dev/docker/Dockerfile.release-container-sa-x86_64
  _BUILD_ARG_BUILD    := CONTAINER_BUILD_X86_64
  _BUILD_ARG_RUNTIME  := CONTAINER_RUNTIME_X86_64
else ifeq ($(ARCH),arm64)
  _BUILD_CONTAINER_DEFAULT   := nico-buildcontainer-aarch64
  _RUNTIME_CONTAINER_DEFAULT := nico-runtime-container-aarch64
  _BUILD_DOCKERFILE   := dev/docker/Dockerfile.build-container-aarch64
  _RUNTIME_DOCKERFILE := dev/docker/Dockerfile.runtime-container-aarch64
  _RELEASE_DOCKERFILE := dev/docker/Dockerfile.release-container-aarch64
  _BUILD_ARG_BUILD    := CONTAINER_BUILD_AARCH64
  _BUILD_ARG_RUNTIME  := CONTAINER_RUNTIME_AARCH64
else
  $(error Unsupported ARCH=$(ARCH). Use ARCH=amd64 or ARCH=arm64)
endif

## Override the intermediate container names if needed.
BUILD_CONTAINER   ?= $(_BUILD_CONTAINER_DEFAULT)
RUNTIME_CONTAINER ?= $(_RUNTIME_CONTAINER_DEFAULT)

## ---------------------------------------------------------------------------
## Source repository
## ---------------------------------------------------------------------------

REPO_URL ?= https://github.com/NVIDIA/infra-controller-core.git
# Branch, tag, or commit to check out. In CI, pin this to the release tag.
REPO_REF ?= main
# Local path for the clone. Override to use an existing checkout and skip clone.
REPO_DIR ?= $(CURDIR)/infra-controller-core

## ---------------------------------------------------------------------------
## Image tags
## ---------------------------------------------------------------------------

# Auto-derived from `git describe` in the source repo when available.
# In CI, always pass IMAGE_TAG explicitly.
_auto_tag := $(shell git -C "$(REPO_DIR)" describe --tags --first-parent --always --long 2>/dev/null | sed 's/^v//')
IMAGE_TAG ?= $(_auto_tag)

# Arch-tagged image pushed by each individual build job.
ARCH_IMAGE     := $(REGISTRY)/$(IMAGE_REPO):$(IMAGE_TAG)-$(ARCH)
# Final multi-arch manifest image (no arch suffix).
MANIFEST_IMAGE := $(REGISTRY)/$(IMAGE_REPO):$(IMAGE_TAG)

## ---------------------------------------------------------------------------
## Extra docker build flags
## Useful for BuildKit layer cache in CI:
##   type=gha                           (GitHub Actions cache)
##   type=registry,ref=registry/cache   (registry-based cache)
##
## Example:
##   DOCKER_BUILD_EXTRA=--cache-from type=gha --cache-to type=gha,mode=max
## ---------------------------------------------------------------------------

DOCKER_BUILD_EXTRA ?=

## ---------------------------------------------------------------------------
## Internal helpers
## ---------------------------------------------------------------------------

.PHONY: _require_repo_dir
_require_repo_dir:
	@if [ ! -d "$(REPO_DIR)" ]; then \
	  echo "ERROR: REPO_DIR=$(REPO_DIR) not found. Run 'make clone' first."; \
	  exit 1; \
	fi

.PHONY: _require_tag
_require_tag:
	@if [ -z "$(IMAGE_TAG)" ]; then \
	  echo "ERROR: IMAGE_TAG is empty. Pass it explicitly: make ... IMAGE_TAG=v0.7.0"; \
	  exit 1; \
	fi

## ---------------------------------------------------------------------------
## Targets
## ---------------------------------------------------------------------------

.PHONY: help
help: ## Show available targets and current variable values
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | sort | \
	  awk 'BEGIN {FS = ":.*## "}; {printf "  %-24s %s\n", $$1, $$2}'
	@echo ""
	@echo "Variables (override on command line or via environment):"
	@printf "  %-26s %s\n" "ARCH"               "$(ARCH)   (amd64 | arm64)"
	@printf "  %-26s %s\n" "REGISTRY"            "$(REGISTRY)"
	@printf "  %-26s %s\n" "IMAGE_REPO"           "$(IMAGE_REPO)"
	@printf "  %-26s %s\n" "IMAGE_TAG"            "$(IMAGE_TAG)"
	@printf "  %-26s %s\n" "REPO_URL"             "$(REPO_URL)"
	@printf "  %-26s %s\n" "REPO_REF"             "$(REPO_REF)"
	@printf "  %-26s %s\n" "REPO_DIR"             "$(REPO_DIR)"
	@printf "  %-26s %s\n" "DOCKER_BUILD_EXTRA"   "$(DOCKER_BUILD_EXTRA)"
	@echo ""
	@echo "Derived:"
	@printf "  %-26s %s\n" "ARCH_IMAGE"           "$(ARCH_IMAGE)"
	@printf "  %-26s %s\n" "MANIFEST_IMAGE"       "$(MANIFEST_IMAGE)"

.PHONY: clone
clone: ## Clone or update REPO_DIR from REPO_URL at REPO_REF
	@if [ -d "$(REPO_DIR)/.git" ]; then \
	  echo "Updating $(REPO_DIR) to $(REPO_REF)..."; \
	  git -C "$(REPO_DIR)" fetch --depth=1 origin "$(REPO_REF)"; \
	  git -C "$(REPO_DIR)" checkout FETCH_HEAD; \
	else \
	  echo "Cloning $(REPO_URL) @ $(REPO_REF) into $(REPO_DIR)..."; \
	  git clone --depth=1 --branch "$(REPO_REF)" "$(REPO_URL)" "$(REPO_DIR)"; \
	fi

.PHONY: build-builder
build-builder: _require_repo_dir ## Build the Rust build container for ARCH (not pushed)
	@echo ">>> [$(ARCH)] Building $(BUILD_CONTAINER)..."
	docker build \
	  $(DOCKER_BUILD_EXTRA) \
	  --file "$(REPO_DIR)/$(_BUILD_DOCKERFILE)" \
	  --tag  "$(BUILD_CONTAINER)" \
	  "$(REPO_DIR)"

.PHONY: build-runtime
build-runtime: _require_repo_dir ## Build the runtime base container for ARCH (not pushed)
	@echo ">>> [$(ARCH)] Building $(RUNTIME_CONTAINER)..."
	docker build \
	  $(DOCKER_BUILD_EXTRA) \
	  --file "$(REPO_DIR)/$(_RUNTIME_DOCKERFILE)" \
	  --tag  "$(RUNTIME_CONTAINER)" \
	  "$(REPO_DIR)"

.PHONY: build-nico
build-nico: _require_tag build-builder build-runtime ## Build and tag ARCH_IMAGE (includes arch suffix)
	@echo ">>> [$(ARCH)] Building $(ARCH_IMAGE)..."
	docker build \
	  $(DOCKER_BUILD_EXTRA) \
	  --build-arg "$(_BUILD_ARG_BUILD)=$(BUILD_CONTAINER)" \
	  --build-arg "$(_BUILD_ARG_RUNTIME)=$(RUNTIME_CONTAINER)" \
	  --build-arg "VERSION=$(IMAGE_TAG)" \
	  --build-arg "CI_COMMIT_SHORT_SHA=$(shell git -C "$(REPO_DIR)" rev-parse --short HEAD 2>/dev/null)" \
	  --file "$(REPO_DIR)/$(_RELEASE_DOCKERFILE)" \
	  --tag  "$(ARCH_IMAGE)" \
	  "$(REPO_DIR)"

.PHONY: push
push: _require_tag ## Push ARCH_IMAGE to the registry
	@echo ">>> [$(ARCH)] Pushing $(ARCH_IMAGE)..."
	docker push "$(ARCH_IMAGE)"

.PHONY: build-push
build-push: clone build-nico push ## Full per-arch workflow: clone, build, push ARCH_IMAGE

.PHONY: manifest
manifest: _require_tag ## Create and push multi-arch manifest from both arch images
	@echo ">>> Creating manifest $(MANIFEST_IMAGE)..."
	docker buildx imagetools create \
	  --tag "$(MANIFEST_IMAGE)" \
	  "$(REGISTRY)/$(IMAGE_REPO):$(IMAGE_TAG)-amd64" \
	  "$(REGISTRY)/$(IMAGE_REPO):$(IMAGE_TAG)-arm64"

## Kind cluster to load images into (matches KCM dev default).
KIND_CLUSTER ?= kcm-dev

.PHONY: kind-load
kind-load: _require_tag ## Load ARCH_IMAGE into the kind cluster (skips registry pull)
	@echo ">>> Loading $(ARCH_IMAGE) into kind cluster $(KIND_CLUSTER)..."
	kind load docker-image "$(ARCH_IMAGE)" --name "$(KIND_CLUSTER)"

.PHONY: build-kind
build-kind: clone build-nico kind-load ## Build and load into kind (dev workflow, no push needed)

.PHONY: print-arch-image
print-arch-image: ## Print the arch-tagged image reference (for use in CI pipelines)
	@echo "$(ARCH_IMAGE)"

.PHONY: print-manifest-image
print-manifest-image: ## Print the multi-arch manifest image reference
	@echo "$(MANIFEST_IMAGE)"

.PHONY: clean-images
clean-images: ## Remove local NICo and intermediate docker images for ARCH
	-docker rmi "$(ARCH_IMAGE)"         2>/dev/null
	-docker rmi "$(BUILD_CONTAINER)"    2>/dev/null
	-docker rmi "$(RUNTIME_CONTAINER)"  2>/dev/null

.PHONY: clean-repo
clean-repo: ## Remove the cloned REPO_DIR
	@if [ "$(REPO_DIR)" = "$(CURDIR)" ] || [ "$(REPO_DIR)" = "/" ]; then \
	  echo "ERROR: refusing to delete REPO_DIR=$(REPO_DIR)"; exit 1; \
	fi
	rm -rf "$(REPO_DIR)"

.PHONY: clean
clean: clean-images clean-repo ## Remove images and cloned source repo
