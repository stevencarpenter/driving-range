# Embedded Practice Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run the exercise shell or editor inside a pane in the `golf` TUI, under a collapsible brief band, so the brief stays visible and checks and hints can be triggered without leaving the attempt.

**Architecture:** A new `internal/pane` package allocates a pseudo-terminal, pipes the child through an `x/vt` virtual terminal emulator, and exposes the rendered screen plus an ordered key-input path. `internal/tui` composes that pane with a brief band and a single intercepted key (`F12`). The existing full-terminal handoff is retained as a fallback. This requires migrating the TUI from Bubble Tea v1 to v2 first, because v1 silently discards partial UTF-8 runes on short reads.

**Tech Stack:** Go 1.26.8, Bubble Tea v2, Lip Gloss v2, Bubbles v2, `charmbracelet/x/vt`, `charmbracelet/ultraviolet`, `creack/pty`, Docker, SQLite via `modernc.org/sqlite`.

**Spec:** `docs/superpowers/specs/2026-09-16-tui-workbench-design.md`

## Global Constraints

- Go 1.26.8 or newer. CI and release builds pin 1.26.8 (`.github/workflows/ci.yml:28`).
- Follow `CONTRIBUTING.md` implementation defaults and verification requirements for every change (`AGENTS.md`).
- Reuse existing code, then the Go standard library, then native platform features. A new dependency needs a concrete requirement existing facilities cannot meet (`CONTRIBUTING.md:8`).
- Update `THIRD_PARTY_NOTICES.md` from the module cache license files for modules returned by `go list -deps` on each release target when dependencies change (`CONTRIBUTING.md:84`).
- Exact dependency versions, all verified building together on 2026-09-16:
  - `charm.land/bubbletea/v2 v2.0.9`
  - `charm.land/lipgloss/v2 v2.0.6`
  - `charm.land/bubbles/v2 v2.2.1`
  - `github.com/charmbracelet/x/vt v0.0.0-20260913004009-c615ff2f7805`
  - `github.com/charmbracelet/ultraviolet v0.0.0-20260910203606-6c9e17dc7a16`
  - `github.com/creack/pty v1.1.24`
- No essential information may depend on color. Support `light`, `dark`, `auto`, and `plain` themes. Plain mode and `NO_COLOR` emit no styling escape sequences, including bold and background styling (`DESIGN.md`).
- Never claim a result was saved if persistence failed (`DESIGN.md`).
- Do not use an em dash or en dash as a separator in prose or commit messages.
- Commit messages must not reference an AI agent, assistant, harness, model, or tool.

## Notes for the implementer

Findings from the design spike that are not obvious from the code, ordered by how much time they will cost if missed.

**1. The Neovim gate is cleared.** The design's biggest assumption, that a full-screen alternate-screen application behaves in the pane, was unproven when this plan was written. It is now covered by `TestNvimEditsInPane` in `internal/pane/nvim_test.go`, which skips when `nvim` is absent. It launches the same argv the Docker runner uses for vim exercises, asserts the file renders and the alternate screen is active, performs a real motion edit (`f8cw9090`), writes with `:wq`, and checks both the exit status and the file contents. Keep that test passing; it is the regression guard for everything the workbench does.

**2. The emulator deadlocks without a drain, even if you never send a key.** Its input path is an `io.Pipe`, and it writes *replies* into that pipe while parsing child output: a DECRQM mode query answered in `handleRequestMode` is enough. Bubble Tea v2 queries modes at startup, so any real TUI child triggers this within milliseconds. With no reader on `Emulator.Read()`, `Write` blocks and Go kills the process with `fatal error: all goroutines are asleep - deadlock!`, whose stack points at the parser, not the cause. `Start` launches the draining goroutine, so `Session` is safe. Any code that constructs a bare `vt.Emulator` or `vt.SafeEmulator`, including a read-only observer that only calls `Render()`, must start its own reader first. This was hit for real while smoke testing the migrated TUI.

**3. `Render()` trims trailing whitespace per line.** A 40 column emulator with the text `hello` renders a 5 column string, not 40. Always set an explicit `Width` and `Height` on the containing Lip Gloss style, or the pane will jump around as content changes. This is why `workbench.view` sets both.

**4. `SafeEmulator` has a narrower API than `Emulator`.** It has no `Close()` and no `InputPipe()`. `Session.Close()` closes the pseudo-terminal instead, which ends the read goroutine. For raw bytes, `SendText` is the path: it was verified to pass control sequences through verbatim and to interleave in order with `SendKey`, which is what makes the single ordered input path work.

**5. `ultraviolet` is pinned newer than its dependents select.** `bubbletea/v2 v2.0.9` and `x/vt` both resolve `ultraviolet` to `v0.0.0-20260703014108-f5a850f9c2b7` on their own. This plan pins `v0.0.0-20260910203606-6c9e17dc7a16`, which was built and exercised end to end. If something breaks in a way that makes no sense, dropping ultraviolet back to the 20260703 revision is the first thing to try.

**6. A security hook will flag the test fixtures.** `exec.Command("/bin/sh", "-c", ...)` in the pane tests trips a command-injection warning. Those arguments are hardcoded literals with no user input, so it is a false positive. Do not rewrite the tests to appease it.

**7. `RegisterOscHandler` is not concurrency safe.** On `SafeEmulator` it has a value receiver, so the emulator's mutex does not cover it, and it mutates a handler map the parser reads. Registering a handler after the output goroutine is running is a data race that `go test -race` catches. `Start` registers everything before any goroutine launches. Run `go test -race ./internal/pane/` on any change to that constructor.

**8. Lip Gloss v2 rendering differences found while migrating.** These bit the existing test suite and will bite the workbench view in Tasks 5 and 7.

- `Style.Width(n)` now counts border and padding *inside* `n`. v1 excluded them, so v1 code that wrote `Width(w - 2)` to leave room for a border must become `Width(w)`. Both `Model.goal` and `Model.panel` needed this.
- The reset sequence is `\x1b[m`, not v1's `\x1b[0m`. `onSurface` matched the old spelling and silently became a no-op, which `TestNestedStyleRestoresContainingSurface` caught. It now uses the `styleReset` constant in `internal/tui/layout.go`.
- `Render()` emits truecolor unconditionally, with no terminal detection. Profile downgrade happens when Bubble Tea writes to the terminal. Tests no longer need to force a profile.
- Alternate screen is a `tea.View` field, not a program option. `tea.WithAltScreen` is gone. v2 also exits the altscreen automatically on quit, so sample `IsAltScreen()` while the program is running or it reads false.
- `help.Model.Width` is now the `SetWidth(int)` method, and `progress.Model.EmptyColor` is a `color.Color` rather than a string.
- There is no `lipgloss.Renderer`. `Model` carries a `darkBackground bool` fed by `tea.BackgroundColorMsg`, requested in `Init` via `tea.RequestBackgroundColor`. Tests set the field directly.

**9. Regenerating the key corpus.** Task 2 ships a 15 case table. To widen encoder coverage, the full set of sequences a terminal actually sends can be extracted from Bubble Tea's own table:

