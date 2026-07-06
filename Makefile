# Local development. `make install` builds straight into your bin path so
# the `itapu` command on PATH picks up the new build immediately.
# Override the destination like install.sh: ITAPU_INSTALL_DIR=/some/bin make install
INSTALL_DIR ?= $(or $(ITAPU_INSTALL_DIR),$(HOME)/.local/bin)

.PHONY: build install test

build:
	go build -o itapu .

install:
	go build -o $(INSTALL_DIR)/itapu .
	@echo "installed $(INSTALL_DIR)/itapu"

test:
	go test ./...
