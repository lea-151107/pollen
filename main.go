package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea/pollen/internal/app"
	"github.com/lea/pollen/internal/history"
)

func main() {
	store, err := history.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load history: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(app.New(store), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "pollen: %v\n", err)
		os.Exit(1)
	}
}
