package core

// Cell holds a single character cell: the displayed rune and its color attribute.
type Cell struct {
	Ch   rune
	Attr Attr
}

// DrawBuffer is a flat array of Cells representing a rectangular region of the
// screen. It is indexed row-major: cell at (x, y) is at index y*Width + x.
type DrawBuffer struct {
	Cells  []Cell
	Width  int
	Height int
}

// NewDrawBuffer allocates a DrawBuffer of the given size, filled with spaces
// using the supplied default attribute.
func NewDrawBuffer(w, h int, def Attr) *DrawBuffer {
	cells := make([]Cell, w*h)
	for i := range cells {
		cells[i] = Cell{Ch: ' ', Attr: def}
	}
	return &DrawBuffer{Cells: cells, Width: w, Height: h}
}

// At returns a pointer to the cell at (x, y). Returns nil if out of bounds.
func (b *DrawBuffer) At(x, y int) *Cell {
	if x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return nil
	}
	return &b.Cells[y*b.Width+x]
}

// MoveChar fills the horizontal run starting at (x, y) of length n with ch/attr.
func (b *DrawBuffer) MoveChar(x, y, n int, ch rune, attr Attr) {
	for i := 0; i < n; i++ {
		if c := b.At(x+i, y); c != nil {
			c.Ch = ch
			c.Attr = attr
		}
	}
}

// MoveStr writes s starting at (x, y) with the given attr.
func (b *DrawBuffer) MoveStr(x, y int, s string, attr Attr) {
	for i, r := range s {
		if c := b.At(x+i, y); c != nil {
			c.Ch = r
			c.Attr = attr
		}
	}
}

// PutAttr sets only the attribute of cells in the horizontal run at (x,y) len n.
func (b *DrawBuffer) PutAttr(x, y, n int, attr Attr) {
	for i := 0; i < n; i++ {
		if c := b.At(x+i, y); c != nil {
			c.Attr = attr
		}
	}
}

// Fill fills the entire buffer with ch and attr.
func (b *DrawBuffer) Fill(ch rune, attr Attr) {
	for i := range b.Cells {
		b.Cells[i] = Cell{Ch: ch, Attr: attr}
	}
}

// CopyFrom copies cells from src into b at offset (dx, dy).
// Only cells within b's bounds are written; src cells outside b are clipped.
func (b *DrawBuffer) CopyFrom(src *DrawBuffer, dx, dy int) {
	for sy := 0; sy < src.Height; sy++ {
		for sx := 0; sx < src.Width; sx++ {
			if c := b.At(dx+sx, dy+sy); c != nil {
				*c = src.Cells[sy*src.Width+sx]
			}
		}
	}
}

// Sub returns a view of b clipped to the given Rect. It shares the underlying
// slice, so writes to the returned buffer are visible in b.
// This is not a true sub-buffer (pointer arithmetic would require unsafe), so
// instead Sub returns a new DrawBuffer that copies only the requested region.
// Callers use CopyFrom to flush it back.
func Sub(b *DrawBuffer, r Rect) *DrawBuffer {
	sub := NewDrawBuffer(r.W, r.H, 0)
	for sy := 0; sy < r.H; sy++ {
		for sx := 0; sx < r.W; sx++ {
			if c := b.At(r.X+sx, r.Y+sy); c != nil {
				sub.Cells[sy*r.W+sx] = *c
			}
		}
	}
	return sub
}
