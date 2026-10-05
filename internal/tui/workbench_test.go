package tui

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stevencarpenter/driving-range/internal/model"
	"github.com/stevencarpenter/driving-range/internal/pane"
)

func TestWorkbenchPaletteOpensOnF12Only(t *testing.T) {
	w := newWorkbench(nil, "Change port 8080 to 9090", "full brief text")
	if w.palette {
		t.Fatal("palette should start closed")
	}
	handled, _ := w.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if handled {
		t.Error("ordinary keys must not be handled by the workbench")
	}
	if w.palette {
		t.Error("ordinary keys must not open the palette")
	}
	handled, _ = w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	if !handled || !w.palette {
		t.Error("F12 must open the palette")
	}
}

func TestWorkbenchPaletteEmitsActions(t *testing.T) {
	for _, c := range []struct {
		key    rune
		action string
	}{{'c', "check"}, {'h', "hint"}, {'v', "reveal"}, {'q', "quit"}} {
		w := newWorkbench(nil, "g", "b")
		w.update(tea.KeyPressMsg{Code: tea.KeyF12})
		handled, action := w.update(tea.KeyPressMsg{Code: c.key, Text: string(c.key)})
		if !handled || action != c.action {
			t.Errorf("palette %q = (%v, %q), want (true, %q)", c.key, handled, action, c.action)
		}
		if w.palette {
			t.Errorf("palette must close after %q", c.key)
		}
	}
}

func TestWorkbenchPaletteEscapeReturnsToChild(t *testing.T) {
	w := newWorkbench(nil, "g", "b")
	w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	handled, action := w.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || action != "" {
		t.Errorf("palette esc = (%v, %q), want (true, \"\")", handled, action)
	}
	if w.palette {
		t.Error("escape must close the palette")
	}
}

func TestWorkbenchPaletteUnknownKeyClosesWithoutActing(t *testing.T) {
	w := newWorkbench(nil, "g", "b")
	w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	handled, action := w.update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if !handled || action != "" {
		t.Errorf("palette z = (%v, %q), want (true, \"\")", handled, action)
	}
	if w.palette {
		t.Error("an unknown key must close the palette rather than swallow the next keystroke")
	}
}

func TestWorkbenchBandTogglesFromThePalette(t *testing.T) {
	w := newWorkbench(nil, "Change port 8080 to 9090", "b")
	if !w.band {
		t.Fatal("band should start expanded")
	}
	w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	w.update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if w.band {
		t.Error("palette b must collapse the band")
	}
}

func TestWorkbenchCtrlCReachesTheChild(t *testing.T) {
	w := newWorkbench(nil, "g", "b")
	handled, _ := w.update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if handled {
		t.Error("ctrl+c must reach the child, not quit golf")
	}
}

func TestWorkbenchLayoutAlwaysFillsTheTerminalExactly(t *testing.T) {
	m := testModel(t)
	short := "Short goal."
	long := strings.Repeat("A very long objective that certainly wraps across lines. ", 4)
	for _, goal := range []string{short, long} {
		for _, band := range []bool{true, false} {
			for _, size := range [][2]int{{80, 24}, {100, 30}, {60, 16}, {40, 10}} {
				w := newWorkbench(nil, goal, "brief detail that differs")
				w.band = band
				view := w.view(size[0], size[1], m.styles(), m.border(), m.plain(), false)
				if got := len(strings.Split(view, "\n")); got != size[1] {
					t.Errorf("band=%v %v: view is %d lines, want %d", band, size, got, size[1])
				}
				if w.bandRows()+w.paneHeight(size[1])+1 != size[1] {
					t.Errorf("band=%v %v: band %d + pane %d + status 1 != %d",
						band, size, w.bandRows(), w.paneHeight(size[1]), size[1])
				}
			}
		}
	}
}

