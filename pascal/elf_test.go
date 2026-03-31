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
