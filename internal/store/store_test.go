package store

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func testAttempt(id string) model.Attempt {
	return model.Attempt{ID: id, ExerciseID: "bash-pipes", Revision: 1, Track: "shell", Seed: "one", Profile: "standard", EnvironmentID: "sha256:one", ValidatorVersion: "stdout-v1"}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	must(t, err)
	t.Cleanup(func() { s.Close() })
	return s, dir
}

func timedSession(t *testing.T, s *Store, id string, duration int64) {
	t.Helper()
	session, err := s.StartSession(id)
	must(t, err)
	must(t, s.EndSession(session, model.SessionResult{DurationKnown: true, DurationMS: duration}))
}

func TestLifecycleRestartRetryAndImmutability(t *testing.T) {
	s, dir := newStore(t)
	a := testAttempt("first")
	must(t, s.CreateAttempt(a))
	timedSession(t, s, a.ID, 1200)
	must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "fail", Summary: "expected sorted output"}))
	must(t, s.SetAssistance(a.ID, 1, false, false))
	must(t, s.Close())
	s, err := Open(dir)
	must(t, err)
	defer s.Close()
	stored, err := s.Attempt(a.ID)
	must(t, err)
	if stored.Status != "active" || stored.HintLevel != 1 {
		t.Fatalf("restart lost state: %+v", stored)
	}
	timedSession(t, s, a.ID, 300)
	must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "pass", Summary: "correct"}))
	stored, err = s.Attempt(a.ID)
	must(t, err)
	if stored.Status != "solved" || stored.FinishedAt == nil {
		t.Fatalf("missing completion: %+v", stored)
	}
	before, _ := json.Marshal(stored)
	if _, err = s.StartSession(a.ID); !errors.Is(err, ErrFinished) {
		t.Fatalf("solved session accepted: %v", err)
	}
	for _, err := range []error{s.SetAssistance(a.ID, 2, true, true), s.RecordCheck(a.ID, model.CheckResult{Outcome: "fail"}), s.FinishAttempt(a.ID, "abandoned")} {
		if !errors.Is(err, ErrFinished) {
			t.Fatalf("solved mutation accepted: %v", err)
		}
	}
	retry := testAttempt("retry")
	retry.RetryOf = a.ID
	must(t, s.CreateAttempt(retry))
	must(t, s.RecoverInterrupted())
	stored, err = s.Attempt(a.ID)
	must(t, err)
	after, _ := json.Marshal(stored)
	if !bytes.Equal(before, after) {
		t.Fatal("retry or recovery changed completed attempt")
	}
	progress, err := s.Progress()
	must(t, err)
	for _, p := range progress {
		if p.Attempt.ID == a.ID && (p.DurationMS != 1500 || p.Checks != 2 || p.UnknownDuration) {
			t.Fatalf("wrong totals: %+v", p)
		}
	}
	checks, err := s.Checks(a.ID)
	must(t, err)
	if len(checks) != 2 || checks[0].Result.Outcome != "fail" || checks[1].Result.Outcome != "pass" {
		t.Fatalf("lost checks: %+v", checks)
	}
}

func TestRecoveryInfrastructureAndResume(t *testing.T) {
	s, dir := newStore(t)
	a := testAttempt("crash")
	must(t, s.CreateAttempt(a))
	timedSession(t, s, a.ID, 30)
	_, err := s.StartSession(a.ID)
	must(t, err)
	must(t, s.Close())
	s, err = Open(dir)
	must(t, err)
	defer s.Close()
	must(t, s.RecoverInterrupted())
	a, err = s.Attempt(a.ID)
	must(t, err)
	if a.Status != "interrupted" || a.FinishedAt != nil {
		t.Fatalf("bad crash recovery: %+v", a)
	}
	sessions, err := s.Sessions(a.ID)
	must(t, err)
	if len(sessions) != 2 || sessions[1].DurationMS != nil || sessions[1].Outcome != "interrupted" || sessions[1].EndedAt == nil {
		t.Fatalf("invented crash duration: %+v", sessions)
	}
	session, err := s.StartSession(a.ID)
	must(t, err)
	must(t, s.EndSession(session, model.SessionResult{Error: "runtime missing", DurationKnown: false, ExitCode: 127}))
	a, err = s.Attempt(a.ID)
	must(t, err)
	if a.Status != "infrastructure_error" {
		t.Fatal(a.Status)
	}
	must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "infrastructure_error", Summary: "runtime missing"}))
	timedSession(t, s, a.ID, 10)
	must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "pass"}))
	best, err := s.Best(a)
	must(t, err)
	if best != nil {
		t.Fatal("unknown crash time was ranked")
	}
	progress, err := s.Progress()
	must(t, err)
	if len(progress) != 1 || !progress[0].UnknownDuration || progress[0].DurationMS != 40 {
		t.Fatalf("wrong partial known duration: %+v", progress)
	}
}

