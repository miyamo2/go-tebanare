GO ?= go
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