func TestWorkbenchViewShowsGoalAndKeepsTerminalHeight(t *testing.T) {
	m := testModel(t)
	w := newWorkbench(nil, "Change port 8080 to 9090", "Keep the comment unchanged.")
	for _, band := range []bool{true, false} {
		w.band = band
		view := w.view(80, 24, m.styles(), m.border(), m.plain(), false)
		if !strings.Contains(view, "Change port 8080 to 9090") {
			t.Errorf("band=%v hides the goal:\n%s", band, view)
		}
		if got := len(strings.Split(view, "\n")); got != 24 {
			t.Errorf("band=%v view is %d lines, want 24", band, got)
		}
	}
}

func TestWorkbenchPlainViewHasNoEscapes(t *testing.T) {
	m := testModel(t)
	m.service.Config.Theme = "plain"
	w := newWorkbench(nil, "goal text", "brief")
	if view := w.view(80, 24, m.styles(), m.border(), true, false); strings.Contains(view, "\x1b") {
		t.Errorf("plain workbench view contains escapes: %q", view)
	}
}

func preparedModel(t *testing.T, classic bool) Model {
	t.Helper()
	m := testModel(t)
	m.service.Config.Classic = classic
	c := m.service.Catalog.Challenges[0]
	m.challenge = &c
	return m
}

