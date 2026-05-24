package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea/pollen/internal/app"
	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
	"github.com/lea/pollen/internal/settings"
)

func main() {
	store, err := history.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load history: %v\n", err)
		os.Exit(1)
	}

	// Restore persistent settings (TLS toggle, etc.). Failures fall back to
	// defaults silently — settings shouldn't block startup.
	cfg, _ := settings.Load()
	if cfg != nil {
		httpx.SkipTLSVerify.Store(cfg.SkipTLSVerify)
	}

	p := tea.NewProgram(app.New(store), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "pollen: %v\n", err)
		os.Exit(1)
	}
}
