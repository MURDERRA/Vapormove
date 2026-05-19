package cursor

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// Sprite is a decoded cursor image ready for OpenGL texturing
type Sprite struct {
	Width   int
	Height  int
	HotspotX int // relative to top-left
	HotspotY int
	Pixels  []byte // RGBA, 4 bytes per pixel
}

// Atlas maps cursor names to their sprites images
type Atlas struct {
	Sprites map[string]*Sprite
	Size    int // preferred size in pixels
}

// NewAtlas creates an empty atlas
func NewAtlas(size int) *Atlas {
	return &Atlas{
		Sprites: make(map[string]*Sprite),
		Size:    size,
	}
}

// LoadTheme loads all cursor sprites from an X11 cursor theme directory.
// It looks in themeDir/cursors/ for standard cursor names.
func (a *Atlas) LoadTheme(themeDir string) error {
	cursorsDir := filepath.Join(themeDir, "cursors")
	if _, err := os.Stat(cursorsDir); os.IsNotExist(err) {
		return fmt.Errorf("cursor theme cursors dir not found: %s", cursorsDir)
	}

	names := []string{
		"default", "left_ptr", "arrow", "help", "wait",
		"crosshair", "text", "move", "pointer", "progress",
		"nw-resize", "ne-resize", "sw-resize", "se-resize",
		"n-resize", "s-resize", "e-resize", "w-resize",
		"ns-resize", "ew-resize", "nesw-resize", "nwse-resize",
		"hand1", "hand2", "grab", "grabbing",
		"col-resize", "row-resize",
		"forbidden", "not-allowed",
	}

	for _, name := range names {
		path := filepath.Join(cursorsDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		if err := a.loadCursorFile(name, path); err != nil {
			continue
		}
	}

	return nil
}

// loadCursorFile loads a single cursor file.  It first tries PNG,
// then falls back to the Xcursor binary format via xcur2png.
func (a *Atlas) loadCursorFile(name, path string) error {
	// Try PNG first (some themes ship PNGs)
	if s, err := loadPNG(path); err == nil {
		a.Sprites[name] = s
		return nil
	}

	// Try Xcursor binary format
	if sprites, err := ParseXCursor(path); err == nil && len(sprites) > 0 {
		// Pick the best-sized image (closest to requested)
		best := pickBestSize(sprites, a.Size)
		a.Sprites[name] = best
		return nil
	}

	return fmt.Errorf("unsupported cursor format: %s", path)
}

// Get returns a sprite by name, falling back to "default" or "left_ptr"
func (a *Atlas) Get(name string) *Sprite {
	if s, ok := a.Sprites[name]; ok {
		return s
	}
	// Common fallbacks
	for _, fb := range []string{"default", "left_ptr", "arrow"} {
		if s, ok := a.Sprites[fb]; ok {
			return s
		}
	}
	return nil
}

// pickBestSize returns the sprite closest to the requested size
func pickBestSize(sprites []*Sprite, size int) *Sprite {
	if len(sprites) == 1 {
		return sprites[0]
	}
	best := sprites[0]
	bestDiff := abs(best.Width - size)
	for _, s := range sprites[1:] {
		diff := abs(s.Width - size)
		if diff < bestDiff {
			best = s
			bestDiff = diff
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// BuiltinAtlas creates a fallback atlas with a simple generated cursor
func BuiltinAtlas(size int) *Atlas {
	atlas := NewAtlas(size)
	// Generate a simple arrow-like cursor sprite
	s := generateArrowCursor(size)
	atlas.Sprites["default"] = s
	atlas.Sprites["left_ptr"] = s
	atlas.Sprites["arrow"] = s
	atlas.Sprites["pointer"] = s
	return atlas
}

// ----------------------------------------------------------------------
// PNG loader
// ----------------------------------------------------------------------

func loadPNG(path string) (*Sprite, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	s := &Sprite{
		Width:   w,
		Height:  h,
		HotspotX: w / 4, // rough estimate
		HotspotY: h / 4,
		Pixels:  make([]byte, w*h*4),
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			off := (y*w + x) * 4
			s.Pixels[off] = byte(r >> 8)
			s.Pixels[off+1] = byte(g >> 8)
			s.Pixels[off+2] = byte(b >> 8)
			s.Pixels[off+3] = byte(a >> 8)
		}
	}

	return s, nil
}

// ----------------------------------------------------------------------
// Xcursor binary loader
// ----------------------------------------------------------------------

func loadXCursor(path string, preferredSize int) (*Sprite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) < 16 {
		return nil, fmt.Errorf("file too short")
	}

	// Magic: 0x0000fe00 (little-endian) for new format
	// X11 cursor files start with header, then toc, then images
	// Simplified: try to parse as Xcursor and extract the best-sized image

	magic := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
	if magic != 0x0000fe00 {
		return nil, fmt.Errorf("not an Xcursor file (magic=0x%08x)", magic)
	}

	// For now, try converting via xcur2png or external tool
	// If that fails, return an error
	return nil, fmt.Errorf("Xcursor binary format not yet implemented, try converting with xcur2png")
}

// ----------------------------------------------------------------------
// Built-in fallback generator
// ----------------------------------------------------------------------

func generateArrowCursor(size int) *Sprite {
	s := &Sprite{
		Width:   size,
		Height:  size,
		HotspotX: size / 8,
		HotspotY: size / 8,
		Pixels:  make([]byte, size*size*4),
	}

	// Simple arrow shape pointing top-left
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			off := (y*size + x) * 4
			// Arrow body along diagonal
			if x < y+size/4 && y < x+size/4 && x+y < size+size/8 {
				alpha := byte(255)
				if x+y > size-size/4 {
					alpha = byte(128) // tail fade
				}
				s.Pixels[off] = 255
				s.Pixels[off+1] = 255
				s.Pixels[off+2] = 255
				s.Pixels[off+3] = alpha
			} else {
				s.Pixels[off+3] = 0
			}
		}
	}

	return s
}

// ----------------------------------------------------------------------
// Theme discovery
// ----------------------------------------------------------------------

// FindTheme locates a cursor theme directory by name.
// It searches XCURSOR_PATH, ~/.icons, /usr/share/icons, etc.
func FindTheme(name string) (string, error) {
	// Search paths in order of priority
	paths := []string{}

	// XCURSOR_PATH
	if p := os.Getenv("XCURSOR_PATH"); p != "" {
		for _, dir := range splitPath(p) {
			paths = append(paths, filepath.Join(dir, name))
		}
	}

	// Standard locations
	home, _ := os.UserHomeDir()
	paths = append(paths,
		filepath.Join(home, ".icons", name),
		filepath.Join(home, ".local", "share", "icons", name),
		"/usr/share/icons/"+name,
		"/usr/local/share/icons/"+name,
	)

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, nil
		}
	}

	return "", fmt.Errorf("cursor theme '%s' not found", name)
}

func splitPath(p string) []string {
	return strings.Split(p, ":")
}

// ----------------------------------------------------------------------
// OpenGL helpers (used by backend renderers)
// ----------------------------------------------------------------------

// SpriteTexture uploads a sprite to an OpenGL texture.
// Returns the texture ID. Caller must delete the texture when done.
// This is a stub; actual implementation uses C.glGenTextures etc.
func SpriteTexture(s *Sprite) uint32 {
	_ = s
	// Actual GL code goes in the backend (wayland.go / x11.go)
	// to avoid CGO in this package.
	return 0
}
