package ide

import (
	"fmt"

	"go-tp/tv/core"
	"go-tp/tv/views"
)

var attrGutter = core.MakeAttr(core.ColorDarkGray, core.ColorBlue)

// LineNumberGutter is a narrow non-focusable view that shows line numbers
// synchronized to an Editor's TopLine.
type LineNumberGutter struct {
	views.ViewBase
	editor *views.Editor
}

// NewLineNumberGutter creates a gutter (W should be 3).
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
	for row := 0; row < b.H; row++ {
		lineNum := topLine + row + 1
		var text string
		if lineNum <= lineCount {
			text = fmt.Sprintf("%3d", lineNum)
		} else {
			text = "   "
		}
		buf.MoveStr(0, row, text, attrGutter)
	}
}
