package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stevencarpenter/driving-range/internal/model"
)

func TestStyledViewsStayWithinTerminal(t *testing.T) {
	m := testModel(t)
	m.renderer.SetColorProfile(termenv.TrueColor)
	t.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	m.records = []model.Progress{{Attempt: model.Attempt{ID: "attempt", ExerciseID: "shell.task-00", Revision: 1, Status: "solved", CreatedAt: m.now()}}}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, today)
	for _, theme := range []string{"dark", "light", "plain", "auto"} {
		m.service.Config.Theme = theme
		m.renderer.SetHasDarkBackground(theme != "auto")
		for _, size := range [][2]int{{1, 1}, {12, 8}, {40, 12}, {72, 24}, {80, 24}, {120, 32}, {200, 48}} {
			m.width, m.height = size[0], size[1]
			for _, screen := range []screen{today, practice, progress, settings, exercise, detail} {
				m.screen = screen
				view := m.View()
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
	m.renderer.SetColorProfile(termenv.TrueColor)
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
		plain := m.View()
		for _, theme := range []string{"dark", "light"} {
			m.service.Config.Theme = theme
			view := m.View()
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
	m.renderer.SetColorProfile(termenv.TrueColor)
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
		view := m.View()
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
	if strings.Contains(m.View(), "\x1b") {
		t.Fatal("NO_COLOR must disable all styling, including bold")
	}
}

func TestExercisePrioritizesActionAndInstructions(t *testing.T) {
	m := testModel(t)
	m.service.Catalog.Challenges[0].Objective = "Change the requested configuration value."
	m.records = []model.Progress{{Attempt: model.Attempt{ID: "attempt", ExerciseID: "shell.task-00", Revision: 1, Status: "active"}}}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, today)
	view := m.View()
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
		for _, screen := range []screen{today, exercise} {
			m.screen = screen
			view := m.View()
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
	if !strings.Contains(m.View(), "First objective") {
		t.Fatal("wide layout has no selected exercise preview")
	}
	m, _ = press(m, "j")
	view := m.View()
	if !strings.Contains(view, "Second objective") || strings.Contains(view, "First objective") {
		t.Fatal("preview did not follow keyboard selection")
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) != m.width {
			t.Fatal("wide layout does not fill the terminal")
		}
	}
	m.width = 80
	if strings.Contains(m.View(), "SELECTED EXERCISE") {
		t.Fatal("preview did not collapse in a narrow terminal")
	}
	m, _ = press(m, "enter")
	if !strings.Contains(m.View(), "Second objective") {
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
	m.renderer.SetColorProfile(termenv.TrueColor)
	t.Setenv("NO_COLOR", "")
	view := m.View()
	if strings.Contains(view, "\x1b") || strings.ContainsAny(view, "╭╮╰╯━─") {
		t.Fatal("NO_COLOR preview must use unstyled ASCII frames and progress")
	}
}

func TestNestedStyleRestoresContainingSurface(t *testing.T) {
	m := testModel(t)
	m.service.Config.Theme = "dark"
	m.renderer.SetColorProfile(termenv.TrueColor)
	t.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	s := m.styles()
	prefix := strings.TrimSuffix(s.surface.Render(""), "\x1b[0m")
	view := onSurface(s.surface, paint(s.action, "Action")+" gap")
	if prefix == "" || !strings.Contains(view, "\x1b[0m"+prefix+" gap") {
		t.Fatal("nested style reset exposes terminal background between elements")
	}
}
