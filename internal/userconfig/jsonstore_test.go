package userconfig

import (
	"os"
	"path/filepath"
	"testing"
)

type testPayload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestSaveLoadJSON_Roundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := testPayload{Name: "alice", Count: 7}
	if err := SaveJSON("foo.json", &src); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var dst testPayload
	ok, err := LoadJSON("foo.json", &dst)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ok {
		t.Fatal("loaded=false for an existing file")
	}
	if dst != src {
		t.Errorf("roundtrip mismatch: got %+v want %+v", dst, src)
	}
}

func TestLoadJSON_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var dst testPayload
	ok, err := LoadJSON("doesnotexist.json", &dst)
	if err != nil {
		t.Errorf("missing file should not error, got %v", err)
	}
	if ok {
		t.Error("loaded=true for a missing file")
	}
}

func TestLoadJSON_CorruptSurfacesError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Write a corrupt file in the right place.
	path, _ := Path("bad.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("not json"), 0o644)

	var dst testPayload
	ok, err := LoadJSON("bad.json", &dst)
	if err == nil {
		t.Error("corrupt JSON should error; callers decide fallback policy")
	}
	if ok {
		t.Error("loaded=true on error")
	}
}

func TestSaveJSON_AtomicViaTmpRename(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SaveJSON("x.json", testPayload{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	dir, _ := Dir()
	// No leftover .tmp file after a successful write.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}
