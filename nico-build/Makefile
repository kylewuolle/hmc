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
## Boot artifact image settings
##
## Boot artifacts are the PXE/iPXE blobs built by cargo make inside a
## privileged container (carbide-pxe-builder).  The release image is just
## an Alpine container with the blobs copied in.
##
## Build flow (amd64 only):
##   make build-pxe-builder              # once — build the builder image
##   make build-push-boot IMAGE_TAG=...  # build artifacts + package + push
##
## Memory: the Rust+mkosi+iPXE build peaks at 16-32GB with 8 jobs.
## BOOT_BUILD_MEMORY caps the container so an OOM kills it cleanly rather
## than crashing the Docker daemon.  BOOT_CARGO_JOBS limits Rust parallelism.
## Keep BOOT_BUILD_MEMORY well below total host RAM (e.g. ≤60%) so the daemon
## itself is never starved when the container OOM fires.  On a 14 GiB WSL2 VM
## 12g left only ~1.6 GiB headroom and killed dockerd; 8g is the safe default.
## --memory-swap is set equal to --memory to disable swap growth, which
## prevents a slower OOM path from evading the limit.
## ---------------------------------------------------------------------------

BOOT_IMAGE_REPO     ?= nico/boot-artifacts
BOOT_PXE_BUILDER    ?= carbide-pxe-builder
BOOT_RUNTIME        ?= alpine:latest
BOOT_BUILD_MEMORY   ?= 8g
BOOT_CARGO_JOBS     ?= 4

BOOT_ARCH_IMAGE     := $(REGISTRY)/$(BOOT_IMAGE_REPO):$(IMAGE_TAG)-$(ARCH)
BOOT_MANIFEST_IMAGE := $(REGISTRY)/$(BOOT_IMAGE_REPO):$(IMAGE_TAG)

## ---------------------------------------------------------------------------
## Cargo binary build settings
##
## Rust binaries are built inside carbide-build-container rather than on the
## host so the build environment is reproducible and host toolchain differences
## can't affect the output.
##
## Build flow:
##   make build-cargo-container              # once — build the builder image
##   make build-binaries                     # compile all NICo Rust packages
##
## Memory: the full workspace link step peaks at ~6 GB with lld.  8g leaves
## headroom for the daemon on a 14 GiB host.  sccache is mounted from the
## host so compiled artifacts are reused across runs.
## ---------------------------------------------------------------------------

CARGO_BUILD_IMAGE   ?= carbide-build-container
CARGO_BUILD_MEMORY  ?= 8g
CARGO_BUILD_JOBS    ?= 4
CARGO_BUILD_PKGS    ?= carbide-api carbide-pxe carbide-dns carbide-admin-cli \
                       carbide-dsx-exchange-consumer carbide-agent carbide-dhcp-server \
                       carbide-health carbide-log-parser carbide-ssh-console carbide-bmc-proxy

CARGO_BUILD_ARGS    := $(addprefix -p ,$(CARGO_BUILD_PKGS))

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
	@printf "  %-26s %s\n" "BOOT_IMAGE_REPO"      "$(BOOT_IMAGE_REPO)"
	@printf "  %-26s %s\n" "BOOT_PXE_BUILDER"     "$(BOOT_PXE_BUILDER)"
	@printf "  %-26s %s\n" "BOOT_RUNTIME"         "$(BOOT_RUNTIME)"
	@printf "  %-26s %s\n" "BOOT_BUILD_MEMORY"    "$(BOOT_BUILD_MEMORY)"
	@printf "  %-26s %s\n" "BOOT_CARGO_JOBS"      "$(BOOT_CARGO_JOBS)"
	@printf "  %-26s %s\n" "CARGO_BUILD_IMAGE"    "$(CARGO_BUILD_IMAGE)"
	@printf "  %-26s %s\n" "CARGO_BUILD_MEMORY"   "$(CARGO_BUILD_MEMORY)"
	@printf "  %-26s %s\n" "CARGO_BUILD_JOBS"     "$(CARGO_BUILD_JOBS)"
	@echo ""
	@echo "Derived:"
	@printf "  %-26s %s\n" "ARCH_IMAGE"           "$(ARCH_IMAGE)"
	@printf "  %-26s %s\n" "MANIFEST_IMAGE"       "$(MANIFEST_IMAGE)"
	@printf "  %-26s %s\n" "BOOT_ARCH_IMAGE"      "$(BOOT_ARCH_IMAGE)"
	@printf "  %-26s %s\n" "BOOT_MANIFEST_IMAGE"  "$(BOOT_MANIFEST_IMAGE)"

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

