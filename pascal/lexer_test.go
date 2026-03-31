package pascal

import "testing"

func TestLexerBasic(t *testing.T) {
	l := NewLexer("program hello; begin end.")
	tokens := []TokenKind{TkProgram, TkIdent, TkSemi, TkBegin, TkEnd, TkDot, TkEOF}
	for _, want := range tokens {
		tok := l.Next()
		if tok.Kind != want {
			t.Errorf("expected %v, got %v (%v)", want, tok.Kind, tok)
		}
	}
}

func TestLexerStrings(t *testing.T) {
	l := NewLexer("'hello world'")
	tok := l.Next()
	if tok.Kind != TkStr || tok.StrVal != "hello world" {
		t.Errorf("got %+v", tok)
	}
}

func TestLexerEscapedQuote(t *testing.T) {
	l := NewLexer("'it''s'")
	tok := l.Next()
	if tok.Kind != TkStr || tok.StrVal != "it's" {
		t.Errorf("got StrVal=%q", tok.StrVal)
	}
}

func TestLexerNumbers(t *testing.T) {
	l := NewLexer("123 456")
	t1 := l.Next()
	if t1.Kind != TkInt || t1.IntVal != 123 {
		t.Errorf("expected 123, got %+v", t1)
	}
	t2 := l.Next()
	if t2.Kind != TkInt || t2.IntVal != 456 {
		t.Errorf("expected 456, got %+v", t2)
	}
}

func TestLexerOperators(t *testing.T) {
	l := NewLexer(":= <> <= >=")
	expected := []TokenKind{TkAssign, TkNe, TkLe, TkGe}
	for _, want := range expected {
		tok := l.Next()
		if tok.Kind != want {
			t.Errorf("expected %v, got %v", want, tok.Kind)
		}
	}
}

func TestLexerComments(t *testing.T) {
	l := NewLexer("{ this is a comment } begin { another } end")
	t1 := l.Next()
	if t1.Kind != TkBegin {
		t.Errorf("expected begin after comment, got %+v", t1)
	}
	t2 := l.Next()
	if t2.Kind != TkEnd {
		t.Errorf("expected end after comment, got %+v", t2)
	}
}

func TestLexerStarParenComment(t *testing.T) {
	l := NewLexer("(* this is a comment *) begin")
	tok := l.Next()
	if tok.Kind != TkBegin {
		t.Errorf("expected begin, got %+v", tok)
	}
}

func TestLexerCaseInsensitive(t *testing.T) {
	l := NewLexer("BEGIN END PROGRAM")
	expected := []TokenKind{TkBegin, TkEnd, TkProgram}
	for _, want := range expected {
		tok := l.Next()
		if tok.Kind != want {
			t.Errorf("expected %v, got %v", want, tok.Kind)
		}
	}
}

func TestLexerLineNumbers(t *testing.T) {
	l := NewLexer("program\nhello")
	t1 := l.Next()
	if t1.Line != 1 {
		t.Errorf("line=%d want 1", t1.Line)
	}
	t2 := l.Next()
	if t2.Line != 2 {
		t.Errorf("line=%d want 2", t2.Line)
	}
}
