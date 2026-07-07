//go:build linux

// Package system: native Linux protocol/file-association lookups by parsing
// the XDG mimeapps.list / shared-mime-info glob databases directly — no
// subprocess (`xdg-mime query ...`) is spawned.
package system

import (
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// xdgMimeappsPaths returns the mimeapps.list search path in XDG priority
// order (user config, user data, system data), matching what `xdg-mime`
// itself consults.
func xdgMimeappsPaths() []string {
	var paths []string
	if cfg, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(cfg, "mimeapps.list"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".local/share/applications/mimeapps.list"))
	}
	paths = append(paths,
		"/usr/local/share/applications/mimeapps.list",
		"/usr/share/applications/mimeapps.list",
	)
	return paths
}

// defaultApplicationFor returns the .desktop handler registered for a given
// mimetype/scheme key (e.g. "x-scheme-handler/https" or "text/plain") from
// the first mimeapps.list that defines it, in XDG priority order.
func defaultApplicationFor(key string) (handler, source string) {
	for _, path := range xdgMimeappsPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if v, ok := parseIniValue(string(data), "Default Applications", key); ok && v != "" {
			// Value may be a comma-separated list; the first entry is the default.
			first := strings.TrimSpace(strings.Split(v, ",")[0])
			if first != "" {
				return first, path
			}
		}
	}
	return "", ""
}

// parseIniValue does a minimal desktop-entry-style INI lookup for a single
// "key=value" pair inside a "[section]" block.
func parseIniValue(data, section, key string) (string, bool) {
	inSection := false
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = strings.EqualFold(line[1:len(line)-1], section)
			continue
		}
		if !inSection {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// nativeProtocolStatus replaces `xdg-mime query default x-scheme-handler/<scheme>`.
func nativeProtocolStatus(scheme string) (bool, string, string) {
	handler, source := defaultApplicationFor("x-scheme-handler/" + scheme)
	if handler == "" {
		return false, "", "mimeapps.list"
	}
	return true, handler, source
}

// nativeFileAssociation replaces
// `xdg-mime query filetype <probe>` + `xdg-mime query default <mime>`.
func nativeFileAssociation(ext string) (registered bool, handler, mimeType, source string) {
	mimeType = mime.TypeByExtension(ext)
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	if mimeType == "" {
		mimeType = mimeTypeFromGlobs(ext)
	}
	if mimeType == "" {
		return false, "", "", "shared-mime-info globs"
	}
	handler, source = defaultApplicationFor(mimeType)
	return handler != "", handler, mimeType, source
}

// mimeTypeFromGlobs parses /usr/share/mime/globs (format: "glob:mimetype",
// one per line, e.g. "*.png:image/png") for the shared-mime-info database's
// mapping of extensions the stdlib "mime" package doesn't already know.
func mimeTypeFromGlobs(ext string) string {
	data, err := os.ReadFile("/usr/share/mime/globs")
	if err != nil {
		return ""
	}
	suffix := "*" + strings.ToLower(ext)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		glob, mt, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(glob, suffix) {
			return mt
		}
	}
	return ""
}
