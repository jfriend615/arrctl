# arrctl Makefile
# Convenience targets for development and installation

ARRCTL_TEST_BIN := /private/tmp/arrctl-test-bin
GOCACHE ?= /private/tmp/arrctl-go-build-cache

.PHONY: install uninstall test test-go test-shell lint clean help

# Default target
help:
	@echo "arrctl - Unified CLI for managing *arr services"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  install    Install arrctl (runs install.sh)"
	@echo "  uninstall  Remove arrctl installation"
	@echo "  test       Run Go tests"
	@echo "  test-go    Run Go tests"
	@echo "  test-shell Run Go CLI smoke tests"
	@echo "  lint       Run shellcheck on all scripts"
	@echo "  clean      Remove generated files"
	@echo "  help       Show this help"

install:
	@chmod +x install.sh
	@./install.sh

uninstall:
	@echo "Removing arrctl..."
	@rm -f "$${BIN_DIR:-/usr/local/bin}/arrctl" 2>/dev/null || sudo rm -f "$${BIN_DIR:-/usr/local/bin}/arrctl"
	@echo "arrctl removed. Config at ~/.config/arrctl was preserved."
	@echo "To remove config: rm -rf ~/.config/arrctl"

test:
	@$(MAKE) test-go

test-go:
	@GOCACHE=$(GOCACHE) go test ./...

test-shell:
	@GOCACHE=$(GOCACHE) go build -buildvcs=false -o $(ARRCTL_TEST_BIN) ./cmd/arrctl
	@chmod +x test/smoke.sh test/completion.sh test/go-smoke.sh test/install-smoke.sh
	@ARRCTL_BIN=$(ARRCTL_TEST_BIN) ./test/go-smoke.sh
	@ARRCTL_BIN=$(ARRCTL_TEST_BIN) ./test/completion.sh
	@./test/install-smoke.sh

lint:
	@echo "Running shellcheck..."
	@shellcheck bin/arrctl lib/*.sh install.sh test/*.sh
	@echo "All checks passed!"

clean:
	@echo "Nothing to clean."
