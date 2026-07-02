package main

import (
	"context"
	"encoding/json"
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
	"github.com/lea-151107/pollen/internal/scenario"
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
		runScenario    string
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
	flag.StringVar(&runScenario, "run", "", "run a scenario headlessly and exit non-zero on failure (saved name, @file, or - for stdin JSON)")
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
	if runScenario != "" {
		oneShotGroups++
	}
	if oneShotGroups > 1 {
		fmt.Fprintln(os.Stderr,
			"pollen: specify only one of --export-postman / --export-collections / --export-openapi / --export-intruder / --import-curl / --run per run")
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

	if runScenario != "" {
		// Honour proxy / TLS / timeout / cookie-jar settings so a headless run
		// behaves like the TUI — the shared cookie jar in particular lets a
		// login step's session cookie carry into later steps.
		cfg, _ := settings.Load()
		applyHTTPXConfig(cfg)

		envVars, _ := env.Load()
		if envName != "" {
			if err := envVars.SetCurrent(envName); err != nil {
				fmt.Fprintf(os.Stderr, "pollen: --env: %v\n", err)
			}
		}

		sc, err := resolveScenario(runScenario)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: run: %v\n", err)
			os.Exit(1)
		}
		results := scenario.Run(context.Background(), sc, scenario.RunOpts{Env: envVars})
		if reportScenarioRun(os.Stdout, sc, results) {
			os.Exit(1)
		}
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
	applyHTTPXConfig(cfg)
	if cfg != nil {
		store.SetMaxEntries(cfg.HistoryLimit)
		ui.TextPreviewLimit = cfg.TextPreviewKiB * 1024
		ui.DefaultHexDumpLimit = cfg.HexDumpKiB * 1024
	}
	// Mouse defaults to ON; only an explicit enable_mouse:false disables it. A
	// nil cfg (settings failed to load entirely) still gets the default.
	enableMouse := cfg == nil || cfg.EnableMouse

	// Variable environment (~/.config/pollen/env.json). Missing/corrupt → empty.
	envVars, _ := env.Load()

	if envName != "" {
		if err := envVars.SetCurrent(envName); err != nil {
			fmt.Fprintf(os.Stderr, "pollen: --env: %v\n", err)
		}
	}

	opts := app.Options{StartCollection: collFilter, MouseEnabled: enableMouse}
	progOpts := []tea.ProgramOption{tea.WithAltScreen()}
	if enableMouse {
		// SGR mouse mode (click + wheel). Off by default because enabling it
		// hijacks the terminal's native text selection / copy (users must hold
		// Shift), which many keyboard-driven users rely on.
		progOpts = append(progOpts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(app.New(store, collStore, envVars, opts), progOpts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "pollen: %v\n", err)
		os.Exit(1)
	}
}

// applyHTTPXConfig applies the network-relevant settings (TLS / timeout /
// proxy / redirects / CA cert / cookie jar) to the global httpx config. Shared
// by the TUI startup and the --run headless path so both behave identically.
// nil cfg (settings failed to load) leaves the defaults in place.
func applyHTTPXConfig(cfg *settings.Settings) {
	if cfg == nil {
		return
	}
	hc := httpx.Snapshot()
	hc.SkipTLSVerify = cfg.SkipTLSVerify
	hc.RequestTimeout = time.Duration(cfg.RequestTimeoutSecs) * time.Second
	hc.MaxResponseBytes = cfg.MaxResponseMiB * 1024 * 1024
	hc.ProxyURL = cfg.ProxyURL
	hc.DisableRedirects = cfg.DisableRedirects
	if cfg.CACertFile != "" {
		data, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollen: ca_cert_file: %v\n", err)
		} else {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(data) {
				fmt.Fprintf(os.Stderr, "pollen: ca_cert_file: no valid PEM certificates\n")
			} else {
				hc.CACertPool = pool
			}
		}
	}
	if cfg.EnableCookies {
		if jar, err := cookiejar.New(nil); err == nil {
			hc.CookieJar = jar
		}
	}
	httpx.SetConfig(hc)
}

// resolveScenario turns the --run argument into a Scenario: "@file" / "-" parse
// a JSON scenario definition (file / stdin); anything else is looked up by name
// in scenarios.json.
func resolveScenario(arg string) (scenario.Scenario, error) {
	if arg == "-" || strings.HasPrefix(arg, "@") {
		var (
			data []byte
			err  error
		)
		if arg == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(strings.TrimPrefix(arg, "@"))
		}
		if err != nil {
			return scenario.Scenario{}, err
		}
		var sc scenario.Scenario
		if err := json.Unmarshal(data, &sc); err != nil {
			return scenario.Scenario{}, fmt.Errorf("parse scenario JSON: %w", err)
		}
		return sc, nil
	}
	store, err := scenario.Open()
	if err != nil {
		return scenario.Scenario{}, err
	}
	sc, ok := store.ByName(arg)
	if !ok {
		return scenario.Scenario{}, fmt.Errorf("no saved scenario named %q", arg)
	}
	return sc, nil
}

// reportScenarioRun prints a per-step summary of a run and returns true if any
// step failed (so the caller can exit non-zero for CI).
func reportScenarioRun(w io.Writer, sc scenario.Scenario, results []scenario.StepResult) (failed bool) {
	name := sc.Name
	if name == "" {
		name = "(unnamed)"
	}
	fmt.Fprintf(w, "Running scenario %q (%d steps)\n", name, len(sc.Steps))
	passed := 0
	for i, r := range results {
		label := r.Name
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		switch {
		case r.Skipped:
			fmt.Fprintf(w, "  - %s  skipped\n", label)
		case r.Err != "":
			failed = true
			fmt.Fprintf(w, "  x %s  error: %s\n", label, r.Err)
		default:
			stepFailed := r.Failed()
			mark := "ok"
			if stepFailed {
				mark, failed = "x", true
			} else {
				passed++
			}
			fmt.Fprintf(w, "  %s %s  %d  %dms\n", markGlyph(mark), label, r.Response.Status, r.DurationMs)
			for _, a := range r.Asserts {
				if !a.Pass {
					fmt.Fprintf(w, "      assertion failed: %s want %q, got %q\n",
						assertLabel(a.Assertion), a.Assertion.Want, a.Got)
				}
			}
		}
	}
	fmt.Fprintf(w, "%d/%d steps passed\n", passed, len(sc.Steps))
	return failed
}

func markGlyph(mark string) string {
	if mark == "ok" {
		return "✓" // ✓
	}
	return "✗" // ✗
}

func assertLabel(a scenario.Assertion) string {
	if a.Kind == scenario.AssertBody {
		if a.Path != "" {
			return "body." + a.Path + " " + string(a.Op)
		}
		return "body " + string(a.Op)
	}
	return string(a.Kind) + " " + string(a.Op)
}
