package ide

import (
	"os"
	"path/filepath"

	"go-tp/tv/core"
	"go-tp/tv/views"
)

// IdeEditorWindow wraps a views.Window with a line number gutter, editor, and scrollbars.
type IdeEditorWindow struct {
	win             *views.Window
	editor          *views.Editor
	gutter          *LineNumberGutter
	vScroll         *views.ScrollBar
	hl              *PascalHighlighter
	filePath        string
	modified        bool
	breakpointLines map[int]bool
	currentLine     int // 1-based; 0 = none
}

// NewIdeEditorWindow creates an editor window filling the given bounds.
func NewIdeEditorWindow(bounds core.Rect) *IdeEditorWindow {
	win := views.NewWindow(bounds, "untitled.pas")
	iW := bounds.W - 2 // interior width (inside 1-cell frame border)
	iH := bounds.H - 2 // interior height

	// Layout: gutter(4) | editor(iW-5) | vscroll(1)
	gutterW := 4
	scrollW := 1
	edW := iW - gutterW - scrollW
	if edW < 1 {
		edW = 1
	}
	edH := iH

	gutterRel := core.Rect{X: 0, Y: 0, W: gutterW, H: edH}
	editorRel := core.Rect{X: gutterW, Y: 0, W: edW, H: edH}
	vScrollRel := core.Rect{X: iW - scrollW, Y: 0, W: scrollW, H: edH}

	editor := views.NewEditor(core.Rect{}) // bounds set by window.Add
	hl := &PascalHighlighter{}
	editor.Highlighter = hl

	gutter := NewLineNumberGutter(core.Rect{}, editor)
	vScroll := views.NewVScrollBar(core.Rect{})

	win.Add(editor, editorRel)
	win.Add(gutter, gutterRel)
	win.Add(vScroll, vScrollRel)

	ew := &IdeEditorWindow{
		win:             win,
		editor:          editor,
		gutter:          gutter,
		vScroll:         vScroll,
		hl:              hl,
		breakpointLines: make(map[int]bool),
	}

	// Wire gutter callbacks.
	gutter.GetBreakpointAt = func(line int) bool { return ew.breakpointLines[line] }
	gutter.GetCurrentLine = func() int { return ew.currentLine }

	// Wire scrollbar to editor.
	vScroll.OnChange = func(v int) {
		ew.editor.GotoLine(v, ew.editor.CursorRow())
	}

	// Keep scrollbar value in sync after every edit.
	editor.OnChange = func() {
		vScroll.SetValue(editor.TopLine())
		vScroll.Max = maxInt(0, editor.LineCount()-edH)
		ew.modified = true
		ew.updateTitle()
	}

	return ew
}

// Win returns the underlying *views.Window (for adding to the desktop).
func (ew *IdeEditorWindow) Win() *views.Window { return ew.win }

// Editor returns the underlying *views.Editor.
func (ew *IdeEditorWindow) Editor() *views.Editor { return ew.editor }

// SetFile loads content into the editor and updates the title.
func (ew *IdeEditorWindow) SetFile(path, content string) {
	ew.filePath = path
	ew.modified = false
	ew.editor.SetText(content)
	ew.updateTitle()
}

// SetErrorLines highlights the given 1-based line numbers as error lines.
// Pass nil or empty map to clear error highlighting.
func (ew *IdeEditorWindow) SetErrorLines(errorLines map[int]bool) {
	if len(errorLines) == 0 {
		ew.editor.Highlighter = ew.hl
	} else {
		ew.editor.Highlighter = &errorLineHighlighter{
			inner:      ew.hl,
			errorLines: errorLines,
		}
	}
}

// MarkModified marks the editor content as modified.
func (ew *IdeEditorWindow) MarkModified() {
	ew.modified = true
	ew.updateTitle()
}

func (ew *IdeEditorWindow) updateTitle() {
	name := ew.filePath
	if name == "" {
		name = "untitled.pas"
	} else {
		name = filepath.Base(name)
	}
	if ew.modified {
		name += " *"
	}
	ew.win.SetTitle(name)
}

// loadFile reads a file from disk into the editor.
func (ew *IdeEditorWindow) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	ew.SetFile(path, string(data))
	return nil
}

// saveFile writes the editor contents to disk.
func (ew *IdeEditorWindow) saveFile() error {
	if ew.filePath == "" {
		return nil
	}
	err := os.WriteFile(ew.filePath, []byte(ew.editor.GetText()), 0644)
	if err != nil {
		return err
	}
	ew.modified = false
	ew.updateTitle()
	return nil
}

// ToggleBreakpoint toggles a breakpoint on the given 1-based line.
// Returns true if a breakpoint was added, false if it was removed.
func (ew *IdeEditorWindow) ToggleBreakpoint(line int) bool {
	if ew.breakpointLines[line] {
		delete(ew.breakpointLines, line)
		return false
	}
	ew.breakpointLines[line] = true
	return true
}

// Breakpoints returns the set of lines that have breakpoints (1-based).
func (ew *IdeEditorWindow) Breakpoints() map[int]bool { return ew.breakpointLines }

// SetCurrentLine sets the current execution line (1-based; 0 = clear).
func (ew *IdeEditorWindow) SetCurrentLine(line int) { ew.currentLine = line }

// ClearCurrentLine removes the current execution indicator.
func (ew *IdeEditorWindow) ClearCurrentLine() { ew.currentLine = 0 }

// CursorLine returns the 1-based line the editor cursor is on.
func (ew *IdeEditorWindow) CursorLine() int { return ew.editor.CursorRow() + 1 }

// setNewFile clears the editor and resets to an untitled file.
func (ew *IdeEditorWindow) setNewFile() {
	ew.SetFile("", "program untitled;\nbegin\n  writeln('Hello, World!');\nend.\n")
	ew.modified = false
	ew.updateTitle()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
