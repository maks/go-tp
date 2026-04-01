package x86_64

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Mach-O file layout (all offsets are in the output file):
//
//	Offset   Size  Description
//	     0     32  mach_header_64
//	    32    152  LC_SEGMENT_64 (__TEXT) + 1 × section_64 (__text)
//	   184    184  LC_UNIXTHREAD (sets rip = entry point)
//	   368      ·  _start stub + user code + builtins  (= fullCode)
//	     ·      ·  .rodata (16-byte aligned after fullCode)
const machoHeaderSize = 32 + 152 + 184 // = 368

// Base virtual address for the single Mach-O __TEXT segment.
// Using the conventional 64-bit macOS load address.
const machoBaseVA uint64 = 0x100000000

// finalizeMachO writes a minimal static Mach-O x86-64 executable.
// fullCode = _start | user code | builtins (already call-patched by Finalize).
// startSize is len(buildStart()) so we can locate string reloc sites.
func (g *CodeGen) finalizeMachO(fullCode []byte, startSize int) error {
	textFileOff := uint64(machoHeaderSize)
	textVA := machoBaseVA + textFileOff

	rodataFileOff := textFileOff + uint64(len(fullCode))
	rodataFileOff = (rodataFileOff + 15) &^ 15
	rodataVA := machoBaseVA + rodataFileOff

	// Patch rip-relative string loads (same arithmetic as ELF, different VAs).
	for _, sr := range g.strRelocs {
		shiftedOff := startSize + sr.codeOff
		instrEndVA := textVA + uint64(shiftedOff+4)
		targetVA := rodataVA + uint64(sr.rodataOff)
		pcRel := int64(targetVA) - int64(instrEndVA)
		binary.LittleEndian.PutUint32(fullCode[shiftedOff:], uint32(int32(pcRel)))
	}

	textPadSize := rodataFileOff - textFileOff - uint64(len(fullCode))
	fileSize := rodataFileOff + uint64(len(g.rodata))

	buf := make([]byte, 0, int(fileSize)+16)
	buf = appendMachOHeaders(buf, textVA /* entryVA */, uint64(len(fullCode)), fileSize)
	buf = append(buf, fullCode...)
	buf = append(buf, make([]byte, textPadSize)...)
	buf = append(buf, g.rodata...)

	if err := os.WriteFile(g.outputPath, buf, 0755); err != nil {
		return fmt.Errorf("write Mach-O binary: %w", err)
	}
	return nil
}

// appendMachOHeaders appends the three load commands that precede the code.
//
//   - entryVA:  virtual address of the first byte of code (_start)
//   - codeSize: size of the code block (fullCode = _start + user + builtins)
//   - fileSize: total file size (code + rodata padding + rodata)
func appendMachOHeaders(buf []byte, entryVA, codeSize, fileSize uint64) []byte {
	const (
		lcSegment64    uint32 = 0x19
		lcUnixthread   uint32 = 0x05
		segCmdSize     uint32 = 72 + 80 // LC_SEGMENT_64 + 1 × section_64
		thrCmdSize     uint32 = 184     // LC_UNIXTHREAD for x86_thread_state64
		threadStateSz  uint32 = 168     // 21 × uint64
		x86State64     uint32 = 4       // x86_THREAD_STATE64 flavor
		x86StateCount  uint32 = 42      // threadStateSz / 4
		sizeofCmds            = segCmdSize + thrCmdSize
		textFileOff    uint64 = machoHeaderSize
	)

	// ── mach_header_64 (32 bytes) ──────────────────────────────────────────
	h := make([]byte, 32)
	binary.LittleEndian.PutUint32(h[0:], 0xFEEDFACF)          // magic MH_MAGIC_64
	binary.LittleEndian.PutUint32(h[4:], 0x01000007)           // cputype  CPU_TYPE_X86_64
	binary.LittleEndian.PutUint32(h[8:], 0x00000003)           // cpusubtype CPU_SUBTYPE_ALL
	binary.LittleEndian.PutUint32(h[12:], 2)                   // filetype MH_EXECUTE
	binary.LittleEndian.PutUint32(h[16:], 2)                   // ncmds
	binary.LittleEndian.PutUint32(h[20:], sizeofCmds)          // sizeofcmds
	binary.LittleEndian.PutUint32(h[24:], 0x00000001)          // flags MH_NOUNDEFS
	buf = append(buf, h...)

	// ── LC_SEGMENT_64 (72 bytes) + section_64 __text (80 bytes) ───────────
	seg := make([]byte, segCmdSize)
	binary.LittleEndian.PutUint32(seg[0:], lcSegment64)
	binary.LittleEndian.PutUint32(seg[4:], segCmdSize)
	copy(seg[8:], "__TEXT") // segname (16-byte field, zero-padded by make)
	binary.LittleEndian.PutUint64(seg[24:], machoBaseVA)       // vmaddr
	binary.LittleEndian.PutUint64(seg[32:], fileSize)          // vmsize
	binary.LittleEndian.PutUint64(seg[40:], 0)                 // fileoff (segment starts at 0)
	binary.LittleEndian.PutUint64(seg[48:], fileSize)          // filesize
	binary.LittleEndian.PutUint32(seg[56:], 7)                 // maxprot  = rwx
	binary.LittleEndian.PutUint32(seg[60:], 5)                 // initprot = r-x
	binary.LittleEndian.PutUint32(seg[64:], 1)                 // nsects
	// flags = 0 at seg[68:]
	// section_64 __text starts at seg[72:]
	copy(seg[72:], "__text")                                    // sectname
	copy(seg[88:], "__TEXT")                                    // segname
	binary.LittleEndian.PutUint64(seg[104:], entryVA)          // addr (= textVA)
	binary.LittleEndian.PutUint64(seg[112:], codeSize)         // size
	binary.LittleEndian.PutUint32(seg[120:], uint32(textFileOff)) // file offset
	// align/reloff/nreloc/flags/reserved all 0 (zeroed by make)
	buf = append(buf, seg...)

	// ── LC_UNIXTHREAD (184 bytes) ──────────────────────────────────────────
	// Sets the initial CPU state. rip points to _start.
	thr := make([]byte, thrCmdSize)
	binary.LittleEndian.PutUint32(thr[0:], lcUnixthread)
	binary.LittleEndian.PutUint32(thr[4:], thrCmdSize)
	binary.LittleEndian.PutUint32(thr[8:], x86State64)         // flavor
	binary.LittleEndian.PutUint32(thr[12:], x86StateCount)     // count
	// x86_thread_state64_t at thr[16:]: 21 × uint64, all zero except rip.
	// rip is field index 16 (rax,rbx,rcx,rdx,rdi,rsi,rbp,rsp,r8..r15,rip).
	const ripOffset = 16 + 16*8 // header(16) + 16 regs × 8
	binary.LittleEndian.PutUint64(thr[ripOffset:], entryVA)
	buf = append(buf, thr...)

	return buf
}