func TestBestComparisonCohorts(t *testing.T) {
	s, _ := newStore(t)
	base := testAttempt("baseline")
	must(t, s.CreateAttempt(base))
	timedSession(t, s, base.ID, 100)
	must(t, s.RecordCheck(base.ID, model.CheckResult{Outcome: "pass"}))
	variants := []model.Attempt{}
	for i, mutate := range []func(*model.Attempt){
		func(a *model.Attempt) { a.Revision++ },
		func(a *model.Attempt) { a.Seed = "two" },
		func(a *model.Attempt) { a.Profile = "personal" },
		func(a *model.Attempt) { a.EnvironmentID = "sha256:two" },
		func(a *model.Attempt) { a.ValidatorVersion = "v2" },
		func(a *model.Attempt) { a.HintLevel = 1 },
		func(a *model.Attempt) { a.SolutionRevealed = true },
		func(a *model.Attempt) { a.ExternalAssistance = true },
	} {
		a := testAttempt(string(rune('a' + i)))
		mutate(&a)
		variants = append(variants, a)
	}
	for _, a := range variants {
		must(t, s.CreateAttempt(a))
		timedSession(t, s, a.ID, 1)
		must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "pass"}))
	}
	noSession := testAttempt("no-session")
	must(t, s.CreateAttempt(noSession))
	must(t, s.RecordCheck(noSession.ID, model.CheckResult{Outcome: "pass"}))
	best, err := s.Best(base)
	must(t, err)
	if best == nil || best.Attempt.ID != base.ID || best.DurationMS != 100 {
		t.Fatalf("incomparable best: %+v", best)
	}
	faster := testAttempt("faster")
	must(t, s.CreateAttempt(faster))
	timedSession(t, s, faster.ID, 50)
	must(t, s.RecordCheck(faster.ID, model.CheckResult{Outcome: "pass"}))
	best, err = s.Best(base)
	must(t, err)
	if best == nil || best.Attempt.ID != faster.ID {
		t.Fatalf("missed best: %+v", best)
	}
}

func TestFailedChecksExcludeInfrastructureEvents(t *testing.T) {
	s, _ := newStore(t)
	must(t, s.CreateAttempt(testAttempt("infra")))
	must(t, s.RecordCheck("infra", model.CheckResult{Outcome: "infrastructure_error"}))
	a := testAttempt("failed-once")
	must(t, s.CreateAttempt(a))
	timedSession(t, s, a.ID, 20)
	must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "fail"}))
	must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "infrastructure_error"}))
	must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "pass"}))
	progress, err := s.Progress()
	must(t, err)
	for _, p := range progress {
		if p.Attempt.ID == "infra" && (p.Checks != 1 || p.FailedChecks != 0) {
			t.Fatalf("infrastructure event counted as exercise failure: %+v", p)
		}
		if p.Attempt.ID == a.ID && (p.Checks != 3 || p.FailedChecks != 1) {
			t.Fatalf("incorrect failed-check count: %+v", p)
		}
	}
	best, err := s.Best(a)
	must(t, err)
	if best == nil || best.FailedChecks != 1 {
		t.Fatalf("best lost failed-check count: %+v", best)
	}
}

