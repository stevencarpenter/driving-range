package tui

import (
	"context"
	"errors"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// lifecycle is shared by Model copies. Bubble Tea deliberately does not wait
// for Cmd goroutines when its event loop exits, so database users and a prepared
// child need their own shutdown boundary before the caller closes the service.
type lifecycle struct {
	ctx       context.Context
	cancel    context.CancelFunc
	program   *tea.Program
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

// attach records the running program so work started outside the Bubble Tea
// event loop, such as an emulator trigger handler, can deliver a message.
func (l *lifecycle) attach(p *tea.Program) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.program = p
}

// send delivers a message from outside the event loop. It is a no-op once
// shutdown has begun, so a late trigger cannot revive a closing program.
func (l *lifecycle) send(msg tea.Msg) {
	l.mu.Lock()
	p, stopping := l.program, l.stopping
	l.mu.Unlock()
	if p != nil && !stopping {
		p.Send(msg)
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
