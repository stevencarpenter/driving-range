package tui

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stevencarpenter/driving-range/internal/pane"
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
	for _, c := range []struct {
		key    rune
		action string
	}{{'c', "check"}, {'h', "hint"}, {'v', "reveal"}, {'q', "quit"}} {
		w := newWorkbench(nil, "t", "g", "b")
		w.update(tea.KeyPressMsg{Code: tea.KeyF12})
		handled, action := w.update(tea.KeyPressMsg{Code: c.key, Text: string(c.key)})
		if !handled || action != c.action {
			t.Errorf("palette %q = (%v, %q), want (true, %q)", c.key, handled, action, c.action)
		}
		if w.palette {
			t.Errorf("palette must close after %q", c.key)
		}
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

func TestWorkbenchPaletteUnknownKeyClosesWithoutActing(t *testing.T) {
	w := newWorkbench(nil, "t", "g", "b")
	w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	handled, action := w.update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if !handled || action != "" {
		t.Errorf("palette z = (%v, %q), want (true, \"\")", handled, action)
	}
	if w.palette {
		t.Error("an unknown key must close the palette rather than swallow the next keystroke")
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

func TestWorkbenchLayoutAlwaysFillsTheTerminalExactly(t *testing.T) {
	m := testModel(t)
	short := "Short goal."
	long := strings.Repeat("A very long objective that certainly wraps across lines. ", 4)
	for _, goal := range []string{short, long} {
		for _, band := range []bool{true, false} {
			for _, size := range [][2]int{{80, 24}, {100, 30}, {60, 16}, {40, 10}} {
				w := newWorkbench(nil, "t", goal, "brief detail that differs")
				w.band = band
				view := w.view(size[0], size[1], m.styles(), m.border(), m.plain())
				if got := len(strings.Split(view, "\n")); got != size[1] {
					t.Errorf("band=%v %v: view is %d lines, want %d", band, size, got, size[1])
				}
				if w.bandRows()+w.paneHeight(size[1])+1 != size[1] {
					t.Errorf("band=%v %v: band %d + pane %d + status 1 != %d",
						band, size, w.bandRows(), w.paneHeight(size[1]), size[1])
				}
			}
		}
	}
}

func TestWorkbenchViewShowsGoalAndKeepsTerminalHeight(t *testing.T) {
	m := testModel(t)
	w := newWorkbench(nil, "vim.change-value", "Change port 8080 to 9090", "Keep the comment unchanged.")
	for _, band := range []bool{true, false} {
		w.band = band
		view := w.view(80, 24, m.styles(), m.border(), m.plain())
		if !strings.Contains(view, "Change port 8080 to 9090") {
			t.Errorf("band=%v hides the goal:\n%s", band, view)
		}
		if got := len(strings.Split(view, "\n")); got != 24 {
			t.Errorf("band=%v view is %d lines, want 24", band, got)
		}
	}
}

func TestWorkbenchPlainViewHasNoEscapes(t *testing.T) {
	m := testModel(t)
	m.service.Config.Theme = "plain"
	w := newWorkbench(nil, "t", "goal text", "brief")
	if view := w.view(80, 24, m.styles(), m.border(), true); strings.Contains(view, "\x1b") {
		t.Errorf("plain workbench view contains escapes: %q", view)
	}
}

func preparedModel(t *testing.T, classic bool) Model {
	t.Helper()
	m := testModel(t)
	m.service.Config.Classic = classic
	c := m.service.Catalog.Challenges[0]
	m.challenge = &c
	return m
}

func TestUseWorkbenchRouting(t *testing.T) {
	cases := []struct {
		name      string
		classic   bool
		challenge bool
		want      bool
	}{
		{"default embeds the exercise", false, true, true},
		{"classic opts out", true, true, false},
		{"no challenge means no brief to show", false, false, false},
		{"classic wins over everything", true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := testModel(t)
			m.service.Config.Classic = c.classic
			if c.challenge {
				ch := m.service.Catalog.Challenges[0]
				m.challenge = &ch
			}
			if got := m.useWorkbench(); got != c.want {
				t.Errorf("useWorkbench() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestWorkbenchKeysReachTheChildAndDoNotNavigate(t *testing.T) {
	m := preparedModel(t, false)
	session, err := pane.Start(exec.Command("/bin/cat"), 40, 6)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	m.workbench = newWorkbench(session, "t", "g", "b")
	m.screen = practice
	before := m.screen

	// A key that normally navigates must not, while an exercise is running.
	updated, _ := m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	if got := updated.(Model).screen; got != before {
		t.Errorf("key navigated to screen %d while the workbench was active", got)
	}
	// ctrl+c must not quit golf out from under a running exercise.
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd != nil {
		t.Error("ctrl+c must reach the child rather than producing a golf command")
	}
}

func TestWorkbenchRenderReplacesTheNormalView(t *testing.T) {
	m := preparedModel(t, false)
	m.workbench = newWorkbench(nil, "vim.change-value", "Change the port", "brief")
	m.width, m.height = 80, 24
	view := m.render()
	if !strings.Contains(view, "Change the port") {
		t.Errorf("workbench view missing the goal:\n%s", view)
	}
	if strings.Contains(view, "DRIVING RANGE") {
		t.Error("workbench view should replace the navigation chrome, not sit under it")
	}
}

func TestWorkbenchViewRendersTheStatusLine(t *testing.T) {
	m := testModel(t)
	w := newWorkbench(nil, "t", "goal", "brief")
	view := w.view(80, 24, m.styles(), m.border(), m.plain())
	lines := strings.Split(view, "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "F12") {
		t.Errorf("last line is not the status line: %q", last)
	}
	if !strings.Contains(last, "CHECK") {
		t.Errorf("status line omits the check state: %q", last)
	}
}

func TestWorkbenchBandDoesNotRepeatTheObjective(t *testing.T) {
	m := testModel(t)
	goal := "Read names.txt and print each line."
	// DESIGN.md: omit the brief's opening paragraph when it exactly repeats
	// the displayed objective.
	w := newWorkbench(nil, "t", goal, goal+"\n\nMore detail follows.")
	view := w.view(80, 24, m.styles(), m.border(), true)
	if n := strings.Count(view, "Read names.txt"); n != 1 {
		t.Errorf("objective appears %d times in the band, want 1:\n%s", n, view)
	}
}

func TestWorkbenchResultLandsOnTheStatusLine(t *testing.T) {
	m := testModel(t)
	w := newWorkbench(nil, "t", "g", "b")
	w.result("FAIL: one fixture differed\nexpected x, got y", nil)
	view := w.view(80, 24, m.styles(), m.border(), true)
	if !strings.Contains(view, "FAIL: one fixture differed") {
		t.Errorf("check result missing from the status line:\n%s", view)
	}
	if strings.Contains(view, "expected x, got y") {
		t.Error("status line must stay one line, not spill the details")
	}
	w.result("", errTest)
	if !strings.Contains(w.check, "error: boom") {
		t.Errorf("error not surfaced: %q", w.check)
	}
}

var errTest = errors.New("boom")
