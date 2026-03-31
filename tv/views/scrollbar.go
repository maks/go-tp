package views

import "go-tp/tv/core"

// ScrollBar is a vertical or horizontal scroll indicator.
type ScrollBar struct {
	ViewBase
	Vertical bool
	Value    int
	Max      int
	PageSize int
	OnChange func(int)
	// drag state
	dragging   bool
	dragStart  int
	dragOrigin int
}

// NewVScrollBar creates a vertical ScrollBar.
func NewVScrollBar(bounds core.Rect) *ScrollBar {
	sb := &ScrollBar{Vertical: true}
	sb.bounds = bounds
	return sb
}

// NewHScrollBar creates a horizontal ScrollBar.
func NewHScrollBar(bounds core.Rect) *ScrollBar {
	sb := &ScrollBar{Vertical: false}
	sb.bounds = bounds
	return sb
}

func (sb *ScrollBar) Draw(buf *core.DrawBuffer) {
	attr := core.AttrScrollBar
	if sb.Vertical {
		h := sb.bounds.H
		if h < 3 {
			return
		}
		// Up arrow.
		buf.MoveChar(0, 0, 1, '▲', attr)
		// Track.
		for y := 1; y < h-1; y++ {
			buf.MoveChar(0, y, 1, '░', attr)
		}
		// Down arrow.
		buf.MoveChar(0, h-1, 1, '▼', attr)
		// Thumb.
		thumbPos := sb.thumbPos()
		if c := buf.At(0, thumbPos); c != nil {
			c.Ch = '█'
		}
	} else {
		w := sb.bounds.W
		if w < 3 {
			return
		}
		buf.MoveChar(0, 0, 1, '◄', attr)
		for x := 1; x < w-1; x++ {
			buf.MoveChar(x, 0, 1, '░', attr)
		}
		buf.MoveChar(w-1, 0, 1, '►', attr)
		thumbPos := sb.thumbPos()
		if c := buf.At(thumbPos, 0); c != nil {
			c.Ch = '█'
		}
	}
}

func (sb *ScrollBar) thumbPos() int {
	if sb.Max <= 0 {
		if sb.Vertical {
			return 1
		}
		return 1
	}
	track := sb.trackLen()
	pos := sb.Value * track / sb.Max
	if pos >= track {
		pos = track - 1
	}
	if sb.Vertical {
		return pos + 1
	}
	return pos + 1
}

func (sb *ScrollBar) trackLen() int {
	if sb.Vertical {
		return sb.bounds.H - 2
	}
	return sb.bounds.W - 2
}

func (sb *ScrollBar) HandleEvent(ev *core.Event) {
	if ev.Type == core.EvMouseDown {
		p := core.Point{X: ev.MouseX, Y: ev.MouseY}
		if !sb.bounds.Contains(p) {
			return
		}
		ev.Handled = true
		if sb.Vertical {
			rel := ev.MouseY - sb.bounds.Y
			if rel == 0 {
				sb.scroll(-1)
			} else if rel == sb.bounds.H-1 {
				sb.scroll(1)
			} else if rel > 0 && rel < sb.bounds.H-1 {
				// Click on track — page scroll.
				if rel < sb.thumbPos() {
					sb.scroll(-sb.PageSize)
				} else {
					sb.scroll(sb.PageSize)
				}
			}
		} else {
			rel := ev.MouseX - sb.bounds.X
			if rel == 0 {
				sb.scroll(-1)
			} else if rel == sb.bounds.W-1 {
				sb.scroll(1)
			} else {
				if rel < sb.thumbPos() {
					sb.scroll(-sb.PageSize)
				} else {
					sb.scroll(sb.PageSize)
				}
			}
		}
	}
}

func (sb *ScrollBar) scroll(delta int) {
	newVal := sb.Value + delta
	if newVal < 0 {
		newVal = 0
	}
	if newVal > sb.Max {
		newVal = sb.Max
	}
	if newVal != sb.Value {
		sb.Value = newVal
		if sb.OnChange != nil {
			sb.OnChange(newVal)
		}
	}
}

// SetValue updates the scroll position without triggering OnChange.
func (sb *ScrollBar) SetValue(v int) {
	if v < 0 {
		v = 0
	}
	if v > sb.Max {
		v = sb.Max
	}
	sb.Value = v
}
