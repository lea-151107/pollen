# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.1] - 2026-05-30

### Fixed

- **Intruder template validation now rejects markers that reference
  payload positions beyond the configured count.** Previously, a
  Sniper run with `{{$payload}}` and a stray `{{$payload2}}` in the
  same URL passed pre-flight validation and dispatched a request
  with `{{$payload2}}` left in as a literal — the same silent
  failure could happen in Pitchfork / ClusterBomb when a template
  referenced `{{$payload5}}` but only 3 positions were configured.
  The runner now returns a clear pre-flight error naming the
  offending marker. The most common trigger was switching modes or
  reducing Positions without cleaning up the template.
- **HTTP requests no longer silently fall through to the env
  proxy when `proxy_url` in settings.json is malformed.** A typo
  in `proxy_url` used to be swallowed by `url.Parse`, leaving the
  default transport's `ProxyFromEnvironment` in place — so a
  non-empty `HTTP_PROXY` / `HTTPS_PROXY` would route the request
  via the env proxy, the opposite of what the user configured.
  Pollen now forces a direct connection on parse failure, and
  emits a clear stderr warning at startup so the parse error is
  visible before the first request goes out.

## [1.3.0] - 2026-05-29

### Added

- **Two new Intruder attack modes: Pitchfork and ClusterBomb.**
  - **Pitchfork** assigns N payload lists to N marker positions and
    iterates them in parallel ("zip"), stopping when any list
    exhausts. Use it for credential pairs, ordered probes, anything
    where lists are pre-aligned.
  - **ClusterBomb** assigns N lists to N positions and enumerates
    the Cartesian product. Use it for combinatorial discovery (path
    × method, user × pass, etc). The product is capped by
    `intruder_max_requests` so a 1000×1000 input still terminates.
  - Both modes support up to 8 payload positions, configured in the
    Intruder modal with the new `Mode` and `Positions` rows.
- **Multi-position marker syntax.** Mark each payload position with
  `{{$payload1}}`, `{{$payload2}}`, …, `{{$payloadN}}`. `{{$payload}}`
  continues to work and is treated as an alias for `{{$payload1}}`,
  so every v1.2.x request template runs unchanged under Sniper. The
  runner validates that all markers a chosen mode needs are present
  pre-flight, naming any missing positions in the error.

### Changed

- `RunConfig.Payload` (single `PayloadConfig`) becomes
  `RunConfig.Payloads` ([]PayloadConfig). The `intruder` package is
  not part of the SemVer-frozen public surface, but downstream Go
  consumers of the package (if any) will need to wrap their existing
  value in `[]PayloadConfig{p}`.
- Sniper's behaviour and the Sniper UI are unchanged — the modal
  still opens to the same single-payload form, and `{{$payload}}`
  with multiple occurrences in the template still gets the same
  value at every position (equivalent to Burp's Battering ram, as
  before).
- Multi-position runs join the per-position payloads with ` | ` in
  the result table's `payload` column and in `--export-intruder`
  CSV/JSON. Sniper rows display the payload bare, matching v1.2.x.

### Fixed

- The Intruder result-table filter input now accepts multi-byte
  characters (CJK kanji, accented Latin, …). The v1.2.1 default
  branch checked byte length and silently dropped any rune longer
  than 1 byte, leaving international payloads unfilterable.
- When the filter excludes every result, the table now shows
  `(no results match filter)` instead of the misleading
  `(waiting for first response…)` message, which previously made
  the run look stuck.

## [1.2.1] - 2026-05-29

### Added

