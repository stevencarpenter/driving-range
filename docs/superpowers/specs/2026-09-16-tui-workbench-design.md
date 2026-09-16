# Embedded practice workbench

## Decision

`golf` gains a workbench mode. It allocates a pseudo-terminal, runs the exercise child (Neovim, Bash, zsh) inside a virtual terminal emulator, and renders that emulator into a pane below a collapsible brief band. The TUI stays resident for the whole attempt. A single intercepted key (`F12`) opens a golf command palette. Every other keystroke reaches the child.

This replaces the current handoff at `internal/tui/tui.go:134`, where `tea.ExecProcess` releases the terminal to the child and tears the TUI down for the duration of the attempt.

The workbench is built on Bubble Tea v2, not v1. A spike established that Bubble Tea v1 silently discards input bytes in a case the workbench would reach. Evidence is recorded below.

## Constraints that shaped this

The operator runs `golf` inside tmux daily. Two consequences:

1. `golf` must not spawn or drive tmux. That would nest a multiplexer and collide with an existing prefix key.
2. Rows and columns are both scarce. The brief occupies a horizontal band, not a side rail, so the child keeps full width.

The operator asked for the minimum number of intercepted keys, because any key golf takes is a key the child can never receive.

## Version pinning

Measured 2026-09-16 with Go 1.26.8, the version pinned by CI at `.github/workflows/ci.yml:28`.

| Module | Version | Role |
| --- | --- | --- |
| `charm.land/bubbletea/v2` | v2.0.9 | TUI runtime, byte-accurate input, cursor API |
| `charm.land/lipgloss/v2` | v2.0.6 | Styling and layout |
| `charm.land/bubbles/v2` | v2.2.1 | Spinner, viewport, progress bar |
| `github.com/charmbracelet/x/vt` | v0.0.0-20260913004009-c615ff2f7805 | Virtual terminal emulator |
| `github.com/charmbracelet/ultraviolet` | v0.0.0-20260910203606-6c9e17dc7a16 | Cell, screen, and input event layer |
| `github.com/creack/pty` | v1.1.24 | Pseudo-terminal allocation and resize |

`x/vt` and `ultraviolet` carry no semantic version tags. `go.sum` still hash-pins them, so builds remain reproducible. The operator accepted upstream instability for this project.

`ultraviolet` at v0.0.0-20260910203606 is newer than the revision `bubbletea/v2` v2.0.9 and `x/vt` select on their own. The combination was built and exercised end to end before being recorded here.

`CONTRIBUTING.md:8` requires a concrete requirement that existing facilities cannot meet before adding a dependency. No Go standard library package emulates a terminal or allocates a pseudo-terminal. `THIRD_PARTY_NOTICES.md` must be regenerated per `CONTRIBUTING.md:84`.

## Why Bubble Tea v2 rather than v1

`bubbletea@v1.3.10/key.go:566` reads input into a fixed 256 byte buffer. Line 578 computes `canHaveMoreData := numBytes == len(buf)`. The carry-forward path at line 590 runs only when a read fills the entire buffer. A short read that ends in the middle of a UTF-8 rune is treated as an event boundary, and the partial rune is discarded.

Measured behavior under v1, feeding bytes as separate reads:

| Input | Result |
| --- | --- |
| `\xe6\x97` then `\xa5` (日) | no key message produced |
| `\xf0\x9f\x90` then `\xbf` (🐿) | no key message produced |

This is reachable by pasting non-ASCII text into the pane, where chunk boundaries are set by the kernel.

The same cases under Bubble Tea v2, which parses through `ultraviolet`, produce `日` and `🐿` correctly. `ultraviolet/terminal_reader.go:117` uses a 4096 byte buffer and a timeout-driven completeness model rather than inferring event boundaries from read size.

Further v1 measurements, for the record:

- A 291 sequence corpus (every entry in `bubbletea@v1.3.10/key.go:354`, plus all C0 bytes, printable ASCII, multibyte UTF-8, Alt combinations, and bracketed paste) round-tripped as 241 byte-exact, 45 aliased, 5 lossy.
- The 45 aliases are terminal-specific variants (urxvt, rxvt, DECCKM) normalized to canonical xterm form. They are safe.
- Of the 5 lossy cases, 4 emit correct xterm sequences that Bubble Tea's own table cannot re-parse. The fifth is an upstream defect: `key.go:407` maps `\x1b[3;2~`, which is Shift+Delete, to `{Type: KeyInsert, Alt: true}`.
- Chunk-boundary testing passed 11 of 12 cases. The failure was the partial rune above.

