package pane

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEncodeModified(t *testing.T) {
	cases := []struct {
		name string
		key  tea.Key
		want string
	}{
		{"unmodified up defers to SendKey", tea.Key{Code: tea.KeyUp}, ""},
		{"printable rune defers", tea.Key{Code: 'a', Text: "a"}, ""},
		{"ctrl+c defers", tea.Key{Code: 'c', Mod: tea.ModCtrl}, ""},
		{"alt rune defers", tea.Key{Code: 'j', Mod: tea.ModAlt, Text: "j"}, ""},
		{"shift+up", tea.Key{Code: tea.KeyUp, Mod: tea.ModShift}, "\x1b[1;2A"},
		{"ctrl+up", tea.Key{Code: tea.KeyUp, Mod: tea.ModCtrl}, "\x1b[1;5A"},
		{"ctrl+shift+left", tea.Key{Code: tea.KeyLeft, Mod: tea.ModCtrl | tea.ModShift}, "\x1b[1;6D"},
		{"alt+down", tea.Key{Code: tea.KeyDown, Mod: tea.ModAlt}, "\x1b[1;3B"},
		{"ctrl+home", tea.Key{Code: tea.KeyHome, Mod: tea.ModCtrl}, "\x1b[1;5H"},
		{"shift+end", tea.Key{Code: tea.KeyEnd, Mod: tea.ModShift}, "\x1b[1;2F"},
		{"ctrl+delete", tea.Key{Code: tea.KeyDelete, Mod: tea.ModCtrl}, "\x1b[3;5~"},
		{"shift+pgup", tea.Key{Code: tea.KeyPgUp, Mod: tea.ModShift}, "\x1b[5;2~"},
		{"ctrl+f5", tea.Key{Code: tea.KeyF5, Mod: tea.ModCtrl}, "\x1b[15;5~"},
		{"shift+f1", tea.Key{Code: tea.KeyF1, Mod: tea.ModShift}, "\x1b[1;2P"},
		{"alt+f12", tea.Key{Code: tea.KeyF12, Mod: tea.ModAlt}, "\x1b[24;3~"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(EncodeModified(c.key)); got != c.want {
				t.Errorf("EncodeModified(%v) = %q, want %q", c.key, got, c.want)
			}
		})
	}
}