func TestUseWorkbenchRouting(t *testing.T) {
	cases := []struct {
		name      string
		classic   bool
		challenge bool
		want      bool
	}{
		{"default embeds the exercise", false, true, true},
		{"classic opts out", true, true, false},
		{"no challenge means no brief to show", false, false, false},
		{"classic wins over everything", true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := testModel(t)
			m.service.Config.Classic = c.classic
			if c.challenge {
				ch := m.service.Catalog.Challenges[0]
				m.challenge = &ch
			}
			if got := m.useWorkbench(); got != c.want {
				t.Errorf("useWorkbench() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestWorkbenchKeysReachTheChildAndDoNotNavigate(t *testing.T) {
	m := preparedModel(t, false)
	session, err := pane.Start(exec.Command("/bin/cat"), 40, 6, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	m.workbench = newWorkbench(session, "g", "b")
	m.screen = practice
	before := m.screen

	// A key that normally navigates must not, while an exercise is running.
	updated, _ := m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	if got := updated.(Model).screen; got != before {
		t.Errorf("key navigated to screen %d while the workbench was active", got)
	}
	// ctrl+c must not quit golf out from under a running exercise.
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd != nil {
		t.Error("ctrl+c must reach the child rather than producing a golf command")
	}
}

func TestWorkbenchRenderReplacesTheNormalView(t *testing.T) {
	m := preparedModel(t, false)
	m.workbench = newWorkbench(nil, "Change the port", "brief")
	m.width, m.height = 80, 24
	view := m.render()
	if !strings.Contains(view, "Change the port") {
		t.Errorf("workbench view missing the goal:\n%s", view)
	}
	if strings.Contains(view, "DRIVING RANGE") {
		t.Error("workbench view should replace the navigation chrome, not sit under it")
	}
}

func TestWorkbenchViewRendersTheStatusLine(t *testing.T) {
	m := testModel(t)
	w := newWorkbench(nil, "goal", "brief")
	view := w.view(80, 24, m.styles(), m.border(), m.plain(), false)
	lines := strings.Split(view, "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "F12") {
		t.Errorf("last line is not the status line: %q", last)
	}
	if !strings.Contains(last, "CHECK") {
		t.Errorf("status line omits the check state: %q", last)
	}
}

func TestWorkbenchBandDoesNotRepeatTheObjective(t *testing.T) {
	m := testModel(t)
	goal := "Read names.txt and print each line."
	// DESIGN.md: omit the brief's opening paragraph when it exactly repeats
	// the displayed objective.
	w := newWorkbench(nil, goal, goal+"\n\nMore detail follows.")
	view := w.view(80, 24, m.styles(), m.border(), true, false)
	if n := strings.Count(view, "Read names.txt"); n != 1 {
		t.Errorf("objective appears %d times in the band, want 1:\n%s", n, view)
	}
}

func TestWorkbenchResultsCanBeReadAndReopened(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 40, 10
	m.workbench = newWorkbench(nil, "g", "b")
	w := m.workbench
	result := "FAIL: one fixture differed\nexpected x, got y\n" + strings.Repeat("More fixture details.\n", 30) + "Last fixture detail."
	updated, _ := m.Update(operationMsg{text: result})
	m = updated.(Model)
	view := m.render()
	if !strings.Contains(view, "FAIL: one fixture differed") {
		t.Fatalf("check result missing:\n%s", view)
	}
	if !strings.Contains(view, "expected x, got y") {
		t.Fatalf("result details hidden:\n%s", view)
	}
	w.update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if view := m.render(); !strings.Contains(view, "Last fixture detail.") {
		t.Fatalf("cannot scroll to the end:\n%s", view)
	}
	w.update(tea.KeyPressMsg{Code: tea.KeyHome})
	if view := m.render(); !strings.Contains(view, "expected x, got y") {
		t.Fatalf("cannot scroll back to the start:\n%s", view)
	}
	w.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if strings.Contains(m.render(), "expected x, got y") {
		t.Fatal("Escape did not return to the child pane")
	}
	w.update(tea.KeyPressMsg{Code: tea.KeyF12})
	w.update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if !strings.Contains(m.render(), "expected x, got y") {
		t.Fatal("the palette could not reopen the saved result")
	}

	w.result("", errTest)
	if !strings.Contains(m.render(), "error: boom") {
		t.Errorf("error not surfaced:\n%s", m.render())
	}
}

func TestWorkbenchRevealConfirmationRecordsAssistance(t *testing.T) {
	m := preparedModel(t, false)
	m.service.Catalog.Challenges[0].Explanation = "Explanation line one.\nExplanation line two."
	m.service.Catalog.Challenges[0].ReferenceSolution = "printf 'reference solution'"
	m.attemptID = "workbench-reveal"
	if err := m.service.Store.CreateAttempt(model.Attempt{
		ID: m.attemptID, ExerciseID: m.challenge.ID, Revision: m.challenge.Revision,
		Track: m.challenge.Track, Profile: m.challenge.Profile, ValidatorVersion: m.challenge.Validator.Version,
		Status: "active", CreatedAt: m.now(),
	}); err != nil {
		t.Fatal(err)
	}
	m.workbench = newWorkbench(nil, "g", "b")
	var cmd tea.Cmd
	for _, cancel := range []string{"n", "esc"} {
		updated, _ := m.workbenchAction("reveal")
		m = updated.(Model)
		if view := m.render(); !strings.Contains(view, "This records assistance") {
			t.Fatalf("confirmation is not visible:\n%s", view)
		}
		m, cmd = press(m, "z")
		if cmd != nil || m.confirm != "reveal" {
			t.Fatal("an unrelated key confirmed or dismissed the reveal")
		}
		m, cmd = press(m, cancel)
		if cmd != nil || m.confirm != "" {
			t.Fatalf("%s did not cancel the reveal", cancel)
		}
		attempt, err := m.service.Store.Attempt(m.attemptID)
		if err != nil || attempt.SolutionRevealed {
			t.Fatalf("cancel recorded assistance: %+v, %v", attempt, err)
		}
	}
	updated, _ := m.workbenchAction("reveal")
	m = updated.(Model)
	m, cmd = press(m, "y")
	if cmd == nil || m.confirm != "" {
		t.Fatal("confirm did not dispatch the reveal")
	}
	message := cmd()
	if batch, ok := message.(tea.BatchMsg); ok {
		for _, command := range batch {
			if result := command(); result != nil {
				if _, ok := result.(operationMsg); ok {
					message = result
				}
			}
		}
	}
	updated, _ = m.Update(message)
	m = updated.(Model)
	attempt, err := m.service.Store.Attempt(m.attemptID)
	if err != nil || !attempt.SolutionRevealed || attempt.Status != "active" {
		t.Fatalf("reveal did not retain an active, assisted attempt: %+v, %v", attempt, err)
	}
	if view := m.render(); !strings.Contains(view, "Explanation line two.") || !strings.Contains(view, "printf 'reference solution'") {
		t.Fatalf("complete explanation and reference are not visible:\n%s", view)
	}
}

func TestWorkbenchPanelsCaptureKeysAndRestoreChildFocus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := pane.Start(exec.CommandContext(ctx, "/bin/sh", "-c", `IFS= read -r line; printf 'RECEIVED:%s\n' "$line"`), 80, 18, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		session.Close()
		session.Wait()
	})
	m := preparedModel(t, false)
	m.workbench = newWorkbench(session, "g", "b")
	m.workbench.result("Hint line one.\nHint line two.", nil)
	if m.View().Cursor != nil {
		t.Fatal("result panel shows the child's cursor")
	}
	m, _ = press(m, "z")
	updated, _ := m.Update(tea.PasteMsg{Content: "result paste must be consumed"})
	m = updated.(Model)
	m, _ = press(m, "esc")
	if m.View().Cursor == nil {
		t.Fatal("closing the result panel did not restore the child's cursor")
	}
	updated, _ = m.workbenchAction("reveal")
	m = updated.(Model)
	if m.View().Cursor != nil {
		t.Fatal("confirmation shows the child's cursor")
	}
	m, _ = press(m, "z")
	updated, _ = m.Update(tea.PasteMsg{Content: "confirmation paste must be consumed"})
	m = updated.(Model)
	m, _ = press(m, "n")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyF12})
	m = updated.(Model)
	updated, _ = m.Update(tea.PasteMsg{Content: "palette paste must be consumed"})
	m = updated.(Model)
	m, _ = press(m, "esc")
	updated, _ = m.Update(tea.PasteMsg{Content: "日本語"})
	m = updated.(Model)
	m, _ = press(m, "x")
	m, _ = press(m, "enter")
	if err := session.Wait(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if out := session.Render(); strings.Contains(out, "RECEIVED:日本語x") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("panel keys reached the child or focus was not restored: %q", session.Render())
}

func TestWorkbenchPanelsStayWithinTerminal(t *testing.T) {
	m := testModel(t)
	for _, theme := range []string{"plain", "dark", "light"} {
		m.service.Config.Theme = theme
		for _, size := range [][2]int{{1, 1}, {12, 8}, {40, 10}, {80, 24}, {120, 32}} {
			m.width, m.height = size[0], size[1]
			for _, state := range []string{"result", "confirmation", "palette"} {
				m.workbench = newWorkbench(nil, strings.Repeat("Long goal 日本語. ", 10), "brief")
				m.confirm = ""
				switch state {
				case "result":
					m.workbench.result("unsafe\x1b[2J\x1b]52;c;secret\a\r\x00\n"+strings.Repeat("Details 日本語. ", 50), nil)
				case "confirmation":
					m.confirm = "reveal"
				case "palette":
					m.workbench.palette = true
				}
				view := m.render()
				if got := len(strings.Split(view, "\n")); got != m.height {
					t.Fatalf("%s %s at %v: height %d", theme, state, size, got)
				}
				for _, line := range strings.Split(view, "\n") {
					if ansi.StringWidth(line) > m.width {
						t.Fatalf("%s %s at %v: line overflows %q", theme, state, size, line)
					}
				}
				if strings.ContainsAny(view, "\a\r\x00") || strings.Contains(view, "52;c;") || m.plain() && strings.Contains(view, "\x1b") {
					t.Fatalf("%s %s at %v: unsafe terminal output %q", theme, state, size, view)
				}
			}
		}
	}
}

func TestWorkbenchChildExitCancelsRevealConfirmation(t *testing.T) {
	m := preparedModel(t, false)
	session, err := pane.Start(exec.Command("/bin/cat"), 80, 18, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		session.Close()
		session.Wait()
	})
	m.workbench = newWorkbench(session, "g", "b")
	updated, _ := m.workbenchAction("reveal")
	m = updated.(Model)
	updated, _ = m.Update(exitedMsg{})
	m = updated.(Model)
	if m.workbench != nil || m.confirm != "" {
		t.Fatal("child exit retained a workbench confirmation")
	}
	if !m.busy {
		t.Fatal("child exit left navigation unlocked while the final result is being saved")
	}
}

func TestWorkbenchDoesNotOverlapOperations(t *testing.T) {
	m := preparedModel(t, false)
	m.workbench = newWorkbench(nil, "g", "b")
	updated, cmd := m.workbenchAction("check")
	m = updated.(Model)
	if cmd == nil || !m.busy {
		t.Fatal("check did not start")
	}
	for _, action := range []string{"check", "hint", "reveal"} {
		updated, next := m.Update(paneTriggerMsg{action: action, workbench: m.workbench})
		m = updated.(Model)
		if next != nil || !m.busy || m.confirm != "" {
			t.Fatalf("%s overlapped the pending check", action)
		}
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyF12})
	m = updated.(Model)
	updated, next := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)
	if next != nil || !m.busy {
		t.Fatal("palette hint overlapped the pending check")
	}
	updated, _ = m.Update(operationMsg{text: "FAIL: saved interim check"})
	m = updated.(Model)
	m.workbench.outputOpen = false
	updated, next = m.workbenchAction("hint")
	if next == nil || !updated.(Model).busy {
		t.Fatal("completion did not allow the next operation")
	}
}

