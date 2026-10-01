# tomfoolery build. `make build` produces a universal (arm64 + x86_64) binary.
BIN      := tomfoolery
BUILD    := build
PREFIX   ?= /usr/local
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0)
LDFLAGS  := -s -w -X github.com/EmberGuild-Labs/tomfoolery/internal/cli.Version=$(VERSION)
PKG      := ./cmd/tomfoolery

.PHONY: build test lint install dev-install dist clean

build:
	@mkdir -p $(BUILD)
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BUILD)/$(BIN)-arm64 $(PKG)
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BUILD)/$(BIN)-amd64 $(PKG)
	lipo -create -output $(BUILD)/$(BIN) $(BUILD)/$(BIN)-arm64 $(BUILD)/$(BIN)-amd64
	@rm -f $(BUILD)/$(BIN)-arm64 $(BUILD)/$(BIN)-amd64
	@echo "Built $(BUILD)/$(BIN) ($(VERSION))"

test:
	go test ./...

lint:
	go vet ./...
	@if command -v staticcheck >/dev/null; then staticcheck ./...; else echo "staticcheck not installed; skipped"; fi

# sudo make install                  -> /usr/local/bin
# make install PREFIX=$$HOME/.local   -> ~/.local/bin, no sudo
install: build
	install -d $(PREFIX)/bin
	install -m 0755 $(BUILD)/$(BIN) $(PREFIX)/bin/$(BIN)
	@echo "Installed $(PREFIX)/bin/$(BIN). Now run: $(BIN) install"

# Builds and runs the one-time setup straight from ./build, for local testing.
dev-install: build
	$(BUILD)/$(BIN) install --no-intro

# Release artifacts that install.sh downloads: a tarball and its checksum.
dist: build
	@rm -rf dist && mkdir -p dist/stage
	cp $(BUILD)/$(BIN) README.md LICENSE dist/stage/
	tar -czf dist/$(BIN)-darwin-universal.tar.gz -C dist/stage .
	@rm -rf dist/stage
	cd dist && shasum -a 256 $(BIN)-darwin-universal.tar.gz > SHA256SUMS
	@cat dist/SHA256SUMS

clean:
	rm -rf $(BUILD) dist
