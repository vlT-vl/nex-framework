//go:build darwin

package system

import (
	"os"
	"path/filepath"
	"strings"
)

// nativeProtocolStatus/nativeFileAssociation read the app's own Info.plist
// (already pure Go, no subprocess) for URL scheme / document type
// registrations — unchanged from before this file existed, just extracted
// here to match the per-platform nativeXxx naming used across the package.

func nativeProtocolStatus(scheme string) (bool, string, string) {
	plist := currentBundleInfoPlist()
	if plist == "" {
		return false, "", "bundle Info.plist"
	}
	b, err := os.ReadFile(plist)
	if err != nil {
		return false, "", "bundle Info.plist"
	}
	if strings.Contains(strings.ToLower(string(b)), "<string>"+scheme+"</string>") {
		return true, filepath.Dir(filepath.Dir(plist)), "bundle Info.plist"
	}
	return false, "", "bundle Info.plist"
}

func nativeFileAssociation(ext string) (bool, string, string, string) {
	plist := currentBundleInfoPlist()
	if plist == "" {
		return false, "", "", "bundle Info.plist"
	}
	b, err := os.ReadFile(plist)
	if err != nil {
		return false, "", "", "bundle Info.plist"
	}
	if strings.Contains(strings.ToLower(string(b)), "<string>"+strings.TrimPrefix(ext, ".")+"</string>") {
		return true, filepath.Dir(filepath.Dir(plist)), "", "bundle Info.plist"
	}
	return false, "", "", "bundle Info.plist"
}
