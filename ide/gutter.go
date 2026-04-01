package ide

import (
	"fmt"

	"go-tp/tv/core"
	"go-tp/tv/views"
)

var (
	attrGutter        = core.MakeAttr(core.ColorDarkGray, core.ColorBlue)
	attrGutterBP      = core.MakeAttr(core.ColorLightRed, core.ColorBlue)
	attrGutterCurrent = core.MakeAttr(core.ColorLightGreen, core.ColorBlue)
)

// LineNumberGutter is a narrow non-focusable view that shows line numbers
// synchronized to an Editor's TopLine. It also renders breakpoint markers (●)
// and the current execution line indicator (→).
type LineNumberGutter struct {
	views.ViewBase
	editor *views.Editor
	// GetBreakpointAt returns true if the given 1-based line has a breakpoint.
	GetBreakpointAt func(line int) bool
	// GetCurrentLine returns the 1-based current execution line (0 = none).
	GetCurrentLine func() int
}

// NewLineNumberGutter creates a gutter (W should be 4 to fit marker + 3-digit number).
func NewLineNumberGutter(bounds core.Rect, ed *views.Editor) *LineNumberGutter {
	g := &LineNumberGutter{editor: ed}
	g.SetBounds(bounds)
	return g
}

func (g *LineNumberGutter) CanFocus() bool { return false }

func (g *LineNumberGutter) Draw(buf *core.DrawBuffer) {
	b := g.Bounds()
	topLine := g.editor.TopLine()
	lineCount := g.editor.LineCount()

	var currentLine int
	if g.GetCurrentLine != nil {
		currentLine = g.GetCurrentLine()
	}

	for row := 0; row < b.H; row++ {
		lineNum := topLine + row + 1
		if lineNum > lineCount {
			buf.MoveStr(0, row, "    ", attrGutter)
			continue
		}

		numText := fmt.Sprintf("%3d", lineNum)

		hasBP := g.GetBreakpointAt != nil && g.GetBreakpointAt(lineNum)
		isCurrent := currentLine > 0 && lineNum == currentLine

		switch {
		case isCurrent:
			buf.MoveChar(0, row, 1, '→', attrGutterCurrent)
			buf.MoveStr(1, row, numText, attrGutterCurrent)
		case hasBP:
			buf.MoveChar(0, row, 1, '●', attrGutterBP)
			buf.MoveStr(1, row, numText, attrGutterBP)
		default:
			buf.MoveChar(0, row, 1, ' ', attrGutter)
			buf.MoveStr(1, row, numText, attrGutter)
		}
	}
}
