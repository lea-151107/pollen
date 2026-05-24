package app

import (
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lea/pollen/internal/history"
)

// saveResponseBytes writes b to a file in the current working directory,
// picking a name from Content-Disposition / URL / fallback. Returns the path
// actually written.
func saveResponseBytes(b []byte, resp *history.Response, reqURL string) (string, error) {
	if len(b) == 0 {
		return "", errors.New("no body to save")
	}
	name := pickFilename(resp, reqURL)
	if resp != nil {
		name = ensureExtension(name, resp.ContentType)
	}
	dest, err := uniquePath(".", name)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, b, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func pickFilename(resp *history.Response, reqURL string) string {
	if resp != nil {
		for _, h := range resp.Headers {
			if !strings.EqualFold(h.Key, "Content-Disposition") {
				continue
			}
			if _, params, err := mime.ParseMediaType(h.Value); err == nil {
				if fn := params["filename"]; fn != "" {
					return sanitizeFilename(fn)
				}
			}
		}
	}
	if u, err := url.Parse(reqURL); err == nil {
		base := path.Base(u.Path)
		if base != "" && base != "/" && base != "." {
			return sanitizeFilename(base)
		}
	}
	return "response.bin"
}

// ensureExtension appends an extension derived from contentType when the name
// has none. Returns name unchanged when a) name already has an extension,
// b) contentType is empty, or c) the MIME type has no known extension.
func ensureExtension(name, contentType string) string {
	if filepath.Ext(name) != "" {
		return name
	}
	if contentType == "" {
		return name
	}
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		return name
	}
	return name + exts[0]
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name) // strip directory components
	name = strings.ReplaceAll(name, string(filepath.Separator), "_")
	name = strings.Trim(name, " .")
	if name == "" {
		return "response.bin"
	}
	return name
}

// uniquePath returns dir/name if it does not exist, otherwise appends "(2)",
// "(3)" before the extension until a non-existing path is found.
func uniquePath(dir, name string) (string, error) {
	candidate := filepath.Join(dir, name)
	if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
		return candidate, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// If we can't stat for a reason other than non-existence, bail out.
		if !errors.Is(err, fs.ErrPermission) {
			return "", err
		}
	}

	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; i < 1000; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s(%d)%s", stem, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", errors.New("could not find a unique filename")
}
