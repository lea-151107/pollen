package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"crypto/x509"
	"net/http/cookiejar"
	"time"

	"github.com/lea-151107/pollen/internal/app"
	"github.com/lea-151107/pollen/internal/exporter"
	"github.com/lea-151107/pollen/internal/version"
	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/env"
	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
	"github.com/lea-151107/pollen/internal/settings"
	"github.com/lea-151107/pollen/internal/ui"
	"github.com/lea-151107/pollen/internal/userconfig"
)

func main() {
	var (
		envName     string
		configDir   string
		collFilter  string
		initConfig  bool
		showVersion bool
		exportColls string
	)
	flag.StringVar(&configDir, "config", "", "config directory (default: ~/.config/pollen)")
	flag.StringVar(&envName, "env", "", "environment name to activate at startup")
	flag.StringVar(&collFilter, "collection", "", "open collections sidebar filtered by name")
	flag.BoolVar(&initConfig, "init-config", false, "write default settings.json and exit")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&exportColls, "export-collections", "", "export collections to Postman v2.1 JSON (use - for stdout)")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "Usage: pollen [--option ...]\n\nOptions:")
		flag.CommandLine.VisitAll(func(f *flag.Flag) {
			// flag.IsBoolFlag is the idiomatic way to detect bool flags.
			if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
				fmt.Fprintf(out, "  --%s\n        %s\n", f.Name, f.Usage)
			} else if f.DefValue != "" {
				fmt.Fprintf(out, "  --%s string\n        %s (default %q)\n", f.Name, f.Usage, f.DefValue)
			} else {
				fmt.Fprintf(out, "  --%s string\n        %s\n", f.Name, f.Usage)
			}
		})
	}
	flag.Parse()

	if showVersion {
		fmt.Println("pollen", version.Version)
		os.Exit(0)
	}

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

	if exportColls != "" {
		collStore, err := collections.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: %v\n", err)
			os.Exit(1)
		}
		data, err := exporter.ExportPostman(collStore.Entries(), "pollen")
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: export: %v\n", err)
			os.Exit(1)
		}
		if exportColls == "-" {
			fmt.Println(string(data))
		} else {
			if err := os.WriteFile(exportColls, data, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "pollen: export: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("exported", len(collStore.Entries()), "entries to", exportColls)
		}
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
		httpx.ProxyURL = cfg.ProxyURL
		httpx.DisableRedirects = cfg.DisableRedirects
		if cfg.CACertFile != "" {
			data, err := os.ReadFile(cfg.CACertFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "pollen: ca_cert_file: %v\n", err)
			} else {
				pool := x509.NewCertPool()
				if !pool.AppendCertsFromPEM(data) {
					fmt.Fprintf(os.Stderr, "pollen: ca_cert_file: no valid PEM certificates\n")
				} else {
					httpx.CACertPool = pool
				}
			}
		}
		if cfg.EnableCookies {
			if jar, err := cookiejar.New(nil); err == nil {
				httpx.CookieJar = jar
			}
		}
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
