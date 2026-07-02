# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.9.1] - 2026-07-03

### Fixed

- **Cancelling a running scenario leaked the runner goroutine.**
  The scenario runner streams step results over an unbuffered
  channel. Pressing Esc during a run cancelled the context and
  stopped the UI from reading further results, but the runner's
  channel sends were not context-aware, so once the in-flight
  request returned the goroutine blocked forever on its next send
  — leaking one goroutine plus the retained response / step
  outputs per cancelled run. Sends now select on the cancellation
  and the runner exits promptly; `stopScenarioRun` also bumps the
  run generation so a result already in flight is discarded
  instead of applied to the (now hidden) overlay.

### Notes

- No user-facing surface change; a fix to the v1.9.0 Scenarios
  feature. Adds `internal/scenario`
  `TestRunStream_CancelUnblocksProducer` as a deterministic
  regression guard, and removes dead mouse hit-testing code
  (`panelRects` / `rect` / `contains`) left unused in
  `internal/app`.

[1.9.1]: https://github.com/lea-151107/pollen/releases/tag/v1.9.1

## [1.9.0] - 2026-07-02

### Added

- **Scenarios — multi-request workflows.** Chain several requests into
  an ordered scenario that shares one variable context. A later step can
  reference an earlier step's response with `{{steps.<name>.body.<jq>}}`,
  `{{steps.<name>.status}}`, or `{{steps.<name>.headers.<name>}}`;
  `{{response.*}}` continues to mean "the immediately preceding step".
  Steps carry optional status/body assertions. Open the overlay with
  `Ctrl+G` to build a scenario from your saved collection entries, run it,
  and watch a live per-step result table. Scenarios persist in
  `scenarios.json`. Reuses the same expansion chain as the request editor
  (env → step/response chaining → dynamic vars) and the same `httpx`
  client, so proxy / TLS / cookie-jar settings — notably a shared cookie
  jar for login flows — apply to a run.
- **Headless scenario runner `--run`.** `pollen --run <name|@file|->`
  runs a scenario without the TUI and exits non-zero on any failure, for
  use in CI (a built-in Newman-style runner). `<src>` is a saved scenario
  name, an `@file` scenario JSON definition, or `-` for stdin. Honours
  `--env`.
- **Mouse support (on by default).** Click a panel to focus it, click a
  history / collections row to load it, and scroll the response body with
  the wheel (the terminal's SGR mouse mode). Toggle it live from the
  settings overlay (`Ctrl+,` → "Enable mouse"), or via `enable_mouse` in
  `settings.json`.

### Changed

- The `{{response.*}}` path grammar (`status`, `body`, `body.<jq>`,
  `headers.<name>`) moved into a shared `internal/respvars` package so the
  request editor and the scenario runner interpret a path identically.
  No behaviour change to existing `{{response.*}}` tokens.

### Notes

- Mouse support is **on by default**. SGR mouse mode overrides the
  terminal's own text selection / copy (hold `Shift` to select while it is
  on); turn it off from the settings overlay (`Ctrl+,`) or with
  `enable_mouse: false` if you prefer native selection.

[1.9.0]: https://github.com/lea-151107/pollen/releases/tag/v1.9.0

## [1.8.2] - 2026-07-02

### Fixed

- **The WebSocket handshake ignored the shared cookie jar.**
  With `enable_cookies` on, HTTP requests share a cookie jar
  (`httpx.Config.CookieJar`), but `httpx.NewHandshakeClient`
  built its `*http.Client` with no `Jar`. A session cookie
  set by a prior HTTP login was therefore never sent on the
  WebSocket upgrade, and the 101 response's `Set-Cookie` was
  never stored — so a cookie-authenticated WebSocket to a
  host you had already logged into over HTTP failed the
  handshake. The handshake client now carries `cfg.CookieJar`,
  matching `httpx.Do`; `Client.Timeout` stays unset since a
  WebSocket outlives its handshake.

### Notes

- No user-facing surface change; a fix to the v1.8.0
  WebSocket feature. Adds `internal/wsconn`
  `TestDial_CarriesSharedCookieJar` as a regression guard.

[1.8.2]: https://github.com/lea-151107/pollen/releases/tag/v1.8.2

## [1.8.1] - 2026-07-01

### Fixed

- **Disconnecting a WebSocket could freeze the TUI for up
  to ~5 seconds.** `closeWebSocket` (Esc) and the stale-
  connection cleanup called `wsconn.Conn.Close()` directly
  on the Bubble Tea update goroutine. coder/websocket's
  `Close` performs a close handshake that blocks until the
  peer replies or an internal timeout (~5s) elapses, so
  against an unresponsive server the whole UI hung. The
  network close now runs off the UI goroutine via a
  `tea.Cmd`; the overlay closes and `wsConn` is cleared
  immediately while the handshake completes in the
  background.
- **A WebSocket connection could leak if you cancelled
  while it was still connecting.** The handshake runs
  asynchronously (up to a 30s dial timeout). Pressing Esc
  during connect closed the overlay, but the later-arriving
  dial result was still stored — a live socket left running
  behind a hidden overlay — and stale read-pump events
  could even clear a *newer* connection. Connection
  attempts are now tagged with a generation counter
  (`wsGen`, mirroring `requestGen`): results from a
  cancelled or superseded attempt are discarded, and a
  stale successful dial is closed instead of stored.

### Changed

- CI: pinned `goreleaser-action` to `~> v2` (matching
  `.goreleaser.yml`'s `version: 2`, silencing the `latest`
  warning) and bumped `actions/checkout` → v5 and
  `actions/setup-go` → v6 off the deprecated Node 20
  runtime. Because setup-go@v6 defaults `GOTOOLCHAIN=local`,
  the test matrix's stale `'1.21'` entry (which go.mod's
  `go 1.26.3` never actually let run — it silently
  auto-upgraded under v5) is replaced by `'1.26.3'`, the
  real supported minimum.

