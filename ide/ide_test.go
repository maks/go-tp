package ide

import (
	"strings"
	"testing"

	"go-tp/pascal"
	"go-tp/tv/core"
	"go-tp/tv/views"
)

// ---- PascalHighlighter tests ----

func TestHighlighterKeywords(t *testing.T) {
	h := &PascalHighlighter{}
	lines := []string{"program hello;"}
	result := h.Highlight(lines)
	if len(result) != 1 {
		t.Fatalf("want 1 row of attrs, got %d", len(result))
	}
	attrs := result[0]
	// "program" (7 chars) should be keyword color
	for i := 0; i < 7; i++ {
		if attrs[i] != attrHlKeyword {
			t.Errorf("char %d: want keyword attr, got %v", i, attrs[i])
		}
	}
	// " " should be normal
	if attrs[7] != attrHlNormal {
		t.Errorf("space: want normal attr, got %v", attrs[7])
	}
}

func TestHighlighterString(t *testing.T) {
	h := &PascalHighlighter{}
	lines := []string{"writeln('hello');"}
	result := h.Highlight(lines)
	attrs := result[0]
	// 'hello' starts at index 8 (writeln( is 8 chars)
	startQuote := strings.Index(lines[0], "'")
	endQuote := strings.LastIndex(lines[0], "'")
	for i := startQuote; i <= endQuote; i++ {
		if attrs[i] != attrHlString {
			t.Errorf("char %d: want string attr, got %v", i, attrs[i])
		}
	}
}

func TestHighlighterLineComment(t *testing.T) {
	h := &PascalHighlighter{}
	lines := []string{"x := 1; // this is a comment"}
	result := h.Highlight(lines)
	attrs := result[0]
	commentStart := strings.Index(lines[0], "//")
	for i := commentStart; i < len(attrs); i++ {
		if attrs[i] != attrHlComment {
			t.Errorf("char %d: want comment attr, got %v", i, attrs[i])
		}
	}
}

func TestHighlighterBlockComment(t *testing.T) {
	h := &PascalHighlighter{}
	lines := []string{"{ this is a comment }"}
	result := h.Highlight(lines)
	attrs := result[0]
	for i := range attrs {
		if attrs[i] != attrHlComment {
			t.Errorf("char %d: want comment attr, got %v", i, attrs[i])
		}
	}
}

func TestHighlighterMultilineBlockComment(t *testing.T) {
	h := &PascalHighlighter{}
	lines := []string{
		"{ start of",
		"  middle",
		"  end } normal",
	}
	result := h.Highlight(lines)
	// First two lines: all comment
	for i, attr := range result[0] {
		if attr != attrHlComment {
			t.Errorf("line 0, char %d: want comment, got %v", i, attr)
		}
	}
	// Third line: comment up to and including '}'
	closeIdx := strings.Index(lines[2], "}")
	for i := 0; i <= closeIdx; i++ {
		if result[2][i] != attrHlComment {
			t.Errorf("line 2, char %d: want comment, got %v", i, result[2][i])
		}
	}
	// After '}': normal
	for i := closeIdx + 1; i < len(result[2]); i++ {
		if result[2][i] == attrHlComment {
			t.Errorf("line 2, char %d: want non-comment, got comment", i)
		}
	}
}

func TestHighlighterNumber(t *testing.T) {
	h := &PascalHighlighter{}
	lines := []string{"x := 42;"}
	result := h.Highlight(lines)
	attrs := result[0]
	numStart := strings.Index(lines[0], "42")
	if attrs[numStart] != attrHlNumber || attrs[numStart+1] != attrHlNumber {
		t.Errorf("want number attrs at %d, got %v %v", numStart, attrs[numStart], attrs[numStart+1])
	}
}

func TestHighlighterStarParenComment(t *testing.T) {
	h := &PascalHighlighter{}
	lines := []string{"(* a comment *) begin"}
	result := h.Highlight(lines)
	attrs := result[0]
	closeIdx := strings.Index(lines[0], "*)")
	for i := 0; i <= closeIdx+1; i++ {
		if attrs[i] != attrHlComment {
			t.Errorf("char %d: want comment, got %v", i, attrs[i])
		}
	}
	// "begin" starts after comment — should be keyword
	beginIdx := strings.Index(lines[0], "begin")
	if attrs[beginIdx] != attrHlKeyword {
		t.Errorf("begin keyword: want keyword attr, got %v", attrs[beginIdx])
	}
}

func TestErrorLineHighlighter(t *testing.T) {
	hl := &PascalHighlighter{}
	errHl := &errorLineHighlighter{
		inner:      hl,
		errorLines: map[int]bool{2: true}, // line 2 has an error (1-based)
	}
	lines := []string{"program test;", "x := undefined;", "end."}
	result := errHl.Highlight(lines)
	// Line index 1 (1-based: 2) should have error attrs.
	for _, attr := range result[1] {
		if attr != attrErrorLine {
			t.Errorf("error line: want attrErrorLine, got %v", attr)
		}
	}
	// Other lines should not be fully error-colored.
	hasNonError := false
	for _, attr := range result[0] {
		if attr != attrErrorLine {
			hasNonError = true
		}
	}
	if !hasNonError {
		t.Error("line 0 should not be all error-colored")
	}
}

