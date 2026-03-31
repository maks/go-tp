package views

import (
	"go-tp/tv/core"
)

// Button is a clickable widget that dispatches a Command when activated.
type Button struct {
	ViewBase
	Label   string
	Cmd     core.CommandId
	OnPress func()
}

// NewButton creates a Button. Width is label+4 for padding and brackets.
func NewButton(label string, cmd core.CommandId) *Button {
	b := &Button{Label: label, Cmd: cmd}
	b.bounds = core.Rect{W: len([]rune(label)) + 4, H: 1}
	return b
}

func (b *Button) CanFocus() bool { return true }

func (b *Button) Draw(buf *core.DrawBuffer) {
	attr := core.AttrButton
	if b.IsFocused() {
		attr = core.AttrButtonFocused
	}
	text := "[ " + b.Label + " ]"
	buf.MoveStr(0, 0, text, attr)
}

func (b *Button) HandleEvent(ev *core.Event) {
	switch ev.Type {
	case core.EvKeyboard:
		if ev.Key == core.KbEnter || ev.Ch == ' ' {
			b.activate(ev)
		}
	case core.EvMouseDown:
		if b.bounds.Contains(core.Point{X: ev.MouseX, Y: ev.MouseY}) {
			b.activate(ev)
		}
	}
}

func (b *Button) activate(ev *core.Event) {
	ev.Handled = true
	if b.OnPress != nil {
		b.OnPress()
		return
	}
	// Walk up the owner chain to dispatch a Command event.
	var owner View = b.owner
	for owner != nil {
		cmd := core.CommandEvent(b.Cmd)
		owner.HandleEvent(&cmd)
		if cmd.Handled {
			return
		}
		owner = owner.Owner()
	}
}