### Notes

- Also hardened `internal/httpx`
  `TestConfig_ConcurrentSetAndDo` to wait for its writer
  goroutine before returning, so it can no longer leak into
  a later test and flip the shared config under it.
- No user-facing surface change; purely fixes to the v1.8.0
  WebSocket feature plus CI maintenance.

[1.8.1]: https://github.com/lea-151107/pollen/releases/tag/v1.8.1

## [1.8.0] - 2026-07-01

### Added

- **WebSocket sessions (Ctrl+W).** pollen can now open a
  live WebSocket connection alongside its HTTP tooling.
  Ctrl+W pops a connect form prefilled from the URL bar;
  the request editor's headers and Auth panel (Bearer /
  Basic / any OAuth grant) ride along on the handshake, and
  `{{var}}` / response-chaining expansion applies to the
  URL and header values exactly as for an HTTP send. Once
  connected, a fullscreen session view logs frames as they
  arrive — outgoing `▶`, incoming `◀`, system/error lines,
  each timestamped — with a send box (Enter sends) and a
  scrollable log (↑/↓, PgUp/PgDn; it auto-follows the tail
  until you scroll up). Esc disconnects and closes. The
  RFC 6455 close / peer-close / error states are surfaced
  distinctly. Built on `github.com/coder/websocket`; the
  dial reuses the shared httpx transport snapshot, so proxy
  and TLS-skip / custom-CA settings apply to the handshake
  too. This is a session-only MVP: connections and their
  message logs are not persisted, and saving WebSocket
  endpoints to collections, auto-reconnect, ping/pong
  visualization, and binary-frame send are deliberately
  deferred to a later release.

### Notes

- v1.x SemVer-frozen surface: this is purely additive. Like
  `AuthOAuthAC` / `AuthOAuthDC` before it, WebSocket support
  adds a new overlay and the Ctrl+W binding without
  changing any existing settings key, key binding, or
  persistence format; existing `collections.json`,
  `settings.json`, and `oauth_tokens.json` load unchanged.
- New dependency: `github.com/coder/websocket`.
- New packages/files: `internal/wsconn` (connection layer),
  `internal/ui/websocket.go` (overlay), and
  `internal/app/websocket.go` (glue), each mirroring the
  Intruder overlay's state-machine + channel-to-tea.Msg
  pattern.

[1.8.0]: https://github.com/lea-151107/pollen/releases/tag/v1.8.0

## [1.7.5] - 2026-07-01

### Fixed

- **Data race on the `httpx` transport globals.** Since
  v1.7.2 the notes have flagged that `applySettings` (plus
  the Ctrl+T TLS toggle and startup) wrote the httpx
  transport options — `RequestTimeout`, `MaxResponseBytes`,
  `ProxyURL`, `DisableRedirects`, `CACertPool`, `CookieJar`
  — as plain package variables on the UI goroutine while
  the Send goroutine read the very same variables inside
  `httpx.Do`. Only `SkipTLSVerify` was atomic; the rest
  raced, so a settings change mid-request could tear a
  request's transport configuration. All of them are now
  folded into a single immutable `httpx.Config` snapshot,
  swapped atomically via `httpx.SetConfig` and read once
  per request through an `atomic.Pointer`. Each `httpx.Do`
  call loads one consistent snapshot for its whole
  lifetime. Callers that change one field (Ctrl+T,
  `applySettings`, startup) Snapshot → mutate → SetConfig;
  writes are serialized by the single UI goroutine, so no
  compare-and-swap is needed. `applySettings` starts from
  the current snapshot so the startup-only CACertPool /
  CookieJar survive a live settings reapply.

### Changed

- Internal API only: the `httpx` package variables are
  replaced by `httpx.Config` + `httpx.SetConfig` /
  `httpx.Snapshot`. No user-facing surface change.

### Notes

- v1.x SemVer-frozen surface unchanged: no settings, key
  bindings, or persistence formats changed.
- Adds `internal/httpx` `TestConfig_ConcurrentSetAndDo`,
  which drives `SetConfig` and `Do` concurrently; run with
  `-race` (needs a cgo-capable toolchain) to catch
  regressions.
- Closes the deferred httpx data-race fix plan carried in
  the v1.7.2 / v1.7.3 / v1.7.4 notes.

[1.7.5]: https://github.com/lea-151107/pollen/releases/tag/v1.7.5

## [1.7.4] - 2026-06-07

### Fixed

- **Ctrl+P closed the Settings overlay while a field
  editor was active, discarding the in-progress edit.**
  v1.7.1 added Ctrl+P as the Settings overlay binding. The
  global Settings case in `handleKey` intercepts that key
  while the overlay is open and calls `Close()` before
  delegating to the panel's own `Update`, and that
  interception had no "am I editing?" guard. The Settings
  panel otherwise deliberately blocks accidental close
  during editing — `q` types into the field and Esc only
  exits the editor (two-step exit, enforced by
  `TestSettings_EscFromEditDiscards` /
  `TestSettings_QClosesFromNavigate`) — so Ctrl+P (and the
  `Ctrl+,` alias) was the one key that punched through that
  protection, silently throwing away whatever the user had
  typed. This mirrors the v1.7.2 fix that guarded the
  overlay-OPEN path against stealing bubbles textarea /
  textinput Ctrl+P; the symmetric overlay-CLOSE path was
  missed. The close shortcut now carries a
  `SettingsPanel.IsEditing()` guard: while editing, Ctrl+P
  falls through to the field editor (a no-op for the plain
  textinput) and the edit survives; from navigation mode
  Ctrl+P still toggles the overlay closed.

