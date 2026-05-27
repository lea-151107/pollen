package userconfig

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// withTempOverride redirects userconfig.Dir() at the package level for the
// duration of t, restoring the previous override on cleanup. Unlike
// XDG_CONFIG_HOME — which only os.UserConfigDir on Linux honours — this works
// uniformly on Linux, macOS, and Windows.
func withTempOverride(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	SetOverride(tmp)
	t.Cleanup(func() { SetOverride("") })
	return tmp
}

func TestDir_UsesXDGConfigHome(t *testing.T) {
	// os.UserConfigDir only consults XDG_CONFIG_HOME on Linux. macOS uses
	// ~/Library/Application Support and Windows uses %AppData%, both of which
	// ignore the env var. Skip elsewhere; the semantic is Linux-only.
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CONFIG_HOME is honored by os.UserConfigDir only on Linux")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, "pollen")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestPath(t *testing.T) {
	tmp := withTempOverride(t)

	got, err := Path("history.json")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, "history.json")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if !strings.HasSuffix(got, "history.json") {
		t.Errorf("Path tail wrong: %q", got)
	}
}

func TestPath_NestedName(t *testing.T) {
	tmp := withTempOverride(t)

	got, err := Path("blobs/abc.bin")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, "blobs", "abc.bin")
	if got != want {
		t.Errorf("nested Path: got %q want %q", got, want)
	}
}

func TestSetOverride_EmptyClearsPrevious(t *testing.T) {
	// SetOverride("") must clear, otherwise t.Cleanup leaks the override into
	// adjacent tests.
	SetOverride("/tmp/some-override")
	if dirOverride == "" {
		t.Fatal("SetOverride should have set dirOverride")
	}
	SetOverride("")
	if dirOverride != "" {
		t.Errorf("SetOverride(\"\") should clear, got %q", dirOverride)
	}
}
