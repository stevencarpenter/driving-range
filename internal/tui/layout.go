package tui

import (
	"fmt"
	"strings"

	bar "charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stevencarpenter/driving-range/internal/model"
)

// View satisfies tea.Model. Bubble Tea v2 carries alternate screen state on
// the view rather than as a program option.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	// Position the terminal cursor over the visible child pane. The offset is
	// the rows the band occupies.
	if m.workbench != nil && m.workbench.session != nil && m.confirm == "" && !m.workbench.outputOpen && !m.workbench.palette {
		x, y := m.workbench.session.Cursor()
		if m.height > 1 && x >= 0 && x < m.width && y >= 0 && y < m.workbench.paneHeight(m.height) {
			v.Cursor = tea.NewCursor(x, y+m.workbench.bandRows())
		}
	}
	return v
}

func (m Model) render() string {
	if m.workbench != nil {
		return m.workbench.view(m.width, m.height, m.styles(), m.border(), m.plain(), m.confirm != "")
	}
	s := m.styles()
	width, height := m.contentWidth(), m.contentHeight()
	lines := m.contentLines()
	offset := min(max(0, m.offset), max(0, len(lines)-height))
	if (m.listNavigating() || m.chooseTrack) && !m.help && m.confirm == "" {
		for i, line := range lines {
			if strings.HasPrefix(ansi.Strip(line), "> ") {
				if i < offset {
					offset = i
				}
				if i >= offset+height {
					offset = i - height + 1
				}
				break
			}
		}
	}
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	vp.SetContent(strings.Join(lines, "\n"))
	vp.SetYOffset(offset)
	content := vp.View()
	if m.confirm != "" && len(lines) < height {
		content = lipgloss.PlaceVertical(height, lipgloss.Center, strings.Join(lines, "\n"))
	}
	body := content
	if m.framed() {
		body = m.panel(content, m.paneWidth(), height+2)
	} else {
		var padded []string
		for _, line := range strings.Split(content, "\n") {
			padded = append(padded, strings.Repeat(" ", m.margin())+line)
		}
		body = strings.Join(padded, "\n")
	}
	if side := m.sidebarWidth(); side > 0 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, s.canvas.Width(2).Height(height+2).Render(""), m.panel(m.sidebar(side-4, height), side, height+2))
	}
	brand := paint(s.brand, " DRIVING RANGE ")
	if m.width >= 60 {
		subtitle := paint(s.nav, "  Daily terminal practice")
		date := paint(s.badge, m.now().Format("Mon 02 Jan"))
		brand += subtitle + strings.Repeat(" ", max(1, m.width-lipgloss.Width(brand)-lipgloss.Width(subtitle)-lipgloss.Width(date))) + date
	}
	var tabs []string
	for i, label := range []string{"Today", "Practice", "Progress", "Settings"} {
		text := fmt.Sprintf(" %d %s ", i+1, label)
		if m.width < 46 {
			text = fmt.Sprintf(" %d ", i+1)
		}
		style := s.nav
		if m.topLevel() == screen(i) {
			style = s.selected
			text = "[" + strings.TrimSpace(text) + "]"
		}
		tabs = append(tabs, paint(style, text))
	}
	status := m.status
	if status == "" {
		status = "Local history. No account required."
		if len(lines) > height {
			status = fmt.Sprintf("Lines %d-%d of %d   j/k scroll", offset+1, min(offset+height, len(lines)), len(lines))
		}
		if m.listNavigating() {
			count := len(m.records)
			if m.screen == practice {
				count = len(m.filtered())
			}
			status = fmt.Sprintf("%d of %d selected   j/k move", min(m.selected+1, count), count)
		}
	}
	status = paint(s.nav, status)
	if m.busy {
		spin := m.spinner
		if m.plain() {
			status = paint(s.nav, "Working: "+m.status)
		} else {
			status = paint(s.key.Background(s.canvas.GetBackground()), spin.View()) + " " + status
		}
	}
	footer := strings.Repeat(" ", m.margin()) + m.footerKeys(m.keyHelp())
	output := []string{brand, strings.Join(tabs, "  "), "", body, strings.Repeat(" ", m.margin()) + status, footer, ""}
	// Compose the entire terminal, including the space between panels, in one surface.
	rendered := strings.Split(strings.Join(output, "\n"), "\n")
	for i, line := range rendered {
		rendered[i] = onSurface(s.canvas, s.canvas.Width(m.width).Render(ansi.Truncate(line, m.width, "")))
	}
	if len(rendered) > m.height {
		rendered = rendered[:max(1, m.height)]
	}
	return strings.Join(rendered, "\n")
}

func (m Model) panel(content string, width, height int) string {
	s := m.styles()
	return s.surface.Border(m.border()).BorderForeground(s.border.GetForeground()).
		BorderBackground(s.canvas.GetBackground()).Padding(0, 1).Width(width).Height(height).Render(onSurface(s.surface, content))
}