func TestWorkbenchHelperActionStartsTheSpinner(t *testing.T) {
	m := preparedModel(t, false)
	m.attemptID = "missing-attempt"
	m.workbench = newWorkbench(nil, "g", "b")
	updated, cmd := m.Update(paneTriggerMsg{action: "hint", workbench: m.workbench})
	m = updated.(Model)
	if cmd == nil || !m.busy {
		t.Fatal("helper action did not start an operation")
	}
	message := cmd()
	batch, ok := message.(tea.BatchMsg)
	if !ok {
		t.Fatal("helper action did not schedule spinner ticks")
	}
	foundTick, foundResult := false, false
	for _, command := range batch {
		switch command().(type) {
		case spinner.TickMsg:
			foundTick = true
		case operationMsg:
			foundResult = true
		}
	}
	if !foundTick || !foundResult {
		t.Fatal("helper action must schedule its result and spinner")
	}
}

func TestWorkbenchExitWaitsForPendingActionAndLocksNavigation(t *testing.T) {
	m := preparedModel(t, false)
	m.screen = exercise
	session, err := pane.Start(exec.Command("/bin/cat"), 80, 18, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close(); session.Wait() })
	w := newWorkbench(session, "g", "b")
	m.workbench, m.busy = w, true
	updated, cmd := m.Update(exitedMsg{})
	m = updated.(Model)
	if cmd != nil || !m.busy || m.pendingExit == nil || m.workbench != nil {
		t.Fatal("child exit did not wait for the pending workbench action")
	}
	for _, key := range []string{"q", "tab", "r", "c"} {
		updated, next := press(m, key)
		if next != nil || updated.screen != exercise || !updated.busy {
			t.Fatalf("%s unlocked navigation before finalization", key)
		}
	}
	updated, cmd = m.Update(operationMsg{text: "old interim result", workbench: w})
	m = updated.(Model)
	if cmd == nil || !m.busy || m.pendingExit != nil || m.notice != "" {
		t.Fatal("interim completion did not begin finalization while retaining the navigation lock")
	}
	updated, _ = m.Update(operationMsg{text: "PASS: final result", finished: true})
	m = updated.(Model)
	updated, cmd = m.Update(operationMsg{text: "late interim result", workbench: w})
	m = updated.(Model)
	if cmd != nil || m.busy || m.notice != "PASS: final result" {
		t.Fatal("stale workbench result replaced the final result")
	}
}

