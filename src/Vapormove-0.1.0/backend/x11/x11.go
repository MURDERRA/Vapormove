package x11

/*
#cgo pkg-config: x11 xcomposite xfixes gl
#cgo LDFLAGS: -lX11 -lXcomposite -lXfixes -lXext -lGL -lm

#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/extensions/Xcomposite.h>
#include <X11/extensions/shape.h>
#include <GL/glx.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

typedef struct {
	Display *dpy;
	int screen;
	Window root;
	Window overlay;
	GLXContext glx_ctx;
	int width, height;
	int x, y; // screen origin (for multi-monitor TODO)
} vm_x11_t;

static vm_x11_t* vm_x11_init(void) {
	vm_x11_t *ctx = calloc(1, sizeof(vm_x11_t));
	ctx->dpy = XOpenDisplay(NULL);
	if (!ctx->dpy) {
		fprintf(stderr, "vapormove: cannot open X display\n");
		free(ctx);
		return NULL;
	}
	ctx->screen = DefaultScreen(ctx->dpy);
	ctx->root = RootWindow(ctx->dpy, ctx->screen);
	ctx->width = DisplayWidth(ctx->dpy, ctx->screen);
	ctx->height = DisplayHeight(ctx->dpy, ctx->screen);

	// Create fullscreen transparent overlay window
	XSetWindowAttributes wa;
	wa.override_redirect = True;
	wa.colormap = XCreateColormap(ctx->dpy, ctx->root,
		DefaultVisual(ctx->dpy, ctx->screen), AllocNone);
	wa.background_pixel = 0;
	wa.border_pixel = 0;

	int vi_attribs[] = {
		GLX_RGBA, GLX_DOUBLEBUFFER,
		GLX_RED_SIZE, 8, GLX_GREEN_SIZE, 8,
		GLX_BLUE_SIZE, 8, GLX_ALPHA_SIZE, 8,
		GLX_DEPTH_SIZE, 0,
		None
	};
	XVisualInfo *vi = glXChooseVisual(ctx->dpy, ctx->screen, vi_attribs);
	if (!vi) {
		fprintf(stderr, "vapormove: glXChooseVisual failed\n");
		free(ctx);
		return NULL;
	}

	wa.colormap = XCreateColormap(ctx->dpy, ctx->root, vi->visual, AllocNone);

	ctx->overlay = XCreateWindow(ctx->dpy, ctx->root,
		0, 0, ctx->width, ctx->height, 0,
		vi->depth, InputOutput, vi->visual,
		CWOverrideRedirect | CWColormap | CWBackPixel | CWBorderPixel, &wa);

	// Set window type to dock so it appears above everything
	Atom type = XInternAtom(ctx->dpy, "_NET_WM_WINDOW_TYPE", False);
	Atom dock = XInternAtom(ctx->dpy, "_NET_WM_WINDOW_TYPE_DOCK", False);
	XChangeProperty(ctx->dpy, ctx->overlay, type, XA_ATOM, 32,
		PropModeReplace, (unsigned char *)&dock, 1);

	// Always on top
	Atom state = XInternAtom(ctx->dpy, "_NET_WM_STATE", False);
	Atom on_top = XInternAtom(ctx->dpy, "_NET_WM_STATE_ABOVE", False);
	XChangeProperty(ctx->dpy, ctx->overlay, state, XA_ATOM, 32,
		PropModeReplace, (unsigned char *)&on_top, 1);

	// Make input pass-through via XShape (empty input region)
	XRectangle rect = {0};
	XShapeCombineRectangles(ctx->dpy, ctx->overlay, ShapeInput,
		0, 0, &rect, 0, ShapeSet, Unsorted);

	// Create GLX context
	ctx->glx_ctx = glXCreateContext(ctx->dpy, vi, NULL, True);
	XFree(vi);
	if (!ctx->glx_ctx) {
		fprintf(stderr, "vapormove: glXCreateContext failed\n");
		XDestroyWindow(ctx->dpy, ctx->overlay);
		free(ctx);
		return NULL;
	}

	XMapRaised(ctx->dpy, ctx->overlay);
	glXMakeCurrent(ctx->dpy, ctx->overlay, ctx->glx_ctx);
	XFlush(ctx->dpy);

	return ctx;
}

static void vm_x11_get_cursor(vm_x11_t *ctx, int *x, int *y) {
	Window root_ret, child_ret;
	int root_x, root_y, win_x, win_y;
	unsigned int mask;
	XQueryPointer(ctx->dpy, ctx->root, &root_ret, &child_ret,
		&root_x, &root_y, &win_x, &win_y, &mask);
	*x = root_x;
	*y = root_y;
}

static void vm_x11_swap(vm_x11_t *ctx) {
	glXSwapBuffers(ctx->dpy, ctx->overlay);
}

static void vm_x11_destroy(vm_x11_t *ctx) {
	if (!ctx) return;
	glXDestroyContext(ctx->dpy, ctx->glx_ctx);
	XDestroyWindow(ctx->dpy, ctx->overlay);
	XCloseDisplay(ctx->dpy);
	free(ctx);
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"time"

	"vapormove/config"
	"vapormove/trail"
)

// Backend is the X11 implementation
type Backend struct {
	cfg         *config.Config
	ctx         *C.vm_x11_t
	cursorTrail *trail.Trail
	windowTrail *trail.Trail
	stopCh      chan struct{}
	// Last window position and size for trail effect
	lastX, lastY int
	lastW, lastH int
	// Track if Close was already called
	closed bool
}

// New creates an X11 backend
func New(cfg *config.Config) (*Backend, error) {
	runtime.LockOSThread()

	ctx := C.vm_x11_init()
	if ctx == nil {
		return nil, fmt.Errorf("x11: failed to initialise display")
	}

	return &Backend{
		cfg:         cfg,
		ctx:         ctx,
		cursorTrail: trail.New(cfg.Cursor),
		windowTrail: trail.New(cfg.Window),
		stopCh:      make(chan struct{}),
	}, nil
}

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
			b.render()
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
		C.vm_x11_destroy(b.ctx)
	}
}

// ── Cursor polling (X11 native — no hyprctl needed) ───────────────────

func (b *Backend) pollCursor() {
	var lastX, lastY C.int = -99999, -99999

	for {
		select {
		case <-b.stopCh:
			return
		default:
		}

		var cx, cy C.int
		C.vm_x11_get_cursor(b.ctx, &cx, &cy)

		dx := cx - lastX
		dy := cy - lastY
		if lastX != -99999 && (dx > 0 || dy > 0 || dx < 0 || dy < 0) {
			b.cursorTrail.Push(float32(cx), float32(cy))
		}
		lastX, lastY = cx, cy
		time.Sleep(4 * time.Millisecond) // ~250Hz is plenty
	}
}

// ── Window polling (xdotool / _NET_ACTIVE_WINDOW) ─────────────────────

type hyprWindow struct {
	At   [2]int `json:"at"`
	Size [2]int `json:"size"`
}

func (b *Backend) pollWindow() {
	// On X11 we use xdotool to get active window geometry
	b.lastX, b.lastY = -99999, -99999
	b.lastW, b.lastH = 0, 0

	for {
		select {
		case <-b.stopCh:
			return
		default:
		}

		// Get window geometry including size
		out, err := exec.Command("xdotool", "getactivewindow", "getwindowgeometry", "--shell").Output()
		if err == nil {
			x, y, w, h := parseXdotoolGeometryWithSize(out)
			if x != 0 || y != 0 {
				// Check if window changed (new window or first detection)
				windowChanged := b.lastW == 0 || w != b.lastW || h != b.lastH

				dx := x - b.lastX
				dy := y - b.lastY

				// Spawn trail at PREVIOUS position (lastX, lastY)
				if b.lastX != -99999 && (abs(dx) > 2 || abs(dy) > 2) && !windowChanged {
					b.windowTrail.PushWithSize(float32(b.lastX), float32(b.lastY), float32(b.lastW), float32(b.lastH))
				}

				// Update last position AFTER spawning
				b.lastX, b.lastY = x, y
				b.lastW, b.lastH = w, h
			}
		} else {
			// No active window - reset
			b.lastX, b.lastY = -99999, -99999
			b.lastW, b.lastH = 0, 0
		}
		time.Sleep(8 * time.Millisecond)
	}
}

// parseXdotoolGeometryWithSize parses "X=123\nY=456\nWIDTH=800\nHEIGHT=600\n..." output
func parseXdotoolGeometryWithSize(out []byte) (x, y, w, h int) {
	for _, line := range splitLines(string(out)) {
		var v int
		if _, err := fmt.Sscanf(line, "X=%d", &v); err == nil {
			x = v
		}
		if _, err := fmt.Sscanf(line, "Y=%d", &v); err == nil {
			y = v
		}
		if _, err := fmt.Sscanf(line, "WIDTH=%d", &v); err == nil {
			w = v
		}
		if _, err := fmt.Sscanf(line, "HEIGHT=%d", &v); err == nil {
			h = v
		}
	}
	return
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return lines
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ── Rendering (OpenGL via GLX) ────────────────────────────────────────

func (b *Backend) render() {
	cursorPoints := b.cursorTrail.Snapshot()
	windowPoints := b.windowTrail.Snapshot()

	w := float64(b.ctx.width)
	h := float64(b.ctx.height)

	C.glViewport(0, 0, C.GLsizei(b.ctx.width), C.GLsizei(b.ctx.height))
	C.glClearColor(0, 0, 0, 0)
	C.glClear(C.GL_COLOR_BUFFER_BIT)
	C.glEnable(C.GL_BLEND)
	C.glBlendFunc(C.GL_SRC_ALPHA, C.GL_ONE_MINUS_SRC_ALPHA)

	C.glMatrixMode(C.GL_PROJECTION)
	C.glLoadIdentity()
	C.glOrtho(0, C.GLdouble(w), C.GLdouble(h), 0, -1, 1)
	C.glMatrixMode(C.GL_MODELVIEW)
	C.glLoadIdentity()

	renderWindowTrail(windowPoints)
	renderCursorTrail(cursorPoints, b.cfg.Cursor.PointSize)

	C.vm_x11_swap(b.ctx)
}

func renderCursorTrail(points []trail.GhostPoint, radius float32) {
	if radius <= 0 {
		radius = 6
	}
	segments := 16
	for _, p := range points {
		C.glColor4f(C.GLfloat(p.R), C.GLfloat(p.G), C.GLfloat(p.B), C.GLfloat(p.A))
		C.glBegin(C.GL_TRIANGLE_FAN)
		C.glVertex2f(C.GLfloat(p.X), C.GLfloat(p.Y))
		for i := 0; i <= segments; i++ {
			angle := float64(i) / float64(segments) * 2 * math.Pi
			C.glVertex2f(
				C.GLfloat(p.X+float32(math.Cos(angle))*radius),
				C.GLfloat(p.Y+float32(math.Sin(angle))*radius),
			)
		}
		C.glEnd()
	}
}

func renderWindowTrail(points []trail.GhostPoint) {
	for _, p := range points {
		ww := p.W
		wh := p.H
		C.glColor4f(C.GLfloat(p.R), C.GLfloat(p.G), C.GLfloat(p.B), C.GLfloat(p.A))
		C.glBegin(C.GL_QUADS)
		C.glVertex2f(C.GLfloat(p.X), C.GLfloat(p.Y))
		C.glVertex2f(C.GLfloat(p.X+ww), C.GLfloat(p.Y))
		C.glVertex2f(C.GLfloat(p.X+ww), C.GLfloat(p.Y+wh))
		C.glVertex2f(C.GLfloat(p.X), C.GLfloat(p.Y+wh))
		C.glEnd()
	}
}

// suppress unused import
var _ = json.Unmarshal
var _ = exec.Command
