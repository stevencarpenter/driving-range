// Package pane runs an exercise child inside a pseudo-terminal and a virtual
// terminal emulator so the TUI can keep rendering around it.
package pane

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// xterm modifier parameter bits. The encoded parameter is 1 plus their sum.
const (
	bitShift = 1
	bitAlt   = 2
	bitCtrl  = 4
)

// special maps a key code to its xterm form: either a CSI final letter or a
// numeric parameter used with a tilde final byte.
type special struct {
	letter byte
	tilde  int
}

var specials = map[rune]special{
	tea.KeyUp:    {letter: 'A'},
	tea.KeyDown:  {letter: 'B'},
	tea.KeyRight: {letter: 'C'},
	tea.KeyLeft:  {letter: 'D'},
	tea.KeyHome:  {letter: 'H'},
	tea.KeyEnd:   {letter: 'F'},

	tea.KeyF1: {letter: 'P'},
	tea.KeyF2: {letter: 'Q'},
	tea.KeyF3: {letter: 'R'},
	tea.KeyF4: {letter: 'S'},

	tea.KeyInsert: {tilde: 2},
	tea.KeyDelete: {tilde: 3},
	tea.KeyPgUp:   {tilde: 5},
	tea.KeyPgDown: {tilde: 6},

	tea.KeyF5:  {tilde: 15},
	tea.KeyF6:  {tilde: 17},
	tea.KeyF7:  {tilde: 18},
	tea.KeyF8:  {tilde: 19},
	tea.KeyF9:  {tilde: 20},
	tea.KeyF10: {tilde: 21},
	tea.KeyF11: {tilde: 23},
	tea.KeyF12: {tilde: 24},
}

// EncodeModified returns the xterm sequence for a special key held with a
// modifier. It returns nil when the emulator's own SendKey already covers the
// event, which is every unmodified key and every modified printable rune.
//
// The emulator needs this because its SendKey is an exhaustive switch on exact
// code and modifier pairs whose default branch handles only an empty modifier,
// so a modified special key would otherwise produce no bytes at all.
func EncodeModified(k tea.Key) []byte {
	s, ok := specials[k.Code]
	if !ok {
		return nil
	}
	var bits int
	if k.Mod&tea.ModShift != 0 {
		bits |= bitShift
	}
	if k.Mod&tea.ModAlt != 0 {
		bits |= bitAlt
	}
	if k.Mod&tea.ModCtrl != 0 {
		bits |= bitCtrl
	}
	if bits == 0 {
		return nil
	}
	if s.letter != 0 {
		return fmt.Appendf(nil, "\x1b[1;%d%c", bits+1, s.letter)
	}
	return fmt.Appendf(nil, "\x1b[%d;%d~", s.tilde, bits+1)
}
