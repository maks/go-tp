package views

import (
	"strings"

	"go-tp/tv/core"
)

// MenuItem represents one entry in a menu. Items with Cmd==0 are separators.
type MenuItem struct {
	Label   string
	Cmd     core.CommandId
	HotKey  core.KeyCode // keyboard shortcut (e.g. KbF9)
	HotText string       // display text for the hotkey (e.g. "F9")
	// SubMenu is non-nil if this item opens a nested menu.
	SubMenu []*MenuItem
}

// Sep returns a separator MenuItem.
func Sep() *MenuItem { return &MenuItem{} }

// MenuBar draws the top-level menu bar and manages popup menu boxes.
type MenuBar struct {
	ViewBase
	items     []*MenuItem
	selected  int  // index of selected top-level item (-1 = none)
	open      bool // whether a popup is currently open
	popup     *MenuBox
	// CommandHandler is called when a menu command is chosen.
	CommandHandler func(core.CommandId)
}

// NewMenuBar creates a MenuBar. bounds must be 1 row tall.
func NewMenuBar(bounds core.Rect, items []*MenuItem) *MenuBar {
	mb := &MenuBar{items: items, selected: -1}
	mb.bounds = bounds
	return mb
}

func (mb *MenuBar) CanFocus() bool { return true }

func (mb *MenuBar) Draw(buf *core.DrawBuffer) {
	w := mb.bounds.W
	buf.MoveChar(0, 0, w, ' ', core.AttrMenuBar)
	x := 1
	for i, item := range mb.items {
		attr := core.AttrMenuBar
		if i == mb.selected {
			attr = core.AttrMenuBarSelected
		}
		label := " " + item.Label + " "
		buf.MoveStr(x, 0, label, attr)
		x += len([]rune(label))
	}
}

func (mb *MenuBar) HandleEvent(ev *core.Event) {
	if ev.Handled {
		return
	}
	// While a popup is open, route events to it first.
	if mb.open && mb.popup != nil {
		mb.popup.HandleEvent(ev)
		if mb.popup.chosen != nil {
			cmd := mb.popup.chosen.Cmd
			mb.closePopup()
			if cmd != 0 && mb.CommandHandler != nil {
				mb.CommandHandler(cmd)
			}
			ev.Handled = true
			return
		}
		if mb.popup.cancelled {
			mb.closePopup()
			ev.Handled = true
			return
		}
		if ev.Handled {
			return
		}
	}

	switch ev.Type {
	case core.EvKeyboard:
		if mb.open {
			switch ev.Key {
			case core.KbLeft:
				mb.selected = (mb.selected - 1 + len(mb.items)) % len(mb.items)
				mb.openPopup()
				ev.Handled = true
			case core.KbRight:
				mb.selected = (mb.selected + 1) % len(mb.items)
				mb.openPopup()
				ev.Handled = true
			case core.KbEsc:
				mb.closePopup()
				ev.Handled = true
			}
			return
		}
		// Alt+letter opens matching menu.
		if ev.Key.IsAlt() {
			letter := ev.Key.AltLetter()
			for i, item := range mb.items {
				if hotLetter(item.Label) == letter {
					mb.selected = i
					mb.openPopup()
					ev.Handled = true
					return
				}
			}
		}
	case core.EvMouseDown:
		p := core.Point{X: ev.MouseX, Y: ev.MouseY}
		if mb.bounds.Contains(p) {
			x := 1
			for i, item := range mb.items {
				label := " " + item.Label + " "
				w := len([]rune(label))
				if ev.MouseX >= mb.bounds.X+x && ev.MouseX < mb.bounds.X+x+w {
					if mb.selected == i && mb.open {
						mb.closePopup()
					} else {
						mb.selected = i
						mb.openPopup()
					}
					ev.Handled = true
					return
				}
				x += w
			}
		} else if mb.open {
			// Click outside — let popup handle it, or close.
			if mb.popup != nil {
				mb.popup.HandleEvent(ev)
				if mb.popup.chosen != nil {
					cmd := mb.popup.chosen.Cmd
					mb.closePopup()
					if cmd != 0 && mb.CommandHandler != nil {
						mb.CommandHandler(cmd)
					}
					ev.Handled = true
					return
				}
			}
			mb.closePopup()
		}
	}
}

func (mb *MenuBar) openPopup() {
	if mb.selected < 0 || mb.selected >= len(mb.items) {
		return
	}
	item := mb.items[mb.selected]
	if len(item.SubMenu) == 0 {
		return
	}
	// Compute popup position: below the selected item on the menu bar.
	x := mb.bounds.X + 1
	for i := 0; i < mb.selected; i++ {
		x += len([]rune(" "+mb.items[i].Label+" "))
	}
	popupBounds := core.Rect{
		X: x,
		Y: mb.bounds.Y + 1,
		W: menuBoxWidth(item.SubMenu),
		H: len(item.SubMenu) + 2,
	}
	mb.popup = NewMenuBox(popupBounds, item.SubMenu)
	mb.open = true
}

