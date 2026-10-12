package tui

import (
	"testing"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func dailyRecord(id, exercise, date, status string, created time.Time, finished *time.Time) model.Progress {
	return model.Progress{Attempt: model.Attempt{ID: id, ExerciseID: exercise, Revision: 1, Track: "shell", AssignmentDate: date, Seed: "default", Profile: "standard", Status: status, CreatedAt: created, FinishedAt: finished}}
}

func TestDailyRosterSkipsSolvedRevisionsAndPreservesUnfinishedAssignment(t *testing.T) {
	m := testModel(t)
	yesterday := m.now().AddDate(0, 0, -1)
	m.records = []model.Progress{
		dailyRecord("practice-solved", "shell.task-00", "", "solved", yesterday, &yesterday),
		dailyRecord("daily-active", "shell.task-01", "2026-09-13", "interrupted", yesterday, nil),
	}
	row := m.selectedDaily()
	if row.challenge.ID != "shell.task-01" || row.assignment.Date != "2026-09-13" || row.status() != "Resume" || row.solved != 1 || row.total != 30 {
		t.Fatalf("row %+v", row)
	}
	m.service.Catalog.Assignments = []model.Assignment{{Date: "2026-09-14", Track: "shell", ExerciseID: "shell.task-08", Revision: 1, Seed: "published"}}
	m.now = func() time.Time { return yesterday.AddDate(0, 0, 3) }
	row = m.selectedDaily()
	if row.challenge.ID != "shell.task-01" || row.assignment.Seed != "default" || row.assignment.Date != "2026-09-13" {
		t.Fatalf("calendar replaced unfinished work: %+v", row)
	}
	m, _ = press(m, "enter")
	if m.attemptID != "daily-active" {
		t.Fatalf("did not resume exact daily attempt: %q", m.attemptID)
	}
}

func TestDailySolvedLookupUsesExactRevisionAndRetainsSuccess(t *testing.T) {
	m := testModel(t)
	now := m.now()
	newer := m.service.Catalog.Challenges[1]
	newer.Revision = 2
	m.service.Catalog.Challenges = append(m.service.Catalog.Challenges, newer)
	m.records = []model.Progress{
		dailyRecord("failed-retry", "shell.task-00", "", "active", now, nil),
		dailyRecord("solved", "shell.task-00", "", "solved", now.Add(-time.Hour), nil),
		dailyRecord("old-revision-solved", "shell.task-01", "", "solved", now.Add(-time.Hour), nil),
	}
	row := m.selectedDaily()
	if row.challenge.ID != newer.ID || row.challenge.Revision != 2 || row.solved != 1 || row.total != 30 {
		t.Fatalf("failed retry or old revision changed solved lookup: %+v", row)
	}
}

func TestDailyCompletionUsesFirstSuccessAndSurvivesFailedRetry(t *testing.T) {
	m := testModel(t)
	now := m.now()
	yesterday := now.AddDate(0, 0, -1)
	first := now.Add(-2 * time.Hour)
	later := now.Add(-time.Hour)
	m.records = []model.Progress{
		dailyRecord("successful-retry", "shell.task-00", "2026-09-13", "solved", later, &later),
		dailyRecord("failed-retry", "shell.task-00", "2026-09-13", "active", later, nil),
		dailyRecord("second-daily", "shell.task-01", "2026-09-14", "solved", later, &later),
		dailyRecord("first-daily", "shell.task-00", "2026-09-13", "solved", yesterday, &first),
	}
	m.records[0].Attempt.HintLevel = 2
	m.records[1].LastCheckOutcome = "fail"
	row := m.selectedDaily()
	if row.challenge.ID != "shell.task-00" || row.status() != "Done today" || row.assignment.Date != "2026-09-13" || row.attempt.Attempt.ID != "first-daily" || row.attempt.Attempt.HintLevel != 0 {
		t.Fatalf("row %+v", row)
	}
	m, _ = press(m, "enter")
	if m.currentAttempt().Status != "solved" || m.currentAttempt().ID != "first-daily" {
		t.Fatal("failed retry replaced success")
	}
	m, _ = press(m, "n")
	if m.challenge.ID != "shell.task-02" || m.assignment != nil {
		t.Fatal("extra exercise is not separate practice")
	}
	m.now = func() time.Time { return now.AddDate(0, 0, 1) }
	m.navigate(today)
	if got := m.selectedDaily(); got.challenge.ID != "shell.task-02" || got.status() != "Ready" {
		t.Fatalf("next day row %+v", got)
	}
	// A retry solved tomorrow must not assign a new completion day to the group.
	tomorrow := now.AddDate(0, 0, 1)
	m.records = append([]model.Progress{dailyRecord("solved-retry", "shell.task-00", "2026-09-13", "solved", tomorrow, &tomorrow)}, m.records...)
	if got := m.selectedDaily(); got.challenge.ID != "shell.task-02" {
		t.Fatal("retry created a second daily completion")
	}
}

func TestDailyAbandonedAssignmentNeedsExactRetryAndRetainsOldRevision(t *testing.T) {
	m := testModel(t)
	yesterday := m.now().AddDate(0, 0, -1)
	old := m.service.Catalog.Challenges[0]
	newer := old
	newer.Revision = 2
	m.service.Catalog.Challenges = append(m.service.Catalog.Challenges, newer)
	m.records = []model.Progress{
		dailyRecord("library-solved", "shell.task-00", "", "solved", yesterday, &yesterday),
		dailyRecord("abandoned", "shell.task-00", "2026-09-13", "abandoned", yesterday, &yesterday),
	}
	row := m.selectedDaily()
	if row.challenge.Revision != 1 || row.status() != "Retry" {
		t.Fatalf("old assignment lost: %+v", row)
	}
	now := m.now()
	m.records = append([]model.Progress{dailyRecord("daily-solved", "shell.task-00", "2026-09-13", "solved", now, &now)}, m.records...)
	m.now = func() time.Time { return now.AddDate(0, 0, 1) }
	row = m.selectedDaily()
	if row.challenge.ID != old.ID || row.challenge.Revision != 2 || row.status() != "Ready" {
		t.Fatalf("new revision not eligible: %+v", row)
	}
}

func TestLibraryAndExtraPracticeNeverResumeDailyAttempt(t *testing.T) {
	m := testModel(t)
	now := m.now()
	m.records = []model.Progress{dailyRecord("daily", "shell.task-00", "2026-09-14", "active", now, nil), dailyRecord("library", "shell.task-00", "", "active", now.Add(-time.Hour), nil)}
	m.openChallenge(m.service.Catalog.Challenges[0], nil, practice)
	if m.attemptID != "library" {
		t.Fatal("library resumed daily attempt")
	}
	m.records = m.records[:1]
	m.openChallenge(m.service.Catalog.Challenges[0], nil, practice)
	if m.attemptID != "" {
		t.Fatal("extra practice reused a daily attempt")
	}
}

func TestDailyStateUsesLatestCheckAndNeverCompletesInterimPass(t *testing.T) {
	m := testModel(t)
	r := dailyRecord("daily", "shell.task-00", "2026-09-14", "active", m.now(), nil)
	for _, item := range []struct{ status, outcome, want string }{{"active", "fail", "Check failed"}, {"active", "pass", "Resume"}, {"infrastructure_error", "infrastructure_error", "Check error"}, {"abandoned", "fail", "Retry"}} {
		r.Attempt.Status, r.LastCheckOutcome = item.status, item.outcome
		r.FailedChecks = 4
		m.records = []model.Progress{r}
		if got := m.selectedDaily().status(); got != item.want {
			t.Fatalf("%s/%s: %s", item.status, item.outcome, got)
		}
	}
}

func TestDailyPublishedAssignmentOnlyMatchesNextExercise(t *testing.T) {
	m := testModel(t)
	a := model.Assignment{Date: "2026-09-14", Track: "shell", ExerciseID: "shell.task-01", Revision: 1, Seed: "published"}
	m.service.Catalog.Assignments = []model.Assignment{a}
	if got := m.selectedDaily(); got.challenge.ID != "shell.task-00" || got.assignment.Seed != "default" {
		t.Fatal("published schedule bypassed next exercise")
	}
	m.records = []model.Progress{dailyRecord("solved", "shell.task-00", "", "solved", m.now(), nil)}
	if got := m.selectedDaily(); *got.assignment != a {
		t.Fatalf("matching publication identity changed: %+v", got.assignment)
	}
}

func TestLibraryFiltersPreserveSolvedOverviewAndTodaySelection(t *testing.T) {
	m := testModel(t)
	c := m.service.Catalog.Challenges[0]
	c.Track, c.ID = "awk", "awk.task-00"
	m.service.Catalog.Challenges = append(m.service.Catalog.Challenges, c)
	now := m.now()
	m.records = []model.Progress{dailyRecord("failed", "shell.task-00", "", "active", now, nil), dailyRecord("passed", "shell.task-00", "", "solved", now.Add(-time.Hour), nil)}
	m.records[0].LastCheckOutcome = "fail"
	m.selected = 1
	m, _ = press(m, "l")
	if m.screen != practice || m.selected != 0 {
		t.Fatal("library shortcut failed")
	}
	m, _ = press(m, "t")
	if m.filterTrack != "awk" || len(m.filtered()) != 1 {
		t.Fatal("track filter failed")
	}
	m, _ = press(m, "t")
	if m.filterTrack != "shell" {
		t.Fatal("track filter did not cycle")
	}
	m, _ = press(m, "f")
	if len(m.filtered()) != 29 {
		t.Fatal("untried filter includes solved retry")
	}
	m, _ = press(m, "f")
	if len(m.filtered()) != 0 {
		t.Fatal("active filter regressed solved exercise")
	}
	m, _ = press(m, "f")
	if got := m.filtered(); len(got) != 1 || got[0].ID != "shell.task-00" {
		t.Fatal("solved filter failed")
	}
	m, _ = press(m, "1")
	if m.screen != today || m.selected != 1 {
		t.Fatal("Today selection lost")
	}
}

func TestDailyRosterSurvivesRelaunchFromSavedHistory(t *testing.T) {
	m := testModel(t)
	a := dailyRecord("persisted", "shell.task-00", "2026-09-14", "active", m.now(), nil).Attempt
	a.ValidatorVersion = "1"
	if err := m.service.Store.CreateAttempt(a); err != nil {
		t.Fatal(err)
	}
	records, err := m.service.Store.Progress()
	if err != nil {
		t.Fatal(err)
	}
	relaunched := New(m.service)
	t.Cleanup(func() { relaunched.lifecycle.shutdown() })
	relaunched.now = m.now
	relaunched.records = records
	if got := relaunched.selectedDaily(); got.challenge.ID != "shell.task-00" || got.status() != "Resume" || got.assignment.Date != "2026-09-14" {
		t.Fatalf("relaunch lost daily assignment: %+v", got)
	}
}

func TestDailyCompleteTrackAndMissingPreferredTrack(t *testing.T) {
	m := testModel(t)
	m.service.Config.Track = "removed-track"
	relaunched := New(m.service)
	t.Cleanup(func() { relaunched.lifecycle.shutdown() })
	if relaunched.chooseTrack || relaunched.selected != 0 || len(relaunched.dailyTracks()) != 1 {
		t.Fatal("missing preferred track prevented all-track launch")
	}
	for _, c := range m.service.Catalog.Current() {
		m.records = append(m.records, model.Progress{Attempt: model.Attempt{ExerciseID: c.ID, Revision: c.Revision, Status: "solved"}})
	}
	row := m.selectedDaily()
	if row.challenge != nil || row.assignment != nil || row.status() != "Complete" || row.solved != row.total {
		t.Fatalf("completed track: %+v", row)
	}
	m, cmd := press(m, "enter")
	if m.screen != today || cmd != nil || m.challenge != nil {
		t.Fatal("completed track started another exercise")
	}
}

func TestPreferredTrackPickerSelectsChosenTodayRow(t *testing.T) {
	m := testModel(t)
	c := m.service.Catalog.Challenges[0]
	c.ID, c.Track = "awk.task-00", "awk"
	m.service.Catalog.Challenges = append(m.service.Catalog.Challenges, c)
	m.screen = settings
	m, _ = press(m, "t")
	if m.trackIndex != 1 {
		t.Fatal("preferred track picker lost current selection")
	}
	m, _ = press(m, "enter")
	if m.screen != today || m.selected != 1 || m.selectedDaily().track != "shell" {
		t.Fatal("saved preferred track did not select its Today row")
	}
}

func TestDailyCompletionDayUsesUTC(t *testing.T) {
	m := testModel(t)
	finished := time.Date(2026, 9, 13, 20, 0, 0, 0, time.FixedZone("local", -7*60*60))
	m.records = []model.Progress{dailyRecord("finished", "shell.task-00", "2026-09-13", "solved", finished.Add(-time.Hour), &finished)}
	if got := m.selectedDaily(); got.status() != "Done today" {
		t.Fatalf("UTC completion missing: %+v", got)
	}
}

func TestDailyOlderUnfinishedAssignmentPrecedesLaterCLICompletion(t *testing.T) {
	m := testModel(t)
	now := m.now()
	yesterday := now.AddDate(0, 0, -1)
	m.records = []model.Progress{
		dailyRecord("later-cli-solved", "shell.task-08", "2026-09-14", "solved", now.Add(-time.Hour), &now),
		dailyRecord("older-active", "shell.task-01", "2026-09-13", "active", yesterday, nil),
	}
	row := m.selectedDaily()
	if row.challenge.ID != "shell.task-01" || row.status() != "Resume" || row.attempt.Attempt.ID != "older-active" {
		t.Fatalf("later CLI completion hid unfinished assignment: %+v", row)
	}
	m, _ = press(m, "enter")
	if m.attemptID != "older-active" {
		t.Fatal("Today did not resume the retained assignment")
	}
}

func TestDailyCompletionPrecedesLaterCLIActiveAssignment(t *testing.T) {
	m := testModel(t)
	now := m.now()
	yesterday := now.AddDate(0, 0, -1)
	m.records = []model.Progress{
		dailyRecord("later-cli-active", "shell.task-08", "2026-09-14", "active", now, nil),
		dailyRecord("first-solved", "shell.task-01", "2026-09-13", "solved", yesterday, &now),
	}
	row := m.selectedDaily()
	if row.challenge.ID != "shell.task-01" || row.status() != "Done today" || row.attempt.Attempt.ID != "first-solved" {
		t.Fatalf("later CLI assignment hid today's completion: %+v", row)
	}
}
