// Package env stores user-defined variables that can be referenced in
// requests as `{{varName}}` and expanded at send time.
//
// Persisted as JSON at ~/.config/pollen/env.json. Missing file yields an
// empty environment (no error).
package env

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

type Env struct {
	Vars map[string]string `json:"vars"`
}

func New() *Env {
	return &Env{Vars: map[string]string{}}
}

// Load reads env from disk. Missing or corrupt files yield an empty Env.
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
	if e.Vars == nil {
		e.Vars = map[string]string{}
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

// Expand replaces every `{{name}}` token with the corresponding Vars entry.
// Unknown names are left in place so the user can see what failed to resolve.
// Expansion is single-pass: a substituted value containing further `{{}}` is
// NOT re-expanded, which keeps the operation predictable and loop-free.
func (e *Env) Expand(s string) string {
	if e == nil || len(e.Vars) == 0 {
		return s
	}
	return varRe.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-2]
		if v, ok := e.Vars[name]; ok {
			return v
		}
		return match
	})
}

// Count returns how many variables are defined; used for status display.
func (e *Env) Count() int {
	if e == nil {
		return 0
	}
	return len(e.Vars)
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
