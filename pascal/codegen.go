package pascal

// CodeGen is the interface implemented by all native code backends.
// The compiler calls these methods directly during single-pass parsing.
type CodeGen interface {
	// Literals.
	EmitLoadInt(n int64)
	EmitLoadStr(s string)
	EmitLoadBool(b bool)

	// Variables.
	EmitLoadVar(offset int)
	EmitStoreVar(offset int)

	// Push rax to the stack (saves lhs before rhs is evaluated for binary ops).
	EmitPush()

	// Arithmetic / comparison / logical.
	// On entry: rhs in rax, lhs on stack (from EmitPush).
	EmitBinaryOp(op TokenKind)
	EmitUnaryOp(op TokenKind)

	// Control flow. Returns placeholder address for PatchJump.
	EmitJumpFalse() int
	EmitJump() int
	EmitJumpTo(addr int)
	PatchJump(placeholder int)

	// Current emit position.
	CurrentAddr() int

	// Procedures.
	EmitProcEntry(frameSize int)
	EmitProcReturn()
	EmitCallProc(addr int)
	EmitCallBuiltin(idx int)

	// SetMainEntry records the code address of the program's main body.
	// Called by the compiler just before emitting the main body prologue.
	SetMainEntry(addr int)

	// Finalize produces the final binary / code buffer.
	Finalize() error
}

// Builtin indices shared between all backends and the compiler.
const (
	BuiltinWriteInt  = 0
	BuiltinWriteStr  = 1
	BuiltinWriteBool = 2
	BuiltinWriteln   = 3
	BuiltinReadInt   = 4
	BuiltinReadStr   = 5
	BuiltinHalt      = 6
)