func TestChecksAssistanceAndAbandon(t *testing.T) {
	s, _ := newStore(t)
	a := testAttempt("active")
	must(t, s.CreateAttempt(a))
	id, err := s.StartSession(a.ID)
	must(t, err)
	if _, err = s.StartSession(a.ID); err == nil {
		t.Fatal("accepted two foreground sessions")
	}
	if err = s.RecordCheck(a.ID, model.CheckResult{Outcome: "pass"}); err == nil {
		t.Fatal("solved while session active")
	}
	must(t, s.EndSession(id, model.SessionResult{DurationKnown: true, DurationMS: 5}))
	if err = s.EndSession(id, model.SessionResult{}); err == nil {
		t.Fatal("overwrote finished session")
	}
	must(t, s.SetAssistance(a.ID, 2, true, true))
	must(t, s.SetAssistance(a.ID, 0, false, false))
	a, err = s.Attempt(a.ID)
	must(t, err)
	if a.HintLevel != 2 || !a.SolutionRevealed || !a.ExternalAssistance {
		t.Fatal("assistance erased")
	}
	if err = s.RecordCheck(a.ID, model.CheckResult{Outcome: "banana"}); err == nil {
		t.Fatal("invalid outcome accepted")
	}
	must(t, s.FinishAttempt(a.ID, "abandoned"))
	if _, err = s.StartSession(a.ID); !errors.Is(err, ErrFinished) {
		t.Fatal("abandoned resumed")
	}
	checks, err := s.Checks(a.ID)
	must(t, err)
	if len(checks) != 0 {
		t.Fatal("rejected checks leaked")
	}
}

func TestSessionInterruptionAndCleanupFailure(t *testing.T) {
	s, _ := newStore(t)
	for _, tc := range []struct {
		id     string
		result model.SessionResult
		want   string
	}{
		{"cancelled", model.SessionResult{Interrupted: true, DurationKnown: true, DurationMS: 20, ExitCode: 130}, "interrupted"},
		{"cleanup-failure", model.SessionResult{Interrupted: true, DurationKnown: true, DurationMS: 30, ExitCode: 130, Error: "container cleanup failed"}, "infrastructure_error"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			must(t, s.CreateAttempt(testAttempt(tc.id)))
			id, err := s.StartSession(tc.id)
			must(t, err)
			must(t, s.EndSession(id, tc.result))
			a, err := s.Attempt(tc.id)
			must(t, err)
			sessions, err := s.Sessions(tc.id)
			must(t, err)
			checks, err := s.Checks(tc.id)
			must(t, err)
			if a.Status != tc.want || len(sessions) != 1 || sessions[0].Outcome != tc.want || len(checks) != 0 {
				t.Fatalf("wrong session classification: attempt=%s sessions=%+v checks=%d", a.Status, sessions, len(checks))
			}
		})
	}
}

func TestExportsAndDeletion(t *testing.T) {
	s, _ := newStore(t)
	a := testAttempt("=formula")
	must(t, s.CreateAttempt(a))
	timedSession(t, s, a.ID, 123)
	must(t, s.RecordCheck(a.ID, model.CheckResult{Outcome: "fail", Details: []string{"line 2 differs"}}))
	var out bytes.Buffer
	must(t, s.ExportJSON(&out))
	var dump model.Export
	must(t, json.Unmarshal(out.Bytes(), &dump))
	if dump.SchemaVersion != 1 || len(dump.Attempts) != 1 || len(dump.Sessions) != 1 || len(dump.Checks) != 1 || dump.Checks[0].Result.Details[0] != "line 2 differs" {
		t.Fatalf("incomplete export: %+v", dump)
	}
	out.Reset()
	must(t, s.ExportCSV(&out))
	records, err := csv.NewReader(&out).ReadAll()
	must(t, err)
	if len(records) != 2 || records[1][1] != "'=formula" || records[1][13] != "123" || records[0][20] != "failed_checks" || records[1][20] != "1" {
		t.Fatalf("wrong CSV: %+v", records)
	}
	must(t, s.DeleteAttempt(a.ID))
	_, err = s.Attempt(a.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("attempt remains: %v", err)
	}
	sessions, err := s.Sessions(a.ID)
	must(t, err)
	checks, err := s.Checks(a.ID)
	must(t, err)
	if len(sessions)+len(checks) != 0 {
		t.Fatal("orphaned child rows")
	}
}

