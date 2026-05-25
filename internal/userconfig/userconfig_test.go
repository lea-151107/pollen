package userconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDir_UsesXDGConfigHome(t *testing.T) {
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
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := Path("history.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, filepath.Join("pollen", "history.json")) {
		t.Errorf("Path tail wrong: %q", got)
	}
}

func TestPath_NestedName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := Path("blobs/abc.bin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, filepath.Join("pollen", "blobs", "abc.bin")) {
		t.Errorf("nested Path wrong: %q", got)
	}
}
