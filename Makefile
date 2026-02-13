# arrctl Makefile
# Convenience targets for development and installation

.PHONY: install uninstall test lint clean help

# Default target
help:
	@echo "arrctl - Unified CLI for managing *arr services"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  install    Install arrctl (runs install.sh)"
	@echo "  uninstall  Remove arrctl installation"
	@echo "  test       Run smoke tests"
	@echo "  lint       Run shellcheck on all scripts"
	@echo "  clean      Remove generated files"
	@echo "  help       Show this help"
	@echo ""
	@echo "Environment variables:"
	@echo "  INSTALL_DIR  Installation directory (default: ~/.arrctl)"
	@echo "  BIN_DIR      Symlink directory (default: /usr/local/bin)"

install:
	@chmod +x install.sh
	@./install.sh

uninstall:
	@echo "Removing arrctl..."
	@rm -f "$${BIN_DIR:-/usr/local/bin}/arrctl" 2>/dev/null || sudo rm -f "$${BIN_DIR:-/usr/local/bin}/arrctl"
	@rm -rf "$${INSTALL_DIR:-$$HOME/.arrctl}"
	@echo "arrctl removed. Config at ~/.config/arrctl was preserved."
	@echo "To remove config: rm -rf ~/.config/arrctl"

test:
	@chmod +x test/smoke.sh
	@./test/smoke.sh

lint:
	@echo "Running shellcheck..."
	@shellcheck bin/arrctl lib/*.sh install.sh test/*.sh
	@echo "All checks passed!"

clean:
	@echo "Nothing to clean."
