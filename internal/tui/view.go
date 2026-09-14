package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func (m Model) contentLines() []string {
	body := m.body()
	if m.screen == today || m.screen == settings {
		if warning := m.service.RecoveryWarning(); warning != "" {
			body = "WORKSPACE RECOVERY PENDING\n" + warning + "\n\nBrowsing is available. Starting an exercise retries recovery.\n\n" + body
		}
	}
	if m.notice != "" {
		body = m.notice + "\n\n" + body
	}
	if m.errorText != "" {
		body = "ERROR: " + m.errorText + "\n\n" + body
	}
	if m.confirm != "" {
		prompt := "Reveal the reference solution? This records assistance."
		if m.confirm == "abandon" {
			prompt = "Abandon this attempt? History stays saved; resume will be disabled."
		}
		body = prompt + "\n\n[y] Confirm   [n / Esc] Cancel"
	}
	if m.help {
		body = helpText
	}
	if m.chooseTrack {
		body = m.trackPicker()
	}
	body = strings.ReplaceAll(clean(ansi.Strip(body)), "\t", "    ")
	return strings.Split(ansi.Wrap(body, max(1, m.width), ""), "\n")
}

func (m *Model) scroll(delta int) {
	m.offset = max(0, min(m.offset+delta, max(0, len(m.contentLines())-m.contentHeight())))
}

