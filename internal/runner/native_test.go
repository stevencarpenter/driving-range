package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

func nativeTestWorkspace(t *testing.T, r *Runner, a model.Attempt) string {
	t.Helper()
	dir := filepath.Join(r.nativeRoot, a.Workspace)
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "owner"), []byte(a.ID), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNativeCommandInheritsUserConfiguration(t *testing.T) {
	r := NewNative("unused", t.TempDir())
	a := model.Attempt{ID: "native", Workspace: "golf-native"}
	dir := nativeTestWorkspace(t, r, a)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "nvim"), []byte("#!/bin/sh\nprintf '%s' \"$NVIM_APPNAME:$XDG_CONFIG_HOME:$HOME\" > inherited\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("NVIM_APPNAME", "personal-vim")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, err := r.Prepare(context.Background(), model.Challenge{Editor: "nvim", Entrypoint: "answer.txt"}, a)
	if err != nil {
		t.Fatal(err)
	}
	cmd := s.Command()
	if !slices.Equal(cmd.Args, []string{"nvim", "--", "answer.txt"}) || cmd.Dir != filepath.Join(dir, "files") {
		t.Fatalf("native command: %v in %q", cmd.Args, cmd.Dir)
	}
	result := s.Finish(cmd.Run())
	if result.ExitCode != 7 || result.Error != "" || result.Interrupted {
		t.Fatalf("native child status: %+v", result)
	}
	got, err := os.ReadFile(filepath.Join(cmd.Dir, "inherited"))
	want := "personal-vim:" + os.Getenv("XDG_CONFIG_HOME") + ":" + os.Getenv("HOME")
	if err != nil || string(got) != want {
		t.Fatalf("configuration environment: %q, %v", got, err)
	}
}

func TestNativeShellPrefersUserShellWhenIncidental(t *testing.T) {
	r := NewNative("unused", t.TempDir())
	a := model.Attempt{ID: "native", Workspace: "golf-native"}
	nativeTestWorkspace(t, r, a)
	bin := t.TempDir()
	fake := filepath.Join(bin, "mysh")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SHELL", fake)

	incidental := model.Challenge{Editor: "bash", Tools: []string{"python3"}, Entrypoint: "solution.py"}
	s, err := r.Prepare(context.Background(), incidental, a)
	if err != nil {
		t.Fatal(err)
	}
	cmd := s.Command()
	s.Finish(context.Canceled)
	if cmd.Args[len(cmd.Args)-2] != fake || cmd.Args[len(cmd.Args)-1] != "-i" {
		t.Fatalf("incidental shell: %v, want %q -i", cmd.Args, fake)
	}

	pinned := model.Challenge{Editor: "bash", Tools: []string{"bash"}, Entrypoint: "solution.sh"}
	s, err = r.Prepare(context.Background(), pinned, a)
	if err != nil {
		t.Fatal(err)
	}
	cmd = s.Command()
	s.Finish(context.Canceled)
	if cmd.Args[len(cmd.Args)-2] != "bash" || cmd.Args[len(cmd.Args)-1] != "-i" {
		t.Fatalf("pinned shell: %v, want bash -i", cmd.Args)
	}

	t.Setenv("SHELL", filepath.Join(bin, "missing-shell"))
	s, err = r.Prepare(context.Background(), incidental, a)
	if err != nil {
		t.Fatal(err)
	}
	cmd = s.Command()
	s.Finish(context.Canceled)
	if cmd.Args[len(cmd.Args)-2] != "bash" || cmd.Args[len(cmd.Args)-1] != "-i" {
		t.Fatalf("fallback shell: %v, want bash -i", cmd.Args)
	}

	// An unset editor defaults to bash, so SHELL may still replace it when the
	// exercise does not teach the shell.
	t.Setenv("SHELL", fake)
	defaulted := model.Challenge{Tools: []string{"python3"}, Entrypoint: "solution.py"}
	s, err = r.Prepare(context.Background(), defaulted, a)
	if err != nil {
		t.Fatal(err)
	}
	cmd = s.Command()
	s.Finish(context.Canceled)
	if cmd.Args[len(cmd.Args)-2] != fake || cmd.Args[len(cmd.Args)-1] != "-i" {
		t.Fatalf("defaulted editor shell: %v, want %q -i", cmd.Args, fake)
	}
}

