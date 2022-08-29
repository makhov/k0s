include embedded-bins/Makefile.variables
include inttest/Makefile.variables
include hack/tools/Makefile.variables

ifndef HOST_ARCH
HOST_HARDWARE := $(shell uname -m)
ifneq (, $(filter $(HOST_HARDWARE), aarch64 arm64 armv8l))
  HOST_ARCH := arm64
else ifneq (, $(filter $(HOST_HARDWARE), armv7l arm))
  HOST_ARCH := arm
else
  ifeq (, $(filter $(HOST_HARDWARE), x86_64 amd64 x64))
    $(warning unknown machine hardware name $(HOST_HARDWARE), assuming amd64)
  endif
  HOST_ARCH := amd64
endif
endif

K0S_GO_BUILD_CACHE ?= build/cache

GO_SRCS := $(shell find . -type f -name '*.go' -not -path './$(K0S_GO_BUILD_CACHE)/*' -not -path './inttest/*' -not -name '*_test.go' -not -name 'zz_generated*')
GO_DIRS := . ./cmd/... ./pkg/... ./internal/... ./static/... ./hack/...

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker
# k0s runs on linux even if it's built on mac or windows
TARGET_OS ?= linux
BUILD_UID ?= $(shell id -u)
BUILD_GID ?= $(shell id -g)
BUILD_GO_FLAGS := -tags osusergo
BUILD_GO_LDFLAGS_EXTRA :=
DEBUG ?= false

VERSION ?= $(shell git describe --tags)
ifeq ($(DEBUG), false)
LD_FLAGS ?= -w -s
endif

# https://reproducible-builds.org/docs/source-date-epoch/#makefile
# https://reproducible-builds.org/docs/source-date-epoch/#git
# https://stackoverflow.com/a/15103333
BUILD_DATE_FMT = %Y-%m-%dT%H:%M:%SZ
ifdef SOURCE_DATE_EPOCH
	BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "+$(BUILD_DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "+$(BUILD_DATE_FMT)" 2>/dev/null || date -u "+$(BUILD_DATE_FMT)")
else
	BUILD_DATE ?= $(shell TZ=UTC git log -1 --pretty=%cd --date='format-local:$(BUILD_DATE_FMT)' || date -u +$(BUILD_DATE_FMT))
endif

LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.Version=$(VERSION)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.RuncVersion=$(runc_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.ContainerdVersion=$(containerd_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KubernetesVersion=$(kubernetes_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KineVersion=$(kine_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.EtcdVersion=$(etcd_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KonnectivityVersion=$(konnectivity_version)
LD_FLAGS += -X "github.com/k0sproject/k0s/pkg/build.EulaNotice=$(EULA_NOTICE)"
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/telemetry.segmentToken=$(SEGMENT_TOKEN)
LD_FLAGS += -X k8s.io/component-base/version.gitVersion=v$(kubernetes_version)
LD_FLAGS += -X k8s.io/component-base/version.gitMajor=$(shell echo '$(kubernetes_version)' | cut -d. -f1)
LD_FLAGS += -X k8s.io/component-base/version.gitMinor=$(shell echo '$(kubernetes_version)' | cut -d. -f2)
LD_FLAGS += -X k8s.io/component-base/version.buildDate=$(BUILD_DATE)
LD_FLAGS += -X k8s.io/component-base/version.gitCommit=not_available
LD_FLAGS += -X github.com/containerd/containerd/version.Version=$(containerd_version)
ifeq ($(EMBEDDED_BINS_BUILDMODE), docker)
LD_FLAGS += -X github.com/containerd/containerd/version.Revision=$(shell ./embedded-bins/staging/linux/bin/containerd --version | awk '{print $$4}')
endif
LD_FLAGS += $(BUILD_GO_LDFLAGS_EXTRA)

GOLANG_IMAGE ?= golang:$(go_version)-alpine3.16
K0S_GO_BUILD_CACHE_VOLUME_PATH=$(shell realpath $(K0S_GO_BUILD_CACHE))
GO_ENV ?= docker run --rm \
	-v '$(K0S_GO_BUILD_CACHE_VOLUME_PATH)':/run/k0s-build \
	-v '$(CURDIR)':/go/src/github.com/k0sproject/k0s \
	-w /go/src/github.com/k0sproject/k0s \
	-e GOOS \
	-e CGO_ENABLED \
	-e GOARCH \
	--user $(BUILD_UID):$(BUILD_GID) \
	k0sbuild.docker-image.k0s
GO ?= $(GO_ENV) go

.PHONY: build
ifeq ($(TARGET_OS),windows)
build: k0s.exe
else
BUILD_GO_LDFLAGS_EXTRA = -extldflags=-static
build: k0s
endif

.PHONY: all
all: k0s k0s.exe

$(K0S_GO_BUILD_CACHE):
	mkdir -p -- '$@'

.k0sbuild.docker-image.k0s: build/Dockerfile embedded-bins/Makefile.variables | $(K0S_GO_BUILD_CACHE)
	docker build --rm \
		--build-arg BUILDIMAGE=$(GOLANG_IMAGE) \
		-f build/Dockerfile \
		-t k0sbuild.docker-image.k0s build/
	touch $@

go.sum: go.mod .k0sbuild.docker-image.k0s
	$(GO) mod tidy && touch -c -- '$@'

codegen_targets += pkg/apis/helm.k0sproject.io/v1beta1/.controller-gen.stamp
pkg/apis/helm.k0sproject.io/v1beta1/.controller-gen.stamp: $(shell find pkg/apis/helm.k0sproject.io/v1beta1/  -maxdepth 1 -type f -name \*.go)
pkg/apis/helm.k0sproject.io/v1beta1/.controller-gen.stamp: gen_output_dir = helm

codegen_targets += pkg/apis/k0s.k0sproject.io/v1beta1/.controller-gen.stamp
pkg/apis/k0s.k0sproject.io/v1beta1/.controller-gen.stamp: $(shell find pkg/apis/k0s.k0sproject.io/v1beta1/ -maxdepth 1 -type f -name \*.go)
pkg/apis/k0s.k0sproject.io/v1beta1/.controller-gen.stamp: gen_output_dir = v1beta1

codegen_targets += pkg/apis/autopilot.k0sproject.io/v1beta2/.controller-gen.stamp
pkg/apis/autopilot.k0sproject.io/v1beta2/.controller-gen.stamp: $(shell find pkg/apis/autopilot.k0sproject.io/v1beta2/ -maxdepth 1 -type f -name \*.go)
pkg/apis/autopilot.k0sproject.io/v1beta2/.controller-gen.stamp: gen_output_dir = autopilot

pkg/apis/%/.controller-gen.stamp: .k0sbuild.docker-image.k0s hack/tools/Makefile.variables
	rm -rf 'static/manifests/$(gen_output_dir)/CustomResourceDefinition'
	rm -f -- '$(dir $@)'zz_*.go
	CGO_ENABLED=0 $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(controller-gen_version)
	$(GO_ENV) controller-gen \
	  crd \
	  paths="./$(dir $@)..." \
	  output:crd:artifacts:config=./static/manifests/$(gen_output_dir)/CustomResourceDefinition \
	  object
	touch -- '$@'

# Note: k0s.k0sproject.io omits the version from the output directory, so this needs
# special handling until this versioning layout is fixed.

codegen_targets += pkg/apis/k0s.k0sproject.io/v1beta1/.client-gen.stamp
pkg/apis/k0s.k0sproject.io/v1beta1/.client-gen.stamp: $(shell find pkg/apis/k0s.k0sproject.io/v1beta1/ -maxdepth 1 -type f -name \*.go -not -name zz_\*.go)
pkg/apis/k0s.k0sproject.io/v1beta1/.client-gen.stamp: groupver = k0s.k0sproject.io/v1beta1
pkg/apis/k0s.k0sproject.io/v1beta1/.client-gen.stamp: gen_output_dir = k0s.k0sproject.io

codegen_targets += pkg/apis/autopilot.k0sproject.io/v1beta2/.client-gen.stamp
pkg/apis/autopilot.k0sproject.io/v1beta2/.client-gen.stamp: $(shell find pkg/apis/autopilot.k0sproject.io/v1beta2/ -maxdepth 1 -type f -name \*.go -not -name zz_\*.go)
pkg/apis/autopilot.k0sproject.io/v1beta2/.client-gen.stamp: groupver = autopilot.k0sproject.io/v1beta2
pkg/apis/autopilot.k0sproject.io/v1beta2/.client-gen.stamp: gen_output_dir = $(groupver)

pkg/apis/%/.client-gen.stamp: .k0sbuild.docker-image.k0s hack/tools/boilerplate.go.txt embedded-bins/Makefile.variables
	rm -rf 'pkg/apis/$(gen_output_dir)/clientset/'
	CGO_ENABLED=0 $(GO) install k8s.io/code-generator/cmd/client-gen@v0.25.1
	$(GO_ENV) client-gen \
	  --go-header-file hack/tools/boilerplate.go.txt \
	  --input=$(groupver) \
	  --input-base github.com/k0sproject/k0s/pkg/apis \
	  --clientset-name=clientset \
	  --output-package=github.com/k0sproject/k0s/pkg/apis/$(gen_output_dir)/
	touch -- '$@'

codegen_targets += static/gen_manifests.go
static/gen_manifests.go: .k0sbuild.docker-image.k0s hack/tools/Makefile.variables
static/gen_manifests.go: $(shell find static/manifests -type f)
	-rm -f -- '$@'
	CGO_ENABLED=0 $(GO) install github.com/kevinburke/go-bindata/go-bindata@v$(go-bindata_version)
	$(GO_ENV) go-bindata -o static/gen_manifests.go -pkg static -prefix static static/manifests/...

codegen_targets += pkg/assets/zz_generated_offsets_$(TARGET_OS).go
zz_os = $(patsubst pkg/assets/zz_generated_offsets_%.go,%,$@)
print_empty_generated_offsets = printf "%s\n\n%s\n%s\n" \
			"package assets" \
			"var BinData = map[string]struct{ offset, size, originalSize int64 }{}" \
			"var BinDataSize int64"
ifeq ($(EMBEDDED_BINS_BUILDMODE),none)
pkg/assets/zz_generated_offsets_linux.go pkg/assets/zz_generated_offsets_windows.go:
	rm -f bindata_$(zz_os) && touch bindata_$(zz_os)
	$(print_empty_generated_offsets) > $@
else
pkg/assets/zz_generated_offsets_linux.go: .bins.linux.stamp
pkg/assets/zz_generated_offsets_windows.go: .bins.windows.stamp
pkg/assets/zz_generated_offsets_linux.go pkg/assets/zz_generated_offsets_windows.go: .k0sbuild.docker-image.k0s go.sum
	GOOS=${GOHOSTOS} $(GO) run -tags=hack hack/gen-bindata/cmd/main.go -o bindata_$(zz_os) -pkg assets \
	     -gofile pkg/assets/zz_generated_offsets_$(zz_os).go \
	     -prefix embedded-bins/staging/$(zz_os)/ embedded-bins/staging/$(zz_os)/bin
endif

# needed for unit tests on macos
pkg/assets/zz_generated_offsets_darwin.go:
	$(print_empty_generated_offsets) > $@

k0s: TARGET_OS = linux
k0s: BUILD_GO_CGO_ENABLED = 1
k0s: .k0sbuild.docker-image.k0s

k0s.exe: TARGET_OS = windows
k0s.exe: BUILD_GO_CGO_ENABLED = 0

k0s.exe k0s: $(GO_SRCS) $(codegen_targets) go.sum
	CGO_ENABLED=$(BUILD_GO_CGO_ENABLED) GOOS=$(TARGET_OS) $(GO) build $(BUILD_GO_FLAGS) -ldflags='$(LD_FLAGS)' -o $@.code main.go
	cat $@.code bindata_$(TARGET_OS) > $@.tmp \
		&& rm -f $@.code \
		&& chmod +x $@.tmp \
		&& mv $@.tmp $@

.bins.windows.stamp .bins.linux.stamp: embedded-bins/Makefile.variables
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE) TARGET_OS=$(patsubst .bins.%.stamp,%,$@)
	touch $@

.PHONY: codegen
codegen: $(codegen_targets)

# bindata contains the parts of codegen which aren't version controlled.
.PHONY: bindata
bindata: static/gen_manifests.go pkg/assets/zz_generated_offsets_$(TARGET_OS).go

.PHONY: lint
lint: GOLANGCI_LINT_FLAGS ?=
lint: .k0sbuild.docker-image.k0s go.sum codegen
	CGO_ENABLED=0 $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v$(golangci-lint_version)
	$(GO_ENV) golangci-lint run --verbose $(GOLANGCI_LINT_FLAGS) $(GO_DIRS)

airgap-images.txt: k0s
	./k0s airgap list-images > '$@' || { \
	  code=$$? && \
	  rm -f -- '$@' && \
	  exit $$code ; \
	}

airgap-image-bundle-linux-amd64.tar: TARGET_PLATFORM := linux/amd64
airgap-image-bundle-linux-arm64.tar: TARGET_PLATFORM := linux/arm64
airgap-image-bundle-linux-arm.tar:   TARGET_PLATFORM := linux/arm/v7
airgap-image-bundle-linux-amd64.tar \
airgap-image-bundle-linux-arm64.tar \
airgap-image-bundle-linux-arm.tar: .k0sbuild.image-bundler.stamp airgap-images.txt
	docker run --rm -i --privileged \
	  -e TARGET_PLATFORM='$(TARGET_PLATFORM)' \
	  k0sbuild.image-bundler < airgap-images.txt > '$@' || { \
	    code=$$? && rm -f -- '$@' && exit $$code ; \
	  }

.k0sbuild.image-bundler.stamp: hack/image-bundler/*
	docker build -t k0sbuild.image-bundler hack/image-bundler
	touch -- '$@'

.PHONY: $(smoketests)
check-airgap check-ap-airgap: airgap-image-bundle-linux-$(HOST_ARCH).tar
$(smoketests): k0s
	$(MAKE) -C inttest K0S_IMAGES_BUNDLE='$(CURDIR)/airgap-image-bundle-linux-$(HOST_ARCH).tar' $@

.PHONY: smoketests
smoketests: $(smoketests)

.PHONY: check-unit
check-unit: GO_TEST_RACE ?= -race
check-unit: go.sum codegen
	$(GO) test -tags=hack $(GO_TEST_RACE) -timeout=30s -ldflags='$(LD_FLAGS)' `$(GO) list -tags=hack $(GO_DIRS)`

.PHONY: check-image-validity
check-image-validity: go.sum
	$(GO) run -tags=hack hack/validate-images/main.go -architectures amd64,arm64,arm

.PHONY: clean-gocache
clean-gocache:
	-find $(K0S_GO_BUILD_CACHE)/go/mod -type d -exec chmod u+w '{}' \;
	rm -rf $(K0S_GO_BUILD_CACHE)/go

clean-docker-image:
	-docker rmi k0sbuild.docker-image.k0s -f
	-rm -f .k0sbuild.docker-image.k0s

clean-airgap-image-bundles:
	-docker rmi -f k0sbuild.image-bundler.k0s
	-rm airgap-images.txt .k0sbuild.image-bundler.stamp
	-rm airgap-image-bundle-linux-amd64.tar airgap-image-bundle-linux-arm64.tar airgap-image-bundle-linux-arm.tar

.PHONY: clean
clean: clean-gocache clean-docker-image clean-airgap-image-bundles
	-rm -f pkg/assets/zz_generated_offsets_*.go k0s k0s.exe .bins.*stamp bindata* static/gen_manifests.go
	-rm -rf $(K0S_GO_BUILD_CACHE) 
	-find pkg/apis -type f \( -name .client-gen.stamp -or -name .controller-gen.stamp \) -delete
	-$(MAKE) -C docs clean
	-$(MAKE) -C embedded-bins clean
	-$(MAKE) -C inttest clean

.PHONY: manifests
manifests: .helmCRD .cfgCRD

.PHONY: .helmCRD


.PHONY: docs
docs:
	$(MAKE) -C docs

.PHONY: docs-serve-dev
docs-serve-dev: DOCS_DEV_PORT ?= 8000
docs-serve-dev:
	$(MAKE) -C docs .docker-image.serve-dev.stamp
	docker run --rm \
	  -v "$(CURDIR):/k0s:ro" \
	  -p '$(DOCS_DEV_PORT):8000' \
	  k0sdocs.docker-image.serve-dev
