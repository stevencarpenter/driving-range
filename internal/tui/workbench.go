package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stevencarpenter/driving-range/internal/pane"
)

// workbench renders an active attempt: a brief band above the child pane, a
// status line below, and a palette opened by the single intercepted key.
type workbench struct {
	session            *pane.Session
	title, goal, brief string
	band               bool
	palette            bool
	// bandHeight is the rows the band occupied in the last render. The band
	// wraps, so its height cannot be assumed; the pane size and the cursor
	// offset both depend on the measured value.
	bandHeight int
	// paneW and paneH are the size last pushed to the child, so the session is
	// resized only when the layout actually changes.
	paneW, paneH int
	check        string
	hints        int
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
	return &workbench{session: s, title: title, goal: goal, brief: brief, band: true, check: "not run", bandHeight: 1}
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

// bandRows is the rows the band occupied in the last render.
func (w *workbench) bandRows() int { return max(1, w.bandHeight) }

// paneHeight is the rows left for the child after the band and status line.
func (w *workbench) paneHeight(height int) int {
	return max(1, height-w.bandRows()-1)
}

// bandView renders the brief band and reports how tall it turned out. Long
// goals wrap, so the height is measured rather than assumed.
func (w *workbench) bandView(width int, st viewStyles, border lipgloss.Border) (string, int) {
	var out string
	if w.band {
		box := st.goal.Border(border).Padding(0, 1).Width(width)
		out = paint(box, "GOAL  "+w.goal+w.briefTail())
	} else {
		out = paint(st.muted, ansi.Truncate("GOAL  "+w.goal, max(1, width), "..."))
	}
	return out, lipgloss.Height(out)
}

// briefTail is the brief's opening line, omitted when it merely repeats the
// objective already shown on the GOAL line.
func (w *workbench) briefTail() string {
	line := firstLine(w.brief)
	if line == "" || strings.TrimSpace(line) == strings.TrimSpace(w.goal) {
		return ""
	}
	return "\n" + line
}

func (w *workbench) view(width, height int, st viewStyles, border lipgloss.Border, plain bool) string {
	band, rows := w.bandView(width, st, border)
	// Never let the band crowd the child out; it yields rows before the pane
	// drops below a usable size.
	w.bandHeight = min(rows, max(1, height-6))
	if w.bandHeight < rows {
		band = strings.Join(strings.Split(band, "\n")[:w.bandHeight], "\n")
	}
	screen := ""
	if w.session != nil {
		screen = w.session.Render()
		if plain {
			// The child emits whatever it likes; plain mode promises none of it.
			screen = uiText(screen)
		}
	}
	pane := st.text.Width(width).Height(w.paneHeight(height)).Render(screen)
	return strings.Join([]string{band, pane, w.status(st)}, "\n")
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

// paneRepaint is the repaint cadence for the embedded pane. It decouples the
// child's write rate from the event loop, so a full editor redraw costs one
// frame rather than one message per write.
const paneRepaint = 16 * time.Millisecond

func paneTick() tea.Cmd {
	return tea.Tick(paneRepaint, func(time.Time) tea.Msg { return paneTickMsg{} })
}

// result records the outcome of an action on the status line. The workbench
// view covers the screen, so a result has nowhere else to appear.
func (w *workbench) result(text string, err error) {
	switch {
	case err != nil:
		w.check = "error: " + firstLine(err.Error())
	case text != "":
		w.check = firstLine(text)
	default:
		w.check = "done"
	}
}

// syncPane keeps the child's terminal the same size as the pane it is drawn
// in. The band wraps and can be toggled, so the pane size changes for reasons
// a window resize alone does not cover.
func (w *workbench) syncPane(width, height int) {
	if w.session == nil {
		return
	}
	h := w.paneHeight(height)
	if width == w.paneW && h == w.paneH {
		return
	}
	w.paneW, w.paneH = width, h
	w.session.Resize(width, h)
}

// useWorkbench reports whether an exercise should run in an embedded pane.
// The classic setting opts out, and a missing challenge means there is no
// brief to show beside the child, so the pane would have nothing to add.
func (m Model) useWorkbench() bool {
	return !m.service.Config.Classic && m.challenge != nil
}

// workbenchAction runs the action the operator chose, from the palette or from
// a golf-check or golf-hint trigger inside the exercise.
func (m Model) workbenchAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "check":
		m.workbench.check = "running"
		return m.perform("checknow")
	case "hint":
		m.workbench.hints++
		return m.perform("hint")
	case "reveal":
		// Revealing records assistance, so it keeps the confirmation the
		// exercise screen requires.
		m.confirm = "reveal"
		return m, nil
	case "quit":
		return m, tea.Quit
	}
	return m, nil
}
