package pane

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func waitForText(t *testing.T, s *Session, want string, d time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if out := ansi.Strip(s.Render()); strings.Contains(out, want) {
			return out
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ansi.Strip(s.Render())
}

func typeText(s *Session, text string) {
	for _, r := range text {
		s.SendKey(tea.KeyPressMsg{Code: r, Text: string(r)})
		time.Sleep(8 * time.Millisecond)
	}
}

// TestNvimEditsInPane is the gate for hosting a full-screen alternate-screen
// editor. The unit tests above use line-oriented children, which never
// exercise the alternate screen, cursor addressing, or a redraw storm.
func TestNvimEditsInPane(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("port: 8080\nhost: local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Matches the argv the Docker runner uses for vim exercises.
	cmd := exec.Command("nvim", "--clean", "-i", "NONE", "--cmd", "set nomodeline", "--", file)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	s := start(t, cmd, 80, 24, nil)

	if out := waitForText(t, s, "port: 8080", 15*time.Second); !strings.Contains(out, "port: 8080") {
		t.Fatalf("nvim did not render the file:\n%s", out)
	}
	if !s.emu.IsAltScreen() {
		t.Error("nvim should be on the alternate screen")
	}
	if !strings.Contains(s.Render(), "\x1b[") {
		t.Error("nvim output carried no styling")
	}

	// Change 8080 to 9090 with real motions, then write and quit. f8 jumps to
	// the first 8, cw replaces the whole number.
	typeText(s, "f8cw")
	s.Paste("9090")
	s.SendKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(150 * time.Millisecond)
	typeText(s, ":wq")
	s.SendKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	done := make(chan error, 1)
	go func() { done <- s.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("nvim exited with error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("nvim did not exit after :wq; screen:\n%s", ansi.Strip(s.Render()))
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "port: 9090") {
		t.Errorf("edit did not land; file = %q", string(got))
	}
}
