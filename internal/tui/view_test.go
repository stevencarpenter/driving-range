package tui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stevencarpenter/driving-range/internal/model"
)

func TestStyledViewsStayWithinTerminal(t *testing.T) {
	m := testModel(t)
	t.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	m.records = []model.Progress{{Attempt: model.Attempt{ID: "attempt", ExerciseID: "shell.task-00", Revision: 1, Status: "solved", CreatedAt: m.now()}}}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, today)
	for _, theme := range []string{"dark", "light", "plain", "auto"} {
		m.service.Config.Theme = theme
		m.darkBackground = theme != "auto"
		for _, size := range [][2]int{{1, 1}, {12, 8}, {40, 12}, {72, 24}, {80, 24}, {120, 32}, {200, 48}} {
			m.width, m.height = size[0], size[1]
			for _, screen := range []screen{today, practice, progress, settings, exercise, detail} {
				m.screen = screen
				view := m.render()
				if len(strings.Split(view, "\n")) > m.height {
					t.Fatalf("%s screen %d exceeds height at %v", theme, screen, size)
				}
				for _, line := range strings.Split(view, "\n") {
					if ansi.StringWidth(line) > m.width {
						t.Fatalf("%s screen %d exceeds width at %v: %q", theme, screen, size, line)
					}
				}
				if theme == "plain" && strings.Contains(view, "\x1b") {
					t.Fatalf("plain screen %d contains escapes", screen)
				}
				if m.width >= 80 && !strings.Contains(ansi.Strip(view), "DRIVING RANGE") {
					t.Fatal("missing navigation header")
				}
			}
		}
	}
}

func TestColorSelectionAndPlainSelectionAgree(t *testing.T) {
	m := testModel(t)
	t.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	m.screen = practice
	m.selected = 29
	for _, size := range [][2]int{{12, 8}, {40, 12}, {80, 24}, {120, 32}} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = updated.(Model)
		m.service.Config.Theme = "plain"
		plain := m.render()
		for _, theme := range []string{"dark", "light"} {
			m.service.Config.Theme = theme
			view := m.render()
			if !strings.Contains(view, "\x1b[") {
				t.Fatal("color test rendered without styling")
			}
			if !strings.Contains(ansi.Strip(view), "> Task 29") {
				t.Fatalf("%s selection hidden at %v:\n%s", theme, size, view)
			}
			if strings.NewReplacer("╭", "+", "╮", "+", "╰", "+", "╯", "+", "│", "|", "─", "-").Replace(ansi.Strip(view)) != plain {
				t.Fatalf("%s layout differs from plain at %v", theme, size)
			}
		}
	}
}

func TestStyledContentCannotInjectTerminalControls(t *testing.T) {
	m := testModel(t)
	t.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	m.service.Config.Theme = "dark"
	unsafe := "visible\x1b[2J\x1b[31m\x1b]52;c;secrets\a\r\x00\u009b output"
	m.service.Catalog.Challenges[0].Title = unsafe
	m.service.Catalog.Challenges[0].Brief = unsafe
	m.service.Catalog.Challenges[0].Objective = unsafe
	m.service.StateDir = unsafe
	m.records = []model.Progress{{Attempt: model.Attempt{ID: "attempt", ExerciseID: unsafe, Status: unsafe}}}
	m.challenge = &m.service.Catalog.Challenges[0]
	m.attemptID = "attempt"
	for _, screen := range []screen{today, practice, progress, settings, exercise, detail} {
		m.screen = screen
		m.notice = unsafe
		view := m.render()
		for _, control := range []string{"\x1b[2J", "\x1b[31m", "\x1b]52", "\a", "\r", "\x00", "\u009b"} {
			if strings.Contains(view, control) {
				t.Fatalf("screen %d leaked %q", screen, control)
			}
		}
		if !strings.Contains(view, "visible") {
			t.Fatal("sanitization removed readable content")
		}
	}
	t.Setenv("NO_COLOR", "")
	if strings.Contains(m.render(), "\x1b") {
		t.Fatal("NO_COLOR must disable all styling, including bold")
	}
}

