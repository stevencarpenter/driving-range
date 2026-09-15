package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type viewStyles struct {
	text, title, section, muted, selected, action, danger, warning, key, nav lipgloss.Style
}

func (m Model) styles() viewStyles {
	base := m.renderer.NewStyle()
	s := viewStyles{base, base, base, base, base, base, base, base, base, base}
	_, noColor := os.LookupEnv("NO_COLOR")
	if noColor || m.service.Config.Theme == "plain" {
		return s
	}
	ink, muted, accent, panel, section, danger, warning := "#E3EAF2", "#A6B4C5", "#9BE0B0", "#263849", "#91C7ED", "#FFACA7", "#F1CE8B"
	light := m.service.Config.Theme == "light" || (m.service.Config.Theme == "auto" && !m.renderer.HasDarkBackground())
	if light {
		ink, muted, accent, panel, section, danger, warning = "#202D3A", "#536171", "#23633C", "#E1EBF2", "#275F87", "#A2262C", "#805700"
	}
	s.text = base.Foreground(lipgloss.Color(ink))
	s.title = s.text.Bold(true)
	s.section = base.Foreground(lipgloss.Color(section)).Bold(true)
	s.muted = base.Foreground(lipgloss.Color(muted))
	s.selected = s.text.Background(lipgloss.Color(panel)).Bold(true)
	s.action = base.Foreground(lipgloss.Color("#142D20")).Background(lipgloss.Color("#9BE0B0")).Bold(true)
	if light {
		s.action = base.Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color(accent)).Bold(true)
	}
	s.danger = base.Foreground(lipgloss.Color(danger)).Bold(true)
	s.warning = base.Foreground(lipgloss.Color(warning)).Bold(true)
	s.key = base.Foreground(lipgloss.Color(accent)).Bold(true)
	s.nav = s.muted
	return s
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

func (m Model) contentWidth() int { return max(1, min(m.width, 120)-2*m.margin()) }

func (m Model) section(label string) string {
	label = uiText(label)
	width := m.contentWidth()
	return paint(m.styles().section, label+" "+strings.Repeat("-", max(0, width-ansi.StringWidth(label)-1)))
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
	parts := strings.Split(text, "  ")
	for i, part := range parts {
		key, label, found := strings.Cut(strings.TrimSpace(part), " ")
		if found {
			parts[i] = paint(s.key, key) + " " + paint(s.muted, label)
		} else {
			parts[i] = paint(s.muted, part)
		}
	}
	return strings.Join(parts, "  ")
}
