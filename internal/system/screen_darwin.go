//go:build darwin

package system

/*
#cgo LDFLAGS: -framework CoreGraphics

#include <CoreGraphics/CoreGraphics.h>

// Enumerates active displays the same way `system_profiler SPDisplaysDataType`
// does, directly via CoreGraphics — no subprocess spawned.
static int nexDisplayList(int *xs, int *ys, int *ws, int *hs, int *isMain, int maxCount) {
    CGDirectDisplayID displays[32];
    uint32_t count = 0;
    if (CGGetActiveDisplayList(32, displays, &count) != kCGErrorSuccess) {
        return 0;
    }
    if ((int)count > maxCount) {
        count = (uint32_t)maxCount;
    }
    for (uint32_t i = 0; i < count; i++) {
        CGRect bounds = CGDisplayBounds(displays[i]);
        xs[i] = (int)bounds.origin.x;
        ys[i] = (int)bounds.origin.y;
        ws[i] = (int)bounds.size.width;
        hs[i] = (int)bounds.size.height;
        isMain[i] = CGDisplayIsMain(displays[i]) ? 1 : 0;
    }
    return (int)count;
}
*/
import "C"

const maxNativeDisplays = 16

// nativeScreenInfo replaces `system_profiler SPDisplaysDataType`.
func nativeScreenInfo() []map[string]any {
	var xs, ys, ws, hs, isMain [maxNativeDisplays]C.int
	n := int(C.nexDisplayList(&xs[0], &ys[0], &ws[0], &hs[0], &isMain[0], C.int(maxNativeDisplays)))
	out := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, map[string]any{
			"x":       int(xs[i]),
			"y":       int(ys[i]),
			"width":   int(ws[i]),
			"height":  int(hs[i]),
			"primary": isMain[i] != 0,
		})
	}
	return out
}
