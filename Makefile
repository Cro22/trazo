# trazo build and release tasks.
#
# The Go core is standard-library only, so these targets need nothing but the Go
# toolchain (plus tar/zip/sha256sum for `dist`, which are present on CI runners
# and most Unix hosts). On Windows, run under Git Bash or WSL.

BIN     := trazo
CMD     := ./cmd/trazo
BINDIR  := bin
DISTDIR := dist

# Version stamped into archive names. The binary itself reports the compiled-in
# const Version plus git metadata from the Go toolchain (see cmd/trazo/version.go);
# this is only for naming the release artifacts.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Release target matrix (GOOS/GOARCH).
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.DEFAULT_GOAL := build
.PHONY: build check test vet fmt install gate agent-test dist clean

build: ## Build the CLI into ./bin
	go build -trimpath -o $(BINDIR)/$(BIN) $(CMD)

check: vet test ## Full Go gate: vet then test
	go build ./...

test: ## Run the Go test suite
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go sources
	go fmt ./...

install: ## Install the CLI into GOBIN (go install)
	go install $(CMD)

# Mirror the CI trazo-gate job locally: clean traces pass, bad traces must fail.
gate: build ## Demonstrate the evaluator gate on the CI fixtures
	$(BINDIR)/$(BIN) -dir testdata/ci/clean
	@echo "clean traces passed (exit 0)"
	@if $(BINDIR)/$(BIN) -dir testdata/ci/failing; then \
	  echo "gate did not fail on a bad finding" >&2; exit 1; \
	fi
	@echo "bad traces correctly failed the gate (exit non-zero)"

# Run the Python reference-agent tests. Requires the venv from
# agents/langgraph-reference (see the README quickstart).
agent-test: ## Run the reference-agent pytest suite
	cd agents/langgraph-reference && pytest -q

# Cross-compile release archives plus a checksums file into ./dist. Each archive
# holds one static binary; unix targets are .tar.gz, windows is .zip.
dist: ## Build release archives for every target platform
	rm -rf $(DISTDIR)
	mkdir -p $(DISTDIR)
	@for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; \
	  ext=; [ $$os = windows ] && ext=.exe; \
	  echo "building $$os/$$arch"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	    go build -trimpath -o $(DISTDIR)/$(BIN)$$ext $(CMD) || exit 1; \
	  base=$(BIN)_$(VERSION)_$${os}_$${arch}; \
	  if [ $$os = windows ]; then \
	    (cd $(DISTDIR) && zip -q $$base.zip $(BIN)$$ext && rm $(BIN)$$ext); \
	  else \
	    (cd $(DISTDIR) && tar czf $$base.tar.gz $(BIN)$$ext && rm $(BIN)$$ext); \
	  fi; \
	done
	@(cd $(DISTDIR) && sha256sum * > checksums.txt)
	@echo "artifacts in $(DISTDIR):"
	@ls -1 $(DISTDIR)

clean: ## Remove build and release artifacts
	rm -rf $(BINDIR) $(DISTDIR) $(BIN) $(BIN).exe
