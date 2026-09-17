package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stevencarpenter/driving-range/internal/model"
	"github.com/stevencarpenter/driving-range/internal/pane"
)

func TestSnapshotBoundary(t *testing.T) {
	cases := []struct {
		name string
		kind byte
		size int64
	}{{"../escape", tar.TypeReg, 0}, {"/absolute", tar.TypeReg, 0}, {"link", tar.TypeSymlink, 0}, {"hard", tar.TypeLink, 0}, {"pipe", tar.TypeFifo, 0}, {"big", tar.TypeReg, maxOutput + 1}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			tw := tar.NewWriter(&b)
			if err := tw.WriteHeader(&tar.Header{Name: tc.name, Typeflag: tc.kind, Size: tc.size, Linkname: "/etc/passwd", Mode: 0644}); err != nil {
				t.Fatal(err)
			}
			tw.Close()
			if _, err := readSnapshot(&b); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	tw.WriteHeader(&tar.Header{Name: "./space name.txt", Mode: 0644, Size: 3})
	tw.Write([]byte("ok\n"))
	tw.Close()
	got, err := readSnapshot(&b)
	if err != nil || got["space name.txt"] != "ok\n" {
		t.Fatalf("safe archive: %v %v", got, err)
	}
	b.Reset()
	tw = tar.NewWriter(&b)
	for i := 0; i < 2; i++ {
		tw.WriteHeader(&tar.Header{Name: "same", Mode: 0644})
	}
	tw.Close()
	if _, err := readSnapshot(&b); err == nil {
		t.Fatal("duplicate archive accepted")
	}
}

func TestOutputSemantics(t *testing.T) {
	for _, tc := range []struct {
		a, b, policy string
		want         bool
	}{{"a\n", "a", "exact", false}, {"a\na\n", "a\n", "exact", false}, {"b\na\n", "a\nb\n", "exact", false}, {"é\n", "é\n", "exact", true}} {
		got, err := outputsEqual(tc.a, tc.b, tc.policy)
		if err != nil || got != tc.want {
			t.Fatalf("%+v: %v %v", tc, got, err)
		}
	}
	if strings.ContainsRune(SafeText("\x1b]52;c;secret\a\r\u009b"), '\x1b') {
		t.Fatal("terminal escape survived")
	}
	for _, policy := range []string{"", "exact-bytes", "unordered-lines", "line-set", "unknown"} {
		if _, err := outputsEqual("", "", policy); err == nil {
			t.Fatalf("unsupported policy %q accepted", policy)
		}
	}
}

func TestBoundedOutputCannotBypassWrite(t *testing.T) {
	var output limitedBuffer
	_, err := io.Copy(&output, strings.NewReader(strings.Repeat("x", maxOutput+1)))
	if err == nil || !output.overflow || len(output.Bytes()) != maxOutput {
		t.Fatal("output cap bypassed")
	}
}

func integrationRunner(t *testing.T) (*Runner, model.Attempt) {
	t.Helper()
	if os.Getenv("GOLF_INTEGRATION") != "1" {
		t.Skip("set GOLF_INTEGRATION=1 after golf setup")
	}
	r := New(os.Getenv("GOLF_TEST_IMAGE"))
	report, err := r.Doctor(context.Background())
	if err != nil || !report.Available {
		t.Fatalf("runtime: %v %+v", err, report)
	}
	id := randomID()
	a := model.Attempt{ID: id, Workspace: "golf-test-" + id, EnvironmentID: report.ImageID}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := r.Cleanup(ctx, a); err != nil {
			t.Error(err)
		}
	})
	return r, a
}

