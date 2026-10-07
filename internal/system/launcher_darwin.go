//go:build darwin

package system

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <stdlib.h>

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
