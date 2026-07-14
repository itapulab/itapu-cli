# Local development. `make install` builds straight into your bin path so
# the `itapu` command on PATH picks up the new build immediately.
# Override the destination like install.sh: ITAPU_INSTALL_DIR=/some/bin make install
INSTALL_DIR ?= $(or $(ITAPU_INSTALL_DIR),$(HOME)/.local/bin)

# Local builds are stamped with git describe (falling back to "dev") so
# `itapu version` never claims a release number it isn't; non-release
# versions also disable the daily update check.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install test release-patch release-minor release-major

build:
	go build -ldflags '$(LDFLAGS)' -o itapu .

install:
	go build -ldflags '$(LDFLAGS)' -o $(INSTALL_DIR)/itapu .
	@echo "installed $(INSTALL_DIR)/itapu"

test:
	go test ./...

# Publish a release: tags the next version and pushes it, which triggers
# the GoReleaser workflow. See scripts/release.sh for the safety checks.
release-patch:
	scripts/release.sh patch

release-minor:
	scripts/release.sh minor

release-major:
	scripts/release.sh major
