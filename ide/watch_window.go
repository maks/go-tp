package ide

import (
	"go-tp/debugger"
	"go-tp/tv/core"
	"go-tp/tv/views"
)

var (
	attrWatchNormal = core.MakeAttr(core.ColorLightGray, core.ColorBlack)
	attrWatchHeader = core.MakeAttr(core.ColorWhite, core.ColorBlue)
)

// watchContentView renders the variable name/value pairs.
type watchContentView struct {
	views.ViewBase
	ww *WatchWindow
}

func (v *watchContentView) CanFocus() bool { return false }

func (v *watchContentView) Draw(buf *core.DrawBuffer) {
	ww := v.ww
	iW := v.Bounds().W
	iH := v.Bounds().H
	for row := 0; row < iH; row++ {
		idx := ww.topLine + row
		var text string
		if idx < len(ww.vars) {
			s := ww.vars[idx]
			text = s.Name + " = " + s.Value
		}
		// Pad or truncate to width.
		runes := []rune(text)
		if len(runes) > iW {
			runes = runes[:iW]
		}
		buf.MoveChar(0, row, iW, ' ', attrWatchNormal)
		if len(runes) > 0 {
			buf.MoveStr(0, row, string(runes), attrWatchNormal)
		}
	}
}

// WatchWindow displays the current values of all debug variables.
type WatchWindow struct {
	win     *views.Window
	content *watchContentView
	vScroll *views.ScrollBar
	vars    []debugger.VarSnapshot
	topLine int
}

// NewWatchWindow creates a WatchWindow filling the given bounds.
func NewWatchWindow(bounds core.Rect) *WatchWindow {
	win := views.NewWindow(bounds, "Watches")
	iW := bounds.W - 2
	iH := bounds.H - 2

	vScroll := views.NewVScrollBar(core.Rect{})
	win.Add(vScroll, core.Rect{X: iW - 1, Y: 0, W: 1, H: iH})

	ww := &WatchWindow{
		win:     win,
		vScroll: vScroll,
	}

	content := &watchContentView{ww: ww}
	win.Add(content, core.Rect{X: 0, Y: 0, W: iW - 1, H: iH})
	ww.content = content

	vScroll.OnChange = func(v int) { ww.topLine = v }

	return ww
}

// Win returns the underlying *views.Window.
func (ww *WatchWindow) Win() *views.Window { return ww.win }

// SetValues updates the displayed variable snapshots.
func (ww *WatchWindow) SetValues(vars []debugger.VarSnapshot) {
	ww.vars = vars
	ww.topLine = 0
	iH := ww.win.Bounds().H - 2
	if iH < 1 {
		iH = 1
	}
	ww.vScroll.Max = maxInt(0, len(vars)-iH)
	ww.vScroll.SetValue(0)
}

// Clear removes all variable entries.
func (ww *WatchWindow) Clear() {
	ww.SetValues(nil)
}
