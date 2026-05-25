# pollen

A terminal UI for testing HTTP APIs — like Postman or Thunder Client, but in
your terminal. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- Send HTTP requests with method, headers, and body (JSON / form-urlencoded / raw)
- Header name autocomplete from a list of common HTTP headers
- Request history (JSON file) with one-key restore and undo-delete
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
| `Ctrl+T` | Toggle TLS verification skip (persists) |
| `Ctrl+C` | Quit |
| `Ctrl+/` | Show help overlay |
| `u` | Undo last history delete (within 5 s) |

### Panel-specific

- **History**: `↑/↓` move · `Enter` load entry · `d` delete · `/` filter (Esc clears)
- **Method**: `↑/↓` cycle methods
- **Query**: `↑/↓ ←/→` navigate · `Enter` new row · `Ctrl+D` delete row
- **Auth**: `←/→` switch type (None/Bearer/Basic) · `Enter/↓` edit fields · `Esc/↑` back
- **Headers**: `↑/↓ ←/→` navigate · `Enter` new row · `Ctrl+D` delete row · `Tab` accept suggestion
- **Body**: `←/→` switch tab · `Enter` enter editor · `Tab` indent (2 spaces) · `Esc` leave editor
- **Response**: `↑/↓ PgUp/PgDn` scroll · `s` save body to file

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

## License

MIT
