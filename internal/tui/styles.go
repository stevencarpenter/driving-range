package tui

import (
	"os"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type viewStyles struct {
	text, title, section, muted, selected, action, danger, warning, key, nav lipgloss.Style
	canvas, surface, brand, badge, goal, border                              lipgloss.Style
}

func (m Model) plain() bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	return noColor || m.service.Config.Theme == "plain"
}

func (m Model) styles() viewStyles {
	base := lipgloss.NewStyle()
	s := viewStyles{text: base, title: base, section: base, muted: base, selected: base, action: base,
		danger: base, warning: base, key: base, nav: base, canvas: base, surface: base, brand: base, badge: base.Padding(0, 1), goal: base, border: base}
	if m.plain() {
		return s
	}
	ink, muted, canvas, surface := "#EEE9FA", "#B6ABC9", "#15111F", "#221B30"
	purple, teal, selection, edge := "#C5A2FF", "#75E4CD", "#7045AF", "#65517F"
	goalBG, goalFG := "#123F3C", "#C3FFED"
	if m.service.Config.Theme == "light" || (m.service.Config.Theme == "auto" && !m.darkBackground) {
		ink, muted, canvas, surface = "#302442", "#655478", "#EEE8F8", "#FAF7FF"
		purple, teal, selection, edge = "#6B349A", "#176953", "#7045AF", "#AC95C4"
		goalBG, goalFG = "#D7F4E9", "#164F3F"
	}
	s.canvas = base.Foreground(lipgloss.Color(ink)).Background(lipgloss.Color(canvas))
	s.surface = base.Foreground(lipgloss.Color(ink)).Background(lipgloss.Color(surface))
	s.text = s.surface
	s.title = s.text.Bold(true)
	s.muted = s.surface.Foreground(lipgloss.Color(muted))
	s.section = s.surface.Foreground(lipgloss.Color(purple)).Bold(true)
	s.selected = base.Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color(selection)).Bold(true)
	s.brand = s.selected
	s.action = base.Foreground(lipgloss.Color("#13382D")).Background(lipgloss.Color("#75E4CD")).Bold(true)
	s.key = s.surface.Foreground(lipgloss.Color(teal)).Bold(true)
	s.badge = s.section.Background(lipgloss.Color(canvas)).Padding(0, 1)
	s.nav = s.canvas.Foreground(lipgloss.Color(muted))
	s.goal = base.Foreground(lipgloss.Color(goalFG)).Background(lipgloss.Color(goalBG)).Bold(true)
	s.border = s.canvas.Foreground(lipgloss.Color(edge))
	s.danger = s.surface.Foreground(lipgloss.Color("#FF7899")).Bold(true)
	s.warning = s.surface.Foreground(lipgloss.Color("#E9AE4D")).Bold(true)
	if goalBG == "#D7F4E9" {
		s.danger = s.surface.Foreground(lipgloss.Color("#A62547")).Bold(true)
		s.warning = s.surface.Foreground(lipgloss.Color("#825000")).Bold(true)
	}
	return s
}

func (m Model) border() lipgloss.Border {
	if m.plain() {
		return lipgloss.ASCIIBorder()
	}
	return lipgloss.RoundedBorder()
}

// Sanitize external text before adding our own terminal styling.
func uiText(text string) string {
	return strings.ReplaceAll(clean(ansi.Strip(text)), "\t", "    ")
}

func paint(style lipgloss.Style, text string) string { return style.Render(uiText(text)) }

func (m Model) margin() int {
	if m.width >= 60 {
		return 2
	}
	if m.width >= 30 {
		return 1
	}
	return 0
}

func (m Model) framed() bool { return m.width >= 60 && m.height >= 16 }

func (m Model) sidebarWidth() int {
	if m.width < 110 || m.height < 24 || m.help || m.chooseTrack || m.confirm != "" || m.notice != "" || m.errorText != "" {
		return 0
	}
	return min(42, m.width/3)
}

func (m Model) paneWidth() int {
	if side := m.sidebarWidth(); side > 0 {
		return m.width - side - 2
	}
	return m.width
}

func (m Model) contentWidth() int { return max(1, m.paneWidth()-2*m.margin()) }

func (m Model) goal(objective string) string {
	styles := m.styles()
	if m.contentWidth() < 8 {
		return paint(styles.title, "GOAL\n"+objective)
	}
	// Lip Gloss v2 counts border and padding inside Width; v1 excluded them.
	box := styles.goal.Border(m.border()).
		BorderForeground(styles.key.GetForeground()).Padding(0, 1).
		Width(m.contentWidth())
	return paint(box, "GOAL\n"+objective)
}

func (m Model) section(label string) string {
	label = uiText(label)
	s := m.styles()
	rule := "─"
	if m.plain() {
		rule = "-"
	}
	return paint(s.section, label) + " " + paint(s.border.Background(s.surface.GetBackground()), strings.Repeat(rule, max(0, m.contentWidth()-ansi.StringWidth(label)-1)))
}

func (m Model) primary(text string) string {
	return paint(m.styles().action, " "+text+" ")
}

func (m Model) tableRow(cells []string, widths []int, selected, header bool) string {
	s := m.styles()
	parts := make([]string, len(cells))
	for i, cell := range cells {
		cell = strings.ReplaceAll(uiText(cell), "\n", " ")
		cell = ansi.Truncate(cell, max(1, widths[i]), "~")
		style := s.text
		if i > 0 {
			style = s.muted
		}
		switch cell {
		case "solved", "pass":
			style = s.key
		case "active", "interrupted":
			style = s.warning
		case "infrastructure_error", "fail":
			style = s.danger
		}
		if header {
			style = s.section
		}
		if selected {
			style = s.selected
		}
		parts[i] = style.Width(max(1, widths[i])).Render(cell)
	}
	prefix := "  "
	if selected {
		prefix = "> "
		return s.selected.Render(prefix + strings.Join(parts, s.selected.Render("  ")))
	}
	return prefix + strings.Join(parts, "  ")
}

func (m Model) listRow(title, track, difficulty, status string, selected, header bool) string {
	width := m.contentWidth()
	if width < 60 {
		if width < 30 {
			return m.tableRow([]string{title}, []int{max(1, width-2)}, selected, header)
		}
		return m.tableRow([]string{title, status}, []int{width - 19, 15}, selected, header)
	}
	return m.tableRow([]string{title, track, difficulty, status}, []int{width - 42, 9, 5, 20}, selected, header)
}

func (m Model) footerKeys(text string) string {
	s := m.styles()
	h := help.New()
	h.SetWidth(max(1, m.width-2*m.margin()))
	h.ShortSeparator = "   "
	h.Ellipsis = "..."
	h.Styles.ShortKey = s.key.Background(s.canvas.GetBackground())
	h.Styles.ShortDesc = s.nav
	h.Styles.ShortSeparator = s.nav
	h.Styles.Ellipsis = s.nav
	var bindings []key.Binding
	for _, part := range strings.Split(text, "  ") {
		k, label, found := strings.Cut(strings.TrimSpace(part), " ")
		if found {
			bindings = append(bindings, key.NewBinding(key.WithKeys(k), key.WithHelp(k, label)))
		}
	}
	return h.ShortHelpView(bindings)
}
