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
- **Export** all collections to Postman v2.1 JSON (`--export-postman`) or
  OpenAPI 3.x JSON / YAML (`--export-openapi`)
- **Intruder** (`Ctrl+R`): fire the current request against a generated
  payload list (numeric range, wordlist, brute force, or case toggles),
  with configurable concurrency and a live result table. Inspired by
  Burp Suite's Sniper mode
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
go build -ldflags="-X github.com/lea-151107/pollen/internal/version.Version=$(git describe --tags)" -o pollen .
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
| `Ctrl+R` | Open Intruder (concurrent requests against a payload list) |
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
| `--export-postman <path>` | Export all collections to a Postman v2.1 JSON file. Use `-` to write to stdout |
| `--export-collections <path>` | Alias for `--export-postman`, kept for backwards compatibility |
| `--export-openapi <path>` | Export all collections as an OpenAPI 3.x document. Format is picked by extension (`.yaml` / `.yml` → YAML, otherwise JSON). Use `-` for JSON on stdout |
| `--export-intruder <path>` | Export the most recent Intruder run as CSV (default) or JSON (when `<path>` ends in `.json`). Use `-` for CSV on stdout. Exits with status 2 if no run has been recorded yet |

Examples:

```sh
pollen --env staging                                   # start in staging environment
pollen --config ./myproject/.pollen                    # project-local config
pollen --collection "User API"                         # open with "User API" pre-selected
pollen --init-config                                   # seed default settings.json
pollen --export-postman /tmp/pollen-collections.json
pollen --export-openapi /tmp/pollen-openapi.yaml      # OpenAPI 3.0.3 in YAML
pollen --export-intruder /tmp/intruder.csv            # last Intruder run
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

Postman v2.1:

```sh
pollen --export-postman collection.json    # write to file
pollen --export-postman -                  # write to stdout (for piping)
```

The output is a Postman Collection v2.1 JSON document. Form-urlencoded bodies
are serialised as Postman's `urlencoded` array, raw bodies as `mode: raw`.

`--export-collections` is kept as an alias for `--export-postman` so existing
scripts keep working. Specifying both flags at the same time is an error.

OpenAPI 3.x:

```sh
pollen --export-openapi api.yaml               # YAML (extension picks format)
pollen --export-openapi api.json               # JSON
pollen --export-openapi -                      # JSON on stdout
```

The output is OpenAPI 3.0.3. When every entry shares one `scheme://host`, that
host is emitted as `servers[0].url` and each entry's path is relative; mixed or
template-tokenised hosts (`{{baseURL}}/users/...`) skip the `servers` block and
keep the raw URL as the path key. Headers and URL query strings become
`parameters` entries with an `example` field set to the stored value. Body
contents are emitted under the natural media type — `application/json` for JSON
bodies, `application/x-www-form-urlencoded` for form bodies (with each pair as a
typed property), `text/plain` (or any explicit `Content-Type` header) for raw
bodies. **No header masking is performed**: `Authorization`, `Cookie`, and other
sensitive headers will appear in the exported spec, so review before sharing.

## Intruder

Press `Ctrl+R` to open the Intruder modal. It fires the current request
template against a generated sequence of payloads using a worker pool —
think Burp Suite's Sniper attack mode, scoped to one payload position.

Mark where the payload should go with the reserved token `{{$payload}}`
anywhere in the URL, body, or header values:

```
URL:    https://api.example.com/users/{{$payload}}
Body:   {"id": "{{$payload}}"}
Header: X-API-Key: {{$payload}}
```

`{{$payload}}` survives `{{varName}}` and `{{response.*}}` expansion
unchanged, so you can combine env variables, response chaining, and an
Intruder run in the same request.

Pollen ships three attack modes; the config modal's first row picks
which (`←` / `→` to cycle):

