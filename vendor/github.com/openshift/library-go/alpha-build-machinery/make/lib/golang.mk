GO ?=go
GOPATH ?=$(shell $(GO) env GOPATH)
GO_PACKAGE ?=$(shell $(GO) list -m -f '{{ .Path }}' || echo 'no_package_detected')

GOOS ?=$(shell $(GO) env GOOS)
GOHOSTOS ?=$(shell $(GO) env GOHOSTOS)
GOARCH ?=$(shell $(GO) env GOARCH)
GOHOSTARCH ?=$(shell $(GO) env GOHOSTARCH)
GOEXE ?=$(shell $(GO) env GOEXE)
GOFLAGS ?=$(shell $(GO) env GOFLAGS)

GOFMT ?=gofmt
GOFMT_FLAGS ?=-s -l
GOLINT ?=golint

GO_FILES ?=$(shell find . -name '*.go' -not -path '*/vendor/*' -not -path '*/_output/*' -print)
GO_PACKAGES ?=./...
GO_TEST_PACKAGES ?=$(GO_PACKAGES)

# Projects not using modules can clear the variable, but by default we want to prevent
# our projects with modules to unknowingly ignore vendor folder until golang is fixed to use
# vendor folder by default if present.
#
# Conditional to avoid Go 1.13 bug on false double flag https://github.com/golang/go/issues/32471
# TODO: Drop the contitional when golang is fixed so we can see the flag being explicitelly set in logs.
ifeq "$(findstring -mod=vendor,$(GOFLAGS))" "-mod=vendor"
GO_MOD_FLAGS ?=
else
GO_MOD_FLAGS ?=-mod=vendor
endif

GO_BUILD_PACKAGES ?=./cmd/...
GO_BUILD_PACKAGES_EXPANDED ?=$(shell $(GO) list $(GO_MOD_FLAGS) $(GO_BUILD_PACKAGES))
go_build_binaries =$(notdir $(GO_BUILD_PACKAGES_EXPANDED))
GO_BUILD_FLAGS ?=
GO_BUILD_BINDIR ?=

GO_TEST_FLAGS ?=-race

GO_LD_EXTRAFLAGS ?=

SOURCE_GIT_TAG ?=$(shell git describe --long --tags --abbrev=7 --match 'v[0-9]*' || echo 'v0.0.0-unknown')
SOURCE_GIT_COMMIT ?=$(shell git rev-parse --short "HEAD^{commit}" 2>/dev/null)
SOURCE_GIT_TREE_STATE ?=$(shell ( ( [ ! -d ".git/" ] || git diff --quiet ) && echo 'clean' ) || echo 'dirty')

define version-ldflags
-X $(1).versionFromGit="$(SOURCE_GIT_TAG)" \
-X $(1).commitFromGit="$(SOURCE_GIT_COMMIT)" \
-X $(1).gitTreeState="$(SOURCE_GIT_TREE_STATE)" \
-X $(1).buildDate="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
endef
GO_LD_FLAGS ?=-ldflags "-s -w $(call version-ldflags,$(GO_PACKAGE)/pkg/version) $(GO_LD_EXTRAFLAGS)"
