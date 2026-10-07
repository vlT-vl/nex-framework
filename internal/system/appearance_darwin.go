//go:build darwin

package system

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>

static char *nexAppearanceStyle(void) {
    @autoreleasepool {
        NSString *style = [[NSUserDefaults standardUserDefaults] stringForKey:@"AppleInterfaceStyle"];
        if (style == nil) {
            return NULL;
        }
        const char *utf8 = [style UTF8String];
        if (utf8 == NULL) {
            return NULL;
        }
        return strdup(utf8);
    }
}
*/
import "C"
import (
	"strings"
	"unsafe"
)

func nativeAppearanceInfo() map[string]any {
	const source = "NSUserDefaults AppleInterfaceStyle"
	cstr := C.nexAppearanceStyle()
	if cstr == nil {
		return map[string]any{"theme": "light", "source": source}
	}
	defer C.free(unsafe.Pointer(cstr))
	style := C.GoString(cstr)
	if strings.EqualFold(strings.TrimSpace(style), "Dark") {
		return map[string]any{"theme": "dark", "source": source}
	}
	return map[string]any{"theme": "light", "source": source}
}
