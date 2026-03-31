package views

import "go-tp/tv/core"

// Window is a draggable bordered window with an interior Group for content.
// The frame occupies the outer 1-cell border; the interior starts at (1,1).
type Window struct {
	ViewBase
	frame   *Frame
	interior *Group
	title   string
	// drag state
	dragging  bool
	dragStartMouseX, dragStartMouseY int
	dragStartBoundsX, dragStartBoundsY int
}

// NewWindow creates a Window with the given bounds and title.
func NewWindow(bounds core.Rect, title string) *Window {
	w := &Window{title: title}
	w.bounds = bounds
	w.frame = NewFrame(bounds, title, FrameDouble)
	// Interior is bounds shrunk by 1 on all sides.
	interior := core.Rect{X: 1, Y: 1, W: bounds.W - 2, H: bounds.H - 2}
	w.interior = NewGroup(core.Rect{
		X: bounds.X + 1,
		Y: bounds.Y + 1,
		W: bounds.W - 2,
		H: bounds.H - 2,
	})
	_ = interior
	return w
}

// Interior returns the Group that holds the window's content views.
func (w *Window) Interior() *Group { return w.interior }

// Add adds a child view to the window interior using relative coordinates.
func (w *Window) Add(child View, rel core.Rect) {
	w.interior.Add(child, rel)
}

// SetBounds repositions the window and updates its frame + interior.
func (w *Window) SetBounds(r core.Rect) {
	w.bounds = r
	w.frame.SetBounds(r)
	w.interior.SetBounds(core.Rect{
		X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2,
	})
	// Reposition all children of interior with updated offsets.
	// (Children store absolute positions; update them.)
	for _, child := range w.interior.Children() {
		cb := child.Bounds()
		// Compute relative position from old interior.
		relX := cb.X - (w.interior.Bounds().X)
		relY := cb.Y - (w.interior.Bounds().Y)
		child.SetBounds(core.Rect{
			X: r.X + 1 + relX,
			Y: r.Y + 1 + relY,
			W: cb.W,
			H: cb.H,
		})
	}
}

// CanFocus allows the window itself to receive focus (for z-ordering).
func (w *Window) CanFocus() bool { return true }

func (w *Window) Draw(buf *core.DrawBuffer) {
	// Fill interior with normal attr.
	for y := 1; y < w.bounds.H-1; y++ {
		buf.MoveChar(1, y, w.bounds.W-2, ' ', core.AttrNormal)
	}
	// Draw frame.
	w.frame.SetBounds(w.bounds)
	w.frame.Draw(buf)
	// Draw children.
	for _, child := range w.interior.Children() {
		cb := child.Bounds()
		// Convert absolute child bounds to relative within buf.
		relX := cb.X - w.bounds.X
		relY := cb.Y - w.bounds.Y
		sub := core.NewDrawBuffer(cb.W, cb.H, core.AttrNormal)
		child.Draw(sub)
		buf.CopyFrom(sub, relX, relY)
	}
}

func (w *Window) HandleEvent(ev *core.Event) {
	if ev.Handled {
		return
	}
	switch ev.Type {
	case core.EvMouseDown:
		p := core.Point{X: ev.MouseX, Y: ev.MouseY}
		if !w.bounds.Contains(p) {
			return
		}
		// Click on title bar (row 0 of bounds) → start drag.
		if ev.MouseY == w.bounds.Y {
			w.dragging = true
			w.dragStartMouseX = ev.MouseX
			w.dragStartMouseY = ev.MouseY
			w.dragStartBoundsX = w.bounds.X
			w.dragStartBoundsY = w.bounds.Y
			ev.Handled = true
			return
		}
		// Click inside interior → let children handle it.
		w.interior.HandleEvent(ev)

	case core.EvMouseMove:
		if w.dragging {
			dx := ev.MouseX - w.dragStartMouseX
			dy := ev.MouseY - w.dragStartMouseY
			newBounds := w.bounds
			newBounds.X = w.dragStartBoundsX + dx
			newBounds.Y = w.dragStartBoundsY + dy
			w.SetBounds(newBounds)
			ev.Handled = true
			return
		}
		w.interior.HandleEvent(ev)

	case core.EvMouseUp:
		w.dragging = false
		w.interior.HandleEvent(ev)

	case core.EvKeyboard:
		w.interior.HandleEvent(ev)

	case core.EvCommand, core.EvBroadcast:
		w.interior.HandleEvent(ev)
	}
}

// SetInitialFocus sets initial focus inside the window.
func (w *Window) SetInitialFocus() {
	w.interior.SetInitialFocus()
}

// SetTitle updates the window title in the frame border.
func (w *Window) SetTitle(title string) {
	w.title = title
	w.frame.Title = title
}

// SetActive sets the frame to active/inactive (affects color).
func (w *Window) SetActive(a bool) {
	w.frame.SetActive(a)
}

// Shadow draws a 1-cell drop shadow to the right and bottom of the window.
func (w *Window) DrawShadow(buf *core.DrawBuffer, offsetX, offsetY int) {
	b := w.bounds
	// Right column shadow.
	for y := b.Y + 1; y < b.Y+b.H; y++ {
		x := b.X + b.W - offsetX
		if c := buf.At(x, y-offsetY); c != nil {
			c.Attr = core.AttrShadow
		}
	}
	// Bottom row shadow.
	for x := b.X + 1; x < b.X+b.W+1; x++ {
		y := b.Y + b.H - offsetY
		if c := buf.At(x-offsetX, y); c != nil {
			c.Attr = core.AttrShadow
		}
	}
}
