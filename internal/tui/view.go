package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func (m Model) contentLines() []string {
	styles := m.styles()
	body := m.body()
	if m.screen == today || m.screen == settings {
		if warning := m.service.RecoveryWarning(); warning != "" {
			body = paint(styles.warning, "WORKSPACE RECOVERY PENDING") + "\n" + paint(styles.text, warning) + "\n\nBrowsing is available. Starting an exercise retries recovery.\n\n" + body
		}
	}
	if m.notice != "" {
		body = m.section("RESULT") + "\n" + paint(styles.text, m.notice) + "\n\n" + body
	}
	if m.errorText != "" {
		body = paint(styles.danger, "ERROR: "+m.errorText) + "\n\n" + body
	}
	if m.confirm != "" {
		prompt := "Reveal the reference solution? This records assistance."
		if m.confirm == "abandon" {
			prompt = "Abandon this attempt? History stays saved; resume will be disabled."
		}
		body = m.section("CONFIRM ACTION") + "\n\n" + paint(styles.warning, prompt) + "\n\n" + m.primary("[y] Confirm") + "   " + paint(styles.text, "[n / Esc] Cancel")
	}
	if m.help {
		var lines []string
		for _, line := range strings.Split(helpText, "\n") {
			if line != "" && line == strings.ToUpper(line) {
				lines = append(lines, m.section(line))
			} else {
				lines = append(lines, paint(styles.text, line))
			}
		}
		body = strings.Join(lines, "\n")
	}
	if m.chooseTrack {
		body = m.trackPicker()
	}
	return strings.Split(ansi.Wrap(body, m.contentWidth(), ""), "\n")
}

func (m *Model) scroll(delta int) {
	m.offset = max(0, min(m.offset+delta, max(0, len(m.contentLines())-m.contentHeight())))
}

func (m Model) View() string {
	width := m.contentWidth()
	styles := m.styles()
	header := paint(styles.title, "DRIVING RANGE")
	if width >= 50 {
		tagline := "Daily terminal practice"
		header += strings.Repeat(" ", max(1, width-13-len(tagline))) + paint(styles.muted, tagline)
	}
	var tabs []string
	for i, label := range []string{"Today", "Practice", "Progress", "Settings"} {
		style := styles.nav
		text := fmt.Sprintf(" %d %s ", i+1, label)
		if width < 46 {
			text = fmt.Sprintf(" %d ", i+1)
		}
		if m.topLevel() == screen(i) {
			style = styles.selected
			text = "[" + strings.TrimSpace(text) + "]"
		}
		tabs = append(tabs, paint(style, text))
	}
	lines := m.contentLines()
	height := m.contentHeight()
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
	visible := append([]string(nil), lines[offset:min(len(lines), offset+height)]...)
	for len(visible) < height {
		visible = append(visible, "")
	}
	status := m.status
	if status == "" {
		status = "Local history. No account required."
		if len(lines) > height {
			status = fmt.Sprintf("Lines %d-%d of %d", offset+1, min(offset+height, len(lines)), len(lines))
			if m.listNavigating() {
				status += "   j/k select"
			} else {
				status += "   j/k scroll"
			}
		}
	}
	rule := paint(styles.muted, strings.Repeat("-", width))
	result := []string{header, strings.Join(tabs, " "), rule}
	result = append(result, visible...)
	result = append(result, rule, paint(styles.muted, status), m.footerKeys(m.keyHelp()))
	margin := strings.Repeat(" ", m.margin())
	for i, line := range result {
		result[i] = margin + ansi.Truncate(line, width, "")
	}
	// Keep the footer anchored without writing beyond the terminal's last row.
	if len(result) < m.height {
		result = append([]string{""}, result...)
	}
	if len(result) > m.height {
		result = result[:max(1, m.height)]
	}
	return strings.Join(result, "\n")
}

func (m Model) body() string {
	switch m.screen {
	case today:
		return m.todayView()
	case practice:
		return m.practiceView()
	case progress:
		return m.progressView()
	case settings:
		return m.settingsView()
	case exercise:
		return m.exerciseView()
	case detail:
		return m.detailView()
	}
	return ""
}