// styleReset is the sequence Lip Gloss v2 emits to close a style. v1 wrote the
// longer "\x1b[0m"; matching the wrong one turns onSurface into a silent no-op.
const styleReset = "\x1b[m"

// Lip Gloss resets nested styles to the terminal default. Restore the
// containing surface after each child reset so gaps keep the panel background.
func onSurface(style lipgloss.Style, content string) string {
	prefix := strings.TrimSuffix(style.Render(""), styleReset)
	if prefix == "" {
		return content
	}
	return style.Render(strings.ReplaceAll(content, styleReset, styleReset+prefix))
}

func (m Model) selectedChallenge() *model.Challenge {
	switch m.screen {
	case today:
		c, _ := m.todayChallenge()
		return c
	case practice:
		items := m.filtered()
		if len(items) > 0 {
			c := items[min(m.selected, len(items)-1)]
			return &c
		}
	case exercise:
		return m.challenge
	case progress, detail:
		a := m.currentAttempt()
		if m.screen == progress && len(m.records) > 0 {
			a = &m.records[min(m.selected, len(m.records)-1)].Attempt
		}
		if a != nil {
			if c, err := m.service.Catalog.Find(a.ExerciseID, a.Revision); err == nil {
				return &c
			}
		}
	}
	return nil
}

func (m Model) sidebar(width, height int) string {
	s := m.styles()
	label := func(text string) string { return paint(s.section, text) }
	var lines []string
	c := m.selectedChallenge()
	if c != nil {
		if m.screen == practice || m.screen == progress || m.screen == detail {
			action := "[Enter] Open full brief"
			if m.screen == progress {
				action = "[Enter] Attempt detail"
			}
			lines = append(lines, label("SELECTED EXERCISE"), "", paint(s.title, c.Title), "", label("GOAL"), paint(s.key, c.Objective), "", paint(s.muted, action), "")
		} else {
			lines = append(lines, label("YOUR SESSION"), "", paint(s.title, strings.ToUpper(c.Track)), "",
				paint(s.badge, fmt.Sprintf("%d MIN", c.Minutes))+" "+paint(s.badge, fmt.Sprintf("LEVEL %d/5", c.Difficulty)), "", label("WORKFLOW"), "",
				paint(s.text, "1  Read the goal"), paint(s.text, "2  Practice with your tools"), paint(s.text, "3  Exit to check your work"), "")
		}
		lines = append(lines, label("TOOLKIT"), paint(s.text, strings.Join(c.Tools, "  /  ")), "", label("TRACK PROGRESS"), m.trackProgress(c.Track, width), "")
		if a := m.currentAttempt(); a != nil && (m.screen == exercise || m.screen == detail) {
			lines = append(lines, label("CURRENT ATTEMPT"), paint(s.text, a.Status), paint(s.muted, fmt.Sprintf("%d hints used", a.HintLevel)))
		}
	} else if m.screen == settings {
		lines = append(lines, label("DISPLAY"), "", paint(s.brand, "  Aa  ")+" "+paint(s.action, "  Aa  "), "", paint(s.title, "Theme: "+m.service.Config.Theme),
			paint(s.muted, "[l] Change theme"), "", label("ON THIS MACHINE"), "", paint(s.text, "Native tools. Your dotfiles."), paint(s.muted, "Fixtures and checks use Docker."), "",
			label("YOUR DATA"), "", paint(s.text, "Saved locally"), paint(s.muted, "No account or telemetry."))
	} else {
		lines = append(lines, label("BUILD YOUR PRACTICE"), "", paint(s.text, "Pick a small task. Work in your own tools. Check the result."), "", paint(s.action, " [1] Open Today "))
	}
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	vp.SetContent(ansi.Wrap(strings.Join(lines, "\n"), width, ""))
	return vp.View()
}

func (m Model) trackProgress(track string, width int) string {
	total, solved := 0, 0
	for _, c := range m.service.Catalog.Current() {
		if c.Track != track {
			continue
		}
		total++
		if m.solved(c) {
			solved++
		}
	}
	full, empty := '━', '─'
	if m.plain() {
		full, empty = '#', '-'
	}
	p := bar.New(bar.WithWidth(width), bar.WithColors(lipgloss.Color("#B07AF0"), lipgloss.Color("#75E4CD")), bar.WithFillCharacters(full, empty), bar.WithoutPercentage())
	p.EmptyColor = lipgloss.Color("7")
	fraction := 0.0
	if total > 0 {
		fraction = float64(solved) / float64(total)
	}
	filled := p.ViewAs(fraction)
	if m.plain() {
		// Lip Gloss v2 renders color unconditionally, so plain mode strips it
		// here rather than downgrading a profile the renderer no longer owns.
		filled = ansi.Strip(filled)
	}
	return filled + "\n" + paint(m.styles().muted, fmt.Sprintf("%d of %d exercises solved", solved, total))
}
