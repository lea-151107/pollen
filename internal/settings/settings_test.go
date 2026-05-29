package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lea-151107/pollen/internal/userconfig"
)

// withTempConfig redirects userconfig.Dir for the duration of the test.
// SetOverride works on all platforms; XDG_CONFIG_HOME is only honoured on
// Linux by os.UserConfigDir, so using it directly would route this test's
// writes into the developer's real config directory on macOS / Windows.
func withTempConfig(t *testing.T) {
	t.Helper()
	userconfig.SetOverride(t.TempDir())
	t.Cleanup(func() { userconfig.SetOverride("") })
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

func TestLoad_IntruderDefaults(t *testing.T) {
	withTempConfig(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.IntruderConcurrency != 5 {
		t.Errorf("IntruderConcurrency: want 5, got %d", s.IntruderConcurrency)
	}
	if s.IntruderDelayMs != 0 {
		t.Errorf("IntruderDelayMs: want 0, got %d", s.IntruderDelayMs)
	}
	if s.IntruderMaxRequests != 1000 {
		t.Errorf("IntruderMaxRequests: want 1000, got %d", s.IntruderMaxRequests)
	}
}

func TestLoad_IntruderClampsOutOfRange(t *testing.T) {
	withTempConfig(t)
	// Persist out-of-range values, reload, expect them clamped to defaults.
	bad := &Settings{IntruderConcurrency: 9999, IntruderDelayMs: -5, IntruderMaxRequests: 0}
	if err := bad.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.IntruderConcurrency != 5 || s.IntruderDelayMs != 0 || s.IntruderMaxRequests != 1000 {
		t.Errorf("clamped values wrong: %+v", s)
	}
}

func TestLoad_CorruptFileReturnsDefault(t *testing.T) {
	withTempConfig(t)
	path, _ := userconfig.Path("settings.json")
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