## ---------------------------------------------------------------------------
## Cargo binary build targets
## ---------------------------------------------------------------------------

.PHONY: build-cargo-container
build-cargo-container: _require_repo_dir ## Build carbide-build-container image for Dockerized cargo builds (needed once)
	@echo ">>> Building $(CARGO_BUILD_IMAGE) from Dockerfile.build-artifacts-container-x86_64..."
	docker build \
	  --file "$(REPO_DIR)/dev/docker/Dockerfile.build-artifacts-container-x86_64" \
	  --tag  "$(CARGO_BUILD_IMAGE)" \
	  "$(REPO_DIR)/dev/docker"

.PHONY: _require_cargo_container
_require_cargo_container:
	@if ! docker image inspect "$(CARGO_BUILD_IMAGE)" >/dev/null 2>&1; then \
	  echo "ERROR: $(CARGO_BUILD_IMAGE) image not found. Run 'make build-cargo-container' first."; \
	  exit 1; \
	fi

.PHONY: build-binaries
build-binaries: _require_repo_dir _require_cargo_container ## Build all NICo Rust binaries inside Docker
	@echo ">>> Building NICo binaries inside $(CARGO_BUILD_IMAGE) (--memory $(CARGO_BUILD_MEMORY))..."
	@SCCACHE_HOST="$${SCCACHE_DIR:-$$HOME/.sccache}"; \
	 mkdir -p "$$SCCACHE_HOST"; \
	 HOST_UID=$$(id -u); \
	 HOST_GID=$$(id -g); \
	 docker run --rm \
	   --memory "$(CARGO_BUILD_MEMORY)" \
	   --memory-swap "$(CARGO_BUILD_MEMORY)" \
	   -v "$(REPO_DIR)":/code \
	   -v "$$SCCACHE_HOST":/sccache \
	   -w /code \
	   -e SCCACHE_DIR=/sccache \
	   -e RUSTC_WRAPPER=sccache \
	   -e RUSTFLAGS="-C link-arg=-fuse-ld=lld --cfg tokio_unstable" \
	   -e CARGO_BUILD_JOBS="$(CARGO_BUILD_JOBS)" \
	   "$(CARGO_BUILD_IMAGE)" \
	   sh -c 'r=0; \
	          cargo build $(CARGO_BUILD_ARGS) || r=$$?; \
	          chown -R $${HOST_UID}:$${HOST_GID} /code/target 2>/dev/null || true; \
	          exit $$r'

## ---------------------------------------------------------------------------
## Boot artifact targets
## ---------------------------------------------------------------------------

.PHONY: build-pxe-builder
build-pxe-builder: _require_repo_dir ## Build carbide-pxe-builder image (needed once before build-boot-artifacts)
	@echo ">>> Building $(BOOT_PXE_BUILDER) from Dockerfile.pxe-build-container..."
	docker build \
	  --file "$(REPO_DIR)/dev/docker/Dockerfile.pxe-build-container" \
	  --tag  "$(BOOT_PXE_BUILDER)" \
	  "$(REPO_DIR)/dev/docker"

.PHONY: _require_pxe_builder
_require_pxe_builder:
	@if ! docker image inspect "$(BOOT_PXE_BUILDER)" >/dev/null 2>&1; then \
	  echo "ERROR: $(BOOT_PXE_BUILDER) image not found. Run 'make build-pxe-builder' first."; \
	  exit 1; \
	fi

.PHONY: init-submodules
init-submodules: _require_repo_dir ## Initialize iPXE and mkosi git submodules in REPO_DIR (required before build-boot-artifacts)
	git -C "$(REPO_DIR)" submodule update --init --recursive pxe/ipxe/upstream pxe/mkosi

