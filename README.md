# pollen

A terminal UI for testing HTTP APIs — like Postman or Thunder Client, but in
your terminal. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- Send HTTP requests with method, headers, and body (JSON / form-urlencoded / raw)
- Header name autocomplete from a list of common HTTP headers
- Request history (JSON file) with one-key restore and undo-delete
- **Collections**: save named requests (`Ctrl+B`), browse and reload from a sidebar (`Ctrl+K`)
- **jq filter** in the response panel — press `/` to filter JSON in real time
- **Request chaining**: `{{response.body.<path>}}` / `{{response.headers.<n>}}` / `{{response.status}}` expand from the last response
- **Import** OpenAPI 3.x (JSON/YAML) or Postman Collection v2.1 (`Ctrl+I`) into collections
- Copy any request as a POSIX `curl` command or JavaScript `fetch()` call
- Binary response detection with hex dump preview and `s`-to-save
- Optional TLS verification skip for self-signed dev/staging certs
- Configurable response size limit (32 MiB hard cap) and display preview (100 KiB)

## Install

```sh
go build -o pollen .
./pollen
```

Requires Go 1.21+ (uses `sync/atomic.Bool`).

On Linux, the clipboard (`Ctrl+Y`) requires `xclip` or `wl-clipboard`. If
neither is installed, pollen writes the content to `~/.config/pollen/clipboard.txt`
as a fallback.

## Keybindings

Press `Ctrl+/` inside the app for the full list at any time.

### Global

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Move focus between panels |
| `Ctrl+S` | Send request |
| `Ctrl+Y` → `c` / `f` | Copy as cURL / fetch |
| `Ctrl+H` | Toggle history panel |
| `Ctrl+K` | Toggle collections panel |
| `Ctrl+B` | Save current request to collection |
| `Ctrl+I` | Import OpenAPI / Postman file into collections |
| `Ctrl+T` | Toggle TLS verification skip (persists) |
| `Ctrl+C` | Quit |
| `Ctrl+/` | Show help overlay |
| `u` | Undo last history delete (within 5 s) |

### Panel-specific

- **History**: `↑/↓` move · `Enter` load entry · `d` delete · `/` filter (Esc clears)
- **Collections**: `↑/↓` move · `Enter` load entry · `d` delete · `/` filter (Esc clears)
- **Method**: `↑/↓` cycle methods
- **Query**: `↑/↓ ←/→` navigate · `Enter` new row · `Ctrl+D` delete row
- **Auth**: `←/→` switch type (None/Bearer/Basic) · `Enter/↓` edit fields · `Esc/↑` back
- **Headers**: `↑/↓ ←/→` navigate · `Enter` new row · `Ctrl+D` delete row · `Tab` accept suggestion
- **Body**: `←/→` switch tab · `Enter` enter editor · `Tab` indent (2 spaces) · `Esc` leave editor
- **Response**: `↑/↓ PgUp/PgDn` scroll · `s` save body to file · `/` jq filter · `Esc` clear filter

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

The **Query** panel between the URL bar and Headers is a dedicated editor
for URL query parameters. Use it instead of typing `?key=val&key2=val2`
into the URL bar:

- Keys and values are URL-encoded automatically when the request is sent
- If the URL bar already contains a `?...` portion, the panel's entries are
  **merged** with it (existing keys are kept; panel additions are appended)
- When the URL contains `{{var}}` tokens (un-parseable), the parameters are
  concatenated as a string with `?` / `&` separators
- Reloading an entry from history splits its full URL — the parameters land
  back in the Query panel, the URL bar shows only the base URL

## Configuration

| File | Purpose |
|------|---------|
| `~/.config/pollen/history.json` | Request/response history (most-recent first, cap 200) |
| `~/.config/pollen/collections.json` | Named saved requests |
| `~/.config/pollen/settings.json` | Persistent toggles (TLS skip) |
| `~/.config/pollen/env.json` | User-defined variables for `{{name}}` expansion |
| `~/.config/pollen/clipboard.txt` | Clipboard fallback if `xclip`/`wl-clipboard` missing |

History stores binary response **metadata only** — the body bytes are dropped
to keep the JSON readable and small. Reload a binary entry and you'll see the
size/Content-Type only.

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
launch starts in the same environment.

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

Press `Ctrl+B` to save the current request with a name — a dialog prompts for the
name (blank defaults to "Untitled"). Saved entries are stored in
`~/.config/pollen/collections.json`.

Press `Ctrl+K` to toggle the Collections sidebar. Like the History panel, it supports:

- `↑/↓` to move between entries
- `Enter` to load the request into the editor
- `d` to delete the entry
- `/` to filter by name, method, or URL

`Ctrl+H` (History) and `Ctrl+K` (Collections) are mutually exclusive — showing one
closes the other.

You can also populate collections from an existing API spec:

- **OpenAPI 3.x** (`.json` or `.yaml`): each path × method pair becomes one entry.
  Entry names come from `summary` → `operationId` → `METHOD /path`, in that order.
  The first server URL is used as the base. Required query parameters are appended as
  empty placeholders.
- **Postman Collection v2.1** (`.json`): each request item (including nested folders)
  becomes one entry, preserving name, method, URL, headers, and raw body.

Press `Ctrl+I`, enter the file path (supports `~`), and press `Enter` to import.

## License

MIT
