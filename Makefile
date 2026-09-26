GO ?= go
TINYGO ?= tinygo
NODE ?= node
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
ENGINE_WASM := packages/engine/wasm/engine.wasm
FUZZTIME ?= 10s
# Minimizing a new input may take this long. Go's default of 60s stalls a
# short run: FuzzCompile stops executing inputs after a few seconds.
FUZZMINIMIZETIME ?= 1s

.PHONY: all test vet lint fmt-check fuzz-smoke

all: vet test

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

lint:
	golangci-lint run ./...

# Files under testdata are skipped: some hold syntax errors on purpose.
fmt-check:
	@out=$$(find . -name '*.go' -not -path '*/testdata/*' -not -path '*/node_modules/*' | xargs gofmt -l) || exit 1; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Runs every Fuzz* test for FUZZTIME each.
fuzz-smoke:
	@set -e; for pkg in $$($(GO) list ./...); do \
		for fz in $$($(GO) test -list '^Fuzz' $$pkg | grep '^Fuzz' || true); do \
			echo "$$pkg $$fz"; \
			$(GO) test -run '^$$' -fuzz "^$$fz$$" -fuzztime $(FUZZTIME) -fuzzminimizetime $(FUZZMINIMIZETIME) $$pkg; \
		done; \
	done

.PHONY: wasm wasm-check wasm-wasip1

wasm:
	@mkdir -p $(dir $(ENGINE_WASM))
	$(TINYGO) build -target=build/tinygo/engine.json -no-debug \
		-ldflags="-X github.com/miyamo2/go-tebanare.EngineVersion=$(VERSION)" \
		-o $(ENGINE_WASM) ./cmd/gotebanare-wasm

# Builds engine.wasm only when it is missing. Run make wasm after changing
# Go code.
$(ENGINE_WASM):
	$(MAKE) wasm

wasm-check:
	$(NODE) packages/engine/scripts/check-wasm.mjs $(ENGINE_WASM)

# Builds the same exports with the standard Go toolchain, the fallback if
# TinyGo cannot be used (plan 2.3).
wasm-wasip1:
	GOOS=wasip1 GOARCH=wasm $(GO) build -buildmode=c-shared -o /dev/null ./cmd/gotebanare-wasm

PARITY_JSON ?= $(or $(TMPDIR),/tmp)/gotebanare-parity.json
# PARITY_FLAGS=-pairs makes the parity test pair declarations across files.
PARITY_FLAGS ?=

.PHONY: parity

# Analyzes the Go standard library with the native build and with
# engine.wasm and compares the results (plan 8).
parity: $(ENGINE_WASM)
	root="$$($(GO) env GOROOT)/src"; \
	$(GO) run ./internal/tools/corpusdump -config testdata/parity/config.yml -root "$$root" $(PARITY_FLAGS) > $(PARITY_JSON) && \
	GOTEBANARE_PARITY=$(PARITY_JSON) GOTEBANARE_PARITY_ROOT="$$root" \
		bun run --cwd packages/engine vitest run test/parity.test.ts
