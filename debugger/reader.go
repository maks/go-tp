//go:build linux

package debugger

import (
	"fmt"
	"strconv"
	"strings"

	"go-tp/pascal"
)

// ReadVar reads the current value of a variable from the stopped child process
// and returns it as a human-readable string.
//
// rbp is the frame base pointer of the relevant stack frame (from PtraceRegs.Rbp).
// Variable address = rbp + v.RbpOffset (using signed arithmetic).
//
// For arrays, elements are stored at decreasing addresses from the base:
//   element[Low+k] is at rbp + RbpOffset − k×8
//
// For records, fields are stored at decreasing addresses from the base:
//   field with Offset f is at rbp + RbpOffset − f
func ReadVar(pid int, rbp uintptr, v pascal.DebugVar) (string, error) {
	// Use signed arithmetic to handle negative RbpOffset values (locals).
	addr := uintptr(int64(rbp) + int64(v.RbpOffset))

	switch v.Type {
	case pascal.TypeInteger:
		word, err := PeekWord(pid, addr)
		if err != nil {
			return "(error)", err
		}
		return strconv.FormatInt(int64(word), 10), nil

	case pascal.TypeBoolean:
		word, err := PeekWord(pid, addr)
		if err != nil {
			return "(error)", err
		}
		if word != 0 {
			return "true", nil
		}
		return "false", nil

	case pascal.TypeChar:
		word, err := PeekWord(pid, addr)
		if err != nil {
			return "(error)", err
		}
		b := byte(word)
		if b >= 32 && b < 127 {
			return "'" + string(rune(b)) + "'", nil
		}
		return fmt.Sprintf("#%d", b), nil

	case pascal.TypeString:
		// String variables hold a pointer to a null-terminated string.
		ptrVal, err := PeekWord(pid, addr)
		if err != nil {
			return "(error)", err
		}
		if ptrVal == 0 {
			return "(nil)", nil
		}
		str, err := readCString(pid, uintptr(ptrVal), 256)
		if err != nil {
			return "(error)", err
		}
		return "'" + str + "'", nil

	case pascal.TypeArray:
		return readArray(pid, addr, v.ArrInfo)

	case pascal.TypeRecord:
		return readRecord(pid, addr, v.RecInfo)

	default:
		return "(unknown)", nil
	}
}

// readCString reads a null-terminated string from the child process, up to maxLen bytes.
func readCString(pid int, addr uintptr, maxLen int) (string, error) {
	var result []byte
outer:
	for len(result) < maxLen {
		aligned := addr &^ 7
		word, err := PeekWord(pid, aligned)
		if err != nil {
			return string(result), err
		}
		start := int(addr - aligned)
		for off := start; off < 8; off++ {
			b := byte(word >> (off * 8))
			if b == 0 {
				break outer
			}
			result = append(result, b)
			if len(result) >= maxLen {
				break outer
			}
		}
		addr = aligned + 8
	}
	return string(result), nil
}

// readArray formats an array variable. Shows up to 8 elements.
func readArray(pid int, base uintptr, ai *pascal.ArrayInfo) (string, error) {
	if ai == nil {
		return "(array)", nil
	}
	count := ai.High - ai.Low + 1
	show := count
	if show > 8 {
		show = 8
	}
	parts := make([]string, show)
	for k := 0; k < show; k++ {
		// Element Low+k is at base - k*8 (elements stored at decreasing addresses).
		elemAddr := uintptr(int64(base) - int64(k)*8)
		word, err := PeekWord(pid, elemAddr)
		if err != nil {
			parts[k] = "?"
			continue
		}
		parts[k] = formatScalar(word, ai.ElemType)
	}
	s := "[" + strings.Join(parts, ", ")
	if count > 8 {
		s += ", ..."
	}
	return s + "]", nil
}

// readRecord formats a record variable. Shows all fields.
func readRecord(pid int, base uintptr, ri *pascal.RecordInfo) (string, error) {
	if ri == nil {
		return "(record)", nil
	}
	parts := make([]string, 0, len(ri.Fields))
	for _, f := range ri.Fields {
		// Field with Offset f.Offset is at base - f.Offset.
		fieldAddr := uintptr(int64(base) - int64(f.Offset))
		word, err := PeekWord(pid, fieldAddr)
		if err != nil {
			parts = append(parts, f.Name+": ?")
			continue
		}
		parts = append(parts, f.Name+": "+formatScalar(word, f.Type))
	}
	return "{" + strings.Join(parts, ", ") + "}", nil
}

// formatScalar formats a raw 64-bit word as the given scalar type.
func formatScalar(word uint64, t pascal.TypeKind) string {
	switch t {
	case pascal.TypeInteger:
		return strconv.FormatInt(int64(word), 10)
	case pascal.TypeBoolean:
		if word != 0 {
			return "true"
		}
		return "false"
	case pascal.TypeChar:
		b := byte(word)
		if b >= 32 && b < 127 {
			return "'" + string(rune(b)) + "'"
		}
		return fmt.Sprintf("#%d", b)
	default:
		return strconv.FormatInt(int64(word), 10)
	}
}
