# Chaos compiler build
#
# Builds the `chaosc` compiler binary into ./build.

.PHONY: all clean

all: build/chaosc

SRC := $(wildcard src/*.go)

build/chaosc: $(SRC) src/go.mod
	mkdir -p build
	go build -C src -o ../build/chaosc .

clean:
	rm -rf build
