package views

import "go-tp/tv/core"

// Desktop is the root view manager. It maintains a z-ordered stack of windows
// and draws a tiled background. Events are routed to the top-most window first.
type Desktop struct {
	ViewBase
	windows []*Window
	bgChar  rune
	bgAttr  core.Attr
}

// NewDesktop creates a Desktop covering the given bounds.
func NewDesktop(bounds core.Rect) *Desktop {
	d := &Desktop{
		bgChar: '░',
		bgAttr: core.AttrDesktop,
	}
	d.bounds = bounds
	return d
}

// AddWindow pushes win on top of the z-stack.
func (d *Desktop) AddWindow(win *Window) {
	d.windows = append(d.windows, win)
}

// BringToFront moves win to the top of the z-stack and deactivates others.
func (d *Desktop) BringToFront(win *Window) {
	for i, w := range d.windows {
		if w == win {
			d.windows = append(d.windows[:i], d.windows[i+1:]...)
			d.windows = append(d.windows, win)
			break
		}
	}
	d.updateActive()
}

// RemoveWindow removes win from the z-stack.
func (d *Desktop) RemoveWindow(win *Window) {
	for i, w := range d.windows {
		if w == win {
			d.windows = append(d.windows[:i], d.windows[i+1:]...)
			break
		}
	}
	d.updateActive()
}

func (d *Desktop) updateActive() {
	for i, w := range d.windows {
		w.SetActive(i == len(d.windows)-1)
	}
}

// Draw renders the background and all windows bottom-to-top.
func (d *Desktop) Draw(buf *core.DrawBuffer) {
	// Background.
	for y := 0; y < d.bounds.H; y++ {
		buf.MoveChar(0, y, d.bounds.W, d.bgChar, d.bgAttr)
	}
	// Windows bottom-to-top.
	for _, win := range d.windows {
		b := win.Bounds()
		if b.IsEmpty() {
			continue
		}
		sub := core.NewDrawBuffer(b.W, b.H, core.AttrNormal)
		win.Draw(sub)
		buf.CopyFrom(sub, b.X-d.bounds.X, b.Y-d.bounds.Y)
		// Shadow — drawn after window, modifies cells already in buf.
		drawShadow(buf, b, d.bounds)
	}
}

// HandleEvent routes to the top window first, then broadcasts.
func (d *Desktop) HandleEvent(ev *core.Event) {
	if ev.Handled {
		return
	}
	n := len(d.windows)
	if n == 0 {
		return
	}

	switch ev.Type {
	case core.EvMouseDown:
		p := core.Point{X: ev.MouseX, Y: ev.MouseY}
		for i := n - 1; i >= 0; i-- {
			win := d.windows[i]
			if win.Bounds().Contains(p) {
				if i != n-1 {
					d.BringToFront(win)
				}
				win.HandleEvent(ev)
				return
			}
		}
	case core.EvMouseMove, core.EvMouseUp:
		if n > 0 {
			d.windows[n-1].HandleEvent(ev)
		}
	case core.EvKeyboard:
		if n > 0 {
			d.windows[n-1].HandleEvent(ev)
		}
	case core.EvCommand, core.EvBroadcast:
		for i := n - 1; i >= 0; i-- {
			d.windows[i].HandleEvent(ev)
			if ev.Handled && ev.Type != core.EvBroadcast {
				return
			}
		}
	}
}

// drawShadow adds a 1-cell drop shadow to the right and bottom of winBounds.
func drawShadow(buf *core.DrawBuffer, winBounds, desktop core.Rect) {
	shadowAttr := core.AttrShadow
	// Right column.
	sx := winBounds.X + winBounds.W - desktop.X
	for y := winBounds.Y + 1 - desktop.Y; y < winBounds.Y+winBounds.H+1-desktop.Y; y++ {
		if c := buf.At(sx, y); c != nil {
			c.Attr = shadowAttr
		}
	}
	// Bottom row.
	sy := winBounds.Y + winBounds.H - desktop.Y
	for x := winBounds.X + 1 - desktop.X; x < winBounds.X+winBounds.W+1-desktop.X; x++ {
		if c := buf.At(x, sy); c != nil {
			c.Attr = shadowAttr
		}
	}
}

// ExecView runs a nested modal event loop for d (a dialog), blocking until
// the dialog closes. The caller supplies the polling function.
func (d *Desktop) ExecView(dialog *Dialog, poll func() *core.Event) {
	d.AddWindow(dialog.Window)
	dialog.Window.SetInitialFocus()
	defer d.RemoveWindow(dialog.Window)
	for {
		ev := poll()
		if ev != nil {
			dialog.HandleEvent(ev)
		}
		if dialog.closed {
			return
		}
	}
}
