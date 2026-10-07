//go:build linux

package system

import (
	"mime"
	"os"
	"path/filepath"
	"strings"
)

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

func defaultApplicationFor(key string) (handler, source string) {
	for _, path := range xdgMimeappsPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if v, ok := parseIniValue(string(data), "Default Applications", key); ok && v != "" {

			first := strings.TrimSpace(strings.Split(v, ",")[0])
			if first != "" {
				return first, path
			}
		}
	}
	return "", ""
}

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

func nativeProtocolStatus(scheme string) (bool, string, string) {
	handler, source := defaultApplicationFor("x-scheme-handler/" + scheme)
	if handler == "" {
		return false, "", "mimeapps.list"
	}
	return true, handler, source
}

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
