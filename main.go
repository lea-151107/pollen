package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"crypto/x509"
	"net/http/cookiejar"
	"time"

	"github.com/lea-151107/pollen/internal/app"
	"github.com/lea-151107/pollen/internal/exporter"
	"github.com/lea-151107/pollen/internal/version"
	"github.com/google/uuid"
	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/curlparse"
	"github.com/lea-151107/pollen/internal/env"
	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
	"github.com/lea-151107/pollen/internal/intruder"
	"github.com/lea-151107/pollen/internal/settings"
	"github.com/lea-151107/pollen/internal/ui"
	"github.com/lea-151107/pollen/internal/userconfig"
)

// readCurlSource resolves the --import-curl flag's value to the raw
// command text. "-" reads stdin, "@<path>" reads the file, anything
// else is the literal command.
func readCurlSource(arg string) (string, error) {
	switch {
	case arg == "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(data), nil
	case strings.HasPrefix(arg, "@"):
		path := strings.TrimPrefix(arg, "@")
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		return string(data), nil
	default:
		return arg, nil
	}
}

func main() {
	var (
		envName     string
		configDir   string
		collFilter  string
		initConfig  bool
		showVersion bool
		exportColls    string
		exportPostman  string
		exportOpenAPI  string
		exportIntruder string
		importCurl     string
	)
	flag.StringVar(&configDir, "config", "", "config directory (default: ~/.config/pollen)")
	flag.StringVar(&envName, "env", "", "environment name to activate at startup")
	flag.StringVar(&collFilter, "collection", "", "open collections sidebar filtered by name")
	flag.BoolVar(&initConfig, "init-config", false, "write default settings.json and exit")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&exportPostman, "export-postman", "", "export collections to Postman v2.1 JSON (use - for stdout)")
	flag.StringVar(&exportColls, "export-collections", "", "alias for --export-postman (kept for backwards compatibility)")
	flag.StringVar(&exportOpenAPI, "export-openapi", "", "export collections as OpenAPI 3.x (.json / .yaml / .yml; use - for stdout JSON)")
	flag.StringVar(&exportIntruder, "export-intruder", "", "export the last Intruder run (.csv / .json; use - for stdout CSV)")
	flag.StringVar(&importCurl, "import-curl", "", "parse a curl command and add it to collections (literal, @file, or - for stdin)")
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

	if exportPostman != "" && exportColls != "" {
		fmt.Fprintln(os.Stderr,
			"pollen: --export-postman and --export-collections are aliases; specify only one")
		os.Exit(2)
	}
	postmanTarget := exportPostman
	if postmanTarget == "" {
		postmanTarget = exportColls
	}
	// Each run dispatches at most one one-shot operation. Refuse to
	// silently drop the later flags when multiple --export-* / --import-curl
	// are passed; the user almost certainly meant a sequence of runs,
	// not "first wins". The export blocks below call os.Exit(0) on
	// success, so without this check the --import-curl branch further
	// down would silently never run.
	oneShotGroups := 0
	if postmanTarget != "" {
		oneShotGroups++
	}
	if exportOpenAPI != "" {
		oneShotGroups++
	}
	if exportIntruder != "" {
		oneShotGroups++
	}
	if importCurl != "" {
		oneShotGroups++
	}
	if oneShotGroups > 1 {
		fmt.Fprintln(os.Stderr,
			"pollen: specify only one of --export-postman / --export-collections / --export-openapi / --export-intruder / --import-curl per run")
		os.Exit(2)
	}
	if postmanTarget != "" {
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
		if postmanTarget == "-" {
			fmt.Println(string(data))
		} else {
			if err := os.WriteFile(postmanTarget, data, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "pollen: export: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("exported", len(collStore.Entries()), "entries to", postmanTarget)
		}
		os.Exit(0)
	}

	if exportOpenAPI != "" {
		collStore, err := collections.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: %v\n", err)
			os.Exit(1)
		}
		format := exporter.OpenAPIJSON
		switch strings.ToLower(filepath.Ext(exportOpenAPI)) {
		case ".yaml", ".yml":
			format = exporter.OpenAPIYAML
		}
		data, err := exporter.ExportOpenAPI(collStore.Entries(), "pollen", format)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: export: %v\n", err)
			os.Exit(1)
		}
		if exportOpenAPI == "-" {
			fmt.Println(string(data))
		} else {
			if err := os.WriteFile(exportOpenAPI, data, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "pollen: export: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("exported", len(collStore.Entries()), "entries to", exportOpenAPI)
		}
		os.Exit(0)
	}

	if exportIntruder != "" {
		results, err := intruder.LoadLastRun()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: export: %v\n", err)
			os.Exit(1)
		}
		if results == nil {
			fmt.Fprintln(os.Stderr, "pollen: no intruder run to export (run one from the TUI first)")
			os.Exit(2)
		}
		var data []byte
		switch strings.ToLower(filepath.Ext(exportIntruder)) {
		case ".json":
			data, err = intruder.JSON(results)
		default:
			// CSV is the default for stdout, .csv, and any other extension.
			data, err = intruder.CSV(results)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: export: %v\n", err)
			os.Exit(1)
		}
		if exportIntruder == "-" {
			fmt.Print(string(data))
		} else {
			if err := os.WriteFile(exportIntruder, data, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "pollen: export: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("exported", len(results), "rows to", exportIntruder)
		}
		os.Exit(0)
	}

	if importCurl != "" {
		cmd, err := readCurlSource(importCurl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: import-curl: %v\n", err)
			os.Exit(1)
		}
		req, err := curlparse.Parse(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: import-curl: %v\n", err)
			os.Exit(1)
		}
		collStore, err := collections.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: %v\n", err)
			os.Exit(1)
		}
		name := req.Method + " " + req.URL
		collStore.Add(collections.Entry{
			ID:      uuid.NewString(),
			Name:    name,
			Request: req,
		})
		if err := collStore.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "pollen: import-curl: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "imported as", name)
		os.Exit(0)
	}

	store, err := history.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pollen: history: %v\n", err)
		os.Exit(1)
	}

	collStore, err := collections.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pollen: collections: %v\n", err)
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
