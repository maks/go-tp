//go:build linux

package debugger

import (
	"testing"

	"go-tp/pascal"
)

// ---------------------------------------------------------------------------
// formatScalar tests (pure function, no ptrace needed)
// ---------------------------------------------------------------------------

func TestFormatScalarInteger(t *testing.T) {
	got := formatScalar(42, pascal.TypeInteger)
	if got != "42" {
		t.Errorf("want '42', got %q", got)
	}
	got = formatScalar(^uint64(0), pascal.TypeInteger) // all-bits-set = -1
	if got != "-1" {
		t.Errorf("want '-1', got %q", got)
	}
}

func TestFormatScalarBoolean(t *testing.T) {
	if got := formatScalar(0, pascal.TypeBoolean); got != "false" {
		t.Errorf("want 'false', got %q", got)
	}
	if got := formatScalar(1, pascal.TypeBoolean); got != "true" {
		t.Errorf("want 'true', got %q", got)
	}
}

func TestFormatScalarChar(t *testing.T) {
	if got := formatScalar('A', pascal.TypeChar); got != "'A'" {
		t.Errorf("want \"'A'\", got %q", got)
	}
	// Non-printable byte should produce #N notation.
	if got := formatScalar(1, pascal.TypeChar); got != "#1" {
		t.Errorf("want '#1', got %q", got)
	}
}

func TestFormatScalarDefault(t *testing.T) {
	// Unknown type should fall through to integer formatting.
	got := formatScalar(99, pascal.TypeKind(255))
	if got != "99" {
		t.Errorf("default case: want '99', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// ripToLine tests (pure calculation, no ptrace needed)
// ---------------------------------------------------------------------------

func makeTestSession(lines []pascal.DebugLine) *Session {
	s := &Session{
		info:     &pascal.DebugInfo{Lines: lines},
		loadBase: 0x400000,
	}
	return s
}

func TestRipToLineEmpty(t *testing.T) {
	s := makeTestSession(nil)
	if line := s.ripToLine(0x4000C1); line != 0 {
		t.Errorf("empty debug info: want 0, got %d", line)
	}
}

func TestRipToLineExact(t *testing.T) {
	const base = 0x400000 + elfTextOffset + startStubSize // 0x4000C1
	lines := []pascal.DebugLine{
		{Line: 3, CodeAddr: 0},
		{Line: 5, CodeAddr: 10},
		{Line: 7, CodeAddr: 20},
	}
	s := makeTestSession(lines)

	// Exactly at codeBase (offset 0) → line 3.
	if got := s.ripToLine(uint64(base)); got != 3 {
		t.Errorf("offset 0: want 3, got %d", got)
	}
	// Offset 10 → line 5.
	if got := s.ripToLine(uint64(base + 10)); got != 5 {
		t.Errorf("offset 10: want 5, got %d", got)
	}
	// Offset 25 → still line 7 (past last entry).
	if got := s.ripToLine(uint64(base + 25)); got != 7 {
		t.Errorf("offset 25: want 7, got %d", got)
	}
}

func TestRipToLineBefore(t *testing.T) {
	lines := []pascal.DebugLine{{Line: 2, CodeAddr: 0}}
	s := makeTestSession(lines)
	// rip before codeBase → 0.
	if got := s.ripToLine(0x100); got != 0 {
		t.Errorf("before codeBase: want 0, got %d", got)
	}
}

func TestRipToLineBetween(t *testing.T) {
	const base = uintptr(0x400000 + elfTextOffset + startStubSize)
	lines := []pascal.DebugLine{
		{Line: 1, CodeAddr: 0},
		{Line: 2, CodeAddr: 20},
	}
	s := makeTestSession(lines)
	// Offset 10 is between entries 0 and 1 → line 1.
	if got := s.ripToLine(uint64(base + 10)); got != 1 {
		t.Errorf("offset 10: want 1, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// codeAddrToVA / codeBase tests
// ---------------------------------------------------------------------------

func TestCodeBase(t *testing.T) {
	s := &Session{loadBase: 0x400000}
	const want = uintptr(0x400000) + uintptr(elfTextOffset+startStubSize)
	if got := s.codeBase(); got != want {
		t.Errorf("codeBase: want %#x, got %#x", want, got)
	}
}

func TestCodeAddrToVA(t *testing.T) {
	s := &Session{loadBase: 0x400000}
	va := s.codeAddrToVA(0)
	if va != s.codeBase() {
		t.Errorf("codeAddrToVA(0): want %#x, got %#x", s.codeBase(), va)
	}
	va = s.codeAddrToVA(10)
	if va != s.codeBase()+10 {
		t.Errorf("codeAddrToVA(10): want %#x, got %#x", s.codeBase()+10, va)
	}
}

// ---------------------------------------------------------------------------
// readLoadBase test — reads our own /proc/self/maps to verify parsing.
// ---------------------------------------------------------------------------

func TestReadLoadBaseSelf(t *testing.T) {
	// readLoadBase parses /proc/<pid>/maps. For this test we pass a bogus path
	// that won't match any mapping, so it should fall back to 0x400000.
	base := readLoadBase(0, "/nonexistent_binary_xyz")
	if base != 0x400000 {
		t.Errorf("fallback: want 0x400000, got %#x", base)
	}
}

// ---------------------------------------------------------------------------
// findBPAt tests
// ---------------------------------------------------------------------------

func TestFindBPAtFound(t *testing.T) {
	s := &Session{}
	bp := &Breakpoint{ID: 1, CodeAddr: 0xDEAD, Enabled: true}
	s.Breakpoints = []*Breakpoint{bp}

	found, idx := s.findBPAt(0xDEAD)
	if found != bp {
		t.Error("findBPAt: expected to find bp")
	}
	if idx != 0 {
		t.Errorf("findBPAt: expected idx 0, got %d", idx)
	}
}

func TestFindBPAtNotEnabled(t *testing.T) {
	s := &Session{}
	s.Breakpoints = []*Breakpoint{
		{ID: 1, CodeAddr: 0xDEAD, Enabled: false},
	}
	found, idx := s.findBPAt(0xDEAD)
	if found != nil {
		t.Error("findBPAt: disabled BP should not be found")
	}
	if idx != -1 {
		t.Errorf("findBPAt: expected idx -1, got %d", idx)
	}
}

func TestFindBPAtMiss(t *testing.T) {
	s := &Session{}
	s.Breakpoints = []*Breakpoint{
		{ID: 1, CodeAddr: 0xDEAD, Enabled: true},
	}
	found, idx := s.findBPAt(0xBEEF)
	if found != nil || idx != -1 {
		t.Error("findBPAt: unexpected hit for wrong address")
	}
}

// ---------------------------------------------------------------------------
// VarSnapshot / StopEvent value tests
// ---------------------------------------------------------------------------

func TestStopEventFields(t *testing.T) {
	ev := StopEvent{
		Reason:  StopBreakpoint,
		Line:    5,
		BPIndex: 1,
		Vars: []VarSnapshot{
			{Name: "x", Value: "42"},
			{Name: "ok", Value: "true"},
		},
	}
	if ev.Reason != StopBreakpoint {
		t.Errorf("Reason: want StopBreakpoint, got %v", ev.Reason)
	}
	if ev.Line != 5 {
		t.Errorf("Line: want 5, got %d", ev.Line)
	}
	if len(ev.Vars) != 2 {
		t.Errorf("Vars: want 2, got %d", len(ev.Vars))
	}
	if ev.Vars[0].Name != "x" || ev.Vars[0].Value != "42" {
		t.Errorf("Vars[0]: want x=42, got %v", ev.Vars[0])
	}
}
