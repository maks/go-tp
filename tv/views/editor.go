package views

import "go-tp/tv/core"

// Highlighter provides per-character syntax coloring for the editor.
type Highlighter interface {
	// Highlight returns one Attr per rune for each line.
	Highlight(lines []string) [][]core.Attr
}

// EditOp records a single undoable edit operation.
type EditOp struct {
	// OpInsert or OpDelete.
	kind    int
	row     int
	col     int
	text    string
}

const opInsert = 1
const opDelete = 2

// Editor is a multi-line text editor with syntax highlighting, selection,
// and undo/redo.
type Editor struct {
	ViewBase
	lines       [][]rune
	curRow      int
	curCol      int
	topLine     int
	leftCol     int
	// Selection (selStart..selEnd, or both -1 if no selection).
	selAnchorRow int
	selAnchorCol int
	selecting    bool
	// Undo stack.
	undoStack []EditOp
	redoStack []EditOp
	// Optional syntax highlighter.
	Highlighter Highlighter
	// Called on every content change.
	OnChange func()
}

// NewEditor creates an empty Editor.
func NewEditor(bounds core.Rect) *Editor {
	e := &Editor{
		lines: [][]rune{{}},
	}
	e.bounds = bounds
	e.selAnchorRow = -1
	e.selAnchorCol = -1
	return e
}

func (e *Editor) CanFocus() bool { return true }

// SetText replaces the editor contents.
func (e *Editor) SetText(text string) {
	e.lines = splitLines(text)
	e.curRow = 0
	e.curCol = 0
	e.topLine = 0
	e.leftCol = 0
	e.undoStack = nil
	e.redoStack = nil
	e.selAnchorRow = -1
	e.selAnchorCol = -1
}

// GetText returns the editor contents as a single string.
func (e *Editor) GetText() string {
	var s []rune
	for i, line := range e.lines {
		if i > 0 {
			s = append(s, '\n')
		}
		s = append(s, line...)
	}
	return string(s)
}

// GetLines returns the raw line slice (read-only — do not modify).
func (e *Editor) GetLines() []string {
	out := make([]string, len(e.lines))
	for i, l := range e.lines {
		out[i] = string(l)
	}
	return out
}

// CursorRow returns the current cursor row.
func (e *Editor) CursorRow() int { return e.curRow }

// CursorCol returns the current cursor column.
func (e *Editor) CursorCol() int { return e.curCol }

// GotoLine scrolls and positions the cursor at (row, col).
func (e *Editor) GotoLine(row, col int) {
	if row < 0 {
		row = 0
	}
	if row >= len(e.lines) {
		row = len(e.lines) - 1
	}
	e.curRow = row
	e.curCol = clampCol(e.lines[row], col)
	e.scrollToCursor()
}

func (e *Editor) Draw(buf *core.DrawBuffer) {
	w := e.bounds.W
	h := e.bounds.H
	var highlights [][]core.Attr
	if e.Highlighter != nil {
		highlights = e.Highlighter.Highlight(e.GetLines())
	}

	for screenRow := 0; screenRow < h; screenRow++ {
		lineIdx := e.topLine + screenRow
		buf.MoveChar(0, screenRow, w, ' ', core.AttrEditor)
		if lineIdx >= len(e.lines) {
			continue
		}
		line := e.lines[lineIdx]
		var lineAttrs []core.Attr
		if highlights != nil && lineIdx < len(highlights) {
			lineAttrs = highlights[lineIdx]
		}
		for screenCol := 0; screenCol < w; screenCol++ {
			runeIdx := e.leftCol + screenCol
			ch := rune(' ')
			attr := core.AttrEditor
			if runeIdx < len(line) {
				ch = line[runeIdx]
				if lineAttrs != nil && runeIdx < len(lineAttrs) {
					attr = lineAttrs[runeIdx]
				}
			}
			// Highlight selection.
			if e.inSelection(lineIdx, runeIdx) {
				attr = core.AttrEditorSelected
			}
			// Cursor.
			if lineIdx == e.curRow && runeIdx == e.curCol {
				attr = attr.Swap()
			}
			if c := buf.At(screenCol, screenRow); c != nil {
				c.Ch = ch
				c.Attr = attr
			}
		}
	}
}

