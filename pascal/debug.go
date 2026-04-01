package pascal

// DebugLine maps a source line to the first code byte of that line.
type DebugLine struct {
	Line     int // 1-based source line number
	CodeAddr int // byte offset within user code (after _start stub in .text)
}

// DebugVar describes one variable visible in a scope.
type DebugVar struct {
	Name      string
	Type      TypeKind
	RbpOffset int // e.g. −8, −16 for locals; +16, +24 for params
	ArrInfo   *ArrayInfo
	RecInfo   *RecordInfo
}

// DebugInfo is produced by the compiler and consumed by the IDE/debugger.
type DebugInfo struct {
	Lines []DebugLine
	Vars  []DebugVar
}
