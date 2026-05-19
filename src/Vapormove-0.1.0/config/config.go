package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Color represents an RGBA color with float32 components [0.0, 1.0]
type Color struct {
	R, G, B, A float32
}

// ParseHex parses "#RRGGBB" or "#RRGGBBAA" into a Color
func ParseHex(s string) (Color, error) {
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	var r, g, b, a uint32
	a = 255
	switch len(s) {
	case 6:
		_, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
		if err != nil {
			return Color{}, fmt.Errorf("invalid color: #%s", s)
		}
	case 8:
		_, err := fmt.Sscanf(s, "%02x%02x%02x%02x", &r, &g, &b, &a)
		if err != nil {
			return Color{}, fmt.Errorf("invalid color: #%s", s)
		}
	default:
		return Color{}, fmt.Errorf("invalid color format: #%s (expected #RRGGBB or #RRGGBBAA)", s)
	}
	return Color{
		R: float32(r) / 255,
		G: float32(g) / 255,
		B: float32(b) / 255,
		A: float32(a) / 255,
	}, nil
}

// GradientStop is a color stop in the trail gradient
type GradientStop struct {
	// Position in [0.0, 1.0]: 0 = tip (newest), 1 = tail (oldest)
	Position float32
	Hex      string `toml:"color"`
	Color    Color  `toml:"-"` // parsed from Hex
}

// TrailConfig holds trail-specific settings
type TrailConfig struct {
	// Length is the number of ghost points kept in the trail
	Length int `toml:"length"`
	// FadeMs is the duration in milliseconds for a ghost to fade out
	FadeMs int `toml:"fade_ms"`
	// PointSize is the radius of each trail point in pixels
	PointSize float32 `toml:"point_size"`
	// Gradient is the color stops from tip (0.0) to tail (1.0)
	Gradient []GradientStop `toml:"gradient"`
}

// Config is the top-level configuration
type Config struct {
	// Backend forces "wayland" or "x11"; empty = auto-detect
	Backend string      `toml:"backend"`
	Cursor  TrailConfig `toml:"cursor"`
	Window  TrailConfig `toml:"window"`
}

// Default returns a config with vaporwave defaults
func Default() *Config {
	return &Config{
		Backend: "",
		Cursor: TrailConfig{
			Length:    24,
			FadeMs:    400,
			PointSize: 6,
			Gradient: []GradientStop{
				{Position: 0.0, Hex: "#00eeffcc"}, // tip: cyan
				{Position: 0.5, Hex: "#8822ffaa"}, // mid: violet
				{Position: 1.0, Hex: "#cc00ff44"}, // tail: purple fade
			},
		},
		Window: TrailConfig{
			Length:    12,
			FadeMs:    380,
			PointSize: 0, // unused for window trail (full rect)
			Gradient: []GradientStop{
				{Position: 0.0, Hex: "#1144ffcc"}, // tip: blue
				{Position: 0.5, Hex: "#7711ee88"}, // mid: violet
				{Position: 1.0, Hex: "#cc11aa33"}, // tail: magenta fade
			},
		},
	}
}

// parseGradients resolves hex strings to Color values in-place
func parseGradients(cfg *Config) error {
	for i := range cfg.Cursor.Gradient {
		c, err := ParseHex(cfg.Cursor.Gradient[i].Hex)
		if err != nil {
			return fmt.Errorf("cursor gradient[%d]: %w", i, err)
		}
		cfg.Cursor.Gradient[i].Color = c
	}
	for i := range cfg.Window.Gradient {
		c, err := ParseHex(cfg.Window.Gradient[i].Hex)
		if err != nil {
			return fmt.Errorf("window gradient[%d]: %w", i, err)
		}
		cfg.Window.Gradient[i].Color = c
	}
	return nil
}

// Load reads config from file (if it exists), then overlays CLI flags.
// Priority: CLI flags > config file > defaults
func Load() (*Config, error) {
	cfg := Default()

	// ── CLI flags ────────────────────────────────────────────────────
	var (
		flagConfig     = flag.String("config", "", "path to config file (default: $XDG_CONFIG_HOME/vapormove/config.toml)")
		flagBackend    = flag.String("backend", "", "force backend: wayland or x11")
		flagCursorLen  = flag.Int("cursor-length", 0, "cursor trail length (number of points)")
		flagCursorFade = flag.Int("cursor-fade", 0, "cursor trail fade duration in ms")
		flagWindowLen  = flag.Int("window-length", 0, "window trail length")
		flagWindowFade = flag.Int("window-fade", 0, "window trail fade duration in ms")
		flagNoWindow   = flag.Bool("no-window", false, "disable window trail")
		flagNoCursor   = flag.Bool("no-cursor", false, "disable cursor trail")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: vapormove [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Config file locations (checked in order):
  $VAPORMOVE_CONFIG
  $XDG_CONFIG_HOME/vapormove/config.toml
  ~/.config/vapormove/config.toml

Example config: vapormove --dump-config
`)
	}

	var flagDump = flag.Bool("dump-config", false, "print default config as TOML and exit")
	flag.Parse()

	if *flagDump {
		DumpDefault()
		os.Exit(0)
	}

	// ── Config file ──────────────────────────────────────────────────
	cfgPath := *flagConfig
	if cfgPath == "" {
		cfgPath = os.Getenv("VAPORMOVE_CONFIG")
	}
	if cfgPath == "" {
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(os.Getenv("HOME"), ".config")
		}
		cfgPath = filepath.Join(xdg, "vapormove", "config.toml")
	}

	if _, err := os.Stat(cfgPath); err == nil {
		if _, err := toml.DecodeFile(cfgPath, cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", cfgPath, err)
		}
	}

	// ── CLI overrides ────────────────────────────────────────────────
	if *flagBackend != "" {
		cfg.Backend = *flagBackend
	}
	if *flagCursorLen > 0 {
		cfg.Cursor.Length = *flagCursorLen
	}
	if *flagCursorFade > 0 {
		cfg.Cursor.FadeMs = *flagCursorFade
	}
	if *flagWindowLen > 0 {
		cfg.Window.Length = *flagWindowLen
	}
	if *flagWindowFade > 0 {
		cfg.Window.FadeMs = *flagWindowFade
	}
	if *flagNoWindow {
		cfg.Window.Length = 0
	}
	if *flagNoCursor {
		cfg.Cursor.Length = 0
	}

	if err := parseGradients(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// DumpDefault prints a default config.toml to stdout
func DumpDefault() {
	fmt.Print(`# vapormove configuration
# Colors are in #RRGGBB or #RRGGBBAA format

# Force backend: "wayland" or "x11" (default: auto-detect)
# backend = "wayland"

[cursor]
length    = 24      # number of trail points
fade_ms   = 400     # fade duration in milliseconds
point_size = 6.0    # point radius in pixels

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
color    = "#1144ffcc"   # tip: blue

[[window.gradient]]
position = 0.5
color    = "#7711ee88"   # mid: violet

[[window.gradient]]
position = 1.0
color    = "#cc11aa33"   # tail: magenta fade
`)
}