func (e *Editor) HandleEvent(ev *core.Event) {
	if ev.Handled {
		return
	}
	if ev.Type == core.EvKeyboard {
		e.handleKey(ev)
		return
	}
	if ev.Type == core.EvMouseDown {
		p := core.Point{X: ev.MouseX, Y: ev.MouseY}
		if e.bounds.Contains(p) {
			row := e.topLine + (ev.MouseY - e.bounds.Y)
			col := e.leftCol + (ev.MouseX - e.bounds.X)
			if row < len(e.lines) {
				e.curRow = row
				e.curCol = clampCol(e.lines[row], col)
				e.selAnchorRow = -1
			}
			ev.Handled = true
		}
	}
}

func (e *Editor) handleKey(ev *core.Event) {
	switch ev.Key {
	case core.KbUp:
		e.moveCursor(-1, 0)
		ev.Handled = true
	case core.KbDown:
		e.moveCursor(1, 0)
		ev.Handled = true
	case core.KbLeft:
		e.moveLeft()
		ev.Handled = true
	case core.KbRight:
		e.moveRight()
		ev.Handled = true
	case core.KbHome:
		e.curCol = 0
		e.scrollToCursor()
		ev.Handled = true
	case core.KbEnd:
		e.curCol = len(e.lines[e.curRow])
		e.scrollToCursor()
		ev.Handled = true
	case core.KbPgUp:
		e.moveCursor(-e.bounds.H, 0)
		ev.Handled = true
	case core.KbPgDn:
		e.moveCursor(e.bounds.H, 0)
		ev.Handled = true
	case core.KbEnter:
		e.insertNewline()
		ev.Handled = true
	case core.KbBackSpace:
		e.backspace()
		ev.Handled = true
	case core.KbDel:
		e.deleteChar()
		ev.Handled = true
	default:
		if ev.Ch >= 32 || ev.Ch == '\t' {
			e.insertChar(ev.Ch)
			ev.Handled = true
		}
	}
}

func (e *Editor) moveCursor(drow, dcol int) {
	newRow := e.curRow + drow
	if newRow < 0 {
		newRow = 0
	}
	if newRow >= len(e.lines) {
		newRow = len(e.lines) - 1
	}
	e.curRow = newRow
	if dcol == 0 {
		e.curCol = clampCol(e.lines[e.curRow], e.curCol)
	} else {
		e.curCol = clampCol(e.lines[e.curRow], e.curCol+dcol)
	}
	e.scrollToCursor()
}

func (e *Editor) moveLeft() {
	if e.curCol > 0 {
		e.curCol--
	} else if e.curRow > 0 {
		e.curRow--
		e.curCol = len(e.lines[e.curRow])
	}
	e.scrollToCursor()
}

func (e *Editor) moveRight() {
	if e.curCol < len(e.lines[e.curRow]) {
		e.curCol++
	} else if e.curRow < len(e.lines)-1 {
		e.curRow++
		e.curCol = 0
	}
	e.scrollToCursor()
}

func (e *Editor) insertChar(ch rune) {
	line := e.lines[e.curRow]
	newLine := make([]rune, len(line)+1)
	copy(newLine, line[:e.curCol])
	newLine[e.curCol] = ch
	copy(newLine[e.curCol+1:], line[e.curCol:])
	e.lines[e.curRow] = newLine
	e.curCol++
	e.scrollToCursor()
	e.pushUndo(EditOp{kind: opInsert, row: e.curRow, col: e.curCol - 1, text: string(ch)})
	e.notify()
}

func (e *Editor) insertNewline() {
	line := e.lines[e.curRow]
	before := make([]rune, e.curCol)
	copy(before, line[:e.curCol])
	after := make([]rune, len(line)-e.curCol)
	copy(after, line[e.curCol:])
	e.lines[e.curRow] = before
	newLines := make([][]rune, len(e.lines)+1)
	copy(newLines, e.lines[:e.curRow+1])
	newLines[e.curRow+1] = after
	copy(newLines[e.curRow+2:], e.lines[e.curRow+1:])
	e.lines = newLines
	e.curRow++
	e.curCol = 0
	e.scrollToCursor()
	e.notify()
}

