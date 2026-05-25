# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