## Key encoding

Bubble Tea v2 removes the need for a hand-written byte encoder. `tea.Key` and `uv.Key` are structurally identical, so `uv.KeyPressEvent(uv.Key(tea.Key(k)))` compiles, and `vt.Emulator.SendKey` accepts the result and writes the correct bytes to the child.

Verified output on the pinned stack:

| Key | Bytes to child |
| --- | --- |
| `a` | `a` |
| `ctrl+c` | `0x03` |
| `esc` | `0x1b` |
| `enter` | `0x0d` |
| `backspace` | `0x7f` |
| `up` | `\x1b[A` |
| `alt+j` | `\x1bj` |
| `f12` | `\x1b[24~` |
| `日` | `日` |

This matters beyond convenience. A first draft of a hand-written encoder contained two defects that the spike caught: Alt encoded as an ESC prefix rather than a CSI parameter, and invalid SS3 Home and End sequences under application cursor key mode. Delegating to the emulator removes that defect class.

### Known gap

`x/vt/key.go` implements `SendKey` as an exhaustive switch on exact `KeyPressEvent{Code, Mod}` values. Its `default` branch, at line 293, handles only `key.Mod == 0`. Modified special keys therefore produce no output. Measured gaps on the pinned version: `shift+up`, `ctrl+up`, `ctrl+home`. The same gap is present at `x/vt@latest`.

Mitigation: `internal/pane` intercepts events where `Mod != 0` and the code is a non-printable special key, and encodes them directly as xterm CSI parameter sequences. The modifier parameter is `1 + bits`, where shift is 1, alt is 2, and ctrl is 4. Letter-final keys encode as `\x1b[1;<mod><letter>`. Tilde-final keys encode as `\x1b[<n>;<mod>~`. Everything else delegates to `SendKey`.

This model was validated during the spike. Correcting a hand-written encoder to use it raised byte-exact round-trips from 200 to 241 of 291 in normal cursor mode, and from 198 to 241 in application cursor key mode, where it also reduced lossy cases from 8 to 5. The 5 that remain are the 4 re-parse artifacts and the 1 upstream Shift+Delete defect described above.

## Architecture

Four units, each independently testable.

### `internal/pane`

Owns the pseudo-terminal and the emulator. Knows nothing about exercises, attempts, or the catalog.

- Allocates the pseudo-terminal with `pty.StartWithSize`.
- Feeds child output into a `vt.SafeEmulator`, which is mutex-guarded for concurrent access.
- Converts `tea.KeyPressMsg` into child bytes, per the encoding rules above.
- Exposes the rendered screen, cursor position, alternate-screen state, and scrollback.
- Forwards resize to both the emulator (`Resize`) and the pseudo-terminal (`pty.Setsize`). The kernel raises `SIGWINCH` in the child.

Tested by writing byte sequences in and asserting cell contents, cursor position, and encoded output. No terminal required.

### `internal/tui/workbench.go`

The Bubble Tea model for an active attempt: brief band, pane, palette overlay, status line. Composes `internal/pane` output with Lip Gloss.

### `internal/runner`

Gains `Session.CheckNow(ctx)` for checking without ending the attempt. Existing terminal-exit checking is unchanged.

### Runtime shims

`runtime/golf-check` and `runtime/golf-hint`, plus native equivalents alongside the existing `golf-brief` at `internal/runner/native.go:124`.

## Rendering and the read loop

A reader goroutine writes child output into the emulator continuously. A separate ticker emits a repaint message at approximately 60 Hz. Repaint rate is therefore decoupled from child output rate, so a Neovim full-screen redraw cannot saturate the Bubble Tea message loop. `vt.SafeEmulator` exists for exactly this access pattern.

`Emulator.Render()` returns a multi-line string with SGR sequences intact and trailing whitespace trimmed per line. The workbench sets an explicit width and height on the containing Lip Gloss style rather than relying on the rendered string's natural dimensions.

The cursor uses Bubble Tea v2's native `tea.Cursor`, populated from `Emulator.CursorPosition()`. Shape and blink are available and are not faked.

## Key routing

`F12` is the only key golf intercepts. It opens a palette: `c` check, `h` hint, `v` reveal, `b` toggle brief band, `q` quit, `Esc` return to the child. Every other keystroke, including all Ctrl and Alt chords, all other function keys, and the tmux prefix, passes through untouched.

## Trigger channel

