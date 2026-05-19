package wayland

/*
#cgo pkg-config: wayland-client wayland-egl egl gl
#cgo CFLAGS: -I.
#cgo LDFLAGS: -lwayland-client -lwayland-egl -lEGL -lGL -lm

#include "wayland.h"
#include <stdlib.h>
#include <GL/gl.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"time"
	"unsafe"

	"vapormove/config"
	"vapormove/trail"
)

// Backend is the Wayland implementation
type Backend struct {
	closed bool
	cfg *config.Config
	ctx *C.vm_wl_ctx_t
	cursorTrail *trail.Trail
	windowTrail *trail.Trail
	stopCh chan struct{}

	// Previous window position for trail effect (in global coordinates)
	lastX, lastY int
	lastW, lastH int

	// Monitor cache for coordinate translation
	monitorCache map[int]*MonitorInfo
}

// MonitorInfo holds Hyprland monitor position
type MonitorInfo struct {
	ID   int
	Name string
	X, Y int
}

// New creates a Wayland backend� Must be called from the main OS thread.
func New(cfg *config.Config) (*Backend, error) {
	runtime.LockOSThread()
	ctx := C.vm_wl_init()
	if ctx == nil {
		return nil, fmt.Errorf("wayland: failed to initialise display")
	}

	n := C.vm_wl_create_surfaces(ctx)
	if n == 0 {
		C.vm_wl_destroy(ctx)
		return nil, fmt.Errorf("wayland: no outputs found")
	}

	return &Backend{
		cfg: cfg,
		ctx: ctx,
		cursorTrail: trail.New(cfg.Cursor),
		windowTrail: trail.New(cfg.Window),
		stopCh: make(chan struct{}),
		monitorCache: make(map[int]*MonitorInfo),
	}, nil
}

// Run starts the render loop. Blocks until Close() is called.
func (b *Backend) Run() error {
	go b.pollCursor()
	go b.pollWindow()

	ticker := time.NewTicker(time.Second / 120)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return nil
		case <-ticker.C:
			C.vm_wl_dispatch(b.ctx)
			b.renderAll()
		}
	}
}

func (b *Backend) Reconfigure(cfg *config.Config) error {
	b.cfg = cfg
	b.cursorTrail.UpdateConfig(cfg.Cursor)
	b.windowTrail.UpdateConfig(cfg.Window)
	return nil
}

func (b *Backend) Close() {
	if !b.closed {
		b.closed = true
		close(b.stopCh)
		C.vm_wl_destroy(b.ctx)
	}
}

type cursorPos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (b *Backend) pollCursor() {
	var lastX, lastY float64 = -99999, -99999

	for {
		select {
		case <-b.stopCh:
			return
		default:
		}

		out, err := exec.Command("hyprctl", "cursorpos", "-j").Output()
		if err == nil {
			var pos cursorPos
			if json.Unmarshal(out, &pos) == nil {
				dx := pos.X - lastX
				dy := pos.Y - lastY
				if lastX != -99999 && (math.Abs(dx) > 0.5 || math.Abs(dy) > 0.5) {
					b.cursorTrail.Push(float32(pos.X), float32(pos.Y))
				}
				lastX, lastY = pos.X, pos.Y
			}
		}
	}
}

type hyprWindow struct {
	At      [2]int `json:"at"`
	Size    [2]int `json:"size"`
	Monitor int    `json:"monitor"`
}

type hyprMonitor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
}

func (b *Backend) updateMonitorCache() {
	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err != nil {
		return
	}

	var monitors []hyprMonitor
	if json.Unmarshal(out, &monitors) != nil {
		return
	}

	b.monitorCache = make(map[int]*MonitorInfo)
	for _, m := range monitors {
		b.monitorCache[m.ID] = &MonitorInfo{ID: m.ID, Name: m.Name, X: m.X, Y: m.Y}
	}
}

func (b *Backend) getOutputPos(surf *C.vm_surface_t) (float32, float32) {
	name := C.GoString(&surf.output.name[0])
	for _, mon := range b.monitorCache {
		if mon.Name == name {
			return float32(mon.X), float32(mon.Y)
		}
	}
	return float32(surf.output.x), float32(surf.output.y)
}

// getGlobalWindowCoords returns window coordinates in global desktop space.
// Note: hyprctl already returns global coordinates in the 'at' field.
func (b *Backend) getGlobalWindowCoords(monID int, localX, localY int) (int, int) {
	return localX, localY
}

func (b *Backend) pollWindow() {
	b.lastX, b.lastY = -99999, -99999
	b.lastW, b.lastH = 0, 0

	b.updateMonitorCache()

	for {
		select {
		case <-b.stopCh:
			return
		default:
		}

		out, err := exec.Command("hyprctl", "activewindow", "-j").Output()
		if err == nil {
			var win hyprWindow
			if json.Unmarshal(out, &win) == nil && win.Size[0] > 0 {
				localX, localY := win.At[0], win.At[1]
				currW, currH := win.Size[0], win.Size[1]
				currX, currY := b.getGlobalWindowCoords(win.Monitor, localX, localY)

				windowChanged := b.lastW == 0 || currW != b.lastW || currH != b.lastH
				dx := currX - b.lastX
				dy := currY - b.lastY

				if b.lastX != -99999 && (abs(dx) > 2 || abs(dy) > 2) && !windowChanged {
					b.windowTrail.PushWithSize(float32(b.lastX), float32(b.lastY), float32(b.lastW), float32(b.lastH))
				}

				b.lastX, b.lastY = currX, currY
				b.lastW, b.lastH = currW, currH
			} else {
				b.lastX, b.lastY = -99999, -99999
				b.lastW, b.lastH = 0, 0
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (b *Backend) renderAll() {
	cursorPoints := b.cursorTrail.Snapshot()
	windowPoints := b.windowTrail.Snapshot()

	surf := b.ctx.surfaces
	for surf != nil {
		if surf.configured != 1 {
			surf = surf.next
			continue
		}

		C.vm_wl_make_current(b.ctx, surf)
		w := float32(surf.width)
		h := float32(surf.height)
		ox, oy := b.getOutputPos(surf)

		C.glViewport(0, 0, C.GLsizei(surf.width), C.GLsizei(surf.height))
		C.glClearColor(0, 0, 0, 0)
		C.glClear(C.GL_COLOR_BUFFER_BIT)
		C.glEnable(C.GL_BLEND)
		C.glBlendFunc(C.GL_SRC_ALPHA, C.GL_ONE_MINUS_SRC_ALPHA)

		C.glMatrixMode(C.GL_PROJECTION)
		C.glLoadIdentity()
		C.glOrtho(0, C.GLdouble(w), C.GLdouble(h), 0, -1, 1)
		C.glMatrixMode(C.GL_MODELVIEW)
		C.glLoadIdentity()

		renderWindowTrail(windowPoints, ox, oy, w, h, b.cfg.Window)
		renderCursorTrail(cursorPoints, ox, oy, w, h, b.cfg.Cursor)

		C.vm_wl_swap(b.ctx, surf)
		surf = surf.next
	}
}

func renderCursorTrail(points []trail.GhostPoint, ox, oy, sw, sh float32, cfg config.TrailConfig) {
	radius := cfg.PointSize
	if radius <= 0 {
		radius = 6
	}
	segments := 16

	for _, p := range points {
		lx := p.X - ox
		ly := p.Y - oy
		if lx < -radius || lx > sw+radius || ly < -radius || ly > sh+radius {
			continue
		}

		C.glColor4f(C.GLfloat(p.R), C.GLfloat(p.G), C.GLfloat(p.B), C.GLfloat(p.A))
		C.glBegin(C.GL_TRIANGLE_FAN)
		C.glVertex2f(C.GLfloat(lx), C.GLfloat(ly))
		for i := 0; i <= segments; i++ {
			angle := float64(i) / float64(segments) * 2 * math.Pi
			C.glVertex2f(
				C.GLfloat(lx+float32(math.Cos(angle))*radius),
				C.GLfloat(ly+float32(math.Sin(angle))*radius),
			)
		}
		C.glEnd()
	}
}

func renderWindowTrail(points []trail.GhostPoint, ox, oy, sw, sh float32, cfg config.TrailConfig) {
	for _, p := range points {
		lx := p.X - ox
		ly := p.Y - oy
		ww := p.W
		wh := p.H

		if lx+ww < 0 || lx > sw || ly+wh < 0 || ly > sh {
			continue
		}

		C.glColor4f(C.GLfloat(p.R), C.GLfloat(p.G), C.GLfloat(p.B), C.GLfloat(p.A))
		C.glBegin(C.GL_QUADS)
		C.glVertex2f(C.GLfloat(lx), C.GLfloat(ly))
		C.glVertex2f(C.GLfloat(lx+ww), C.GLfloat(ly))
		C.glVertex2f(C.GLfloat(lx+ww), C.GLfloat(ly+wh))
		C.glVertex2f(C.GLfloat(lx), C.GLfloat(ly+wh))
		C.glEnd()
	}
}

var _ = unsafe.Pointer(nil)