// ---- LineNumberGutter tests ----

func TestGutterDraw(t *testing.T) {
	editor := views.NewEditor(core.Rect{W: 40, H: 10})
	editor.SetText("line1\nline2\nline3")
	gutter := NewLineNumberGutter(core.Rect{W: 3, H: 5}, editor)
	buf := core.NewDrawBuffer(3, 5, core.AttrNormal)
	gutter.Draw(buf)

	// Check first row contains "  1"
	var row0 string
	for x := 0; x < 3; x++ {
		if c := buf.At(x, 0); c != nil {
			row0 += string(c.Ch)
		}
	}
	if row0 != "  1" {
		t.Errorf("gutter row 0: want '  1', got %q", row0)
	}
}

func TestGutterCannotFocus(t *testing.T) {
	editor := views.NewEditor(core.Rect{W: 40, H: 10})
	gutter := NewLineNumberGutter(core.Rect{W: 3, H: 5}, editor)
	if gutter.CanFocus() {
		t.Error("gutter should not be focusable")
	}
}

// ---- IdeEditorWindow tests ----

func TestEditorWindowCreation(t *testing.T) {
	ew := NewIdeEditorWindow(core.Rect{X: 0, Y: 0, W: 80, H: 24})
	if ew == nil {
		t.Fatal("NewIdeEditorWindow returned nil")
	}
	if ew.Win() == nil {
		t.Error("Win() returned nil")
	}
	if ew.Editor() == nil {
		t.Error("Editor() returned nil")
	}
}

func TestEditorWindowSetFile(t *testing.T) {
	ew := NewIdeEditorWindow(core.Rect{X: 0, Y: 0, W: 80, H: 24})
	ew.SetFile("/tmp/test.pas", "program hello;\nbegin\nend.")
	if ew.editor.GetText() != "program hello;\nbegin\nend." {
		t.Error("SetFile did not set editor text")
	}
	if ew.modified {
		t.Error("SetFile should clear modified flag")
	}
}

func TestEditorWindowSetErrorLines(t *testing.T) {
	ew := NewIdeEditorWindow(core.Rect{X: 0, Y: 0, W: 80, H: 24})
	ew.SetFile("", "program x;\nbegin\nend.")
	// Should not panic
	ew.SetErrorLines(map[int]bool{2: true})
	ew.SetErrorLines(nil)
}

// ---- OutputWindow tests ----

func TestOutputWindowAppend(t *testing.T) {
	ow := NewOutputWindow(core.Rect{X: 0, Y: 0, W: 80, H: 10})
	ow.AppendLine("hello", attrOutputNormal)
	ow.AppendLine("world", attrOutputOK)
	if len(ow.lines) != 2 {
		t.Errorf("want 2 lines, got %d", len(ow.lines))
	}
	if ow.lines[0].text != "hello" {
		t.Errorf("line 0: want 'hello', got %q", ow.lines[0].text)
	}
}

func TestOutputWindowAppendError(t *testing.T) {
	ow := NewOutputWindow(core.Rect{X: 0, Y: 0, W: 80, H: 10})
	d := pascal.Diagnostic{Line: 3, Col: 5, Msg: "undefined variable"}
	ow.AppendError(d)
	if len(ow.lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(ow.lines))
	}
	if ow.lines[0].srcLine != 3 {
		t.Errorf("srcLine: want 3, got %d", ow.lines[0].srcLine)
	}
	if !strings.Contains(ow.lines[0].text, "undefined variable") {
		t.Errorf("line text should contain error message, got %q", ow.lines[0].text)
	}
}

func TestOutputWindowClear(t *testing.T) {
	ow := NewOutputWindow(core.Rect{X: 0, Y: 0, W: 80, H: 10})
	ow.AppendLine("a", attrOutputNormal)
	ow.AppendLine("b", attrOutputNormal)
	ow.Clear()
	if len(ow.lines) != 0 {
		t.Errorf("after Clear: want 0 lines, got %d", len(ow.lines))
	}
}

func TestOutputWindowGotoErr(t *testing.T) {
	ow := NewOutputWindow(core.Rect{X: 0, Y: 0, W: 80, H: 10})
	d := pascal.Diagnostic{Line: 5, Col: 1, Msg: "error"}
	ow.AppendError(d)
	ow.selected = 0

	var gotLine int
	ow.OnGotoErr = func(line int) { gotLine = line }
	ow.activateSelected()

	if gotLine != 5 {
		t.Errorf("OnGotoErr called with %d, want 5", gotLine)
	}
}

// ---- buildOutputPath tests ----

func TestBuildOutputPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "/tmp/pascal_out"},
		{"/home/user/hello.pas", "/home/user/hello"},
		{"/home/user/test.pas", "/home/user/test"},
	}
	for _, c := range cases {
		got := buildOutputPath(c.in)
		if got != c.want {
			t.Errorf("buildOutputPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
