package core

import "testing"

func TestRectContains(t *testing.T) {
	r := Rect{X: 5, Y: 5, W: 10, H: 10}
	cases := []struct {
		p    Point
		want bool
	}{
		{Point{5, 5}, true},
		{Point{14, 14}, true},
		{Point{15, 15}, false}, // outside (exclusive)
		{Point{4, 5}, false},
		{Point{5, 4}, false},
	}
	for _, c := range cases {
		got := r.Contains(c.p)
		if got != c.want {
			t.Errorf("Contains(%v) = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestRectIntersect(t *testing.T) {
	a := Rect{X: 0, Y: 0, W: 10, H: 10}
	b := Rect{X: 5, Y: 5, W: 10, H: 10}
	got := a.Intersect(b)
	want := Rect{X: 5, Y: 5, W: 5, H: 5}
	if got != want {
		t.Errorf("Intersect = %v, want %v", got, want)
	}
	// No intersection.
	c := Rect{X: 20, Y: 20, W: 5, H: 5}
	got = a.Intersect(c)
	if !got.IsEmpty() {
		t.Errorf("expected empty intersection, got %v", got)
	}
}

func TestRectGrow(t *testing.T) {
	r := Rect{X: 5, Y: 5, W: 10, H: 10}
	got := r.Grow(2, 3)
	want := Rect{X: 3, Y: 2, W: 14, H: 16}
	if got != want {
		t.Errorf("Grow = %v, want %v", got, want)
	}
}

func TestRectOffset(t *testing.T) {
	r := Rect{X: 5, Y: 5, W: 10, H: 10}
	got := r.Offset(3, -2)
	want := Rect{X: 8, Y: 3, W: 10, H: 10}
	if got != want {
		t.Errorf("Offset = %v, want %v", got, want)
	}
}
