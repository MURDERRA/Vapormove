#pragma once

#include <stdint.h>
#include <wayland-client.h>
#include <wayland-egl.h>
#include <EGL/egl.h>
#include <GL/gl.h>

// Generated from xdg-shell.xml and wlr-layer-shell-unstable-v1.xml
#include "xdg-shell-client-protocol.h"
#include "wlr-layer-shell-unstable-v1-client-protocol.h"
// Generated from pointer-constraints-unstable-v1 (for cursor pos)
// We use a simpler approach: read cursor pos via hyprctl socket

// ── Output (monitor) ─────────────────────────────────────────────────

typedef struct vm_output {
    struct wl_output        *wl_output;
    int32_t                  x, y;        // logical position
    int32_t                  width, height;
    int32_t                  scale;
    char                     name[64];
    struct vm_output        *next;
} vm_output_t;

// ── Overlay surface (one per output) ─────────────────────────────────

typedef struct vm_surface {
    struct wl_surface               *wl_surface;
    struct zwlr_layer_surface_v1    *layer_surface;
    struct wl_egl_window            *egl_window;
    EGLSurface                       egl_surface;
    vm_output_t                     *output;
    int32_t                          width, height;
    int                              configured; // 1 after first configure
    struct vm_surface               *next;
} vm_surface_t;

// ── Top-level context ─────────────────────────────────────────────────

typedef struct vm_wl_ctx {
    struct wl_display               *display;
    struct wl_registry              *registry;
    struct wl_compositor            *compositor;
    struct zwlr_layer_shell_v1      *layer_shell;

    EGLDisplay                       egl_display;
    EGLContext                       egl_context;
    EGLConfig                        egl_config;

    vm_output_t                     *outputs;   // linked list
    vm_surface_t                    *surfaces;  // linked list

    // Cursor position (updated by Go via hyprctl)
    double cursor_x, cursor_y;

    int running;
} vm_wl_ctx_t;

// ── API ───────────────────────────────────────────────────────────────

// Initialise wayland connection, bind globals, create EGL context
vm_wl_ctx_t* vm_wl_init(void);

// Create layer-shell overlay surfaces for all known outputs
int vm_wl_create_surfaces(vm_wl_ctx_t *ctx);

// Dispatch pending wayland events (non-blocking)
int vm_wl_dispatch(vm_wl_ctx_t *ctx);

// Make a surface's EGL context current for rendering
int vm_wl_make_current(vm_wl_ctx_t *ctx, vm_surface_t *surf);

// Swap buffers (present frame)
void vm_wl_swap(vm_wl_ctx_t *ctx, vm_surface_t *surf);

// Destroy everything
void vm_wl_destroy(vm_wl_ctx_t *ctx);