### Notes

- v1.x SemVer-frozen surface unchanged: no settings, key
  bindings, or persistence formats changed; the Settings
  binding's reach is narrowed (it no longer fires while a
  field editor is active), not extended.
- Adds the first app-layer test that drives `Model.Update`
  with key messages
  (`internal/app/update_settings_keys_test.go`); the
  routing layer had no key-driven coverage before, which is
  why this regression slipped through.
- The `applySettings` httpx package-globals data race
  flagged in v1.7.2 / v1.7.3 is unchanged; the deferred fix
  plan (Snapshot pattern or atomic primitives in `httpx`)
  still stands.

[1.7.4]: https://github.com/lea-151107/pollen/releases/tag/v1.7.4

## [1.7.3] - 2026-05-31

### Added

- **Two action buttons at the top of the Ctrl+/ help
  overlay.** Until v1.7.3 the help was an informational
  accordion only: ↑/↓ moved between sections, Enter
  expanded or collapsed them. The new "Open Settings"
  button opens the Settings overlay (same path as
  Ctrl+P), and the "Reset settings to defaults" button
  resets every `settings.json` key to its built-in
  default after a y/n confirmation. The destructive
  button switches the help body to a confirmation view
  ("Reset all settings to their default values?"); Y
  runs the reset, N or Esc cancels. A status-bar toast
  ("settings reset to defaults") confirms the reset.
- **`settings.Defaults()`** returns the canonical default
  `*Settings`, factored out of `Load()` so startup and
  the new in-TUI reset share the same source of truth.

### Changed

- **Help overlay focus spans buttons + sections in a
  single ↑/↓ cycle.** Cursor starts on the first button
  when Ctrl+/ opens. Enter on a focused button activates
  it; Enter on a focused section header still expands or
  collapses that section. g/G, j/k, PgUp/PgDn navigate
  across both groups.
- **Help overlay footer hint** updated to reflect the new
  Enter semantics: `Enter: run / expand`.

### Notes

- v1.x SemVer-frozen surface unchanged: no new global key
  bindings, no settings keys, no persistence format
  change. The reset action is reachable only through the
  help overlay's button.
- The reset path goes through the existing
  `applySettings` call site, so it inherits the same
  lower-priority `httpx` package-globals data race
  flagged in v1.7.2. The deferred fix plan (Snapshot
  pattern or atomic primitives in `httpx`) is unchanged.

[1.7.3]: https://github.com/lea-151107/pollen/releases/tag/v1.7.3

## [1.7.2] - 2026-05-31

### Fixed

- **OAuth DC status renderer hid the user_code during a
  re-fetch.** The switch in `renderOAuthDCStatus` evaluated
  the static `token != nil` case before the in-flight
  polling case, so pressing `g` on a panel that already held
  a Device Code token rendered the stale "Bearer …" preview
  for the entire flow. The user never saw the new IdP-issued
  user code, couldn't authorize on a second device, and the
  Poll loop ran for 30 minutes returning
  `authorization_pending` before timing out. A failed
  re-fetch had the same shape: the error was hidden behind
  the stale token until the user manually pressed `d` to
  forget it. The cases are now ordered transient → error →
  static, matching CC's `renderOAuthStatus` and AC's
  `renderOAuthACStatus`.
- **Settings overlay accepted `NaN` in float fields.**
  `strconv.ParseFloat("NaN", 64)` succeeds with math.NaN(),
  and IEEE 754 NaN comparisons return false, so the
  `f <= min || f >= max` range check let NaN slip through.
  The value landed in `m.responsePanelRatio` and propagated
  into View's layout math, corrupting column counts for
  the session. (json.Marshal coincidentally rejects NaN so
  settings.json stayed clean.) The float validator now
  rejects NaN and ±Inf via `math.IsNaN` / `math.IsInf`
  before the range comparison.
- **Ctrl+P swallowed bubbles textarea / textinput default
  bindings.** v1.7.1 bound Ctrl+P to the Settings overlay
  for universal terminal support. The global Settings case
  in `handleKey` fires before focus delegation, so Ctrl+P
  never reached `bubbles/textarea.LinePrevious` (default
  Ctrl+P, used to move the cursor up a line in the Body
  editor) or `bubbles/textinput.PrevSuggestion` (default
  Ctrl+P, used to cycle suggestions backwards in Headers).
  Both surfaces now keep their Ctrl+P behaviour: the
  Settings binding gets the existing `isTextEditingFocus`
  guard, same pattern the `u` undo shortcut already uses.
  Ctrl+P still opens Settings from any non-editing focus.

### Notes

- v1.x SemVer-frozen surface unchanged: no settings, key
  bindings, or persistence formats changed; the Settings
  binding's behaviour is narrowed, not extended.
- A real but lower-priority data race remains:
  `applySettings` writes plain (non-atomic) `httpx` package
  vars that the Send Cmd goroutine reads from `httpx.Do`.
  CI's `go test -race ./...` doesn't catch it because no
  test exercises a parallel Send + Settings edit; production
  occurrences are rare and effects are limited to torn
  reads of primitive types. A larger refactor (Snapshot
  pattern or atomic primitives in httpx) is reserved for a
  future release.

[1.7.2]: https://github.com/lea-151107/pollen/releases/tag/v1.7.2

## [1.7.1] - 2026-05-31

### Fixed