- **Sort the Intruder result table.** Press `s` in the results
  overlay to cycle the sort column (`#` → `status` → `size` → `ms`
  → `#`); the active column shows a ▲ / ▼ marker in the header so
  the current order is visible at a glance. `Shift+S` reverses the
  direction on the current column. The default direction is
  ascending for `#` (matching v1.2.0's send-order behaviour) and
  descending for the numeric columns so the largest, slowest, or
  highest-status rows surface first.
- **Filter the Intruder result table.** Press `/` to open a
  payload-substring filter (case-insensitive, matches anywhere in
  the payload). Press `f` to cycle a status-class preset: `All`
  → `Errors (4xx/5xx + network errors)` → `Success (2xx)` → `All`.
  The two filters compose, and the header shows a
  `(N/M shown · …)` badge whenever filtering is active so the
  effect is obvious.

### Changed

- The Intruder results overlay's `Esc` key now applies three layers:
  inside the filter input it cancels the input, with an active filter
  outside input mode it clears the filter only, and with no filter
  active it aborts the run as before. This matches the existing
  three-layer `Esc` behaviour in the History panel.
- The result table now truncates and pads by visual width
  (lipgloss.Width) instead of byte length, so the new ▲ / ▼
  header markers and any CJK characters in payloads or
  content-types display without cutting UTF-8 sequences in half.

## [1.2.0] - 2026-05-29

### Added

- **Intruder mode** (`Ctrl+R`). Pollen now ships a Burp Suite Sniper
  workflow scoped to a single payload position: fire the current
  request template against a generated payload list using a worker
  pool, with a live result table that streams responses as they
  complete.
  - Mark the payload position with the reserved token `{{$payload}}`
    anywhere in the URL, body, or header values. The token survives
    `{{varName}}` env expansion and `{{response.*}}` chaining
    unchanged, so an Intruder run can compose with the existing
    expanders.
  - Four payload generators: `Range` (`1-100` or `1-100/5`), `List`
    (comma-separated or `@/path/to/wordlist`), `Brute` (`<alphabet>
    <min>-<max>`, lexicographic), and `CaseToggle` (all 2^L
    upper/lower permutations of the ASCII letters in a base string).
  - Worker pool with configurable concurrency, per-worker delay, and a
    hard cap on total requests. `Esc` cancels in-flight workers via
    `context.Cancel`; in-flight HTTP requests respect the existing
    `request_timeout_secs` setting.
  - Live result table with `#`, payload, status, size, ms,
    content-type columns. 4xx, 5xx, and network-error rows are
    highlighted in red. `↑/↓ PgUp/PgDn` scroll a windowed view sized
    to the terminal height.
- **`--export-intruder <path>`** CLI flag. Exports the most recent
  run's results as CSV (default) or JSON (when `<path>` ends in
  `.json`). Use `-` for CSV on stdout. Exits with status 2 if no run
  has ever been recorded in the config directory.
- **Three new `settings.json` fields** with the existing clamp-on-load
  pattern: `intruder_concurrency` (default 5, clamped to [1, 256]),
  `intruder_delay_ms` (default 0, clamped to [0, 60000]), and
  `intruder_max_requests` (default 1000, clamped to [1, 1000000]).
  Existing files load unchanged; the defaults populate the Intruder
  config modal so a one-off run never needs the user to type the
  knobs by hand.

### Notes

- The marker syntax (`{{$payload}}`), the four payload kinds, the
  Intruder keybinding (`Ctrl+R`), and the three new `settings.json`
  fields are part of pollen's stable v1.x surface — they won't be
  removed or renamed in a minor release. The exact wording in the
  result table header and any colour tweaks remain unstable, as
  declared by the SemVer policy in the README.
- Only the most recent run is persisted to disk (in
  `~/.config/pollen/intruder_last.json`). Older runs are not kept;
  the file is overwritten on every completed run.
- v1.2.0 ships Sniper mode only — Battering ram / Pitchfork / Cluster
  bomb (multi-position payload distribution) are reserved for a
  future release.

[1.2.0]: https://github.com/lea-151107/pollen/releases/tag/v1.2.0

## [1.1.0] - 2026-05-29

### Added

- **`--export-postman <path>`**: new CLI flag that writes the
  collection store as a Postman v2.1 JSON document. Behaviour is
  byte-identical to the existing `--export-collections`; the new name
  exists so the two export flags read symmetrically — `--export-postman`
  and `--export-openapi` are both named by their target format.

### Deprecated

- **`--export-collections <path>`** is now an alias for
  `--export-postman` and is kept for backwards compatibility. No warning
  is printed at runtime so existing scripts keep working unchanged.
  Specifying both `--export-postman` and `--export-collections` at the
  same time is an error and exits with status 2; pick one.

No other behaviour changed: CLI semantics, JSON config schemas,
keybindings, and variable-expansion tokens all match v1.0.4 exactly.

[1.1.0]: https://github.com/lea-151107/pollen/releases/tag/v1.1.0

## [1.0.4] - 2026-05-29

### Changed

- **Unified wording across the TUI**. Panel hints, modal dialogs, and the
  help overlay now use one consistent style: key names are Title-Case
  (`Enter`, `Ctrl+D`, `Esc`, `Tab`), items are joined with the same
  middle-dot separator (`·`), and the `Key: action` form is used
  throughout. Concretely:
  - Query / Headers hints switched from lowercase `enter: new row` to
    Title-Case `Enter: new row`.
  - Auth / Body hints switched the `•` separator to `·` and added the
    explicit `Key: action` colon so they parse the same as Query /
    Headers.
  - Help overlay key names are derived through a normaliser, so binding
    tokens stored as `ctrl+s` render as `Ctrl+S`. The one inline
    outlier (`ctrl+f` in the Response section) was fixed to match.
  - Modal titles lost their trailing punctuation
    (`Update collection entry?` → `Update collection entry`,
    `Copy request as:` → `Copy request`).
  - The env switcher's instruction line gained colons to match the
    other modals (`Enter confirm` → `Enter: confirm`).
- **Response panel state lines no longer repeat the panel name**. While
  loading / on error / before the first request, the panel rendered
  `Response: loading…`, `Response: error`, `Response: (no requests
  yet)`. The successful-response branch already omitted the prefix
  (rendering the status line directly), so the prefix is now dropped
  from the idle paths too.
