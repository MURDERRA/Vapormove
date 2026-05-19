BINARY := vapormove
PREFIX ?= /usr/local

# Generate wayland protocol glue (run once after cloning)
WAYLAND_SCANNER := wayland-scanner
PROTOCOLS_DIR := /usr/share/wayland-protocols
WLR_PROTOCOLS := /usr/share/wlr-protocols

.PHONY: all generate build install clean

all: generate build

generate:
	@echo "Generating wayland protocol bindings..."
	$(WAYLAND_SCANNER) client-header \
		$(PROTOCOLS_DIR)/stable/xdg-shell/xdg-shell.xml \
		backend/wayland/xdg-shell-client-protocol.h
	$(WAYLAND_SCANNER) private-code \
		$(PROTOCOLS_DIR)/stable/xdg-shell/xdg-shell.xml \
		backend/wayland/xdg-shell-protocol.c
	$(WAYLAND_SCANNER) client-header \
		$(WLR_PROTOCOLS)/unstable/wlr-layer-shell-unstable-v1.xml \
		backend/wayland/wlr-layer-shell-unstable-v1-client-protocol.h
	$(WAYLAND_SCANNER) private-code \
		$(WLR_PROTOCOLS)/unstable/wlr-layer-shell-unstable-v1.xml \
		backend/wayland/wlr-layer-shell-unstable-v1-protocol.c

build:
	go build -o $(BINARY) .

install: build
	install -Dm755 $(BINARY) $(PREFIX)/bin/$(BINARY)
	@echo "Installed to $(PREFIX)/bin/$(BINARY)"
	@echo ""
	@echo "To generate a default config:"
	@echo "  mkdir -p ~/.config/vapormove"
	@echo "  vapormove --dump-config > ~/.config/vapormove/config.toml"

clean:
	rm -f $(BINARY)
	rm -f backend/wayland/wlr-layer-shell-unstable-v1-client-protocol.h
	rm -f backend/wayland/wlr-layer-shell-unstable-v1-protocol.c
	rm -f backend/wayland/xdg-shell-client-protocol.h
	rm -f backend/wayland/xdg-shell-protocol.c
