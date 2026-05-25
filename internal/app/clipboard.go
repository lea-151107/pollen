package app

import (
	"os"

	"github.com/atotto/clipboard"

	"github.com/lea/pollen/internal/userconfig"
)

// CopyMode reports how content was delivered to the user.
type copyMode int

const (
	copyClipboard copyMode = iota // wrote to system clipboard
	copyFile                      // clipboard unavailable, wrote to fallback file
)

// copyOrFallback tries the system clipboard first; if that fails (typically
// xclip/wl-clipboard missing on Linux), writes to ~/.config/pollen/clipboard.txt
// so the user still has somewhere to grab the content from.
func copyOrFallback(content string) (mode copyMode, path string, err error) {
	if cbErr := clipboard.WriteAll(content); cbErr == nil {
		return copyClipboard, "", nil
	} else {
		// fall through to file fallback
		_ = cbErr
	}

	fallback, err := userconfig.Path("clipboard.txt")
	if err != nil {
		return 0, "", err
	}
	dir, err := userconfig.Dir()
	if err != nil {
		return 0, "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, "", err
	}
	if err := os.WriteFile(fallback, []byte(content), 0o644); err != nil {
		return 0, "", err
	}
	return copyFile, fallback, nil
}
