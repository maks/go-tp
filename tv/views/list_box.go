package views

import "go-tp/tv/core"

// ListBox displays a scrollable list of strings.
type ListBox struct {
	ViewBase
	Items    []string
	Selected int
	topLine  int
	OnSelect func(int)
}

// NewListBox creates a ListBox.
func NewListBox(bounds core.Rect) *ListBox {
	lb := &ListBox{}
	lb.bounds = bounds
	return lb
}

func (lb *ListBox) CanFocus() bool { return true }

func (lb *ListBox) Draw(buf *core.DrawBuffer) {
	h := lb.bounds.H
	for i := 0; i < h; i++ {
		idx := lb.topLine + i
		attr := core.AttrListBox
		if idx == lb.Selected {
			attr = core.AttrListBoxSelected
		}
		buf.MoveChar(0, i, lb.bounds.W, ' ', attr)
		if idx < len(lb.Items) {
			runes := []rune(lb.Items[idx])
			if len(runes) > lb.bounds.W {
				runes = runes[:lb.bounds.W]
			}
			buf.MoveStr(0, i, string(runes), attr)
		}
	}
}

func (lb *ListBox) HandleEvent(ev *core.Event) {
	if ev.Handled {
		return
	}
	switch ev.Type {
	case core.EvKeyboard:
		switch ev.Key {
		case core.KbUp:
			lb.moveSelect(-1)
			ev.Handled = true
		case core.KbDown:
			lb.moveSelect(1)
			ev.Handled = true
		case core.KbPgUp:
			lb.moveSelect(-lb.bounds.H)
			ev.Handled = true
		case core.KbPgDn:
			lb.moveSelect(lb.bounds.H)
			ev.Handled = true
		case core.KbHome:
			lb.setSelected(0)
			ev.Handled = true
		case core.KbEnd:
			lb.setSelected(len(lb.Items) - 1)
			ev.Handled = true
		}
	case core.EvMouseDown:
		p := core.Point{X: ev.MouseX, Y: ev.MouseY}
		if lb.bounds.Contains(p) {
			row := ev.MouseY - lb.bounds.Y
			lb.setSelected(lb.topLine + row)
			ev.Handled = true
		}
	}
}

func (lb *ListBox) moveSelect(delta int) {
	lb.setSelected(lb.Selected + delta)
}

func (lb *ListBox) setSelected(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(lb.Items) {
		idx = len(lb.Items) - 1
	}
	if idx < 0 {
		idx = 0
	}
	lb.Selected = idx
	// Scroll to keep selection in view.
	if lb.Selected < lb.topLine {
		lb.topLine = lb.Selected
	}
	if lb.Selected >= lb.topLine+lb.bounds.H {
		lb.topLine = lb.Selected - lb.bounds.H + 1
	}
	if lb.OnSelect != nil {
		lb.OnSelect(idx)
	}
}
