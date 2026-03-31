package ide

import (
	"fmt"

	"go-tp/pascal"
	"go-tp/tv/core"
	"go-tp/tv/views"
)

var (
	attrOutputNormal = core.MakeAttr(core.ColorLightGray, core.ColorBlack)
	attrOutputError  = core.MakeAttr(core.ColorLightRed, core.ColorBlack)
	attrOutputOK     = core.MakeAttr(core.ColorLightGreen, core.ColorBlack)
	attrOutputSelect = core.MakeAttr(core.ColorBlack, core.ColorLightCyan)
)

type outputLine struct {
	text    string
	attr    core.Attr
	srcLine int // 0 = not an error; >0 = 1-based source line number
}

// OutputWindow displays compiler diagnostics and program output.
type OutputWindow struct {
	win      *views.Window
	content  *outputContentView
	lines    []outputLine
	topLine  int
	selected int
	vScroll  *views.ScrollBar
	// OnGotoErr is called with the 1-based source line when the user activates an error line.
	OnGotoErr func(line int)
}

// outputContentView is a child view of the output window that renders text lines.
type outputContentView struct {
	views.ViewBase
	ow *OutputWindow
}

func (v *outputContentView) CanFocus() bool { return true }

func (v *outputContentView) Draw(buf *core.DrawBuffer) {
	ow := v.ow
	iW := v.Bounds().W
	iH := v.Bounds().H
	for row := 0; row < iH; row++ {
		lineIdx := ow.topLine + row
		var text string
		var attr core.Attr
		if lineIdx < len(ow.lines) {
			text = ow.lines[lineIdx].text
			attr = ow.lines[lineIdx].attr
			if lineIdx == ow.selected {
				attr = attrOutputSelect
			}
		} else {
			attr = attrOutputNormal
		}
		buf.MoveChar(0, row, iW, ' ', attr)
		if text != "" {
			runes := []rune(text)
			if len(runes) > iW {
				runes = runes[:iW]
			}
			buf.MoveStr(0, row, string(runes), attr)
		}
	}
}

func (v *outputContentView) HandleEvent(ev *core.Event) {
	if ev.Handled || ev.Type != core.EvKeyboard {
		return
	}
	ow := v.ow
	iH := v.Bounds().H

	switch ev.Key {
	case core.KbUp:
		if ow.selected > 0 {
			ow.selected--
			if ow.selected < ow.topLine {
				ow.topLine = ow.selected
			}
		} else if ow.topLine > 0 {
			ow.topLine--
		}
		ev.Handled = true
	case core.KbDown:
		if ow.selected < len(ow.lines)-1 {
			ow.selected++
			if ow.selected >= ow.topLine+iH {
				ow.topLine = ow.selected - iH + 1
			}
		}
		ev.Handled = true
	case core.KbEnter:
		ow.activateSelected()
		ev.Handled = true
	}
}

// NewOutputWindow creates an output window filling the given bounds.
func NewOutputWindow(bounds core.Rect) *OutputWindow {
	win := views.NewWindow(bounds, "Output")
	iW := bounds.W - 2
	iH := bounds.H - 2

	vScroll := views.NewVScrollBar(core.Rect{})
	win.Add(vScroll, core.Rect{X: iW - 1, Y: 0, W: 1, H: iH})

	ow := &OutputWindow{
		win:      win,
		selected: -1,
		vScroll:  vScroll,
	}

	content := &outputContentView{ow: ow}
	win.Add(content, core.Rect{X: 0, Y: 0, W: iW - 1, H: iH})
	ow.content = content

	vScroll.OnChange = func(v int) {
		ow.topLine = v
	}

	return ow
}

// Win returns the underlying *views.Window.
func (ow *OutputWindow) Win() *views.Window { return ow.win }

// AppendLine appends a plain output line.
func (ow *OutputWindow) AppendLine(text string, attr core.Attr) {
	ow.lines = append(ow.lines, outputLine{text: text, attr: attr})
	ow.syncScrollbar()
	ow.autoScroll()
}

// AppendError appends a compiler diagnostic as a clickable error line.
func (ow *OutputWindow) AppendError(diag pascal.Diagnostic) {
	text := fmt.Sprintf("  Line %d, Col %d: %s", diag.Line, diag.Col, diag.Msg)
	ow.lines = append(ow.lines, outputLine{
		text:    text,
		attr:    attrOutputError,
		srcLine: diag.Line,
	})
	ow.syncScrollbar()
	ow.autoScroll()
}

// Clear removes all output lines.
func (ow *OutputWindow) Clear() {
	ow.lines = nil
	ow.topLine = 0
	ow.selected = -1
	ow.syncScrollbar()
}

func (ow *OutputWindow) visibleH() int {
	b := ow.win.Bounds()
	iH := b.H - 2
	if iH < 1 {
		return 1
	}
	return iH
}

func (ow *OutputWindow) syncScrollbar() {
	visH := ow.visibleH()
	ow.vScroll.Max = maxInt(0, len(ow.lines)-visH)
	ow.vScroll.SetValue(ow.topLine)
}

func (ow *OutputWindow) autoScroll() {
	visH := ow.visibleH()
	if len(ow.lines) > visH {
		ow.topLine = len(ow.lines) - visH
		ow.vScroll.SetValue(ow.topLine)
	}
}

func (ow *OutputWindow) activateSelected() {
	if ow.selected >= 0 && ow.selected < len(ow.lines) {
		line := ow.lines[ow.selected]
		if line.srcLine > 0 && ow.OnGotoErr != nil {
			ow.OnGotoErr(line.srcLine)
		}
	}
}