- **Settings overlay couldn't be opened with `Ctrl+,` in
  standard terminals.** v1.7.0 bound the Settings overlay
  to `Ctrl+,` only, mirroring VS Code. The comma key has
  no traditional ASCII control code (Ctrl+A..Z map to
  0x01..0x1A; Ctrl+[, ], ^, _ to 0x1B..0x1F; Ctrl+, sends
  a plain `,`), and bubbletea v1 doesn't enable the kitty
  / CSI-u keyboard protocol that would carry the distinct
  sequence. The binding therefore never fired in xterm,
  GNOME Terminal, macOS Terminal.app, Windows Terminal, or
  WSL — every press was seen as a literal comma. The fix
  adds `Ctrl+P` (ASCII 0x10, universally available) as the
  primary binding; `Ctrl+,` is kept as an alias for
  terminals that speak CSI-u so the VS Code muscle memory
  still works there. The Help overlay's Global section
  now reads "Ctrl+P / Ctrl+, : Open settings overlay".

### Notes

- The Settings overlay implementation itself is unchanged;
  only the keybinding that opens it is fixed.
- v1.x SemVer-frozen surface: additive only — Ctrl+P is a
  new binding, Ctrl+, remains documented.

[1.7.1]: https://github.com/lea-151107/pollen/releases/tag/v1.7.1

## [1.7.0] - 2026-05-31

### Added

- **OAuth 2.0 Device Authorization Grant (RFC 8628).** The
  Auth panel gains a sixth type, "OAuth DC", that drives the
  Device Code flow. Unlike Authorization Code, pollen does
  not launch a browser — it displays the IdP-issued user
  code + verification URL for the user to transcribe on
  whatever device they already have logged in, then polls
  the token endpoint until authorization completes. Fields:
  Device URL, Token URL, Client ID, Client Secret
  (optional), Scope. Pressing `g` starts the flow; Esc
  cancels in-flight Authorize or Poll. Honors the RFC
  state machine — `authorization_pending` continues
  polling, `slow_down` adds 5 seconds to the interval,
  `access_denied` / `expired_token` are terminal. 30-minute
  total timeout. This finally makes OAuth usable in
  WSL/SSH/CI/headless environments where the Authorization
  Code flow's browser launch isn't viable, structurally
  resolving the openBrowser-failure recovery item deferred
  since v1.6.2.
- **In-TUI Settings overlay (`Ctrl+,`).** All 17 keys from
  `settings.json` are now editable from inside pollen.
  Bool fields toggle on Enter; int / float / string fields
  drop into an editor with range validation. Each commit
  is applied to the matching runtime global (HTTP timeout,
  response cap, intruder defaults, proxy URL, …) and
  written back to `settings.json` immediately, so no
  restart is needed for most edits. Two fields — CA cert
  file and Enable cookies — are flagged with a `restart`
  badge because they're consumed only at startup.

### Notes

- Token persistence handles `device_code` tokens the same
  as CC and AC entries: keyed by
  `(token_url, client_id, grant)` in
  `~/.config/pollen/oauth_tokens.json`. Auto-refresh-on-
  send applies uniformly across all three grants.
- v1.x SemVer-frozen surface gains:
  - `AuthOAuthDC` AuthType
  - `Ctrl+,` keybinding for the Settings overlay
- Existing configuration files load unchanged.

[1.7.0]: https://github.com/lea-151107/pollen/releases/tag/v1.7.0

## [1.6.6] - 2026-05-31

### Fixed

- **GitHub Actions CI was red on `windows-latest` since
  v1.6.4.** The three regression tests added with the v1.6.4
  disk-persistence work pinned exact POSIX file mode bits
  (0o600 for `SaveJSONSecure` / TokenStore, 0o644 for
  `SaveJSON`) via `os.Stat(...).Mode().Perm()`. On Windows,
  Go's `os.WriteFile` reports 0o666 regardless of the
  requested mode because permissions are ACL-based — so
  every `go test -race ./...` run on the windows-latest
  matrix job failed for two releases. The underlying
  production code (`SaveJSON` / `SaveJSONSecure`) is correct
  on POSIX and Linux/macOS CI jobs continue to verify that.
  The Windows-only mode assertions now skip with a
  `runtime.GOOS == "windows"` guard and an inline comment
  explaining the asymmetry.

### Notes

- No production code changed. Windows file protection is
  still provided by the user-profile default ACL pollen's
  config directory inherits.
- This patch's CI run is itself the verification — six
  matrix jobs need to pass.

[1.6.6]: https://github.com/lea-151107/pollen/releases/tag/v1.6.6

## [1.6.5] - 2026-05-31

### Fixed

- **OAuth Refresh dropped the existing refresh_token when the
  IdP omitted it from the response.** RFC 6749 §6 lets the
  authorization server choose whether to rotate the refresh
  token on each refresh — when it does not (omits the field
  from the response), the client is expected to continue
  using the existing one. Google OAuth, for example,
  documents exactly this behavior, and Microsoft Entra /
  Auth0 / Okta all support non-rotating configurations.
  Pollen 1.6.0–1.6.4 silently overwrote the in-memory
  refresh_token with `""` whenever the IdP omitted it, which
  was annoying but recoverable (press `g` or re-authorize).
  v1.6.4's disk persistence made this break permanent: the
  empty refresh_token landed in
  `~/.config/pollen/oauth_tokens.json` and every subsequent
  session lost the ability to auto-refresh. `oauth.Refresh`
  now fills the returned Token's `RefreshToken` from the
  caller-supplied config when the IdP omits it, matching
  the behavior of `golang.org/x/oauth2`.

### Notes

- v1.x SemVer-frozen surface unchanged. No settings, key
  bindings, or persistence formats changed.
- The rotation case (server returns a new refresh_token) is
  unaffected — the new token is still adopted and persisted.