func TestNativeSnapshotAndOwnershipBoundary(t *testing.T) {
	r := NewNative("unused", t.TempDir())
	a := model.Attempt{ID: "native", Workspace: "golf-native"}
	dir := nativeTestWorkspace(t, r, a)
	file := filepath.Join(dir, "files", "answer")
	if err := os.WriteFile(file, []byte("saved\n"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := r.snapshot(context.Background(), a)
	if err != nil || files["answer"] != "saved\n" {
		t.Fatalf("snapshot: %v, %v", files, err)
	}
	for _, hard := range []bool{false, true} {
		link := filepath.Join(dir, "files", "link")
		if hard {
			err = os.Link(file, link)
		} else {
			err = os.Symlink(file, link)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.snapshot(context.Background(), a); err == nil {
			t.Fatal("link accepted")
		}
		if err = os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(file, []byte(strings.Repeat("x", maxOutput+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = r.snapshot(context.Background(), a); err == nil {
		t.Fatal("oversized native file accepted")
	}
	wrong := a
	wrong.ID = "someone-else"
	if err = r.cleanupNative(wrong); err == nil {
		t.Fatal("deleted another owner's workspace")
	}
	if _, err = os.Stat(file); err != nil {
		t.Fatal("ownership failure deleted files", err)
	}
	if err = r.cleanupNative(a); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("owned workspace survived cleanup", err)
	}
}

func TestIntegrationNativeWorkspaceResumeAndValidation(t *testing.T) {
	isolated, a := integrationRunner(t)
	r := NewNative(isolated.image, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := model.Challenge{Editor: "bash", Brief: "native brief", Validator: model.Validator{Kind: "tree", OutputPolicy: "exact"}, Fixtures: []model.Fixture{{Files: map[string]string{"answer": "before\n", "delete-me": "remove\n"}, ExpectedFiles: map[string]string{"answer": "after\n"}}}}
	s, err := r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	cmd := s.Command()
	for _, action := range []string{"check", "hint"} {
		for _, enabled := range []bool{false, true} {
			shim := exec.Command(filepath.Join(r.nativeRoot, a.Workspace, "bin", "golf-"+action))
			shim.Env = append(os.Environ(), "GOLF_WORKBENCH="+workbenchEnv(enabled))
			out, err := shim.CombinedOutput()
			if enabled {
				if err != nil || string(out) != "\x1b]9270;golf="+action+"\a" {
					t.Fatalf("%s trigger: %q, %v", action, out, err)
				}
			} else if err == nil || !strings.Contains(string(out), "needs the embedded workbench") || strings.Contains(string(out), "\x1b") {
				t.Fatalf("%s classic mode: %q, %v", action, out, err)
			}
		}
	}
	cmd.Stdin = strings.NewReader("golf-brief; printf 'after\\n' > answer; rm delete-me; exit\n")
	out, runErr := cmd.CombinedOutput()
	result := s.Finish(runErr)
	if result.Error != "" || result.ExitCode != 0 || !strings.Contains(string(out), c.Brief) {
		t.Fatalf("native shell: %+v, %s", result, out)
	}
	// Recreate the runner as after a crash. Local edits, including deletion, win.
	r = NewNative(isolated.image, r.nativeRoot)
	s, err = r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	s.Finish(context.Canceled)
	checked, err := r.Check(ctx, c, a)
	if err != nil || checked.Outcome != "pass" {
		t.Fatalf("native validation: %+v, %v", checked, err)
	}
	if err = r.Cleanup(ctx, a); err != nil {
		t.Fatal(err)
	}
}
