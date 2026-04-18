BINARY      := devrune
CMD         := ./cmd/devrune
VERSION     ?= dev
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GOFLAGS     ?= -trimpath
LDFLAGS     ?= -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)
INSTALL_DIR ?= $(HOME)/.local/bin

.PHONY: build build-debug build-all install uninstall test vet fmt lint check setup clean run help

## build: Build optimized binary for the current platform
build:
	go build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o $(BINARY) $(CMD)

## build-debug: Build with full debug symbols (for dlv)
build-debug:
	go build -ldflags='-X main.version=$(VERSION) -X main.commit=$(COMMIT)' -o $(BINARY) $(CMD)

## install: Build and install to INSTALL_DIR (default: ~/.local/bin)
install: build
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@chmod 755 $(INSTALL_DIR)/$(BINARY)
	@echo "✓ $(BINARY) installed to $(INSTALL_DIR)/$(BINARY)"
	@# PATH check — warn if INSTALL_DIR is not in $PATH.
	@case ":$$PATH:" in *":$(INSTALL_DIR):"*) ;; \
		*) printf '\033[33m!\033[0m %s is not in your PATH.\n' "$(INSTALL_DIR)"; \
		   printf '  Add: \033[2mexport PATH="%s:$$PATH"\033[0m to your shell profile.\n' "$(INSTALL_DIR)" ;; \
	esac
	@# Shadow detection — warn if a different devrune binary precedes ours in PATH.
	@resolved="$$(command -v $(BINARY) 2>/dev/null || true)"; \
	expected="$(INSTALL_DIR)/$(BINARY)"; \
	if [ -n "$$resolved" ] && [ "$$resolved" != "$$expected" ]; then \
		printf '\033[33m!\033[0m Shadow binary: `which $(BINARY)` → %s\n' "$$resolved"; \
		printf '  That path precedes %s in PATH. Remove it or reorder PATH to use the fresh build.\n' "$(INSTALL_DIR)"; \
	fi

## uninstall: Remove binary from INSTALL_DIR
uninstall:
	@rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "$(BINARY) removed from $(INSTALL_DIR)/$(BINARY)"

## build-all: Cross-compile for darwin-arm64, darwin-amd64, linux-amd64
build-all:
	GOOS=darwin  GOARCH=arm64 go build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o $(BINARY)-darwin-arm64  $(CMD)
	GOOS=darwin  GOARCH=amd64 go build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o $(BINARY)-darwin-amd64  $(CMD)
	GOOS=linux   GOARCH=amd64 go build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o $(BINARY)-linux-amd64   $(CMD)

## test: Run all tests with race detector and coverage
test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format source code
fmt:
	gofmt -w .

## lint: Run golangci-lint (same linters as CI)
lint:
	go tool -modfile=golangci-lint.mod golangci-lint run ./...

## check: Run fmt + vet + lint + tests (full pre-push validation)
check: fmt vet lint test

## setup: Install git hooks for local development
setup:
	@cp scripts/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-push
	@echo "Git hooks installed."

## clean: Remove build artifacts
clean:
	rm -f $(BINARY) $(BINARY)-darwin-arm64 $(BINARY)-darwin-amd64 $(BINARY)-linux-amd64
	rm -f coverage.txt coverage.html
	rm -rf dist/

## run: Run directly via go run
run:
	go run -ldflags='-X main.version=$(VERSION) -X main.commit=$(COMMIT)' $(CMD)

## help: Print this help message
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
