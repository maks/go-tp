package core

// Point is an (X, Y) coordinate pair.
type Point struct {
	X, Y int
}

// Rect is an axis-aligned rectangle defined by its top-left corner (X, Y),
// width W, and height H. All coordinates are in character-cell units.
type Rect struct {
	X, Y, W, H int
}

// Contains reports whether p is inside r.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
}

// Intersect returns the intersection of r and s. If there is no intersection
// the returned Rect has W==0 or H==0.
func (r Rect) Intersect(s Rect) Rect {
	x0 := max(r.X, s.X)
	y0 := max(r.Y, s.Y)
	x1 := min(r.X+r.W, s.X+s.W)
	y1 := min(r.Y+r.H, s.Y+s.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{}
	}
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// Grow returns a copy of r expanded by dx on each horizontal side and dy on
// each vertical side (negative values shrink).
func (r Rect) Grow(dx, dy int) Rect {
	return Rect{X: r.X - dx, Y: r.Y - dy, W: r.W + 2*dx, H: r.H + 2*dy}
}

// Offset returns a copy of r translated by (dx, dy).
func (r Rect) Offset(dx, dy int) Rect {
	return Rect{X: r.X + dx, Y: r.Y + dy, W: r.W, H: r.H}
}

// IsEmpty reports whether r has zero area.
func (r Rect) IsEmpty() bool { return r.W <= 0 || r.H <= 0 }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
