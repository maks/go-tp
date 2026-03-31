package pascal

import (
	"strings"
	"testing"
)

// mockGen is a no-op CodeGen used to test the parser without emitting code.
type mockGen struct {
	ops []string
}

func (m *mockGen) EmitLoadInt(n int64)         { m.ops = append(m.ops, "loadint") }
func (m *mockGen) EmitLoadStr(s string)         { m.ops = append(m.ops, "loadstr") }
func (m *mockGen) EmitLoadBool(b bool)          { m.ops = append(m.ops, "loadbool") }
func (m *mockGen) EmitLoadVar(offset int)       { m.ops = append(m.ops, "loadvar") }
func (m *mockGen) EmitStoreVar(offset int)      { m.ops = append(m.ops, "storevar") }
func (m *mockGen) EmitPush()                    { m.ops = append(m.ops, "push") }
func (m *mockGen) EmitBinaryOp(op TokenKind)   { m.ops = append(m.ops, "binop") }
func (m *mockGen) EmitUnaryOp(op TokenKind)    { m.ops = append(m.ops, "unop") }
func (m *mockGen) EmitJumpFalse() int          { m.ops = append(m.ops, "jf"); return 0 }
func (m *mockGen) EmitJump() int               { m.ops = append(m.ops, "jmp"); return 0 }
func (m *mockGen) EmitJumpTo(addr int)         { m.ops = append(m.ops, "jmpto") }
func (m *mockGen) PatchJump(placeholder int)   { m.ops = append(m.ops, "patch") }
func (m *mockGen) CurrentAddr() int            { return 0 }
func (m *mockGen) EmitProcEntry(frameSize int) { m.ops = append(m.ops, "entry") }
func (m *mockGen) EmitProcReturn()             { m.ops = append(m.ops, "ret") }
func (m *mockGen) EmitCallProc(addr int)       { m.ops = append(m.ops, "call") }
func (m *mockGen) EmitCallBuiltin(idx int)     { m.ops = append(m.ops, "builtin") }
func (m *mockGen) SetMainEntry(addr int)        {}
func (m *mockGen) Finalize() error             { return nil }

func TestCompilerHello(t *testing.T) {
	src := `program hello;
begin
  writeln('Hello, World!');
end.`
	gen := &mockGen{}
	diags := NewCompiler(src, gen).Compile()
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	found := false
	for _, op := range gen.ops {
		if op == "builtin" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected builtin call, ops=%v", gen.ops)
	}
}

func TestCompilerIfElse(t *testing.T) {
	src := `program test;
var x: integer;
begin
  x := 5;
  if x > 3 then writeln('big') else writeln('small');
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerWhile(t *testing.T) {
	src := `program test;
var i: integer;
begin
  i := 0;
  while i < 10 do begin i := i + 1; end;
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerUndefined(t *testing.T) {
	src := `program test; begin x := 5; end.`
	gen := &mockGen{}
	diags := NewCompiler(src, gen).Compile()
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for undefined 'x'")
	}
	if !strings.Contains(diags[0].Msg, "undefined") {
		t.Errorf("expected 'undefined' in diagnostic, got: %s", diags[0].Msg)
	}
}

func TestCompilerProcedure(t *testing.T) {
	src := `program test;
procedure greet;
begin writeln('hello'); end;
begin greet; end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	found := false
	for _, op := range gen.ops {
		if op == "call" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected call op, got: %v", gen.ops)
	}
}

func TestCompilerRepeat(t *testing.T) {
	src := `program test;
var i: integer;
begin
  i := 0;
  repeat i := i + 1; until i >= 3;
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}
