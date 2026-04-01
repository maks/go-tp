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

// DrawPopup draws the active popup menu box (and any nested child menus)
// into the full application buffer. Call this after drawing the desktop.
func (mb *MenuBar) DrawPopup(buf *core.DrawBuffer, offsetX, offsetY int) {
	if !mb.open || mb.popup == nil {
		return
	}
	b := mb.popup.bounds
	sub := core.NewDrawBuffer(b.W, b.H, core.AttrMenuBox)
	mb.popup.Draw(sub)
	buf.CopyFrom(sub, b.X-offsetX, b.Y-offsetY)
	// Draw any open nested child popups directly into the full buffer.
	mb.popup.DrawChildPopup(buf)
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
		if item.Cmd == 0 && item.SubMenu == nil {
			continue // pure separator
		}
		n := len([]rune(item.Label))
		if item.HotText != "" {
			n += len([]rune(item.HotText)) + 2
		}
		if item.SubMenu != nil {
			n += 2 // " ▸" indicator
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
	chosen    *MenuItem // set when user activates a leaf item
	cancelled bool
	child     *MenuBox  // open nested submenu, or nil
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
		if item.Cmd == 0 && item.SubMenu == nil {
			// Pure separator.
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
		if item.SubMenu != nil {
			// Right-aligned arrow indicator for submenus.
			buf.MoveStr(w-3, y, "▸", attr)
		} else if item.HotText != "" {
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

	// Route to child submenu first.
	if mb.child != nil {
		mb.child.HandleEvent(ev)
		if mb.child.chosen != nil {
			mb.chosen = mb.child.chosen // bubble up
			mb.child = nil
			return
		}
		if mb.child.cancelled {
			mb.child = nil
			ev.Handled = true
			return
		}
		if ev.Handled {
			return
		}
	}

	switch ev.Type {
	case core.EvKeyboard:
		switch ev.Key {
		case core.KbUp:
			mb.moveSel(-1)
			mb.child = nil
			ev.Handled = true
		case core.KbDown:
			mb.moveSel(1)
			mb.child = nil
			ev.Handled = true
		case core.KbRight, core.KbEnter:
			mb.activate()
			ev.Handled = true
		case core.KbLeft:
			if mb.child != nil {
				mb.child = nil
			} else {
				mb.cancelled = true
			}
			ev.Handled = true
		case core.KbEsc:
			mb.child = nil
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
		} else if mb.child != nil && mb.child.bounds.Contains(p) {
			// click inside child — already routed above
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
		item := mb.items[next]
		if item.Cmd != 0 || item.SubMenu != nil {
			mb.selected = next
			return
		}
	}
}

func (mb *MenuBox) activate() {
	if mb.selected < 0 || mb.selected >= len(mb.items) {
		return
	}
	item := mb.items[mb.selected]
	if item.SubMenu != nil {
		mb.openChild(item)
	} else if item.Cmd != 0 {
		mb.chosen = item
	}
}

// openChild opens a nested MenuBox for item to the right of this box.
func (mb *MenuBox) openChild(item *MenuItem) {
	childW := menuBoxWidth(item.SubMenu)
	childH := len(item.SubMenu) + 2
	// Position: right edge of parent, aligned to the selected row.
	x := mb.bounds.X + mb.bounds.W
	y := mb.bounds.Y + mb.selected + 1 // +1 for border row
	mb.child = NewMenuBox(core.Rect{X: x, Y: y, W: childW, H: childH}, item.SubMenu)
}

// DrawChildPopup draws any open child (and grandchildren) into the full
// application buffer. Called by MenuBar.DrawPopup after drawing the root popup.
func (mb *MenuBox) DrawChildPopup(buf *core.DrawBuffer) {
	if mb.child == nil {
		return
	}
	b := mb.child.bounds
	sub := core.NewDrawBuffer(b.W, b.H, core.AttrMenuBox)
	mb.child.Draw(sub)
	buf.CopyFrom(sub, b.X, b.Y)
	mb.child.DrawChildPopup(buf) // recurse for deeper nesting
}

func firstSelectable(items []*MenuItem) int {
	for i, item := range items {
		if item.Cmd != 0 || item.SubMenu != nil {
			return i
		}
	}
	return 0
}
