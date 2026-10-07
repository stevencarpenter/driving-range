package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stevencarpenter/driving-range/internal/pane"
)

// workbench renders an active attempt: a brief band above the child pane, a
// status line below, and a palette opened by the single intercepted key.
type workbench struct {
	session     *pane.Session
	goal, brief string
	band        bool
	palette     bool
	// bandHeight is the rows the band occupied in the last render. The band
	// wraps, so its height cannot be assumed; the pane size and the cursor
	// offset both depend on the measured value.
	bandHeight int
	// paneW and paneH are the size last pushed to the child, so the session is
	// resized only when the layout actually changes.
	paneW, paneH int
	check        string
	output       viewport.Model
	outputOpen   bool
}

// paletteKeys maps a palette key to the action it emits. Toggling the band is
// handled separately because it changes the workbench rather than the attempt.
var paletteKeys = map[string]string{
	"c": "check",
	"h": "hint",
	"v": "reveal",
	"q": "quit",
}

func newWorkbench(s *pane.Session, goal, brief string) *workbench {
	output := viewport.New(viewport.WithWidth(1), viewport.WithHeight(1))
	output.SoftWrap, output.FillHeight = true, true
	return &workbench{session: s, goal: goal, brief: brief, band: true, check: "not run", bandHeight: 1, output: output}
}

// update routes focused keys and returns the selected golf action.
func (w *workbench) update(key tea.KeyPressMsg) string {
	if w.outputOpen {
		switch key.String() {
		case "esc":
			w.outputOpen = false
		case "home":
			w.output.GotoTop()
		case "end":
			w.output.GotoBottom()
		default:
			w.output, _ = w.output.Update(key)
		}
		return ""
	}
	if w.palette {
		name := key.String()
		w.palette = false
		switch name {
		case "b":
			w.band = !w.band
			return ""
		case "r":
			w.outputOpen = w.output.GetContent() != ""
			return ""
		}
		// An unknown key closes the palette without acting, so a stray press
		// never silently swallows the next keystroke.
		return paletteKeys[name]
	}
	if key.String() == "f12" {
		w.palette = true
		return ""
	}
	if w.session != nil {
		w.session.SendKey(key)
	}
	return ""
}

// bandRows is the rows the band occupied in the last render.
func (w *workbench) bandRows() int { return max(0, w.bandHeight) }

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

func (w *workbench) view(width, height int, st viewStyles, border lipgloss.Border, plain, confirming bool) string {
	band, rows := w.bandView(width, st, border)
	// Never let the band crowd the child out; it yields rows before the pane
	// drops below a usable size.
	w.bandHeight = min(rows, max(1, height-6))
	if height < 3 {
		w.bandHeight = 0
	}
	lines := strings.Split(band, "\n")[:w.bandHeight]
	screen := ""
	switch {
	case confirming:
		screen = ansi.Wrap(paint(st.warning, "Reveal the reference solution?\nThis records assistance."), width, "")
	case w.outputOpen:
		w.output.SetWidth(width)
		w.output.SetHeight(w.paneHeight(height))
		w.output.SetYOffset(w.output.YOffset())
		screen = w.output.View()
	case w.session != nil:
		screen = w.session.Render()
		if plain {
			// The child emits whatever it likes; plain mode promises none of it.
			screen = uiText(screen)
		}
	}
	pane := st.text.Width(width).Height(w.paneHeight(height)).Render(screen)
	if height > 1 {
		lines = append(lines, strings.Split(pane, "\n")[:w.paneHeight(height)]...)
	}
	lines = append(lines, w.status(st, width, confirming))
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
	}
	return strings.Join(lines, "\n")
}

func (w *workbench) status(st viewStyles, width int, confirming bool) string {
	if confirming {
		if width < 30 {
			return paint(st.action, "y yes n no")
		}
		return paint(st.action, " y reveal   n / Esc cancel ")
	}
	if w.outputOpen {
		if width < 40 {
			return paint(st.nav, "Esc back  j/k scroll")
		}
		return paint(st.nav, fmt.Sprintf(" Esc back   j/k scroll   PgUp/PgDn   %.0f%%", 100*w.output.ScrollPercent()))
	}
	if w.palette {
		return paint(st.action, " c check   h hint   v reveal   r result   b brief   q quit   esc back ")
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

// result keeps a one-line summary and opens the full response for reading.
func (w *workbench) result(text string, err error) {
	if err != nil {
		text = "error: " + err.Error()
	} else if text == "" {
		text = "done"
	}
	text = uiText(text)
	w.check = firstLine(text)
	w.output.SetContent("RESULT\n\n" + text)
	w.output.GotoTop()
	w.outputOpen, w.palette = true, false
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
	if action != "quit" && (m.busy || m.confirm != "") {
		return m, nil
	}
	switch action {
	case "check":
		m.workbench.check = "running"
		return m.perform("checknow")
	case "hint":
		return m.perform("hint")
	case "reveal":
		// Revealing records assistance, so it keeps the confirmation the
		// exercise screen requires.
		m.confirm = "reveal"
		m.workbench.outputOpen = false
		return m, nil
	case "quit":
		return m, tea.Quit
	}
	return m, nil
}
