// Package store retains local practice results. Callers hold Lock during writer use.
package store

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
	_ "modernc.org/sqlite"
)

const schemaVersion = 1
const DatabaseName = "driving-range.db"

var ErrFinished = errors.New("attempt is already finished")

type Store struct{ db *sql.DB }

func Open(stateDir string) (*Store, error)         { return open(stateDir, false) }
func OpenReadOnly(stateDir string) (*Store, error) { return open(stateDir, true) }

func open(stateDir string, readOnly bool) (*Store, error) {
	if !readOnly {
		if err := os.MkdirAll(stateDir, 0700); err != nil {
			return nil, err
		}
	}
	path, err := filepath.Abs(filepath.Join(stateDir, DatabaseName))
	if err != nil {
		return nil, err
	}
	info, statErr := os.Stat(path)
	existed := statErr == nil && info.Size() > 0
	if readOnly && os.IsNotExist(statErr) {
		return nil, statErr
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, statErr
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	if readOnly {
		q.Set("mode", "ro")
	} else {
		q.Add("_pragma", "journal_mode(WAL)")
	}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err = s.migrate(path, existed, readOnly); err != nil {
		db.Close()
		return nil, err
	}
	if !readOnly {
		if err = os.Chmod(path, 0600); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(path string, existed, readOnly bool) error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > schemaVersion {
		return fmt.Errorf("database schema %d is newer than supported schema %d", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}
	if readOnly {
		return fmt.Errorf("database schema %d needs migration; open golf as a writer first", version)
	}
	if existed {
		backup := path + ".backup-" + time.Now().UTC().Format("20060102T150405.000000000")
		// VACUUM INTO includes committed WAL pages, unlike copying only the main file.
		if _, err := s.db.Exec("VACUUM INTO ?", backup); err != nil {
			return fmt.Errorf("backup database: %w", err)
		}
		if err := os.Chmod(backup, 0600); err != nil {
			return err
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
CREATE TABLE attempts (
 id TEXT PRIMARY KEY NOT NULL,
 data TEXT NOT NULL CHECK(json_valid(data))
);
CREATE TABLE sessions (
 id INTEGER PRIMARY KEY,
 attempt_id TEXT NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
 started_at TEXT NOT NULL,
 ended_at TEXT,
 duration_ms INTEGER CHECK(duration_ms >= 0),
 outcome TEXT NOT NULL CHECK(outcome IN ('active','completed','interrupted','infrastructure_error')),
 exit_code INTEGER
);
CREATE UNIQUE INDEX one_open_session ON sessions(attempt_id) WHERE ended_at IS NULL;
CREATE INDEX sessions_attempt ON sessions(attempt_id);
CREATE TABLE check_events (
 id INTEGER PRIMARY KEY,
 attempt_id TEXT NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
 created_at TEXT NOT NULL,
 result TEXT NOT NULL CHECK(json_valid(result))
);
CREATE INDEX checks_attempt ON check_events(attempt_id);
PRAGMA user_version = 1;`)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return tx.Commit()
}

func (s *Store) CreateAttempt(a model.Attempt) error {
	if a.ID == "" || a.ExerciseID == "" || a.Revision < 1 || a.Track == "" || a.Profile == "" || a.ValidatorVersion == "" {
		return errors.New("attempt requires id, exercise, positive revision, track, profile, and validator version")
	}
	if a.Status == "" {
		a.Status = "active"
	}
	if a.Status != "active" || a.FinishedAt != nil || a.HintLevel < 0 {
		return errors.New("new attempt must be active with nonnegative hint level")
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	} else {
		a.CreatedAt = a.CreatedAt.UTC()
	}
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if a.RetryOf != "" {
		prior, err := attempt(tx, a.RetryOf)
		if err != nil {
			return fmt.Errorf("retry parent: %w", err)
		}
		if prior.ExerciseID != a.ExerciseID {
			return errors.New("retry parent belongs to another exercise")
		}
	}
	if _, err = tx.Exec("INSERT INTO attempts(id,data) VALUES(?,?)", a.ID, string(data)); err != nil {
		return err
	}
	return tx.Commit()
}

type queryRower interface{ QueryRow(string, ...any) *sql.Row }

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
}

func attempt(q queryRower, id string) (model.Attempt, error) {
	var a model.Attempt
	var data string
	if err := q.QueryRow("SELECT data FROM attempts WHERE id = ?", id).Scan(&data); err != nil {
		return a, err
	}
	err := json.Unmarshal([]byte(data), &a)
	return a, err
}

func (s *Store) Attempt(id string) (model.Attempt, error) { return attempt(s.db, id) }

func (s *Store) Attempts() ([]model.Attempt, error) { return attempts(s.db) }

func attempts(q queryer) ([]model.Attempt, error) {
	rows, err := q.Query("SELECT data FROM attempts ORDER BY json_extract(data,'$.created_at') DESC, id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Attempt{}
	for rows.Next() {
		var raw string
		var a model.Attempt
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func saveAttempt(tx *sql.Tx, a model.Attempt) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = tx.Exec("UPDATE attempts SET data = ? WHERE id = ?", string(data), a.ID)
	return err
}

func finished(a model.Attempt) bool { return a.Status == "solved" || a.Status == "abandoned" }

func (s *Store) mutateAttempt(id string, fn func(*sql.Tx, *model.Attempt) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	a, err := attempt(tx, id)
	if err != nil {
		return err
	}
	if finished(a) {
		return ErrFinished
	}
	if err = fn(tx, &a); err != nil {
		return err
	}
	if err = saveAttempt(tx, a); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) StartSession(attemptID string) (int64, error) {
	var id int64
	err := s.mutateAttempt(attemptID, func(tx *sql.Tx, a *model.Attempt) error {
		result, err := tx.Exec("INSERT INTO sessions(attempt_id,started_at,outcome) VALUES(?,?,'active')", attemptID, now())
		if err != nil {
			return err
		}
		id, err = result.LastInsertId()
		a.Status = "active"
		return err
	})
	return id, err
}

func (s *Store) EndSession(sessionID int64, result model.SessionResult) error {
	if result.DurationKnown && result.DurationMS < 0 {
		return errors.New("session duration cannot be negative")
	}
	var attemptID string
	if err := s.db.QueryRow("SELECT attempt_id FROM sessions WHERE id=?", sessionID).Scan(&attemptID); err != nil {
		return err
	}
	return s.mutateAttempt(attemptID, func(tx *sql.Tx, a *model.Attempt) error {
		outcome := "completed"
		if result.Error != "" {
			outcome = "infrastructure_error"
		} else if result.Interrupted {
			outcome = "interrupted"
		}
		var duration any
		if result.DurationKnown {
			duration = result.DurationMS
		}
		r, err := tx.Exec("UPDATE sessions SET ended_at=?,duration_ms=?,outcome=?,exit_code=? WHERE id=? AND ended_at IS NULL", now(), duration, outcome, result.ExitCode, sessionID)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("session is already finished")
		}
		if outcome != "completed" {
			a.Status = outcome
		}
		return nil
	})
}

// RecordInterimCheck records a check taken while the exercise session is still
// running. It keeps the history entry but never finishes the attempt: the
// operator can keep editing after a passing probe, so only the check that runs
// when the session ends may mark an attempt solved.
func (s *Store) RecordInterimCheck(attemptID string, result model.CheckResult) error {
	if result.Outcome != "pass" && result.Outcome != "fail" && result.Outcome != "infrastructure_error" {
		return fmt.Errorf("invalid check outcome %q", result.Outcome)
	}
	return s.mutateAttempt(attemptID, func(tx *sql.Tx, a *model.Attempt) error {
		if a.Status == "solved" || a.Status == "abandoned" {
			return errors.New("finished attempts are immutable")
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO check_events(attempt_id,created_at,result) VALUES(?,?,?)", attemptID, now(), string(data)); err != nil {
			return err
		}
		if result.Outcome == "infrastructure_error" {
			a.Status = "infrastructure_error"
		} else {
			a.Status = "active"
		}
		return nil
	})
}

func (s *Store) RecordCheck(attemptID string, result model.CheckResult) error {
	if result.Outcome != "pass" && result.Outcome != "fail" && result.Outcome != "infrastructure_error" {
		return fmt.Errorf("invalid check outcome %q", result.Outcome)
	}
	return s.mutateAttempt(attemptID, func(tx *sql.Tx, a *model.Attempt) error {
		var active int
		if err := tx.QueryRow("SELECT count(*) FROM sessions WHERE attempt_id=? AND ended_at IS NULL", attemptID).Scan(&active); err != nil {
			return err
		}
		if active != 0 {
			return errors.New("end the exercise session before checking")
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO check_events(attempt_id,created_at,result) VALUES(?,?,?)", attemptID, now(), string(data)); err != nil {
			return err
		}
		switch result.Outcome {
		case "pass":
			a.Status = "solved"
			t := time.Now().UTC()
			a.FinishedAt = &t
		case "fail":
			a.Status = "active"
		case "infrastructure_error":
			a.Status = "infrastructure_error"
		}
		return nil
	})
}

func (s *Store) SetAssistance(attemptID string, hints int, revealed, external bool) error {
	if hints < 0 {
		return errors.New("hint level cannot be negative")
	}
	return s.mutateAttempt(attemptID, func(_ *sql.Tx, a *model.Attempt) error {
		// Assistance is cumulative, so a later session cannot erase an earlier hint.
		if hints > a.HintLevel {
			a.HintLevel = hints
		}
		a.SolutionRevealed = a.SolutionRevealed || revealed
		a.ExternalAssistance = a.ExternalAssistance || external
		return nil
	})
}

func (s *Store) AbandonAttempt(attemptID string) error {
	return s.mutateAttempt(attemptID, func(tx *sql.Tx, a *model.Attempt) error {
		t := time.Now().UTC()
		if _, err := tx.Exec("UPDATE sessions SET ended_at=?,duration_ms=NULL,outcome='interrupted' WHERE attempt_id=? AND ended_at IS NULL", t.Format(time.RFC3339Nano), attemptID); err != nil {
			return err
		}
		a.Status = "abandoned"
		a.FinishedAt = &t
		return nil
	})
}

func (s *Store) RecoverInterrupted() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Only sessions with no recorded end indicate unobserved foreground time.
	_, err = tx.Exec(`UPDATE attempts SET data=json_set(data,'$.status','interrupted')
WHERE json_extract(data,'$.status') NOT IN ('solved','abandoned')
AND id IN (SELECT attempt_id FROM sessions WHERE ended_at IS NULL)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec("UPDATE sessions SET ended_at=?,duration_ms=NULL,outcome='interrupted' WHERE ended_at IS NULL", now())
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Sessions(attemptID string) ([]model.Session, error) { return sessions(s.db, attemptID) }

func sessions(q queryer, attemptID string) ([]model.Session, error) {
	rows, err := q.Query("SELECT id,attempt_id,started_at,ended_at,duration_ms,outcome,exit_code FROM sessions WHERE (?='' OR attempt_id=?) ORDER BY id", attemptID, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Session{}
	for rows.Next() {
		var item model.Session
		var start string
		var end sql.NullString
		if err = rows.Scan(&item.ID, &item.AttemptID, &start, &end, &item.DurationMS, &item.Outcome, &item.ExitCode); err != nil {
			return nil, err
		}
		if item.StartedAt, err = time.Parse(time.RFC3339Nano, start); err != nil {
			return nil, err
		}
		if end.Valid {
			t, err := time.Parse(time.RFC3339Nano, end.String)
			if err != nil {
				return nil, err
			}
			item.EndedAt = &t
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Checks(attemptID string) ([]model.CheckEvent, error) { return checks(s.db, attemptID) }

func checks(q queryer, attemptID string) ([]model.CheckEvent, error) {
	rows, err := q.Query("SELECT id,attempt_id,created_at,result FROM check_events WHERE (?='' OR attempt_id=?) ORDER BY id", attemptID, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.CheckEvent{}
	for rows.Next() {
		var item model.CheckEvent
		var created, raw string
		if err = rows.Scan(&item.ID, &item.AttemptID, &created, &raw); err != nil {
			return nil, err
		}
		if item.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &item.Result); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Progress() ([]model.Progress, error) {
	rows, err := s.db.Query(`SELECT a.data,
COALESCE((SELECT SUM(duration_ms) FROM sessions s WHERE s.attempt_id=a.id),0),
NOT EXISTS(SELECT 1 FROM sessions s WHERE s.attempt_id=a.id) OR EXISTS(SELECT 1 FROM sessions s WHERE s.attempt_id=a.id AND duration_ms IS NULL),
(SELECT COUNT(*) FROM check_events c WHERE c.attempt_id=a.id),
(SELECT COUNT(*) FROM check_events c WHERE c.attempt_id=a.id AND json_extract(c.result,'$.outcome')='fail')
FROM attempts a ORDER BY json_extract(a.data,'$.created_at') DESC, a.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Progress{}
	for rows.Next() {
		var p model.Progress
		var raw string
		if err = rows.Scan(&raw, &p.DurationMS, &p.UnknownDuration, &p.Checks, &p.FailedChecks); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &p.Attempt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Best(a model.Attempt) (*model.Progress, error) {
	progress, err := s.Progress()
	if err != nil {
		return nil, err
	}
	var best *model.Progress
	for _, p := range progress {
		b := p.Attempt
		if b.Status != "solved" || p.UnknownDuration || b.ExerciseID != a.ExerciseID || b.Revision != a.Revision || b.Seed != a.Seed || b.Profile != a.Profile || b.EnvironmentID != a.EnvironmentID || b.ValidatorVersion != a.ValidatorVersion || b.HintLevel != a.HintLevel || b.SolutionRevealed != a.SolutionRevealed || b.ExternalAssistance != a.ExternalAssistance {
			continue
		}
		if best == nil || p.DurationMS < best.DurationMS {
			v := p
			best = &v
		}
	}
	return best, nil
}

func (s *Store) ExportJSON(w io.Writer) error {
	// A read transaction keeps every exported table on the same WAL snapshot.
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var data model.Export
	data.SchemaVersion = schemaVersion
	data.ExportedAt = time.Now().UTC()
	if data.Attempts, err = attempts(tx); err != nil {
		return err
	}
	if data.Sessions, err = sessions(tx, ""); err != nil {
		return err
	}
	if data.Checks, err = checks(tx, ""); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func (s *Store) ExportCSV(w io.Writer) error {
	progress, err := s.Progress()
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err = cw.Write([]string{"schema_version", "attempt_id", "exercise_id", "revision", "track", "assignment_date", "seed", "profile", "environment_id", "validator_version", "status", "created_at", "finished_at", "duration_ms", "unknown_duration", "checks", "hint_level", "solution_revealed", "external_assistance", "retry_of", "failed_checks"}); err != nil {
		return err
	}
	for _, p := range progress {
		a := p.Attempt
		finish := ""
		if a.FinishedAt != nil {
			finish = a.FinishedAt.Format(time.RFC3339Nano)
		}
		duration := ""
		if !p.UnknownDuration {
			duration = strconv.FormatInt(p.DurationMS, 10)
		}
		row := []string{strconv.Itoa(schemaVersion), a.ID, a.ExerciseID, strconv.Itoa(a.Revision), a.Track, a.AssignmentDate, a.Seed, a.Profile, a.EnvironmentID, a.ValidatorVersion, a.Status, a.CreatedAt.Format(time.RFC3339Nano), finish, duration, strconv.FormatBool(p.UnknownDuration), strconv.Itoa(p.Checks), strconv.Itoa(a.HintLevel), strconv.FormatBool(a.SolutionRevealed), strconv.FormatBool(a.ExternalAssistance), a.RetryOf, strconv.Itoa(p.FailedChecks)}
		for i := range row {
			row[i] = csvText(row[i])
		}
		if err = cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func csvText(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsAny(trimmed[:1], "=+-@") {
		return "'" + value
	}
	return value
}

func (s *Store) DeleteAttempt(id string) error {
	r, err := s.db.Exec("DELETE FROM attempts WHERE id=?", id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