func (m Model) trackPicker() string {
	styles := m.styles()
	title := "Choose your starting track"
	if m.service.Config.Track != "" {
		title = "Choose your track"
	}
	lines := []string{paint(styles.title, title), paint(styles.muted, "One daily exercise. Change your track anytime in Settings."), "", m.section("TRACKS")}
	for i, track := range m.service.Catalog.Tracks() {
		count := 0
		for _, c := range m.service.Catalog.All() {
			if c.Track == track {
				count++
			}
		}
		lines = append(lines, m.tableRow([]string{track, fmt.Sprintf("%d exercises", count)}, []int{max(1, m.contentWidth()-22), 18}, i == m.trackIndex, false))
	}
	if len(m.service.Catalog.Tracks()) == 0 {
		lines = append(lines, "No tracks are available in this catalog.")
	}
	lines = append(lines, "", m.primary("[Enter] Select track"))
	if m.errorText != "" {
		lines = append(lines, "", paint(styles.danger, "ERROR: "+m.errorText))
	}
	return strings.Join(lines, "\n")
}

func (m Model) todayView() string {
	styles := m.styles()
	c, a := m.todayChallenge()
	if c == nil {
		return "No exercise is available for this track. Press t to choose a track."
	}
	label := "PRACTICE / " + strings.ToUpper(c.Track)
	calendar := "No shared assignment for today's UTC date. This is archive practice."
	if a != nil {
		label = "TODAY / " + strings.ToUpper(c.Track) + " / " + a.Date + " UTC"
		now := m.now()
		next := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day()+1, 0, 0, 0, 0, time.UTC).In(now.Location())
		calendar = "Next daily reset: " + next.Format("Mon 15:04 MST") + " (your local time)."
	}
	lines := []string{paint(styles.title, c.Title), paint(styles.muted, label), "", paint(styles.text, c.Objective), "",
		paint(styles.key, fmt.Sprintf("About %d minutes   |   difficulty %d/5", c.Minutes, c.Difficulty)),
		paint(styles.muted, "Tools: "+strings.Join(c.Tools, ", ")+" | profile: "+c.Profile), "",
		m.primary("> Open exercise [Enter]") + "   " + paint(styles.text, "[t] Choose track"), "", m.section("YOUR PRACTICE")}
	latest := "No attempt yet. Start when you're ready."
	for _, r := range m.records {
		if r.Attempt.ExerciseID == c.ID && r.Attempt.Revision == c.Revision && (a == nil || r.Attempt.MatchesAssignment(*a)) {
			latest = "Latest: " + r.Attempt.Status + " | elapsed exercise time " + duration(r) + fmt.Sprintf(" | %d hints", r.Attempt.HintLevel)
			break
		}
	}
	lines = append(lines, paint(styles.text, latest), "", paint(styles.muted, calendar), paint(styles.muted, "Completed history remains available in Progress."))
	return strings.Join(lines, "\n")
}

func (m Model) practiceView() string {
	styles := m.styles()
	query := m.query
	if m.searching {
		query += "_"
	}
	items := m.filtered()
	lines := []string{paint(styles.title, "PRACTICE / archive") + paint(styles.muted, fmt.Sprintf("   %d exercises", len(items))),
		paint(styles.key, "Search [/]: "+query), paint(styles.muted, "Tool, concept, difficulty:1, solved, missed, or untried."), "",
		m.listRow("EXERCISE", "TRACK", "LEVEL", "STATUS", false, true)}
	if len(items) == 0 {
		return strings.Join(append(lines, "No exercises match. Press / then Ctrl-U to clear the search."), "\n")
	}
	for i, c := range items {
		status := "untried"
		for _, r := range m.records {
			if r.Attempt.ExerciseID == c.ID && r.Attempt.Revision == c.Revision {
				status = r.Attempt.Status
				break
			}
		}
		lines = append(lines, m.listRow(c.Title, c.Track, fmt.Sprintf("%d/5", c.Difficulty), status, i == m.selected, false))
	}
	return strings.Join(lines, "\n")
}

