//go:build linux

// Package system: native Linux monitor enumeration via GDK, reusing the
// gtk+-3.0 linkage already required by webview_go's GTK/WebKitGTK backend —
// no subprocess (`xrandr --listmonitors`) is spawned. GDK's monitor API is
// backend agnostic (X11 and Wayland both go through GdkDisplay).
package system

/*
#cgo pkg-config: gtk+-3.0
#include <gdk/gdk.h>

static int nexDisplayList(int *xs, int *ys, int *ws, int *hs, int *isPrimary, int maxCount) {
    GdkDisplay *display = gdk_display_get_default();
    if (display == NULL) {
        return 0;
    }
    int n = gdk_display_get_n_monitors(display);
    if (n > maxCount) {
        n = maxCount;
    }
    GdkMonitor *primary = gdk_display_get_primary_monitor(display);
    for (int i = 0; i < n; i++) {
        GdkMonitor *mon = gdk_display_get_monitor(display, i);
        GdkRectangle geo;
        gdk_monitor_get_geometry(mon, &geo);
        xs[i] = geo.x;
        ys[i] = geo.y;
        ws[i] = geo.width;
        hs[i] = geo.height;
        isPrimary[i] = (mon == primary) ? 1 : 0;
    }
    return n;
}
*/
import "C"

const maxNativeDisplays = 16

// nativeScreenInfo replaces `xrandr --listmonitors`.
func nativeScreenInfo() []map[string]any {
	var xs, ys, ws, hs, isPrimary [maxNativeDisplays]C.int
	n := int(C.nexDisplayList(&xs[0], &ys[0], &ws[0], &hs[0], &isPrimary[0], C.int(maxNativeDisplays)))
	out := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, map[string]any{
			"x":       int(xs[i]),
			"y":       int(ys[i]),
			"width":   int(ws[i]),
			"height":  int(hs[i]),
			"primary": isPrimary[i] != 0,
		})
	}
	return out
}
