// Package x86_64 implements the CodeGen interface for Linux x86-64.
// It encodes x86-64 machine code bytes directly and writes a static ELF64
// executable — no assembler or linker is involved.
package x86_64

import (
	"encoding/binary"
	"fmt"
	"os"

	"go-tp/pascal"
)

const numBuiltins = 7

// CodeGen encodes x86-64 instructions and writes an ELF64 binary.
type CodeGen struct {
	// code accumulates .text section bytes for user procedures.
	code []byte
	// rodata accumulates string literal bytes (null-terminated).
	rodata []byte
	// strOffsets maps string content to its rodata offset.
	strOffsets map[string]int
	// strRelocs records rip-relative patches needed for string loads.
	strRelocs []strReloc
	// procCalls records call sites targeting user procedures.
	procCalls []procCall
	// builtinCalls records call sites targeting builtin helpers.
	builtinCalls []builtinCall
	// outputPath is where the finished ELF64 binary is written.
	outputPath string
	// mainEntry is the code offset of the program's main body.
	mainEntry int
}

type strReloc struct {
	codeOff  int // offset of the rel32 field in code[]
	rodataOff int // offset of the string in rodata[]
}

type procCall struct {
	codeOff int // offset of the rel32 field in code[]
	target  int // target code offset
}

type builtinCall struct {
	codeOff int
	idx     int
}

// New creates a CodeGen that will write to outputPath.
func New(outputPath string) *CodeGen {
	return &CodeGen{
		outputPath: outputPath,
		strOffsets: make(map[string]int),
		mainEntry:  -1,
	}
}

func (g *CodeGen) SetMainEntry(addr int) { g.mainEntry = addr }

// ---- CodeGen interface ----

func (g *CodeGen) CurrentAddr() int { return len(g.code) }

func (g *CodeGen) EmitPush() {
	g.emit(0x50) // push rax
}

func (g *CodeGen) EmitLoadInt(n int64) {
	if n >= -2147483648 && n <= 2147483647 {
		// mov rax, imm32 (sign-extended): REX.W + C7 /0 imm32
		g.emit(0x48, 0xC7, 0xC0)
		g.emit32(int32(n))
	} else {
		// mov rax, imm64
		g.emit(0x48, 0xB8)
		g.emit64(n)
	}
}

func (g *CodeGen) EmitLoadBool(b bool) {
	if b {
		g.EmitLoadInt(1)
	} else {
		g.EmitLoadInt(0)
	}
}

func (g *CodeGen) EmitLoadStr(s string) {
	off := g.internString(s)
	// lea rax, [rip + rel32]  →  REX.W 8D 05 <rel32>
	g.emit(0x48, 0x8D, 0x05)
	site := len(g.code)
	g.emit32(0) // placeholder patched in Finalize
	g.strRelocs = append(g.strRelocs, strReloc{codeOff: site, rodataOff: off})
}

func (g *CodeGen) EmitLoadVar(offset int) {
	// mov rax, [rbp + offset]
	g.emitMovRaxRBP(offset)
}

func (g *CodeGen) EmitStoreVar(offset int) {
	// mov [rbp + offset], rax
	g.emitMovRBPRax(offset)
}

