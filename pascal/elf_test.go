package pascal_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go-tp/pascal"
	"go-tp/pascal/codegen/x86_64"
)

func isLinuxAMD64(t *testing.T) bool {
	t.Helper()
	out, err := exec.Command("uname", "-sm").Output()
	if err != nil || !strings.Contains(string(out), "Linux x86_64") {
		t.Skip("ELF test only runs on Linux x86-64")
		return false
	}
	return true
}

func compileAndRun(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "prog")
	gen := x86_64.New(bin)
	c := pascal.NewCompiler(src, gen)
	diags := c.Compile()
	if len(diags) > 0 {
		t.Fatalf("compile errors: %v", diags)
	}
	out, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	return strings.TrimRight(string(out), "\n")
}

func TestELFHelloWorld(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program hello;
begin
  writeln('Hello, World!');
end.`)
	if got != "Hello, World!" {
		t.Errorf("got %q, want %q", got, "Hello, World!")
	}
}

func TestELFArithmetic(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program arith;
var x: integer;
begin
  x := (3 + 4) * 2;
  writeln(x);
end.`)
	if got != "14" {
		t.Errorf("got %q, want %q", got, "14")
	}
}

func TestELFIfElse(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program iftest;
var x: integer;
begin
  x := 10;
  if x > 5 then writeln('big') else writeln('small');
end.`)
	if got != "big" {
		t.Errorf("got %q, want %q", got, "big")
	}
}

func TestELFWhileLoop(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program loop;
var i: integer;
begin
  i := 1;
  while i <= 3 do begin
    writeln(i);
    i := i + 1;
  end;
end.`)
	if got != "1\n2\n3" {
		t.Errorf("got %q, want %q", got, "1\n2\n3")
	}
}

func TestELFNegativeNumber(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program negtest;
var x: integer;
begin
  x := -42;
  writeln(x);
end.`)
	if got != "-42" {
		t.Errorf("got %q, want %q", got, "-42")
	}
}

func TestELFZero(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program zerotest;
var x: integer;
begin
  x := 0;
  writeln(x);
end.`)
	if got != "0" {
		t.Errorf("got %q, want %q", got, "0")
	}
}

func TestELFWriteBoolean(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program booltest;
begin
  writeln(true);
  writeln(false);
end.`)
	if got != "true\nfalse" {
		t.Errorf("got %q, want %q", got, "true\nfalse")
	}
}

func TestELFMultipleWrites(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program multiwrite;
begin
  write('Hello');
  write(', ');
  writeln('World!');
end.`)
	if got != "Hello, World!" {
		t.Errorf("got %q, want %q", got, "Hello, World!")
	}
}

func TestELFProcedure(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program proctest;
procedure sayHello;
begin
  writeln('hello from proc');
end;
begin
  sayHello;
  sayHello;
end.`)
	if got != "hello from proc\nhello from proc" {
		t.Errorf("got %q, want %q", got, "hello from proc\nhello from proc")
	}
}

func TestELFBooleanLogic(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program boollogic;
var a, b: boolean;
begin
  a := true;
  b := false;
  if a and (not b) then
    writeln('yes')
  else
    writeln('no');
end.`)
	if got != "yes" {
		t.Errorf("got %q, want %q", got, "yes")
	}
}

func TestELFArrayWriteRead(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program arrtest;
var a: array[1..5] of integer;
begin
  a[1] := 10;
  a[2] := 20;
  a[3] := 30;
  a[4] := 40;
  a[5] := 50;
  writeln(a[1]);
  writeln(a[3]);
  writeln(a[5]);
end.`)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 || lines[0] != "10" || lines[1] != "30" || lines[2] != "50" {
		t.Errorf("got %q, want 10/30/50", got)
	}
}

func TestELFArrayLoop(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program arrloop;
var a: array[1..4] of integer;
    i: integer;
    s: integer;
begin
  a[1] := 1; a[2] := 2; a[3] := 3; a[4] := 4;
  s := 0;
  for i := 1 to 4 do
    s := s + a[i];
  writeln(s);
end.`)
	if got != "10" {
		t.Errorf("got %q, want 10", got)
	}
}