func (mb *MenuBar) closePopup() {
	mb.popup = nil
	mb.open = false
	mb.selected = -1
}

// DrawPopup draws the active popup menu box into the desktop buffer.
// Call this from Application after drawing the desktop.
func (mb *MenuBar) DrawPopup(buf *core.DrawBuffer, offsetX, offsetY int) {
	if !mb.open || mb.popup == nil {
		return
	}
	b := mb.popup.bounds
	sub := core.NewDrawBuffer(b.W, b.H, core.AttrMenuBox)
	mb.popup.Draw(sub)
	buf.CopyFrom(sub, b.X-offsetX, b.Y-offsetY)
}

// hotLetter returns the hot letter from a label. By convention the first letter
// is the hot letter (lower-cased).
func hotLetter(label string) rune {
	label = strings.TrimSpace(label)
	if len(label) == 0 {
		return 0
	}
	for _, r := range label {
		return r + 32 // to lower
	}
	return 0
}

// menuBoxWidth computes the minimum width of a popup box for the given items.
func menuBoxWidth(items []*MenuItem) int {
	w := 4 // 2 border + 2 padding
	for _, item := range items {
		if item.Cmd == 0 {
			continue
		}
		n := len([]rune(item.Label))
		if item.HotText != "" {
			n += len([]rune(item.HotText)) + 2
		}
		if n > w-4 {
			w = n + 4
		}
	}
	if w < 16 {
		w = 16
	}
	return w
}

// MenuBox is a floating popup list of MenuItems.
type MenuBox struct {
	ViewBase
	items     []*MenuItem
	selected  int
	chosen    *MenuItem // set when user activates an item
	cancelled bool
}

// NewMenuBox creates a MenuBox. bounds must include the border.
func NewMenuBox(bounds core.Rect, items []*MenuItem) *MenuBox {
	mb := &MenuBox{items: items, selected: firstSelectable(items)}
	mb.bounds = bounds
	return mb
}

func (mb *MenuBox) Draw(buf *core.DrawBuffer) {
	w, h := mb.bounds.W, mb.bounds.H
	// Frame.
	frame := NewFrame(core.Rect{W: w, H: h}, "", FrameSingle)
	frame.AttrActive = core.AttrMenuBox
	frame.AttrInactive = core.AttrMenuBox
	frame.Draw(buf)
	// Items.
	for i, item := range mb.items {
		y := i + 1
		if y >= h-1 {
			break
		}
		if item.Cmd == 0 {
			// Separator.
			buf.MoveChar(1, y, w-2, '─', core.AttrMenuBox)
			if c := buf.At(0, y); c != nil {
				c.Ch = '├'
			}
			if c := buf.At(w-1, y); c != nil {
				c.Ch = '┤'
			}
			continue
		}
		attr := core.AttrMenuBox
		if i == mb.selected {
			attr = core.AttrMenuBoxSelected
		}
		buf.MoveChar(1, y, w-2, ' ', attr)
		buf.MoveStr(2, y, item.Label, attr)
		if item.HotText != "" {
			hotStart := w - 2 - len([]rune(item.HotText))
			if hotStart > 2 {
				buf.MoveStr(hotStart, y, item.HotText, attr)
			}
		}
	}
}

func (mb *MenuBox) HandleEvent(ev *core.Event) {
	if ev.Handled {
		return
	}
	switch ev.Type {
	case core.EvKeyboard:
		switch ev.Key {
		case core.KbUp:
			mb.moveSel(-1)
			ev.Handled = true
		case core.KbDown:
			mb.moveSel(1)
			ev.Handled = true
		case core.KbEnter:
			mb.activate()
			ev.Handled = true
		case core.KbEsc:
			mb.cancelled = true
			ev.Handled = true
		}
	case core.EvMouseDown:
		p := core.Point{X: ev.MouseX, Y: ev.MouseY}
		if mb.bounds.Contains(p) {
			row := ev.MouseY - mb.bounds.Y - 1
			if row >= 0 && row < len(mb.items) {
				mb.selected = row
				mb.activate()
			}
			ev.Handled = true
		} else {
			mb.cancelled = true
			ev.Handled = true
		}
	}
}

func (mb *MenuBox) moveSel(delta int) {
	n := len(mb.items)
	for i := 1; i <= n; i++ {
		next := (mb.selected + delta*i + n*i) % n
		if mb.items[next].Cmd != 0 {
			mb.selected = next
			return
		}
	}
}

func (mb *MenuBox) activate() {
	if mb.selected >= 0 && mb.selected < len(mb.items) {
		item := mb.items[mb.selected]
		if item.Cmd != 0 {
			mb.chosen = item
		}
	}
}

func firstSelectable(items []*MenuItem) int {
	for i, item := range items {
		if item.Cmd != 0 {
			return i
		}
	}
	return 0
}
