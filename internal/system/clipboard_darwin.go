//go:build darwin

// Package system: native macOS clipboard via NSPasteboard (Cocoa), linked
// with the same framework webview_go already requires on darwin — no shell
// subprocess (pbcopy/pbpaste) is spawned.
package system

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>

static char *nexClipboardReadText(void) {
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        NSString *str = [pb stringForType:NSPasteboardTypeString];
        if (str == nil) {
            return strdup("");
        }
        const char *utf8 = [str UTF8String];
        if (utf8 == NULL) {
            return strdup("");
        }
        return strdup(utf8);
    }
}

static int nexClipboardWriteText(const char *text) {
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        NSString *str = [NSString stringWithUTF8String:text];
        if (str == nil) {
            return 0;
        }
        BOOL ok = [pb setString:str forType:NSPasteboardTypeString];
        return ok ? 1 : 0;
    }
}
*/
import "C"
import "unsafe"

func nativeClipboardReadText() (string, bool, string, error) {
	cstr := C.nexClipboardReadText()
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr), true, "NSPasteboard", nil
}

func nativeClipboardWriteText(text string) (bool, string, error) {
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	ok := C.nexClipboardWriteText(ctext)
	return ok != 0, "NSPasteboard", nil
}

func nativeClipboardAvailable() (bool, string) {
	return true, "NSPasteboard"
}
