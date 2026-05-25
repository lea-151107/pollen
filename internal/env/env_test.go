package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lea/pollen/internal/userconfig"
)

func withTempConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

// envFor builds an Env with a single active environment containing vars.
func envFor(vars map[string]string) *Env {
	return &Env{
		Current:      "test",
		Environments: map[string]map[string]string{"test": vars},
	}
}

func TestExpand_Basic(t *testing.T) {
	e := envFor(map[string]string{
		"baseUrl": "https://api.example.com",
		"version": "v1",
	})
	got := e.Expand("{{baseUrl}}/{{version}}/users")
	want := "https://api.example.com/v1/users"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExpand_UnknownVarLeftIntact(t *testing.T) {
	e := envFor(map[string]string{"a": "1"})
	got := e.Expand("{{a}} and {{missing}}")
	want := "1 and {{missing}}"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExpand_NoRecursion(t *testing.T) {
	// Substituted value is not re-expanded — preventing infinite loops.
	e := envFor(map[string]string{
		"a": "{{b}}",
		"b": "final",
	})
	got := e.Expand("{{a}}")
	want := "{{b}}" // not "final"
	if got != want {
		t.Errorf("got %q want %q (single-pass expansion expected)", got, want)
	}
}

func TestExpand_NilOrEmptyEnv(t *testing.T) {
	var nilEnv *Env
	if got := nilEnv.Expand("{{x}}"); got != "{{x}}" {
		t.Errorf("nil env should passthrough, got %q", got)
	}
	empty := New()
	if got := empty.Expand("{{x}}"); got != "{{x}}" {
		t.Errorf("empty env should passthrough, got %q", got)
	}
}

func TestExpand_NoCurrentEnvSelected(t *testing.T) {
	// Environments exist but Current is unset → no expansion.
	e := &Env{Environments: map[string]map[string]string{
		"dev": {"a": "1"},
	}}
	if got := e.Expand("{{a}}"); got != "{{a}}" {
		t.Errorf("got %q want %q", got, "{{a}}")
	}
}

func TestSetCurrent(t *testing.T) {
	e := &Env{Environments: map[string]map[string]string{
		"dev":  {"a": "1"},
		"prod": {"a": "2"},
	}}
	if err := e.SetCurrent("prod"); err != nil {
		t.Fatal(err)
	}
	if got := e.Expand("{{a}}"); got != "2" {
		t.Errorf("after SetCurrent(prod), got %q want %q", got, "2")
	}
	if err := e.SetCurrent("nope"); err == nil {
		t.Error("SetCurrent with unknown name should error")
	}
	// Previous current should be preserved on error.
	if e.Current != "prod" {
		t.Errorf("failed SetCurrent leaked: Current=%q", e.Current)
	}
}

func TestNames_Sorted(t *testing.T) {
	e := &Env{Environments: map[string]map[string]string{
		"prod":    {},
		"dev":     {},
		"staging": {},
	}}
	got := e.Names()
	want := []string{"dev", "prod", "staging"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	withTempConfig(t)
	e, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if e == nil || e.Count() != 0 || len(e.Environments) != 0 {
		t.Errorf("expected empty env, got %+v", e)
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	withTempConfig(t)
	e := &Env{
		Current: "dev",
		Environments: map[string]map[string]string{
			"dev":  {"baseUrl": "http://localhost:8080"},
			"prod": {"baseUrl": "https://api.example.com"},
		},
	}
	if err := e.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Current != "dev" {
		t.Errorf("Current: got %q want dev", got.Current)
	}
	if got.Environments["dev"]["baseUrl"] != "http://localhost:8080" {
		t.Errorf("dev vars lost: %+v", got.Environments["dev"])
	}
	if got.Environments["prod"]["baseUrl"] != "https://api.example.com" {
		t.Errorf("prod vars lost: %+v", got.Environments["prod"])
	}
}

func TestLoad_MigratesLegacyVarsFormat(t *testing.T) {
	withTempConfig(t)
	path, _ := userconfig.Path("env.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	legacy := `{"vars": {"baseUrl": "https://legacy.example.com", "token": "old"}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	e, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if e.Current != "default" {
		t.Errorf("expected Current=default, got %q", e.Current)
	}
	if e.Environments["default"]["baseUrl"] != "https://legacy.example.com" {
		t.Errorf("migration lost baseUrl: %+v", e.Environments)
	}
	if e.LegacyVars != nil {
		t.Errorf("LegacyVars should be cleared after migration, got %+v", e.LegacyVars)
	}
}

func TestLoad_CurrentEmptyButHasEnvsPicksFirst(t *testing.T) {
	withTempConfig(t)
	path, _ := userconfig.Path("env.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data := `{"environments": {"zeta": {"a":"1"}, "alpha": {"a":"2"}}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	e, _ := Load()
	if e.Current != "alpha" {
		t.Errorf("expected first sorted name (alpha), got %q", e.Current)
	}
}

func TestLoad_CorruptFileReturnsEmpty(t *testing.T) {
	withTempConfig(t)
	path, _ := userconfig.Path("env.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("not json"), 0o644)

	e, err := Load()
	if err != nil {
		t.Fatalf("corrupt file should not error: %v", err)
	}
	if e.Count() != 0 {
		t.Errorf("expected empty env on corrupt, got %+v", e.Environments)
	}
}
