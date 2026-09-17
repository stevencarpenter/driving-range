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

// paletteKeys maps a palette key to the action it emits. Toggling the band is
// handled separately because it changes the workbench rather than the attempt.
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
// the action the operator chose. An unconsumed key belongs to the child, which
// has already received it.
func (w *workbench) update(msg tea.Msg) (bool, string) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, ""
	}
	if w.palette {
		name := key.String()
		w.palette = false
		switch name {
		case "esc":
			return true, ""
		case "b":
			w.band = !w.band
			return true, ""
		}
		if action, found := paletteKeys[name]; found {
			return true, action
		}
		// An unknown key closes the palette without acting, so a stray press
		// never silently swallows the next keystroke.
		return true, ""
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

// bandRows is the number of rows the band occupies above the pane.
func (w *workbench) bandRows() int {
	if w.band {
		return 4 // two content lines plus a top and bottom border
	}
	return 1
}

// paneHeight is the rows left for the child after the band and status line.
func (w *workbench) paneHeight(height int) int {
	return max(1, height-w.bandRows()-1)
}

func (w *workbench) view(width, height int, st viewStyles, border lipgloss.Border, plain bool) string {
	var parts []string
	if w.band {
		box := st.goal.Border(border).Padding(0, 1).Width(width)
		parts = append(parts, paint(box, "GOAL  "+w.goal+"\n"+firstLine(w.brief)))
	} else {
		parts = append(parts, paint(st.muted, "GOAL  "+w.goal))
	}
	screen := ""
	if w.session != nil {
		screen = w.session.Render()
		if plain {
			// The child emits whatever it likes; plain mode promises none of it.
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
