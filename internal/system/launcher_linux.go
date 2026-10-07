//go:build linux

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
