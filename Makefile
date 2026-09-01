# omarchy-skk-popup: Go engine + QML panel for the Omarchy shell.

BIN      ?= skk-popup-engine
PREFIX   ?= $(HOME)/.local
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
PLUGIN_DIR ?= $(HOME)/.config/omarchy/plugins/takeshy.skk-popup

.PHONY: build install install-engine install-plugin dict test validate clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/skk-popup-engine

# Build the engine into ~/.local/bin (what Panel.qml looks for).
install-engine: build
	install -Dm755 bin/$(BIN) $(PREFIX)/bin/$(BIN)

# Symlink-free copy of the plugin into the Omarchy plugins dir for hacking
# (omarchy plugin add <git url> is the normal install path).
install-plugin:
	mkdir -p $(PLUGIN_DIR)
	cp manifest.json Panel.qml BarWidget.qml SkkButton.qml SkkModeBadge.qml README.md LICENSE $(PLUGIN_DIR)/
	-omarchy-shell shell rescanPlugins

install: install-engine install-plugin

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
