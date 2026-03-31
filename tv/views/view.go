package views

import "go-tp/tv/core"

// View is the base interface implemented by all UI elements.
type View interface {
	// Bounds returns the view's absolute position and size on screen.
	Bounds() core.Rect
	// SetBounds repositions/resizes the view.
	SetBounds(core.Rect)
	// Draw renders the view into buf. buf covers the view's own bounds.
	Draw(buf *core.DrawBuffer)
	// HandleEvent processes ev. Set ev.Handled = true to stop propagation.
	HandleEvent(ev *core.Event)
	// CanFocus reports whether the view can receive keyboard focus.
	CanFocus() bool
	// SetFocused grants or removes keyboard focus.
	SetFocused(bool)
	// IsFocused reports whether the view currently has focus.
	IsFocused() bool
	// SetOwner links this view to its parent container.
	SetOwner(View)
	// Owner returns the parent container, or nil for top-level views.
	Owner() View
}

// ViewBase provides default implementations for all View methods.
// Embed *ViewBase in a concrete view and override only what you need.
type ViewBase struct {
	bounds  core.Rect
	focused bool
	owner   View
}

func (v *ViewBase) Bounds() core.Rect            { return v.bounds }
func (v *ViewBase) SetBounds(r core.Rect)        { v.bounds = r }
func (v *ViewBase) Draw(_ *core.DrawBuffer)      {}
func (v *ViewBase) HandleEvent(_ *core.Event)    {}
func (v *ViewBase) CanFocus() bool               { return false }
func (v *ViewBase) SetFocused(f bool)            { v.focused = f }
func (v *ViewBase) IsFocused() bool              { return v.focused }
func (v *ViewBase) SetOwner(o View)              { v.owner = o }
func (v *ViewBase) Owner() View                  { return v.owner }
