# Chaos compiler build
#
# Builds the `chaosc` compiler binary and the `chaos-lsp` language server
# binary into ./build.

.PHONY: all fmt fmt-check test vet e2e check race coverage clean

all: build/chaosc build/chaos-lsp

SRC := $(shell find src -name '*.go')

build/chaosc: $(SRC) src/go.mod
	mkdir -p build
	go build -C src -o ../build/chaosc ./cmd/chaosc

build/chaos-lsp: $(SRC) src/go.mod
	mkdir -p build
	go build -C src -o ../build/chaos-lsp ./cmd/chaos-lsp

test: $(SRC)
	go test -C src ./...

vet: $(SRC)
	go vet -C src ./...

e2e: $(SRC) examples/go.mod
	@if command -v fasm >/dev/null 2>&1; then \
		echo "fasm found: executable runtime checks enabled"; \
	else \
		echo "fasm not found: executable runtime checks will be skipped"; \
	fi
	go test -C examples ./...

fmt:
	gofmt -w $(SRC)

fmt-check:
	@files="$$(gofmt -l $(SRC))"; \
	if [ -n "$$files" ]; then \
		echo "Go files need formatting:"; \
		echo "$$files"; \
		exit 1; \
	fi

check: fmt-check vet test e2e

race: $(SRC) examples/go.mod
	go test -C src -race ./...
	go test -C examples -race ./...

coverage: $(SRC) examples/go.mod
	mkdir -p build/coverage
	go test -C src -coverprofile=../build/coverage/src.out ./...
	go test -C examples -coverprofile=../build/coverage/examples.out ./...

clean:
	rm -rf build
