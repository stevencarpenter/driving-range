// Package runner launches native practice and isolates setup and validation in Docker.
package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stevencarpenter/driving-range/internal/model"
	runtimefiles "github.com/stevencarpenter/driving-range/runtime"
)

const DefaultImage = "driving-range-runtime:1"
const ownerLabel = "io.driving-range.attempt"
const maxOutput = 2 << 20
const maxSnapshot = 16 << 20
const maxFiles = 4096

var volumePattern = regexp.MustCompile(`^(golf-|driving-range-)[a-zA-Z0-9][a-zA-Z0-9_.-]{0,120}$`)

type Runner struct {
	image      string
	nativeRoot string
}
type Report struct {
	Available bool   `json:"available"`
	ImageID   string `json:"image_id"`
	Message   string `json:"message"`
}
type Session struct {
	runner         *Runner
	container      string
	challenge      model.Challenge
	ctx            context.Context
	cancel         context.CancelFunc
	stopCleanup    func() bool
	started        time.Time
	once           sync.Once
	result         model.SessionResult
	statusFile     string
	commandStarted bool
	cleanupOnce    sync.Once
	cleanupErr     error
	directory      string
	command        *exec.Cmd
}

func New(image string) *Runner {
	if image == "" {
		image = DefaultImage
	}
	return &Runner{image: image}
}

func (r *Runner) Doctor(ctx context.Context) (Report, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := exec.LookPath("docker"); err != nil {
		return Report{Message: "Docker is missing. Install a Docker-compatible Linux runtime."}, nil
	}
	if _, _, err := docker(ctx, nil, "version", "--format", "{{.Server.Version}}"); err != nil {
		return Report{Message: "Docker daemon is unavailable: " + err.Error()}, nil
	}
	out, _, err := docker(ctx, nil, "image", "inspect", "--format", "{{.Id}}", r.image)
	if err != nil {
		return Report{Message: "Runtime image is missing. Run golf setup."}, nil
	}
	return Report{Available: true, ImageID: strings.TrimSpace(string(out)), Message: "Isolated Linux runtime is ready."}, nil
}