func TestELFRecord(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program rectest;
type Point = record x: integer; y: integer; end;
var p: Point;
begin
  p.x := 3;
  p.y := 4;
  writeln(p.x);
  writeln(p.y);
end.`)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 || lines[0] != "3" || lines[1] != "4" {
		t.Errorf("got %q, want 3/4", got)
	}
}

func TestELFRecordArithmetic(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program rectarith;
type Point = record x: integer; y: integer; end;
var p: Point;
    d: integer;
begin
  p.x := 3;
  p.y := 4;
  d := p.x * p.x + p.y * p.y;
  writeln(d);
end.`)
	if got != "25" {
		t.Errorf("got %q, want 25", got)
	}
}

func TestELFForLoop(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program fortest;
var i, s: integer;
begin
  s := 0;
  for i := 1 to 5 do
    s := s + i;
  writeln(s);
end.`)
	if got != "15" {
		t.Errorf("got %q, want 15", got)
	}
}

func TestELFForDownto(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program downtotest;
var i: integer;
begin
  for i := 3 downto 1 do
    write(i);
  writeln;
end.`)
	if got != "321" {
		t.Errorf("got %q, want 321", got)
	}
}

func TestELFRepeatUntil(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program repeattest;
var i: integer;
begin
  i := 0;
  repeat
    i := i + 1;
  until i >= 5;
  writeln(i);
end.`)
	if got != "5" {
		t.Errorf("got %q, want 5", got)
	}
}

func TestELFFunction(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program functest;
function double(x: integer): integer;
begin
  double := x * 2;
end;
begin
  writeln(double(7));
end.`)
	if got != "14" {
		t.Errorf("got %q, want 14", got)
	}
}

func TestELFFunctionTwoParams(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program addtest;
function add(a: integer; b: integer): integer;
begin
  add := a + b;
end;
begin
  writeln(add(3, 4));
end.`)
	if got != "7" {
		t.Errorf("got %q, want 7", got)
	}
}

func TestELFFunctionInExpr(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program funcexpr;
function square(n: integer): integer;
begin
  square := n * n;
end;
begin
  writeln(square(3) + square(4));
end.`)
	if got != "25" {
		t.Errorf("got %q, want 25", got)
	}
}

func TestELFProcedureParams(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program paramstest;
procedure printPair(a: integer; b: integer);
begin
  write(a);
  write(' ');
  writeln(b);
end;
begin
  printPair(10, 20);
  printPair(3, 7);
end.`)
	if got != "10 20\n3 7" {
		t.Errorf("got %q, want %q", got, "10 20\n3 7")
	}
}

func TestELFProcLocalVars(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program localvartest;
procedure compute;
var x, y: integer;
begin
  x := 6;
  y := 7;
  writeln(x * y);
end;
begin
  compute;
end.`)
	if got != "42" {
		t.Errorf("got %q, want 42", got)
	}
}

func TestELFConst(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program consttest;
const
  LIMIT = 3;
  MSG = 'done';
var i: integer;
begin
  for i := 1 to LIMIT do
    writeln(i);
  writeln(MSG);
end.`)
	if got != "1\n2\n3\ndone" {
		t.Errorf("got %q, want 1/2/3/done", got)
	}
}

func TestELFDivMod(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program divmodtest;
begin
  writeln(17 div 5);
  writeln(17 mod 5);
end.`)
	if got != "3\n2" {
		t.Errorf("got %q, want 3/2", got)
	}
}

func TestELFOrOperator(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program ortest;
begin
  if false or true then writeln('yes') else writeln('no');
  if false or false then writeln('yes') else writeln('no');
end.`)
	if got != "yes\nno" {
		t.Errorf("got %q, want yes/no", got)
	}
}

func TestELFComparisonEq(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program eqtest;
var x: integer;
begin
  x := 5;
  if x = 5 then writeln('eq') else writeln('ne');
  if x = 6 then writeln('eq') else writeln('ne');
end.`)
	if got != "eq\nne" {
		t.Errorf("got %q, want eq/ne", got)
	}
}

