package pascal

import (
	"fmt"
	"strings"
)

// TokenKind identifies the type of a lexer token.
type TokenKind int

const (
	TkEOF     TokenKind = iota
	TkIdent              // identifier
	TkInt                // integer literal
	TkStr                // string literal
	TkTrue               // true
	TkFalse              // false
	// Keywords.
	TkProgram
	TkVar
	TkBegin
	TkEnd
	TkIf
	TkThen
	TkElse
	TkWhile
	TkDo
	TkFor
	TkTo
	TkDownto
	TkRepeat
	TkUntil
	TkProcedure
	TkFunction
	TkType
	TkConst
	TkUses
	TkExit
	TkNot
	TkAnd
	TkOr
	TkDiv
	TkMod
	TkArray
	TkOf
	TkRecord
	// Types.
	TkInteger
	TkString
	TkBoolean
	TkChar
	// Operators.
	TkAssign // :=
	TkEq     // =
	TkNe     // <>
	TkLt     // <
	TkLe     // <=
	TkGt     // >
	TkGe     // >=
	TkPlus   // +
	TkMinus  // -
	TkStar   // *
	TkSlash  // /
	// Delimiters.
	TkLParen // (
	TkRParen // )
	TkLBrack // [
	TkRBrack // ]
	TkComma  // ,
	TkSemi   // ;
	TkColon  // :
	TkDot    // .
	TkDotDot // ..
	TkCaret  // ^
)

// Token is one lexical unit.
type Token struct {
	Kind    TokenKind
	IntVal  int64
	StrVal  string
	Line    int
	Col     int
}

func (t Token) String() string {
	switch t.Kind {
	case TkIdent:
		return fmt.Sprintf("IDENT(%s)", t.StrVal)
	case TkInt:
		return fmt.Sprintf("INT(%d)", t.IntVal)
	case TkStr:
		return fmt.Sprintf("STR(%q)", t.StrVal)
	case TkEOF:
		return "EOF"
	default:
		return fmt.Sprintf("TK(%d)", t.Kind)
	}
}

// keywords maps lower-case keyword strings to TokenKind.
var keywords = map[string]TokenKind{
	"program":   TkProgram,
	"var":       TkVar,
	"begin":     TkBegin,
	"end":       TkEnd,
	"if":        TkIf,
	"then":      TkThen,
	"else":      TkElse,
	"while":     TkWhile,
	"do":        TkDo,
	"for":       TkFor,
	"to":        TkTo,
	"downto":    TkDownto,
	"repeat":    TkRepeat,
	"until":     TkUntil,
	"procedure": TkProcedure,
	"function":  TkFunction,
	"type":      TkType,
	"const":     TkConst,
	"uses":      TkUses,
	"exit":      TkExit,
	"not":       TkNot,
	"and":       TkAnd,
	"or":        TkOr,
	"div":       TkDiv,
	"mod":       TkMod,
	"array":     TkArray,
	"of":        TkOf,
	"record":    TkRecord,
	"true":      TkTrue,
	"false":     TkFalse,
	"integer":   TkInteger,
	"string":    TkString,
	"boolean":   TkBoolean,
	"char":      TkChar,
}

// Lexer tokenizes Pascal source code.
type Lexer struct {
	src  []rune
	pos  int
	line int
	col  int
	peek *Token // one-token lookahead
}

// NewLexer creates a Lexer for the given source.
func NewLexer(src string) *Lexer {
	return &Lexer{src: []rune(src), line: 1, col: 1}
}

// Peek returns the next token without consuming it.
func (l *Lexer) Peek() Token {
	if l.peek == nil {
		t := l.fetch()
		l.peek = &t
	}
	return *l.peek
}

// Next consumes and returns the next token.
func (l *Lexer) Next() Token {
	t := l.Peek()
	l.peek = nil
	return t
}

// Expect consumes the next token and returns an error if it is not kind.
func (l *Lexer) Expect(kind TokenKind) (Token, error) {
	t := l.Next()
	if t.Kind != kind {
		return t, fmt.Errorf("line %d col %d: expected %v got %v", t.Line, t.Col, kind, t)
	}
	return t, nil
}