func TestWorkbenchTriggersCannotCrossSessionsOrConfirmation(t *testing.T) {
	m := preparedModel(t, false)
	old := newWorkbench(nil, "old goal", "b")
	m.workbench = newWorkbench(nil, "current goal", "b")
	updated, cmd := m.Update(paneTriggerMsg{action: "hint", workbench: old})
	m = updated.(Model)
	if cmd != nil || m.busy {
		t.Fatal("a trigger from the previous session started an action")
	}
	m.confirm = "reveal"
	updated, cmd = m.Update(paneTriggerMsg{action: "check", workbench: m.workbench})
	m = updated.(Model)
	if cmd != nil || m.busy || m.confirm != "reveal" {
		t.Fatal("a trigger overtook the reveal confirmation")
	}
}

func TestWorkbenchHelperCannotQuitOrReveal(t *testing.T) {
	for _, action := range []string{"quit", "reveal", "unknown"} {
		t.Run(action, func(t *testing.T) {
			m := preparedModel(t, false)
			m.workbench = newWorkbench(nil, "g", "b")
			updated, cmd := m.Update(paneTriggerMsg{action: action, workbench: m.workbench})
			m = updated.(Model)
			if cmd != nil || m.confirm != "" || m.busy {
				t.Fatalf("unsupported helper action %q changed the outer UI", action)
			}
		})
	}
}

