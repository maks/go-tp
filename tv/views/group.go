package views

import "go-tp/tv/core"

// Group is a container for child views. It manages focus and event routing.
type Group struct {
	ViewBase
	children []View
}

// NewGroup creates an empty Group with the given bounds.
func NewGroup(bounds core.Rect) *Group {
	g := &Group{}
	g.bounds = bounds
	return g
}

// Add appends child to the group. rel is the child's position relative to the
// group's top-left corner; it is converted to absolute screen coordinates.
func (g *Group) Add(child View, rel core.Rect) {
	abs := core.Rect{
		X: g.bounds.X + rel.X,
		Y: g.bounds.Y + rel.Y,
		W: rel.W,
		H: rel.H,
	}
	child.SetBounds(abs)
	child.SetOwner(g)
	g.children = append(g.children, child)
}

// Children returns the child slice (read-only).
func (g *Group) Children() []View { return g.children }

// Draw draws all children in order (first child is drawn first — at the bottom).
func (g *Group) Draw(buf *core.DrawBuffer) {
	for _, child := range g.children {
		b := child.Bounds()
		sub := core.NewDrawBuffer(b.W, b.H, 0)
		child.Draw(sub)
		buf.CopyFrom(sub, b.X-g.bounds.X, b.Y-g.bounds.Y)
	}
}

// HandleEvent routes the event. Focused child gets first crack; unfocused
// children receive broadcasts; commands propagate to all.
func (g *Group) HandleEvent(ev *core.Event) {
	if ev.Handled {
		return
	}
	// Focused child handles keyboard events first.
	if ev.Type == core.EvKeyboard {
		for _, child := range g.children {
			if child.IsFocused() {
				child.HandleEvent(ev)
				if ev.Handled {
					return
				}
			}
		}
		// Tab/Shift+Tab cycle focus.
		if ev.Key == core.KbTab {
			g.focusNext()
			ev.Handled = true
			return
		}
		if ev.Key == core.KbShiftTab {
			g.focusPrev()
			ev.Handled = true
			return
		}
	}

	// Mouse events: deliver to the child under the cursor.
	if ev.Type == core.EvMouseDown || ev.Type == core.EvMouseUp || ev.Type == core.EvMouseMove {
		p := core.Point{X: ev.MouseX, Y: ev.MouseY}
		for i := len(g.children) - 1; i >= 0; i-- {
			child := g.children[i]
			if child.Bounds().Contains(p) {
				if ev.Type == core.EvMouseDown && child.CanFocus() {
					g.focusChild(child)
				}
				child.HandleEvent(ev)
				if ev.Handled {
					return
				}
				break
			}
		}
		return
	}

	// Commands and broadcasts reach all children.
	for _, child := range g.children {
		child.HandleEvent(ev)
		if ev.Handled && ev.Type != core.EvBroadcast {
			return
		}
	}
}

// SetInitialFocus focuses the first focusable child.
func (g *Group) SetInitialFocus() {
	for _, child := range g.children {
		if child.CanFocus() {
			g.focusChild(child)
			return
		}
	}
}

func (g *Group) focusChild(target View) {
	for _, child := range g.children {
		if child == target {
			child.SetFocused(true)
		} else {
			child.SetFocused(false)
		}
	}
}

func (g *Group) focusNext() {
	idx := g.focusedIndex()
	n := len(g.children)
	for i := 1; i < n; i++ {
		next := g.children[(idx+i)%n]
		if next.CanFocus() {
			g.focusChild(next)
			return
		}
	}
}

func (g *Group) focusPrev() {
	idx := g.focusedIndex()
	n := len(g.children)
	for i := 1; i < n; i++ {
		prev := g.children[(idx-i+n)%n]
		if prev.CanFocus() {
			g.focusChild(prev)
			return
		}
	}
}

func (g *Group) focusedIndex() int {
	for i, child := range g.children {
		if child.IsFocused() {
			return i
		}
	}
	return 0
}
