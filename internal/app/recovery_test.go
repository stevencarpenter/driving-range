package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func TestOfflineRecoveryKeepsBrowsingAndGatesExecution(t *testing.T) {
	dir := t.TempDir()
	service, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := service.Catalog.Find("vim.change-value", 0)
	if err != nil {
		t.Fatal(err)
	}
	attempt := model.Attempt{ID: "offline-recovery", ExerciseID: challenge.ID, Revision: challenge.Revision, Track: challenge.Track, Profile: challenge.Profile, ValidatorVersion: challenge.Validator.Version, EnvironmentID: "test-image", Seed: "default", Workspace: "golf-offline-recovery", Status: "active", CreatedAt: time.Now().UTC()}
	if err = service.Store.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Store.StartSession(attempt.ID); err != nil {
		t.Fatal(err)
	}
	if err = service.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", t.TempDir())
	service, err = Open(dir, false)
	if err != nil {
		t.Fatalf("offline browse failed: %v", err)
	}
	defer service.Close()
	if service.RecoveryWarning() == "" {
		t.Fatal("missing pending-recovery warning")
	}
	if _, err = service.Catalog.Find(challenge.ID, challenge.Revision); err != nil {
		t.Fatal(err)
	}
	var exported bytes.Buffer
	if err = service.ExportTo("json", &exported); err != nil || !strings.Contains(exported.String(), attempt.ID) {
		t.Fatalf("offline history: %v", err)
	}
	config := service.Config
	config.Theme = "plain"
	if err = service.SetConfig(config); err != nil {
		t.Fatalf("offline settings: %v", err)
	}
	if _, err = service.NewAttempt(context.Background(), challenge.ID, nil, ""); err == nil {
		t.Fatal("created attempt before recovery")
	}
	if _, err = service.Prepare(context.Background(), attempt.ID); err == nil {
		t.Fatal("prepared before recovery")
	}
	if _, err = service.Check(context.Background(), attempt.ID); err == nil {
		t.Fatal("checked before recovery")
	}
	if err = service.Abandon(attempt.ID); err == nil {
		t.Fatal("abandon hid pending container cleanup")
	}
	sessions, err := service.Store.Sessions(attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].EndedAt != nil {
		t.Fatal("interrupted session recorded before cleanup succeeded")
	}
	attempts, err := service.Store.Attempts()
	if err != nil || len(attempts) != 1 {
		t.Fatalf("offline actions created attempt: %v %v", attempts, err)
	}
	checks, err := service.Store.Checks(attempt.ID)
	if err != nil || len(checks) != 0 {
		t.Fatalf("recovery failure fabricated a check: %v %v", checks, err)
	}

	// A tiny daemon stand-in proves successful cleanup must precede DB recovery.
	// It only accepts the owned-container lookup, so accidental execution fails.
	bin := t.TempDir()
	probe := filepath.Join(bin, "calls")
	dockerScript := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GOLF_RECOVERY_PROBE\"\n[ \"$1\" = ps ] || exit 1\n"
	if err = os.WriteFile(filepath.Join(bin, "docker"), []byte(dockerScript), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOLF_RECOVERY_PROBE", probe)
	t.Setenv("PATH", bin)
	if err = service.retryRecovery(context.Background()); err != nil {
		t.Fatal(err)
	}
	if service.RecoveryWarning() != "" {
		t.Fatal("warning survived successful recovery")
	}
	sessions, err = service.Store.Sessions(attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sessions[0].EndedAt == nil || sessions[0].DurationMS != nil || sessions[0].Outcome != "interrupted" {
		t.Fatalf("incorrect recovered session: %+v", sessions[0])
	}
	calls, err := os.ReadFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	if string(calls) != "ps --all --quiet --filter label=io.driving-range.attempt=offline-recovery\n" {
		t.Fatalf("unexpected recovery actions: %q", calls)
	}
	t.Setenv("PATH", t.TempDir())
	if err = service.retryRecovery(context.Background()); err != nil {
		t.Fatal("completed recovery retried unavailable Docker", err)
	}
}

func TestFreshOfflineOpenDoesNotNeedRecovery(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	service, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if service.RecoveryWarning() != "" {
		t.Fatal("fresh local state requires Docker")
	}
}