func TestIntegrationIsolationResumeAndChecks(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := model.Challenge{Editor: "nvim", Entrypoint: "answer.txt", Validator: model.Validator{Kind: "tree", OutputPolicy: "exact"}, Fixtures: []model.Fixture{{Files: map[string]string{"answer.txt": "before\n"}, ExpectedFiles: map[string]string{"answer.txt": "after\n"}}}}
	s, err := r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := docker(ctx, nil, "inspect", s.container)
	if err != nil {
		t.Fatal(err)
	}
	var inspected []struct {
		HostConfig struct {
			NetworkMode    string
			ReadonlyRootfs bool
			Privileged     bool
			Memory         int64
			PidsLimit      int
			CapDrop        []string
			SecurityOpt    []string
		}
		Config struct{ User string }
		Mounts []struct{ Type, Destination string }
	}
	if err = json.Unmarshal(out, &inspected); err != nil {
		t.Fatal(err)
	}
	cfg := inspected[0]
	if cfg.HostConfig.NetworkMode != "none" || !cfg.HostConfig.ReadonlyRootfs || cfg.HostConfig.Privileged || cfg.Config.User != "1000:1000" || cfg.HostConfig.Memory != 512<<20 || cfg.HostConfig.PidsLimit != 128 {
		t.Fatalf("unsafe config: %+v", cfg)
	}
	for _, mount := range cfg.Mounts {
		if mount.Type == "bind" {
			t.Fatal("host bind mounted")
		}
	}
	canary := t.TempDir() + "/secret"
	if err = os.WriteFile(canary, []byte("host secret"), 0600); err != nil {
		t.Fatal(err)
	}
	script := `test "$(id -u)" = 1000
test ! -e /var/run/docker.sock
test ! -e /root/.ssh
test ! -e "$1"
test -z "${SSH_AUTH_SOCK:-}"
test -z "${AWS_SECRET_ACCESS_KEY:-}"
! touch /etc/golf-canary
! bash -c 'echo > /dev/tcp/1.1.1.1/80'
printf 'after\n' > answer.txt
sleep 300 &
`
	if _, _, err = docker(ctx, strings.NewReader(script), "exec", "-i", s.container, "bash", "-e", "-s", "--", canary); err != nil {
		t.Fatal(err)
	}
	if result := s.Finish(nil); result.Error != "" {
		t.Fatal(result.Error)
	}
	if _, _, err = docker(ctx, nil, "inspect", s.container); err == nil {
		t.Fatal("background process container survived Finish")
	}
	checked, err := r.Check(ctx, c, a)
	if err != nil || checked.Outcome != "pass" {
		t.Fatalf("check: %+v %v", checked, err)
	}
	s, err = r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	s.Finish(nil)
	checked, err = r.Check(ctx, c, a)
	if err != nil || checked.Outcome != "pass" {
		t.Fatalf("resume lost work: %+v %v", checked, err)
	}
	if data, err := os.ReadFile(canary); err != nil || string(data) != "host secret" {
		t.Fatal("host canary changed")
	}
	_, err = r.Execute(ctx, c, a, "ln -s /etc/passwd escape")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Check(ctx, c, a); err == nil {
		t.Fatal("symlink workspace accepted")
	}
}

func TestIntegrationSubmissionReplay(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := model.Challenge{SubmissionFile: "solution.sh", Validator: model.Validator{Kind: "stdout", OutputPolicy: "exact"}, Fixtures: []model.Fixture{{Name: "one", Files: map[string]string{"input.txt": "one\n", "solution.sh": ""}, ExpectedStdout: "one\n"}, {Name: "two", Files: map[string]string{"input.txt": "two\n"}, ExpectedStdout: "two\n"}}}
	if _, err := r.Execute(ctx, c, a, "printf 'cat input.txt\\n' > solution.sh"); err != nil {
		t.Fatal(err)
	}
	result, err := r.Check(ctx, c, a)
	if err != nil || result.Outcome != "pass" {
		t.Fatalf("replay: %+v %v", result, err)
	}
	if _, err = r.Execute(ctx, c, a, "printf 'cat input.txt >&2\\n' > solution.sh"); err != nil {
		t.Fatal(err)
	}
	result, err = r.Check(ctx, c, a)
	if err != nil || result.Outcome != "fail" {
		t.Fatalf("stderr should fail: %+v %v", result, err)
	}
}

func TestIntegrationTimeoutKillsContainer(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := model.Challenge{Fixtures: []model.Fixture{{Files: map[string]string{"input": "x"}}}}
	s, err := r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	bounded, stop := context.WithTimeout(ctx, 200*time.Millisecond)
	defer stop()
	_, _, err = docker(bounded, nil, "exec", s.container, "bash", "-c", "while :; do :; done")
	if err == nil {
		t.Fatal("runaway command succeeded")
	}
	s.Finish(err)
	if _, _, err = docker(ctx, nil, "inspect", s.container); err == nil {
		t.Fatal("timed out container survives")
	}
}

