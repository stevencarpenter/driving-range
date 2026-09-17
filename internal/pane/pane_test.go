package pane

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func waitFor(t *testing.T, s *Session, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if out := s.Render(); strings.Contains(out, want) {
			return out
		}
		time.Sleep(20 * time.Millisecond)
	}
	return s.Render()
}

func start(t *testing.T, cmd *exec.Cmd, w, h int, onTrigger func(string)) *Session {
	t.Helper()
	s, err := Start(cmd, w, h, onTrigger)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSessionRendersChildOutput(t *testing.T) {
	s := start(t, exec.Command("/bin/sh", "-c", "printf 'hello pane\\n'; sleep 30"), 40, 6, nil)
	if out := waitFor(t, s, "hello pane"); !strings.Contains(out, "hello pane") {
		t.Errorf("Render() = %q, want it to contain %q", out, "hello pane")
	}
}

func TestSessionForwardsKeys(t *testing.T) {
	s := start(t, exec.Command("/bin/cat"), 40, 6, nil)
	for _, r := range "abc" {
		s.SendKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	s.SendKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if out := waitFor(t, s, "abc"); !strings.Contains(out, "abc") {
		t.Errorf("Render() = %q, want it to contain %q", out, "abc")
	}
}

func TestSessionForwardsModifiedSpecialKeys(t *testing.T) {
	// cat -v renders control bytes visibly, so the exact sequence is checked.
	s := start(t, exec.Command("/bin/cat", "-v"), 40, 6, nil)
	s.SendKey(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})
	s.SendKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if out := waitFor(t, s, "^[[1;5A"); !strings.Contains(out, "^[[1;5A") {
		t.Errorf("Render() = %q, want it to contain %q", out, "^[[1;5A")
	}
}

func TestSessionResizePropagatesToChild(t *testing.T) {
	// The child polls its own terminal size rather than trapping SIGWINCH,
	// because a POSIX shell defers trap handlers until the foreground command
	// finishes, so a trap around sleep would never run.
	s := start(t, exec.Command("/bin/sh", "-c", "while :; do stty size; sleep 0.2; done"), 40, 6, nil)
	if out := waitFor(t, s, "6 40"); !strings.Contains(out, "6 40") {
		t.Fatalf("child did not start at 6x40; Render() = %q", out)
	}
	if err := s.Resize(72, 10); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if out := waitFor(t, s, "10 72"); !strings.Contains(out, "10 72") {
		t.Errorf("child did not observe the resize; Render() = %q", out)
	}
}

func TestSessionWaitReturnsAfterChildExits(t *testing.T) {
	s := start(t, exec.Command("/bin/sh", "-c", "exit 3"), 40, 6, nil)
	done := make(chan error, 1)
	go func() { done <- s.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 3 {
			t.Errorf("Wait() = %v, want exit status 3", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return")
	}
}

func TestSessionRejectsNonPositiveSize(t *testing.T) {
	if _, err := Start(exec.Command("/bin/cat"), 0, 6, nil); err == nil {
		t.Error("Start accepted a zero width")
	}
}

func TestSessionTriggerFiresAndIsNotRendered(t *testing.T) {
	got := make(chan string, 1)
	s := start(t, exec.Command("/bin/sh", "-c", `printf 'before\033]9270;golf=check\007after\n'; sleep 30`), 40, 6, func(action string) {
		select {
		case got <- action:
		default:
		}
	})
	select {
	case action := <-got:
		if action != "check" {
			t.Errorf("trigger action = %q, want %q", action, "check")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("trigger did not fire")
	}
	out := waitFor(t, s, "after")
	if strings.Contains(out, "9270") || strings.Contains(out, "golf=check") {
		t.Errorf("trigger sequence leaked into the rendered screen: %q", out)
	}
	if !strings.Contains(out, "before") || !strings.Contains(out, "after") {
		t.Errorf("surrounding output was lost: %q", out)
	}
}

func TestSessionDropsClipboardWrites(t *testing.T) {
	s := start(t, exec.Command("/bin/sh", "-c", `printf '\033]52;c;aGVsbG8=\007visible\n'; sleep 30`), 40, 6, nil)
	out := waitFor(t, s, "visible")
	if strings.Contains(out, "52;c") || strings.Contains(out, "aGVsbG8") {
		t.Errorf("clipboard sequence leaked: %q", out)
	}
}

func TestSessionImmediateTriggerIsNotLost(t *testing.T) {
	got := make(chan string, 1)
	start(t, exec.Command("/bin/sh", "-c", `printf '\033]9270;golf=check\007'`), 40, 6, func(action string) {
		got <- action
	})
	select {
	case action := <-got:
		if action != "check" {
			t.Errorf("immediate trigger = %q, want %q", action, "check")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("an immediate trigger was lost")
	}
}
