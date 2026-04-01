package x86_64

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	// Align to 16KB for Apple Silicon / Rosetta compatibility
	machoPageSize   = 0x4000
	machoHeaderSize = machoPageSize
	machoBaseVA     = 0x100000000
)

func alignUp(v, align uint64) uint64 {
	return (v + align - 1) &^ (align - 1)
}

// finalizeMachO writes a static Mach-O x86-64 executable that can be signed.
func (g *CodeGen) finalizeMachO(fullCode []byte, startSize int) error {
	textFileOff := uint64(machoHeaderSize)
	textVA := machoBaseVA + textFileOff

	rodataFileOff := textFileOff + uint64(len(fullCode))
	rodataFileOff = alignUp(rodataFileOff, 16)
	rodataVA := machoBaseVA + rodataFileOff

	// Patch rip-relative string loads
	for _, sr := range g.strRelocs {
		shiftedOff := startSize + sr.codeOff
		instrEndVA := textVA + uint64(shiftedOff+4)
		targetVA := rodataVA + uint64(sr.rodataOff)
		pcRel := int64(targetVA) - int64(instrEndVA)
		binary.LittleEndian.PutUint32(fullCode[shiftedOff:], uint32(int32(pcRel)))
	}

	codeSize := rodataFileOff - textFileOff
	fileSize := rodataFileOff + uint64(len(g.rodata))

	textVmsize := alignUp(fileSize, machoPageSize)

	buf := make([]byte, 0, int(fileSize))
	buf = appendMachOHeaders(buf, textVA /* entryVA */, codeSize, fileSize, textVmsize)

	// Pad header to machoHeaderSize
	pad := machoHeaderSize - len(buf)
	if pad < 0 {
		return fmt.Errorf("Mach-O headers too large")
	}
	buf = append(buf, make([]byte, pad)...)

	// Append code and rodata
	buf = append(buf, fullCode...)
	buf = append(buf, make([]byte, rodataFileOff - textFileOff - uint64(len(fullCode)))...)
	buf = append(buf, g.rodata...)

	if err := os.WriteFile(g.outputPath, buf, 0755); err != nil {
		return fmt.Errorf("write Mach-O binary: %w", err)
	}
	return nil
}

