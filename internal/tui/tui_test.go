package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stevencarpenter/driving-range/internal/app"
	"github.com/stevencarpenter/driving-range/internal/catalog"
	"github.com/stevencarpenter/driving-range/internal/model"
	"github.com/stevencarpenter/driving-range/internal/runner"
	"github.com/stevencarpenter/driving-range/internal/store"
)

func testModel(t *testing.T) Model {
	t.Helper()
	state := t.TempDir()
	db, err := store.Open(state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cat := &catalog.Catalog{}
	for i := 0; i < 30; i++ {
		cat.Challenges = append(cat.Challenges, model.Challenge{ID: fmt.Sprintf("shell.task-%02d", i), Revision: 1, Title: fmt.Sprintf("Task %02d", i), Track: "shell", Difficulty: 1, Tools: []string{"bash", "rg"}, Concepts: []string{"quoting"}, Profile: "standard", Brief: strings.Repeat("Read the files carefully. ", 30), Validator: model.Validator{Kind: "tree", OutputPolicy: "exact", Version: "1"}})
	}
	s := &app.Service{Catalog: cat, Store: db, StateDir: state, Config: app.Config{Track: "shell", Theme: "plain", Image: runner.DefaultImage}}
	m := New(s)
	t.Cleanup(func() {
		if err := m.lifecycle.shutdown(); err != nil {
			t.Error(err)
		}
	})
	m.now = func() time.Time { return time.Date(2026, 9, 14, 18, 0, 0, 0, time.UTC) }
	return m
}

func press(m Model, key string) (Model, tea.Cmd) {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg.Type = tea.KeyEnter
	case "esc":
		msg.Type = tea.KeyEsc
	case "pgdown":
		msg.Type = tea.KeyPgDown
	case "pgup":
		msg.Type = tea.KeyPgUp
	case "ctrl+u":
		msg.Type = tea.KeyCtrlU
	case "ctrl+c":
		msg.Type = tea.KeyCtrlC
	case "tab":
		msg.Type = tea.KeyTab
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

func TestPracticeSearchAndSelection(t *testing.T) {
	m := testModel(t)
	m.screen = practice
	m.records = []model.Progress{{Attempt: model.Attempt{ExerciseID: "shell.task-04", Revision: 1, Status: "active"}, Checks: 2, FailedChecks: 1}}
	m, _ = press(m, "/")
	m, _ = press(m, "rg quoting difficulty:1 missed")
	m, _ = press(m, "enter")
	if got := m.filtered(); len(got) != 1 || got[0].ID != "shell.task-04" {
		t.Fatalf("filter results: %+v", got)
	}
	m, _ = press(m, "enter")
	if m.screen != exercise || m.challenge.ID != "shell.task-04" {
		t.Fatal("search selection did not open its challenge")
	}
	m, _ = press(m, "esc")
	m, _ = press(m, "/")
	m, _ = press(m, "ctrl+u")
	m, _ = press(m, "absent")
	m, _ = press(m, "enter")
	if !strings.Contains(m.View(), "No exercises match") {
		t.Fatal(m.View())
	}
	m, cmd := press(m, "enter")
	if cmd != nil || m.screen != practice {
		t.Fatal("empty selection performed an action")
	}
}

func TestResizeKeepsSelectionVisibleAndOutputBounded(t *testing.T) {
	m := testModel(t)
	m.screen = practice
	for i := 0; i < 29; i++ {
		m, _ = press(m, "j")
	}
	for _, size := range [][2]int{{80, 24}, {40, 12}, {12, 8}, {1, 1}} {
		u, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = u.(Model)
		view := m.View()
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Fatalf("height %d: %d lines", size[1], len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("width %d exceeded by %q", size[0], line)
			}
		}
		if size[0] >= 12 && !strings.Contains(view, "> Task 29") {
			t.Fatalf("selection hidden at %v:\n%s", size, view)
		}
	}
}

func TestHelpScrollAndPageNavigation(t *testing.T) {
	m := testModel(t)
	m.screen = practice
	m, _ = press(m, "pgdown")
	if m.selected != m.contentHeight() {
		t.Fatal("page down did not advance selection")
	}
	m, _ = press(m, "?")
	before := m.View()
	m, _ = press(m, "pgdown")
	if m.View() == before {
		t.Fatal("help is not scrollable")
	}
	m, _ = press(m, "esc")
	if m.help || m.offset != 0 {
		t.Fatal("help did not restore navigation")
	}
}

func TestDailyAssignmentFreezesAndExpiredScheduleIsPractice(t *testing.T) {
	m := testModel(t)
	if !strings.Contains(m.View(), "PRACTICE") || !strings.Contains(m.View(), "No shared assignment") {
		t.Fatal(m.View())
	}
	a := model.Assignment{Date: "2026-09-14", Track: "shell", ExerciseID: "shell.task-00", Revision: 1, Seed: "daily"}
	m.service.Catalog.Assignments = []model.Assignment{a}
	m, _ = press(m, "enter")
	m.now = func() time.Time { return time.Date(2026, 9, 15, 18, 0, 0, 0, time.UTC) }
	if m.assignment == nil || m.assignment.Date != "2026-09-14" {
		t.Fatal("active assignment changed at midnight")
	}
	if !strings.Contains(m.View(), "2026-09-14") {
		t.Fatal(m.View())
	}
}

func TestDailyResumeAndLatestRequireCompleteAssignment(t *testing.T) {
	m := testModel(t)
	a := model.Assignment{Date: "2026-09-14", Track: "shell", ExerciseID: "shell.task-00", Revision: 1, Seed: "daily-seed"}
	m.service.Catalog.Assignments = []model.Assignment{a}
	matching := model.Attempt{ID: "matching", ExerciseID: a.ExerciseID, Revision: a.Revision, Track: a.Track, AssignmentDate: a.Date, Seed: a.Seed, Profile: "standard", Status: "active"}
	for _, field := range []string{"date", "seed", "track", "revision", "profile"} {
		t.Run(field, func(t *testing.T) {
			wrong := matching
			wrong.ID, wrong.Status = "wrong", "solved"
			switch field {
			case "date":
				wrong.AssignmentDate = "2026-09-13"
			case "seed":
				wrong.Seed = "different-seed"
			case "track":
				wrong.Track = "vim"
			case "revision":
				wrong.Revision = 2
			case "profile":
				wrong.Profile = "personal"
			}
			m.records = []model.Progress{{Attempt: wrong}, {Attempt: matching}}
			if view := m.todayView(); !strings.Contains(view, "Latest: active") || strings.Contains(view, "Latest: solved") {
				t.Fatal(view)
			}
			m.openChallenge(m.service.Catalog.Challenges[0], &a, today)
			if m.attemptID != matching.ID {
				t.Fatalf("daily resumed %q with mismatched %s", m.attemptID, field)
			}
		})
	}
	practiceAttempt := matching
	practiceAttempt.ID, practiceAttempt.Seed = "practice", "default"
	m.records = []model.Progress{{Attempt: practiceAttempt}, {Attempt: matching}}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, practice)
	if m.attemptID != practiceAttempt.ID {
		t.Fatal("practice no longer selects the latest matching exercise/revision")
	}
}

