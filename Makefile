VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  = -s -w -X main.version=$(VERSION)

.PHONY: build test clean release

build:
	go build -ldflags "$(LDFLAGS)" -o bin/claude-config ./cmd/claude-config

test:
	go test ./... -v

# Build release tarballs for all supported platforms.
# Output: dist/claude-config_<version>_<os>_<arch>.tar.gz
release: clean
	@mkdir -p dist
	@for pair in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do \
		os=$${pair%/*}; arch=$${pair#*/}; \
		name="claude-config_$(VERSION)_$${os}_$${arch}"; \
		echo "Building $${name}..."; \
		GOOS=$${os} GOARCH=$${arch} go build -ldflags "$(LDFLAGS)" -o "dist/claude-config" ./cmd/claude-config && \
		tar -czf "dist/$${name}.tar.gz" -C dist claude-config && \
		rm dist/claude-config; \
	done
	@echo "Release artifacts in dist/"
	@ls -lh dist/

clean:
	rm -rf bin/ dist/
