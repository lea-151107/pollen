// Package env stores user-defined variables, organised into named
// environments (e.g. dev/staging/prod), that can be referenced in requests as
// `{{varName}}` and expanded at send time. The active environment is
// switchable from inside the app.
//
// Persisted as JSON at ~/.config/pollen/env.json. Missing file yields an
// empty environment (no error). Older v0.1.0 files written as a flat
// `{"vars": {...}}` are migrated transparently into a single "default"
// environment on the first load.
package env

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

const defaultEnvName = "default"

type Env struct {
	Current      string                       `json:"current"`
	Environments map[string]map[string]string `json:"environments"`

	// LegacyVars exists solely to migrate v0.1.0 files of the form
	// {"vars": {...}}. On Load it's folded into Environments["default"] and
	// then cleared so subsequent Save calls write only the new schema.
	LegacyVars map[string]string `json:"vars,omitempty"`
}

func New() *Env {
	return &Env{Environments: map[string]map[string]string{}}
}

// Load reads env from disk. Missing or corrupt files yield an empty Env.
// v0.1.0 "vars" payloads are migrated into a "default" environment.
func Load() (*Env, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return New(), nil
		}
		return nil, err
	}
	var e Env
	if err := json.Unmarshal(data, &e); err != nil {
		// Corrupt file shouldn't brick startup — fall back to empty.
		return New(), nil
	}
	if e.Environments == nil {
		e.Environments = map[string]map[string]string{}
	}
	// Migrate v0.1.0 flat format into a "default" environment.
	if len(e.LegacyVars) > 0 && len(e.Environments) == 0 {
		e.Environments[defaultEnvName] = e.LegacyVars
		e.Current = defaultEnvName
	}
	e.LegacyVars = nil
	// If Current is empty but at least one environment exists, pick the first
	// alphabetical name so something is selected by default.
	if e.Current == "" && len(e.Environments) > 0 {
		e.Current = e.Names()[0]
	}
	return &e, nil
}

// Save writes env atomically.
func (e *Env) Save() error {
	path, err := defaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var varRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// Vars returns the variable map of the currently selected environment, or
// nil if no environment is selected or it has no vars.
func (e *Env) Vars() map[string]string {
	if e == nil {
		return nil
	}
	return e.Environments[e.Current]
}

// Expand replaces every `{{name}}` token with the value from the currently
// selected environment. Unknown names are left in place so the user can see
// what failed to resolve. Expansion is single-pass: a substituted value
// containing further `{{}}` is NOT re-expanded.
func (e *Env) Expand(s string) string {
	vars := e.Vars()
	if len(vars) == 0 {
		return s
	}
	return varRe.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-2]
		if v, ok := vars[name]; ok {
			return v
		}
		return match
	})
}

// Count returns how many variables are in the active environment.
func (e *Env) Count() int {
	return len(e.Vars())
}

// Names returns sorted environment names.
func (e *Env) Names() []string {
	if e == nil {
		return nil
	}
	names := make([]string, 0, len(e.Environments))
	for n := range e.Environments {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// SetCurrent switches to the named environment. Returns an error if the name
// doesn't exist; the previous selection is preserved on failure.
func (e *Env) SetCurrent(name string) error {
	if e == nil {
		return errors.New("nil env")
	}
	if _, ok := e.Environments[name]; !ok {
		return fmt.Errorf("unknown environment: %s", name)
	}
	e.Current = name
	return nil
}

// Path returns the on-disk location of env.json.
func Path() string {
	p, _ := defaultPath()
	return p
}

func defaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pollen", "env.json"), nil
}
