package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func TestSettingsAndReadOnlyHistorySurviveRestart(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg := s.Config
	cfg.Track = "vim"
	cfg.Theme = "plain"
	if err = s.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	ch, err := s.Catalog.Find("vim.change-value", 0)
	if err != nil {
		t.Fatal(err)
	}
	a := model.Attempt{ID: "test-local", ExerciseID: ch.ID, Revision: ch.Revision, Track: ch.Track, Profile: ch.Profile, ValidatorVersion: ch.Validator.Version, EnvironmentID: "test-image", Seed: "default", Workspace: "golf-test-local", Status: "active", CreatedAt: time.Now().UTC()}
	if err = s.Store.CreateAttempt(a); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Hint(a.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.SetExternalAssistance(a.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = s.Store.RecordCheck(a.ID, model.CheckResult{Outcome: "pass", Summary: "correct"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Hint(a.ID); err == nil {
		t.Fatal("mutated a finished attempt")
	}
	if _, err = s.Reveal(a.ID); err != nil {
		t.Fatal("reviewing solved explanation failed", err)
	}
	old := filepath.Join(dir, "keep.txt")
	if err = os.WriteFile(old, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.Export("json", old); err == nil {
		t.Fatal("export overwrote existing file")
	}
	content, _ := os.ReadFile(old)
	if string(content) != "keep" {
		t.Fatal("existing export target changed")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Config.Track != "vim" || s.Config.Theme != "plain" {
		t.Fatal("settings lost")
	}
	saved, err := s.Store.Attempt(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "solved" || saved.HintLevel != 1 || !saved.ExternalAssistance || saved.SolutionRevealed {
		t.Fatalf("wrong durable assistance/result: %+v", saved)
	}
	target := filepath.Join(dir, "results.json")
	if err = s.Export("json", target); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	var exported model.Export
	if err = json.Unmarshal(data, &exported); err != nil {
		t.Fatal(err)
	}
	if len(exported.Attempts) != 1 || len(exported.Checks) != 1 {
		t.Fatal("history omitted")
	}
	share, err := s.Share(a.ID)
	if err != nil || !strings.Contains(share, "assisted") {
		t.Fatal(share, err)
	}
}

func TestDockerAttemptLifecycle(t *testing.T) {
	if os.Getenv("GOLF_INTEGRATION") != "1" {
		t.Skip("set GOLF_INTEGRATION=1 for real isolated execution")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.NewAttempt(ctx, "search.error-records", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Runner.Cleanup(context.Background(), a)
	ch, _ := s.Catalog.Find(a.ExerciseID, a.Revision)
	sessionID, err := s.Store.StartSession(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Runner.Execute(ctx, ch, a, ":\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Store.EndSession(sessionID, res); err != nil {
		t.Fatal(err)
	}
	failed, err := s.Check(ctx, a.ID)
	if err != nil || failed.Outcome != "fail" {
		t.Fatalf("unchanged submission: %+v %v", failed, err)
	}
	sessionID, err = s.Store.StartSession(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	res, err = s.Runner.Execute(ctx, ch, a, ch.ReferenceSolution)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Store.EndSession(sessionID, res); err != nil {
		t.Fatal(err)
	}
	passed, err := s.Check(ctx, a.ID)
	if err != nil || passed.Outcome != "pass" {
		t.Fatalf("reference: %+v %v", passed, err)
	}
	sessions, _ := s.Store.Sessions(a.ID)
	checks, _ := s.Store.Checks(a.ID)
	if len(sessions) != 2 || len(checks) != 2 {
		t.Fatal("resume lost prior records")
	}
	retry, err := s.NewAttempt(ctx, ch.ID, nil, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Runner.Cleanup(context.Background(), retry)
	if retry.ID == a.ID || retry.Workspace == a.Workspace || retry.RetryOf != a.ID {
		t.Fatal("retry reused attempt state")
	}
	old, _ := s.Store.Attempt(a.ID)
	if old.Status != "solved" {
		t.Fatal("retry altered solved record")
	}
	play, err := s.Prepare(ctx, retry.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Finish(ctx, play, context.Canceled)
	if err != nil {
		t.Fatal(err)
	}
	interrupted, _ := s.Store.Attempt(retry.ID)
	events, _ := s.Store.Checks(retry.ID)
	if interrupted.Status != "interrupted" || len(events) != 0 {
		t.Fatalf("interruption fabricated a validation result: %+v %d", interrupted, len(events))
	}
}

func appDockerOutput(t *testing.T, ctx context.Context, args ...string) string {
	t.Helper()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestDockerPrepareRecoversBeforeFirstSessionWithoutErasingResume(t *testing.T) {
	if os.Getenv("GOLF_INTEGRATION") != "1" {
		t.Skip("set GOLF_INTEGRATION=1 for real isolated execution")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := t.TempDir()
	s, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	a, err := s.NewAttempt(ctx, "vim.change-value", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Runner.Cleanup(context.Background(), a)
	ch, err := s.Catalog.Find(a.ExerciseID, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce an interrupted population before app.Prepare commits StartSession.
	partial := ch
	partial.Fixtures = append([]model.Fixture(nil), ch.Fixtures...)
	partial.Fixtures[0].Files = map[string]string{ch.Entrypoint: "truncated", "preparation-only": "incomplete"}
	prepared, err := s.Runner.Prepare(ctx, partial, a)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Finish(context.Canceled)
	sessions, err := s.Store.Sessions(a.ID)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("preparation already recorded play: %+v %v", sessions, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	play, err := s.Prepare(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer play.Session.Finish(context.Canceled)
	container := strings.Fields(appDockerOutput(t, ctx, "ps", "--quiet", "--filter", "label=io.driving-range.attempt="+a.ID))
	if len(container) != 1 {
		t.Fatalf("expected one prepared container, got %v", container)
	}
	for name, want := range ch.Fixtures[0].Files {
		if got := appDockerOutput(t, ctx, "exec", container[0], "cat", "--", "/workspace/"+name); got != want {
			t.Fatalf("first-session recovery retained partial fixture %s: got %q want %q", name, got, want)
		}
	}
	appDockerOutput(t, ctx, "exec", container[0], "test", "!", "-e", "/workspace/preparation-only")
	appDockerOutput(t, ctx, "exec", container[0], "/bin/bash", "--noprofile", "--norc", "-c", `printf 'player edit\n' > "$1"; printf saved > /workspace/player-note`, "golf-test", "/workspace/"+ch.Entrypoint)
	if _, err = s.Finish(ctx, play, context.Canceled); err != nil {
		t.Fatal(err)
	}
	play, err = s.Prepare(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer play.Session.Finish(context.Canceled)
	container = strings.Fields(appDockerOutput(t, ctx, "ps", "--quiet", "--filter", "label=io.driving-range.attempt="+a.ID))
	if len(container) != 1 {
		t.Fatalf("expected one resumed container, got %v", container)
	}
	if got := appDockerOutput(t, ctx, "exec", container[0], "cat", "--", "/workspace/"+ch.Entrypoint); got != "player edit\n" {
		t.Fatalf("resume erased player edit: %q", got)
	}
	if got := appDockerOutput(t, ctx, "exec", container[0], "cat", "/workspace/player-note"); got != "saved" {
		t.Fatalf("resume erased player file: %q", got)
	}
	if _, err = s.Finish(ctx, play, context.Canceled); err != nil {
		t.Fatal(err)
	}
	sessions, err = s.Store.Sessions(a.ID)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("expected exactly two played sessions: %+v %v", sessions, err)
	}
	checks, err := s.Store.Checks(a.ID)
	if err != nil || len(checks) != 0 {
		t.Fatalf("preparation/recovery fabricated checks: %+v %v", checks, err)
	}
}

func TestDockerRetryKeepsOriginalAssignmentAndEnvironment(t *testing.T) {
	if os.Getenv("GOLF_INTEGRATION") != "1" {
		t.Skip("set GOLF_INTEGRATION=1 for real isolated execution")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ch, err := s.Catalog.Find("vim.change-value", 0)
	if err != nil {
		t.Fatal(err)
	}
	assignment := model.Assignment{Date: "2026-09-14", Track: ch.Track, ExerciseID: ch.ID, Revision: ch.Revision, Seed: "original-fixture"}
	a, err := s.NewAttempt(ctx, ch.ID, &assignment, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Abandon(a.ID); err != nil {
		t.Fatal(err)
	}
	before, err := s.Store.Attempt(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	newer := ch
	newer.Revision++
	newer.Validator.Version = "new-validator-version"
	s.Catalog.Challenges = append(s.Catalog.Challenges, newer)
	latest, err := s.Catalog.Find(ch.ID, 0)
	if err != nil || latest.Revision != newer.Revision {
		t.Fatalf("new revision was not installed: %+v %v", latest, err)
	}
	cfg := s.Config
	cfg.Image = "driving-range-missing-retry:" + a.ID
	if err = s.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	changedAssignment := assignment
	changedAssignment.Date = "2026-09-15"
	changedAssignment.Seed = "new-fixture"
	changedAssignment.Revision = newer.Revision
	retry, err := s.NewAttempt(ctx, ch.ID, &changedAssignment, a.ID)
	if err != nil {
		t.Fatalf("retry used changed runtime configuration: %v", err)
	}
	if retry.ID == a.ID || retry.Workspace == a.Workspace || retry.RetryOf != a.ID || retry.Status != "active" || retry.Revision != a.Revision || retry.EnvironmentID != a.EnvironmentID || retry.AssignmentDate != a.AssignmentDate || retry.Seed != a.Seed || retry.Profile != a.Profile || retry.ValidatorVersion != a.ValidatorVersion {
		t.Fatalf("retry changed frozen identity: previous=%+v retry=%+v", a, retry)
	}
	resolved, err := s.resolveAttempt(retry)
	if err != nil || resolved.Revision != ch.Revision {
		t.Fatalf("retry resolves to new revision: %+v %v", resolved, err)
	}
	after, err := s.Store.Attempt(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("retry rewrote original result: before=%+v after=%+v", before, after)
	}
}
