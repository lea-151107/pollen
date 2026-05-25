package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea/pollen/internal/app"
	"github.com/lea/pollen/internal/collections"
	"github.com/lea/pollen/internal/env"
	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
	"github.com/lea/pollen/internal/settings"
	"github.com/lea/pollen/internal/userconfig"
)

func main() {
	var (
		envName    string
		configDir  string
		collFilter string
	)
	flag.StringVar(&configDir, "config", "", "config directory (default: ~/.config/pollen)")
	flag.StringVar(&envName, "env", "", "environment name to activate at startup")
	flag.StringVar(&collFilter, "collection", "", "open collections sidebar filtered by name")
	flag.Parse()

	if configDir != "" {
		userconfig.SetOverride(configDir)
	}

	store, err := history.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load history: %v\n", err)
		os.Exit(1)
	}

	collStore, err := collections.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load collections: %v\n", err)
		os.Exit(1)
	}

	// Restore persistent settings (TLS toggle, etc.). Failures fall back to
	// defaults silently — settings shouldn't block startup.
	cfg, _ := settings.Load()
	if cfg != nil {
		httpx.SkipTLSVerify.Store(cfg.SkipTLSVerify)
	}

	// Variable environment (~/.config/pollen/env.json). Missing/corrupt → empty.
	envVars, _ := env.Load()

	if envName != "" {
		if err := envVars.SetCurrent(envName); err != nil {
			fmt.Fprintf(os.Stderr, "pollen: --env: %v\n", err)
		}
	}

	opts := app.Options{StartCollection: collFilter}
	p := tea.NewProgram(app.New(store, collStore, envVars, opts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "pollen: %v\n", err)
		os.Exit(1)
	}
}
