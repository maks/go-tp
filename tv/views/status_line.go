package views

import "go-tp/tv/core"

// StatusItem is one entry in the status line: a label, hotkey, and command.
type StatusItem struct {
	Label string
	Key   core.KeyCode
	Cmd   core.CommandId
}

// StatusLine draws a row of StatusItems at the bottom of the screen and fires
// Commands when the corresponding key is pressed.
type StatusLine struct {
	ViewBase
	items          []StatusItem
	CommandHandler func(core.CommandId)
}

// NewStatusLine creates a StatusLine.
func NewStatusLine(bounds core.Rect, items []StatusItem) *StatusLine {
	sl := &StatusLine{items: items}
	sl.bounds = bounds
	return sl
}

func (sl *StatusLine) Draw(buf *core.DrawBuffer) {
	w := sl.bounds.W
	buf.MoveChar(0, 0, w, ' ', core.AttrStatusLine)
	x := 0
	for _, item := range sl.items {
		text := item.Label
		runes := []rune(text)
		// Detect "Fnn " or "^Fn " prefix to highlight as a hotkey.
		// Simple approach: first token is the key label.
		// Key part ends at first space.
		spaceIdx := -1
		for i, r := range runes {
			if r == ' ' {
				spaceIdx = i
				break
			}
		}
		if spaceIdx > 0 {
			keyPart := string(runes[:spaceIdx])
			descPart := string(runes[spaceIdx:])
			buf.MoveStr(x, 0, keyPart, core.AttrStatusKey)
			x += len([]rune(keyPart))
			buf.MoveStr(x, 0, descPart, core.AttrStatusLine)
			x += len([]rune(descPart))
		} else {
			buf.MoveStr(x, 0, text, core.AttrStatusLine)
			x += len(runes)
		}
		buf.MoveStr(x, 0, " ", core.AttrStatusLine)
		x++
	}
}

func (sl *StatusLine) HandleEvent(ev *core.Event) {
	if ev.Type != core.EvKeyboard {
		return
	}
	for _, item := range sl.items {
		if item.Key != 0 && ev.Key == item.Key {
			ev.Handled = true
			if sl.CommandHandler != nil {
				sl.CommandHandler(item.Cmd)
			}
			return
		}
	}
}
