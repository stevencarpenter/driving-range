package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const upstreamPackage = "github.com/stevencarpenter/driving-range/cmd/golf@main"

func updateExecutable(ctx context.Context, executable string, out io.Writer) error {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("update requires Go 1.27 or newer on PATH: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve installed golf: %w", err)
	}
	installed, err := os.Stat(executable)
	if err != nil {
		return err
	}
	if !installed.Mode().IsRegular() {
		return fmt.Errorf("installed golf is not a regular file: %s", executable)
	}
	// Stage on the same filesystem so replacement is an atomic rename.
	stage, err := os.MkdirTemp(filepath.Dir(executable), ".golf-update-")
	if err != nil {
		return fmt.Errorf("cannot stage update beside %s; install golf in a writable directory: %w", executable, err)
	}
	defer os.RemoveAll(stage)
	fmt.Fprintln(out, "Fetching and building the latest Driving Range from upstream main...")
	cmd := exec.CommandContext(ctx, goTool, "install", "-trimpath", upstreamPackage)
	cmd.Dir = stage
	cmd.Env = append(cmd.Environ(), "GOBIN="+stage, "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build upstream update (installed golf is unchanged): %w", err)
	}
	replacement := filepath.Join(stage, "golf")
	built, err := os.Lstat(replacement)
	if err != nil {
		return fmt.Errorf("read built golf (installed golf is unchanged): %w", err)
	}
	if !built.Mode().IsRegular() || built.Size() == 0 || built.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("upstream build did not produce an executable (installed golf is unchanged)")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Chmod(replacement, installed.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Rename(replacement, executable); err != nil {
		return fmt.Errorf("replace installed golf (installed golf is unchanged): %w", err)
	}
	fmt.Fprintf(out, "Updated %s. Run golf version to inspect the installed build.\n", executable)
	return nil
}