func TestBusyPreventsDuplicateLaunchAndQuit(t *testing.T) {
	m := testModel(t)
	m.openChallenge(m.service.Catalog.Challenges[0], nil, today)
	m, cmd := press(m, "enter")
	if !m.busy || cmd == nil {
		t.Fatal("launch did not enter busy state")
	}
	for _, key := range []string{"enter", "q", "ctrl+c", "tab", "r", "c"} {
		updated, next := press(m, key)
		if next != nil || !updated.busy || updated.screen != exercise {
			t.Fatalf("busy accepted %s", key)
		}
	}
	updated, _ := m.Update(preparedMsg{err: errors.New("missing runtime")})
	m = updated.(Model)
	if m.busy || !strings.Contains(m.View(), "missing runtime") {
		t.Fatal("prepare failure did not restore navigation and error")
	}
	if strings.Contains(m.View(), "Your attempt is retained") {
		t.Fatal("failed creation claimed a saved attempt")
	}
}

func TestDoctorAndDetailBecomeBusyBeforeCommandRuns(t *testing.T) {
	m := testModel(t)
	m.screen = settings
	m, cmd := press(m, "d")
	if !m.busy || cmd == nil {
		t.Fatal("doctor did not lock duplicate input")
	}
	m.busy = false
	m.screen = progress
	m.records = []model.Progress{{Attempt: model.Attempt{ID: "id"}}}
	m, cmd = press(m, "enter")
	if !m.busy || cmd == nil || m.screen != detail {
		t.Fatal("detail load did not lock duplicate input")
	}
}

func TestDetailReturnsThroughExerciseToPractice(t *testing.T) {
	m := testModel(t)
	m.records = []model.Progress{{Attempt: model.Attempt{ID: "id", ExerciseID: "shell.task-00", Revision: 1, Status: "active"}}}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, practice)
	m, _ = press(m, "p")
	m.busy = false
	m, _ = press(m, "esc")
	if m.screen != exercise {
		t.Fatal("detail did not return to exercise")
	}
	m, _ = press(m, "esc")
	if m.screen != practice {
		t.Fatal("exercise lost original destination")
	}
}

func TestRevealAndAbandonRequireConfirmation(t *testing.T) {
	m := testModel(t)
	m.openChallenge(m.service.Catalog.Challenges[0], nil, today)
	m.attemptID = "id"
	for _, key := range []string{"v", "a"} {
		m, cmd := press(m, key)
		if cmd != nil || m.confirm == "" {
			t.Fatalf("%s lacks confirmation", key)
		}
		m, cmd = press(m, "n")
		if cmd != nil || m.confirm != "" {
			t.Fatal("cancel performed operation")
		}
	}
}