- **`pollen:` prefix is now uniform on stderr**. The two startup paths
  that previously emitted `failed to load history: ...` /
  `failed to load collections: ...` now read
  `pollen: history: ...` / `pollen: collections: ...`, matching every
  other error in `main.go`.
- **TLS-toggle status uses the state form even on save failure**.
  Previously the save-failed branch said `TLS toggled (...)` while the
  success branches said `TLS verification: ON/OFF`; the save-failed
  branch now also leads with `TLS verification: ...`.

### Fixed

- **Response panel no longer leaves a one-row gap below itself**. The
  `vpH` calculation in `internal/ui/response.go` was off by one because
  the `"\n"` separator between the status line and the viewport was
  being counted as an extra row (and `filterBarH` / `searchBarH` carried
  the same mistake). `lipgloss.Height` treats `"\n"` as a line break
  rather than an additional row, so the viewport reserved one row fewer
  than it should and the rendered Response column ended one row shorter
  than the request column — the status bar accordingly sat one row
  below where it should have. The calculation now sums only the actual
  rendered rows, so the Response panel's bottom border sits flush
  against the status bar.

### Documentation

- Every `internal/*` package now carries a `// Package <name> ...` doc
  comment. Previously only 4 of the 12 packages had one.

No protocol behaviour changed: CLI flags, JSON keys, keybindings, and
variable-expansion tokens all match v1.0.3 exactly. The visual changes
are limited to the wording and layout points above.

[1.0.4]: https://github.com/lea-151107/pollen/releases/tag/v1.0.4

## [1.0.3] - 2026-05-29

### Added

- **OpenAPI 3.x export**. `--export-openapi <path>` writes the current
  collection store as an OpenAPI 3.0.3 document. Format is picked by
  extension: `.yaml` / `.yml` produce YAML, anything else (including `-`
  for stdout) produces JSON. When every entry shares a single
  `scheme://host` it is emitted as `servers[0].url` and paths are
  relative; mixed or template-tokenised hosts skip the `servers` block
  and keep the raw URL as the path key. Headers and URL query strings
  become `parameters` with `in: header` / `in: query` and an `example`
  carrying the stored value. Body content is emitted under the natural
  media type (`application/json`, `application/x-www-form-urlencoded`,
  or `text/plain` / the explicit `Content-Type` header). **No header
  masking is performed** — `Authorization`, `Cookie`, and any other
  sensitive headers appear in the exported spec, so review before
  sharing.

### Fixed

- **Layout no longer shifts when focus moves between request-column
  panels**. `Query`, `Auth`, and `Headers` previously appended a one-row
  hint only when focused, and `Body` concatenated its hint onto the
  tab-bar row in a way that could wrap on narrow widths. Together these
  let each panel's height oscillate by one row as Tab moved through
  them, and on short terminals the body's `bodyH < 4` clamp let the
  request column overflow past `contentH` and push the status bar out
  of view. Each panel now always reserves the hint row (rendered blank
  when unfocused), and `Body` places its hint on its own row with the
  textarea height reduced by one, so panel heights are independent of
  focus state.

[1.0.3]: https://github.com/lea-151107/pollen/releases/tag/v1.0.3

## [1.0.2] - 2026-05-29

### Fixed

- **`go install` now works**. The module path declared in `go.mod`
  (`github.com/lea/pollen`) did not match this repository's location on
  GitHub (`github.com/lea-151107/pollen`), so
  `go install github.com/lea-151107/pollen@latest` failed with a module
  path mismatch error. The module declaration and every internal import
  have been renamed to `github.com/lea-151107/pollen`, and the ldflags
  example in the README and the `-X` flag in `.goreleaser.yml` now use
  the corrected `github.com/lea-151107/pollen/internal/version.Version`
  path.

### Notes

- Source-only change: the produced binary's runtime behaviour is
  unchanged from v1.0.1.
- v1.0.1 and earlier remain uninstallable via `go install` because their
  embedded `go.mod` still carries the old module declaration. Use
  v1.0.2 or later for `go install`, or build from source.

[1.0.2]: https://github.com/lea-151107/pollen/releases/tag/v1.0.2

## [1.0.1] - 2026-05-27

### Fixed

- **CI: cross-platform test stability**. `TestDir_UsesXDGConfigHome`
  asserted Linux-specific `os.UserConfigDir` behaviour (the env var
  `XDG_CONFIG_HOME`) and failed on the macOS and Windows runners; it now
  skips on non-Linux. Other test helpers that previously redirected via
  `XDG_CONFIG_HOME` — and would silently write into the runner's real
  config directory on macOS/Windows — now go through
  `userconfig.SetOverride`, which works on every platform.
  `userconfig.SetOverride("")` now correctly clears the override (was
  treated as `"."` by `filepath.Clean`), so test cleanup actually resets
  state between cases.

