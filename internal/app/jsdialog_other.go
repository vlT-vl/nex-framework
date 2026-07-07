//go:build !darwin

package app

// ensureJSDialogSupport is a no-op outside macOS: WebKitGTK (Linux) and
// WebView2 (Windows) already show a default native dialog for
// window.alert/confirm/prompt without any extra delegate wiring.
func ensureJSDialogSupport() {}
