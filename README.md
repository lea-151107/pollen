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

Press `?` inside the app for the full list at any time.

### Global

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Move focus between panels |
| `Ctrl+S` | Send request |
| `Ctrl+Y` → `c` / `f` | Copy as cURL / fetch |
| `Ctrl+H` | Toggle history panel |
| `Ctrl+T` | Toggle TLS verification skip (persists) |
| `Ctrl+C` | Quit |
| `?` | Show help overlay |
| `u` | Undo last history delete (within 5 s) |

### Panel-specific

- **History**: `↑/↓` move · `Enter` load entry · `d` delete
- **Method**: `↑/↓` cycle methods
- **Headers**: `↑/↓ ←/→` navigate · `Enter` new row · `Ctrl+D` delete row · `Tab` accept suggestion
- **Body**: `←/→` switch tab · `Enter` enter editor · `Tab` indent (2 spaces) · `Esc` leave editor
- **Response**: `↑/↓ PgUp/PgDn` scroll · `s` save body to file

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

## Variables

Pollen expands `{{name}}` tokens in the URL, header values, and request body
at send time. Define variables in `~/.config/pollen/env.json`:

```json
{
  "vars": {
    "baseUrl": "https://api.example.com",
    "token": "sk-test-abc123"
  }
}
```

Then write requests like:

- URL: `{{baseUrl}}/v1/users`
- Header `Authorization`: `Bearer {{token}}`
- Body: `{ "callback": "{{baseUrl}}/done" }`

Notes:

- Unknown names (no entry in `vars`) are **left untouched** so you can spot
  unresolved references in the response or saved history
- Expansion is single-pass — a value that itself contains `{{...}}` is not
  re-expanded (avoids infinite loops)
- **History stores the expanded form**, so secrets in `env.json` will end
  up in `history.json` once sent. Treat history.json with the same care as
  any file containing credentials

## License

MIT
