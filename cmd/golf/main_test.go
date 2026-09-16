package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func TestPlainCatalogCommands(t *testing.T) {
	t.Setenv("GOLF_STATE_DIR", t.TempDir())
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--plain"}, "Driving Range"},
		{[]string{"list", "vim"}, "vim"},
		{[]string{"today", "vim", "2026-09-14"}, "Daily 2026-09-14 UTC"},
		{[]string{"today", "vim", "2030-01-01"}, "No published daily"},
		{[]string{"audit"}, "Catalog metadata valid"},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			var out bytes.Buffer
			if err := run(tc.args, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("want %q in %q", tc.want, out.String())
			}
		})
	}
}

func TestConfigurationCommandsDoNotOpenHistory(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"config", "track", "shell"}, {"config"}, {"today"}, {"history"}} {
		var out bytes.Buffer
		if err := run(append([]string{"--state-dir", dir}, args...), &out); err != nil {
			t.Fatal(args, err)
		}
		if args[0] == "today" && !strings.Contains(out.String(), "shell") {
			t.Fatal("today ignored saved track", out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "driving-range.db")); !os.IsNotExist(err) {
		t.Fatal("configuration/read-only command created history", err)
	}
	var out bytes.Buffer
	if err := run([]string{"--state-dir", dir, "not-a-command"}, &out); err == nil {
		t.Fatal("unknown command accepted")
	}
}

func TestDailyNeverResumesPracticeOrAnotherRevision(t *testing.T) {
	daily := model.Assignment{Date: "2026-09-14", Track: "search", ExerciseID: "search.error-records", Revision: 1, Seed: "fixed"}
	attempt := model.Attempt{ExerciseID: daily.ExerciseID, Track: daily.Track, Revision: daily.Revision, Profile: "standard", Seed: "default"}
	if attempt.MatchesAssignment(daily) {
		t.Fatal("practice resumed as a daily")
	}
	attempt.AssignmentDate = daily.Date
	attempt.Seed = daily.Seed
	if !attempt.MatchesAssignment(daily) {
		t.Fatal("same frozen assignment not resumed")
	}
	attempt.Revision++
	if attempt.MatchesAssignment(daily) {
		t.Fatal("different revision resumed")
	}
}

func TestDailyResumesOlderMatchingAttempt(t *testing.T) {
	daily := model.Assignment{Date: "2026-09-14", Track: "search", ExerciseID: "search.error-records", Revision: 1, Seed: "fixed"}
	a := model.Attempt{ID: "daily", ExerciseID: daily.ExerciseID, AssignmentDate: daily.Date, Track: daily.Track, Revision: daily.Revision, Seed: daily.Seed, Profile: "standard", CreatedAt: time.Now()}
	practice := a
	practice.ID, practice.AssignmentDate = "practice", ""
	practice.CreatedAt = a.CreatedAt.Add(time.Minute)
	if got := latestAssignment([]model.Attempt{practice, a}, daily); got.ID != a.ID {
		t.Fatalf("newer practice hides daily: %+v", got)
	}
}

func TestPlainOutputSanitizesTerminalControls(t *testing.T) {
	got := safe("hello\x1b]52;c;secret\a\rworld\n\tend\u009b31m")
	if strings.ContainsAny(got, "\x1b\a\r\u009b") {
		t.Fatal("terminal controls survived")
	}
	if !strings.Contains(got, "\n\t") {
		t.Fatal("readable whitespace removed")
	}
}