| Mode | Lists × positions | Iteration |
|------|-------------------|-----------|
| **Sniper** | 1 list × any positions | Same payload substituted at every marker (Burp's "Battering ram" for free — mark multiple positions and each receives the same value) |
| **Pitchfork** | N lists × N positions | Zip (request `k` uses list1[k], list2[k], …, lN[k]); stops at the shortest list |
| **ClusterBomb** | N lists × N positions | Cartesian product (every combination, bounded by `intruder_max_requests`) |

For Pitchfork / ClusterBomb the second row picks the number of payload
positions (2–8). Mark them in the request as `{{$payload1}}`,
`{{$payload2}}`, …, up to the position count you chose. `{{$payload}}`
is preserved as an alias of `{{$payload1}}` so existing v1.2.x
templates still run unchanged under Sniper.

```
URL:    https://api.example.com/login?user={{$payload1}}&pass={{$payload2}}
Body:   {"token":"{{$payload3}}"}
```

In each payload position pick a kind with `←` / `→` and enter its
parameters. Format strings appear inline below the input.

| Kind | Format | Example |
|------|--------|---------|
| `Range` | `<from>-<to>` or `<from>-<to>/<step>` | `1-100/5` |
| `List` | `a,b,c` or `@/path/to/wordlist` | `admin,root,guest` |
| `Brute` | `<alphabet> <min>-<max>` | `abc 1-3` |
| `CaseToggle` | `<base>` | `admin` (16 permutations of upper/lower) |

`Concurrency`, `Delay (ms)`, and `Max requests` are pre-filled from
`settings.json`. Press `Enter` to start; the modal becomes a live table
that streams in as workers complete each request:

- `↑/↓ PgUp/PgDn` move the row cursor (the ▶-marked row); `g` / `G`
  jump to the first / last row
- **`Enter`** opens a per-result detail view showing the full HTTP
  response (status, headers, body) for that row. `↑/↓ PgUp/PgDn`
  scroll the body; `Esc` returns to the table. The body is body-cap
  truncated by `intruder_response_body_cap_kib` (default 64 KiB)
  so a 1000-payload run doesn't pin GiBs of RAM; when truncated a
  hint at the bottom says so
- 4xx rows are tinted yellow, 5xx and network-error rows red
- Outlier rows whose size deviates by more than 50% from the
  median (of the visible / filtered set) get a `!` marker on
  their size cell
- `s` cycles the sort column (`#` → `status` → `size` → `ms` → `#`).
  The active column shows a ▲ / ▼ marker. `Shift+S` reverses the
  current direction
- **`/`** opens a filter prompt. The DSL is small but composable:

  | Token | Meaning |
  |-------|---------|
  | `admin` | payload substring contains "admin" (case-insensitive) |
  | `size:>1000`, `size:<100`, `size:1000-2000`, `size:>=1024` | size range |
  | `dur:>500`, `dur:<10`, `dur:100-300` | duration in milliseconds |
  | `s:404`, `s:4xx`, `s:>=500`, `s:200-299` | status range |

  Tokens are AND-composed: `/admin size:>=1000 s:4xx` keeps rows
  where payload contains "admin" AND size ≥ 1000 AND status is 4xx.
  `Enter` commits, `Esc` drops. `f` cycles a status preset
  (All → Errors → 2xx → All) independent of the DSL
- **`e`** opens an in-app CSV export prompt with a timestamped
  default path; `Enter` saves, `Esc` cancels
- `Esc` (with no filter active) cancels the run and closes the
  overlay (the most recent run is still cached on disk for
  `--export-intruder`)

To export the most recent run from outside the TUI:

```sh
pollen --export-intruder results.csv     # CSV (default for stdout too)
pollen --export-intruder results.json    # JSON, indented
pollen --export-intruder -               # CSV on stdout
```

If no Intruder run has ever finished in the same config directory, the
command exits with status 2.

## GraphQL

Pollen has first-class support for GraphQL requests as one of the body
tabs. In the Body panel, `←` / `→` cycle through `JSON / FORM / RAW /
GRAPHQL`; on the GraphQL tab the editor area splits into a larger
**query** pane on top and a smaller **variables (JSON)** pane below.

`Ctrl+G` (in editor mode) toggles focus between query and variables.
`Tab` indents inside whichever pane is focused; `Esc` leaves editor mode
as usual.

At send time pollen wraps the two panes in the canonical envelope:

```json
{
  "query": "query ($id: ID!) { user(id: $id) { name email } }",
  "variables": { "id": 42 }
}
```

…and POSTs it with `Content-Type: application/json`. Variables that
don't parse as JSON are silently omitted from the envelope (the server
will surface the error). The query and variables both go through env
expansion and response chaining, so you can write things like

