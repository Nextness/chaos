# Chaos compiler build
#
# Builds the `chaosc` compiler binary and the `chaos-lsp` language server
# binary into ./build.

.PHONY: all test vet e2e clean

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
	go test -C examples ./...

clean:
	rm -rf build
