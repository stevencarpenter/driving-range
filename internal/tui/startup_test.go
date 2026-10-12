package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func TestStartupWaitsForSavedHistoryBeforeOpeningExercises(t *testing.T) {
	base := testModel(t)
	yesterday := base.now().AddDate(0, 0, -1)
	a := dailyRecord("saved-daily", "shell.task-01", "2026-09-13", "active", yesterday, nil).Attempt
	a.ValidatorVersion = "1"
	if err := base.service.Store.CreateAttempt(a); err != nil {
		t.Fatal(err)
	}
	for _, destination := range []screen{today, practice} {
		t.Run(map[screen]string{today: "Today", practice: "Library"}[destination], func(t *testing.T) {
			m := New(base.service)
			t.Cleanup(func() { m.lifecycle.shutdown() })
			m.now, m.screen = base.now, destination
			m.width = 112
			if view := m.render(); !strings.Contains(view, "Loading saved history") || strings.Contains(view, "Ready") || strings.Contains(view, "GOAL") {
				t.Fatal("startup displayed an exercise before history arrived")
			}
			m, cmd := press(m, "enter")
			if m.screen != destination || m.challenge != nil || cmd != nil {
				t.Fatal("opened an exercise before saved history arrived")
			}
			m, cmd = press(m, "h")
			if m.attemptID != "" || cmd != nil {
				t.Fatal("created an attempt before saved history arrived")
			}
			updated, _ := m.Update(m.refresh()())
			m = updated.(Model)
			m.navigate(today)
			m, _ = press(m, "enter")
			if m.challenge == nil || m.challenge.ID != a.ExerciseID || m.attemptID != a.ID || m.assignment.Date != a.AssignmentDate {
				t.Fatal("did not open the retained daily assignment after loading")
			}
		})
	}
}

func TestStartupHistoryFailureCanRetryOrQuit(t *testing.T) {
	base := testModel(t)
	m := New(base.service)
	t.Cleanup(func() { m.lifecycle.shutdown() })
	updated, _ := m.Update(loadedMsg{err: errors.New("read failed")})
	m = updated.(Model)
	m, cmd := press(m, "enter")
	if m.challenge != nil || cmd != nil || !strings.Contains(m.render(), "read failed") {
		t.Fatal("history failure allowed an exercise or hid its error")
	}
	_, quit := press(m, "q")
	if quit == nil {
		t.Fatal("cannot quit after an initial history failure")
	}
	m, retry := press(m, "r")
	if retry == nil {
		t.Fatal("cannot retry the initial history read")
	}
	updated, _ = m.Update(retry())
	m = updated.(Model)
	m, _ = press(m, "enter")
	if m.challenge == nil || m.errorText != "" {
		t.Fatal("successful retry did not enable exercises and clear the read error")
	}
}

func TestLaterHistoryFailureRetainsLoadedProgress(t *testing.T) {
	m := testModel(t)
	m.records = []model.Progress{dailyRecord("retained", "shell.task-01", "2026-09-14", "active", m.now(), nil)}
	updated, _ := m.Update(loadedMsg{err: errors.New("refresh failed")})
	m = updated.(Model)
	m, _ = press(m, "enter")
	if m.challenge == nil || m.challenge.ID != "shell.task-01" || m.attemptID != "retained" {
		t.Fatal("later refresh error discarded the last loaded assignment")
	}
}
