//go:build darwin

// Package system: native macOS "open with default app" / "reveal in Finder"
// via NSWorkspace (Cocoa) — no `open`/`open -R` subprocess is spawned.
package system

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <stdlib.h>

// nexOpenTarget opens a URL (any scheme) or a plain filesystem path with the
// user's default handler application, via NSWorkspace — equivalent to the
// `open` CLI, but without spawning it.
static int nexOpenTarget(const char *target) {
    @autoreleasepool {
        NSString *str = [NSString stringWithUTF8String:target];
        if (str == nil) return 0;
        NSURL *url = [NSURL URLWithString:str];
        if (url == nil || url.scheme == nil) {
            url = [NSURL fileURLWithPath:str];
        }
        return [[NSWorkspace sharedWorkspace] openURL:url] ? 1 : 0;
    }
}

// nexRevealInFinder selects a file/folder in Finder — equivalent to
// `open -R <path>`, but without spawning it.
static int nexRevealInFinder(const char *path) {
    @autoreleasepool {
        NSString *str = [NSString stringWithUTF8String:path];
        if (str == nil) return 0;
        return [[NSWorkspace sharedWorkspace] selectFile:str inFileViewerRootedAtPath:@""] ? 1 : 0;
    }
}
*/
import "C"
import (
	"errors"
	"unsafe"
)

func nativeOpenTarget(target string) error {
	cTarget := C.CString(target)
	defer C.free(unsafe.Pointer(cTarget))
	if C.nexOpenTarget(cTarget) == 0 {
		return errors.New("NSWorkspace could not open target")
	}
	return nil
}

func nativeRevealInFolder(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	if C.nexRevealInFinder(cPath) == 0 {
		return errors.New("NSWorkspace could not reveal path in Finder")
	}
	return nil
}
