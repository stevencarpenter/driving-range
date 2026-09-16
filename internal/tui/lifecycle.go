package tui

import (
	"context"
	"errors"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// lifecycle is shared by Model copies. Bubble Tea deliberately does not wait
// for Cmd goroutines when its event loop exits, so database users and a prepared
// child need their own shutdown boundary before the caller closes the service.
type lifecycle struct {
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	stopping  bool
	wg        sync.WaitGroup
	cleanup   func() error
	finishErr error
}

func newLifecycle() *lifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &lifecycle{ctx: ctx, cancel: cancel}
}

func (l *lifecycle) command(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		l.mu.Lock()
		if l.stopping {
			l.mu.Unlock()
			return nil
		}
		l.wg.Add(1)
		l.mu.Unlock()
		defer l.wg.Done()
		return cmd()
	}
}

func (l *lifecycle) prepared(cleanup func() error) {
	l.mu.Lock()
	l.cleanup = cleanup
	l.mu.Unlock()
}

func (l *lifecycle) finished(err error) {
	l.mu.Lock()
	l.cleanup = nil
	l.finishErr = err
	l.mu.Unlock()
}

func (l *lifecycle) acknowledge() { l.mu.Lock(); l.finishErr = nil; l.mu.Unlock() }

func (l *lifecycle) shutdown() error {
	l.mu.Lock()
	l.stopping = true
	l.cancel()
	l.mu.Unlock()
	// Cancellation bounds runner operations; wait also covers their final
	// persistence writes. Returning early would race Service.Close again.
	l.wg.Wait()
	l.mu.Lock()
	cleanup := l.cleanup
	err := l.finishErr
	l.cleanup = nil
	l.mu.Unlock()
	if cleanup != nil {
		return errors.Join(err, cleanup())
	}
	return err
}
