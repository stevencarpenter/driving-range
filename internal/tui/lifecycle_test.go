package tui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestShutdownWaitsForCanceledEffectAndCleansUndeliveredPreparation(t *testing.T) {
	l := newLifecycle()
	started, release := make(chan struct{}), make(chan struct{})
	cleanupFailure := errors.New("save interrupted session")
	finalWrite, cleaned := false, false
	cmd := l.command(func() tea.Msg {
		close(started)
		<-l.ctx.Done()
		// Preparation can finish just as Bubble Tea stops consuming messages.
		l.prepared(func() error {
			if !finalWrite {
				t.Error("cleanup ran before the effect's final persistence write")
			}
			cleaned = true
			return cleanupFailure
		})
		<-release
		finalWrite = true
		return preparedMsg{}
	})
	go cmd()
	<-started
	done := make(chan error, 1)
	go func() { done <- l.shutdown() }()
	select {
	case <-l.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel running effect")
	}
	select {
	case <-done:
		t.Fatal("shutdown returned before the effect finished")
	default:
	}
	close(release)
	select {
	case err := <-done:
		if !errors.Is(err, cleanupFailure) || !cleaned {
			t.Fatalf("prepared child was not finalized: cleaned=%t err=%v", cleaned, err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not complete after cancellation and final write")
	}
}

func TestShutdownRejectsQueuedEffects(t *testing.T) {
	l := newLifecycle()
	ran := false
	cmd := l.command(func() tea.Msg { ran = true; return loadedMsg{} })
	if err := l.shutdown(); err != nil {
		t.Fatal(err)
	}
	if msg := cmd(); msg != nil || ran {
		t.Fatal("queued command accessed a service after shutdown")
	}
}

func TestConsumedFinishDoesNotFinalizeTwice(t *testing.T) {
	l := newLifecycle()
	l.prepared(func() error { t.Error("completed session was finalized twice"); return nil })
	l.finished(errors.New("already shown persistence failure"))
	l.acknowledge()
	if err := l.shutdown(); err != nil {
		t.Fatal(err)
	}
}

func TestUnconsumedFinishFailureSurvivesShutdown(t *testing.T) {
	l := newLifecycle()
	failure := errors.New("save session: disk full")
	l.finished(failure)
	if err := l.shutdown(); !errors.Is(err, failure) {
		t.Fatalf("lost persistence failure: %v", err)
	}
}

func TestInterimResultDoesNotAcknowledgeFinalSaveFailure(t *testing.T) {
	m := testModel(t)
	failure := errors.New("save session: disk full")
	m.lifecycle.finished(failure)
	updated, _ := m.Update(operationMsg{text: "interim check"})
	m = updated.(Model)
	if !errors.Is(m.lifecycle.finishErr, failure) {
		t.Error("interim result cleared an unconsumed final save failure")
	}
	m.Update(operationMsg{err: failure, finished: true})
	if m.lifecycle.finishErr != nil {
		t.Fatal("final result did not acknowledge its displayed failure")
	}
}
