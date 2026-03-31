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

func TestLexerKeywordsExtended(t *testing.T) {
	cases := []struct {
		src  string
		want TokenKind
	}{
		{"array", TkArray}, {"of", TkOf}, {"record", TkRecord},
		{"for", TkFor}, {"to", TkTo}, {"downto", TkDownto},
		{"repeat", TkRepeat}, {"until", TkUntil},
		{"div", TkDiv}, {"mod", TkMod},
		{"and", TkAnd}, {"or", TkOr}, {"not", TkNot},
		{"exit", TkExit}, {"type", TkType}, {"const", TkConst},
		{"uses", TkUses}, {"function", TkFunction},
	}
	for _, tc := range cases {
		tok := NewLexer(tc.src).Next()
		if tok.Kind != tc.want {
			t.Errorf("src=%q: want %v, got %v", tc.src, tc.want, tok.Kind)
		}
	}
}

func TestLexerBracketsAndRange(t *testing.T) {
	l := NewLexer("a[1..10]")
	expected := []TokenKind{TkIdent, TkLBrack, TkInt, TkDotDot, TkInt, TkRBrack}
	for _, want := range expected {
		tok := l.Next()
		if tok.Kind != want {
			t.Errorf("want %v, got %v (%v)", want, tok.Kind, tok)
		}
	}
}

func TestLexerArithmeticOps(t *testing.T) {
	l := NewLexer("+ - * /")
	expected := []TokenKind{TkPlus, TkMinus, TkStar, TkSlash}
	for _, want := range expected {
		tok := l.Next()
		if tok.Kind != want {
			t.Errorf("want %v, got %v", want, tok.Kind)
		}
	}
}

func TestLexerAllComparisonOps(t *testing.T) {
	l := NewLexer("= < > <> <= >=")
	expected := []TokenKind{TkEq, TkLt, TkGt, TkNe, TkLe, TkGe}
	for _, want := range expected {
		tok := l.Next()
		if tok.Kind != want {
			t.Errorf("want %v, got %v", want, tok.Kind)
		}
	}
}

func TestLexerDelimiters(t *testing.T) {
	l := NewLexer("; , : . ( )")
	expected := []TokenKind{TkSemi, TkComma, TkColon, TkDot, TkLParen, TkRParen}
	for _, want := range expected {
		tok := l.Next()
		if tok.Kind != want {
			t.Errorf("want %v, got %v", want, tok.Kind)
		}
	}
}

func TestLexerLineComment(t *testing.T) {
	l := NewLexer("begin // this is ignored\nend")
	t1 := l.Next()
	if t1.Kind != TkBegin {
		t.Errorf("expected begin, got %+v", t1)
	}
	t2 := l.Next()
	if t2.Kind != TkEnd {
		t.Errorf("expected end after line comment, got %+v", t2)
	}
}

func TestLexerPeek(t *testing.T) {
	l := NewLexer("begin end")
	p1 := l.Peek()
	p2 := l.Peek() // second peek returns same token
	if p1.Kind != TkBegin || p2.Kind != TkBegin {
		t.Errorf("peek should return begin twice, got %v and %v", p1.Kind, p2.Kind)
	}
	n := l.Next() // consumes begin
	if n.Kind != TkBegin {
		t.Errorf("next should return begin, got %v", n.Kind)
	}
	if l.Next().Kind != TkEnd {
		t.Errorf("expected end after consuming begin")
	}
}

func TestLexerBoolLiterals(t *testing.T) {
	l := NewLexer("true false TRUE FALSE")
	expected := []TokenKind{TkTrue, TkFalse, TkTrue, TkFalse}
	for _, want := range expected {
		tok := l.Next()
		if tok.Kind != want {
			t.Errorf("want %v, got %v", want, tok.Kind)
		}
	}
}

func TestLexerColNumbers(t *testing.T) {
	l := NewLexer("ab cd")
	t1 := l.Next() // "ab" at col 1
	if t1.Col != 1 {
		t.Errorf("col=%d want 1", t1.Col)
	}
	t2 := l.Next() // "cd" at col 4
	if t2.Col != 4 {
		t.Errorf("col=%d want 4", t2.Col)
	}
}
