# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