func TestELFComparisonNe(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program netest;
var x: integer;
begin
  x := 5;
  if x <> 6 then writeln('ne') else writeln('eq');
  if x <> 5 then writeln('ne') else writeln('eq');
end.`)
	if got != "ne\neq" {
		t.Errorf("got %q, want ne/eq", got)
	}
}

func TestELFComparisonLt(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program lttest;
var x: integer;
begin
  x := 3;
  if x < 5 then writeln('lt') else writeln('ge');
  if x < 3 then writeln('lt') else writeln('ge');
end.`)
	if got != "lt\nge" {
		t.Errorf("got %q, want lt/ge", got)
	}
}

func TestELFComparisonGe(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program getest;
var x: integer;
begin
  x := 5;
  if x >= 5 then writeln('ge') else writeln('lt');
  if x >= 6 then writeln('ge') else writeln('lt');
end.`)
	if got != "ge\nlt" {
		t.Errorf("got %q, want ge/lt", got)
	}
}

func TestELFNestedIfElse(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program nestedif;
var x: integer;
begin
  x := 5;
  if x > 10 then
    writeln('big')
  else if x > 3 then
    writeln('mid')
  else
    writeln('small');
end.`)
	if got != "mid" {
		t.Errorf("got %q, want mid", got)
	}
}

func TestELFExit(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program exittest;
procedure printIfPositive(x: integer);
begin
  if x <= 0 then exit;
  writeln(x);
end;
begin
  printIfPositive(5);
  printIfPositive(-1);
  printIfPositive(3);
end.`)
	if got != "5\n3" {
		t.Errorf("got %q, want 5/3", got)
	}
}

func TestELFWritelnNoArgs(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	// writeln with no args just emits a newline
	got := compileAndRun(t, `program nltest;
begin
  write('a');
  writeln;
  write('b');
  writeln;
end.`)
	if got != "a\nb" {
		t.Errorf("got %q, want a/b", got)
	}
}

func TestELFRecord3Fields(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program rec3test;
type Triple = record a: integer; b: integer; c: integer; end;
var t: Triple;
begin
  t.a := 1;
  t.b := 2;
  t.c := 3;
  writeln(t.a + t.b + t.c);
end.`)
	if got != "6" {
		t.Errorf("got %q, want 6", got)
	}
}

func TestELFTwoRecordVars(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program tworectest;
type Point = record x: integer; y: integer; end;
var p1, p2: Point;
begin
  p1.x := 1; p1.y := 2;
  p2.x := 3; p2.y := 4;
  writeln(p1.x + p2.x);
  writeln(p1.y + p2.y);
end.`)
	if got != "4\n6" {
		t.Errorf("got %q, want 4/6", got)
	}
}

func TestELFStringVar(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program strvar;
var s: string;
begin
  s := 'hello';
  writeln(s);
  s := 'world';
  writeln(s);
end.`)
	if got != "hello\nworld" {
		t.Errorf("got %q, want hello/world", got)
	}
}

func TestELFZeroIndexedArray(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program zeroarr;
var a: array[0..2] of integer;
begin
  a[0] := 10;
  a[1] := 20;
  a[2] := 30;
  writeln(a[0] + a[1] + a[2]);
end.`)
	if got != "60" {
		t.Errorf("got %q, want 60", got)
	}
}

func TestELFArithmeticSubDiv(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program subdiv;
begin
  writeln(20 - 8);
  writeln(100 div 4);
end.`)
	if got != "12\n25" {
		t.Errorf("got %q, want 12/25", got)
	}
}

func TestELFMultipleProcedures(t *testing.T) {
	if !isLinuxAMD64(t) {
		return
	}
	got := compileAndRun(t, `program multiproc;
procedure greet(n: integer);
begin
  write('hello ');
  writeln(n);
end;
procedure bye;
begin
  writeln('bye');
end;
begin
  greet(1);
  greet(2);
  bye;
end.`)
	if got != "hello 1\nhello 2\nbye" {
		t.Errorf("got %q, want hello1/hello2/bye", got)
	}
}