func (g *CodeGen) EmitBinaryOp(op pascal.TokenKind) {
	// lhs is on stack (from EmitPush), rhs in rax.
	g.emit(0x59) // pop rcx  (lhs → rcx)
	switch op {
	case pascal.TkPlus:
		g.emit(0x48, 0x01, 0xC8) // add rax, rcx
	case pascal.TkMinus:
		// lhs - rhs  = rcx - rax
		g.emit(0x48, 0x29, 0xC1) // sub rcx, rax
		g.emit(0x48, 0x89, 0xC8) // mov rax, rcx
	case pascal.TkStar:
		g.emit(0x48, 0x0F, 0xAF, 0xC1) // imul rax, rcx
	case pascal.TkDiv, pascal.TkSlash:
		// lhs / rhs  = rcx / rax  → swap so rax=lhs
		g.emit(0x48, 0x87, 0xC1) // xchg rax, rcx  (rax=lhs, rcx=rhs)
		g.emit(0x48, 0x99)       // cqo
		g.emit(0x48, 0xF7, 0xF9) // idiv rcx
	case pascal.TkMod:
		g.emit(0x48, 0x87, 0xC1) // xchg rax, rcx
		g.emit(0x48, 0x99)       // cqo
		g.emit(0x48, 0xF7, 0xF9) // idiv rcx
		g.emit(0x48, 0x89, 0xD0) // mov rax, rdx  (remainder)
	case pascal.TkEq:
		g.emitCmp(0x94) // sete
	case pascal.TkNe:
		g.emitCmp(0x95) // setne
	case pascal.TkLt:
		// lhs < rhs = rcx < rax: cmp rcx, rax then setl
		g.emit(0x48, 0x3B, 0xC8) // cmp rcx, rax
		g.emit(0x0F, 0x9C, 0xC0) // setl al
		g.emit(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		return
	case pascal.TkLe:
		g.emit(0x48, 0x3B, 0xC8)
		g.emit(0x0F, 0x9E, 0xC0) // setle al
		g.emit(0x48, 0x0F, 0xB6, 0xC0)
		return
	case pascal.TkGt:
		g.emit(0x48, 0x3B, 0xC8)
		g.emit(0x0F, 0x9F, 0xC0) // setg al
		g.emit(0x48, 0x0F, 0xB6, 0xC0)
		return
	case pascal.TkGe:
		g.emit(0x48, 0x3B, 0xC8)
		g.emit(0x0F, 0x9D, 0xC0) // setge al
		g.emit(0x48, 0x0F, 0xB6, 0xC0)
		return
	case pascal.TkAnd:
		g.emit(0x48, 0x21, 0xC8) // and rax, rcx
	case pascal.TkOr:
		g.emit(0x48, 0x09, 0xC8) // or rax, rcx
	}
}

// emitCmp emits: cmp rcx, rax; set<cc> al; movzx rax, al
// (used for equality comparisons where both operands are in rcx/rax)
func (g *CodeGen) emitCmp(setcc byte) {
	g.emit(0x48, 0x39, 0xC1)       // cmp rcx, rax
	g.emit(0x0F, setcc, 0xC0)      // set<cc> al
	g.emit(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
}

func (g *CodeGen) EmitUnaryOp(op pascal.TokenKind) {
	switch op {
	case pascal.TkMinus:
		g.emit(0x48, 0xF7, 0xD8) // neg rax
	case pascal.TkNot:
		g.emit(0x48, 0x85, 0xC0)       // test rax, rax
		g.emit(0x0F, 0x94, 0xC0)       // sete al
		g.emit(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
	}
}

func (g *CodeGen) EmitJumpFalse() int {
	g.emit(0x48, 0x85, 0xC0) // test rax, rax
	g.emit(0x0F, 0x84)       // je rel32
	site := len(g.code)
	g.emit32(0)
	return site
}

func (g *CodeGen) EmitJump() int {
	g.emit(0xE9) // jmp rel32
	site := len(g.code)
	g.emit32(0)
	return site
}

func (g *CodeGen) EmitJumpTo(addr int) {
	// jmp rel32 — compute relative to next instruction.
	g.emit(0xE9)
	rel := addr - (len(g.code) + 4)
	g.emit32(int32(rel))
}

func (g *CodeGen) PatchJump(placeholder int) {
	rel := len(g.code) - (placeholder + 4)
	binary.LittleEndian.PutUint32(g.code[placeholder:], uint32(int32(rel)))
}

func (g *CodeGen) EmitProcEntry(frameSize int) {
	g.emit(0x55)             // push rbp
	g.emit(0x48, 0x89, 0xE5) // mov rbp, rsp
	if frameSize > 0 {
		fs := align16(frameSize)
		if fs <= 127 {
			g.emit(0x48, 0x83, 0xEC, byte(fs))
		} else {
			g.emit(0x48, 0x81, 0xEC)
			g.emit32(int32(fs))
		}
	}
}

func (g *CodeGen) EmitProcReturn() {
	g.emit(0xC9) // leave
	g.emit(0xC3) // ret
}

func (g *CodeGen) EmitCallProc(addr int) {
	g.emit(0xE8)
	site := len(g.code)
	g.emit32(0)
	g.procCalls = append(g.procCalls, procCall{codeOff: site, target: addr})
}

func (g *CodeGen) EmitCallBuiltin(idx int) {
	g.emit(0xE8)
	site := len(g.code)
	g.emit32(0)
	g.builtinCalls = append(g.builtinCalls, builtinCall{codeOff: site, idx: idx})
}

// ---- ELF64 output ----

// Finalize patches relocations, appends builtins and _start, then writes ELF64.
func (g *CodeGen) Finalize() error {
	// Build builtin helpers.
	builtins := buildBuiltins()

	// Build _start stub (17 bytes: call + exit syscall).
	// _start is prepended so it's the ELF entry point.
	// It calls main (at startSize) then exits.
	startStub := buildStart()
	startSize := len(startStub)

	// Full code: _start | user code | builtins
	fullCode := make([]byte, 0, startSize+len(g.code)+len(builtins))
	fullCode = append(fullCode, startStub...)
	fullCode = append(fullCode, g.code...)
	fullCode = append(fullCode, builtins...)

	// Patch _start's call to main.
	// call at offset 0 (rel32 at offset 1): target = startSize + mainEntry.
	mainOff := startSize
	if g.mainEntry >= 0 {
		mainOff = startSize + g.mainEntry
	}
	rel := mainOff - 5
	binary.LittleEndian.PutUint32(fullCode[1:5], uint32(int32(rel)))

	// Patch user proc calls (offsets in g.code, now shifted by startSize).
	for _, pc := range g.procCalls {
		abs := startSize + pc.codeOff          // position of rel32 in fullCode
		target := startSize + pc.target        // target in fullCode
		pcRel := target - (abs + 4)
		binary.LittleEndian.PutUint32(fullCode[abs:], uint32(int32(pcRel)))
	}

	// Patch builtin calls.
	builtinBase := startSize + len(g.code)
	builtinOffsets := builtinCodeOffsets()
	for _, bc := range g.builtinCalls {
		abs := startSize + bc.codeOff
		target := builtinBase + builtinOffsets[bc.idx]
		pcRel := target - (abs + 4)
		binary.LittleEndian.PutUint32(fullCode[abs:], uint32(int32(pcRel)))
	}

	// ELF layout constants.
	const (
		elfHeaderSize = 64
		phdrSize      = 56
		numPhdrs      = 2
		headersSize   = elfHeaderSize + numPhdrs*phdrSize // = 176
	)
	const baseVA uint64 = 0x400000

	textFileOff := uint64(headersSize)
	textVA := baseVA + textFileOff

	// .rodata immediately after .text, aligned to 16.
	rodataFileOff := textFileOff + uint64(len(fullCode))
	rodataFileOff = (rodataFileOff + 15) &^ 15
	rodataVA := baseVA + rodataFileOff

	// Patch rip-relative string loads.
	for _, sr := range g.strRelocs {
		// The rel32 field is at fullCode[startSize + sr.codeOff].
		shiftedOff := startSize + sr.codeOff
		// Instruction end (next byte after rel32).
		instrEndVA := textVA + uint64(shiftedOff+4)
		targetVA := rodataVA + uint64(sr.rodataOff)
		pcRel := int64(targetVA) - int64(instrEndVA)
		binary.LittleEndian.PutUint32(fullCode[shiftedOff:], uint32(int32(pcRel)))
	}

	// File size.
	textPadSize := rodataFileOff - textFileOff - uint64(len(fullCode))
	fileSize := rodataFileOff + uint64(len(g.rodata))
	segSize := fileSize

	// ELF header entry point = _start = textVA.
	entryVA := textVA

	buf := make([]byte, 0, int(fileSize)+16)
	buf = appendELFHeader(buf, entryVA, numPhdrs, elfHeaderSize)
	// PHDR self-describing header.
	buf = appendPHDR(buf, 6 /*PT_PHDR*/, uint64(elfHeaderSize), baseVA+uint64(elfHeaderSize),
		uint64(numPhdrs*phdrSize), uint64(numPhdrs*phdrSize), 4 /*R*/, 8)
	// LOAD segment covering everything.
	buf = appendPHDR(buf, 1 /*PT_LOAD*/, 0, baseVA, segSize, segSize, 5 /*R|X*/, 0x200000)

	buf = append(buf, fullCode...)
	buf = append(buf, make([]byte, textPadSize)...) // padding between text and rodata
	buf = append(buf, g.rodata...)

	if err := os.WriteFile(g.outputPath, buf, 0755); err != nil {
		return fmt.Errorf("write ELF64 binary: %w", err)
	}
	return nil
}

// ---- builtin helper byte arrays ----

// buildBuiltins concatenates all builtin helpers in index order.
func buildBuiltins() []byte {
	var buf []byte
	for _, fn := range []func() []byte{
		buildWriteInt,
		buildWriteStr,
		buildWriteBool,
		buildWriteln,
		buildReadInt,
		buildReadStr,
		buildHalt,
	} {
		buf = append(buf, fn()...)
	}
	return buf
}

// builtinCodeOffsets returns the starting byte offset of each builtin within
// the concatenated builtin block returned by buildBuiltins.
func builtinCodeOffsets() [numBuiltins]int {
	var offsets [numBuiltins]int
	fns := []func() []byte{
		buildWriteInt, buildWriteStr, buildWriteBool, buildWriteln,
		buildReadInt, buildReadStr, buildHalt,
	}
	cur := 0
	for i, fn := range fns {
		offsets[i] = cur
		cur += len(fn())
	}
	return offsets
}

var _ = pascal.BuiltinWriteInt // ensure pascal package is accessible

// buildStart builds the _start stub (17 bytes).
// call rel32 at byte 0 (rel32 placeholder at bytes 1-4), then exit(0).
func buildStart() []byte {
	return []byte{
		0xE8, 0, 0, 0, 0,                // call main (rel32 patched later)
		0x48, 0x31, 0xFF,                 // xor rdi, rdi
		0x48, 0xC7, 0xC0, 60, 0, 0, 0,   // mov rax, 60 (SYS_exit)
		0x0F, 0x05,                       // syscall
	}
}

// write_int: write rax as signed decimal to stdout.
// Saved registers: rbx, rbp. Uses 24-byte stack buffer.
func buildWriteInt() []byte { return buildWriteIntVerified() }

// buildWriteIntVerified returns a verified x86-64 write_int helper.
// Encoded from the following assembly:
//
//	write_int:
//	  push rbx
//	  push rbp
//	  sub rsp, 32
//	  mov rbx, rax          ; value
//	  xor ebp, ebp          ; sign=0
//	  lea rax, [rsp+31]     ; rax = end of buffer
//	  test rbx, rbx
//	  jge .nonneg
//	  mov ebp, 1
//	  neg rbx
//	.nonneg:
//	  test rbx, rbx
//	  jnz .loop
//	  mov byte[rax], '0'
//	  dec rax
//	  jmp .sign
//	.loop:
//	  test rbx, rbx
//	  jz .sign
//	  mov rcx, 10
//	  xchg rax, rbx         ; rax=value, rbx=ptr
//	  xor edx, edx
//	  div rcx               ; rax=quot, rdx=rem
//	  xchg rax, rbx         ; rax=ptr, rbx=quot
//	  add dl, '0'
//	  mov [rax], dl
//	  dec rax
//	  jmp .loop
//	.sign:
//	  test ebp, ebp
//	  jz .write
//	  mov byte[rax], '-'
//	  dec rax
//	.write:
//	  lea rsi, [rax+1]      ; start of string
//	  lea rdx, [rsp+32]     ; one past end of buffer
//	  sub rdx, rsi          ; length
//	  mov edi, 1
//	  mov eax, 1
//	  syscall
//	  add rsp, 32
//	  pop rbp
//	  pop rbx
//	  ret
func buildWriteIntVerified() []byte {
	return []byte{
		// push rbx
		0x53,
		// push rbp
		0x55,
		// sub rsp, 32
		0x48, 0x83, 0xEC, 0x20,
		// mov rbx, rax
		0x48, 0x89, 0xC3,
		// xor ebp, ebp
		0x31, 0xED,
		// lea rax, [rsp+31]  : 48 8D 44 24 1F
		0x48, 0x8D, 0x44, 0x24, 0x1F,
		// test rbx, rbx
		0x48, 0x85, 0xDB,
		// jge .nonneg (+8)
		0x7D, 0x08,
		// mov ebp, 1
		0xBD, 0x01, 0x00, 0x00, 0x00,
		// neg rbx
		0x48, 0xF7, 0xDB,
		// .nonneg: test rbx, rbx
		0x48, 0x85, 0xDB,
		// jnz .loop (+8)
		0x75, 0x08,
		// mov byte[rax], '0'  : C6 00 30
		0xC6, 0x00, '0',
		// dec rax  : 48 FF C8
		0x48, 0xFF, 0xC8,
		// jmp .sign (+31): target=73, after-jmp=42, rel=73-42=31=0x1F
		0xEB, 0x1F,
		// .loop: test rbx, rbx
		0x48, 0x85, 0xDB,
		// jz .sign (+26): target=73, after-jz=47, rel=73-47=26=0x1A
		0x74, 0x1A,
		// mov rcx, 10  : 48 C7 C1 0A 00 00 00
		0x48, 0xC7, 0xC1, 0x0A, 0x00, 0x00, 0x00,
		// xchg rax, rbx  : 48 93
		0x48, 0x93,
		// xor edx, edx  : 31 D2
		0x31, 0xD2,
		// div rcx  : 48 F7 F1
		0x48, 0xF7, 0xF1,
		// xchg rax, rbx  : 48 93
		0x48, 0x93,
		// add dl, '0'  : 80 C2 30
		0x80, 0xC2, '0',
		// mov [rax], dl  : 88 10
		0x88, 0x10,
		// dec rax  : 48 FF C8
		0x48, 0xFF, 0xC8,
		// jmp .loop (-31): target=42, after-jmp=73, rel=42-73=-31=0xE1
		0xEB, 0xE1,
		// .sign: test ebp, ebp  : 85 ED
		0x85, 0xED,
		// jz .write (+6)
		0x74, 0x06,
		// mov byte[rax], '-'  : C6 00 2D
		0xC6, 0x00, 0x2D,
		// dec rax  : 48 FF C8
		0x48, 0xFF, 0xC8,
		// .write: lea rsi, [rax+1]  : 48 8D 70 01
		0x48, 0x8D, 0x70, 0x01,
		// lea rdx, [rsp+32]  : 48 8D 54 24 20
		0x48, 0x8D, 0x54, 0x24, 0x20,
		// sub rdx, rsi  : 48 29 F2
		0x48, 0x29, 0xF2,
		// mov edi, 1  : BF 01 00 00 00
		0xBF, 0x01, 0x00, 0x00, 0x00,
		// mov eax, 1  : B8 01 00 00 00
		0xB8, 0x01, 0x00, 0x00, 0x00,
		// syscall  : 0F 05
		0x0F, 0x05,
		// add rsp, 32  : 48 83 C4 20
		0x48, 0x83, 0xC4, 0x20,
		// pop rbp  : 5D
		0x5D,
		// pop rbx  : 5B
		0x5B,
		// ret  : C3
		0xC3,
	}
}

// write_str: write null-terminated string in rax to stdout.
func buildWriteStr() []byte {
	return []byte{
		// push rbx
		0x53,
		// mov rbx, rax    ; save ptr
		0x48, 0x89, 0xC3,
		// mov rcx, rax    ; strlen: scan for NUL
		0x48, 0x89, 0xC1,
		// .loop: cmp byte[rcx], 0; je .done; inc rcx; jmp .loop
		0x80, 0x39, 0x00, // cmp byte[rcx], 0
		0x74, 0x05,       // je .done (+5): after-je=12, .done=17, rel=5
		0x48, 0xFF, 0xC1, // inc rcx
		0xEB, 0xF6,       // jmp .loop (-10): after-jmp=17, .loop=7, rel=-10
		// .done: rdx = rcx - rbx  (length)
		0x48, 0x89, 0xCA,             // mov rdx, rcx
		0x48, 0x29, 0xDA,             // sub rdx, rbx
		// rsi = rbx (ptr)
		0x48, 0x89, 0xDE,             // mov rsi, rbx
		// write(1, rsi, rdx)
		0xBF, 0x01, 0x00, 0x00, 0x00, // mov edi, 1
		0xB8, 0x01, 0x00, 0x00, 0x00, // mov eax, 1
		0x0F, 0x05,                   // syscall
		// pop rbx; ret
		0x5B, 0xC3,
	}
}

// write_bool: write "true" or "false" for rax (0=false, non-zero=true).
func buildWriteBool() []byte {
	return []byte{
		// sub rsp, 8
		0x48, 0x83, 0xEC, 0x08,
		// test rax, rax
		0x48, 0x85, 0xC0,
		// jz .false (+29): target=38, after-jz=9, rel=38-9=29=0x1D
		0x74, 0x1D,
		// write "true" (4 bytes)
		0xC7, 0x04, 0x24, 't', 'r', 'u', 'e', // mov dword[rsp], "true"
		0xBF, 0x01, 0x00, 0x00, 0x00,         // mov edi, 1
		0x48, 0x89, 0xE6,                     // mov rsi, rsp
		0xBA, 0x04, 0x00, 0x00, 0x00,         // mov edx, 4
		0xB8, 0x01, 0x00, 0x00, 0x00,         // mov eax, 1
		0x0F, 0x05,                           // syscall
		// jmp .done (+32): target=70, after-jmp=38, rel=70-38=32=0x20
		0xEB, 0x20,                           // jmp .done
		// .false: write "false" (5 bytes)
		0xC7, 0x04, 0x24, 'f', 'a', 'l', 's', // "fals"
		0xC6, 0x44, 0x24, 0x04, 'e',          // byte[rsp+4]='e'
		0xBF, 0x01, 0x00, 0x00, 0x00,
		0x48, 0x89, 0xE6,
		0xBA, 0x05, 0x00, 0x00, 0x00,
		0xB8, 0x01, 0x00, 0x00, 0x00,
		0x0F, 0x05,
		// .done:
		0x48, 0x83, 0xC4, 0x08, // add rsp, 8
		0xC3,
	}
}

// writeln: write newline to stdout.
func buildWriteln() []byte {
	return []byte{
		0x48, 0x83, 0xEC, 0x08,             // sub rsp, 8
		0xC6, 0x04, 0x24, '\n',             // mov byte[rsp], '\n'
		0xBF, 0x01, 0x00, 0x00, 0x00,       // mov edi, 1
		0x48, 0x89, 0xE6,                   // mov rsi, rsp
		0xBA, 0x01, 0x00, 0x00, 0x00,       // mov edx, 1
		0xB8, 0x01, 0x00, 0x00, 0x00,       // mov eax, 1
		0x0F, 0x05,                         // syscall
		0x48, 0x83, 0xC4, 0x08,             // add rsp, 8
		0xC3,
	}
}

// read_int: read a decimal integer from stdin, store result at address in rax.
func buildReadInt() []byte {
	// On entry: rax = pointer to int64 variable.
	// Reads ASCII digits from stdin, parses, stores.
	return []byte{
		// push rbx; push r12; push r13
		0x53, 0x41, 0x54, 0x41, 0x55,
		// r12 = target pointer
		0x49, 0x89, 0xC4,
		// sub rsp, 32  (read buffer at rsp)
		0x48, 0x83, 0xEC, 0x20,
		// read(0, rsp, 20)
		0x48, 0x31, 0xC0,               // xor rax, rax (SYS_read=0)
		0x48, 0x31, 0xFF,               // xor rdi, rdi
		0x48, 0x89, 0xE6,               // mov rsi, rsp
		0xBA, 20, 0, 0, 0,              // mov edx, 20
		0x0F, 0x05,                     // syscall → rax=count
		// r13 = count
		0x49, 0x89, 0xC5,               // mov r13, rax
		// rbx = result = 0
		0x48, 0x31, 0xDB,               // xor rbx, rbx
		// rcx = i = 0
		0x48, 0x31, 0xC9,               // xor rcx, rcx
		// .loop: if rcx >= r13, break
		0x4C, 0x39, 0xE9,               // cmp rcx, r13
		// jae .done (+29): target=71, after-jae=42, rel=71-42=29=0x1D
		0x73, 0x1D,                     // jae .done
		// al = buf[rcx]
		0x0F, 0xB6, 0x04, 0x0E,         // movzx eax, byte[rsi+rcx]  (rsi=rsp)
		// if al < '0' || al > '9': break
		0x3C, '0',                      // cmp al, '0'
		// jb .done (+21): target=71, after-jb=50, rel=71-50=21=0x15
		0x72, 0x15,                     // jb .done
		0x3C, '9',                      // cmp al, '9'
		// ja .done (+17): target=71, after-ja=54, rel=71-54=17=0x11
		0x77, 0x11,                     // ja .done
		// rbx = rbx*10 + (al-'0')
		0x48, 0x6B, 0xDB, 0x0A,         // imul rbx, rbx, 10
		0x2C, '0',                      // sub al, '0'
		0x48, 0x0F, 0xB6, 0xC0,         // movzx rax, al
		0x48, 0x01, 0xC3,               // add rbx, rax
		0xFF, 0xC1,                     // inc ecx
		0xEB, 0xDE,                     // jmp .loop (-34)
		// .done: mov [r12], rbx
		0x4C, 0x89, 0xE0,               // mov rax, r12
		0x48, 0x89, 0x18,               // mov [rax], rbx
		// epilogue
		0x48, 0x83, 0xC4, 0x20,         // add rsp, 32
		0x41, 0x5D,                     // pop r13
		0x41, 0x5C,                     // pop r12
		0x5B,                           // pop rbx
		0xC3,
	}
}

// read_str: read a line from stdin, store null-terminated at address in rax.
func buildReadStr() []byte {
	return []byte{
		0x53,                               // push rbx
		0x48, 0x89, 0xC3,                   // mov rbx, rax
		0x48, 0x31, 0xC0,                   // xor rax, rax
		0x48, 0x31, 0xFF,                   // xor rdi, rdi
		0x48, 0x89, 0xDE,                   // mov rsi, rbx
		0xBA, 0xFE, 0, 0, 0,                // mov edx, 254
		0x0F, 0x05,                         // syscall → rax=count
		0x48, 0x01, 0xD8,                   // add rax, rbx
		0xC6, 0x00, 0x00,                   // mov byte[rax], 0
		0x5B, 0xC3,
	}
}

// halt: exit(0).
func buildHalt() []byte {
	return []byte{
		0xBF, 0x00, 0x00, 0x00, 0x00,       // mov edi, 0
		0xB8, 0x3C, 0x00, 0x00, 0x00,       // mov eax, 60
		0x0F, 0x05,
	}
}

// ---- low-level emit helpers ----

func (g *CodeGen) emit(bs ...byte) { g.code = append(g.code, bs...) }

func (g *CodeGen) emit32(v int32) {
	g.code = binary.LittleEndian.AppendUint32(g.code, uint32(v))
}

func (g *CodeGen) emit64(v int64) {
	g.code = binary.LittleEndian.AppendUint64(g.code, uint64(v))
}

func (g *CodeGen) emitMovRaxRBP(offset int) {
	// mov rax, [rbp + offset]
	if offset >= -128 && offset <= 127 {
		g.emit(0x48, 0x8B, 0x45, byte(int8(offset)))
	} else {
		g.emit(0x48, 0x8B, 0x85)
		g.emit32(int32(offset))
	}
}

func (g *CodeGen) emitMovRBPRax(offset int) {
	// mov [rbp + offset], rax
	if offset >= -128 && offset <= 127 {
		g.emit(0x48, 0x89, 0x45, byte(int8(offset)))
	} else {
		g.emit(0x48, 0x89, 0x85)
		g.emit32(int32(offset))
	}
}

func (g *CodeGen) internString(s string) int {
	if off, ok := g.strOffsets[s]; ok {
		return off
	}
	off := len(g.rodata)
	g.rodata = append(g.rodata, []byte(s)...)
	g.rodata = append(g.rodata, 0)
	g.strOffsets[s] = off
	return off
}

// ---- ELF64 helpers ----

func appendELFHeader(buf []byte, entry uint64, numPhdrs int, phoff int) []byte {
	h := make([]byte, 64)
	copy(h[0:], []byte{0x7F, 'E', 'L', 'F', 2, 1, 1, 0})
	binary.LittleEndian.PutUint16(h[16:], 2)          // ET_EXEC
	binary.LittleEndian.PutUint16(h[18:], 62)          // EM_X86_64
	binary.LittleEndian.PutUint32(h[20:], 1)           // EV_CURRENT
	binary.LittleEndian.PutUint64(h[24:], entry)       // e_entry
	binary.LittleEndian.PutUint64(h[32:], uint64(phoff)) // e_phoff
	binary.LittleEndian.PutUint16(h[52:], 64)          // e_ehsize
	binary.LittleEndian.PutUint16(h[54:], 56)          // e_phentsize
	binary.LittleEndian.PutUint16(h[56:], uint16(numPhdrs))
	binary.LittleEndian.PutUint16(h[58:], 64)          // e_shentsize
	return append(buf, h...)
}

func appendPHDR(buf []byte, ptype uint32, offset, vaddr, filesz, memsz uint64, flags uint32, align uint64) []byte {
	h := make([]byte, 56)
	binary.LittleEndian.PutUint32(h[0:], ptype)
	binary.LittleEndian.PutUint32(h[4:], flags)
	binary.LittleEndian.PutUint64(h[8:], offset)
	binary.LittleEndian.PutUint64(h[16:], vaddr)
	binary.LittleEndian.PutUint64(h[24:], vaddr) // paddr = vaddr
	binary.LittleEndian.PutUint64(h[32:], filesz)
	binary.LittleEndian.PutUint64(h[40:], memsz)
	binary.LittleEndian.PutUint64(h[48:], align)
	return append(buf, h...)
}

func align16(n int) int { return (n + 15) &^ 15 }

// EmitLoadVarAddr emits: lea rax, [rbp + offset]
func (g *CodeGen) EmitLoadVarAddr(offset int) {
	if offset >= -128 && offset <= 127 {
		// lea rax, [rbp + disp8]: 48 8D 45 <disp8>
		g.emit(0x48, 0x8D, 0x45, byte(int8(offset)))
	} else {
		// lea rax, [rbp + disp32]: 48 8D 85 <disp32>
		g.emit(0x48, 0x8D, 0x85)
		g.emit32(int32(offset))
	}
}

// EmitForCmp emits a for-loop condition check.
// On entry: rax = current var value, [rsp] = limit.
// Sets rax to 1 if the loop should continue, 0 if it should exit.
// For to: continue if var <= limit
// For downto: continue if var >= limit
func (g *CodeGen) EmitForCmp(downto bool) {
	// rcx = limit ([rsp] without modifying rsp)
	g.emit(0x48, 0x8B, 0x0C, 0x24) // mov rcx, [rsp]
	// cmp rax, rcx
	g.emit(0x48, 0x3B, 0xC1)       // cmp rax, rcx
	if downto {
		// setge al (continue if rax >= rcx)
		g.emit(0x0F, 0x9D, 0xC0)
	} else {
		// setle al (continue if rax <= rcx)
		g.emit(0x0F, 0x9E, 0xC0)
	}
	// movzx rax, al
	g.emit(0x48, 0x0F, 0xB6, 0xC0)
}

// EmitAddRSP emits: add rsp, n (for caller stack cleanup after proc calls).
func (g *CodeGen) EmitAddRSP(n int) {
	if n == 0 {
		return
	}
	if n <= 127 {
		g.emit(0x48, 0x83, 0xC4, byte(n))
	} else {
		g.emit(0x48, 0x81, 0xC4)
		g.emit32(int32(n))
	}
}
