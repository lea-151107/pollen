# pollen

A terminal UI for testing HTTP APIs — like Postman or Thunder Client, but in
your terminal. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- Send HTTP requests with method, headers, body (JSON / form-urlencoded / raw),
  and built-in Bearer / Basic auth
- Header name autocomplete from a list of common HTTP headers
- Request history (JSON file) with one-key restore and 5-second undo-delete
- **Collections**: save named requests (`Ctrl+B`), browse/load/rename/delete
  from a sidebar (`Ctrl+K`), update-in-place after loading
- **Variable environments**: `{{varName}}` expansion from `env.json`, multiple
  named environments (dev / staging / prod / …), switch at runtime with `Ctrl+E`
- **Request chaining**: `{{response.body.<jq-path>}}` /
  `{{response.headers.<name>}}` / `{{response.status}}` expand from the last
  response — perfect for login → use-token flows
- **jq filter** (`/` in response panel) for narrowing JSON output
- **In-body search** (`Ctrl+F` in response panel) with live highlight
- **Response diff** (`D`): character-level diff vs the previous response
- **Copy response body** (`y`): clipboard, with a file fallback on Linux
  without `xclip` / `wl-clipboard`
- **Import** OpenAPI 3.x (JSON/YAML) or Postman Collection v2.1 (`Ctrl+I`)
- **Export** all collections to Postman v2.1 JSON (`--export-collections`)
- Binary response detection with hex dump preview, `s`-to-save
- TLS options: skip verification, custom CA certificate file, HTTP(S) proxy,
  cookie jar, redirect control — all toggleable from `settings.json`
- Copy any request as a POSIX `curl` command or JavaScript `fetch()` call
- Terminal-control-character sanitisation: malicious or buggy server output
  can't smuggle ANSI escapes through the renderer

## Install

### From source

```sh
go install github.com/lea-151107/pollen@latest
```

Or build a local checkout (lets you embed a specific version string):

```sh
git clone https://github.com/lea-151107/pollen.git
cd pollen
go build -ldflags="-X github.com/lea/pollen/internal/version.Version=$(git describe --tags)" -o pollen .
./pollen --version
```

### Pre-built binaries