.PHONY: build-boot-artifacts
build-boot-artifacts: _require_repo_dir _require_pxe_builder ## Build PXE boot artifacts inside privileged carbide-pxe-builder container (amd64 only)
	@if [ "$(ARCH)" != "amd64" ]; then \
	  echo "ERROR: build-boot-artifacts only supports ARCH=amd64 (aarch64 uses a different upstream target)"; \
	  exit 1; \
	fi
	@echo ">>> Building PXE boot artifacts inside $(BOOT_PXE_BUILDER) (--privileged required for mkosi)..."
	@CARGO_HOME_DIR="$${CARGO_HOME:-$$HOME/.cargo}"; \
	 SCCACHE_HOST="$${SCCACHE_DIR:-$$HOME/.sccache}"; \
	 mkdir -p "$$SCCACHE_HOST"; \
	 HOST_UID=$$(id -u); \
	 HOST_GID=$$(id -g); \
	 PKG_VERSION=$$(git -C "$(REPO_DIR)" describe --tags --first-parent --always --long 2>/dev/null | sed 's/^v//'); \
	 case "$$PKG_VERSION" in [0-9]*) ;; *) PKG_VERSION="0.0.0-$$PKG_VERSION" ;; esac; \
	 docker run --rm \
	   --privileged \
	   --memory "$(BOOT_BUILD_MEMORY)" \
	   --memory-swap "$(BOOT_BUILD_MEMORY)" \
	   -v "$(REPO_DIR)":/code \
	   -v "$$CARGO_HOME_DIR":/cargo \
	   -v "$$SCCACHE_HOST":/sccache \
	   -w /code \
	   -e CARGO_HOME=/cargo \
	   -e SCCACHE_DIR=/sccache \
	   -e CARGO_BUILD_JOBS="$(BOOT_CARGO_JOBS)" \
	   -e SA_ENABLEMENT=1 \
	   -e PKG_VERSION="$$PKG_VERSION" \
	   -e HOST_UID="$$HOST_UID" \
	   -e HOST_GID="$$HOST_GID" \
	   "$(BOOT_PXE_BUILDER)" \
	   sh -c 'git config --global --add safe.directory /code && \
	          git config --global --add safe.directory /code/pxe/ipxe/upstream && \
	          r=0; \
	          cargo make --cwd pxe --env PKG_VERSION=$${PKG_VERSION} --env SA_ENABLEMENT=1 build-boot-artifacts-x86-host-sa || r=$$?; \
	          chown -R $${HOST_UID}:$${HOST_GID} /code 2>/dev/null || true; \
	          exit $$r'

.PHONY: build-boot-release
build-boot-release: _require_repo_dir _require_tag ## Build the boot artifacts release container (run after build-boot-artifacts)
	@echo ">>> [$(ARCH)] Building boot artifacts release image $(BOOT_ARCH_IMAGE)..."
	docker build \
	  $(DOCKER_BUILD_EXTRA) \
	  --build-arg "CONTAINER_RUNTIME_X86_64=$(BOOT_RUNTIME)" \
	  --file "$(REPO_DIR)/dev/docker/Dockerfile.release-artifacts-x86_64" \
	  --tag  "$(BOOT_ARCH_IMAGE)" \
	  "$(REPO_DIR)"

.PHONY: push-boot
push-boot: _require_tag ## Push the boot artifacts release image to the registry
	@echo ">>> [$(ARCH)] Pushing $(BOOT_ARCH_IMAGE)..."
	docker push "$(BOOT_ARCH_IMAGE)"

.PHONY: build-push-boot
build-push-boot: build-boot-artifacts build-boot-release push-boot ## Full boot artifacts workflow: build artifacts, package, push

.PHONY: boot-manifest
boot-manifest: _require_tag ## Create and push multi-arch manifest for boot artifact images
	@echo ">>> Creating boot artifacts manifest $(BOOT_MANIFEST_IMAGE)..."
	docker buildx imagetools create \
	  --tag "$(BOOT_MANIFEST_IMAGE)" \
	  "$(REGISTRY)/$(BOOT_IMAGE_REPO):$(IMAGE_TAG)-amd64" \
	  "$(REGISTRY)/$(BOOT_IMAGE_REPO):$(IMAGE_TAG)-arm64"

.PHONY: kind-load-boot
kind-load-boot: _require_tag ## Load boot artifacts image into the kind cluster
	@echo ">>> Loading $(BOOT_ARCH_IMAGE) into kind cluster $(KIND_CLUSTER)..."
	kind load docker-image "$(BOOT_ARCH_IMAGE)" --name "$(KIND_CLUSTER)"

.PHONY: build-kind-boot
build-kind-boot: build-boot-artifacts build-boot-release kind-load-boot ## Build boot artifacts and load into kind (dev workflow, no push needed)

.PHONY: clean-boot-images
clean-boot-images: ## Remove local boot artifact images
	-docker rmi "$(BOOT_ARCH_IMAGE)"       2>/dev/null
	-docker rmi "$(BOOT_PXE_BUILDER)"      2>/dev/null

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