func TestExercisePrioritizesActionAndInstructions(t *testing.T) {
	m := testModel(t)
	m.service.Catalog.Challenges[0].Objective = "Change the requested configuration value."
	m.records = []model.Progress{{Attempt: model.Attempt{ID: "attempt", ExerciseID: "shell.task-00", Revision: 1, Status: "active"}}}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, today)
	view := m.render()
	for _, text := range []string{"[Enter] Start / resume", "GOAL", "BRIEF", "Change the requested configuration value.", "Esc back"} {
		if !strings.Contains(view, text) {
			t.Fatalf("missing %q from initial 80x24 exercise view:\n%s", text, view)
		}
	}
	if strings.Index(m.exerciseView(), "BRIEF") > strings.Index(m.exerciseView(), "ATTEMPT") {
		t.Fatal("attempt bookkeeping precedes instructions")
	}
}

func TestGoalLeadsActionsAndUsesAvailableWidth(t *testing.T) {
	m := testModel(t)
	c := &m.service.Catalog.Challenges[0]
	c.Objective = "Change port=3000 to port=8080."
	c.Brief = c.Objective + "\n\nKeep the comment and host unchanged."
	m.openChallenge(*c, nil, today)
	for _, width := range []int{80, 120, 160, 240} {
		m.width, m.height = width, 40
		for _, screen := range []screen{exercise} {
			m.screen = screen
			view := m.render()
			if strings.Index(view, c.Objective) < 0 || strings.Index(view, c.Objective) > strings.Index(view, "[Enter]") {
				t.Fatalf("goal must precede the primary action on screen %d", screen)
			}
			if len(strings.Split(view, "\n")) != m.height {
				t.Fatal("view does not use terminal height")
			}
			for _, line := range strings.Split(m.goal(c.Objective), "\n") {
				if ansi.StringWidth(line) != m.contentWidth() {
					t.Fatalf("goal panel does not fill %d columns: %q", width, line)
				}
			}
		}
	}
	body := m.exerciseView()
	if strings.Count(body, c.Objective) != 1 || !strings.Contains(body, "Keep the comment and host unchanged.") {
		t.Fatal("brief must retain instructions without repeating the goal")
	}
	// A nonmatching brief must remain intact.
	m.challenge.Brief = "Different instructions.\n\n" + c.Objective
	if !strings.Contains(m.exerciseView(), "Different instructions.") {
		t.Fatal("different opening paragraph was removed")
	}
}

func TestGoalWrapsLongUnicodeContent(t *testing.T) {
	m := testModel(t)
	objective := strings.Repeat("日本語", 40) + " finishgoal"
	for _, width := range []int{12, 40, 80, 160} {
		m.width = width
		goal := ansi.Wrap(m.goal(objective), m.contentWidth(), "")
		if strings.Count(goal, "日") != 40 || strings.Count(goal, "語") != 40 {
			t.Fatalf("goal text lost at width %d", width)
		}
		for _, line := range strings.Split(goal, "\n") {
			if ansi.StringWidth(line) > m.contentWidth() {
				t.Fatalf("goal overflows at width %d: %q", width, line)
			}
		}
	}
}

func TestWideLayoutTracksSelectionAndCollapses(t *testing.T) {
	m := testModel(t)
	m.screen = practice
	m.width, m.height = 160, 40
	m.service.Catalog.Challenges[0].Objective = "First objective"
	m.service.Catalog.Challenges[1].Objective = "Second objective"
	if !strings.Contains(m.render(), "First objective") {
		t.Fatal("wide layout has no selected exercise preview")
	}
	m, _ = press(m, "j")
	view := m.render()
	if !strings.Contains(view, "Second objective") || strings.Contains(view, "First objective") {
		t.Fatal("preview did not follow keyboard selection")
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) != m.width {
			t.Fatal("wide layout does not fill the terminal")
		}
	}
	m.width = 80
	if strings.Contains(m.render(), "SELECTED EXERCISE") {
		t.Fatal("preview did not collapse in a narrow terminal")
	}
	m, _ = press(m, "enter")
	if !strings.Contains(m.render(), "Second objective") {
		t.Fatal("selected goal is not accessible after collapsing the preview")
	}
}