Each release publishes Linux / macOS / Windows binaries (amd64 + arm64) on the
[Releases page](https://github.com/lea-151107/pollen/releases). Download the
archive for your platform and unpack the `pollen` binary into a directory on
your `PATH`.

### Requirements

- Go 1.21+ (only for building from source)
- On Linux, the clipboard (`Ctrl+Y` / `y`) requires `xclip` or `wl-clipboard`.
  Without either, pollen writes the content to `~/.config/pollen/clipboard.txt`
  as a fallback.

## Keybindings

Press `Ctrl+/` inside the app for the full list at any time.

### Global

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Move focus between panels |
| `Ctrl+S` | Send request |
| `Ctrl+Y` → `c` / `f` | Copy request as cURL / fetch |
| `Ctrl+H` | Toggle history panel |
| `Ctrl+K` | Toggle collections panel |
| `Ctrl+B` | Save current request to collection (or update if loaded from one) |
| `Ctrl+I` | Import OpenAPI / Postman file into collections |
| `Ctrl+E` | Switch variable environment |
| `Ctrl+T` | Toggle TLS verification skip (persists) |
| `Ctrl+L` | Force terminal redraw (recover from stray output) |
| `Ctrl+/` | Show help overlay |
| `u` | Undo last history delete (within 5 s) |
| `Ctrl+C` | Quit |

### Panel-specific

- **History**: `↑/↓` move · `G` jump to last · `gg` jump to first · `Enter` load · `d` delete · `/` filter (Esc clears)
- **Collections**: `↑/↓` move · `G` / `gg` jump · `Enter` load · `e` rename · `d` delete · `/` filter (Esc clears)
- **Method**: `↑/↓` cycle methods
- **Query**: `↑/↓ ←/→` navigate · `Enter` new row · `Ctrl+D` delete row
- **Auth**: `←/→` switch type (None/Bearer/Basic) · `Enter/↓` edit fields · `Esc/↑` back
- **Headers**: `↑/↓ ←/→` navigate · `Enter` new row · `Ctrl+D` delete row · `Tab` accept suggestion
- **Body**: `←/→` switch tab · `Enter` enter editor · `Tab` indent (2 spaces) · `Esc` leave editor
- **Response**: `↑/↓ PgUp/PgDn` scroll · `s` save body to file · `y` copy body · `/` jq filter · `Ctrl+F` search in body · `D` toggle diff vs prev

## Authentication

The **Auth** panel between Query and Headers offers three modes:

- **None**: no Authorization header is added
- **Bearer**: enter a token; the request gets `Authorization: Bearer <token>`
- **Basic**: enter username/password; the request gets
  `Authorization: Basic <base64(user:pass)>` (password input is masked)

Use `←/→` on the type-selector row to switch modes, then `Enter`/`↓` to drop
into the fields. `Esc` or `↑` returns to the type selector.

If the **Headers** panel already contains an explicit `Authorization` entry,
the Auth panel does **not** override it — your manual value wins.

Auth state is **session-only**: it's not stored in `~/.config/pollen/`, and
loading a history entry resets the panel to None (Authorization remains in
the restored Headers, so the request still works).

## Query parameters

The **Query** panel between the URL bar and Headers is a dedicated editor for
URL query parameters. Use it instead of typing `?key=val&key2=val2` into the
URL bar:

- Keys and values are URL-encoded automatically when the request is sent
- When the query panel has entries, any `?...` already in the URL bar is
  discarded so the panel is the authoritative source
- When the URL contains `{{var}}` tokens (un-parseable), the parameters are
  concatenated as a string with `?` / `&` separators
- Reloading an entry from history splits its full URL — the parameters land
  back in the Query panel, the URL bar shows only the base URL

## CLI flags

```sh
pollen [--option ...]
```

| Flag | Effect |
|------|--------|
| `--version` | Print the version and exit |
| `--config <dir>` | Use `<dir>` as the config directory instead of `~/.config/pollen` |
| `--env <name>` | Activate the named environment at startup (warning to stderr if unknown) |
| `--collection <name>` | Open the Collections sidebar pre-filtered by this name |
| `--init-config` | Write a default `settings.json` to the config directory and exit |
| `--export-collections <path>` | Export all collections to a Postman v2.1 JSON file. Use `-` to write to stdout |

Examples:

```sh
pollen --env staging                                   # start in staging environment
pollen --config ./myproject/.pollen                    # project-local config
pollen --collection "User API"                         # open with "User API" pre-selected
pollen --init-config                                   # seed default settings.json
pollen --export-collections /tmp/pollen-collections.json
```

## Configuration

All configuration lives in `~/.config/pollen/` (or the directory passed via
`--config`). Files are JSON and are written atomically (`.tmp` + rename) so a
crash mid-write never leaves a corrupt half-file.

| File | Purpose |
|------|---------|
| `history.json` | Request/response history (most-recent first, capped at `history_limit`) |
| `collections.json` | Named saved requests |
| `settings.json` | Persistent toggles and tunables |
| `env.json` | User-defined variables for `{{name}}` expansion |
| `clipboard.txt` | Clipboard fallback if `xclip`/`wl-clipboard` missing |

### settings.json

Run `pollen --init-config` to create the file pre-populated with the defaults
below. Every field is optional; missing or out-of-range values fall back to
the default silently so a partial or corrupt file never blocks startup.

| Field | Default | Notes |
|-------|--------:|-------|
| `skip_tls_verify` | `false` | Toggle at runtime with `Ctrl+T` |
| `response_panel_ratio` | `0.5` | Fraction of available width given to the response panel (0.0–1.0 exclusive) |
| `request_timeout_secs` | `60` | HTTP client timeout (1–600 s) |
| `max_response_mib` | `32` | Response body cap; bytes past this are dropped and `Truncated` is set (1–1024 MiB) |
| `history_limit` | `200` | Maximum entries kept in `history.json` (1–10000) |
| `text_preview_kib` | `100` | Display truncation threshold for text bodies; `s` still saves the full body (1–10240 KiB) |
| `sidebar_max_width` | `40` | Maximum column width of the history/collections sidebar (20–200) |
| `hex_dump_kib` | `4` | Bytes shown in the hex dump preview for binary bodies (1–1024 KiB) |
| `proxy_url` | `""` | When non-empty, routes all requests through this HTTP(S) proxy |
| `disable_redirects` | `false` | When `true`, returns 3xx responses as-is instead of following them |
| `ca_cert_file` | `""` | Path to a PEM file with extra trusted CAs (safer than `skip_tls_verify` for internal/self-signed certs) |
| `enable_cookies` | `false` | When `true`, cookies set by a response are replayed in subsequent requests within the same session |

History stores binary response **metadata only** — the body bytes are dropped
to keep the JSON readable and small. In-memory body bytes are also dropped
from history entries past the 10 most recent prepends to bound memory growth.
Reload a binary entry from outside that window and you'll see the size /
Content-Type only.

## Variables and environments

Pollen expands `{{name}}` tokens in the URL, header values, and request body
at send time, looking up the name in the **currently active environment**
from `~/.config/pollen/env.json`:

```json
{
  "current": "dev",
  "environments": {
    "dev": {
      "baseUrl": "http://localhost:8080",
      "token": "dev-token"
    },
    "prod": {
      "baseUrl": "https://api.example.com",
      "token": "sk-live-abc123"
    }
  }
}
```

Use the variables in any request:

- URL: `{{baseUrl}}/v1/users`
- Header `Authorization`: `Bearer {{token}}`
- Body: `{ "callback": "{{baseUrl}}/done" }`

Switch environments at runtime with **`Ctrl+E`** — a menu lists every
environment defined in env.json. The selection is persisted, so the next
launch starts in the same environment. `--env <name>` at startup overrides
the persisted selection for that session.

The status bar shows the active environment as `[env: dev]` whenever any
environment is selected.

Notes:

- The v0.1.0 flat-`vars` format is migrated automatically into a single
  `default` environment on first load
- Unknown names (no entry in the active env) are **left untouched** so you
  can spot unresolved references in the response or saved history
- Expansion is single-pass — a value that itself contains `{{...}}` is not
  re-expanded (avoids infinite loops)
- **History stores the expanded form**, so secrets in `env.json` end up in
  `history.json` once sent. Treat `history.json` with the same care as any
  file containing credentials

### Request chaining

After receiving a response, reference its values in the next request using
`{{response.*}}` tokens. These are evaluated *after* env-variable tokens:

| Token | Value |
|-------|-------|
| `{{response.body.<path>}}` | jq path applied to the last JSON response body |
| `{{response.body}}` | whole response body as a string |
| `{{response.headers.<name>}}` | response header value (case-insensitive name) |
| `{{response.status}}` | HTTP status code as a string, e.g. `"200"` |

Example — log in, then use the token in the next request:

1. `POST {{baseUrl}}/auth/login` with credentials in the body
2. Next request: header `Authorization: Bearer {{response.body.token}}`

If no previous response exists, or the jq path produces no match, the token is
left untouched.

## Collections

Press `Ctrl+B` to save the current request with a name — a dialog prompts for
the name (blank defaults to "Untitled"). Saved entries are stored in
`~/.config/pollen/collections.json`.

Press `Ctrl+K` to toggle the Collections sidebar. Like the History panel, it
supports:

- `↑/↓` to move between entries (also `G` / `gg` for jump-to-end / start)
- `Enter` to load the request into the editor
- `e` to rename the selected entry
- `d` to delete the entry
- `/` to filter by name, method, or URL

`Ctrl+H` (History) and `Ctrl+K` (Collections) are mutually exclusive — showing
one closes the other.

### Editing saved requests

- Press **`e`** on a selected entry to rename it — a dialog pre-fills the
  current name
- Load an entry with `Enter`, then modify the request and press **`Ctrl+B`**
  to choose:
  - **Enter** — update the loaded entry in-place
  - **n** — save as a new entry (original is unchanged)

### Importing from a spec

Populate collections from an existing API spec:

- **OpenAPI 3.x** (`.json` or `.yaml`): each path × method pair becomes one
  entry. Entry names come from `summary` → `operationId` → `METHOD /path`, in
  that order. The first server URL is used as the base. Required query
  parameters are appended as empty placeholders.
- **Postman Collection v2.1** (`.json`): each request item (including nested
  folders) becomes one entry, preserving name, method, URL, headers, raw body
  and url-encoded form body.

Press `Ctrl+I`, enter the file path (supports `~`), and press `Enter` to
import. See `examples/` for sample input files.

### Exporting collections

```sh
pollen --export-collections collection.json    # write to file
pollen --export-collections -                  # write to stdout (for piping)
```

The output is a Postman Collection v2.1 JSON document. Form-urlencoded bodies
are serialised as Postman's `urlencoded` array, raw bodies as `mode: raw`.

## Versioning and stability

Pollen follows [Semantic Versioning](https://semver.org). Starting at v1.0.0,
the following surfaces are part of the public, semver-covered API:

- **CLI flags and their semantics** (everything documented in *CLI flags*)
- **Configuration file schemas** (`settings.json`, `env.json`,
  `history.json`, `collections.json`) — fields may be **added** in minor
  releases; removing or renaming a field requires a major bump
- **Keybindings** — additions in minor releases are fine; changing or
  removing a binding requires a major bump
- **Behaviour of variable expansion and request chaining** —
  the `{{name}}` / `{{response.*}}` token syntax is stable

The following are **not** semver-covered and may change in any release:

- The Go package layout under `internal/` (this is an end-user tool, not a
  library)
- Status-line wording, log messages, and UI colours
- Terminal rendering details (spacing, borders, suggestion ordering)
- The on-disk format of `clipboard.txt`

Deprecation policy: a CLI flag or settings field marked for removal will
continue to function — with a `pollen: ...` warning to stderr — for at least
one minor release before being removed in the following major release.

## License

MIT