- Existing on-disk entries that already lost their
  refresh_token from earlier sessions cannot be recovered by
  this fix; affected users need to re-fetch (CC: `g`) or
  re-authorize (AC: `g`, then browser flow) once. Future
  refreshes against non-rotating IdPs then survive across
  sessions as designed.

[1.6.5]: https://github.com/lea-151107/pollen/releases/tag/v1.6.5

## [1.6.4] - 2026-05-31

### Added

- **OAuth token persistence to disk.** Successful OAuth
  fetches (Client Credentials and Authorization Code) and
  refreshes are now written to
  `~/.config/pollen/oauth_tokens.json` (mode 0600). On next
  start, when the Auth panel's Token URL + Client ID match
  a stored entry, the access + refresh token are hydrated
  automatically — no second browser dance for AC, no
  in-memory cache lost across sessions for CC. Entries are
  keyed by `(token_url, client_id, grant)` so CC and AC
  tokens for the same IdP/client coexist. The hydrated-but-
  expired path goes through the existing v1.6.0
  auto-refresh-on-send.
- **`d` on the Auth panel action row forgets the persisted
  token** for the current Token URL + Client ID. The
  on-disk entry is removed and the in-memory token is
  cleared; a status toast confirms.
- **`oauth_persist_tokens` setting** (default `true`).
  Users who prefer session-only OAuth set this to `false`
  in `settings.json`; when disabled, pollen neither reads
  nor writes the token file.
- **`userconfig.SaveJSONSecure`** internal helper writes
  JSON with 0o600 mode (owner read/write only) via the
  same atomic temp+rename pattern as `SaveJSON`. Existing
  `SaveJSON` callers (settings, env, history, collections)
  continue to use 0o644.

### Notes

- v1.x SemVer-frozen surface: only additive changes —
  a new settings field, a new keybinding (`d` on the OAuth
  action row), and a new on-disk file. Existing
  configurations load unchanged. The Auth panel's tab
  strip is unchanged.
- `oauth_persist_tokens` defaults to `true` because the
  feature exists to remove friction from the OAuth dance;
  the file mode (0600) and `d` forget shortcut give
  security-conscious users two opt-out paths.
- Token encryption at rest is intentionally not provided;
  0600 in `~/.config` follows the same posture as
  `gh`, `gcloud`, and `aws-cli`.

[1.6.4]: https://github.com/lea-151107/pollen/releases/tag/v1.6.4

## [1.6.3] - 2026-05-30

### Fixed

- **Status toasts no longer auto-cleared on two specific
  paths.** The `y` (copy response body) shortcut left
  "copied as response body" pinned to the status bar
  indefinitely; every other copy/delete/save path schedules
  a `statusTick` to dismiss the toast after a few seconds,
  but the `ResponseCopyMsg` arm of Update was an arm-level
  omission. Similarly, the auto-refresh-on-send path
  (v1.6.0) set "refreshing OAuth token…" up front and the
  success handler then ran the actual send, but
  `sendResultMsg` never replaced that status — the
  "refreshing…" string stayed on screen long after the
  request had completed and the response was visible. Both
  arms now follow the rest of the codebase's pattern: the
  copy path returns a 2-second statusTick, and the OAuth
  refresh success path replaces the in-flight string with
  "OAuth token refreshed" + a 2-second tick using
  `tea.Batch` so the send and the tick run independently.

### Notes

- v1.x SemVer-frozen surface unchanged: no new keybindings,
  no changes to settings/persistence formats.
- The OAuth refresh-failure path (already in v1.6.0)
  remained correct and is untouched.

[1.6.3]: https://github.com/lea-151107/pollen/releases/tag/v1.6.3

## [1.6.2] - 2026-05-30

### Fixed