func (m Model) exerciseView() string {
	if m.challenge == nil {
		return "Choose an exercise in Today or Practice."
	}
	styles := m.styles()
	c := m.challenge
	label := "PRACTICE"
	if m.assignment != nil {
		label = "DAILY / " + m.assignment.Date + " UTC (assignment fixed for this attempt)"
	}
	a := m.currentAttempt()
	if a != nil && a.AssignmentDate != "" {
		label = "DAILY / " + a.AssignmentDate + " UTC (assignment fixed for this attempt)"
	}
	lines := []string{paint(styles.title, c.Title), paint(styles.muted, label+" / "+c.Track),
		paint(styles.muted, fmt.Sprintf("%s revision %d | %s | %d minutes", c.ID, c.Revision, c.Profile, c.Minutes)), ""}
	if a != nil && (a.Status == "solved" || a.Status == "abandoned") {
		lines = append(lines, m.primary("[r] New attempt")+"   "+paint(styles.text, "[p] Saved result   [v] Explanation"))
	} else {
		lines = append(lines, m.primary("[Enter] Start / resume")+"   "+paint(styles.text, "[c] Check   [h] Hint"),
			paint(styles.muted, "[v] Reveal solution   [x] Mark external assistance   [a] Abandon"))
	}
	lines = append(lines, "", m.section("GOAL"), paint(styles.text, c.Objective), "", m.section("BRIEF"), paint(styles.text, c.Brief))
	if a != nil {
		lines = append(lines, "", m.section("ATTEMPT"), paint(styles.text, "Status: "+a.Status),
			paint(styles.muted, fmt.Sprintf("Hints: %d | solution revealed: %t | external assistance: %t", a.HintLevel, a.SolutionRevealed, a.ExternalAssistance)))
		for _, r := range m.records {
			if r.Attempt.ID == a.ID {
				lines = append(lines, paint(styles.text, "Elapsed exercise time: "+duration(r)+fmt.Sprintf(" | checks: %d", r.Checks)))
				break
			}
		}
		lines = append(lines, paint(styles.muted, "Attempt: "+a.ID))
	}
	lines = append(lines, "", m.section("SUCCESS CONDITIONS"),
		paint(styles.text, "Validator: "+c.Validator.Kind+" | output: "+c.Validator.OutputPolicy+" | version: "+c.Validator.Version),
		paint(styles.text, "Tools: "+strings.Join(c.Tools, ", ")), paint(styles.text, "Concepts: "+strings.Join(c.Concepts, ", ")))
	if len(c.Fixtures) > 0 && c.Fixtures[0].ExpectedStdout != "" {
		lines = append(lines, "", m.section("EXPECTED OUTPUT (first fixture)"), paint(styles.text, c.Fixtures[0].ExpectedStdout))
	}
	lines = append(lines, "", paint(styles.muted, "Inside the exercise, the real editor or shell owns every key."),
		paint(styles.muted, "Exit the child tool to return and check. Setup and menus are excluded from elapsed exercise time."))
	if a != nil && (a.Status == "solved" || a.SolutionRevealed) {
		lines = append(lines, "", m.section("EXPLANATION"), paint(styles.text, c.Explanation), "", m.section("REFERENCE SOLUTION"), paint(styles.text, c.ReferenceSolution))
	}
	return strings.Join(lines, "\n")
}

