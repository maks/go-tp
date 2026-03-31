package views

import "go-tp/tv/core"

// FrameStyle selects the border character set.
type FrameStyle int

const (
	FrameSingle FrameStyle = iota
	FrameDouble
)

// Frame draws a bordered rectangle around its bounds, with an optional title
// in the top border. It is purely decorative — it does not manage children.
type Frame struct {
	ViewBase
	Title   string
	Style   FrameStyle
	AttrActive   core.Attr
	AttrInactive core.Attr
	active  bool
}

// NewFrame creates a Frame with the given bounds and title.
func NewFrame(bounds core.Rect, title string, style FrameStyle) *Frame {
	f := &Frame{
		Title:        title,
		Style:        style,
		AttrActive:   core.AttrActive,
		AttrInactive: core.AttrInactive,
	}
	f.bounds = bounds
	f.active = true
	return f
}

func (f *Frame) SetActive(a bool) { f.active = a }

func (f *Frame) Draw(buf *core.DrawBuffer) {
	attr := f.AttrActive
	if !f.active {
		attr = f.AttrInactive
	}
	w, h := f.bounds.W, f.bounds.H
	if w < 2 || h < 2 {
		return
	}

	var tl, tr, bl, br, hz, vt rune
	if f.Style == FrameDouble {
		tl, tr, bl, br = '╔', '╗', '╚', '╝'
		hz, vt = '═', '║'
	} else {
		tl, tr, bl, br = '┌', '┐', '└', '┘'
		hz, vt = '─', '│'
	}

	// Top border.
	buf.MoveChar(0, 0, w, hz, attr)
	buf.At(0, 0).Ch = tl
	buf.At(w-1, 0).Ch = tr

	// Bottom border.
	buf.MoveChar(0, h-1, w, hz, attr)
	buf.At(0, h-1).Ch = bl
	buf.At(w-1, h-1).Ch = br

	// Side borders.
	for y := 1; y < h-1; y++ {
		if c := buf.At(0, y); c != nil {
			c.Ch = vt
			c.Attr = attr
		}
		if c := buf.At(w-1, y); c != nil {
			c.Ch = vt
			c.Attr = attr
		}
	}

	// Title (centered in top border).
	if f.Title != "" {
		title := " " + f.Title + " "
		runes := []rune(title)
		maxLen := w - 4
		if maxLen < 1 {
			return
		}
		if len(runes) > maxLen {
			runes = runes[:maxLen]
		}
		startX := (w - len(runes)) / 2
		buf.MoveStr(startX, 0, string(runes), attr)
	}
}
