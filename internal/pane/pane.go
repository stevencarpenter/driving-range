package pane

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// TriggerOSC is an unregistered OSC number carrying golf actions from inside
// the exercise. A host terminal that receives it when the workbench is not
// running ignores it.
const TriggerOSC = 9270

// clipboardOSC is OSC 52. The child must not write the host clipboard.
const clipboardOSC = 52

// io.Copy reads at most 32 KiB per chunk, bounding queued input to 2 MiB.
type inputQueue chan []byte

var errInputBacklog = errors.New("terminal input backlog full; session stopped to avoid truncating input")

func (q inputQueue) Write(p []byte) (int, error) {
	select {
	case q <- bytes.Clone(p):
		return len(p), nil
	default:
		return 0, errInputBacklog
	}
}

// Session couples a child process on a pseudo-terminal to a virtual terminal
// emulator. The emulator parses child output into cells, so escape sequences
// from the child never reach the host terminal.
type Session struct {
	cmd           *exec.Cmd
	ptmx          *os.File
	emu           *vt.SafeEmulator
	input         io.Closer
	inputFailure  chan error
	cursorVisible atomic.Bool

	closeOnce sync.Once
	closeErr  error
	waitOnce  sync.Once
	waitErr   error
}

// Start launches cmd on a pseudo-terminal sized to width by height.
// onTrigger receives golf actions asynchronously and may be nil. It is registered
// before output parsing starts, so even an immediate trigger is delivered.
func Start(cmd *exec.Cmd, width, height int, onTrigger func(string)) (*Session, error) {
	if width < 1 || height < 1 {
		return nil, errors.New("pane needs a positive width and height")
	}
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
	if err != nil {
		return nil, err
	}
	s := &Session{
		cmd:          cmd,
		ptmx:         ptmx,
		emu:          vt.NewSafeEmulator(width, height),
		inputFailure: make(chan error, 1),
	}
	s.cursorVisible.Store(true)
	s.emu.SetCallbacks(vt.Callbacks{CursorVisibility: s.cursorVisible.Store})
	// The emulator exposes its io.PipeWriter through InputPipe. Closing that
	// pipe releases input without racing SafeEmulator's unsynchronized Close.
	s.input = s.emu.InputPipe().(io.Closer)
	// Register handlers before any goroutine can write to the emulator.
	// SafeEmulator.RegisterOscHandler has a value receiver, so it is not
	// covered by the emulator's mutex and races the parser otherwise.
	s.emu.RegisterOscHandler(TriggerOSC, func(data []byte) bool {
		_, action, found := strings.Cut(string(data), "golf=")
		if !found {
			return true
		}
		if onTrigger != nil {
			// The handler runs on the output parsing goroutine; never block it.
			go onTrigger(action)
		}
		return true // consumed, so it is never rendered
	})
	s.emu.RegisterOscHandler(clipboardOSC, func([]byte) bool { return true })

	// Child output into the emulator.
	go func() { io.Copy(s.emu, ptmx) }()
	// Drain encoded input promptly before writing it to the child. A live child
	// may stop reading its PTY, but Paste and Render share the emulator mutex,
	// so PTY backpressure must not block this drain or the UI event loop.
	// This drain is not optional and not only for keys we send: the
	// emulator answers the child's own mode queries by writing into this pipe,
	// and a child that queries modes at startup will deadlock the parser if
	// nobody is reading.
	// ponytail: cap at 64 chunks (at most 2 MiB); use byte accounting if small writes fill it.
	queue := make(inputQueue, 64)
	go func() {
		defer s.input.Close()
		defer close(queue)
		if _, err := io.Copy(queue, s.emu); errors.Is(err, errInputBacklog) {
			s.inputFailure <- err
			s.Close()
		}
	}()
	// One PTY writer preserves the order of keys, pastes and emulator replies.
	go func() {
		defer s.input.Close()
		for data := range queue {
			if _, err := ptmx.Write(data); err != nil {
				return
			}
		}
	}()

	return s, nil
}

// SendKey forwards a key press to the child. Printable text takes the direct
// path so shifted characters survive: the emulator emits nothing for a
// modified printable rune, and its Code is unshifted. Modified special keys
// take the explicit encoder for the same reason. Both paths go through the
// emulator so their bytes stay in order.
func (s *Session) SendKey(k tea.KeyPressMsg) {
	send := tea.Key(k)
	if send.Text != "" && send.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		s.emu.SendText(send.Text)
		return
	}
	if b := EncodeModified(send); b != nil {
		s.emu.SendText(string(b))
		return
	}
	s.emu.SendKey(uv.KeyEvent(uv.KeyPressEvent(uv.Key(send))))
}

// Paste preserves the child's bracketed-paste mode and input ordering.
func (s *Session) Paste(text string) { s.emu.Paste(text) }

// Resize updates the emulator and the pseudo-terminal. Resizing the
// pseudo-terminal makes the kernel raise SIGWINCH in the child.
func (s *Session) Resize(width, height int) error {
	if width < 1 || height < 1 {
		return errors.New("pane needs a positive width and height")
	}
	s.emu.Resize(width, height)
	return pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
}

// Render returns the emulator screen with SGR sequences intact. Trailing
// whitespace is trimmed per line, so callers set an explicit width and height
// on the containing style rather than trusting the string's own dimensions.
func (s *Session) Render() string { return s.emu.Render() }

// Cursor reports the child's cursor position in cells and visibility.
func (s *Session) Cursor() (int, int, bool) {
	p := s.emu.CursorPosition()
	return p.X, p.Y, s.cursorVisible.Load()
}

// Wait blocks until the child exits and returns its error.
func (s *Session) Wait() error {
	s.waitOnce.Do(func() {
		s.waitErr = s.cmd.Wait()
		select {
		case err := <-s.inputFailure:
			// The forced process exit is a consequence of the input failure.
			s.waitErr = err
		default:
		}
	})
	return s.waitErr
}

// Close releases the pseudo-terminal and stops a child that is still running.
// It does not wait for the child; call Wait for the exit status. Killing is
// deliberate: closing the master alone does not reliably signal a child that
// never reads from the terminal, which would leave the process orphaned.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		inputErr := s.input.Close()
		if p := s.cmd.Process; p != nil {
			p.Kill()
		}
		s.closeErr = errors.Join(inputErr, s.ptmx.Close())
	})
	return s.closeErr
}