func TestTrackProgressCountsExercisesAndHonorsNoColor(t *testing.T) {
	m := testModel(t)
	m.records = []model.Progress{
		{Attempt: model.Attempt{ExerciseID: "shell.task-00", Revision: 1, Status: "solved"}},
		{Attempt: model.Attempt{ExerciseID: "shell.task-00", Revision: 1, Status: "solved"}},
		{Attempt: model.Attempt{ExerciseID: "shell.task-01", Revision: 2, Status: "solved"}},
		{Attempt: model.Attempt{ExerciseID: "shell.task-02", Revision: 1, Status: "active"}},
	}
	if !strings.Contains(m.trackProgress("shell", 30), "1 of 30 exercises solved") {
		t.Fatal("track progress counted retries, other revisions, or unsolved attempts")
	}
	m.screen, m.width, m.height = practice, 160, 40
	m.service.Config.Theme = "dark"
	t.Setenv("NO_COLOR", "")
	view := m.render()
	if strings.Contains(view, "\x1b") || strings.ContainsAny(view, "╭╮╰╯━─") {
		t.Fatal("NO_COLOR preview must use unstyled ASCII frames and progress")
	}
}

func TestNestedStyleRestoresContainingSurface(t *testing.T) {
	m := testModel(t)
	m.service.Config.Theme = "dark"
	t.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	s := m.styles()
	prefix := strings.TrimSuffix(s.surface.Render(""), styleReset)
	view := onSurface(s.surface, paint(s.action, "Action")+" gap")
	if prefix == "" || !strings.Contains(view, styleReset+prefix+" gap") {
		t.Fatal("nested style reset exposes terminal background between elements")
	}
}

func TestTodayShowsAllTracksAndSelectedNameAt80Columns(t *testing.T) {
	m := testModel(t)
	tracks := []string{"awk", "fd", "find", "fzf", "git", "jj", "python", "search", "sed", "shell", "vim", "zsh"}
	m.service.Catalog.Challenges = m.service.Catalog.Challenges[:12]
	for i, track := range tracks {
		c := &m.service.Catalog.Challenges[i]
		c.Track, c.ID = track, track+".first"
		c.Title = "Read the supplied configuration and update the requested setting while preserving unrelated values"
		c.Objective = "Change only the requested value in the configuration file."
		c.Minutes = 5
	}
	view := m.render()
	for _, track := range tracks {
		if !strings.Contains(view, " "+track+" ") {
			t.Fatalf("track %s is hidden at 80x24:\n%s", track, view)
		}
	}
	for _, heading := range []string{"0/12 done", "STATE", "LVL", "HNT", "MIN", "SOLVED"} {
		if !strings.Contains(view, heading) {
			t.Fatalf("missing daily progress %q", heading)
		}
	}
	if strings.Contains(view, "GOAL") {
		t.Fatal("full goal must open separately at 80 columns")
	}
	if !strings.Contains(strings.Join(strings.Fields(view), " "), "preserving unrelated values") {
		t.Fatal("selected name is not available in full")
	}
	for range 11 {
		m, _ = press(m, "j")
	}
	if !strings.Contains(m.render(), "> zsh") {
		t.Fatal("last track cannot be selected without losing its row")
	}
	m, _ = press(m, "enter")
	if !strings.Contains(m.render(), "GOAL") || !strings.Contains(m.exerciseView(), m.challenge.Objective) {
		t.Fatal("Enter does not expose the selected full goal")
	}
}

func TestTodaySidebarContainsSelectedNameAndGoalOnly(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 112, 24
	c := &m.service.Catalog.Challenges[0]
	c.Objective = "Change only the selected configuration value."
	side := m.sidebar(m.sidebarWidth()-4, m.contentHeight())
	for _, text := range []string{"SELECTED EXERCISE", c.Title, "GOAL", "configuration value."} {
		if !strings.Contains(side, text) {
			t.Fatalf("selected preview missing %q: %s", text, side)
		}
	}
	for _, text := range []string{"TOOLKIT", "TRACK PROGRESS", "CURRENT ATTEMPT", "Ready", "Resume"} {
		if strings.Contains(side, text) {
			t.Fatalf("daily preview duplicates %q", text)
		}
	}
	m.width = 80
	if m.sidebarWidth() != 0 {
		t.Fatal("daily preview did not collapse at 80 columns")
	}
}

