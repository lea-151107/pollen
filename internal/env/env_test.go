package env

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func TestExpand_Basic(t *testing.T) {
	e := &Env{Vars: map[string]string{
		"baseUrl": "https://api.example.com",
		"version": "v1",
	}}
	got := e.Expand("{{baseUrl}}/{{version}}/users")
	want := "https://api.example.com/v1/users"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExpand_UnknownVarLeftIntact(t *testing.T) {
	e := &Env{Vars: map[string]string{"a": "1"}}
	got := e.Expand("{{a}} and {{missing}}")
	want := "1 and {{missing}}"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestExpand_NoRecursion(t *testing.T) {
	// Substituted value is not re-expanded — preventing infinite loops.
	e := &Env{Vars: map[string]string{
		"a": "{{b}}",
		"b": "final",
	}}
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

func TestExpand_NoVarsInInput(t *testing.T) {
	e := &Env{Vars: map[string]string{"a": "1"}}
	if got := e.Expand("plain text"); got != "plain text" {
		t.Errorf("got %q want %q", got, "plain text")
	}
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	withTempConfig(t)
	e, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if e == nil || e.Count() != 0 {
		t.Errorf("expected empty env, got %+v", e)
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	withTempConfig(t)
	e := &Env{Vars: map[string]string{
		"baseUrl": "https://api.example.com",
		"token":   "secret",
	}}
	if err := e.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Vars["baseUrl"] != "https://api.example.com" || got.Vars["token"] != "secret" {
		t.Errorf("roundtrip lost values: %+v", got.Vars)
	}
}

func TestLoad_CorruptFileReturnsEmpty(t *testing.T) {
	withTempConfig(t)
	path, _ := defaultPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("not json"), 0o644)

	e, err := Load()
	if err != nil {
		t.Fatalf("corrupt file should not error: %v", err)
	}
	if e.Count() != 0 {
		t.Errorf("expected empty env on corrupt, got %+v", e.Vars)
	}
}