func TestSavedAttemptAndErrorPresentation(t *testing.T) {
	m := testModel(t)
	m.records = []model.Progress{{Attempt: model.Attempt{ID: "id", ExerciseID: "shell.task-00", Revision: 1, Status: "solved"}}}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, today)
	m, cmd := press(m, "enter")
	if cmd != nil || m.busy || !strings.Contains(m.notice, "preserved") {
		t.Fatal("completed attempt was relaunched")
	}
	m.busy = true
	updated, _ := m.Update(operationMsg{err: errors.New("save session: disk full")})
	m = updated.(Model)
	if m.busy || !strings.Contains(m.View(), "disk full") {
		t.Fatal("save failure is not visible")
	}
}

func TestNoColorAndUnsafeOutput(t *testing.T) {
	m := testModel(t)
	m.service.Config.Theme = "dark"
	t.Setenv("NO_COLOR", "")
	m.notice = "unsafe\x1b[2J\x1b]52;c;secrets\a\r\x00 output 日本語"
	view := m.View()
	if strings.ContainsAny(view, "\x1b\a\r\x00") {
		t.Fatalf("terminal controls in rendered output: %q", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatal("unicode overflow")
		}
	}
}

func TestFirstLaunchAndEmptyHistory(t *testing.T) {
	m := testModel(t)
	m.service.Config.Track = ""
	m = New(m.service)
	if !m.chooseTrack || !strings.Contains(m.View(), "Choose your starting track") {
		t.Fatal("no initial track selection")
	}
	m.chooseTrack = false
	m.screen = progress
	if !strings.Contains(m.View(), "No attempts yet") {
		t.Fatal(m.View())
	}
	m, cmd := press(m, "enter")
	if cmd != nil || m.screen != progress {
		t.Fatal("empty progress performed an action")
	}
}

func TestMissedFilterExcludesInfrastructureChecks(t *testing.T) {
	m := testModel(t)
	m.query = "missed"
	m.records = []model.Progress{
		{Attempt: model.Attempt{ExerciseID: "shell.task-00", Revision: 1, Status: "infrastructure_error"}, Checks: 3},
		{Attempt: model.Attempt{ExerciseID: "shell.task-01", Revision: 1, Status: "active"}, Checks: 2, FailedChecks: 1},
		{Attempt: model.Attempt{ExerciseID: "shell.task-02", Revision: 1, Status: "solved"}, Checks: 2, FailedChecks: 1},
	}
	got := m.filtered()
	if len(got) != 1 || got[0].ID != "shell.task-01" {
		t.Fatalf("missed includes infrastructure errors or solved attempts: %+v", got)
	}
}

func TestExternalAssistanceCannotBeUnmarked(t *testing.T) {
	m := testModel(t)
	a := model.Attempt{ID: "assisted", ExerciseID: "shell.task-00", Revision: 1, Track: "shell", Profile: "standard", ValidatorVersion: "1", Status: "active", CreatedAt: time.Now().UTC()}
	if err := m.service.Store.CreateAttempt(a); err != nil {
		t.Fatal(err)
	}
	m.records = []model.Progress{{Attempt: a}}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, practice)
	m, cmd := press(m, "x")
	if cmd == nil {
		t.Fatal("marking assistance did not persist")
	}
	updated, refresh := m.Update(cmd())
	m = updated.(Model)
	if m.errorText != "" {
		t.Fatal(m.errorText)
	}
	updated, _ = m.Update(refresh())
	m = updated.(Model)
	if a := m.currentAttempt(); a == nil || !a.ExternalAssistance {
		t.Fatal("assistance was not recorded")
	}
	m, cmd = press(m, "x")
	if cmd != nil || !strings.Contains(m.notice, "remains recorded") {
		t.Fatal("repeated mark did not preserve the recorded assistance")
	}
	if strings.Contains(m.notice, "false") || strings.Contains(helpText, "unmark") || strings.Contains(m.exerciseView(), "Toggle") {
		t.Fatal("UI still offers reversible assistance")
	}
}

func TestPendingRecoveryRemainsVisibleWhileBrowsing(t *testing.T) {
	state := t.TempDir()
	db, err := store.Open(state)
	if err != nil {
		t.Fatal(err)
	}
	a := model.Attempt{ID: "0123456789abcdef0123456789abcdef", ExerciseID: "vim.change-value", Revision: 1, Track: "vim", Profile: "standard", ValidatorVersion: "1", Status: "active", CreatedAt: time.Now().UTC()}
	if err := db.CreateAttempt(a); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	s, err := app.Open(state, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	m := New(s)
	t.Cleanup(func() { m.lifecycle.shutdown() })
	m.chooseTrack = false
	for _, screen := range []screen{today, settings} {
		m.screen = screen
		if view := m.View(); !strings.Contains(view, "WORKSPACE RECOVERY PENDING") || !strings.Contains(view, "Browsing is available") {
			t.Fatal(view)
		}
	}
	m.screen = practice
	if !strings.Contains(m.View(), "PRACTICE / archive") {
		t.Fatal("pending recovery prevented catalog browsing")
	}
}