func (l *Lexer) fetch() Token {
	l.skipWhitespaceAndComments()
	if l.pos >= len(l.src) {
		return Token{Kind: TkEOF, Line: l.line, Col: l.col}
	}
	ch := l.src[l.pos]
	line, col := l.line, l.col

	// String literal.
	if ch == '\'' {
		return l.readString(line, col)
	}

	// Number.
	if isDigit(ch) {
		return l.readInt(line, col)
	}

	// Identifier or keyword.
	if isLetter(ch) || ch == '_' {
		return l.readIdent(line, col)
	}

	// Symbols.
	l.advance()
	switch ch {
	case ':':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return Token{Kind: TkAssign, Line: line, Col: col}
		}
		return Token{Kind: TkColon, Line: line, Col: col}
	case '<':
		if l.pos < len(l.src) && l.src[l.pos] == '>' {
			l.advance()
			return Token{Kind: TkNe, Line: line, Col: col}
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return Token{Kind: TkLe, Line: line, Col: col}
		}
		return Token{Kind: TkLt, Line: line, Col: col}
	case '>':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return Token{Kind: TkGe, Line: line, Col: col}
		}
		return Token{Kind: TkGt, Line: line, Col: col}
	case '.':
		if l.pos < len(l.src) && l.src[l.pos] == '.' {
			l.advance()
			return Token{Kind: TkDotDot, Line: line, Col: col}
		}
		return Token{Kind: TkDot, Line: line, Col: col}
	case '(':
		return Token{Kind: TkLParen, Line: line, Col: col}
	case ')':
		return Token{Kind: TkRParen, Line: line, Col: col}
	case '[':
		return Token{Kind: TkLBrack, Line: line, Col: col}
	case ']':
		return Token{Kind: TkRBrack, Line: line, Col: col}
	case ',':
		return Token{Kind: TkComma, Line: line, Col: col}
	case ';':
		return Token{Kind: TkSemi, Line: line, Col: col}
	case '=':
		return Token{Kind: TkEq, Line: line, Col: col}
	case '+':
		return Token{Kind: TkPlus, Line: line, Col: col}
	case '-':
		return Token{Kind: TkMinus, Line: line, Col: col}
	case '*':
		return Token{Kind: TkStar, Line: line, Col: col}
	case '/':
		return Token{Kind: TkSlash, Line: line, Col: col}
	case '^':
		return Token{Kind: TkCaret, Line: line, Col: col}
	}
	// Unknown character — return an EOF-like token with an error embedded.
	return Token{Kind: TkEOF, StrVal: fmt.Sprintf("unexpected char %q", ch), Line: line, Col: col}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.advance()
			continue
		}
		// { } comment.
		if ch == '{' {
			l.advance()
			for l.pos < len(l.src) && l.src[l.pos] != '}' {
				l.advance()
			}
			if l.pos < len(l.src) {
				l.advance() // consume '}'
			}
			continue
		}
		// (* *) comment.
		if ch == '(' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '*' {
			l.advance()
			l.advance()
			for l.pos+1 < len(l.src) {
				if l.src[l.pos] == '*' && l.src[l.pos+1] == ')' {
					l.advance()
					l.advance()
					break
				}
				l.advance()
			}
			continue
		}
		// // comment (extension).
		if ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.advance()
			}
			continue
		}
		break
	}
}

func (l *Lexer) readString(line, col int) Token {
	l.advance() // consume opening '
	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\'' {
			l.advance()
			// '' is an escaped quote inside a string.
			if l.pos < len(l.src) && l.src[l.pos] == '\'' {
				sb.WriteRune('\'')
				l.advance()
				continue
			}
			break
		}
		sb.WriteRune(ch)
		l.advance()
	}
	return Token{Kind: TkStr, StrVal: sb.String(), Line: line, Col: col}
}

func (l *Lexer) readInt(line, col int) Token {
	var val int64
	for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		val = val*10 + int64(l.src[l.pos]-'0')
		l.advance()
	}
	return Token{Kind: TkInt, IntVal: val, Line: line, Col: col}
}

func (l *Lexer) readIdent(line, col int) Token {
	start := l.pos
	for l.pos < len(l.src) && (isLetter(l.src[l.pos]) || isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.advance()
	}
	name := string(l.src[start:l.pos])
	lower := strings.ToLower(name)
	if kind, ok := keywords[lower]; ok {
		return Token{Kind: kind, StrVal: name, Line: line, Col: col}
	}
	return Token{Kind: TkIdent, StrVal: name, Line: line, Col: col}
}

func (l *Lexer) advance() {
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func isDigit(ch rune) bool  { return ch >= '0' && ch <= '9' }
func isLetter(ch rune) bool { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') }