```bash
awk '/^var sequences = map\[string\]Key\{/,/^\}/' \
  "$(go env GOMODCACHE)"/github.com/charmbracelet/bubbletea@v1.3.10/key.go | grep '^\s*"'
```

That is the v1 module, kept here only as a corpus source. It yields 142 entries. The spike ran those plus all C0 bytes, printable ASCII, multibyte UTF-8, Alt combinations, and bracketed paste.

## File Structure

| Path | Responsibility |
| --- | --- |
| `internal/pane/encode.go` | Encode modified special keys as xterm CSI sequences. Pure, no I/O. |
| `internal/pane/encode_test.go` | Corpus table test for the encoder. |
| `internal/pane/pane.go` | `Session`: pseudo-terminal, emulator, ordered input path, resize, lifecycle. |
| `internal/pane/pane_test.go` | Emulator integration, resize, OSC trigger, partial-input tests. |
| `internal/tui/workbench.go` | Bubble Tea model for an active attempt: band, pane, palette, status line. |
| `internal/tui/workbench_test.go` | Band collapse, palette routing, key pass-through. |
| `internal/tui/tui.go` | Migrated to v2. Routes to workbench or classic handoff. |
| `internal/tui/styles.go` | Migrated off `lipgloss.Renderer`. |
| `internal/runner/runner.go` | Gains `Session.CheckNow`. |
| `runtime/golf-check`, `runtime/golf-hint` | Docker shims emitting the OSC trigger. |
| `runtime/Dockerfile` | Installs the two new shims. |

---

### Task 1: Migrate the TUI to Charm v2 (DONE)

