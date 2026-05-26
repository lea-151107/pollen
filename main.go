package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"time"

	"github.com/lea/pollen/internal/app"
	"github.com/lea/pollen/internal/collections"
	"github.com/lea/pollen/internal/env"
	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
	"github.com/lea/pollen/internal/settings"
	"github.com/lea/pollen/internal/ui"
	"github.com/lea/pollen/internal/userconfig"
)

func main() {
	var (
		envName    string
		configDir  string
		collFilter string
		initConfig bool
	)
	flag.StringVar(&configDir, "config", "", "config directory (default: ~/.config/pollen)")
	flag.StringVar(&envName, "env", "", "environment name to activate at startup")
	flag.StringVar(&collFilter, "collection", "", "open collections sidebar filtered by name")
	flag.BoolVar(&initConfig, "init-config", false, "write default settings.json and exit")
	flag.Parse()

	if configDir != "" {
		userconfig.SetOverride(configDir)
	}

	if initConfig {
		path, err := settings.WriteDefaults()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("created", path)
		os.Exit(0)
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

	// Restore persistent settings. Failures fall back to defaults silently —
	// settings shouldn't block startup.
	cfg, _ := settings.Load()
	if cfg != nil {
		httpx.SkipTLSVerify.Store(cfg.SkipTLSVerify)
		httpx.RequestTimeout = time.Duration(cfg.RequestTimeoutSecs) * time.Second
		httpx.MaxResponseBytes = cfg.MaxResponseMiB * 1024 * 1024
		store.SetMaxEntries(cfg.HistoryLimit)
		ui.TextPreviewLimit = cfg.TextPreviewKiB * 1024
		ui.DefaultHexDumpLimit = cfg.HexDumpKiB * 1024
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
