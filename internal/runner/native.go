package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
)

// triggerShims are the helper commands placed on the exercise PATH. golf-brief
// prints the brief; the others ask the workbench to act without the operator
// leaving the exercise or golf intercepting a key.
func triggerShims() map[string]string {
	shim := func(action, desc string) string {
		return "#!/bin/sh\n" +
			"if [ -z \"$GOLF_WORKBENCH\" ]; then\n" +
			"  echo \"golf-" + action + " needs the embedded workbench. Exit the exercise to " + desc + ".\" >&2\n" +
			"  exit 1\n" +
			"fi\n" +
			"printf '\\033]9270;golf=" + action + "\\007'\n"
	}
	return map[string]string{
		"golf-brief": "#!/bin/sh\ncat -- \"$GOLF_BRIEF_FILE\"\n",
		"golf-check": shim("check", "check your work"),
		"golf-hint":  shim("hint", "get the next hint"),
	}
}

// workbenchEnv reports the value the shims test for. It is empty in classic
// mode, where no emulator is listening for the trigger sequence.
func workbenchEnv(workbench bool) string {
	if workbench {
		return "1"
	}
	return ""
}

// NewNative uses the player's installed tools for practice and Docker for checks.
func NewNative(image, workspaceRoot string) *Runner {
	r := New(image)
	r.nativeRoot = workspaceRoot
	return r
}

func (r *Runner) nativeDirectory(a model.Attempt) (string, error) {
	if r.nativeRoot == "" {
		return "", nil
	}
	if err := identity(a); err != nil {
		return "", err
	}
	dir := filepath.Join(r.nativeRoot, a.Workspace)
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("native workspace must be an owned directory, not a link")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	owner, err := root.ReadFile("owner")
	if err != nil || string(owner) != a.ID {
		return "", errors.New("native workspace belongs to another owner")
	}
	info, err = root.Lstat("files")
	if err != nil || !info.IsDir() {
		return "", errors.New("native workspace files must be a directory, not a link")
	}
	return dir, nil
}

func (r *Runner) prepareNative(ctx context.Context, c model.Challenge, a model.Attempt) (*Session, error) {
	argv := []string{c.Editor, "-i"}
	if c.Editor == "nvim" || c.Editor == "vim" {
		argv = []string{"nvim", "--", c.Entrypoint}
	} else if c.Editor == "" {
		argv[0] = "bash"
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return nil, fmt.Errorf("install %s on your host to practice: %w", argv[0], err)
	}
	if c.Editor != "nvim" && c.Editor != "vim" {
		argv = append([]string{"/bin/sh", "-c", "printf 'Driving Range. Run golf-brief for the exercise. Exit to check your work.\\n'; exec \"$@\"", "golf"}, argv...)
	}
	dir, err := r.nativeDirectory(a)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		// Trusted setup scripts run only in Docker. Export a validated snapshot.
		isolated := New(r.image)
		s, err := isolated.Prepare(ctx, c, a)
		if err != nil {
			return nil, err
		}
		if result := s.Finish(nil); result.Error != "" {
			return nil, errors.New(result.Error)
		}
		files, err := isolated.snapshot(ctx, a)
		if err != nil {
			return nil, err
		}
		if err = os.MkdirAll(r.nativeRoot, 0700); err != nil {
			return nil, err
		}
		stage, err := os.MkdirTemp(r.nativeRoot, ".prepare-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(stage)
		if err = os.Mkdir(filepath.Join(stage, "files"), 0700); err != nil {
			return nil, err
		}
		for name, data := range files {
			target := filepath.Join(stage, "files", filepath.FromSlash(name))
			if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return nil, err
			}
			file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return nil, err
			}
			_, writeErr := io.WriteString(file, data)
			if err = errors.Join(writeErr, file.Close()); err != nil {
				return nil, err
			}
		}
		if err = os.WriteFile(filepath.Join(stage, "owner"), []byte(a.ID), 0600); err != nil {
			return nil, err
		}
		if err = os.WriteFile(filepath.Join(stage, "brief.txt"), []byte(c.Brief), 0600); err != nil {
			return nil, err
		}
		if err = os.Mkdir(filepath.Join(stage, "bin"), 0700); err != nil {
			return nil, err
		}
		for name, body := range triggerShims() {
			if err = os.WriteFile(filepath.Join(stage, "bin", name), []byte(body), 0700); err != nil {
				return nil, err
			}
		}
		dir = filepath.Join(r.nativeRoot, a.Workspace)
		if err = os.Rename(stage, dir); err != nil {
			return nil, err
		}
	}
	lifetime, cancel := context.WithTimeout(ctx, time.Hour)
	cmd := exec.CommandContext(lifetime, argv[0], argv[1:]...)
	cmd.Dir = filepath.Join(dir, "files")
	cmd.Env = append(cmd.Environ(), "PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"), "GOLF_BRIEF_FILE="+filepath.Join(dir, "brief.txt"), "GOLF_WORKBENCH="+workbenchEnv(r.workbench))
	s := &Session{runner: r, challenge: c, ctx: lifetime, cancel: cancel, started: time.Now(), directory: dir, command: cmd}
	return s, nil
}

// nativeSnapshot confines reads to the owned tree and applies the Docker snapshot limits.
func nativeSnapshot(dir string) (map[string]string, error) {
	root, err := os.OpenRoot(filepath.Join(dir, "files"))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	files := map[string]string{}
	count, total := 0, 0
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || name == "." {
			return walkErr
		}
		count++
		if !safePath(name) || count > maxFiles {
			return errors.New("native workspace has unsafe paths or exceeds 4096 entries")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Sys().(*syscall.Stat_t).Nlink != 1 {
			return fmt.Errorf("unsupported artifact type at %q: links and special files are forbidden", name)
		}
		if info.Size() > maxOutput {
			return errors.New("native workspace file exceeds 2 MiB")
		}
		file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		opened, err := file.Stat()
		if err != nil || !os.SameFile(info, opened) {
			file.Close()
			return fmt.Errorf("native workspace changed while reading %q", name)
		}
		data, err := io.ReadAll(io.LimitReader(file, maxOutput+1))
		file.Close()
		if err != nil {
			return err
		}
		total += len(data)
		if len(data) > maxOutput || total > maxSnapshot {
			return errors.New("native workspace exceeds snapshot byte limits")
		}
		files[name] = string(data)
		return nil
	})
	return files, err
}

func (r *Runner) cleanupNative(a model.Attempt) error {
	dir, err := r.nativeDirectory(a)
	if err != nil || dir == "" {
		return err
	}
	root, err := os.OpenRoot(r.nativeRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(a.Workspace)
}