No user-visible runtime behaviour changed. The pollen binary itself is
byte-equivalent to v1.0.0.

[1.0.1]: https://github.com/lea-151107/pollen/releases/tag/v1.0.1

## [1.0.0] - 2026-05-27

First stable release. The CLI, configuration file schemas, keybindings, and
variable-expansion syntax are now covered by Semantic Versioning — see the
*Versioning and stability* section of the README for what is and isn't part
of the stable surface.

### Stable

- **CLI flags** (`--version`, `--config`, `--env`, `--collection`,
  `--init-config`, `--export-collections`)
- **`settings.json` schema** (12 fields: `skip_tls_verify`,
  `response_panel_ratio`, `request_timeout_secs`, `max_response_mib`,
  `history_limit`, `text_preview_kib`, `sidebar_max_width`, `hex_dump_kib`,
  `proxy_url`, `disable_redirects`, `ca_cert_file`, `enable_cookies`)
- **`env.json` schema** with multiple named environments and the v0.1.0
  flat-`vars` migration path
- **`history.json` and `collections.json` schemas** with their `version: 1`
  envelope
- **Keybindings** documented in the README and the in-app help overlay
  (`Ctrl+/`)
- **Variable-expansion syntax**: `{{name}}` for env vars,
  `{{response.body.<jq-path>}}` / `{{response.headers.<name>}}` /
  `{{response.status}}` for request chaining

### Added

- Pre-built binaries published on each release for Linux / macOS / Windows
  (amd64 + arm64) via `goreleaser`
- GitHub Actions CI runs `go test` / `go vet` / `go build` on every push and
  pull request across Linux, macOS, and Windows
- `examples/` directory with sample OpenAPI spec, Postman collection, and
  reference `settings.json` / `env.json`

### Notes

- No breaking changes from v0.6.4. Existing config files load unchanged.
- v0.1.0 flat-`vars` env.json files are still auto-migrated to the
  per-environment format on load.

[1.0.0]: https://github.com/lea-151107/pollen/releases/tag/v1.0.0

## [0.6.4] - 2026-05-27

### Fixed

- **Terminal-control sanitisation: cover the remaining display paths**:
  v0.6.3 sanitised response bodies, headers, and the diff view, but
  `Response.SetError` (network/HTTP error messages — also re-displayed
  from history via `applyEntry`), the `binaryHeader` Content-Type
  readout, and Content-Disposition filenames written by the `s` save
  action still passed raw bytes through. A server returning an
  `\x1b`-laced Content-Type, error text, or filename could clear the
  terminal or reposition the cursor. All three paths are now sanitised —
  display paths via the existing `sanitizeTerminalControl` helper,
  filenames via a `_` replacement applied before writing to disk so the
  on-disk name and the "saved to ..." status message agree

[0.6.4]: https://github.com/lea-151107/pollen/releases/tag/v0.6.4

## [0.6.3] - 2026-05-27

### Fixed

- **Response panel: search overlay no longer hides filter / diff content**:
  enabling the jq filter (`/`) or diff mode (`D`) and then typing in the
  in-body search bar caused the viewport to revert to the raw body with
  search highlights, even though the filter bar and diff badge stayed
  visible. The search overlay now composes with the active base content
  (filter > diff > plain), so search highlights are applied inside the
  filtered or diff view rather than replacing it
- **Diff badge hidden when filter overrides display**: with both diff mode
  and a locked jq filter active, the `[diff]` badge stayed visible even
  though the viewport showed the filtered content. The badge now only
  appears when the diff view is actually shown
- **Shift+Tab in body editor cycled focus**: Tab in the body editor inserts
  two spaces of indent, but Shift+Tab triggered the global Prev-Focus
  shortcut and dropped the user out of editor mode. Shift+Tab is now a
  no-op in the editor (reserved for a future un-indent action)
- **Terminal-control character sanitisation**: response bodies and header
  values containing C0 / DEL / C1 bytes (e.g. ANSI escape sequences from a
  buggy or malicious server) are now rendered as `\xHH` placeholders
  instead of being passed through to the terminal, where they could
  clear the screen or reposition the cursor
- **BodyBytes memory growth bounded**: `BodyBytes` (in-session raw bytes
  used by the `s` save action) are now dropped from entries older than the
  10 most recent prepends, preventing unbounded memory accumulation
  (previously up to `max_response_mib × history_limit` ≈ 6.4 GiB by
  default). Text bodies remain savable from any entry via a Body string
  fallback; binary bodies can only be re-saved from the 10 most recent

[0.6.3]: https://github.com/lea-151107/pollen/releases/tag/v0.6.3

## [0.6.2] - 2026-05-26

### Fixed