Shell commands provide a second path to the same actions, requiring no intercepted keys at all.

```sh
#!/bin/sh
printf '\033]9270;golf=check\007'
```

`golf` registers an OSC handler with `Emulator.RegisterOscHandler(9270, ...)`. The emulator consumes the sequence, so it never renders and never reaches the host terminal. The mechanism is identical in Docker and native modes and requires no socket, bind mount, or polling. OSC 9270 is unregistered, so a host terminal receiving it when the workbench is inactive ignores it. The shims print a plain instruction instead when `GOLF_WORKBENCH` is unset.

`Emulator.RegisterOscHandler` is also used to drop OSC 52 clipboard writes originating in the child.

## Mid-session check

`runner.Runner.snapshot` at `internal/runner/runner.go:540` already creates a separate container for Docker snapshots. `create` at line 145 generates a unique name per call and mounts the workspace volume read-only with `--network=none`. A snapshot taken while the practice container is live therefore does not disturb it.

One hazard: `CleanupContainers` removes every container carrying the attempt's owner label, which includes the live practice container. Mid-session check must not route through that path.

Native mode reads the live directory through `nativeSnapshot`, which already operates on a running workspace.

## Error handling and fallback

The existing `tea.ExecProcess` full-handoff path is retained and selectable, as `--classic` and as a Settings toggle. `tea.ExecProcess` survives in Bubble Tea v2 at `exec.go:50`. If the workbench conflicts with a specific Neovim configuration, one setting restores current behavior.

Failure to allocate a pseudo-terminal falls back to the classic path with a stated reason rather than failing the attempt.

The plain and `NO_COLOR` themes require that no styling escape sequences are emitted. A child emits whatever it chooses. Under plain mode the workbench strips SGR from the child render.

`golf --plain`, non-TTY invocation, and the `golf play` command path are unchanged.

## Contract changes

Three documented statements become false and must be revised in the same change.

1. `DESIGN.md`: "Child tools own the entire terminal and every key during an exercise." The child now owns every key except `F12`, and owns a pane rather than the terminal.
2. `SECURITY.md:21`: "Interactive child output goes directly to your terminal; only output returning to the outer TUI is sanitized." Child output now passes through the emulator, which parses bytes into cells. Arbitrary escape sequences from the child can no longer reach the host terminal. This is a net improvement, but golf becomes responsible for what it re-emits.
3. `README.md` describes exiting the child to trigger a check. Checking mid-session becomes possible.

## Testing

| Target | Method |
| --- | --- |
| `internal/pane` encoding | Table test over the 291 sequence corpus, asserting bytes delivered to the child |
| Partial input | Chunked delivery including split UTF-8 runes, split CSI sequences, and split bracketed paste |
| Emulator integration | Write known output, assert cells, cursor position, and alternate-screen state |
| Resize | Assert emulator dimensions and `pty.Setsize` propagation |
| OSC trigger | Assert the handler fires and the sequence does not appear in rendered output |
| Mid-session check | Assert the live container survives and the read-only snapshot succeeds |
| Migration regression | The existing `internal/tui` and `internal/catalog` suites must pass. `internal/catalog` needs no edits. `internal/tui` tests construct key messages directly (`tea.KeyMsg`, `tea.KeyRunes`, `tea.KeyCtrlC` and peers at 16 call sites), so they need mechanical API updates. No behavioral assertion may change. |

## Implementation sequence

1. Migrate to Bubble Tea v2, Lip Gloss v2, and Bubbles v2. Gate on the existing test suite. The measured API surface is 45 `tea.*`, 35 `lipgloss.*`, and 15 `bubbles.*` call sites. The one genuine refactor is `lipgloss.NewRenderer`, used at `internal/tui/tui.go:87`, which Lip Gloss v2 removes in favor of `colorprofile`. `View() string` becomes `View() tea.View` via `tea.NewView`. `tea.KeyMsg` becomes `tea.KeyPressMsg`.
2. Build `internal/pane` with the encoding tests as the gate. No TUI integration yet.
3. Integrate the workbench for native mode: band, pane, `F12` palette, status line.
4. Extend to Docker mode. Resize reaches the container through `SIGWINCH` on the pseudo-terminal, which the `docker` client forwards.
5. Add `Session.CheckNow`, the OSC trigger channel, and the `golf-check` and `golf-hint` shims.
6. Revise `DESIGN.md`, `SECURITY.md`, and `README.md`. Regenerate `THIRD_PARTY_NOTICES.md`.
