package ide

import (
	"unicode"

	"go-tp/tv/core"
)

// PascalHighlighter implements views.Highlighter for Turbo Pascal source.
type PascalHighlighter struct{}

type hlState int

const (
	hlStateNormal hlState = iota
	hlStateBlock          // inside { } comment
	hlStateStarParen      // inside (* *) comment
)

var (
	attrHlNormal  = core.MakeAttr(core.ColorYellow, core.ColorBlue)
	attrHlKeyword = core.MakeAttr(core.ColorWhite, core.ColorBlue)
	attrHlString  = core.MakeAttr(core.ColorLightGreen, core.ColorBlue)
	attrHlComment = core.MakeAttr(core.ColorCyan, core.ColorBlue)
	attrHlNumber  = core.MakeAttr(core.ColorLightCyan, core.ColorBlue)
)

var pascalKeywords = map[string]bool{
	"program": true, "var": true, "begin": true, "end": true,
	"if": true, "then": true, "else": true,
	"while": true, "do": true,
	"for": true, "to": true, "downto": true,
	"repeat": true, "until": true,
	"procedure": true, "function": true,
	"type": true, "const": true, "uses": true,
	"and": true, "or": true, "not": true,
	"div": true, "mod": true,
	"true": true, "false": true,
	"integer": true, "string": true, "boolean": true, "char": true,
}

// Highlight implements views.Highlighter.
func (h *PascalHighlighter) Highlight(lines []string) [][]core.Attr {
	result := make([][]core.Attr, len(lines))
	state := hlStateNormal
	for i, line := range lines {
		attrs, newState := highlightLine(line, state)
		result[i] = attrs
		state = newState
	}
	return result
}

func highlightLine(line string, state hlState) ([]core.Attr, hlState) {
	runes := []rune(line)
	attrs := make([]core.Attr, len(runes))
	for i := range attrs {
		attrs[i] = attrHlNormal
	}

	i := 0
	for i < len(runes) {
		ch := runes[i]

		// Inside block comment { }.
		if state == hlStateBlock {
			attrs[i] = attrHlComment
			if ch == '}' {
				state = hlStateNormal
			}
			i++
			continue
		}

		// Inside (* *) comment.
		if state == hlStateStarParen {
			attrs[i] = attrHlComment
			if ch == '*' && i+1 < len(runes) && runes[i+1] == ')' {
				attrs[i+1] = attrHlComment
				i += 2
				state = hlStateNormal
			} else {
				i++
			}
			continue
		}

		// Line comment //.
		if ch == '/' && i+1 < len(runes) && runes[i+1] == '/' {
			for j := i; j < len(runes); j++ {
				attrs[j] = attrHlComment
			}
			i = len(runes)
			continue
		}

		// Block comment {.
		if ch == '{' {
			attrs[i] = attrHlComment
			state = hlStateBlock
			i++
			continue
		}

		// Block comment (*.
		if ch == '(' && i+1 < len(runes) && runes[i+1] == '*' {
			attrs[i] = attrHlComment
			attrs[i+1] = attrHlComment
			i += 2
			state = hlStateStarParen
			continue
		}

		// String literal.
		if ch == '\'' {
			j := i + 1
			for j < len(runes) {
				if runes[j] == '\'' {
					if j+1 < len(runes) && runes[j+1] == '\'' {
						j += 2 // escaped quote
						continue
					}
					j++
					break
				}
				j++
			}
			for k := i; k < j; k++ {
				attrs[k] = attrHlString
			}
			i = j
			continue
		}

		// Number literal.
		if unicode.IsDigit(ch) {
			j := i
			for j < len(runes) && unicode.IsDigit(runes[j]) {
				j++
			}
			for k := i; k < j; k++ {
				attrs[k] = attrHlNumber
			}
			i = j
			continue
		}

		// Identifier or keyword.
		if unicode.IsLetter(ch) || ch == '_' {
			j := i
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			word := toLower(string(runes[i:j]))
			attr := attrHlNormal
			if pascalKeywords[word] {
				attr = attrHlKeyword
			}
			for k := i; k < j; k++ {
				attrs[k] = attr
			}
			i = j
			continue
		}

		i++
	}

	return attrs, state
}

func toLower(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		runes[i] = unicode.ToLower(r)
	}
	return string(runes)
}

// errorLineHighlighter wraps PascalHighlighter and overlays error-line coloring.
type errorLineHighlighter struct {
	inner      *PascalHighlighter
	errorLines map[int]bool // 1-based line numbers with errors
}

var attrErrorLine = core.MakeAttr(core.ColorWhite, core.ColorRed)

func (e *errorLineHighlighter) Highlight(lines []string) [][]core.Attr {
	result := e.inner.Highlight(lines)
	for lineIdx, attrs := range result {
		if e.errorLines[lineIdx+1] { // convert 0-based to 1-based
			for i := range attrs {
				attrs[i] = attrErrorLine
			}
		}
	}
	return result
}
