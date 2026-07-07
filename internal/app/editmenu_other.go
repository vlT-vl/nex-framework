//go:build !darwin

package app

// ensureEditMenu is a no-op outside macOS: Windows and Linux webview hosts
// already route Ctrl+C/V/X/A through native edit controls without a
// separate application-level Edit menu.
func ensureEditMenu() {}
