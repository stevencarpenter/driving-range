package tui

import (
	"strings"
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
