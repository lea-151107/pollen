package userconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type testPayload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestSaveLoadJSON_Roundtrip(t *testing.T) {
	withTempOverride(t)

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
	withTempOverride(t)

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
	withTempOverride(t)
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

func TestSaveJSONSecure_Uses0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows file permissions are ACL-based; Go's os.WriteFile
		// reports 0o666 regardless of the requested mode. The 0o600
		// guarantee is meaningful only on POSIX systems, where this
		// test continues to pin it.
		t.Skip("POSIX mode bits don't apply on Windows (ACL-based)")
	}
	withTempOverride(t)

	if err := SaveJSONSecure("secret.json", testPayload{Name: "creds"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, _ := Path("secret.json")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 0600", perm)
	}
}

func TestSaveJSON_Uses0644(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows file permissions are ACL-based; Go's os.WriteFile
		// reports 0o666 regardless of the requested mode. This
		// regression test pins POSIX behaviour only.
		t.Skip("POSIX mode bits don't apply on Windows (ACL-based)")
	}
	withTempOverride(t)

	if err := SaveJSON("plain.json", testPayload{Name: "plain"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, _ := Path("plain.json")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o644 {
		t.Errorf("file mode = %o, want 0644 (regression: should not change with SaveJSONSecure addition)", perm)
	}
}

func TestSaveJSON_AtomicViaTmpRename(t *testing.T) {
	withTempOverride(t)

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
