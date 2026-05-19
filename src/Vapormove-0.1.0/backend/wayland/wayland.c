#include "wayland.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <EGL/eglext.h>

// ── Layer surface callbacks ───────────────────────────────────────────

static void layer_surface_configure(
    void *data,
    struct zwlr_layer_surface_v1 *layer_surface,
    uint32_t serial,
    uint32_t width,
    uint32_t height)
{
    vm_surface_t *surf = (vm_surface_t *)data;

    // Resize EGL window if dimensions changed
    if (surf->egl_window && (surf->width != (int32_t)width || surf->height != (int32_t)height)) {
        wl_egl_window_resize(surf->egl_window, width, height, 0, 0);
    }
    surf->width  = width;
    surf->height = height;
    surf->configured = 1;

    zwlr_layer_surface_v1_ack_configure(layer_surface, serial);
    wl_surface_commit(surf->wl_surface);
}

static void layer_surface_closed(
    void *data,
    struct zwlr_layer_surface_v1 *layer_surface)
{
    vm_surface_t *surf = (vm_surface_t *)data;
    // Output was removed; mark for cleanup
    surf->configured = -1;
}

static const struct zwlr_layer_surface_v1_listener layer_surface_listener = {
    .configure = layer_surface_configure,
    .closed    = layer_surface_closed,
};

// ── Output callbacks ──────────────────────────────────────────────────

static void output_geometry(void *data, struct wl_output *wl_output,
    int32_t x, int32_t y, int32_t phys_w, int32_t phys_h,
    int32_t subpixel, const char *make, const char *model, int32_t transform)
{
    vm_output_t *out = (vm_output_t *)data;
    out->x = x;
    out->y = y;
}

static void output_mode(void *data, struct wl_output *wl_output,
    uint32_t flags, int32_t width, int32_t height, int32_t refresh)
{
    if (flags & WL_OUTPUT_MODE_CURRENT) {
        vm_output_t *out = (vm_output_t *)data;
        out->width  = width;
        out->height = height;
    }
}

static void output_scale(void *data, struct wl_output *wl_output, int32_t scale) {
    vm_output_t *out = (vm_output_t *)data;
    out->scale = scale;
}

static void output_name(void *data, struct wl_output *wl_output, const char *name) {
    vm_output_t *out = (vm_output_t *)data;
    if (name) strncpy(out->name, name, sizeof(out->name) - 1);
}

static void output_description(void *data, struct wl_output *wl_output, const char *desc) {}
static void output_done(void *data, struct wl_output *wl_output) {}

static const struct wl_output_listener output_listener = {
    .geometry    = output_geometry,
    .mode        = output_mode,
    .scale       = output_scale,
    .name        = output_name,
    .description = output_description,
    .done        = output_done,
};

// ── Registry callbacks ────────────────────────────────────────────────

static void registry_global(void *data, struct wl_registry *registry,
    uint32_t name, const char *interface, uint32_t version)
{
    vm_wl_ctx_t *ctx = (vm_wl_ctx_t *)data;

    if (strcmp(interface, wl_compositor_interface.name) == 0) {
        ctx->compositor = wl_registry_bind(registry, name,
            &wl_compositor_interface, 4);
    } else if (strcmp(interface, zwlr_layer_shell_v1_interface.name) == 0) {
        ctx->layer_shell = wl_registry_bind(registry, name,
            &zwlr_layer_shell_v1_interface, 1);
    } else if (strcmp(interface, wl_output_interface.name) == 0) {
        vm_output_t *out = calloc(1, sizeof(vm_output_t));
        out->scale = 1;
        out->wl_output = wl_registry_bind(registry, name,
            &wl_output_interface, 4);
        wl_output_add_listener(out->wl_output, &output_listener, out);
        out->next = ctx->outputs;
        ctx->outputs = out;
    }
}

static void registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {}

static const struct wl_registry_listener registry_listener = {
    .global        = registry_global,
    .global_remove = registry_global_remove,
};

// ── EGL initialisation ────────────────────────────────────────────────

static int init_egl(vm_wl_ctx_t *ctx) {
    ctx->egl_display = eglGetDisplay((EGLNativeDisplayType)ctx->display);
    if (ctx->egl_display == EGL_NO_DISPLAY) {
        fprintf(stderr, "vapormove: eglGetDisplay failed\n");
        return 0;
    }

    EGLint major, minor;
    if (!eglInitialize(ctx->egl_display, &major, &minor)) {
        fprintf(stderr, "vapormove: eglInitialize failed\n");
        return 0;
    }

    eglBindAPI(EGL_OPENGL_API);

    EGLint attribs[] = {
        EGL_SURFACE_TYPE,    EGL_WINDOW_BIT,
        EGL_RENDERABLE_TYPE, EGL_OPENGL_BIT,
        EGL_RED_SIZE,        8,
        EGL_GREEN_SIZE,      8,
        EGL_BLUE_SIZE,       8,
        EGL_ALPHA_SIZE,      8,
        EGL_NONE,
    };

    EGLint num_configs;
    if (!eglChooseConfig(ctx->egl_display, attribs, &ctx->egl_config, 1, &num_configs)
        || num_configs == 0) {
        fprintf(stderr, "vapormove: eglChooseConfig failed\n");
        return 0;
    }

    EGLint ctx_attribs[] = { EGL_NONE };
    ctx->egl_context = eglCreateContext(ctx->egl_display, ctx->egl_config,
        EGL_NO_CONTEXT, ctx_attribs);
    if (ctx->egl_context == EGL_NO_CONTEXT) {
        fprintf(stderr, "vapormove: eglCreateContext failed\n");
        return 0;
    }

    return 1;
}