func (m Model) progressView() string {
	styles := m.styles()
	lines := []string{paint(styles.title, "PROGRESS / saved locally"), paint(styles.muted, "[Enter] Attempt detail   [e] Export JSON   [c] Export CSV"), ""}
	if len(m.records) == 0 {
		return strings.Join(append(lines, m.section("YOUR PRACTICE"), "No attempts yet. Open Today to begin an exercise.", "", paint(styles.key, "[1] Go to Today")), "\n")
	}
	solved, firstPass, hinted := 0, 0, 0
	for _, r := range m.records {
		if r.Attempt.Status == "solved" {
			solved++
			if r.Checks == 1 {
				firstPass++
			}
		}
		if r.Attempt.HintLevel > 0 {
			hinted++
		}
	}
	lines = append(lines, paint(styles.key, fmt.Sprintf("%d attempts | %d solved | %d first-check passes | %d used hints", len(m.records), solved, firstPass, hinted)),
		paint(styles.muted, "Elapsed exercise time includes thinking time; interrupted time may be unknown."), "")
	widths := []int{m.contentWidth() - 48, 12, 20, 8}
	if m.contentWidth() >= 68 {
		lines = append(lines, m.tableRow([]string{"EXERCISE", "STARTED", "STATUS", "ELAPSED"}, widths, false, true))
	} else {
		lines = append(lines, m.listRow("EXERCISE", "", "", "STATUS", false, true))
	}
	for i, r := range m.records {
		title := r.Attempt.ExerciseID
		if c, err := m.service.Catalog.Find(r.Attempt.ExerciseID, r.Attempt.Revision); err == nil {
			title = c.Title
		}
		if m.contentWidth() >= 68 {
			lines = append(lines, m.tableRow([]string{title, r.Attempt.CreatedAt.Local().Format("Jan 02 15:04"), r.Attempt.Status, duration(r)}, widths, i == m.selected, false))
		} else {
			lines = append(lines, m.listRow(title, "", "", r.Attempt.Status, i == m.selected, false))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) detailView() string {
	a := m.currentAttempt()
	if a == nil {
		return "Attempt is no longer available. Press Esc to return to Progress."
	}
	styles := m.styles()
	lines := []string{paint(styles.title, "SAVED ATTEMPT"), paint(styles.text, a.ExerciseID+fmt.Sprintf(" / revision %d", a.Revision)),
		paint(styles.key, "Status: "+a.Status), "", m.primary("[Enter] Open exercise") + "   " + paint(styles.text, "[s] Spoiler-free share text"),
		paint(styles.muted, "[e] Export all history as JSON   [c] Export all history as CSV"), "", m.section("PERFORMANCE")}
	for _, r := range m.records {
		if r.Attempt.ID == a.ID {
			lines = append(lines, paint(styles.text, "Elapsed exercise time: "+duration(r)))
			break
		}
	}
	if m.best != nil {
		lines = append(lines, paint(styles.key, "Comparable personal best: "+duration(*m.best)), paint(styles.muted, "Same revision, seed, environment, validator and assistance."))
	} else {
		lines = append(lines, paint(styles.muted, "No completed comparable personal best."))
	}
	lines = append(lines, paint(styles.text, fmt.Sprintf("Hints: %d | revealed: %t | external assistance: %t", a.HintLevel, a.SolutionRevealed, a.ExternalAssistance)), "", m.section("SESSIONS"))
	if len(m.sessions) == 0 {
		lines = append(lines, "No exercise sessions.")
	}
	for _, session := range m.sessions {
		elapsed := "unknown"
		if session.DurationMS != nil {
			elapsed = (time.Duration(*session.DurationMS) * time.Millisecond).Round(time.Second).String()
		}
		lines = append(lines, paint(styles.text, session.StartedAt.Local().Format(time.RFC3339)+" | "+session.Outcome+" | "+elapsed))
	}
	lines = append(lines, "", m.section("CHECKS"))
	if len(m.checks) == 0 {
		lines = append(lines, "No checks recorded.")
	}
	for _, check := range m.checks {
		lines = append(lines, paint(styles.muted, check.CreatedAt.Local().Format(time.RFC3339)), paint(styles.text, formatCheck(check.Result)), "")
	}
	lines = append(lines, "", m.section("RECORD"), paint(styles.muted, a.ID),
		paint(styles.text, "Created: "+a.CreatedAt.Local().Format(time.RFC1123)),
		paint(styles.text, "Assignment: "+a.AssignmentDate+" | seed: "+a.Seed),
		paint(styles.text, "Profile: "+a.Profile), paint(styles.text, "Environment: "+a.EnvironmentID),
		paint(styles.text, "Validator version: "+a.ValidatorVersion))
	if a.RetryOf != "" {
		lines = append(lines, paint(styles.text, "Retry of: "+a.RetryOf))
	}
	return strings.Join(lines, "\n")
}

func (m Model) settingsView() string {
	styles := m.styles()
	return strings.Join([]string{paint(styles.title, "SETTINGS"), "", m.section("PREFERENCES"),
		paint(styles.text, "Track: "+m.service.Config.Track) + paint(styles.key, "   [t] Choose"),
		paint(styles.text, "Theme: "+m.service.Config.Theme) + paint(styles.key, "   [l] Cycle light / dark / plain"), "",
		m.section("RUNTIME"), paint(styles.text, "Practice: native tools and your dotfiles"), paint(styles.muted, "Image: "+m.service.Config.Image),
		paint(styles.key, "[d] Check dependencies"), paint(styles.muted, "First use: run golf setup from your shell to build the tool image."),
		paint(styles.muted, "Docker prepares fixtures and validates your work."), "", m.section("LOCAL DATA"),
		paint(styles.text, "State directory: "+m.service.StateDir), paint(styles.muted, "Exports are saved under this directory's exports folder."), "",
		m.section("ACCESSIBILITY & PRIVACY"), paint(styles.text, "NO_COLOR disables styling. All controls use ASCII text."),
		paint(styles.muted, "For linear output and exports, use golf --help."), paint(styles.muted, "No account or telemetry. Native tools use your normal host access.")}, "\n")
}

func (m Model) keyHelp() string {
	if m.busy {
		return "Operation in progress. Navigation returns when state is saved."
	}
	if m.confirm != "" {
		return "y confirm   n / Esc cancel"
	}
	if m.chooseTrack {
		return "j/k or arrows choose   Enter select   q quit"
	}
	if m.help {
		return "j/k or PgUp/PgDn scroll   ? / Esc close help"
	}
	if m.searching {
		return "Type to filter   Ctrl-U clear   Enter / Esc finish"
	}
	if (m.screen == practice || m.screen == progress) && (m.notice != "" || m.errorText != "") {
		return "j/k scroll message   Esc dismiss   Tab navigate   q quit"
	}
	if m.contentWidth() < 36 {
		return "Tab menu  ? help  q quit"
	}
	if m.contentWidth() < 72 {
		switch m.screen {
		case today:
			return "Enter open  t track  ? help  q quit"
		case practice:
			return "j/k move  / find  Enter open  q quit"
		case progress:
			return "j/k move  Enter detail  ? help  q quit"
		case exercise:
			return "Enter work  c check  Esc back  ? help"
		case detail:
			return "j/k scroll  Esc back  ? help  q quit"
		default:
			return "t track  l theme  ? help  q quit"
		}
	}
	switch m.screen {
	case today:
		return "Enter open   t track   Tab navigate   ? help   q quit"
	case practice:
		return "j/k move   / search   Enter open   Tab navigate   ? help   q quit"
	case progress:
		return "j/k move   Enter detail   e JSON   c CSV   Tab navigate   q quit"
	case exercise:
		return "Enter resume   c check   h hint   r retry   j/k scroll   Esc back   ? help"
	case detail:
		return "j/k scroll   s share   Enter exercise   Esc back   ? help   q quit"
	default:
		return "t track   l theme   d doctor   j/k scroll   Tab navigate   q quit"
	}
}

const helpText = `KEYBOARD

Tab / Shift-Tab: next / previous destination.
1 Today   2 Practice   3 Progress   4 Settings
j/k or arrows: move through lists or scroll text.
Page Up / Page Down: scroll one page. Home: return to top.
Enter: open the selected exercise, resume work, or inspect a result.
Esc: go back, close a prompt, or clear a search.
q / Ctrl-C: quit the outer application. Active work remains resumable.

PRACTICE
/ starts search. All space-separated terms must match.
Search tool, concept, track, difficulty:1, solved, missed, or untried.
Ctrl-U clears the search. Enter or Esc finishes editing.

EXERCISE
Enter / s: start or resume. c: check. h: next hint.
v: reveal explanation (confirmation records assistance).
r: create a separate retry. a: abandon (confirmation required).
x: permanently record external assistance for this attempt.
p: inspect saved result.
Native editor and shell keys are untouched inside an exercise.
Exit the editor or shell to return. Ctrl-C goes to the child.
Finished attempts remain immutable. Retry preserves the prior result.

PROGRESS
Enter opens sessions, check results, assistance and a comparable best.
e exports all history as JSON. c exports all history as CSV.
s in attempt detail produces spoiler-free text for manual sharing.

SETTINGS
t changes track. l cycles display theme. d checks dependencies.
NO_COLOR and the plain theme disable color. No fonts or mouse required.
Use golf --help for linear commands and redirected output.`