func TestLibraryShowsFiltersAndDoesNotRegressSolvedState(t *testing.T) {
	m := testModel(t)
	m.screen, m.filterTrack, m.filterState = practice, "shell", "solved"
	m.records = []model.Progress{
		{Attempt: model.Attempt{ExerciseID: "shell.task-00", Revision: 1, Status: "active"}},
		{Attempt: model.Attempt{ExerciseID: "shell.task-00", Revision: 1, Status: "solved"}},
	}
	view := m.practiceView()
	for _, text := range []string{"LIBRARY", "Track [t]: shell", "State [f]: solved", "Task 00", "solved"} {
		if !strings.Contains(view, text) {
			t.Fatalf("Library missing %q", text)
		}
	}
	if strings.Contains(view, "active") {
		t.Fatal("failed retry replaced the solved exercise overview")
	}
}

func TestDailyStateColorsAndBlueGoalRemainDistinct(t *testing.T) {
	m := testModel(t)
	t.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"dark", "light"} {
		m.service.Config.Theme = theme
		s := m.styles()
		for _, state := range []string{"Done today", "Complete", "Resume", "Retry", "Check failed", "Check error"} {
			style := s.key
			if state == "Resume" || state == "Retry" {
				style = s.warning
			} else if state == "Check failed" || state == "Check error" {
				style = s.danger
			}
			if got := m.tableRow([]string{state}, []int{13}, false, false); got != "  "+style.Width(13).Render(state) {
				t.Fatalf("%s state %q lost its status color", theme, state)
			}
			if got := m.tableRow([]string{state}, []int{13}, true, false); got != s.selected.Render("> "+s.selected.Width(13).Render(state)) {
				t.Fatalf("%s state %q overrides selection", theme, state)
			}
		}
		gr, gg, gb, _ := s.goal.GetForeground().RGBA()
		sr, sg, sb, _ := s.key.GetForeground().RGBA()
		if gr == sr && gg == sg && gb == sb {
			t.Fatal("goal and solved state share a color")
		}
		if gb <= gr || gb <= gg {
			t.Fatal("goal foreground does not use the approved blue palette")
		}
	}
}

func TestLibraryNamesUnfinishedStateFilter(t *testing.T) {
	m := testModel(t)
	m.screen, m.filterState = practice, "active"
	m.records = []model.Progress{{Attempt: model.Attempt{ExerciseID: "shell.task-00", Revision: 1, Status: "interrupted"}}}
	view := m.practiceView()
	if !strings.Contains(view, "State [f]: unfinished") || !strings.Contains(view, "Task 00") {
		t.Fatal("unfinished filter does not describe interrupted attempts")
	}
	if strings.Contains(view, "State [f]: active") {
		t.Fatal("filter label incorrectly limits unfinished attempts to active")
	}
}

func TestLibraryPreservesAbandonedStateAfterFailedCheck(t *testing.T) {
	m := testModel(t)
	m.screen = practice
	a := dailyRecord("failed-abandoned", "shell.task-00", "", "active", m.now(), nil).Attempt
	a.ValidatorVersion = "1"
	if err := m.service.Store.CreateAttempt(a); err != nil {
		t.Fatal(err)
	}
	if err := m.service.Store.RecordCheck(a.ID, model.CheckResult{Outcome: "fail"}); err != nil {
		t.Fatal(err)
	}
	if err := m.service.Store.AbandonAttempt(a.ID); err != nil {
		t.Fatal(err)
	}
	var err error
	m.records, err = m.service.Store.Progress()
	if err != nil {
		t.Fatal(err)
	}
	view := m.practiceView()
	if !strings.Contains(view, "abandoned") || strings.Contains(view, "Check failed") {
		t.Fatalf("abandoned attempt lost its state:\n%s", view)
	}
	m, _ = press(m, "enter")
	if !strings.Contains(m.exerciseView(), "[r] New attempt") {
		t.Fatal("abandoned attempt did not offer a new retry")
	}
}
