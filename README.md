# Vapormove

Vaporwave-style trail effects for Hyprland, built with Quickshell.

Two independent effects:
- **Window trail** — a blue-violet ghost follows dragged windows, rendered below all windows
- **Cursor trail** — a cyan-to-violet fading arrow trail follows the cursor

## Dependencies

- [Hyprland](https://hyprland.org)
- [Quickshell](https://quickshell.org)

## Installation

Clone the repository:

```fish
git clone https://github.com/yourusername/Vapormove ~/.config/quickshell/vapormove
```

The repository contains two separate configs:

```
vapormove/
├── vaportrail/
│   └── shell.qml       # window trail
└── cursortrail/
│   └── shell.qml       # cursor trail
└── README.md
```

Add to `~/.config/hypr/hyprland.conf`:

```ini
exec-once = qs -c vaportrail
exec-once = qs -c cursortrail
```

## How it works

**Window trail** listens to Hyprland socket2 events via `Hyprland.rawEvent`. When a window is being dragged, `activewindow` events fire continuously. On each event, `hyprctl activewindow -j` is called in a tight chain to poll the window position. A ghost rectangle is spawned at the previous position, creating a trail behind the window. The overlay runs on `WlrLayer.Bottom` so it renders beneath all windows.

**Cursor trail** polls `hyprctl cursorpos -j` in a continuous chain (no timer) for maximum update rate. On each position change, a cursor-shaped ghost is spawned at the old position. Each ghost receives a `trailIndex` that increments every frame, shifting its color from cyan (fresh, near cursor) to violet (old, end of trail). The overlay runs on `WlrLayer.Overlay` with an empty input region so clicks pass through.

## Customization

**Window trail** — edit `vaportrail/shell.qml`:

| Property              | Description             |
| --------------------- | ----------------------- |
| `duration: 380`       | fade-out duration in ms |
| `GradientStop` colors | trail color scheme      |

**Cursor trail** — edit `cursortrail/shell.qml`:

| Property          | Description                                         |
| ----------------- | --------------------------------------------------- |
| `trailLength: 18` | number of ghosts in the gradient                    |
| `duration: 380`   | fade-out duration in ms                             |
| `x: dotX - 50`    | cursor hotspot offset, adjust for your cursor theme |
| `y: dotY - 10`    | cursor hotspot offset, adjust for your cursor theme |

## Cursor hotspot offset

The `x (dotX) or y (dotY)` value in `cursortrail/shell.qml` depends on your cursor theme. If the ghost does not align with your cursor, adjust the offset until it matches.