func TestReadOnlyAndLock(t *testing.T) {
	s, dir := newStore(t)
	must(t, s.CreateAttempt(testAttempt("one")))
	_, err := s.StartSession("one")
	must(t, err)
	unlock, err := Lock(dir)
	must(t, err)
	defer unlock()
	if release, err := Lock(dir); err == nil {
		release()
		t.Fatal("concurrent writer acquired lock")
	}
	ro, err := OpenReadOnly(dir)
	must(t, err)
	defer ro.Close()
	progress, err := ro.Progress()
	must(t, err)
	if len(progress) != 1 || progress[0].Attempt.Status != "active" {
		t.Fatal("readonly recovered or lost active attempt")
	}
	if err := ro.RecoverInterrupted(); err == nil {
		t.Fatal("readonly mutation accepted")
	}
	unlock()
	unlock()
	unlock2, err := Lock(dir)
	must(t, err)
	unlock2()
	missing := filepath.Join(t.TempDir(), "missing")
	if db, err := OpenReadOnly(missing); !errors.Is(err, os.ErrNotExist) {
		if db != nil {
			db.Close()
		}
		t.Fatalf("readonly missing state must retain os.ErrNotExist: %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("readonly created directory")
	}
}

func TestMigrationBackupAndNewerSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DatabaseName)
	db, err := sql.Open("sqlite", path)
	must(t, err)
	_, err = db.Exec("CREATE TABLE marker(value TEXT); INSERT INTO marker VALUES('saved')")
	must(t, err)
	must(t, db.Close())
	ro, err := OpenReadOnly(dir)
	if err == nil {
		ro.Close()
		t.Fatal("readonly migrated database")
	}
	s, err := Open(dir)
	must(t, err)
	backups, err := filepath.Glob(path + ".backup-*")
	must(t, err)
	if len(backups) != 1 {
		t.Fatalf("missing migration backup: %v", backups)
	}
	backup, err := sql.Open("sqlite", backups[0])
	must(t, err)
	var marker string
	must(t, backup.QueryRow("SELECT value FROM marker").Scan(&marker))
	must(t, backup.Close())
	if marker != "saved" {
		t.Fatal("backup lost original data")
	}
	_, err = s.db.Exec("PRAGMA user_version = 99")
	must(t, err)
	must(t, s.Close())
	if s, err = Open(dir); err == nil {
		s.Close()
		t.Fatal("newer database accepted")
	} else if !strings.Contains(err.Error(), "newer") {
		t.Fatal(err)
	}
}

func TestRecordInterimCheckKeepsTheAttemptOpen(t *testing.T) {
	db, _ := newStore(t)
	a := testAttempt("interim")
	must(t, db.CreateAttempt(a))
	if _, err := db.StartSession(a.ID); err != nil {
		t.Fatal(err)
	}
	// The ordinary record refuses while a session is live, because a pass
	// would finish an attempt the operator can still edit.
	if err := db.RecordCheck(a.ID, model.CheckResult{Outcome: "pass", Summary: "ok"}); err == nil {
		t.Fatal("RecordCheck should refuse while a session is open")
	}
	if err := db.RecordInterimCheck(a.ID, model.CheckResult{Outcome: "pass", Summary: "ok"}); err != nil {
		t.Fatalf("RecordInterimCheck: %v", err)
	}
	got, err := db.Attempt(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == "solved" {
		t.Error("a passing interim check must not finish the attempt")
	}
	if got.FinishedAt != nil {
		t.Error("a passing interim check must not set a finish time")
	}
	checks, err := db.Checks(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 {
		t.Errorf("interim check was not recorded in history: %d events", len(checks))
	}
}
