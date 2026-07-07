//go:build linux

// Package system: native Linux "open with default app" / "reveal in file
// manager" via GIO (part of GLib, already a transitive dependency of the
// gtk+-3.0 linkage webview_go requires) — no `xdg-open` subprocess is
// spawned. Reveal-in-folder uses the freedesktop.org FileManager1 D-Bus
// service (implemented by Nautilus/Dolphin/Nemo/etc.), which messages the
// already-running file manager instead of launching a new one; if that
// service isn't available, it falls back to opening the containing
// directory with the default handler, still via GIO (no subprocess either
// way).
package system

/*
#cgo pkg-config: gio-2.0
#include <gio/gio.h>
#include <stdlib.h>

static int nexOpenURI(const char *uri) {
    GError *error = NULL;
    gboolean ok = g_app_info_launch_default_for_uri(uri, NULL, &error);
    if (error != NULL) {
        g_error_free(error);
    }
    return ok ? 1 : 0;
}

static int nexShowItemInFolder(const char *path) {
    GError *error = NULL;
    GDBusProxy *proxy = g_dbus_proxy_new_for_bus_sync(
        G_BUS_TYPE_SESSION, G_DBUS_PROXY_FLAGS_NONE, NULL,
        "org.freedesktop.FileManager1", "/org/freedesktop/FileManager1",
        "org.freedesktop.FileManager1", NULL, &error);
    if (proxy == NULL) {
        if (error != NULL) g_error_free(error);
        return 0;
    }

    GFile *file = g_file_new_for_path(path);
    char *uri = g_file_get_uri(file);
    const char *uris[2] = { uri, NULL };

    GVariant *result = g_dbus_proxy_call_sync(
        proxy, "ShowItems",
        g_variant_new("(^ass)", uris, ""),
        G_DBUS_CALL_FLAGS_NONE, 5000, NULL, &error);

    g_free(uri);
    g_object_unref(file);
    g_object_unref(proxy);

    if (result == NULL) {
        if (error != NULL) g_error_free(error);
        return 0;
    }
    g_variant_unref(result);
    return 1;
}
*/
import "C"
import (
	"errors"
	"path/filepath"
	"unsafe"
)

// nativeOpenTarget replaces `xdg-open <target>` for both URLs and plain
// filesystem paths (a bare path is converted to a file:// URI first, since
// g_app_info_launch_default_for_uri requires a URI).
func nativeOpenTarget(target string) error {
	uri := target
	if !looksLikeURI(target) {
		abs, err := filepath.Abs(target)
		if err != nil {
			abs = target
		}
		uri = "file://" + abs
	}
	cURI := C.CString(uri)
	defer C.free(unsafe.Pointer(cURI))
	if C.nexOpenURI(cURI) == 0 {
		return errors.New("GIO could not open target")
	}
	return nil
}

// nativeRevealInFolder replaces `xdg-open <parent-dir>` (the previous
// best-effort fallback, since there's no `open -R` equivalent CLI on Linux):
// it now asks the running file manager to select the item via the
// freedesktop.org FileManager1 D-Bus service, falling back to opening the
// containing directory (still via GIO, no subprocess) if that service isn't
// registered on the session bus.
func nativeRevealInFolder(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	cPath := C.CString(abs)
	defer C.free(unsafe.Pointer(cPath))
	if C.nexShowItemInFolder(cPath) != 0 {
		return nil
	}
	return nativeOpenTarget(filepath.Dir(abs))
}

func looksLikeURI(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ':':
			return i > 0
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '+', c == '-', c == '.':
			continue
		default:
			return false
		}
	}
	return false
}
