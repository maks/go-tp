package views

import (
	"testing"

	"go-tp/tv/core"
)

// TestGroupAddAndDraw verifies that Group converts relative child bounds to
// absolute and draws children into the correct position.
func TestGroupAddAndDraw(t *testing.T) {
	g := NewGroup(core.Rect{X: 10, Y: 5, W: 20, H: 10})
	lbl := NewLabel("Hello", core.AttrNormal)
	g.Add(lbl, core.Rect{X: 2, Y: 1, W: 5, H: 1})

	if lbl.Bounds().X != 12 || lbl.Bounds().Y != 6 {
		t.Errorf("child bounds = %+v, want X=12 Y=6", lbl.Bounds())
	}

	buf := core.NewDrawBuffer(20, 10, 0)
	g.Draw(buf)
	// "Hello" should appear at (2,1) in the buffer.
	for i, r := range "Hello" {
		c := buf.At(2+i, 1)
		if c == nil || c.Ch != r {
			t.Errorf("buf[%d,1].Ch = %q, want %q", 2+i, c.Ch, r)
		}
	}
}

// TestGroupFocusCycling verifies Tab cycles focus between focusable widgets.
func TestGroupFocusCycling(t *testing.T) {
	g := NewGroup(core.Rect{X: 0, Y: 0, W: 40, H: 10})
	b1 := NewButton("One", core.CmOK)
	b2 := NewButton("Two", core.CmCancel)
	g.Add(b1, core.Rect{X: 0, Y: 0, W: 8, H: 1})
	g.Add(b2, core.Rect{X: 10, Y: 0, W: 8, H: 1})
	g.SetInitialFocus()
	if !b1.IsFocused() {
		t.Fatal("expected b1 to have initial focus")
	}
	ev := core.KeyEvent(core.KbTab, 0)
	g.HandleEvent(&ev)
	if !b2.IsFocused() {
		t.Fatal("expected b2 to have focus after Tab")
	}
	ev = core.KeyEvent(core.KbTab, 0)
	g.HandleEvent(&ev)
	if !b1.IsFocused() {
		t.Fatal("expected b1 to have focus after second Tab")
	}
}

// TestInputLine verifies basic text insertion and backspace.
func TestInputLine(t *testing.T) {
	il := NewInputLine(20, 100)
	il.SetFocused(true)

	type tc struct {
		ev   core.Event
		want string
	}
	cases := []tc{
		{core.KeyEvent(0, 'H'), "H"},
		{core.KeyEvent(0, 'i'), "Hi"},
		{core.KeyEvent(core.KbBackSpace, 0), "H"},
	}
	for _, c := range cases {
		ev := c.ev
		il.HandleEvent(&ev)
		if il.Value != c.want {
			t.Errorf("after %+v: Value = %q, want %q", c.ev, il.Value, c.want)
		}
	}
}

// TestEditorInsertAndBackspace exercises basic editor editing.
func TestEditorInsertAndBackspace(t *testing.T) {
	e := NewEditor(core.Rect{X: 0, Y: 0, W: 40, H: 20})
	e.SetFocused(true)

	for _, r := range "hello" {
		ev := core.KeyEvent(0, r)
		e.HandleEvent(&ev)
	}
	if e.GetText() != "hello" {
		t.Errorf("text = %q, want %q", e.GetText(), "hello")
	}
	// Backspace.
	ev := core.KeyEvent(core.KbBackSpace, 0)
	e.HandleEvent(&ev)
	if e.GetText() != "hell" {
		t.Errorf("after backspace = %q, want %q", e.GetText(), "hell")
	}
}

// TestEditorNewline verifies Enter splits lines.
func TestEditorNewline(t *testing.T) {
	e := NewEditor(core.Rect{X: 0, Y: 0, W: 40, H: 20})
	e.SetFocused(true)
	for _, r := range "ab" {
		ev := core.KeyEvent(0, r)
		e.HandleEvent(&ev)
	}
	ev := core.KeyEvent(core.KbEnter, 0)
	e.HandleEvent(&ev)
	for _, r := range "cd" {
		ev := core.KeyEvent(0, r)
		e.HandleEvent(&ev)
	}
	if e.GetText() != "ab\ncd" {
		t.Errorf("text = %q, want %q", e.GetText(), "ab\ncd")
	}
	if e.LineCount() != 2 {
		t.Errorf("LineCount = %d, want 2", e.LineCount())
	}
}

// TestEditorSetText verifies SetText and GetText round-trip.
func TestEditorSetText(t *testing.T) {
	e := NewEditor(core.Rect{X: 0, Y: 0, W: 40, H: 20})
	src := "line one\nline two\nline three"
	e.SetText(src)
	if e.GetText() != src {
		t.Errorf("GetText = %q, want %q", e.GetText(), src)
	}
	if e.LineCount() != 3 {
		t.Errorf("LineCount = %d, want 3", e.LineCount())
	}
}

// TestListBox verifies arrow-key navigation.
func TestListBox(t *testing.T) {
	lb := NewListBox(core.Rect{X: 0, Y: 0, W: 20, H: 5})
	lb.Items = []string{"alpha", "beta", "gamma", "delta"}
	lb.SetFocused(true)

	ev := core.KeyEvent(core.KbDown, 0)
	lb.HandleEvent(&ev)
	if lb.Selected != 1 {
		t.Errorf("Selected = %d, want 1", lb.Selected)
	}
	ev = core.KeyEvent(core.KbDown, 0)
	lb.HandleEvent(&ev)
	if lb.Selected != 2 {
		t.Errorf("Selected = %d, want 2", lb.Selected)
	}
	ev = core.KeyEvent(core.KbUp, 0)
	lb.HandleEvent(&ev)
	if lb.Selected != 1 {
		t.Errorf("Selected = %d, want 1", lb.Selected)
	}
}

// TestFrameDraw verifies that Frame draws corner and border characters.
func TestFrameDraw(t *testing.T) {
	f := NewFrame(core.Rect{X: 0, Y: 0, W: 10, H: 5}, "Test", FrameDouble)
	buf := core.NewDrawBuffer(10, 5, 0)
	f.Draw(buf)

	// Corners for double frame.
	corners := [][2]int{{0, 0}, {9, 0}, {0, 4}, {9, 4}}
	expected := []rune{'╔', '╗', '╚', '╝'}
	for i, xy := range corners {
		c := buf.At(xy[0], xy[1])
		if c == nil || c.Ch != expected[i] {
			t.Errorf("corner [%d,%d] = %q, want %q", xy[0], xy[1], c.Ch, expected[i])
		}
	}
}
