package views

import "go-tp/tv/core"

// InputLine is a single-line text input widget.
type InputLine struct {
	ViewBase
	Value   string
	MaxLen  int
	cursor  int // byte index into Value rune slice
	// Called after every change.
	OnChange func(string)
}

// NewInputLine creates an InputLine of the given width.
func NewInputLine(width, maxLen int) *InputLine {
	il := &InputLine{MaxLen: maxLen}
	il.bounds = core.Rect{W: width, H: 1}
	return il
}

func (il *InputLine) CanFocus() bool { return true }

func (il *InputLine) Draw(buf *core.DrawBuffer) {
	attr := core.AttrInput
	if il.IsFocused() {
		attr = core.AttrInputFocused
	}
	w := il.bounds.W
	runes := []rune(il.Value)
	// Show a window of the string that keeps cursor in view.
	start := 0
	if il.cursor > w-1 {
		start = il.cursor - (w - 1)
	}
	for x := 0; x < w; x++ {
		idx := start + x
		var ch rune = ' '
		if idx < len(runes) {
			ch = runes[idx]
		}
		a := attr
		// Highlight cursor position.
		if il.IsFocused() && idx == il.cursor {
			a = attr.Swap()
		}
		if c := buf.At(x, 0); c != nil {
			c.Ch = ch
			c.Attr = a
		}
	}
}

func (il *InputLine) HandleEvent(ev *core.Event) {
	if ev.Type != core.EvKeyboard {
		return
	}
	runes := []rune(il.Value)
	changed := false
	switch ev.Key {
	case core.KbLeft:
		if il.cursor > 0 {
			il.cursor--
		}
		ev.Handled = true
	case core.KbRight:
		if il.cursor < len(runes) {
			il.cursor++
		}
		ev.Handled = true
	case core.KbHome:
		il.cursor = 0
		ev.Handled = true
	case core.KbEnd:
		il.cursor = len(runes)
		ev.Handled = true
	case core.KbBackSpace:
		if il.cursor > 0 {
			runes = append(runes[:il.cursor-1], runes[il.cursor:]...)
			il.cursor--
			il.Value = string(runes)
			changed = true
		}
		ev.Handled = true
	case core.KbDel:
		if il.cursor < len(runes) {
			runes = append(runes[:il.cursor], runes[il.cursor+1:]...)
			il.Value = string(runes)
			changed = true
		}
		ev.Handled = true
	default:
		if ev.Ch >= 32 {
			if il.MaxLen == 0 || len(runes) < il.MaxLen {
				runes = append(runes[:il.cursor], append([]rune{ev.Ch}, runes[il.cursor:]...)...)
				il.cursor++
				il.Value = string(runes)
				changed = true
			}
			ev.Handled = true
		}
	}
	if changed && il.OnChange != nil {
		il.OnChange(il.Value)
	}
}