func TestIntegrationResourceBounds(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := model.Challenge{Fixtures: []model.Fixture{{Files: map[string]string{"input": "x"}}}}
	s, err := r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Finish(nil)
	for _, probe := range []string{
		`test "$(cat /sys/fs/cgroup/memory.max)" = 536870912; test "$(cat /sys/fs/cgroup/pids.max)" = 128; test "$(awk '/NoNewPrivs/ {print $2}' /proc/self/status)" = 1; test "$(awk '/CapEff/ {print $2}' /proc/self/status)" = 0000000000000000`,
		`! python3 -c 'open("oversized", "wb").write(b"x" * (17 << 20))'`,
		`! python3 -c 'x = bytearray(768 << 20)'`,
	} {
		if _, _, err = docker(ctx, nil, "exec", s.container, "bash", "-e", "-c", probe); err != nil {
			t.Fatal(err)
		}
	}
	bounded, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	if _, _, err = docker(bounded, nil, "exec", s.container, "python3", "-c", "import sys; sys.stdout.write('x' * (3 << 20))"); err == nil {
		t.Fatal("excessive output accepted")
	}
}

func TestIntegrationFailedPreparationCanRetry(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := model.Challenge{Fixtures: []model.Fixture{{Files: map[string]string{"input": "original"}}}}
	wrong := a
	wrong.EnvironmentID = "golf-nonexistent-" + randomID() + ":missing"
	if _, err := r.Prepare(ctx, c, wrong); err == nil {
		t.Fatal("missing image accepted")
	}
	if _, _, err := docker(ctx, nil, "volume", "inspect", a.Workspace); err == nil {
		t.Fatal("failed preparation left uninitialized volume")
	}
	s, err := r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	s.Finish(nil)
	files, err := r.snapshot(ctx, a)
	if err != nil || files["input"] != "original" {
		t.Fatalf("retry did not initialize: %v %v", files, err)
	}
}

func TestIntegrationChildExitIsNotInfrastructure(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := model.Challenge{Fixtures: []model.Fixture{{Files: map[string]string{"input": "x"}}}}
	for _, code := range []int{1, 127} {
		result, err := r.Execute(ctx, c, a, fmt.Sprintf("exit %d", code))
		if err == nil || result.ExitCode != code || result.Error != "" || result.Interrupted {
			t.Fatalf("child exit %d: %+v %v", code, result, err)
		}
	}
	out, code, err := r.fixtureCommand(ctx, a, model.Fixture{Files: map[string]string{}}, nil, []string{"bash", "-c", "exit 127"})
	if err != nil || code != 127 || out != "" {
		t.Fatalf("validator child status: %d %q %v", code, out, err)
	}
	if _, _, err = r.fixtureCommand(ctx, a, model.Fixture{Files: map[string]string{}}, nil, []string{"golf-missing-interpreter"}); err == nil {
		t.Fatal("missing interpreter classified as candidate failure")
	}
}

