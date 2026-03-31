package core

import "testing"

func TestVT100ParserArrows(t *testing.T) {
	p := &VT100Parser{}
	tests := []struct {
		input []byte
		want  KeyCode
	}{
		{[]byte{0x1B, '[', 'A'}, KbUp},
		{[]byte{0x1B, '[', 'B'}, KbDown},
		{[]byte{0x1B, '[', 'C'}, KbRight},
		{[]byte{0x1B, '[', 'D'}, KbLeft},
	}
	for _, tc := range tests {
		for i, b := range tc.input {
			ev := p.Feed(b)
			if i < len(tc.input)-1 {
				if ev.Type != EvNothing {
					t.Errorf("unexpected event mid-sequence: %+v", ev)
				}
			} else {
				if ev.Key != tc.want {
					t.Errorf("input %v: Key = %v, want %v", tc.input, ev.Key, tc.want)
				}
			}
		}
	}
}

func TestVT100ParserPrintable(t *testing.T) {
	p := &VT100Parser{}
	ev := p.Feed('A')
	if ev.Type != EvKeyboard || ev.Ch != 'A' {
		t.Errorf("expected printable 'A', got %+v", ev)
	}
}

func TestVT100ParserEnter(t *testing.T) {
	p := &VT100Parser{}
	ev := p.Feed(0x0D)
	if ev.Key != KbEnter {
		t.Errorf("expected Enter, got %+v", ev)
	}
}

func TestVT100ParserBackspace(t *testing.T) {
	p := &VT100Parser{}
	ev := p.Feed(0x7F)
	if ev.Key != KbBackSpace {
		t.Errorf("expected Backspace, got %+v", ev)
	}
}

func TestVT100ParserF1(t *testing.T) {
	p := &VT100Parser{}
	// ESC O P = F1 (SS3 form)
	seq := []byte{0x1B, 'O', 'P'}
	var ev Event
	for _, b := range seq {
		ev = p.Feed(b)
	}
	if ev.Key != KbF1 {
		t.Errorf("F1 (SS3): Key = %v, want KbF1", ev.Key)
	}
}

func TestVT100ParserDelete(t *testing.T) {
	p := &VT100Parser{}
	// ESC [ 3 ~
	seq := []byte{0x1B, '[', '3', '~'}
	var ev Event
	for _, b := range seq {
		ev = p.Feed(b)
	}
	if ev.Key != KbDel {
		t.Errorf("Delete: Key = %v, want KbDel", ev.Key)
	}
}

func TestVT100ParserAltA(t *testing.T) {
	p := &VT100Parser{}
	// ESC a
	seq := []byte{0x1B, 'a'}
	var ev Event
	for _, b := range seq {
		ev = p.Feed(b)
	}
	if ev.Key != KbAltA {
		t.Errorf("Alt+A: Key = %v, want KbAltA", ev.Key)
	}
}

func TestVT100ParserBareEsc(t *testing.T) {
	p := &VT100Parser{}
	p.Feed(0x1B)
	ev := p.Flush()
	if ev.Key != KbEsc {
		t.Errorf("bare ESC flush: Key = %v, want KbEsc", ev.Key)
	}
}