- **Response jq filter / search bar swallowed by global `u` and `s`**: when
  the response panel had an active jq filter (`/`) or in-body search (`Ctrl+F`),
  the global `u` shortcut (undo last history delete) consumed the `u`
  keystroke instead of inserting it into the input, and the panel-local `s`
  shortcut (save response body) similarly intercepted `s`. Filters like
  `.users` or `.servers` were unreachable while the undo window was open or
  while the response panel had focus. The text-editing focus guard now treats
  `focusResponse` as text-editing when either input is active, mirroring how
  History/Collections filter modes are already handled
- **Diff toggle dropped search highlights**: with a locked search query
  (Enter pressed after typing in the search bar), toggling diff off cleared
  the search overlay even though the search bar still displayed the query.
  The diff toggle's "off" branch now defers to `currentDisplayBody`, restoring
  the documented `search > filter > diff > plain` priority
- **Postman export `"item": null` for empty collections**: exporting an empty
  collection produced `"item": null` instead of the spec-required `"item": []`,
  which strict Postman v2.1 parsers may reject

[0.6.2]: https://github.com/lea-151107/pollen/releases/tag/v0.6.2

## [0.6.1] - 2026-05-26

### Fixed

- **Postman form body roundtrip**: exporting a collection containing form
  bodies via `--export-collections` and re-importing the resulting JSON
  produced empty bodies. The exporter now writes Postman v2.1's `urlencoded`
  array (instead of stuffing the form pairs into `raw`), and the importer
  accepts both `raw` and `urlencoded` modes
- **Response headers panel — spurious "(+ 0 more headers)" row**: when the
  response had exactly 5 headers, the truncation note appeared with a zero
  count; it now only appears when there are genuinely additional headers to
  hide
- **`Response.SetResponse` nil safety**: the diff-mode branch dereferenced
  the freshly-stored response without a nil check; callers all guarded
  against nil, but the function itself now does too
- **History/Collections filter highlight — Unicode case mapping**: characters
  whose lowercase form has a different byte width (e.g. U+212A KELVIN SIGN
  → `k`) misaligned the highlight slice on the original text, producing
  garbled output. The matcher now operates rune-by-rune

[0.6.1]: https://github.com/lea-151107/pollen/releases/tag/v0.6.1

## [0.6.0] - 2026-05-26

### Added

- **`--version` flag**: prints `pollen <version>` and exits. The version string
  is embedded at build time via `-ldflags="-X github.com/lea/pollen/internal/version.Version=v0.6.0"`
- **Proxy support** (`proxy_url` in `settings.json`): routes all requests through
  the specified HTTP/HTTPS proxy (e.g. `"http://localhost:8080"` for mitmproxy)
- **Redirect control** (`disable_redirects` in `settings.json`): when `true`,
  the HTTP client returns the redirect response as-is instead of following it —
  useful for inspecting OAuth 302s or POST-redirect sequences
- **Custom CA certificate** (`ca_cert_file` in `settings.json`): path to a PEM
  file containing trusted CA certificates; a safer alternative to `skip_tls_verify`
  for self-signed or internal TLS endpoints
- **Cookie jar** (`enable_cookies` in `settings.json`): when `true`, cookies set
  by a response are stored and replayed in subsequent requests within the same session
- **Copy response body** (`y` in Response panel): copies the current body to the
  clipboard (jq-filtered body when a filter is active). Falls back to
  `~/.config/pollen/clipboard.txt` when no clipboard tool is available
- **In-body search** (`Ctrl+F` in Response panel): opens a search bar at the
  bottom of the panel; matching text is bold+underlined as you type. `Enter`
  locks the highlight; `Esc` clears it
- **`--export-collections`**: exports all saved collections to a Postman
  Collection v2.1 JSON file. Pass `-` to write to stdout.
  Example: `pollen --export-collections /tmp/my-collection.json`

[0.6.0]: https://github.com/lea-151107/pollen/releases/tag/v0.6.0

## [0.5.2] - 2026-05-26

### Fixed

- **`--help` flag display**: options are now shown with `--` prefix (double hyphen)
  instead of the Go `flag` package default of `-` (single hyphen)

[0.5.2]: https://github.com/lea-151107/pollen/releases/tag/v0.5.2

## [0.5.1] - 2026-05-26

### Added

- **`--init-config`**: new startup flag that creates `~/.config/pollen/settings.json`
  with all default values and exits. Combine with `--config` to target a custom
  directory. Exits with an error if the file already exists (delete it first to reset).

[0.5.1]: https://github.com/lea-151107/pollen/releases/tag/v0.5.1

## [0.5.0] - 2026-05-26

### Added

- **Response right panel**: the response viewer is now a persistent right-side
  panel instead of sharing vertical space with the body editor. The body editor
  expands to fill all remaining height. The split ratio defaults to 50 % and
  can be changed by setting `"response_panel_ratio"` (e.g. `0.6`) in
  `~/.config/pollen/settings.json`
- **History / Collections — G / gg jump**: press `G` to jump to the last
  entry; press `g g` (twice) to jump to the first. Matches vim-style navigation