func TestIntegrationDockerProxyCredentialsAreNotForwarded(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	endpoint, _, err := docker(ctx, nil, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
	if err != nil {
		t.Fatal(err)
	}
	config := t.TempDir()
	if err = os.WriteFile(config+"/config.json", []byte(`{"proxies":{"default":{"httpProxy":"http://synthetic:secret@example.invalid:3128","httpsProxy":"http://synthetic:secret@example.invalid:3128","allProxy":"socks5://synthetic:secret@example.invalid:1080"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_CONFIG", config)
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_HOST", strings.TrimSpace(string(endpoint)))
	s, err := r.Prepare(ctx, model.Challenge{Fixtures: []model.Fixture{{Files: map[string]string{}}}}, a)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Finish(nil)
	out, _, err := docker(ctx, nil, "inspect", "--format", "{{json .Config.Env}}", s.container)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "synthetic") || strings.Contains(string(out), "example.invalid:3128") {
		t.Fatal("Docker client proxy credentials entered exercise")
	}
}

func TestIntegrationCancellationIsInterrupted(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := r.Prepare(ctx, model.Challenge{Fixtures: []model.Fixture{{Files: map[string]string{"input": "x"}}}}, a)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	// Model cancellation cleanup winning the race with terminal handoff completion.
	if err = s.cleanupContainer(); err != nil {
		t.Fatal(err)
	}
	result := s.Finish(context.Canceled)
	if !result.Interrupted || result.Error != "" {
		t.Fatalf("cancellation misclassified: %+v", result)
	}
	s, err = r.Prepare(context.Background(), model.Challenge{Fixtures: []model.Fixture{{Files: map[string]string{"input": "x"}}}}, a)
	if err != nil {
		t.Fatal(err)
	}
	result = s.Finish(context.Canceled)
	if !result.Interrupted || result.Error != "" {
		t.Fatalf("handoff cancellation misclassified: %+v", result)
	}
}

// TestIntegrationCheckNowLeavesTheSessionRunning covers the mid-session check.
// Check removes every container carrying the attempt's owner label, which
// would kill the exercise the operator is working in; CheckNow must not.
func TestIntegrationCheckNowLeavesTheSessionRunning(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := model.Challenge{
		ID: "shell.checknow", Revision: 1, Profile: "standard",
		Validator: model.Validator{Kind: "tree", OutputPolicy: "exact", Version: "1"},
		Fixtures: []model.Fixture{{
			Name:          "one",
			Files:         map[string]string{"answer.txt": "before\n"},
			ExpectedFiles: map[string]string{"answer.txt": "after\n"},
		}},
	}
	s, err := r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	container := s.container

	// Failing check while the session container is still alive.
	result, err := r.CheckNow(ctx, c, a)
	if err != nil {
		t.Fatalf("CheckNow before the edit: %v", err)
	}
	if result.Outcome == "pass" {
		t.Fatal("CheckNow passed before the workspace was edited")
	}
	if !containerRunning(t, container) {
		t.Fatal("CheckNow stopped the live session container")
	}

	// Apply the fix through the live session container, the way the operator
	// would by editing inside the running exercise.
	if err := r.populate(ctx, container, map[string]string{"answer.txt": "after\n"}); err != nil {
		t.Fatal(err)
	}
	result, err = r.CheckNow(ctx, c, a)
	if err != nil {
		t.Fatalf("CheckNow after the edit: %v", err)
	}
	if result.Outcome != "pass" {
		t.Fatalf("CheckNow did not pass after the fix: %+v", result)
	}
	if !containerRunning(t, container) {
		t.Fatal("the second CheckNow stopped the live session container")
	}
	s.Finish(nil)
}

func containerRunning(t *testing.T, name string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, _, err := docker(ctx, nil, "inspect", "--format", "{{.State.Running}}", name)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// TestIntegrationDockerSessionRunsUnderAPseudoTerminal covers the workbench
// path for Docker-backed exercises. The session command is `docker exec -it`,
// and the resize must reach the container: the kernel raises SIGWINCH on the
// docker client, which forwards it over the API.
func TestIntegrationDockerSessionRunsUnderAPseudoTerminal(t *testing.T) {
	r, a := integrationRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := model.Challenge{
		ID: "shell.docker-pane", Revision: 1, Profile: "standard",
		Validator: model.Validator{Kind: "tree", OutputPolicy: "exact", Version: "1"},
		Fixtures:  []model.Fixture{{Name: "one", Files: map[string]string{"answer.txt": "before\n"}}},
	}
	s, err := r.Prepare(ctx, c, a)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Finish(nil)

	p, err := pane.Start(s.Command(), 80, 24)
	if err != nil {
		t.Fatalf("pane.Start: %v", err)
	}
	defer p.Close()

	waitFor := func(want string, d time.Duration) string {
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			if out := ansi.Strip(p.Render()); strings.Contains(out, want) {
				return out
			}
			time.Sleep(50 * time.Millisecond)
		}
		return ansi.Strip(p.Render())
	}
	if out := waitFor("Driving Range.", 60*time.Second); !strings.Contains(out, "Driving Range.") {
		t.Fatalf("container shell banner never rendered:\n%s", out)
	}

	// The child polls its own size. A single sample would race the resize,
	// which the docker client forwards asynchronously over the API, and a
	// stale reading could never be corrected.
	typeLine(p, "while :; do stty size; sleep 0.3; done")
	if out := waitFor("24 80", 30*time.Second); !strings.Contains(out, "24 80") {
		t.Fatalf("child did not start at 24x80:\n%s", out)
	}
	if err := p.Resize(100, 30); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if out := waitFor("30 100", 30*time.Second); !strings.Contains(out, "30 100") {
		t.Errorf("resize did not reach the container:\n%s", out)
	}
}

func typeLine(p *pane.Session, text string) {
	for _, r := range text {
		p.SendKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	p.SendKey(tea.KeyPressMsg{Code: tea.KeyEnter})
}
