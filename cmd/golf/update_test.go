package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpdateExecutable(t *testing.T) {
	for _, tc := range []struct {
		name, build string
		wantError   bool
	}{
		{"success", "printf 'new golf' >\"$GOBIN/golf\"; chmod 755 \"$GOBIN/golf\"", false},
		{"build failure", "printf 'partial' >\"$GOBIN/golf\"; exit 1", true},
		{"missing binary", "exit 0", true},
		{"nonexecutable binary", "printf 'new golf' >\"$GOBIN/golf\"", true},
		{"symlink binary", "ln -s \"$GOLF_UPDATE_OLD\" \"$GOBIN/golf\"", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			installed := filepath.Join(dir, "golf")
			if err := os.WriteFile(installed, []byte("old golf"), 0755); err != nil {
				t.Fatal(err)
			}
			oldLink := filepath.Join(dir, "running-golf")
			if err := os.Link(installed, oldLink); err != nil {
				t.Fatal(err)
			}
			tools := t.TempDir()
			record := filepath.Join(dir, "build-arguments")
			script := "#!/bin/sh\nset -eu\nprintf '%s\\n' \"$@\" \"$GOOS\" \"$GOARCH\" \"$CGO_ENABLED\" \"$GOFLAGS\" \"$GOWORK\" >\"$GOLF_UPDATE_RECORD\"\n" + tc.build + "\n"
			if err := os.WriteFile(filepath.Join(tools, "go"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GOLF_UPDATE_RECORD", record)
			t.Setenv("GOLF_UPDATE_OLD", installed)
			t.Setenv("GOBIN", t.TempDir())
			t.Setenv("GOOS", "windows")
			t.Setenv("GOARCH", "386")
			t.Setenv("GOFLAGS", "-race")
			t.Setenv("GOWORK", "invalid")
			var out bytes.Buffer
			err := updateExecutable(context.Background(), installed, &out)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error = %v", err, tc.wantError)
			}
			got, err := os.ReadFile(installed)
			if err != nil {
				t.Fatal(err)
			}
			want := "new golf"
			if tc.wantError {
				want = "old golf"
			}
			if string(got) != want {
				t.Fatalf("installed = %q, want %q", got, want)
			}
			old, err := os.ReadFile(oldLink)
			if err != nil || string(old) != "old golf" {
				t.Fatalf("running binary changed: %q, %v", old, err)
			}
			args, err := os.ReadFile(record)
			wantArgs := strings.Join([]string{"install", "-trimpath", "github.com/stevencarpenter/driving-range/cmd/golf@latest", runtime.GOOS, runtime.GOARCH, "0", "", "off", ""}, "\n")
			if err != nil || string(args) != wantArgs {
				t.Fatalf("build arguments = %q, want %q, error = %v", args, wantArgs, err)
			}
			staged, err := filepath.Glob(filepath.Join(dir, ".golf-update-*"))
			if err != nil || len(staged) != 0 {
				t.Fatalf("staged updates remain: %v, %v", staged, err)
			}
		})
	}
}

func TestUpdateRejectsMissingGoWithoutOpeningHistory(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	state := filepath.Join(t.TempDir(), "no-state")
	var out bytes.Buffer
	if err := run([]string{"--state-dir", state, "update"}, &out); err == nil || !strings.Contains(err.Error(), "requires Go") {
		t.Fatalf("missing Go error = %v", err)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("update created state: %v", err)
	}
	if err := run([]string{"update", "unexpected"}, &out); err == nil || err.Error() != "usage: golf update" {
		t.Fatalf("argument error = %v", err)
	}
}

func TestUpdateFollowsInstalledSymlinkAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installed-golf")
	link := filepath.Join(dir, "golf")
	if err := os.WriteFile(target, []byte("old golf"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nprintf 'new golf' >\"$GOBIN/golf\"\nchmod 755 \"$GOBIN/golf\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := updateExecutable(context.Background(), link, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(link)
	if err != nil || string(got) != "new golf" {
		t.Fatalf("updated target = %q, %v", got, err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("installation symlink replaced: %v, %v", info, err)
	}
	info, err = os.Stat(target)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("permissions changed: %v, %v", info, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := updateExecutable(ctx, link, &bytes.Buffer{}); err == nil {
		t.Fatal("canceled update succeeded")
	}
	got, err = os.ReadFile(link)
	if err != nil || string(got) != "new golf" {
		t.Fatalf("canceled update changed binary: %q, %v", got, err)
	}
}
