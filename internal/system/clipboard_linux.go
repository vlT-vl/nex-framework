//go:build linux

package system

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <stdlib.h>
#include <string.h>

static char *nexClipboardReadText(void) {
    GtkClipboard *cb = gtk_clipboard_get(GDK_SELECTION_CLIPBOARD);
    if (cb == NULL) {
        return strdup("");
    }
    gchar *text = gtk_clipboard_wait_for_text(cb);
    if (text == NULL) {
        return strdup("");
    }
    char *out = strdup(text);
    g_free(text);
    return out;
}

static int nexClipboardWriteText(const char *text) {
    GtkClipboard *cb = gtk_clipboard_get(GDK_SELECTION_CLIPBOARD);
    if (cb == NULL) {
        return 0;
    }
    gtk_clipboard_set_text(cb, text, -1);
    gtk_clipboard_store(cb);
    return 1;
}
*/
import "C"
import "unsafe"

func nativeClipboardReadText() (string, bool, string, error) {
	cstr := C.nexClipboardReadText()
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr), true, "gtk_clipboard", nil
}

func nativeClipboardWriteText(text string) (bool, string, error) {
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	ok := C.nexClipboardWriteText(ctext)
	return ok != 0, "gtk_clipboard", nil
}

func nativeClipboardAvailable() (bool, string) {
	return true, "gtk_clipboard"
}
