package core

import "testing"

func TestDrawBufferMoveStr(t *testing.T) {
	buf := NewDrawBuffer(10, 5, 0)
	buf.MoveStr(2, 1, "hello", MakeAttr(ColorWhite, ColorBlue))
	for i, r := range "hello" {
		c := buf.At(2+i, 1)
		if c == nil {
			t.Fatalf("At(%d,1) is nil", 2+i)
		}
		if c.Ch != r {
			t.Errorf("At(%d,1).Ch = %q, want %q", 2+i, c.Ch, r)
		}
	}
}

func TestDrawBufferMoveChar(t *testing.T) {
	buf := NewDrawBuffer(10, 5, 0)
	buf.MoveChar(0, 0, 10, '░', MakeAttr(ColorBlue, ColorBlue))
	for x := 0; x < 10; x++ {
		c := buf.At(x, 0)
		if c == nil || c.Ch != '░' {
			t.Errorf("At(%d,0).Ch = %q, want '░'", x, c.Ch)
		}
	}
}

func TestAttrFgBg(t *testing.T) {
	a := MakeAttr(ColorLightCyan, ColorBlue)
	if a.Fg() != ColorLightCyan {
		t.Errorf("Fg = %d, want %d", a.Fg(), ColorLightCyan)
	}
	if a.Bg() != ColorBlue {
		t.Errorf("Bg = %d, want %d", a.Bg(), ColorBlue)
	}
}

func TestAttrSwap(t *testing.T) {
	a := MakeAttr(ColorWhite, ColorBlue)
	s := a.Swap()
	if s.Fg() != ColorBlue || s.Bg() != ColorWhite {
		t.Errorf("Swap: fg=%d bg=%d", s.Fg(), s.Bg())
	}
}

func TestCopyFrom(t *testing.T) {
	dst := NewDrawBuffer(10, 5, 0)
	src := NewDrawBuffer(3, 2, MakeAttr(ColorRed, ColorGreen))
	src.MoveStr(0, 0, "ABC", MakeAttr(ColorRed, ColorGreen))
	src.MoveStr(0, 1, "DEF", MakeAttr(ColorRed, ColorGreen))
	dst.CopyFrom(src, 2, 1)
	for i, r := range "ABC" {
		c := dst.At(2+i, 1)
		if c.Ch != r {
			t.Errorf("dst[%d,1] = %q, want %q", 2+i, c.Ch, r)
		}
	}
}