func (m Model) View() string {
	width := max(1, m.width)
	header := "DRIVING RANGE   Daily terminal practice"
	nav := ""
	for i, label := range []string{"Today", "Practice", "Progress", "Settings"} {
		if i > 0 {
			nav += "  "
		}
		if m.topLevel() == screen(i) {
			nav += fmt.Sprintf("[%d %s]", i+1, label)
		} else {
			nav += fmt.Sprintf("%d %s", i+1, label)
		}
	}
	lines := m.contentLines()
	height := m.contentHeight()
	offset := min(max(0, m.offset), max(0, len(lines)-height))
	if (m.listNavigating() || m.chooseTrack) && !m.help && m.confirm == "" {
		for i, line := range lines {
			if strings.HasPrefix(line, "> ") {
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
		status = fmt.Sprintf("Lines %d-%d of %d", offset+1, min(offset+height, len(lines)), len(lines))
		if len(lines) <= height {
			status = "Local history. No account required."
		}
	}
	help := m.keyHelp()
	result := []string{m.accent(ansi.Truncate(header, width, "")), ansi.Truncate(nav, width, ""), ""}
	for _, line := range visible {
		if strings.HasPrefix(line, "> ") {
			line = m.accent(line)
		}
		result = append(result, line)
	}
	result = append(result, "", ansi.Truncate(clean(status), width, ""), ansi.Truncate(help, width, ""))
	if len(result) > m.height {
		result = result[:max(1, m.height)]
	}
	return strings.Join(result, "\n")
}

func (m Model) accent(s string) string {
	_, noColor := os.LookupEnv("NO_COLOR")
	if noColor || m.service.Config.Theme == "plain" {
		return s
	}
	code := "32"
	if m.service.Config.Theme == "light" {
		code = "32"
	} else if m.service.Config.Theme == "dark" {
		code = "92"
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
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
	title := "Choose your starting track"
	if m.service.Config.Track != "" {
		title = "Choose your track"
	}
	lines := []string{title, "One daily exercise. Change your track anytime in Settings.", ""}
	for i, track := range m.service.Catalog.Tracks() {
		prefix := "  "
		if i == m.trackIndex {
			prefix = "> "
		}
		lines = append(lines, prefix+track)
	}
	if len(m.service.Catalog.Tracks()) == 0 {
		lines = append(lines, "No tracks are available in this catalog.")
	}
	if m.errorText != "" {
		lines = append(lines, "", "ERROR: "+m.errorText)
	}
	return strings.Join(lines, "\n")
}

func (m Model) todayView() string {
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
	lines := []string{label, "", c.Title, c.Objective, "", fmt.Sprintf("About %d minutes | difficulty %d/5", c.Minutes, c.Difficulty), "Tools: " + strings.Join(c.Tools, ", ") + " | profile: " + c.Profile, ""}
	for _, r := range m.records {
		if r.Attempt.ExerciseID == c.ID && r.Attempt.Revision == c.Revision && (a == nil || r.Attempt.MatchesAssignment(*a)) {
			lines = append(lines, "Latest: "+r.Attempt.Status+" | elapsed exercise time "+duration(r)+fmt.Sprintf(" | %d hints", r.Attempt.HintLevel))
			break
		}
	}
	lines = append(lines, "> Open exercise [Enter]", "Choose track [t]", "", calendar, "Completed history remains available in Progress.")
	return strings.Join(lines, "\n")
}

func (m Model) practiceView() string {
	query := m.query
	if m.searching {
		query += "_"
	}
	lines := []string{"PRACTICE / archive", "Search [/]: " + query, "Tool, concept, difficulty:1, solved, missed, or untried.", ""}
	items := m.filtered()
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
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		lines = append(lines, ansi.Truncate(prefix+fmt.Sprintf("%s | %s | %d/5 | %s", c.Title, c.Track, c.Difficulty, status), max(1, m.width), ""))
	}
	return strings.Join(lines, "\n")
}

func (m Model) exerciseView() string {
	if m.challenge == nil {
		return "Choose an exercise in Today or Practice."
	}
	c := m.challenge
	label := "PRACTICE"
	if m.assignment != nil {
		label = "DAILY / " + m.assignment.Date + " UTC (assignment fixed for this attempt)"
	}
	a := m.currentAttempt()
	if a != nil && a.AssignmentDate != "" {
		label = "DAILY / " + a.AssignmentDate + " UTC (assignment fixed for this attempt)"
	}
	lines := []string{label + " / " + c.Track, c.Title, fmt.Sprintf("%s revision %d | %s | %d minutes", c.ID, c.Revision, c.Profile, c.Minutes), ""}
	if a != nil {
		lines = append(lines, fmt.Sprintf("Attempt: %s | %s", a.ID, a.Status), fmt.Sprintf("Hints: %d | solution revealed: %t | external assistance: %t", a.HintLevel, a.SolutionRevealed, a.ExternalAssistance))
		for _, r := range m.records {
			if r.Attempt.ID == a.ID {
				lines = append(lines, "Elapsed exercise time: "+duration(r)+fmt.Sprintf(" | checks: %d", r.Checks))
				break
			}
		}
	}
	if a != nil && (a.Status == "solved" || a.Status == "abandoned") {
		lines = append(lines, "[r] New attempt   [p] Saved result   [v] Explanation")
	} else {
		lines = append(lines, "[Enter] Start / resume   [c] Check   [h] Hint", "[v] Reveal solution   [x] Mark external assistance   [a] Abandon")
	}
	lines = append(lines, "", "GOAL", c.Objective, "", "BRIEF", c.Brief, "", "SUCCESS CONDITIONS", "Validator: "+c.Validator.Kind+" | output: "+c.Validator.OutputPolicy+" | version: "+c.Validator.Version, "Tools: "+strings.Join(c.Tools, ", "), "Concepts: "+strings.Join(c.Concepts, ", "))
	if len(c.Fixtures) > 0 {
		f := c.Fixtures[0]
		if f.ExpectedStdout != "" {
			lines = append(lines, "", "EXPECTED OUTPUT (first fixture)", f.ExpectedStdout)
		}
	}
	lines = append(lines, "", "Inside the exercise, the real editor or shell owns every key.", "Exit the child tool to return and check. Setup and menus are excluded from elapsed exercise time.")
	if a != nil && (a.Status == "solved" || a.SolutionRevealed) {
		lines = append(lines, "", "EXPLANATION", c.Explanation, "", "REFERENCE SOLUTION", c.ReferenceSolution)
	}
	return strings.Join(lines, "\n")
}

func (m Model) progressView() string {
	lines := []string{"PROGRESS / saved locally", "[Enter] Attempt detail   [e] Export JSON   [c] Export CSV", ""}
	if len(m.records) == 0 {
		return strings.Join(append(lines, "No attempts yet. Open Today to begin an exercise."), "\n")
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
	lines = append(lines, fmt.Sprintf("%d attempts | %d solved | %d first-check passes | %d used hints", len(m.records), solved, firstPass, hinted), "Elapsed exercise time includes thinking time; interrupted time may be unknown.", "")
	for i, r := range m.records {
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		lines = append(lines, ansi.Truncate(prefix+fmt.Sprintf("%s | %s | %s | %s", r.Attempt.CreatedAt.Local().Format("Jan 02 15:04"), r.Attempt.ExerciseID, r.Attempt.Status, duration(r)), max(1, m.width), ""))
	}
	return strings.Join(lines, "\n")
}

func (m Model) detailView() string {
	a := m.currentAttempt()
	if a == nil {
		return "Attempt is no longer available. Press Esc to return to Progress."
	}
	lines := []string{"SAVED ATTEMPT", a.ExerciseID + fmt.Sprintf(" / revision %d", a.Revision), a.ID, "Status: " + a.Status, "Created: " + a.CreatedAt.Local().Format(time.RFC1123), "Assignment: " + a.AssignmentDate + " | seed: " + a.Seed, "Profile: " + a.Profile, "Environment: " + a.EnvironmentID, "Validator version: " + a.ValidatorVersion, fmt.Sprintf("Hints: %d | revealed: %t | external assistance: %t", a.HintLevel, a.SolutionRevealed, a.ExternalAssistance)}
	if a.RetryOf != "" {
		lines = append(lines, "Retry of: "+a.RetryOf)
	}
	for _, r := range m.records {
		if r.Attempt.ID == a.ID {
			lines = append(lines, "Elapsed exercise time: "+duration(r))
			break
		}
	}
	if m.best != nil {
		lines = append(lines, "Comparable personal best: "+duration(*m.best), "Same revision, seed, environment, validator and assistance.")
	} else {
		lines = append(lines, "No completed comparable personal best.")
	}
	lines = append(lines, "", "[Enter] Open exercise   [s] Spoiler-free share text", "[e] Export all history as JSON   [c] Export all history as CSV", "", "SESSIONS")
	if len(m.sessions) == 0 {
		lines = append(lines, "No exercise sessions.")
	}
	for _, s := range m.sessions {
		elapsed := "unknown"
		if s.DurationMS != nil {
			elapsed = (time.Duration(*s.DurationMS) * time.Millisecond).Round(time.Second).String()
		}
		lines = append(lines, s.StartedAt.Local().Format(time.RFC3339)+" | "+s.Outcome+" | "+elapsed)
	}
	lines = append(lines, "", "CHECKS")
	if len(m.checks) == 0 {
		lines = append(lines, "No checks recorded.")
	}
	for _, c := range m.checks {
		lines = append(lines, c.CreatedAt.Local().Format(time.RFC3339), formatCheck(c.Result), "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) settingsView() string {
	return strings.Join([]string{"SETTINGS", "", "Track: " + m.service.Config.Track + " [t] Choose", "Theme: " + m.service.Config.Theme + " [l] Cycle light / dark / plain", "Profile: standard Linux tools in Docker", "Image: " + m.service.Config.Image, "", "[d] Check dependencies", "First use: run golf setup from your shell to build the tool image.", "No automatic installation or host execution fallback.", "", "State directory:", m.service.StateDir, "", "Exports are saved under this directory's exports folder.", "NO_COLOR disables accents. All controls use ASCII text.", "For linear output and exports, use golf --help.", "No account, network during exercises, or telemetry is required."}, "\n")
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
	if m.width < 40 {
		return "Tab menu  ? help  q quit"
	}
	if m.width < 72 {
		switch m.screen {
		case today:
			return "Enter open  t track  Tab menu  ? help  q quit"
		case practice:
			return "j/k move  / search  Enter open  ? help  q quit"
		case progress:
			return "j/k move  Enter detail  e JSON  c CSV  ? help  q quit"
		case exercise:
			return "Enter work  c check  h hint  Esc back  ? help"
		case detail:
			return "j/k scroll  s share  Esc back  ? help  q quit"
		default:
			return "t track  l theme  d doctor  ? help  q quit"
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