- **Ctrl+L**: force-redraws the terminal (useful after the screen is garbled
  by another program's output)
- **Configurable settings** — six new fields in `settings.json`:
  - `request_timeout_secs` (default 60): HTTP request timeout
  - `max_response_mib` (default 32): maximum response body to buffer
  - `history_limit` (default 200): maximum entries kept in `history.json`
  - `text_preview_kib` (default 100): threshold above which bodies switch to hex view
  - `sidebar_max_width` (default 40): maximum column width of the history / collections sidebar
  - `hex_dump_kib` (default 4): bytes shown in the hex dump preview

### Fixed

- **Env variable expansion**: variable names containing hyphens (e.g. `{{API-KEY}}`) were silently skipped and sent verbatim; now expanded correctly
- **TRACE method**: importing an OpenAPI or Postman collection with `TRACE` endpoints and then selecting them left the method picker on its previous value; `TRACE` is now a recognised method
- **settings.json corruption**: if `settings.json` contained invalid JSON the app started with zero-valued settings (no timeout, 0-byte response limit). It now falls back to defaults in all error cases
- **`fetch()` export — duplicate Content-Type**: when the user had a lowercase `content-type` header and a structured body, the generated `fetch()` snippet contained two conflicting `Content-Type` keys. The check is now case-insensitive, matching the existing `curl` export behaviour
- **Basic auth whitespace**: leading/trailing spaces in the username or password fields of the Basic auth panel were included in the Base64-encoded credential; they are now trimmed
- **Postman import — string URL form**: Postman v2.1 allows the `url` field as either a plain string or an object with a `raw` key; only the object form was previously handled, causing import failures for collections that use the string form
- **URL bar minimum width**: setting the URL bar width to a negative value when the terminal was very narrow caused a panic in the underlying text input; now clamped to zero
- **Filter backspace with multi-byte characters**: pressing Backspace in the history/collections filter while the filter contained CJK or emoji characters removed one byte instead of one character, producing invalid UTF-8; now uses rune-aware slicing
- **Filter accepting named keys as text**: keys such as `↑`, `ctrl+a`, and function keys were appended to the filter string in filter mode; only single printable characters are now accepted
- **pendingG reset on Blur and SetEntries**: a `g` keypress followed by losing focus or a list refresh could combine with the next `g` in the re-focused panel to trigger an accidental `gg` (jump-to-top); `pendingG` is now reset in all relevant transitions
- **`.tmp` cleanup on rename failure**: `userconfig.SaveJSON` did not remove the `.tmp` file when `os.Rename` failed, leaving orphaned files in the config directory
- **Collections update prompt — deleted entry**: selecting "update in-place" for a collection entry that had been deleted between save and confirm now shows an error status instead of silently doing nothing

[0.5.0]: https://github.com/lea-151107/pollen/releases/tag/v0.5.0

## [0.4.3] - 2026-05-25

### Fixed

- **Query parameter doubling**: when the query component had params and the
  URL bar also contained a query string, both were merged causing the same
  key to appear twice. The query component is now the authoritative source
  when non-empty; any existing query string in the URL bar is discarded
- **Auth panel blocked by history-restored header**: loading a request from
  history restored the `Authorization` header into the Headers component,
  preventing the Auth panel from overriding it on subsequent sends. The Auth
  panel (Bearer/Basic) now always takes precedence and removes any existing
  `Authorization` header before adding its own value
- **Postman import empty URL fallback**: when a Postman item had no name and
  no URL, the generated name was `"GET "` (with trailing space); now falls
  back to `"GET (no URL)"`

[0.4.3]: https://github.com/lea-151107/pollen/releases/tag/v0.4.3

## [0.4.2] - 2026-05-25

### Fixed

- **Response diff auto-exit on binary**: diff mode now turns off when a
  binary response arrives, instead of showing a spurious "everything deleted"
  diff with the `[diff]` badge still visible
- **Postman import unnamed entries**: items with an empty `name` field now
  fall back to `"METHOD URL"`, matching the OpenAPI importer's behaviour
- **Store atomic write**: `history.json` and `collections.json` now clean up
  the `.tmp` file when `os.Rename` fails, preventing temp file accumulation

[0.4.2]: https://github.com/lea-151107/pollen/releases/tag/v0.4.2

## [0.4.1] - 2026-05-25

### Fixed

- **Response diff + jq filter**: opening the jq filter (`/`) while diff mode was
  active and then pressing `Esc` left the `[diff]` badge visible but the viewport
  showing the plain body. `resetFilter` now restores the diff view when diff mode
  is on
- **Binary diff**: pressing `D` when either the current or previous response is
  binary no longer produces a meaningless diff — the key press is silently ignored
- **Save-as-new pre-fill**: pressing `n` in the update-in-place prompt (Ctrl+B)
  could open the save dialog with leftover text from a previous session; the input
  is now cleared before focusing
- **`SetFilter` defensive**: `SetFilter()` (used by `--collection` startup flag)
  now explicitly resets `filterMode` to false

[0.4.1]: https://github.com/lea-151107/pollen/releases/tag/v0.4.1

## [0.4.0] - 2026-05-25

### Added

- **Filter match highlighting**: History and Collections panels now bold+underline
  the matching substring in each filtered row, making it easy to spot why an entry
  was included
- **Collection rename** (`e` key in Collections panel): opens a dialog pre-filled
  with the current name; Enter saves, Esc cancels
- **Collection update-in-place** (Ctrl+B after loading an entry): prompts to update
  the loaded entry with the current request (Enter) or save as a new entry (n)
- **Response diff** (`D` in Response panel): toggles a character-level diff of the
  previous response body against the current one — additions in green, deletions in
  red strikethrough. Stays active across successive sends. Requires 2+ responses
- **CLI startup flags**:
  - `--config <dir>` — use an alternate config directory instead of `~/.config/pollen`
  - `--env <name>` — activate the named environment at startup
  - `--collection <name>` — open the Collections sidebar pre-filtered by name

### Fixed

- Toggling response diff OFF while a jq filter was locked now correctly restores
  the filtered view instead of showing the full body
- Collection name column color no longer resets after a highlighted filter match
  (was caused by ANSI nesting; fixed by per-segment styling)
- `lastLoadedCollID` is now cleared after "save as new", preventing a stale update
  prompt on the next Ctrl+B press

[0.4.0]: https://github.com/lea-151107/pollen/releases/tag/v0.4.0

## [0.3.0] - 2026-05-25

### Added

- **jq filter** (Response panel, `/`): filter a JSON response with a jq expression
  in real time. `Enter` locks the filter and re-enables scrolling; `Esc` restores
  the original body
- **Collections**: save the current request with a name (`Ctrl+B`), then browse,
  load, and delete entries from a sidebar (`Ctrl+K`). Stored in
  `~/.config/pollen/collections.json`. Supports `/` filter and is mutually exclusive
  with the History panel
- **Request chaining**: reference the last response in the next request via
  `{{response.body.<jq-path>}}`, `{{response.headers.<name>}}`, and
  `{{response.status}}` tokens — evaluated after env-variable expansion
- **Import** (`Ctrl+I`): load endpoints from an OpenAPI 3.x spec (JSON or YAML) or
  a Postman Collection v2.1 (JSON) directly into collections

### Fixed

- Response panel overflowed by one line while the jq filter bar was visible
  (height calculation subtracted 1 instead of 2 for the bar)
- Toggling one sidebar with `Ctrl+H`/`Ctrl+K` while the other sidebar was focused
  left focus on the now-hidden panel, causing keystrokes to be swallowed silently

[0.3.0]: https://github.com/lea-151107/pollen/releases/tag/v0.3.0

## [0.2.0] - 2026-05-25

Quality-of-life release focused on environment management, request
ergonomics, and response readability — plus a round of internal
restructuring to clean up cross-layer coupling.

### Added

- **Variable expansion**: `{{varName}}` tokens in the URL, header values,
  and request body are substituted at send time from a `~/.config/pollen/env.json`
  store
- **Multiple environments**: env.json now holds named environments
  (`dev`/`staging`/`prod`/...) with a current selection; switch in-app
  via `Ctrl+E` (selection is persisted)
- **Query parameter editor**: a dedicated panel between the URL bar and
  Headers; keys/values are URL-encoded automatically and merged with any
  `?...` already in the URL (or string-concatenated when the URL contains
  `{{var}}`)
- **Authentication panel**: None / Bearer / Basic, with password masked.
  The Authorization header is built only at send time and yields to any
  explicit Authorization the user wrote in Headers
- **JSON pretty-print**: response bodies with `application/json` (or any
  `+json` subtype per RFC 6839) are re-indented to 2 spaces for display.
  Save (`s`) still writes the original bytes
- **History filter**: press `/` while focused on History to type a
  case-insensitive substring (matched against `METHOD URL`); `Esc` clears
- **status badge `[env: <name>]`** in the status bar whenever any
  environment is selected

### Changed

- **Help overlay shortcut: `?` → `Ctrl+/`**. `?` is a valid character in
  URLs and headers; the old binding needed an awkward text-edit guard
  that the new control-sequence avoids entirely
- **env.json schema**: v0.1.0's flat `{"vars": {...}}` is automatically
  migrated into `{"current": "default", "environments": {"default": {...}}}`
  on the next load
- **History delete**: now identifies entries by ID (was: by UI index), so
  delete works correctly while a filter is active

### Internal

These are pure refactors — no behaviour change, but the package layout is
cleaner than v0.1.0:

- New `internal/userconfig` package owns `~/.config/pollen/` path
  resolution (was duplicated across 4 packages) and atomic JSON storage
  (was duplicated across `settings` and `env`)
- `app/update.go` split from 482 lines into focused files
  (`url.go`, `request.go`, `overlays.go`, plus the existing
  `save.go`/`clipboard.go` gaining their relevant methods)
- `internal/httpx` is now closer to a pure HTTP layer:
  - HexDump moved out to `internal/ui` (it's presentation, not HTTP)
  - Authorization wire format (Bearer prefix, Base64 encoding) moved
    *in* from `ui/auth.go` as `httpx.BuildAuthHeader`
- `view.go` reads TLS-insecure state from the Model instead of reaching
  into `httpx`'s package global directly

### Fixed

- Multiple in-flight requests no longer race: a stale older response can
  no longer overwrite the latest one (gated by a per-Send generation
  counter)
- Corrupt `history.json` no longer prevents startup; the app warns to
  stderr and starts with an empty history (file preserved for inspection)
- `Authorization` header now uses `http.DefaultTransport.Clone()` when
  TLS-skip is enabled, preserving proxy/keepalive/HTTP-2 defaults
- The text-response preview-truncation notice now appears at the **top**
  of the viewport, not after the 100 KiB of content

[0.2.0]: https://github.com/lea-151107/pollen/releases/tag/v0.2.0

## [0.1.0] - 2026-05-25

Initial release. A terminal-UI HTTP client for testing APIs (Postman/Thunder
Client style), built with Go and Bubble Tea.

### Added

#### Core HTTP

- Send HTTP requests with method (`GET`/`POST`/`PUT`/`PATCH`/`DELETE`/`HEAD`/`OPTIONS`),
  URL, custom headers, and body
- Body editor with three modes: JSON, form-urlencoded, and raw text
- Tab inserts two spaces inside the body editor (JSON-friendly indent)
- Automatic `Content-Type` fallback when one isn't set explicitly
- 60-second request timeout
- Asynchronous send via `tea.Cmd` so the UI stays responsive

#### Response handling

- Status line shows status code (colored by class), elapsed ms, body size,
  and Content-Type
- Binary response detection via Content-Type plus 512-byte content sniff
  (NUL bytes, UTF-8 validity, non-printable ratio)
- Hex dump preview (xxd-style, first 4 KiB) for binary bodies
- Text responses larger than 100 KiB are display-truncated; a notice at the
  top tells the user the full body is still available via save
- Press `s` to save the response body to a file in the current directory.
  Filename derived from `Content-Disposition` → URL path → `response.bin`,
  with an extension appended from `mime.ExtensionsByType` when missing
- 32 MiB hard cap on response body read; truncation is surfaced in the UI

#### History

- Persistent request/response history at `~/.config/pollen/history.json`
  (atomic writes, capped at 200 entries, most recent first)
- Left panel shows colored status badge, method, URL, and relative time
  ("3m ago", "1h ago")
- `Enter` loads an entry back into the request builder
- `d` deletes the focused entry; `u` undoes the deletion within 5 seconds
- Binary response bodies are stored as metadata only — the bytes are dropped
  to keep the JSON small and human-readable
- Corrupt `history.json` no longer prevents startup; the app warns to stderr
  and starts with an empty history (the file is preserved for inspection)

#### Productivity

- Header name autocomplete from a curated list of ~70 common HTTP headers,
  showing up to 5 suggestions inline; `Tab` accepts the first
- `Ctrl+Y` opens a copy menu: press `c` to copy the request as a POSIX
  `curl` command, `f` for a JavaScript `fetch()` call
- `?` opens a keybinding help overlay (compact layout for narrow terminals)
- Dynamic status-bar hints surface panel-specific keys (`s: save`,
  `↑↓: cycle method`)

#### Configuration

- `Ctrl+T` toggles TLS certificate verification skip for testing
  self-signed/internal endpoints. The setting persists across sessions in
  `~/.config/pollen/settings.json`, and a red `[TLS: insecure]` badge stays
  visible in the status bar while skip is active
- When `xclip`/`wl-clipboard` is unavailable on Linux, copy actions fall
  back to writing `~/.config/pollen/clipboard.txt`

#### Safety / correctness

- All response bytes are validated/sniffed before being written into the
  JSON history (text-MIME with invalid UTF-8 is downgraded to binary so the
  history file isn't mangled by `U+FFFD` replacements)
- TLS-skip transport is cloned from `http.DefaultTransport`, preserving
  proxy, keep-alive, and HTTP/2 defaults
- Concurrent in-flight requests can't clobber each other: older responses
  whose request was superseded by `Ctrl+S` again are discarded
- Status toasts are tagged with a generation counter so a stale clear-tick
  can't wipe a newer message
- Save filename is derived from the response's *original* request URL, not
  whatever the URL bar holds at the moment `s` is pressed

[0.1.0]: https://github.com/lea-151107/pollen/releases/tag/v0.1.0
