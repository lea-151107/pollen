// Package userconfig centralises pollen's on-disk config layout.
//
// Every user-visible file (history.json, settings.json, env.json,
// clipboard.txt, ...) lives under a single directory whose location is
// resolved here. Other packages should call Path() rather than constructing
// `~/.config/pollen/<name>` themselves — this keeps the location adjustable
// from one place (e.g. for a future POLLEN_HOME env var override).
package userconfig

import (
	"os"
	"path/filepath"
)

const appDirName = "pollen"

var dirOverride string

// SetOverride replaces the default config directory for this process lifetime.
// Call before opening any stores (e.g. from main, before history.Open).
// Passing an empty string clears any previous override (useful in tests).
func SetOverride(path string) {
	if path == "" {
		dirOverride = ""
		return
	}
	dirOverride = filepath.Clean(path)
}

// Dir returns the absolute path of the pollen config directory. It does NOT
// create the directory; callers that need to write should MkdirAll themselves
// (or use the JSON helpers in this package, which do it for them).
func Dir() (string, error) {
	if dirOverride != "" {
		return dirOverride, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

// Path returns the full path of a file inside the pollen config directory.
// The file name is joined with filepath.Join, so callers can pass either a
// flat name ("history.json") or a relative path ("blobs/abc.bin").
func Path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