```
variables: {"token": "{{authToken}}", "after": "{{response.body.cursor}}"}
```

…and they're resolved before the request leaves pollen.

Intruder runs on GraphQL templates: the `{{$payload}}`, `{{$payload1}}`,
… markers are recognised inside the variables pane too, so
fuzzing GraphQL inputs is just an ordinary Intruder run.

cURL / fetch exports build the envelope into the `--data` / `body`
field; Postman v2.1 export/import round-trip the GraphQL body using
the spec's native `{"mode": "graphql", "graphql": {"query": "...",
"variables": "..."}}` shape.

## multipart/form-data

Pollen's body editor has a `MULTIPART` tab alongside JSON / Form / Raw
/ GraphQL. Each line describes one part:

```
name=value                  text part
upload=@/path/to/file       file upload (default content-type)
img=@/path/x.png;type=image/png   file upload with explicit content-type
```

At send time pollen streams the file parts through `mime/multipart`,
auto-generating the boundary and setting `Content-Type:
multipart/form-data; boundary=...` for you. cURL export uses `-F`
flags (`-F 'meta=...' -F 'upload=@/tmp/x.png;type=image/png'`) so the
exported command actually performs the upload. fetch export builds a
`FormData` object with placeholder file references and a comment
pointing at the original path (browser `fetch` needs a real `File`
handle from an `<input>` that pollen can't materialise from a
filesystem path).

Postman v2.1 export and import roundtrip the multipart body using
the spec's native `{"mode": "formdata", "formdata": [...]}` shape.

Intruder markers (`{{$payload}}`, `{{$payload1..N}}`) and dynamic
variables (`{{$uuid}}`, `{{$timestamp}}`, ...) work inside the
multipart line-based DSL — typically in text values, but file paths
can include them too if you want per-iteration file names.

## cURL paste import

A `--import-curl` flag converts a curl command into a collections
entry without launching the TUI. Three input modes:

```sh
pollen --import-curl 'curl -X POST https://api.example.com -d body'    # literal
pollen --import-curl @/tmp/cmd.txt                                     # file
pollen --import-curl -                                                 # stdin
```

Supported curl flags: `-X / --request`, `-H / --header`, `-d /
--data / --data-raw / --data-binary`, `--data-urlencode`, `-F /
--form` (multipart), `-u / --user` (becomes a Basic-auth header),
`-A / --user-agent`, `-e / --referer`, `--cookie / -b`, `-G /
--get`. Transport flags (`-L`, `-k`, `-s`, `-v`, `-i`, plus the
clumped form `-sLv`) are silently dropped. Unsupported flags exit
1 with an error so the user knows to enter them by hand.

Method inference: `-X` explicit wins; `-G` forces GET; any data
flag implies POST; otherwise GET. A `Content-Type: application/json`
header promotes the inferred body to BodyJSON so the editor opens
on the right tab.

The new entry is named `<METHOD> <URL>` and appended to
`collections.json`. Successful import prints `imported as <name>`
to stderr and exits 0.

## Dynamic variables

Pollen expands a small set of pollen-computed `{{$name}}` tokens at
send time. Unknown names are passed through unchanged, so the existing
intruder marker `{{$payload}}` continues to work alongside these.

| Token | Replaced with |
|-------|---------------|
| `{{$timestamp}}` | Unix epoch seconds |
| `{{$timestamp_ms}}` | Unix epoch milliseconds |
| `{{$datetime}}` | RFC3339 UTC timestamp |
| `{{$uuid}}` | UUID v4 |
| `{{$random}}` | random uint32 |
| `{{$random:N}}` | random 0..N-1 |
| `{{$random:M-N}}` | random M..N (inclusive) |
| `{{$base64:VALUE}}` | base64-encode VALUE |
| `{{$urlencode:VALUE}}` | URL-encode VALUE |

Each request gets fresh values, so `{{$uuid}}` in an Intruder template
yields a different UUID per iteration (the runner expands dynvars in
the worker loop, not at template build time). Useful for
correlation IDs, idempotency keys, timestamp-based queries, and
rate-limit testing.

Expansion order is **env vars → response chaining → dynamic vars**, so
an env value that embeds `{{$uuid}}` resolves correctly at send time.

## OAuth 2.0 (Client Credentials)

The Auth panel's type selector now includes **OAuth**. Selecting it
exposes four input rows — Token URL, Client ID, Client Secret, Scope —
plus an action row at the bottom showing the current token state.
Pressing `g` on the action row runs the OAuth 2.0 Client Credentials
flow (RFC 6749 §4.4): pollen POSTs `grant_type=client_credentials` to
the token URL with the credentials in a Basic-Auth header and parses
the JSON response.

On success, the action row shows the masked token, time-to-expiry, and
"press g to refresh". Pollen then injects
`Authorization: Bearer <access_token>` on every Send while the OAuth
type is selected. Errors (network, bad credentials, missing
`access_token`) surface inline with the server's `error_description`
when present.

Tokens fetched here are written to disk by default since v1.6.4
— see *Token persistence* below.

## OAuth 2.0 Authorization Code with PKCE

The Auth panel's type selector adds a fifth option, **OAuth AC**, for
the OAuth 2.0 Authorization Code grant with PKCE
(RFC 6749 §4.1, RFC 7636, RFC 8252). It exposes six input rows —
Auth URL, Token URL, Client ID, Client Secret (optional), Redirect
URI, Scope — plus an action row.

Pressing `g` on the action row generates a 256-bit `state` and a
PKCE `code_verifier` / S256 `code_challenge`, starts a tiny HTTP
server bound to the loopback host and port of the Redirect URI, then
opens the user's default browser at the authorization endpoint.
When the IdP redirects back to the loopback callback, pollen
validates `state`, exchanges the `code` for an access token at the
Token endpoint (sending `code_verifier`), and stores the resulting
token. Esc cancels an in-flight flow. The whole flow has a 5-minute
timeout.

Redirect URI defaults to `http://127.0.0.1:8765/callback`, which
matches what most IdPs let you register once and forget. Pollen
only supports **loopback** redirects (`127.0.0.1`, `::1`,
`localhost`) on `http://` with an explicit port — non-loopback
hosts and custom schemes are refused. This follows RFC 8252's
recommendation for native apps.