func (e *Editor) backspace() {
	if e.curCol > 0 {
		line := e.lines[e.curRow]
		ch := line[e.curCol-1]
		newLine := make([]rune, len(line)-1)
		copy(newLine, line[:e.curCol-1])
		copy(newLine[e.curCol-1:], line[e.curCol:])
		e.lines[e.curRow] = newLine
		e.curCol--
		e.scrollToCursor()
		e.pushUndo(EditOp{kind: opDelete, row: e.curRow, col: e.curCol, text: string(ch)})
		e.notify()
	} else if e.curRow > 0 {
		// Join with previous line.
		prevLine := e.lines[e.curRow-1]
		curLine := e.lines[e.curRow]
		joinedCol := len(prevLine)
		joined := append(prevLine, curLine...)
		newLines := make([][]rune, len(e.lines)-1)
		copy(newLines, e.lines[:e.curRow-1])
		newLines[e.curRow-1] = joined
		copy(newLines[e.curRow:], e.lines[e.curRow+1:])
		e.lines = newLines
		e.curRow--
		e.curCol = joinedCol
		e.scrollToCursor()
		e.notify()
	}
}

func (e *Editor) deleteChar() {
	line := e.lines[e.curRow]
	if e.curCol < len(line) {
		newLine := make([]rune, len(line)-1)
		copy(newLine, line[:e.curCol])
		copy(newLine[e.curCol:], line[e.curCol+1:])
		e.lines[e.curRow] = newLine
		e.notify()
	} else if e.curRow < len(e.lines)-1 {
		// Join with next line.
		nextLine := e.lines[e.curRow+1]
		joined := append(line, nextLine...)
		newLines := make([][]rune, len(e.lines)-1)
		copy(newLines, e.lines[:e.curRow])
		newLines[e.curRow] = joined
		copy(newLines[e.curRow+1:], e.lines[e.curRow+2:])
		e.lines = newLines
		e.notify()
	}
}

func (e *Editor) scrollToCursor() {
	h := e.bounds.H
	w := e.bounds.W
	if e.curRow < e.topLine {
		e.topLine = e.curRow
	}
	if e.curRow >= e.topLine+h {
		e.topLine = e.curRow - h + 1
	}
	if e.curCol < e.leftCol {
		e.leftCol = e.curCol
	}
	if e.curCol >= e.leftCol+w {
		e.leftCol = e.curCol - w + 1
	}
}

func (e *Editor) inSelection(row, col int) bool {
	if e.selAnchorRow < 0 {
		return false
	}
	// Determine selection range.
	r1, c1, r2, c2 := e.selAnchorRow, e.selAnchorCol, e.curRow, e.curCol
	if r1 > r2 || (r1 == r2 && c1 > c2) {
		r1, c1, r2, c2 = r2, c2, r1, c1
	}
	if row < r1 || row > r2 {
		return false
	}
	if row == r1 && col < c1 {
		return false
	}
	if row == r2 && col >= c2 {
		return false
	}
	return true
}

func (e *Editor) pushUndo(op EditOp) {
	e.undoStack = append(e.undoStack, op)
	e.redoStack = nil
}

func (e *Editor) notify() {
	if e.OnChange != nil {
		e.OnChange()
	}
}

// TopLine returns the index of the first visible line.
func (e *Editor) TopLine() int { return e.topLine }

// LineCount returns the total number of lines.
func (e *Editor) LineCount() int { return len(e.lines) }

func clampCol(line []rune, col int) int {
	if col < 0 {
		return 0
	}
	if col > len(line) {
		return len(line)
	}
	return col
}

func splitLines(text string) [][]rune {
	var lines [][]rune
	start := 0
	runes := []rune(text)
	for i, r := range runes {
		if r == '\n' {
			lines = append(lines, runes[start:i])
			start = i + 1
		}
	}
	lines = append(lines, runes[start:])
	return lines
}
