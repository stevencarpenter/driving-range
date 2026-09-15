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
			if ansi.Strip(view) != plain {
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
