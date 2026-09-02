# omarchy-skk-popup: Go engine + QML panel for the Omarchy shell.

BIN      ?= skk-popup-engine
PREFIX   ?= $(HOME)/.local
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# Vendored binaries report the plugin's manifest version so Settings shows a
# stable number regardless of the checkout's git state.
VENDOR_VERSION ?= $(shell sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' manifest.json)
LDFLAGS  := -s -w -X main.version=$(VERSION)
PLUGIN_DIR ?= $(HOME)/.config/omarchy/plugins/takeshy.skk-popup
ARCHES   := amd64 arm64

.PHONY: build vendor-engine install install-engine install-plugin dict test validate clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/skk-popup-engine

# Build the arch binaries that ship with the plugin (bin/skk-popup-engine-linux-*).
# These are committed so `omarchy plugin add/update` delivers a matching engine.
vendor-engine:
	@for arch in $(ARCHES); do \
	  echo "vendor: linux/$$arch @ $(VENDOR_VERSION)"; \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath \
	    -ldflags "-s -w -X main.version=$(VENDOR_VERSION)" \
	    -o bin/$(BIN)-linux-$$arch ./cmd/skk-popup-engine; \
	done

# Build the engine into ~/.local/bin. Optional now that the plugin bundles
# one; kept for development and for those who prefer their own build.
install-engine: build
	install -Dm755 bin/$(BIN) $(PREFIX)/bin/$(BIN)

# Symlink-free copy of the plugin into the Omarchy plugins dir for hacking
# (omarchy plugin add <git url> is the normal install path).
install-plugin: vendor-engine
	mkdir -p $(PLUGIN_DIR)/scripts $(PLUGIN_DIR)/bin
	cp manifest.json Panel.qml BarWidget.qml SkkButton.qml SkkModeBadge.qml README.md LICENSE $(PLUGIN_DIR)/
	cp scripts/fetch-engine.sh scripts/setup.sh $(PLUGIN_DIR)/scripts/
	# install (unlink + create) so a running engine's binary can be replaced.
	for f in bin/$(BIN)-linux-*; do install -m755 "$$f" "$(PLUGIN_DIR)/bin/$$(basename $$f)"; done
	-omarchy-shell shell rescanPlugins

install: install-plugin

# Download SKK-JISYO.L etc. into ~/.local/share/skk-popup/dict.
dict: build
	bin/$(BIN) dict fetch

test:
	go vet ./...
	go test ./...

# Same checks as `omarchy plugin validate` (needs jq).
validate:
	./scripts/validate-manifest.sh .

clean:
	rm -rf bin dist
