// Package backend defines the interface that each platform backend must implement.
package backend

import (
	"os"

	"vapormove/config"
)

// Backend is the platform-specific implementation.
// It owns the display connection, overlay window(s), and render loop.
type Backend interface {
	// Run starts the event/render loop. Blocks until the process is killed
	// or an unrecoverable error occurs.
	Run() error

	// Reconfigure applies updated config without restarting (hot-reload).
	Reconfigure(cfg *config.Config) error

	// Close releases all resources.
	Close()
}

// Detect returns "wayland" if WAYLAND_DISPLAY is set, otherwise "x11"
func Detect() string {
	if wd := envOr("WAYLAND_DISPLAY", ""); wd != "" {
		return "wayland"
	}
	return "x11"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
