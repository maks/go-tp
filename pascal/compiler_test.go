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

func TestCompilerArray(t *testing.T) {
	src := `program test;
var a: array[1..5] of integer;
begin
  a[1] := 42;
  a[3] := a[1] + 1;
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerRecord(t *testing.T) {
	src := `program test;
type Point = record x: integer; y: integer; end;
var p: Point;
begin
  p.x := 10;
  p.y := 20;
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerRecordReadField(t *testing.T) {
	src := `program test;
type Pair = record a: integer; b: integer; end;
var p: Pair;
var s: integer;
begin
  p.a := 3;
  p.b := 4;
  s := p.a + p.b;
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerArrayInlineVar(t *testing.T) {
	src := `program test;
var nums: array[0..9] of integer;
    i: integer;
begin
  i := 0;
  nums[i] := 99;
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerFor(t *testing.T) {
	src := `program test;
var i: integer;
begin
  for i := 1 to 5 do writeln(i);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerForDownto(t *testing.T) {
	src := `program test;
var i: integer;
begin
  for i := 5 downto 1 do writeln(i);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerConst(t *testing.T) {
	src := `program test;
const N = 42;
begin
  writeln(N);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	found := false
	for _, op := range gen.ops {
		if op == "loadint" {
			found = true
		}
	}
	if !found {
		t.Error("expected loadint for const, not found in ops")
	}
}

func TestCompilerFunction(t *testing.T) {
	src := `program test;
function double(x: integer): integer;
begin
  double := x * 2;
end;
var r: integer;
begin
  r := double(5);
  writeln(r);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerProcedureParams(t *testing.T) {
	src := `program test;
procedure printSum(a: integer; b: integer);
begin
  writeln(a + b);
end;
begin
  printSum(3, 4);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerExit(t *testing.T) {
	src := `program test;
procedure maybeExit(x: integer);
begin
  if x = 0 then exit;
  writeln(x);
end;
begin
  maybeExit(0);
  maybeExit(1);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerDivMod(t *testing.T) {
	src := `program test;
var a, b, c: integer;
begin
  a := 10;
  b := 3;
  c := a div b;
  writeln(c);
  c := a mod b;
  writeln(c);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerBooleanOps(t *testing.T) {
	src := `program test;
var x: boolean;
begin
  x := true or false;
  x := true and false;
  x := not true;
  writeln(x);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerNestedBlock(t *testing.T) {
	src := `program test;
var i: integer;
begin
  begin
    i := 1;
    begin
      i := i + 1;
    end;
  end;
  writeln(i);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerWriteNoArgs(t *testing.T) {
	src := `program test;
begin
  writeln('before');
  writeln;
  writeln('after');
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerUses(t *testing.T) {
	src := `program test;
uses crt, sysutils;
begin
  writeln('ok');
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerDuplicateDeclError(t *testing.T) {
	src := `program test;
var x: integer;
var x: integer;
begin end.`
	gen := &mockGen{}
	diags := NewCompiler(src, gen).Compile()
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for duplicate declaration")
	}
}

func TestCompilerConstAssignError(t *testing.T) {
	src := `program test;
const N = 5;
begin
  N := 10;
end.`
	gen := &mockGen{}
	diags := NewCompiler(src, gen).Compile()
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for const assignment")
	}
	if !strings.Contains(diags[0].Msg, "constant") {
		t.Errorf("expected 'constant' in diagnostic, got: %s", diags[0].Msg)
	}
}

func TestCompilerTypeSection(t *testing.T) {
	src := `program test;
type MyInt = record val: integer; end;
var r: MyInt;
begin
  r.val := 99;
  writeln(r.val);
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerMultipleConstTypeVar(t *testing.T) {
	src := `program test;
const A = 1;
type Pt = record x: integer; end;
var p: Pt;
const B = 2;
var n: integer;
begin
  p.x := A;
  n := B;
end.`
	gen := &mockGen{}
	if diags := NewCompiler(src, gen).Compile(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestCompilerUndefinedField(t *testing.T) {
	src := `program test;
type Pt = record x: integer; end;
var p: Pt;
begin
  p.z := 1;
end.`
	gen := &mockGen{}
	diags := NewCompiler(src, gen).Compile()
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for unknown field")
	}
}