// ── Public API ────────────────────────────────────────────────────────

vm_wl_ctx_t* vm_wl_init(void) {
    vm_wl_ctx_t *ctx = calloc(1, sizeof(vm_wl_ctx_t));

    ctx->display = wl_display_connect(NULL);
    if (!ctx->display) {
        fprintf(stderr, "vapormove: cannot connect to Wayland display\n");
        free(ctx);
        return NULL;
    }

    ctx->registry = wl_display_get_registry(ctx->display);
    wl_registry_add_listener(ctx->registry, &registry_listener, ctx);
    wl_display_roundtrip(ctx->display); // bind globals
    wl_display_roundtrip(ctx->display); // receive output events

    if (!ctx->compositor || !ctx->layer_shell) {
        fprintf(stderr, "vapormove: compositor or layer-shell not available\n");
        wl_display_disconnect(ctx->display);
        free(ctx);
        return NULL;
    }

    if (!init_egl(ctx)) {
        wl_display_disconnect(ctx->display);
        free(ctx);
        return NULL;
    }

    ctx->running = 1;
    return ctx;
}

int vm_wl_create_surfaces(vm_wl_ctx_t *ctx) {
    int count = 0;
    for (vm_output_t *out = ctx->outputs; out; out = out->next) {
        vm_surface_t *surf = calloc(1, sizeof(vm_surface_t));
        surf->output = out;

        surf->wl_surface = wl_compositor_create_surface(ctx->compositor);
	// Set empty input region for cursor pass-through
	struct wl_region *empty_region = wl_compositor_create_region(ctx->compositor);
	wl_surface_set_input_region(surf->wl_surface, empty_region);
	wl_region_destroy(empty_region);

        surf->layer_surface = zwlr_layer_shell_v1_get_layer_surface(
            ctx->layer_shell,
            surf->wl_surface,
            out->wl_output,
            ZWLR_LAYER_SHELL_V1_LAYER_BOTTOM,
            "vapormove"
        );

        // Anchor to all edges (fullscreen), exclusive zone 0 (no space reserved)
        zwlr_layer_surface_v1_set_anchor(surf->layer_surface,
            ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP |
            ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM |
            ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT |
            ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT);
        zwlr_layer_surface_v1_set_exclusive_zone(surf->layer_surface, -1);
        zwlr_layer_surface_v1_set_keyboard_interactivity(surf->layer_surface,
            ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_NONE);

        // Size 0,0 means "use the output size"
        zwlr_layer_surface_v1_set_size(surf->layer_surface, 0, 0);

        zwlr_layer_surface_v1_add_listener(surf->layer_surface,
            &layer_surface_listener, surf);

        wl_surface_commit(surf->wl_surface);
        wl_display_roundtrip(ctx->display);

        // Create EGL window after configure
        if (surf->configured > 0) {
            surf->egl_window = wl_egl_window_create(surf->wl_surface,
                surf->width, surf->height);
            surf->egl_surface = eglCreateWindowSurface(ctx->egl_display,
                ctx->egl_config,
                (EGLNativeWindowType)surf->egl_window,
                NULL);
        }

        surf->next    = ctx->surfaces;
        ctx->surfaces = surf;
        count++;
    }
    return count;
}

int vm_wl_dispatch(vm_wl_ctx_t *ctx) {
    return wl_display_dispatch_pending(ctx->display);
}

int vm_wl_make_current(vm_wl_ctx_t *ctx, vm_surface_t *surf) {
    return eglMakeCurrent(ctx->egl_display, surf->egl_surface,
        surf->egl_surface, ctx->egl_context);
}

void vm_wl_swap(vm_wl_ctx_t *ctx, vm_surface_t *surf) {
    eglSwapBuffers(ctx->egl_display, surf->egl_surface);
}

void vm_wl_destroy(vm_wl_ctx_t *ctx) {
    if (!ctx) return;

    vm_surface_t *surf = ctx->surfaces;
    while (surf) {
        vm_surface_t *next = surf->next;
        if (surf->egl_surface != EGL_NO_SURFACE)
            eglDestroySurface(ctx->egl_display, surf->egl_surface);
        if (surf->egl_window)
            wl_egl_window_destroy(surf->egl_window);
        if (surf->layer_surface)
            zwlr_layer_surface_v1_destroy(surf->layer_surface);
        if (surf->wl_surface)
            wl_surface_destroy(surf->wl_surface);
        free(surf);
        surf = next;
    }

    vm_output_t *out = ctx->outputs;
    while (out) {
        vm_output_t *next = out->next;
        wl_output_destroy(out->wl_output);
        free(out);
        out = next;
    }

    eglDestroyContext(ctx->egl_display, ctx->egl_context);
    eglTerminate(ctx->egl_display);
    wl_registry_destroy(ctx->registry);
    wl_display_disconnect(ctx->display);
    free(ctx);
}