func appendMachOHeaders(buf []byte, entryVA, codeSize, fileSize, textVmsize uint64) []byte {
	const (
		lcSegment64    uint32 = 0x19
		lcUnixthread   uint32 = 0x05
		segCmdSize     uint32 = 72
		segTextCmdSize uint32 = 72 + 80 // LC_SEGMENT_64 + 1 × section_64
		thrCmdSize     uint32 = 184     // LC_UNIXTHREAD for x86_thread_state64
		x86State64     uint32 = 4       // x86_THREAD_STATE64 flavor
		x86StateCount  uint32 = 42      // threadStateSz / 4
		sizeofCmds            = segCmdSize + segTextCmdSize + segCmdSize + thrCmdSize
		textFileOff    uint64 = machoHeaderSize
	)

	// ── mach_header_64 (32 bytes) ──────────────────────────────────────────
	h := make([]byte, 32)
	binary.LittleEndian.PutUint32(h[0:], 0xFEEDFACF)          // magic MH_MAGIC_64
	binary.LittleEndian.PutUint32(h[4:], 0x01000007)           // cputype  CPU_TYPE_X86_64
	binary.LittleEndian.PutUint32(h[8:], 0x00000003)           // cpusubtype CPU_SUBTYPE_ALL
	binary.LittleEndian.PutUint32(h[12:], 2)                   // filetype MH_EXECUTE
	binary.LittleEndian.PutUint32(h[16:], 4)                   // ncmds (PAGEZERO, TEXT, LINKEDIT, THREAD)
	binary.LittleEndian.PutUint32(h[20:], sizeofCmds)          // sizeofcmds
	binary.LittleEndian.PutUint32(h[24:], 0x00000001)          // flags MH_NOUNDEFS
	buf = append(buf, h...)

	// ── LC_SEGMENT_64 (__PAGEZERO) (72 bytes) ─────────────────────────────
	pz := make([]byte, segCmdSize)
	binary.LittleEndian.PutUint32(pz[0:], lcSegment64)
	binary.LittleEndian.PutUint32(pz[4:], segCmdSize)
	copy(pz[8:], "__PAGEZERO")                                 // segname
	binary.LittleEndian.PutUint64(pz[24:], 0)                  // vmaddr
	binary.LittleEndian.PutUint64(pz[32:], machoBaseVA)        // vmsize
	binary.LittleEndian.PutUint64(pz[40:], 0)                  // fileoff
	binary.LittleEndian.PutUint64(pz[48:], 0)                  // filesize
	binary.LittleEndian.PutUint32(pz[56:], 0)                  // maxprot
	binary.LittleEndian.PutUint32(pz[60:], 0)                  // initprot
	binary.LittleEndian.PutUint32(pz[64:], 0)                  // nsects
	buf = append(buf, pz...)

	// ── LC_SEGMENT_64 (__TEXT) + section_64 __text (152 bytes) ────────────
	seg := make([]byte, segTextCmdSize)
	binary.LittleEndian.PutUint32(seg[0:], lcSegment64)
	binary.LittleEndian.PutUint32(seg[4:], segTextCmdSize)
	copy(seg[8:], "__TEXT")                                    // segname
	binary.LittleEndian.PutUint64(seg[24:], machoBaseVA)       // vmaddr
	binary.LittleEndian.PutUint64(seg[32:], textVmsize)        // vmsize
	binary.LittleEndian.PutUint64(seg[40:], 0)                 // fileoff (segment starts at 0)
	binary.LittleEndian.PutUint64(seg[48:], fileSize)          // filesize
	binary.LittleEndian.PutUint32(seg[56:], 7)                 // maxprot  = rwx
	binary.LittleEndian.PutUint32(seg[60:], 5)                 // initprot = r-x
	binary.LittleEndian.PutUint32(seg[64:], 1)                 // nsects
	
	// section_64 __text starts at seg[72:]
	copy(seg[72:], "__text")                                   // sectname
	copy(seg[88:], "__TEXT")                                   // segname
	binary.LittleEndian.PutUint64(seg[104:], entryVA)          // addr (= textVA)
	binary.LittleEndian.PutUint64(seg[112:], codeSize)         // size
	binary.LittleEndian.PutUint32(seg[120:], uint32(textFileOff)) // file offset
	binary.LittleEndian.PutUint32(seg[124:], 4)                // align = 2^4 = 16
	buf = append(buf, seg...)

	// ── LC_SEGMENT_64 (__LINKEDIT) (72 bytes) ─────────────────────────────
	le := make([]byte, segCmdSize)
	binary.LittleEndian.PutUint32(le[0:], lcSegment64)
	binary.LittleEndian.PutUint32(le[4:], segCmdSize)
	copy(le[8:], "__LINKEDIT")                                 // segname
	binary.LittleEndian.PutUint64(le[24:], machoBaseVA + textVmsize) // vmaddr
	binary.LittleEndian.PutUint64(le[32:], machoPageSize)      // vmsize
	binary.LittleEndian.PutUint64(le[40:], fileSize)           // fileoff
	binary.LittleEndian.PutUint64(le[48:], 0)                  // filesize
	binary.LittleEndian.PutUint32(le[56:], 7)                  // maxprot
	binary.LittleEndian.PutUint32(le[60:], 1)                  // initprot = r--
	binary.LittleEndian.PutUint32(le[64:], 0)                  // nsects
	buf = append(buf, le...)

	// ── LC_UNIXTHREAD (184 bytes) ──────────────────────────────────────────
	thr := make([]byte, thrCmdSize)
	binary.LittleEndian.PutUint32(thr[0:], lcUnixthread)
	binary.LittleEndian.PutUint32(thr[4:], thrCmdSize)
	binary.LittleEndian.PutUint32(thr[8:], x86State64)         // flavor
	binary.LittleEndian.PutUint32(thr[12:], x86StateCount)     // count
	const ripOffset = 16 + 16*8 // header(16) + 16 regs × 8
	binary.LittleEndian.PutUint64(thr[ripOffset:], entryVA)
	buf = append(buf, thr...)

	return buf
}