The migration is one unit because the package does not compile between the dependency swap and the API updates. The gate is the existing test suite with no behavioral assertion changed.

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `internal/tui/tui.go`, `internal/tui/view.go`, `internal/tui/layout.go`, `internal/tui/styles.go`, `internal/tui/lifecycle.go`
- Modify: `internal/tui/tui_test.go`, `internal/tui/view_test.go`, `internal/tui/lifecycle_test.go`
- Modify: `cmd/golf/main.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `internal/tui` on Bubble Tea v2. `Model.View() tea.View`. Key handling on `tea.KeyPressMsg`. Styling without `lipgloss.Renderer`.

- [x] **Step 1: Record the current test baseline**

```bash
cd /Users/carpenter/projects/driving-range
go test ./... 2>&1 | tee /tmp/golf-baseline.txt
```

Expected: all packages `ok` or `no test files`. This is the artifact Task 1 must reproduce.

- [x] **Step 2: Swap the dependencies**

```bash
go get charm.land/bubbletea/v2@v2.0.9
go get charm.land/lipgloss/v2@v2.0.6
go get charm.land/bubbles/v2@v2.2.1
go mod edit -droprequire=github.com/charmbracelet/bubbletea
go mod edit -droprequire=github.com/charmbracelet/lipgloss
go mod edit -droprequire=github.com/charmbracelet/bubbles
```

- [x] **Step 3: Update imports across the package**

In `internal/tui/*.go` and `cmd/golf/main.go`, replace:

| Old | New |
| --- | --- |
| `github.com/charmbracelet/bubbletea` | `charm.land/bubbletea/v2` |
| `github.com/charmbracelet/lipgloss` | `charm.land/lipgloss/v2` |
| `github.com/charmbracelet/bubbles/spinner` | `charm.land/bubbles/v2/spinner` |
| `github.com/charmbracelet/bubbles/viewport` | `charm.land/bubbles/v2/viewport` |
| `github.com/charmbracelet/bubbles/progress` | `charm.land/bubbles/v2/progress` |
| `github.com/charmbracelet/bubbles/help` | `charm.land/bubbles/v2/help` |
| `github.com/charmbracelet/bubbles/key` | `charm.land/bubbles/v2/key` |

- [x] **Step 4: Remove the Lip Gloss renderer**

Lip Gloss v2 has no `Renderer`. In `internal/tui/tui.go`, delete the `renderer *lipgloss.Renderer` field from `Model` and drop `renderer: lipgloss.NewRenderer(os.Stdout)` from `New`.

In `internal/tui/styles.go`, change the two call sites:

```go
// styles.go, in Model.styles()
base := lipgloss.NewStyle()
```

```go
// styles.go, in Model.styles(), the auto-theme branch
if m.service.Config.Theme == "light" || (m.service.Config.Theme == "auto" && !lipgloss.HasDarkBackground(os.Stdin, os.Stdout)) {
```

`lipgloss.HasDarkBackground(in, out term.File) bool` is the v2 replacement. `os` is already imported in `styles.go`.

- [x] **Step 5: Update the Model interface**

Bubble Tea v2 changes `View() string` to `View() tea.View`. In `internal/tui/layout.go`, rename the existing method to `render()` and add the interface method:

```go
func (m Model) View() tea.View { return tea.NewView(m.render()) }

func (m Model) render() string {
	// body of the previous View() method, unchanged
}
```

- [x] **Step 6: Update key handling**

`tea.KeyMsg` becomes `tea.KeyPressMsg`. Key identity moves from a `Type` enum to a `Code` rune plus a `Mod` bitmask. In `internal/tui/tui.go`, the `case tea.KeyMsg:` arm becomes `case tea.KeyPressMsg:`. Replace type comparisons with `msg.String()` comparisons, which are stable across both versions:

| v1 | v2 replacement |
| --- | --- |
| `msg.Type == tea.KeyRunes` | `msg.Text != ""` |
| `msg.Type == tea.KeyEnter` | `msg.String() == "enter"` |
| `msg.Type == tea.KeyEsc` | `msg.String() == "esc"` |
| `msg.Type == tea.KeyTab` | `msg.String() == "tab"` |
| `msg.Type == tea.KeyCtrlC` | `msg.String() == "ctrl+c"` |
| `msg.Type == tea.KeyCtrlU` | `msg.String() == "ctrl+u"` |
| `msg.Type == tea.KeyPgUp` | `msg.String() == "pgup"` |
| `msg.Type == tea.KeyPgDown` | `msg.String() == "pgdown"` |

- [x] **Step 7: Update the Bubbles constructors**

`viewport.New` takes options in v2. In `internal/tui/layout.go`:

```go
vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
```

The progress bar loses `WithGradient` and `WithColorProfile`. In `internal/tui/layout.go`, replace those two options with `progress.WithColors`, passing the two endpoint colors the gradient used, and drop the color profile option. `progress.WithWidth`, `progress.WithoutPercentage`, and `progress.WithFillCharacters` are unchanged.

`spinner.New`, `spinner.WithSpinner`, `spinner.MiniDot`, `spinner.TickMsg`, and `spinner.Model.Update` keep their v1 signatures.

- [x] **Step 8: Update the tests**

Nearly all of this is one function. `press(m Model, key string) (Model, tea.Cmd)` at `internal/tui/tui_test.go:43` maps a key name to a `tea.KeyMsg`, and the tests call `press` rather than constructing messages themselves. Rewrite `press` and most of the migration is done:

```go
func press(m Model, key string) (Model, tea.Cmd) {
	var msg tea.KeyPressMsg
	switch key {
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	case "pgdown":
		msg = tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "pgup":
		msg = tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "ctrl+u":
		msg = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	case "ctrl+c":
		msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "tab":
		msg = tea.KeyPressMsg{Code: tea.KeyTab}
	default:
		msg = tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
	}
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}
```

Note the `default` arm: v1 carried a whole `[]rune` in `Runes`, while v2 carries one `Code` rune plus the `Text` it produced. Every current caller passes a single character, so taking `[]rune(key)[0]` is faithful. If any caller passes a multi-character string, split it into one `press` per rune instead of widening this helper.

For the two remaining direct constructions, use the v2 equivalent:

```go
tea.KeyPressMsg{Code: tea.KeyEnter}
tea.KeyPressMsg{Code: tea.KeyEscape}
tea.KeyPressMsg{Code: tea.KeyTab}
tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
tea.KeyPressMsg{Code: tea.KeyPgUp}
tea.KeyPressMsg{Code: tea.KeyPgDown}
```

A v1 rune key `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}` becomes `tea.KeyPressMsg{Code: 'j', Text: "j"}`.

Where a test calls `m.View()`, it now receives a `tea.View`. Use `m.View().Content` for string assertions, or call `m.render()` directly.

Do not change any assertion about rendered content or model state.

- [x] **Step 9: Build and run the suite**

```bash
go build ./... && go test ./... 2>&1 | tee /tmp/golf-v2.txt
diff <(grep -E '^(ok|FAIL)' /tmp/golf-baseline.txt) <(grep -E '^(ok|FAIL)' /tmp/golf-v2.txt)
```

Expected: `go build` clean, every package that was `ok` is `ok`, and the diff shows no `FAIL` that was not already failing.

- [x] **Step 10: Verify the TUI renders**

```bash
just build && ./golf --plain list vim | head -5
```

Expected: the exercise list prints with no escape sequences.

- [x] **Step 11: Regenerate third-party notices**

Follow the procedure in `CONTRIBUTING.md:84`.

- [x] **Step 12: Commit**

```bash
git add go.mod go.sum internal/tui cmd/golf THIRD_PARTY_NOTICES.md
git commit -m "refactor: migrate TUI to Bubble Tea v2, Lip Gloss v2, Bubbles v2"
```

---

### Task 2: Modified special key encoder (DONE)

`x/vt`'s `SendKey` is an exhaustive switch whose `default` branch (`x/vt/key.go:293`) handles only `key.Mod == 0`. Modified special keys produce no output. This task supplies the fallback.

**Files:**
- Create: `internal/pane/encode.go`
- Test: `internal/pane/encode_test.go`

**Interfaces:**
- Consumes: `charm.land/bubbletea/v2` from Task 1.
- Produces: `func EncodeModified(k tea.Key) []byte`, returning `nil` when the emulator's own `SendKey` covers the event.

- [x] **Step 1: Write the failing test**

```go
package pane

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEncodeModified(t *testing.T) {
	cases := []struct {
		name string
		key  tea.Key
		want string
	}{
		{"unmodified up defers to SendKey", tea.Key{Code: tea.KeyUp}, ""},
		{"printable rune defers", tea.Key{Code: 'a', Text: "a"}, ""},
		{"ctrl+c defers", tea.Key{Code: 'c', Mod: tea.ModCtrl}, ""},
		{"alt rune defers", tea.Key{Code: 'j', Mod: tea.ModAlt, Text: "j"}, ""},
		{"shift+up", tea.Key{Code: tea.KeyUp, Mod: tea.ModShift}, "\x1b[1;2A"},
		{"ctrl+up", tea.Key{Code: tea.KeyUp, Mod: tea.ModCtrl}, "\x1b[1;5A"},
		{"ctrl+shift+left", tea.Key{Code: tea.KeyLeft, Mod: tea.ModCtrl | tea.ModShift}, "\x1b[1;6D"},
		{"alt+down", tea.Key{Code: tea.KeyDown, Mod: tea.ModAlt}, "\x1b[1;3B"},
		{"ctrl+home", tea.Key{Code: tea.KeyHome, Mod: tea.ModCtrl}, "\x1b[1;5H"},
		{"shift+end", tea.Key{Code: tea.KeyEnd, Mod: tea.ModShift}, "\x1b[1;2F"},
		{"ctrl+delete", tea.Key{Code: tea.KeyDelete, Mod: tea.ModCtrl}, "\x1b[3;5~"},
		{"shift+pgup", tea.Key{Code: tea.KeyPgUp, Mod: tea.ModShift}, "\x1b[5;2~"},
		{"ctrl+f5", tea.Key{Code: tea.KeyF5, Mod: tea.ModCtrl}, "\x1b[15;5~"},
		{"shift+f1", tea.Key{Code: tea.KeyF1, Mod: tea.ModShift}, "\x1b[1;2P"},
		{"alt+f12", tea.Key{Code: tea.KeyF12, Mod: tea.ModAlt}, "\x1b[24;3~"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(EncodeModified(c.key)); got != c.want {
				t.Errorf("EncodeModified(%v) = %q, want %q", c.key, got, c.want)
			}
		})
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/pane/ -run TestEncodeModified -v`
Expected: FAIL, `undefined: EncodeModified`.

- [x] **Step 3: Write the implementation**

```go
// Package pane runs an exercise child inside a pseudo-terminal and a virtual
// terminal emulator so the TUI can keep rendering around it.
package pane

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// xterm modifier parameter bits. The encoded parameter is 1 plus their sum.
const (
	bitShift = 1
	bitAlt   = 2
	bitCtrl  = 4
)

// special maps a key code to its xterm form: either a CSI final letter or a
// numeric parameter used with a tilde final byte.
type special struct {
	letter byte
	tilde  int
}

var specials = map[rune]special{
	tea.KeyUp:    {letter: 'A'},
	tea.KeyDown:  {letter: 'B'},
	tea.KeyRight: {letter: 'C'},
	tea.KeyLeft:  {letter: 'D'},
	tea.KeyHome:  {letter: 'H'},
	tea.KeyEnd:   {letter: 'F'},

	tea.KeyF1: {letter: 'P'},
	tea.KeyF2: {letter: 'Q'},
	tea.KeyF3: {letter: 'R'},
	tea.KeyF4: {letter: 'S'},

	tea.KeyInsert: {tilde: 2},
	tea.KeyDelete: {tilde: 3},
	tea.KeyPgUp:   {tilde: 5},
	tea.KeyPgDown: {tilde: 6},

	tea.KeyF5:  {tilde: 15},
	tea.KeyF6:  {tilde: 17},
	tea.KeyF7:  {tilde: 18},
	tea.KeyF8:  {tilde: 19},
	tea.KeyF9:  {tilde: 20},
	tea.KeyF10: {tilde: 21},
	tea.KeyF11: {tilde: 23},
	tea.KeyF12: {tilde: 24},
}

// EncodeModified returns the xterm sequence for a special key held with a
// modifier. It returns nil when the emulator's own SendKey already covers the
// event, which is every unmodified key and every modified printable rune.
func EncodeModified(k tea.Key) []byte {
	s, ok := specials[k.Code]
	if !ok || k.Mod == 0 {
		return nil
	}
	var bits int
	if k.Mod&tea.ModShift != 0 {
		bits |= bitShift
	}
	if k.Mod&tea.ModAlt != 0 {
		bits |= bitAlt
	}
	if k.Mod&tea.ModCtrl != 0 {
		bits |= bitCtrl
	}
	if bits == 0 {
		return nil
	}
	if s.letter != 0 {
		return fmt.Appendf(nil, "\x1b[1;%d%c", bits+1, s.letter)
	}
	return fmt.Appendf(nil, "\x1b[%d;%d~", s.tilde, bits+1)
}
```

- [x] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/pane/ -run TestEncodeModified -v`
Expected: PASS, all 15 subtests.

- [x] **Step 5: Commit**

```bash
git add internal/pane/encode.go internal/pane/encode_test.go
git commit -m "feat(pane): encode modified special keys as xterm CSI sequences"
```

---

### Task 3: Pane session (DONE)

**Files:**
- Create: `internal/pane/pane.go`
- Test: `internal/pane/pane_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `EncodeModified` from Task 2.
- Produces:
  - `func Start(cmd *exec.Cmd, width, height int) (*Session, error)`
  - `func (s *Session) SendKey(k tea.KeyPressMsg)`
  - `func (s *Session) Resize(width, height int) error`
  - `func (s *Session) Render() string`
  - `func (s *Session) Cursor() (x, y int)`
  - `func (s *Session) AltScreen() bool`
  - `func (s *Session) Wait() error`
  - `func (s *Session) Close() error`
  - `func (s *Session) Output() <-chan struct{}` signalling that the screen changed

- [x] **Step 1: Add the dependencies**

```bash
go get github.com/charmbracelet/x/vt@v0.0.0-20260913004009-c615ff2f7805
go get github.com/charmbracelet/ultraviolet@v0.0.0-20260910203606-6c9e17dc7a16
go get github.com/creack/pty@v1.1.24
```

- [x] **Step 2: Write the failing test**

```go
package pane

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func waitFor(t *testing.T, s *Session, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if out := s.Render(); strings.Contains(out, want) {
			return out
		}
		time.Sleep(20 * time.Millisecond)
	}
	return s.Render()
}

func TestSessionRendersChildOutput(t *testing.T) {
	s, err := Start(exec.Command("/bin/sh", "-c", "printf 'hello pane\\n'; sleep 30"), 40, 6)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()
	if out := waitFor(t, s, "hello pane"); !strings.Contains(out, "hello pane") {
		t.Errorf("Render() = %q, want it to contain %q", out, "hello pane")
	}
}

func TestSessionForwardsKeys(t *testing.T) {
	s, err := Start(exec.Command("/bin/cat"), 40, 6)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()
	for _, r := range "abc" {
		s.SendKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	s.SendKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if out := waitFor(t, s, "abc"); !strings.Contains(out, "abc") {
		t.Errorf("Render() = %q, want it to contain %q", out, "abc")
	}
}

func TestSessionForwardsModifiedSpecialKeys(t *testing.T) {
	// `cat -v` renders control bytes visibly, so the exact sequence is checked.
	s, err := Start(exec.Command("/bin/cat", "-v"), 40, 6)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()
	s.SendKey(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})
	s.SendKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if out := waitFor(t, s, "^[[1;5A"); !strings.Contains(out, "^[[1;5A") {
		t.Errorf("Render() = %q, want it to contain %q", out, "^[[1;5A")
	}
}

func TestSessionResizePropagatesToChild(t *testing.T) {
	s, err := Start(exec.Command("/bin/sh", "-c", "trap 'tput cols' WINCH; sleep 30"), 40, 6)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()
	time.Sleep(200 * time.Millisecond)
	if err := s.Resize(72, 10); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if out := waitFor(t, s, "72"); !strings.Contains(out, "72") {
		t.Errorf("child did not observe the resize; Render() = %q", out)
	}
}

func TestSessionWaitReturnsAfterChildExits(t *testing.T) {
	s, err := Start(exec.Command("/bin/sh", "-c", "exit 3"), 40, 6)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()
	done := make(chan error, 1)
	go func() { done <- s.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 3 {
			t.Errorf("Wait() = %v, want exit status 3", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return")
	}
}
```

- [x] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/pane/ -run TestSession -v`
Expected: FAIL, `undefined: Start`.

- [x] **Step 4: Write the implementation**

```go
package pane

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Session couples a child process on a pseudo-terminal to a virtual terminal
// emulator. The emulator parses child output into cells, so escape sequences
// from the child never reach the host terminal.
type Session struct {
	cmd     *exec.Cmd
	ptmx    *os.File
	emu     *vt.SafeEmulator
	changed chan struct{}
	once    sync.Once
	waited  sync.Once
	waitErr error
}

// Start launches cmd on a pseudo-terminal sized to width by height.
func Start(cmd *exec.Cmd, width, height int) (*Session, error) {
	if width < 1 || height < 1 {
		return nil, errors.New("pane needs a positive width and height")
	}
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
	if err != nil {
		return nil, err
	}
	s := &Session{
		cmd:     cmd,
		ptmx:    ptmx,
		emu:     vt.NewSafeEmulator(width, height),
		changed: make(chan struct{}, 1),
	}
	// Child output into the emulator.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				s.emu.Write(buf[:n])
				s.notify()
			}
			if err != nil {
				s.notify()
				return
			}
		}
	}()
	// Emulator-encoded input out to the child. One goroutine keeps ordering.
	go func() { io.Copy(ptmx, s.emu) }()
	return s, nil
}

func (s *Session) notify() {
	select {
	case s.changed <- struct{}{}:
	default:
	}
}

// Output signals that the rendered screen may have changed. It coalesces, so a
// caller repainting on a ticker never falls behind a noisy child.
func (s *Session) Output() <-chan struct{} { return s.changed }

// SendKey forwards a key press to the child. Modified special keys take the
// explicit encoder because the emulator's SendKey drops them.
func (s *Session) SendKey(k tea.KeyPressMsg) {
	if b := EncodeModified(tea.Key(k)); b != nil {
		s.emu.SendText(string(b))
		return
	}
	s.emu.SendKey(uv.KeyEvent(uv.KeyPressEvent(uv.Key(tea.Key(k)))))
}

// Resize updates the emulator and the pseudo-terminal. Resizing the
// pseudo-terminal makes the kernel raise SIGWINCH in the child.
func (s *Session) Resize(width, height int) error {
	if width < 1 || height < 1 {
		return errors.New("pane needs a positive width and height")
	}
	s.emu.Resize(width, height)
	return pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
}

// Render returns the emulator screen with SGR sequences intact. Trailing
// whitespace is trimmed per line, so callers set an explicit width and height
// on the containing style.
func (s *Session) Render() string { return s.emu.Render() }

// Cursor reports the child's cursor position in cells.
func (s *Session) Cursor() (int, int) {
	p := s.emu.CursorPosition()
	return p.X, p.Y
}

// AltScreen reports whether the child is on the alternate screen.
func (s *Session) AltScreen() bool { return s.emu.IsAltScreen() }

// Wait blocks until the child exits and returns its error.
func (s *Session) Wait() error {
	s.waited.Do(func() { s.waitErr = s.cmd.Wait() })
	return s.waitErr
}

// Close releases the pseudo-terminal. It does not wait for the child.
func (s *Session) Close() error {
	var err error
	s.once.Do(func() { err = s.ptmx.Close() })
	return err
}
```

- [x] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/pane/ -v`
Expected: PASS for all five session tests and the encoder test.

- [x] **Step 6: Commit**

```bash
git add internal/pane go.mod go.sum
git commit -m "feat(pane): run a child on a pseudo-terminal behind a virtual terminal emulator"
```

---

### Task 4: OSC trigger channel

Shell commands give a second path to check, hint, and reveal that intercepts no keys at all. The child prints a private OSC sequence, the emulator consumes it, and it never renders.

**Files:**
- Modify: `internal/pane/pane.go`
- Test: `internal/pane/pane_test.go`

**Interfaces:**
- Consumes: `Session` from Task 3.
- Produces: `func (s *Session) OnTrigger(fn func(action string))`, and the exported constant `TriggerOSC = 9270`.

- [ ] **Step 1: Write the failing test**

```go
func TestSessionTriggerFiresAndIsNotRendered(t *testing.T) {
	s, err := Start(exec.Command("/bin/sh", "-c", `printf 'before\033]9270;golf=check\007after\n'; sleep 30`), 40, 6)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()
	got := make(chan string, 1)
	s.OnTrigger(func(action string) {
		select {
		case got <- action:
		default:
		}
	})
	select {
	case action := <-got:
		if action != "check" {
			t.Errorf("trigger action = %q, want %q", action, "check")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("trigger did not fire")
	}
	out := waitFor(t, s, "after")
	if strings.Contains(out, "9270") || strings.Contains(out, "golf=check") {
		t.Errorf("trigger sequence leaked into the rendered screen: %q", out)
	}
	if !strings.Contains(out, "before") || !strings.Contains(out, "after") {
		t.Errorf("surrounding output was lost: %q", out)
	}
}

func TestSessionDropsClipboardWrites(t *testing.T) {
	s, err := Start(exec.Command("/bin/sh", "-c", `printf '\033]52;c;aGVsbG8=\007visible\n'; sleep 30`), 40, 6)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()
	out := waitFor(t, s, "visible")
	if strings.Contains(out, "52;c") || strings.Contains(out, "aGVsbG8") {
		t.Errorf("clipboard sequence leaked: %q", out)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/pane/ -run 'TestSessionTrigger|TestSessionDropsClipboard' -v`
Expected: FAIL, `s.OnTrigger undefined`.

- [ ] **Step 3: Write the implementation**

Add to `internal/pane/pane.go`:

```go
// TriggerOSC is an unregistered OSC number. A host terminal that receives it
// when the workbench is inactive ignores it.
const TriggerOSC = 9270

// clipboardOSC is OSC 52. The child must not write the host clipboard.
const clipboardOSC = 52
```

Add two fields to `Session`:

```go
	mu      sync.Mutex
	trigger func(string)
```

Register the handlers at the end of `Start`, before `return s, nil`:

```go
	s.emu.RegisterOscHandler(TriggerOSC, func(data []byte) bool {
		_, action, found := strings.Cut(string(data), "golf=")
		if !found {
			return true
		}
		s.mu.Lock()
		fn := s.trigger
		s.mu.Unlock()
		if fn != nil {
			go fn(action)
		}
		return true // consumed, so it is never rendered
	})
	s.emu.RegisterOscHandler(clipboardOSC, func([]byte) bool { return true })
```

Add the setter:

```go
// OnTrigger registers the handler for golf trigger sequences emitted by the
// golf-check and golf-hint shims running inside the exercise.
func (s *Session) OnTrigger(fn func(action string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trigger = fn
}
```

Add `"strings"` to the imports.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/pane/ -v`
Expected: PASS for all pane tests.

- [ ] **Step 5: Commit**

```bash
git add internal/pane
git commit -m "feat(pane): consume golf trigger sequences and drop child clipboard writes"
```

---

### Task 5: Workbench model

**Files:**
- Create: `internal/tui/workbench.go`
- Test: `internal/tui/workbench_test.go`

**Interfaces:**
- Consumes: `pane.Session` from Tasks 3 and 4.
- Produces:
  - `type workbench struct`
  - `func newWorkbench(s *pane.Session, title, goal, brief string) *workbench`
  - `func (w *workbench) update(msg tea.Msg) (handled bool, action string)`
  - `func (w *workbench) view(width, height int, st viewStyles, border lipgloss.Border, plain bool) string`

The band renders the goal in two lines when expanded and one line when collapsed. `F12` opens the palette. Every other key goes to the child.

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestWorkbenchPaletteOpensOnF12Only(t *testing.T) {
	w := newWorkbench(nil, "vim.change-value", "Change port 8080 to 9090", "full brief text")
	if w.palette {
		t.Fatal("palette should start closed")
	}
	handled, _ := w.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if handled {
		t.Error("ordinary keys must not be handled by the workbench")
	}
	if w.palette {
		t.Error("ordinary keys must not open the palette")
	}
	handled, _ = w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	if !handled || !w.palette {
		t.Error("F12 must open the palette")
	}
}

func TestWorkbenchPaletteEmitsActions(t *testing.T) {
	w := newWorkbench(nil, "t", "g", "b")
	w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	handled, action := w.update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if !handled || action != "check" {
		t.Errorf("palette c = (%v, %q), want (true, \"check\")", handled, action)
	}
	if w.palette {
		t.Error("palette must close after an action")
	}
}

func TestWorkbenchPaletteEscapeReturnsToChild(t *testing.T) {
	w := newWorkbench(nil, "t", "g", "b")
	w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	handled, action := w.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || action != "" {
		t.Errorf("palette esc = (%v, %q), want (true, \"\")", handled, action)
	}
	if w.palette {
		t.Error("escape must close the palette")
	}
}

func TestWorkbenchBandTogglesFromThePalette(t *testing.T) {
	w := newWorkbench(nil, "vim.change-value", "Change port 8080 to 9090", "b")
	if !w.band {
		t.Fatal("band should start expanded")
	}
	w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	w.update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if w.band {
		t.Error("palette b must collapse the band")
	}
}

func TestWorkbenchCtrlCReachesTheChild(t *testing.T) {
	w := newWorkbench(nil, "t", "g", "b")
	handled, _ := w.update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if handled {
		t.Error("ctrl+c must reach the child, not quit golf")
	}
}

func TestWorkbenchPaneHeightLeavesRoomForBandAndStatus(t *testing.T) {
	w := newWorkbench(nil, "t", "g", "b")
	if got := w.paneHeight(24); got != 19 {
		t.Errorf("paneHeight(24) with band = %d, want 19", got)
	}
	w.band = false
	if got := w.paneHeight(24); got != 22 {
		t.Errorf("paneHeight(24) collapsed = %d, want 22", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui/ -run TestWorkbench -v`
Expected: FAIL, `undefined: newWorkbench`.

- [ ] **Step 3: Write the implementation**

```go
package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stevencarpenter/driving-range/internal/pane"
)

// workbench renders an active attempt: a brief band above the child pane, a
// status line below, and a palette opened by the single intercepted key.
type workbench struct {
	session            *pane.Session
	title, goal, brief string
	band               bool
	palette            bool
	check              string
	hints              int
}

// paletteKeys maps a palette key to the action it emits.
var paletteKeys = map[string]string{
	"c": "check",
	"h": "hint",
	"v": "reveal",
	"q": "quit",
}

func newWorkbench(s *pane.Session, title, goal, brief string) *workbench {
	return &workbench{session: s, title: title, goal: goal, brief: brief, band: true, check: "not run"}
}

// update routes a message. It reports whether the workbench consumed it, and
// the action the operator chose. An unconsumed key belongs to the child.
func (w *workbench) update(msg tea.Msg) (bool, string) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, ""
	}
	if w.palette {
		switch name := key.String(); name {
		case "esc":
			w.palette = false
			return true, ""
		case "b":
			w.palette = false
			w.band = !w.band
			return true, ""
		default:
			w.palette = false
			if action, found := paletteKeys[name]; found {
				return true, action
			}
			// An unknown key closes the palette without acting, so a stray
			// press never silently swallows the next keystroke.
			return true, ""
		}
	}
	if key.String() == "f12" {
		w.palette = true
		return true, ""
	}
	if w.session != nil {
		w.session.SendKey(key)
	}
	return false, ""
}

// paneHeight is the rows left for the child after the band and status line.
func (w *workbench) paneHeight(height int) int {
	rows := height - 1 // status line
	if w.band {
		rows -= 4 // two content lines plus a top and bottom border
	} else {
		rows -= 1
	}
	return max(1, rows)
}

func (w *workbench) view(width, height int, st viewStyles, border lipgloss.Border, plain bool) string {
	var parts []string
	if w.band {
		box := st.goal.Border(border).Padding(0, 1).Width(max(1, width-2))
		parts = append(parts, paint(box, "GOAL  "+w.goal+"\n"+firstLine(w.brief)))
	} else {
		parts = append(parts, paint(st.muted, "GOAL  "+w.goal))
	}
	screen := ""
	if w.session != nil {
		screen = w.session.Render()
		if plain {
			screen = uiText(screen)
		}
	}
	parts = append(parts, st.text.Width(width).Height(w.paneHeight(height)).Render(screen))
	parts = append(parts, w.status(st))
	return strings.Join(parts, "\n")
}

func (w *workbench) status(st viewStyles) string {
	if w.palette {
		return paint(st.action, " c check   h hint   v reveal   b brief   q quit   esc back ")
	}
	return paint(st.nav, " F12 golf   golf-check   CHECK "+w.check)
}

func firstLine(brief string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(brief), "\n")
	return line
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -run TestWorkbench -v`
Expected: PASS, all six tests.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/workbench.go internal/tui/workbench_test.go
git commit -m "feat(tui): add the workbench model with a brief band and an F12 palette"
```

---

### Task 6: Mid-session check

The TUI wiring in Task 7 calls `Service.CheckNow`, so it must exist first.

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/app/app.go`
- Test: `internal/runner/runner_test.go`

**Interfaces:**
- Consumes: `Runner.snapshot` and `Runner.create` in `internal/runner`.
- Produces: `func (s *Session) CheckNow(ctx context.Context) model.CheckResult`, `func (svc *Service) CheckNow(ctx context.Context, attemptID string) (model.CheckResult, error)`, and a `"checknow"` case in `Model.perform`.

- [ ] **Step 1: Write the failing test**

```go
func TestCheckNowLeavesTheLiveContainerRunning(t *testing.T) {
	r, a := integrationRunner(t) // skips unless GOLF_INTEGRATION=1; internal/runner/runner_test.go:85
	// 1. Prepare a session and start its child through pane.Start.
	// 2. Call CheckNow and assert it returns a result with no error.
	// 3. Assert `docker ps --filter label=<owner>` still lists the practice
	//    container, so the check did not stop the exercise.
	// 4. Call CheckNow a second time and assert it also succeeds.
}

func TestCheckNowOnNativeWorkspaceReadsTheLiveTree(t *testing.T) {
	// Prepare a native session, write a file into the workspace, call
	// CheckNow, and assert the result reflects the file without the child
	// having exited.
}
```

Fill in both bodies using `integrationRunner(t)` and the container helpers already in `internal/runner/runner_test.go`. Run them with `GOLF_INTEGRATION=1 go test ./internal/runner/` after `./golf setup`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/runner/ -run TestCheckNow -v`
Expected: FAIL, `s.CheckNow undefined`.

- [ ] **Step 3: Implement CheckNow**

`Runner.snapshot` at `internal/runner/runner.go:540` already creates a separate container for the Docker path. `create` at line 145 names it `"golf-session-" + randomID()` and mounts the workspace volume read-only with `--network=none`, so it does not disturb the live practice container. The native path reads the live directory through `nativeSnapshot`. Reuse both rather than writing a second snapshot path.

`CheckNow` must not call `CleanupContainers`, which removes every container carrying the attempt's owner label, including the live one. Record that constraint in a comment at the `CheckNow` definition.

`CheckNow` records a check event the same way the existing terminal check does, and leaves the attempt open.

- [ ] **Step 4: Expose it through the existing action dispatcher**

`func (m Model) perform(action string)` at `internal/tui/tui.go:607` already owns the busy flag, the status text, the lifecycle command, and the `operationMsg` result. Add one case to its action switch at `internal/tui/tui.go:648`, beside the existing `check`:

```go
		case "checknow":
			var result model.CheckResult
			result, err = s.CheckNow(m.lifecycle.ctx, id)
			text = formatCheck(result)
```

Do not add a parallel command path. Task 7 calls `m.perform("checknow")`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/runner/ -v`
Expected: PASS, or SKIP for the Docker case without a daemon.

- [ ] **Step 6: Commit**

```bash
git add internal/runner internal/app internal/tui
git commit -m "feat(runner): check an attempt without ending the session"
```

---

### Task 7: Wire the workbench into the TUI for native mode

**Files:**
- Modify: `internal/tui/tui.go:130-140`
- Modify: `internal/tui/layout.go`
- Modify: `internal/tui/lifecycle.go`
- Modify: `cmd/golf/main.go`
- Modify: `internal/app/app.go`
- Test: `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: `newWorkbench` from Task 5, `pane.Start` from Task 3, `Service.CheckNow` from Task 6.
- Produces: `Model.workbench *workbench`, the `paneTickMsg` and `paneTriggerMsg` messages, `lifecycle.attach` and `lifecycle.send`, and a `Config.Classic bool` setting selecting the old handoff.

- [ ] **Step 1: Write the failing test**

```go
func TestClassicModeStillUsesExecProcess(t *testing.T) {
	m := testModel(t)
	m.service.Config.Classic = true
	updated, cmd := m.Update(preparedMsg{play: testPlay(t), id: "a1"})
	if cmd == nil {
		t.Fatal("classic mode must return a command")
	}
	if updated.(Model).workbench != nil {
		t.Error("classic mode must not build a workbench")
	}
}

func TestWorkbenchModeBuildsAPane(t *testing.T) {
	m := testModel(t)
	m.service.Config.Classic = false
	updated, _ := m.Update(preparedMsg{play: testPlay(t), id: "a1"})
	if updated.(Model).workbench == nil {
		t.Error("workbench mode must build a workbench")
	}
}
```

The model helper is `testModel(t)` at `internal/tui/tui_test.go:20`. There is no play helper, so add `testPlay(t)` returning an `*app.Play` whose `Session.Command()` is `exec.Command("/bin/cat")`. `app.Play` has fields `Session`, `SessionID`, and `Attempt` only.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestClassicMode|TestWorkbenchMode' -v`
Expected: FAIL, `Config.Classic undefined` and `Model.workbench undefined`.

- [ ] **Step 3: Add the setting**

In `internal/app/app.go:27`, `Config` currently holds `Track string ` and `Theme string ` with json tags `"track"` and `"theme"`. Add `Classic bool` with json tag `"classic"` beside them. The test fixture builds `app.Config` at `internal/tui/tui_test.go:32`; set the field there only where a test needs it.

- [ ] **Step 4: Add the lifecycle sender**

`OnTrigger` fires from a goroutine the emulator owns, so it needs a way into the Bubble Tea loop. In `internal/tui/lifecycle.go`, add a `program *tea.Program` field and these methods:

```go
func (l *lifecycle) attach(p *tea.Program) { l.mu.Lock(); l.program = p; l.mu.Unlock() }

func (l *lifecycle) send(msg tea.Msg) {
	l.mu.Lock()
	p, stopping := l.program, l.stopping
	l.mu.Unlock()
	if p != nil && !stopping {
		p.Send(msg)
	}
}
```

In `Run`, construct the program into a variable, call `m.lifecycle.attach(p)`, then call `p.Run()`.

- [ ] **Step 5: Replace the handoff**

In `internal/tui/tui.go`, add to `Model`:

```go
	workbench *workbench
```

Add the messages beside the other message types:

```go
type paneTickMsg struct{}
type paneTriggerMsg struct{ action string }
```

Replace the `preparedMsg` success arm, currently at `internal/tui/tui.go:134`:

```go
		m.status = "Exercise running."
		if m.service.Config.Classic {
			return m, tea.ExecProcess(msg.play.Command(), func(err error) tea.Msg { return exitedMsg{msg.play, err} })
		}
		session, err := pane.Start(msg.play.Command(), m.width, max(1, m.height-6))
		if err != nil {
			// A pseudo-terminal is not available. Say why, then fall back.
			m.notice = "Embedded pane unavailable (" + err.Error() + "). Using full-screen practice."
			return m, tea.ExecProcess(msg.play.Command(), func(err error) tea.Msg { return exitedMsg{msg.play, err} })
		}
		play := msg.play
		life := m.lifecycle
		session.OnTrigger(func(action string) { life.send(paneTriggerMsg{action}) })
		m.workbench = newWorkbench(session, m.challenge.Title, m.challenge.Objective, m.challenge.Brief)
		return m, tea.Batch(
			paneTick(),
			life.command(func() tea.Msg { return exitedMsg{play, session.Wait()} }),
		)
```

`app.Play` carries only `Session`, `SessionID`, and `Attempt` (`internal/app/app.go:46-50`). It has no challenge. The challenge is already on the model as `m.challenge *model.Challenge`, whose fields are `Title`, `Objective`, and `Brief` (`internal/model/model.go`). `m.challenge` is non-nil on this path because the exercise screen sets it before launching, but guard it anyway and fall back to classic if it is nil.

Add the repaint ticker beside the other commands:

```go
// paneTick repaints the pane on a fixed cadence, so a noisy child cannot
// saturate the Bubble Tea message loop.
func paneTick() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return paneTickMsg{} })
}
```

Handle the new messages in `Update`:

```go
	case paneTickMsg:
		if m.workbench == nil {
			return m, nil
		}
		return m, paneTick()
	case paneTriggerMsg:
		return m.workbenchAction(msg.action)
```

Route keys while the workbench is active. Add this as the first statement in the `case tea.KeyPressMsg:` arm:

```go
		if m.workbench != nil {
			handled, action := m.workbench.update(msg)
			if !handled || action == "" {
				return m, nil
			}
			return m.workbenchAction(action)
		}
```

Forward resize. Add this to the `tea.WindowSizeMsg` arm after the existing width and height assignment:

```go
		if m.workbench != nil && m.workbench.session != nil {
			m.workbench.session.Resize(m.width, m.workbench.paneHeight(m.height))
		}
```

Add the action dispatcher:

```go
func (m Model) workbenchAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "check":
		m.workbench.check = "running"
		return m.perform("checknow")
	case "hint":
		m.workbench.hints++
		return m.perform("hint")
	case "reveal":
		// Revealing records assistance, so it keeps its confirmation step.
		// TestRevealAndAbandonRequireConfirmation enforces this.
		m.confirm = "reveal"
		return m, nil
	case "quit":
		return m, tea.Quit
	}
	return m, nil
}
```

There is no `m.hint()` or `m.reveal()`. The existing mechanism is `func (m Model) perform(action string) (tea.Model, tea.Cmd)` at `internal/tui/tui.go:607`, which already sets `busy`, sets the status, runs through `m.lifecycle.command`, and returns an `operationMsg`. Its action switch at `internal/tui/tui.go:648` handles `check`, `hint`, `reveal`, `abandon`, and `assistance`. Reuse it rather than hand-rolling a command.

The exercise screen binds these at `internal/tui/tui.go:407-412`: `c` calls `perform("check")`, `h` calls `perform("hint")`, and `v` sets `m.confirm = "reveal"`.

Close the pane in the `exitedMsg` arm, before its existing body:

```go
		if m.workbench != nil {
			m.workbench.session.Close()
			m.workbench = nil
		}
```

- [ ] **Step 6: Render the workbench**

In `internal/tui/layout.go`, as the first statement of `render()`:

```go
	if m.workbench != nil {
		return m.workbench.view(m.width, m.height, m.styles(), m.border(), m.plain())
	}
```

- [ ] **Step 7: Place the real cursor**

`tea.View` carries an optional `Cursor *tea.Cursor` that the runtime renders on top of the content, so the child's cursor is a real terminal cursor with its own shape and blink rather than a painted cell. Change `View()` in `internal/tui/layout.go`:

```go
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	if m.workbench != nil && m.workbench.session != nil {
		x, y := m.workbench.session.Cursor()
		// Offset by the band rows the pane is drawn below.
		v.Cursor = tea.NewCursor(x, y+m.workbench.bandRows())
	}
	return v
}
```

Add the matching helper to `internal/tui/workbench.go`, and a test asserting both values:

```go
// bandRows is the number of rows the band occupies above the pane.
func (w *workbench) bandRows() int {
	if w.band {
		return 4
	}
	return 1
}
```

```go
func TestWorkbenchBandRowsMatchPaneHeight(t *testing.T) {
	w := newWorkbench(nil, "t", "g", "b")
	if w.bandRows()+w.paneHeight(24)+1 != 24 {
		t.Errorf("band %d + pane %d + status 1 != 24", w.bandRows(), w.paneHeight(24))
	}
	w.band = false
	if w.bandRows()+w.paneHeight(24)+1 != 24 {
		t.Errorf("collapsed: band %d + pane %d + status 1 != 24", w.bandRows(), w.paneHeight(24))
	}
}
```

- [ ] **Step 8: Add the flag**

In `cmd/golf/main.go`, add a `--classic` global flag that sets `Config.Classic` for the process, and document it in `golf help`.

- [ ] **Step 9: Run the tests**

Run: `go test ./internal/tui/ ./internal/pane/ -v`
Expected: PASS.

- [ ] **Step 10: Verify by hand**

```bash
just build && ./golf
```

Open a Bash exercise. Expected: the brief band is visible above a working shell prompt, typing works, `F12` opens the palette, `esc` closes it, resizing the terminal reflows the child, and `exit` returns to the TUI with a check result.

- [ ] **Step 11: Commit**

```bash
git add internal/tui internal/app cmd/golf
git commit -m "feat(tui): run native exercises in an embedded pane with the brief visible"
```

---

### Task 8: Docker mode

The Docker path returns `docker exec -it ...` from `internal/runner/runner.go:246`. Attaching it to a pseudo-terminal needs no production change, because the kernel raises `SIGWINCH` on the `docker` client, which forwards the resize over the API. This task proves that.

**Files:**
- Test: `internal/runner/runner_test.go`

**Interfaces:**
- Consumes: Task 7's wiring and `pane.Start` from Task 3.
- Produces: no new exported symbols.

- [ ] **Step 1: Write the failing test**

Follow the existing integration test style in `internal/runner/runner_test.go`, calling `integrationRunner(t)` (`internal/runner/runner_test.go:85`), which skips unless `GOLF_INTEGRATION=1` and returns a `*Runner` plus a `model.Attempt`. Prepare a shell exercise through the Docker runner, drive `Session.Command()` through `pane.Start`, and assert the rendered screen contains the banner `Driving Range.` that `runtime/golf-shell` prints. Then call `Resize` and assert the child observes the new width.

- [ ] **Step 2: Run the test**

Run: `go test ./internal/runner/ -run TestDockerSession -v`
Expected: FAIL with Docker running, SKIP without.

- [ ] **Step 3: Make it pass**

Task 7 already routes every prepared session through `pane.Start`, so no production change should be required. If the test passes with no source edit, that is the correct outcome. If the resize assertion fails, add `--env COLUMNS` and `--env LINES` handling in `completionArgs` at `internal/runner/runner.go:330` and re-run.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/runner/ -run TestDockerSession -v`
Expected: PASS.

- [ ] **Step 5: Verify by hand**

```bash
./golf doctor && ./golf
```

Open a Docker-backed exercise, resize the terminal, confirm the child reflows.

- [ ] **Step 6: Commit**

```bash
git add internal/runner
git commit -m "test(runner): cover Docker sessions driven through a pseudo-terminal"
```

---

### Task 9: Runtime shims for the trigger channel

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/native.go`
- Create: `runtime/golf-check`, `runtime/golf-hint`
- Modify: `runtime/Dockerfile`

**Interfaces:**
- Consumes: `pane.TriggerOSC` from Task 4, `Service.CheckNow` from Task 6.
- Produces: `golf-check` and `golf-hint` on the exercise `PATH`, and `GOLF_WORKBENCH` in the child environment.

- [ ] **Step 1: Write the Docker shims**

`runtime/golf-check`:

```sh
#!/bin/bash
if [ -z "$GOLF_WORKBENCH" ]; then
  echo "golf-check needs the embedded workbench. Exit the exercise to check your work."
  exit 1
fi
printf '\033]9270;golf=check\007'
```

`runtime/golf-hint` is the same with `golf=hint` and its own message.

Both need mode `0755` and a `COPY` line in `runtime/Dockerfile` beside the existing `golf-brief` and `golf-shell`.

- [ ] **Step 2: Write the native shims**

In `internal/runner/native.go`, beside the existing `golf-brief` write at line 124, write `golf-check` and `golf-hint` into the same `bin` directory with the same content and mode `0700`.

- [ ] **Step 3: Export the environment variable**

Set `GOLF_WORKBENCH=1` in the child environment when the workbench is active, and leave it unset in classic mode. For the native path this joins the existing `cmd.Env` assignment in `internal/runner/native.go`. For Docker it is an `--env` argument in `completionArgs` at `internal/runner/runner.go:330`.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/runner/ -v`
Expected: PASS.

- [ ] **Step 5: Verify by hand**

```bash
just build && ./golf setup && ./golf
```

Open a shell exercise, run `golf-check` at the prompt. Expected: the status line updates, the sequence does not appear on screen, and the shell keeps running.

- [ ] **Step 6: Commit**

```bash
git add internal/runner runtime
git commit -m "feat(runtime): add golf-check and golf-hint shims for the trigger channel"
```

---

### Task 10: Documentation

**Files:**
- Modify: `DESIGN.md`, `SECURITY.md`, `README.md`

**Interfaces:**
- Consumes: the behavior shipped in Tasks 1 through 9.
- Produces: no code.

- [ ] **Step 1: Revise DESIGN.md**

Replace "Child tools own the entire terminal and every key during an exercise. Restore the TUI after they exit." with a description of the workbench: the child owns a pane and every key except `F12`, the brief band stays visible and collapses to one line, and classic full-terminal handoff remains available.

- [ ] **Step 2: Revise SECURITY.md**

The claim at line 21, "Interactive child output goes directly to your terminal; only output returning to the outer TUI is sanitized", is false under the workbench. Record that child output now passes through a virtual terminal emulator that parses bytes into cells, that escape sequences from the child cannot reach the host terminal, that OSC 52 clipboard writes are dropped, and that classic mode retains the old behavior.

- [ ] **Step 3: Revise README.md**

Update the practice walkthrough: the brief stays visible, `F12` opens the palette, `golf-check` and `golf-hint` work at the exercise prompt, and checking no longer requires exiting. Document `--classic`.

- [ ] **Step 4: Verify**

```bash
rg -n '\x{2014}|\x{2013}' DESIGN.md SECURITY.md README.md ; go test ./...
```

Expected: no dash separators reported, all tests pass.

- [ ] **Step 5: Commit**

```bash
git add DESIGN.md SECURITY.md README.md
git commit -m "docs: describe the embedded workbench and its terminal ownership"
```
