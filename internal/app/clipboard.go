package app

import (
	"os"
	"runtime"

	"github.com/atotto/clipboard"

	"github.com/lea-151107/pollen/internal/userconfig"
)

// clipboardHint returns a platform-specific install suggestion to append to a
// failed-copy message. atotto/clipboard shells out to xclip/wl-copy on Linux.
func clipboardHint() string {
	if runtime.GOOS == "linux" {
		return " (install xclip or wl-clipboard)"
	}
	return ""
}

// deliverCopy puts `content` on the clipboard, or on disk if clipboard fails,
// and reports the outcome through statusMsg with appropriate kind.
func (m *Model) deliverCopy(content, label string) {
	mode, path, err := copyOrFallback(content)
	switch {
	case err != nil:
		// err means BOTH the system clipboard AND the file fallback failed.
		// The xclip-install hint isn't useful here — the real problem is
		// likely disk/permission, so spell that out instead.
		m.setStatus(statusError, "copy failed (clipboard and file fallback): "+err.Error())
	case mode == copyClipboard:
		m.setStatus(statusOK, "copied as "+label)
	case mode == copyFile:
		m.setStatus(statusWarn, "clipboard unavailable - wrote "+label+" to "+path+clipboardHint())
	}
}

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
