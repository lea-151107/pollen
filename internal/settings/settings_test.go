package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempConfig redirects XDG_CONFIG_HOME for the duration of the test.
func withTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestLoad_MissingFileReturnsDefault(t *testing.T) {
	withTempConfig(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.SkipTLSVerify {
		t.Error("default should have SkipTLSVerify=false")
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	withTempConfig(t)
	s := &Settings{SkipTLSVerify: true}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.SkipTLSVerify {
		t.Errorf("roundtrip lost SkipTLSVerify")
	}
}

func TestLoad_CorruptFileReturnsDefault(t *testing.T) {
	withTempConfig(t)
	path, _ := defaultPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("not json"), 0o644)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load should not error on corrupt JSON: %v", err)
	}
	if s.SkipTLSVerify {
		t.Error("corrupt file should fall back to default")
	}
}