// BuildImage uses an embedded context; setup also works from an installed binary.
func (r *Runner) BuildImage(ctx context.Context, output io.Writer) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range []string{"Dockerfile", "golf-shell", "golf-brief"} {
		data, err := runtimefiles.Files.ReadFile(name)
		if err != nil {
			return err
		}
		if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
			return err
		}
		if _, err = tw.Write(data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", "build", "--tag", r.image, "-")
	cmd.Stdin = &buf
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

func identity(a model.Attempt) error {
	if a.ID == "" || len(a.ID) > 128 || strings.ContainsAny(a.ID, "\x00\r\n") || !volumePattern.MatchString(a.Workspace) {
		return errors.New("invalid attempt or owned workspace identity")
	}
	return nil
}

func (r *Runner) ensureVolume(ctx context.Context, a model.Attempt) (bool, error) {
	if err := identity(a); err != nil {
		return false, err
	}
	out, stderr, err := docker(ctx, nil, "volume", "inspect", a.Workspace)
	if err == nil {
		var volumes []struct{ Labels map[string]string }
		if json.Unmarshal(out, &volumes) != nil || len(volumes) != 1 || volumes[0].Labels[ownerLabel] != a.ID {
			return false, errors.New("workspace volume belongs to another owner")
		}
		return false, nil
	}
	if !strings.Contains(strings.ToLower(stderr), "no such volume") {
		return false, err
	}
	_, _, err = docker(ctx, nil, "volume", "create", "--label", ownerLabel+"="+a.ID, a.Workspace)
	return true, err
}

func (r *Runner) create(ctx context.Context, a model.Attempt, readonly bool) (string, error) {
	if err := identity(a); err != nil {
		return "", err
	}
	image := a.EnvironmentID
	if image == "" {
		image = r.image
	}
	name := "golf-session-" + randomID()
	mount := "type=volume,source=" + a.Workspace + ",target=/workspace"
	if readonly {
		mount += ",readonly"
	}
	args := []string{"create", "--name", name, "--label", ownerLabel + "=" + a.ID, "--pull=never", "--network=none", "--user=1000:1000", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--read-only", "--cpus=1", "--memory=512m", "--memory-swap=512m", "--pids-limit=128", "--ulimit=nofile=256:256", "--ulimit=fsize=16777216:16777216", "--tmpfs=/tmp:rw,nosuid,nodev,size=64m", "--log-driver=none", "--mount", mount, "--workdir=/workspace"}
	// Docker otherwise injects proxy URLs, which may include host credentials.
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "FTP_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "ftp_proxy", "all_proxy", "no_proxy"} {
		args = append(args, "--env", key+"=")
	}
	args = append(args, image, "/bin/sleep", "3600")
	if _, _, err := docker(ctx, nil, args...); err != nil {
		r.removeContainer(name)
		return "", err
	}
	if _, _, err := docker(ctx, nil, "start", name); err != nil {
		r.removeContainer(name)
		return "", err
	}
	return name, nil
}

func (r *Runner) removeContainer(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, stderr, err := docker(ctx, nil, "rm", "--force", name)
	if err != nil && strings.Contains(strings.ToLower(stderr), "no such container:") {
		return nil
	}
	return err
}

// CleanupContainers only stops containers belonging to the supplied local attempts.
func (r *Runner) CleanupContainers(ctx context.Context, attempts []model.Attempt) error {
	for _, a := range attempts {
		if err := identity(a); err != nil {
			return err
		}
		out, _, err := docker(ctx, nil, "ps", "--all", "--quiet", "--filter", "label="+ownerLabel+"="+a.ID)
		if err != nil {
			return err
		}
		for _, id := range strings.Fields(string(out)) {
			if err = r.removeContainer(id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Runner) Prepare(ctx context.Context, c model.Challenge, a model.Attempt) (*Session, error) {
	if r.nativeRoot != "" {
		return r.prepareNative(ctx, c, a)
	}
	if len(c.Fixtures) == 0 {
		return nil, errors.New("exercise has no fixtures")
	}
	if err := r.CleanupContainers(ctx, []model.Attempt{a}); err != nil {
		return nil, err
	}
	fresh, err := r.ensureVolume(ctx, a)
	if err != nil {
		return nil, err
	}
	container := ""
	ok := false
	defer func() {
		if !ok {
			if container != "" {
				r.removeContainer(container)
			}
			if fresh {
				cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				r.Cleanup(cleanup, a)
			}
		}
	}()
	container, err = r.create(ctx, a, false)
	if err != nil {
		return nil, err
	}
	if fresh {
		if err = r.populate(ctx, container, c.Fixtures[0].Files); err != nil {
			return nil, err
		}
		if err = r.setup(ctx, container, c.Fixtures[0].Setup); err != nil {
			return nil, err
		}
	}
	if _, _, err = docker(ctx, strings.NewReader(c.Brief), "exec", "--interactive", container, "/bin/bash", "--noprofile", "--norc", "-c", "mkdir -p \"$HOME\"; cat > /tmp/golf-brief.txt"); err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithTimeout(ctx, time.Hour)
	s := &Session{runner: r, container: container, challenge: c, ctx: lifetime, cancel: cancel, started: time.Now(), statusFile: "/tmp/golf-status-" + randomID()}
	s.stopCleanup = context.AfterFunc(lifetime, func() { s.cleanupContainer() })
	ok = true
	return s, nil
}

func (s *Session) Command() *exec.Cmd {
	if s.directory != "" {
		s.commandStarted = true
		s.started = time.Now()
		return s.command
	}
	var argv []string
	switch s.challenge.Editor {
	case "nvim", "vim":
		argv = []string{"nvim", "--clean", "-i", "NONE", "--cmd", "set nomodeline", "--", s.challenge.Entrypoint}
	case "zsh":
		argv = []string{"zsh", "-f"}
	default:
		argv = []string{"/usr/local/bin/golf-shell"}
	}
	s.commandStarted = true
	s.started = time.Now()
	return exec.CommandContext(s.ctx, "docker", completionArgs(s.container, s.statusFile, true, argv)...)
}

func (s *Session) Finish(childErr error) model.SessionResult {
	s.once.Do(func() {
		if s.stopCleanup != nil {
			s.stopCleanup()
		}
		interrupted := s.ctx.Err() != nil || errors.Is(childErr, context.Canceled) || errors.Is(childErr, context.DeadlineExceeded)
		s.cancel()
		code := exitCode(childErr)
		if s.directory == "" && s.commandStarted && !interrupted && childErr == nil {
			var err error
			code, err = containerStatus(s.container, s.statusFile)
			if err != nil {
				childErr = err
			}
		}
		var exitErr *exec.ExitError
		signaled := errors.As(childErr, &exitErr) && exitErr.ExitCode() < 0
		result := model.SessionResult{DurationMS: time.Since(s.started).Milliseconds(), DurationKnown: true, ExitCode: code, Interrupted: interrupted || code == 130 || code == 137 || signaled}
		if childErr != nil && !result.Interrupted && !(s.directory != "" && errors.As(childErr, &exitErr)) {
			result.Error = childErr.Error()
		}
		if err := s.cleanupContainer(); err != nil {
			result.Error = "container cleanup failed: " + err.Error()
			result.Interrupted = true
		}
		s.result = result
	})
	return s.result
}

func (s *Session) cleanupContainer() error {
	if s.directory != "" {
		return nil
	}
	s.cleanupOnce.Do(func() { s.cleanupErr = s.runner.removeContainer(s.container) })
	return s.cleanupErr
}

// Execute runs a submitted Bash script without allocating a terminal.
func (r *Runner) Execute(ctx context.Context, c model.Challenge, a model.Attempt, script string) (model.SessionResult, error) {
	if len(script) > maxOutput {
		return model.SessionResult{}, errors.New("submission exceeds 2 MiB")
	}
	s, err := New(r.image).Prepare(ctx, c, a)
	if err != nil {
		return model.SessionResult{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s.commandStarted = true
	s.started = time.Now()
	_, _, runErr := docker(bounded, strings.NewReader(script), completionArgs(s.container, s.statusFile, false, []string{"/bin/bash", "--noprofile", "--norc", "-s"})...)
	result := s.Finish(runErr)
	if runErr == nil && result.Error != "" {
		runErr = errors.New(result.Error)
	}
	if runErr == nil && result.ExitCode != 0 {
		runErr = fmt.Errorf("submission exited with status %d", result.ExitCode)
	}
	return result, runErr
}

// A successful Docker CLI exit is insufficient evidence that a candidate ran.
// This trusted wrapper separates the child status from Docker transport errors.
func completionArgs(container, status string, tty bool, argv []string) []string {
	args := []string{"exec", "--interactive"}
	if tty {
		args = append(args, "--tty")
	}
	args = append(args, container, "/bin/bash", "--noprofile", "--norc", "-c", `marker=$1; shift; command -v -- "$1" >/dev/null || exit 126; "$@"; result=$?; printf '%s\n' "$result" > "$marker"`, "golf", status)
	return append(args, argv...)
}

func containerStatus(container, status string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _, err := docker(ctx, nil, "exec", container, "cat", status)
	if err != nil {
		return -1, fmt.Errorf("execution completion unavailable: %w", err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || code < 0 || code > 255 {
		return -1, errors.New("invalid execution completion status")
	}
	return code, nil
}

func (r *Runner) populate(ctx context.Context, container string, files map[string]string) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	names := make([]string, 0, len(files))
	total := 0
	for name, data := range files {
		if !safePath(name) {
			return fmt.Errorf("unsafe fixture path %q", name)
		}
		total += len(data)
		if len(data) > maxOutput || total > maxSnapshot || len(files) > maxFiles {
			return errors.New("fixture size limit exceeded")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
			return err
		}
		if _, err := io.WriteString(tw, data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	_, _, err := docker(ctx, &buf, "exec", "--interactive", container, "tar", "--extract", "--file=-", "--no-same-owner", "--no-same-permissions", "--directory=/workspace")
	return err
}

func (r *Runner) setup(ctx context.Context, container, script string) error {
	if script == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, _, err := docker(ctx, strings.NewReader(script), "exec", "--interactive", container, "/bin/bash", "--noprofile", "--norc", "-e", "-s")
	return err
}

func (r *Runner) Cleanup(ctx context.Context, a model.Attempt) error {
	if err := r.CleanupContainers(ctx, []model.Attempt{a}); err != nil {
		return err
	}
	fresh, err := r.ensureVolume(ctx, a)
	if err != nil {
		return err
	}
	_ = fresh
	if err = r.cleanupNative(a); err != nil {
		return err
	}
	_, _, err = docker(ctx, nil, "volume", "rm", a.Workspace)
	return err
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	overflow bool
}

func (b *limitedBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *limitedBuffer) String() string { return b.buffer.String() }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := maxOutput - b.buffer.Len()
	if left < 0 {
		left = 0
	}
	if len(p) > left {
		b.overflow = true
		p = p[:left]
	}
	b.buffer.Write(p)
	// Returning a write error terminates the CLI pipe; callers remove its container.
	if b.overflow {
		return n, errors.New("output exceeds 2 MiB")
	}
	return n, nil
}
func docker(ctx context.Context, input io.Reader, args ...string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = input
	var out, stderr limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if out.overflow || stderr.overflow {
		err = errors.New("container output exceeds 2 MiB")
	}
	if err != nil {
		message := stderr.String()
		if len(message) > 4096 {
			message = message[:4096] + "..."
		}
		return out.Bytes(), stderr.String(), fmt.Errorf("docker %s: %w: %s", args[0], err, SafeText(message))
	}
	return out.Bytes(), stderr.String(), nil
}

// SafeText escapes terminal controls before any child output enters the outer UI.
func SafeText(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c == '\n' || c == '\t' || (c >= 32 && c != 127 && !(c >= 128 && c <= 159)) {
			b.WriteRune(c)
		} else {
			fmt.Fprintf(&b, "\\u%04x", c)
		}
	}
	return b.String()
}

func safePath(name string) bool {
	return name != "" && name != "." && !strings.ContainsAny(name, "\\\x00") && !strings.HasPrefix(name, "/") && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../")
}

// readSnapshot never extracts container paths onto the host filesystem.
func readSnapshot(input io.Reader) (map[string]string, error) {
	tr := tar.NewReader(io.LimitReader(input, 32<<20))
	files := map[string]string{}
	seen := map[string]bool{}
	total := int64(0)
	count := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(h.Name, "./")
		if h.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
			if name == "." || name == "" {
				continue
			}
		}
		if !safePath(name) || seen[name] {
			return nil, fmt.Errorf("unsafe or duplicate artifact path %q", h.Name)
		}
		seen[name] = true
		count++
		if count > maxFiles {
			return nil, errors.New("workspace exceeds 4096 entries")
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("unsupported artifact type at %q: links and special files are forbidden", name)
		}
		total += h.Size
		if h.Size < 0 || h.Size > maxOutput || total > maxSnapshot {
			return nil, errors.New("workspace exceeds snapshot byte limits")
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[name] = string(data)
	}
}

func (r *Runner) snapshot(ctx context.Context, a model.Attempt) (map[string]string, error) {
	if dir, err := r.nativeDirectory(a); err != nil {
		return nil, err
	} else if dir != "" {
		return nativeSnapshot(dir)
	}
	container, err := r.create(ctx, a, true)
	if err != nil {
		return nil, err
	}
	defer r.removeContainer(container)
	if _, _, err = docker(ctx, nil, "stop", "--time=0", container); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "docker", "cp", container+":/workspace/.", "-")
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	files, readErr := readSnapshot(output)
	if readErr != nil {
		cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil {
		return nil, fmt.Errorf("workspace snapshot failed: %w: %s", waitErr, SafeText(stderr.String()))
	}
	return files, nil
}