func TestWorkbenchCursorStaysInsideTheVisibleChildPane(t *testing.T) {
	m := preparedModel(t, false)
	session, err := pane.Start(exec.Command("/bin/sh", "-c", "printf '\\033[18;1HREADY'; exec cat"), 80, 18, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close(); session.Wait() })
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(session.Render(), "READY") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(session.Render(), "READY") {
		t.Fatal("child cursor was not positioned")
	}
	m.workbench = newWorkbench(session, "g", "b")
	for _, height := range []int{1, 2, 8, 24} {
		m.height = height
		view := m.View()
		if height == 1 && view.Cursor != nil {
			t.Fatal("cursor is visible when there is no child pane")
		}
		if view.Cursor != nil && (view.Cursor.Y < m.workbench.bandRows() || view.Cursor.Y >= height-1) {
			t.Fatalf("height %d: cursor outside the child pane: %+v", height, view.Cursor)
		}
	}
}

func TestWorkbenchPasteKeepsUpdateAndViewResponsive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := pane.Start(exec.CommandContext(ctx, "/bin/sh", "-c", "stty raw -echo; printf READY; exec sleep 30"), 80, 18, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); s.Wait(); s.Close() })
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(s.Render(), "READY") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(s.Render(), "READY") {
		t.Fatal("child did not enter raw input mode")
	}
	m := preparedModel(t, false)
	m.workbench = newWorkbench(s, "g", "b")
	done := make(chan struct{})
	go func() {
		updated, _ := m.Update(tea.PasteMsg{Content: strings.Repeat("x", 1<<20)})
		updated.View()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("paste blocked the event loop while the child was not reading input")
	}
}

func TestWorkbenchCursorFollowsChildVisibility(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := pane.Start(exec.CommandContext(ctx, "/bin/sh", "-c", "printf '\\033[?25lHIDDEN'; read line; printf '\\033[?25hVISIBLE'; exec cat"), 80, 18, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); s.Wait(); s.Close() })
	m := preparedModel(t, false)
	m.workbench = newWorkbench(s, "g", "b")
	for _, state := range []struct {
		text    string
		visible bool
	}{{"HIDDEN", false}, {"VISIBLE", true}} {
		deadline := time.Now().Add(time.Second)
		for !strings.Contains(s.Render(), state.text) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if !strings.Contains(s.Render(), state.text) {
			t.Fatalf("child did not report %s", state.text)
		}
		if got := m.View().Cursor != nil; got != state.visible {
			t.Fatalf("%s: host cursor visible=%t, want %t", state.text, got, state.visible)
		}
		m, _ = press(m, "enter")
	}
}

var errTest = errors.New("boom")
