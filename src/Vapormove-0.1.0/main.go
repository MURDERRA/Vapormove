package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"vapormove/backend"
	"vapormove/backend/wayland"
	"vapormove/backend/x11"
	"vapormove/config"
)

func init() {
	// OpenGL and Wayland both require operations on the main OS thread
	runtime.LockOSThread()
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vapormove: config error: %v\n", err)
		os.Exit(1)
	}

	// Detect or force backend
	backendName := cfg.Backend
	if backendName == "" {
		backendName = backend.Detect()
	}

	var b backend.Backend
	switch backendName {
	case "wayland":
		b, err = wayland.New(cfg)
	case "x11":
		b, err = x11.New(cfg)
	default:
		fmt.Fprintf(os.Stderr, "vapormove: unknown backend %q (use 'wayland' or 'x11')\n", backendName)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "vapormove: backend init failed: %v\n", err)
		os.Exit(1)
	}
	defer b.Close()

	fmt.Printf("vapormove: running on %s backend\n", backendName)

	// Handle SIGINT / SIGTERM gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nvapormove: shutting down")
		b.Close()
		os.Exit(0)
	}()

	if err := b.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "vapormove: runtime error: %v\n", err)
		os.Exit(1)
	}
}