- **OAuth Authorization Code callback never arrived when
  Redirect URI used `[::1]` (IPv6 loopback).** v1.6.0's
  `parseLoopback` accepted `::1` as a valid loopback host
  but the actual `net.Listen` call was hard-coded to
  `127.0.0.1`, so a browser following the IdP's
  `[::1]:port/callback` redirect hit nothing and the flow
  timed out after 5 minutes. The listener now binds to the
  host the user wrote in their Redirect URI
  (`127.0.0.1` → IPv4-only, `::1` → IPv6-only, `localhost`
  → whatever the kernel's dual-stack resolves). A new
  regression test pins the IPv6 end-to-end path and skips
  cleanly on environments without IPv6 loopback.
- **Ctrl+/ help overlay's Auth section was stuck on
  pre-v1.6.0 wording.** The type list showed only
  "None / Bearer / Basic / OAuth", the `g` row was framed
  as Client-Credentials-only, and Esc-on-action-row (the
  cancel for an in-flight OAuth AC flow added in v1.6.0)
  was undocumented. The section now lists all five types,
  describes `g` as covering both grants, and includes the
  Esc cancel.

### Notes

- The `internal/oauth.parseLoopback` helper's signature
  changed from `(int, string, error)` to
  `(string, int, string, error)` to expose the parsed
  host. The function is unexported so this is not a
  surface change.
- Comment in `internal/oauth/authcode.go` around
  `openBrowser` rewritten — the old comment claimed the
  full constructed URL was printed to stderr / shown in
  the status line as a recovery path, neither of which
  was implemented. The functional gap (no recovery from
  openBrowser failure) is intentional scope-out and is
  deferred to a future release.

[1.6.2]: https://github.com/lea-151107/pollen/releases/tag/v1.6.2

## [1.6.1] - 2026-05-30

### Fixed

- **Ctrl+/ help overlay overflowed standard terminals.** The
  overlay had grown to 17 sections and ~80 item rows by
  v1.6.0; rendered as a single centered string, the bottom
  half clipped off-screen on 24-row terminals and could not
  be scrolled. The overlay is now a navigable accordion: all
  section titles are always visible in one column with
  `▶`/`▼` glyphs, Enter (or Space) toggles the focused
  section open / closed, ↑/↓ (j/k) move the cursor, g/G
  jump to first/last, PgUp/PgDn jump by 5, and the viewport
  follows the cursor so the focused section header is
  always on screen. Multiple sections can be expanded
  simultaneously. Global is pre-expanded on open so the
  first Ctrl+/ press shows content immediately. Esc, q, or
  Ctrl+/ closes.

### Notes

- v1.x SemVer-frozen surface unchanged: Ctrl+/, Esc, q
  still open/close the overlay. The new navigation keys
  (↑/↓, j/k, Enter, Space, g, G, PgUp, PgDn) are additive
  inside an overlay that previously accepted only the close
  keys, so existing muscle memory is preserved.

[1.6.1]: https://github.com/lea-151107/pollen/releases/tag/v1.6.1

## [1.6.0] - 2026-05-30

### Added

- **OAuth 2.0 Authorization Code with PKCE.** The Auth panel
  gains a fifth type, "OAuth AC", driving the Authorization
  Code grant with PKCE (RFC 6749 §4.1, RFC 7636, RFC 8252).
  Fields: Auth URL, Token URL, Client ID, Client Secret
  (optional for public clients), Redirect URI, Scope.
  Pressing `g` on the action row generates a 256-bit state
  and a PKCE verifier + S256 challenge, starts a loopback
  callback server on the Redirect URI's port, opens the
  user's default browser at the auth endpoint, validates
  state on callback, exchanges the code (with
  `code_verifier`) at the token endpoint, and stores the
  resulting token. Esc cancels an in-flight flow; the whole
  thing has a 5-minute timeout.
  Browser launch uses `open` (macOS), `rundll32 url.dll,…`
  (Windows), or `wslview` / `xdg-open` (Linux/WSL). Redirect
  URI defaults to `http://127.0.0.1:8765/callback`; pollen
  only accepts loopback redirects (RFC 8252 §7.3).
- **Auto-refresh on send.** When the active auth type is
  OAuth (CC) or OAuth AC, the current access token is within
  30 seconds of expiry, and a refresh token was issued,
  pollen now issues an OAuth 2.0 refresh request before
  sending and sends with the new token transparently.
  Refresh failure aborts the send and prompts re-auth via a
  red status line. Skipped silently for non-OAuth auth types
  or tokens without a `refresh_token`.

### Notes

- v1.x SemVer-frozen surface gains `AuthOAuthAC` as a new
  auth type. Existing `collections.json` entries with
  `"auth_type": "oauth"` continue to load as Client
  Credentials unchanged.
- OAuth tokens (CC and AC) remain session-only — disk
  persistence is intentionally deferred.

[1.6.0]: https://github.com/lea-151107/pollen/releases/tag/v1.6.0

## [1.5.3] - 2026-05-30

### Fixed

- **Auth and Body tab strips wrapped onto a second line at
  narrow terminal widths.** The auth-type selector
  (`None / Bearer / Basic / OAuth`) and the body-type selector
  (`JSON / FORM / RAW / GRAPHQL / MULTIPART`) rendered at a
  fixed width regardless of the panel size, so shrinking the
  terminal wrapped the bar and pushed the input fields /
  editor area out of the panel border. Both selectors now
  measure the assembled bar with `lipgloss.Width` and, when it
  would overflow the available width, collapse to the single
  selected label wrapped in dim `‹ ›` guillemets — matching
  the `←/→` cycle hint on the line below. The threshold is
  measured rather than hardcoded against the current tab set,
  so future tab additions (e.g. v1.6 Authorization Code) do
  not need follow-up tuning here.

### Notes

- v1.x SemVer-frozen surface is unchanged: this patch only
  reflows two rendering helpers; no CLI, settings, or
  keybinding changes.
- Authorization Code with PKCE remains on track for v1.6.

[1.5.3]: https://github.com/lea-151107/pollen/releases/tag/v1.5.3

## [1.5.2] - 2026-05-30

### Fixed

- **`--import-curl` silently dropped when combined with any
  `--export-*` flag.** Running, for example,
  `pollen --import-curl 'curl ...' --export-postman foo.json`
  ran the export, called `os.Exit(0)`, and silently skipped
  the curl import — the same kind of silent-drop hazard the
  v1.3.2 multi-export check addressed. pollen now refuses any
  combination of two single-shot CLI actions with exit code 2
  and a stderr message listing all five flags involved
  (`--export-postman` / `--export-collections` /
  `--export-openapi` / `--export-intruder` / `--import-curl`).
- **OAuth token preview could break multi-byte UTF-8.** The
  authorization panel sliced the token's preview on byte
  boundaries (`tok[:8]`, `tok[len(tok)-4:]`), which would
  chop a multi-byte rune mid-sequence and produce invalid
  UTF-8 for a non-ASCII access_token. Mainstream IdPs return
  ASCII tokens, but RFC 6749 does not require it; the
  renderer now rune-slices so the preview always lands on
  rune boundaries.

### Notes

- v1.x SemVer-frozen surface is unchanged: this patch only
  tightens an existing CLI guard and a rendering helper.
- Authorization Code with PKCE remains on track for v1.6.

[1.5.2]: https://github.com/lea-151107/pollen/releases/tag/v1.5.2

## [1.5.1] - 2026-05-30

### Added

- **multipart/form-data body support.** The body editor's
  `←/→` tab cycle now includes `MULTIPART`. The body is a
  line-based DSL — `name=value` for text parts,
  `name=@/path/to/file` (optionally `;type=image/png`) for file
  uploads. At send time pollen streams the file parts through
  `mime/multipart` and sets `Content-Type:
  multipart/form-data` with the right boundary. cURL export
  emits `-F` arguments so the shared command actually
  performs the upload; fetch export builds a `FormData` IIFE
  with placeholder file references. Postman v2.1 export and
  import round-trip the multipart body using the spec's
  `{"mode": "formdata", "formdata": [...]}` shape. Intruder
  markers and dynamic variables apply to the multipart DSL
  string at send time.
- **`--import-curl` CLI flag.** Convert a curl command into a
  collections entry without launching the TUI. Three input
  modes: `--import-curl 'curl ...'` (literal),
  `--import-curl @file` (read from file), `--import-curl -`
  (stdin). Supported flags: `-X / --request`, `-H / --header`,
  `-d / --data / --data-raw / --data-binary`,
  `--data-urlencode`, `-F / --form`, `-u / --user`,
  `-A / --user-agent`, `-e / --referer`, `--cookie / -b`,
  `-G / --get`. Transport flags (`-L`, `-k`, `-s`, `-v`, `-i`,
  plus the clumped form `-sLv`) are silently dropped. Method
  inference: explicit `-X` wins; `-G` forces GET; any data
  flag implies POST.

### Notes

- This is a patch release (matching pollen's v1.2.1 precedent
  for additive functionality in a patch). The new
  `BodyMultipart` constant, the multipart line DSL, the
  `--import-curl` flag, and the new `MULTIPART` body tab join
  the v1.x SemVer-frozen surface alongside the v1.5.0
  additions.
- Authorization Code with PKCE is still on track for v1.6, as
  reserved in the v1.5.0 release notes.

[1.5.1]: https://github.com/lea-151107/pollen/releases/tag/v1.5.1

## [1.5.0] - 2026-05-30

### Added

- **Dynamic variables.** A small built-in set of `{{$name}}` /
  `{{$name:arg}}` tokens that pollen evaluates at send time:
  `{{$timestamp}}`, `{{$timestamp_ms}}`, `{{$datetime}}`,
  `{{$uuid}}`, `{{$random}}`, `{{$random:N}}`,
  `{{$random:M-N}}`, `{{$base64:VALUE}}`, `{{$urlencode:VALUE}}`.
  Unknown `{{$name}}` tokens are passed through verbatim so the
  existing intruder marker `{{$payload}}` continues to compose.
  In Intruder runs, dynamic variables are expanded **per
  request** inside the worker loop, so `{{$uuid}}` yields a
  different value per iteration.
- **OAuth 2.0 Client Credentials in the Auth panel.** The
  ←/→ type selector now cycles
  None / Bearer / Basic / **OAuth**. Selecting OAuth shows
  four input rows (Token URL, Client ID, Client Secret,
  Scope) and an action row that fetches the token on `g`.
  On success, pollen injects
  `Authorization: Bearer <access_token>` on every Send. The
  action row shows the masked token, time-to-expiry, and
  "press g to refresh". Errors surface the server's
  `error_description` when present (RFC 6749 §5.2).

### Changed

- Send-time expansion chain is now **env → response chaining
  → dynamic vars**. The dynamic step runs last so an env
  value that embeds a `{{$name}}` token resolves at send
  time and so each Send press receives fresh
  timestamps / UUIDs.

### Notes

- OAuth tokens are session-only — v1.5.0 does not write them
  to disk (keyring integration is reserved for v1.6+ along
  with Authorization Code with PKCE).
- The dynamic-variable namespace (`{{$...}}`), built-in
  names, and OAuth's `g` action key join the v1.x
  SemVer-frozen surface alongside the v1.4 additions.

[1.5.0]: https://github.com/lea-151107/pollen/releases/tag/v1.5.0

## [1.4.2] - 2026-05-30

### Fixed

- **`Response.SetError` now resets filter / search / diff
  view state.** `SetResponse` cleared these via `resetFilter`,
  but `SetError` didn't — so after a successful response with
  the user having opened the jq filter prompt, the `Ctrl+F`
  search bar, or `D` diff mode, a subsequent network error
  would render the red error text **underneath** still-visible
  prompts and the stale `[diff]` badge. The user couldn't
  dismiss them either (their key handlers gate on
  `r.resp != nil`). Mirror the `resetFilter` cleanup inside
  `SetError`, extended to cover `searchActive` / `searchQuery`
  / `diffMode` / `diffBody` so the error view replaces the
  body cleanly.

[1.4.2]: https://github.com/lea-151107/pollen/releases/tag/v1.4.2

## [1.4.1] - 2026-05-30

### Fixed

- **Intruder result-table cursor now stays inside the visible
  view.** v1.4.0's row cursor wasn't re-clamped when the
  filter narrowed the view between keystrokes, so the cursor
  could sit past the last row: the `▶` marker disappeared,
  Down became a no-op, and the user had to press Up many
  times before the cursor reappeared. The clamp now runs
  alongside the existing scroll-offset clamp at the top of
  every keystroke so any path that shrinks the view (filter,
  preset, sort) self-heals.
- **Intruder detail view no longer lets the user scroll past
  EOF into a blank window.** The Down / PgDown keys now stop
  at `len(body) - visibleRows`; an `end` / `G` key jumps to
  that bottom. Implementation extracts a shared
  `detailBodyLines` helper so the view (rendering) and the
  update loop (scroll clamp) agree on what "end of body"
  means.
- **Misleading binary-body hint in the Intruder detail view.**
  The hint used to say "see hex preview in the main Response
  panel" but the main Response panel shows the live
  single-request response, not the selected Intruder result —
  there's no path to view this particular binary through that
  panel. Trim to "(binary, N bytes — body not rendered here)"
  to avoid the bad pointer.
- **Postman v2.1 import now accepts both string and object
  forms of `body.graphql.variables`.** The spec calls for the
  string form, but Insomnia and some hand-written collections
  emit the object form (`"variables": {"id": 1}`). v1.4.0
  declared the importer's Variables field as `string`, so an
  object-form file failed to unmarshal and the whole
  collection import errored. v1.4.1 reads it as
  `json.RawMessage` and normalises into the JSON-string shape
  pollen stores internally.

[1.4.1]: https://github.com/lea-151107/pollen/releases/tag/v1.4.1

## [1.4.0] - 2026-05-30

### Added

- **GraphQL body support.** A new `GraphQL` tab joins the body
  editor's `JSON / Form / Raw` rotation. The editor area splits
  into a top **query** pane and a bottom **variables (JSON)**
  pane; `Ctrl+G` toggles focus between them in editor mode.
  Pollen wraps the two panes in the canonical `{"query":"...",
  "variables":{...}}` envelope and POSTs as
  `application/json`. Variables that don't parse are silently
  omitted from the envelope (the server reports the error).
- **GraphQL through the rest of pollen.** Env expansion and
  response chaining cover the variables pane; Intruder markers
  (`{{$payload}}`, `{{$payload1..N}}`) work inside variables so
  fuzzing GraphQL inputs is just an ordinary Intruder run. The
  cURL / fetch exports build the envelope into `--data` /
  `body`; Postman v2.1 export and import round-trip GraphQL
  using the spec's native `{"mode":"graphql","graphql":{...}}`
  shape.
- **Per-row response detail in Intruder.** Pressing `Enter` on
  a result row now opens an in-overlay detail view (status,
  headers, body) for that single request. `Esc` returns to the
  results table; `↑/↓ PgUp/PgDn` scroll the body. Response
  bodies are body-cap truncated by the new
  `intruder_response_body_cap_kib` setting (default 64 KiB) so
  a 1000-payload run doesn't pin GiBs of RAM just to support
  the detail view.
- **In-app CSV export in Intruder.** `e` in the results table
  opens a path prompt with a timestamped default
  (`intruder-YYYYMMDD-HHMMSS.csv`); `Enter` writes the same
  format as the `--export-intruder` CLI flag.
- **Filter DSL in Intruder.** `/` now accepts a small AND-
  composed token DSL on top of the existing payload substring
  match: `size:>1000`, `size:50-100`, `dur:>500`, `dur:<10`,
  `s:4xx`, `s:>=500`, `s:200-299`. Bare tokens still match
  payload substring. Tokens combine: `/admin size:>=1000
  s:4xx` keeps rows where payload contains "admin", size ≥
  1000, and status is 4xx.
- **Size median outlier highlight.** Result rows whose size
  deviates by more than 50% from the median (of the currently
  visible / filtered set) get a `!` marker on the size cell.
- **`intruder_response_body_cap_kib` setting** (default 64,
  clamped to [1, 10240]) controls how many KiB of each
  response are retained for the detail view.

### Changed

- **4xx and 5xx rows now use different colors.** 4xx is
  yellow, 5xx and network errors are red, 2xx / 3xx use the
  terminal default. Previously every non-2xx row shared the
  same red.
- **The Intruder result cursor is now a separate row pointer
  from the scroll offset.** `↑/↓` moves the cursor; the
  window only scrolls when the cursor would leave the visible
  band. `g` / `G` jump to first / last row. The focused row
  shows a `▶` prefix and bold text.
- **`history.Request` gains a `GraphQLVariables` field**
  (`json:"graphql_variables,omitempty"`). Pre-v1.4 history /
  collection JSON files load unchanged via `omitempty`; the
  field is only populated when `body_type` is `graphql`.

[1.4.0]: https://github.com/lea-151107/pollen/releases/tag/v1.4.0

## [1.3.2] - 2026-05-30

### Changed

- `settings.Load` now prints a clear stderr warning when
  `proxy_url` in settings.json is malformed before resetting
  the value, instead of dropping the bad URL silently. The user
  can now tell whether their proxy setting was honoured at
  startup. The v1.3.1 startup warning in `main.go` was a
  duplicate of this path and has been removed.
- The Intruder help overlay (`Ctrl+/`) is now split into three
  sub-sections — markers, config modal, results table — and
  documents every key bound by the Intruder UI, including the
  v1.2.1 sort / filter (`s`, `Shift+S`, `/`, `f`) and the v1.3.0
  Mode / Positions / multi-position markers
  (`{{$payload1..N}}`).

### Fixed

- Passing multiple `--export-*` flags in a single run no longer
  silently drops every flag past the first. Pollen previously
  exited zero after writing only the first target, which made
  scripts that intended to produce all three formats fail
  silently. The combination is now refused at startup with
  exit code 2 and an explanatory stderr line, matching the
  existing alias-collision check between `--export-postman` and
  `--export-collections`.

### Removed

- Unused `KeyMap.Cancel` binding. It was defined in `app/keys.go`
  but never matched against — all `esc` handling in the TUI
  uses the `"esc"` string literal directly. No user-visible
  change.

[1.3.2]: https://github.com/lea-151107/pollen/releases/tag/v1.3.2

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

[1.3.1]: https://github.com/lea-151107/pollen/releases/tag/v1.3.1

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

[1.3.0]: https://github.com/lea-151107/pollen/releases/tag/v1.3.0

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

[1.2.1]: https://github.com/lea-151107/pollen/releases/tag/v1.2.1

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