Public clients (no Client Secret) are supported: when Secret is
left blank, pollen omits HTTP Basic auth on the token exchange and
includes `client_id` in the form body instead.

Browser launch uses `open` on macOS, `rundll32 url.dll,…` on
Windows, and `wslview` (if present) or `xdg-open` on Linux/WSL.
If the launch fails the flow still runs; the URL is reachable
manually if you copy it from the auth panel.

### Auto-refresh on send

When the active auth type is OAuth (CC) or OAuth AC, the current
access token is within 30 seconds of expiry, and a refresh token
was issued, pollen issues a refresh before sending and sends with
the new token transparently. Refresh failure aborts the send and
the status line prompts you to re-authorize. Skipped silently for
non-OAuth auth types or tokens without a `refresh_token`.

### Token persistence (v1.6.4+)

Successful OAuth fetches (CC and AC) and refreshes are written
to `~/.config/pollen/oauth_tokens.json` with **mode 0600**
(owner read/write only). On next start, when the Auth panel
contains a matching Token URL + Client ID, pollen automatically
hydrates the access token + refresh token from disk — for
Authorization Code that means no second browser dance, and for
Client Credentials it means the in-memory cache survives a
restart.

Entries are keyed by `(token_url, client_id, grant)`. CC and AC
tokens for the same IdP/client coexist. Scope is stored in each
entry but not part of the key; re-fetching with a different
scope overwrites the prior entry.

To forget the persisted token for the current Token URL + Client
ID, press `d` on the Auth panel's action row (the same row
where `g` triggers a fetch / refresh). The on-disk entry is
removed and the in-memory token is cleared. A status toast
confirms.

To disable persistence entirely, set
`"oauth_persist_tokens": false` in `settings.json`. The default
is `true` (opt-out). When disabled, pollen neither reads nor
writes the token file — the file may still exist on disk from a
prior session, but it's left untouched.

The auto-refresh-on-send path from v1.6.0 still works: a
hydrated-but-expired token gets refreshed transparently before
the next Send and the refreshed token is written back to
`oauth_tokens.json`.

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
