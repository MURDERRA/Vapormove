# Vapormove

Vaporwave motion trail effects for Hyprland (Wayland) and X11.
Draws a fading gradient trail behind the cursor and dragged windows.

Written in Go with cgo for native Wayland (wlr-layer-shell + EGL/OpenGL)
and X11 (XComposite + GLX) backends.

## Features

- Cursor trail with configurable gradient (default: cyan to violet)
- Window drag trail rendered below all windows (Katana Zero style ghosts)
- Multi-monitor support: trails appear on the correct monitor
- Hot-configurable: colors, trail length, fade duration
- Wayland: native wlr-layer-shell overlay, input pass-through
- X11: XComposite overlay window, XShape input pass-through
- Config file (TOML) and CLI flags, flags override config file

## Dependencies

Runtime:

- Wayland backend: `wayland-client`, `wayland-egl`, `egl`, `gl`
- X11 backend: `libX11`, `libXcomposite`, `libXfixes`, `libGL`, `xdotool`
- `hyprctl` in PATH (Wayland cursor/window position)

Build:

- Go 1.22+
- `gcc` / `clang`
- `pkg-config`
- `wayland-scanner`
- `wlr-protocols` (for layer-shell header generation)

On Arch Linux:

```sh
sudo pacman -S go gcc wayland wayland-protocols mesa libxcomposite xdotool
# wlr-protocols from AUR:
yay -S wlr-protocols
```

## Build

```sh
git clone https://github.com/MURDERRA/Vapormove
cd Vapormove

# Generate wayland protocol bindings (once)
make generate

# Build
make build

# Install to /usr/local/bin
sudo make install
```

## Installation from Packages

Pre-built packages are available in the repository root:

- **Debian/Ubuntu**: `sudo dpkg -i vapormove_0.1.0_amd64.deb`
- **Fedora/RHEL**: `sudo rpm -i vapormove-0.1.0-1.x86_64.rpm`
- **Arch Linux**: `sudo pacman -U vapormove-0.1.0-1-x86_64.pkg.tar.zst`

### AUR (Arch User Repository)

You can also install via AUR using an AUR helper:

```sh
yay -S vapormove
# or
paru -S vapormove
```

## Usage

```sh
# Auto-detect backend (wayland if WAYLAND_DISPLAY is set, else x11)
vapormove

# Force backend
vapormove --backend wayland
vapormove --backend x11

# Override trail settings via flags
vapormove --cursor-length 32 --cursor-fade 500
vapormove --no-window        # cursor trail only
vapormove --no-cursor        # window trail only

# Print default config and exit
vapormove --dump-config > ~/.config/vapormove/config.toml
```

## Configuration

Default config location: `$XDG_CONFIG_HOME/vapormove/config.toml`
(usually `~/.config/vapormove/config.toml`)

Generate default config:

```sh
mkdir -p ~/.config/vapormove
vapormove --dump-config > ~/.config/vapormove/config.toml
```

Example config:

```toml
# Force backend: "wayland" or "x11" (default: auto-detect)
# backend = "wayland"

[cursor]
length     = 24      # number of trail points
fade_ms    = 400     # fade duration in milliseconds
point_size = 6.0     # point radius in pixels

[[cursor.gradient]]
position = 0.0
color    = "#00eeffcc"   # tip: cyan

[[cursor.gradient]]
position = 0.5
color    = "#8822ffaa"   # mid: violet

[[cursor.gradient]]
position = 1.0
color    = "#cc00ff44"   # tail: purple fade

[window]
length  = 12
fade_ms = 380

[[window.gradient]]
position = 0.0
color    = "#1144ffcc"

[[window.gradient]]
position = 0.5
color    = "#7711ee88"

[[window.gradient]]
position = 1.0
color    = "#cc11aa33"
```

Colors are in `#RRGGBB` or `#RRGGBBAA` format.
Gradient `position` is in [0.0, 1.0]: 0 = tip (newest point), 1 = tail (oldest).

## Autostart

Add to `~/.config/hypr/hyprland.conf`:

```ini
exec-once = vapormove
```

Or with systemd user session:

```sh
# ~/.config/systemd/user/vapormove.service
[Unit]
Description=Vaporwave motion trail
PartOf=graphical-session.target

[Service]
ExecStart=/usr/local/bin/vapormove
Restart=on-failure

[Install]
WantedBy=graphical-session.target
```

```sh
systemctl --user enable --now vapormove
```
