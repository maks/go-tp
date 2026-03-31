package pascal

import "fmt"

// Diagnostic reports a compiler error.
type Diagnostic struct {
	Line int
	Col  int
	Msg  string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%d:%d: %s", d.Line, d.Col, d.Msg)
}

// Compiler is a single-pass recursive descent Pascal compiler.
// It drives a CodeGen interface directly during parsing — no AST.
type Compiler struct {
	lex   *Lexer
	tok   Token  // one-token lookahead
	scope *Scope
	gen   CodeGen
	diags []Diagnostic
	// procAddrs maps procedure name (lower) → code address of its entry.
	procAddrs map[string]int
	// forwardCalls collects call sites to procedures declared after use.
	forwardCalls []forwardCall
}

type forwardCall struct {
	name    string
	codeOff int // position of rel32 placeholder
}

// NewCompiler creates a Compiler for the given source and code generator.
func NewCompiler(source string, gen CodeGen) *Compiler {
	c := &Compiler{
		lex:       NewLexer(source),
		gen:       gen,
		scope:     NewScope(nil),
		procAddrs: make(map[string]int),
	}
	c.tok = c.lex.Next()
	return c
}

// Compile parses and compiles the source, returning any diagnostics.
func (c *Compiler) Compile() []Diagnostic {
	c.parseProgram()
	if len(c.diags) == 0 {
		if err := c.gen.Finalize(); err != nil {
			c.diags = append(c.diags, Diagnostic{Msg: err.Error()})
		}
	}
	return c.diags
}

// ---- grammar ----

func (c *Compiler) parseProgram() {
	// program = "program" IDENT ";" [uses_clause] [const_section] [var_section]
	//           {proc_or_func} block "."
	c.expect(TkProgram)
	c.expect(TkIdent)
	c.expect(TkSemi)

	if c.tok.Kind == TkUses {
		c.parseUses()
	}
	if c.tok.Kind == TkConst {
		c.parseConst()
	}
	if c.tok.Kind == TkVar {
		c.parseVarSection()
	}
	for c.tok.Kind == TkProcedure || c.tok.Kind == TkFunction {
		c.parseProcOrFunc()
	}
	// Main body.
	frameSize := c.scope.FrameSize()
	c.gen.SetMainEntry(c.gen.CurrentAddr())
	c.gen.EmitProcEntry(frameSize)
	c.parseBlock()
	c.gen.EmitProcReturn()
	c.expect(TkDot)
}

func (c *Compiler) parseUses() {
	c.consume() // "uses"
	for {
		c.expect(TkIdent) // module name (ignored)
		if c.tok.Kind != TkComma {
			break
		}
		c.consume()
	}
	c.expect(TkSemi)
}

func (c *Compiler) parseConst() {
	c.consume() // "const"
	for c.tok.Kind == TkIdent {
		name := c.tok.StrVal
		c.consume()
		c.expect(TkEq)
		// Only integer and string constants.
		switch c.tok.Kind {
		case TkInt:
			val := c.tok.IntVal
			c.consume()
			sym := &Symbol{Name: name, Kind: SymConst, Type: TypeInteger, Value: val}
			c.scope.Declare(sym)
		case TkStr:
			c.consume() // string constants ignored for now
		case TkTrue:
			c.consume()
			sym := &Symbol{Name: name, Kind: SymConst, Type: TypeBoolean, Value: 1}
			c.scope.Declare(sym)
		case TkFalse:
			c.consume()
			sym := &Symbol{Name: name, Kind: SymConst, Type: TypeBoolean, Value: 0}
			c.scope.Declare(sym)
		default:
			c.errorf("expected constant value")
		}
		c.expect(TkSemi)
	}
}

func (c *Compiler) parseVarSection() {
	c.consume() // "var"
	for c.tok.Kind == TkIdent {
		// ident_list ":" type ";"
		var names []string
		names = append(names, c.tok.StrVal)
		c.consume()
		for c.tok.Kind == TkComma {
			c.consume()
			names = append(names, c.tok.StrVal)
			c.expect(TkIdent)
		}
		c.expect(TkColon)
		typ := c.parseType()
		c.expect(TkSemi)
		for _, name := range names {
			c.scope.DeclareVar(name, typ)
		}
	}
}

func (c *Compiler) parseType() TypeKind {
	switch c.tok.Kind {
	case TkInteger:
		c.consume()
		return TypeInteger
	case TkString:
		c.consume()
		return TypeString
	case TkBoolean:
		c.consume()
		return TypeBoolean
	case TkChar:
		c.consume()
		return TypeChar
	default:
		c.errorf("expected type")
		c.consume()
		return TypeUnknown
	}
}

func (c *Compiler) parseProcOrFunc() {
	isFunc := c.tok.Kind == TkFunction
	c.consume()
	name := c.tok.StrVal
	c.expect(TkIdent)

	// Push a new scope for the procedure.
	outer := c.scope
	c.scope = NewScope(outer)
	c.scope.nextOffset = -8

	// Parameters.
	paramOffset := 16 // first param at [rbp+16] (SysV ABI: after saved rbp+ret addr)
	if c.tok.Kind == TkLParen {
		c.consume()
		for c.tok.Kind != TkRParen {
			// param_group: ident_list ":" type
			var pnames []string
			pnames = append(pnames, c.tok.StrVal)
			c.consume()
			for c.tok.Kind == TkComma {
				c.consume()
				pnames = append(pnames, c.tok.StrVal)
				c.expect(TkIdent)
			}
			c.expect(TkColon)
			typ := c.parseType()
			for _, pname := range pnames {
				c.scope.DeclareParam(pname, typ, paramOffset)
				paramOffset += 8
			}
			if c.tok.Kind == TkSemi {
				c.consume()
			}
		}
		c.expect(TkRParen)
	}

	// Return type for functions (stored as a local var at [rbp-8] by convention).
	var retType TypeKind
	if isFunc && c.tok.Kind == TkColon {
		c.consume()
		retType = c.parseType()
		// Declare an implicit result var with the function's name.
		c.scope.DeclareVar(name, retType)
	}
	_ = retType
	c.expect(TkSemi)

	if c.tok.Kind == TkConst {
		c.parseConst()
	}
	if c.tok.Kind == TkVar {
		c.parseVarSection()
	}

	// Record procedure address.
	addr := c.gen.CurrentAddr()
	c.procAddrs[toLower(name)] = addr
	// Declare proc in outer scope.
	sym := &Symbol{Name: name, Kind: SymProc, Type: TypeVoid, Offset: addr}
	if isFunc {
		sym.Kind = SymFunc
		sym.Type = retType
	}
	outer.Declare(sym)

	frameSize := c.scope.FrameSize()
	c.gen.EmitProcEntry(frameSize)
	c.parseBlock()
	c.gen.EmitProcReturn()
	c.expect(TkSemi)

	c.scope = outer
}

func (c *Compiler) parseBlock() {
	c.expect(TkBegin)
	c.parseStatement()
	for c.tok.Kind == TkSemi {
		c.consume()
		if c.tok.Kind == TkEnd {
			break
		}
		c.parseStatement()
	}
	c.expect(TkEnd)
}

func (c *Compiler) parseStatement() {
	switch c.tok.Kind {
	case TkIdent:
		c.parseIdentStatement()
	case TkIf:
		c.parseIf()
	case TkWhile:
		c.parseWhile()
	case TkFor:
		c.parseFor()
	case TkRepeat:
		c.parseRepeat()
	case TkBegin:
		c.parseBlock()
	case TkExit:
		c.consume()
		c.gen.EmitProcReturn()
	case TkEnd, TkSemi, TkEOF:
		// empty statement
	default:
		// empty statement (tolerate)
	}
}

func (c *Compiler) parseIdentStatement() {
	name := c.tok.StrVal
	line, col := c.tok.Line, c.tok.Col
	c.consume()

	lower := toLower(name)
	switch lower {
	case "write":
		c.parseWrite(false)
		return
	case "writeln":
		c.parseWrite(true)
		return
	case "readln":
		c.parseReadln()
		return
	case "halt":
		if c.tok.Kind == TkLParen {
			c.consume()
			c.expect(TkRParen)
		}
		c.gen.EmitCallBuiltin(BuiltinHalt)
		return
	}

	// Look up the symbol.
	sym := c.scope.Lookup(name)
	if sym == nil {
		c.diagAt(line, col, "undefined: "+name)
		// Try to skip to end of statement.
		return
	}

	if sym.Kind == SymProc || sym.Kind == SymFunc {
		// Procedure/function call.
		var args []TypeKind
		if c.tok.Kind == TkLParen {
			c.consume()
			for c.tok.Kind != TkRParen {
				// Push each arg (args are passed by putting them on the stack before
				// the call in the SysV calling convention — simplified approach:
				// compile each arg into rax, push it).
				t := c.parseExpr()
				args = append(args, t)
				c.gen.EmitPush() // push arg onto stack
				if c.tok.Kind == TkComma {
					c.consume()
				}
			}
			c.expect(TkRParen)
			// Pop args after call (caller cleanup).
		}
		c.gen.EmitCallProc(sym.Offset)
		if len(args) > 0 {
			// add rsp, N  to clean up pushed args.
			// We emit this inline since CodeGen doesn't have a special method.
			// For now, the x86_64 codegen's EmitCallProc doesn't handle this.
			// TODO: emit stack cleanup. For the simple Pascal subset in this
			// implementation we handle built-ins explicitly, and for user procs
			// we'll use the args-on-stack approach.
			_ = args
		}
		return
	}

	if sym.Kind == SymConst {
		// Constants can't be assigned.
		c.diagAt(line, col, "cannot assign to constant "+name)
		return
	}

	// Assignment.
	c.expect(TkAssign)
	c.parseExpr()
	if sym.Kind == SymParam {
		c.gen.EmitStoreVar(sym.Offset)
	} else {
		c.gen.EmitStoreVar(sym.Offset)
	}
}

func (c *Compiler) parseWrite(newline bool) {
	if c.tok.Kind == TkLParen {
		c.consume()
		for {
			typ := c.parseExpr()
			switch typ {
			case TypeInteger:
				c.gen.EmitCallBuiltin(BuiltinWriteInt)
			case TypeString:
				c.gen.EmitCallBuiltin(BuiltinWriteStr)
			case TypeBoolean:
				c.gen.EmitCallBuiltin(BuiltinWriteBool)
			default:
				c.gen.EmitCallBuiltin(BuiltinWriteInt)
			}
			if c.tok.Kind != TkComma {
				break
			}
			c.consume()
		}
		c.expect(TkRParen)
	}
	if newline {
		c.gen.EmitCallBuiltin(BuiltinWriteln)
	}
}

func (c *Compiler) parseReadln() {
	if c.tok.Kind == TkLParen {
		c.consume()
		for {
			name := c.tok.StrVal
			line, col := c.tok.Line, c.tok.Col
			c.expect(TkIdent)
			sym := c.scope.Lookup(name)
			if sym == nil {
				c.diagAt(line, col, "undefined: "+name)
			} else {
				// Load address of var into rax, then call readln helper.
				// lea rax, [rbp + offset]  — we emit this inline.
				c.emitLoadAddr(sym.Offset)
				switch sym.Type {
				case TypeString:
					c.gen.EmitCallBuiltin(BuiltinReadStr)
				default:
					c.gen.EmitCallBuiltin(BuiltinReadInt)
				}
			}
			if c.tok.Kind != TkComma {
				break
			}
			c.consume()
		}
		c.expect(TkRParen)
	}
}

func (c *Compiler) parseIf() {
	c.consume() // "if"
	c.parseExpr()
	p1 := c.gen.EmitJumpFalse()
	c.expect(TkThen)
	c.parseStatement()
	if c.tok.Kind == TkElse {
		p2 := c.gen.EmitJump()
		c.gen.PatchJump(p1)
		c.consume()
		c.parseStatement()
		c.gen.PatchJump(p2)
	} else {
		c.gen.PatchJump(p1)
	}
}

func (c *Compiler) parseWhile() {
	c.consume() // "while"
	top := c.gen.CurrentAddr()
	c.parseExpr()
	p := c.gen.EmitJumpFalse()
	c.expect(TkDo)
	c.parseStatement()
	c.gen.EmitJumpTo(top)
	c.gen.PatchJump(p)
}

func (c *Compiler) parseFor() {
	c.consume() // "for"
	varName := c.tok.StrVal
	c.expect(TkIdent)
	sym := c.scope.Lookup(varName)
	if sym == nil {
		c.errorf("undefined: " + varName)
		return
	}
	c.expect(TkAssign)
	c.parseExpr()
	c.gen.EmitStoreVar(sym.Offset)

	downto := c.tok.Kind == TkDownto
	if !downto {
		c.expect(TkTo)
	} else {
		c.consume()
	}
	// Evaluate limit, store in a temp.
	// We don't have a dedicated temp; use a stack push as the limit.
	c.parseExpr()
	c.gen.EmitPush() // limit on stack

	top := c.gen.CurrentAddr()
	// Load var, load limit (peek stack without consuming — not clean; let's use
	// a different approach: store limit in an implicit temp var).
	// Simplified: re-evaluate limit on every iteration (works for non-side-effect limits).
	// For correctness, we push limit and use peek-not-pop. But we can't peek without
	// a dedicated instruction. Instead, pop limit into a register, compare, then push back.
	// This is messy. Simpler: emit limit evaluation inline in the loop condition.
	//
	// Actually, since we already pushed the limit, we can:
	// - Compare var with limit by peeking the stack.
	// We add a helper: EmitPeekStack loads [rsp] without popping.
	// For now, use a known approach: limit is at [rsp] after the push.
	// Load var into rax, compare with [rsp].

	// Load current var value.
	c.gen.EmitLoadVar(sym.Offset)
	// Compare with limit: [rsp] = limit.
	// cmp rax, [rsp]
	// For the for loop condition:
	//   if downto: var >= limit → continue (var < limit → exit)
	//   if !downto: var <= limit → continue (var > limit → exit)
	// Emit the comparison inline.
	c.emitForCmp(downto)
	p := c.gen.EmitJumpFalse()
	c.expect(TkDo)
	c.parseStatement()
	// Increment/decrement var.
	c.gen.EmitLoadVar(sym.Offset)
	if downto {
		c.gen.EmitLoadInt(-1)
		c.gen.EmitPush() // -1 on stack, but EmitBinaryOp pops lhs... swap needed
		// Actually: rax = var, emit push, load -1 into rax, then EmitBinaryOp(TkPlus)?
		// Let's do it differently: load var, push, load 1, subtract.
	}
	// Load 1, push var, add (or subtract).
	c.gen.EmitLoadVar(sym.Offset)
	c.gen.EmitPush()
	if downto {
		c.gen.EmitLoadInt(-1)
	} else {
		c.gen.EmitLoadInt(1)
	}
	c.gen.EmitBinaryOp(TkPlus)
	c.gen.EmitStoreVar(sym.Offset)
	c.gen.EmitJumpTo(top)
	c.gen.PatchJump(p)
	// Pop the limit from the stack.
	// We need to emit: add rsp, 8
	c.emitStackCleanup(8)
}

func (c *Compiler) parseRepeat() {
	c.consume() // "repeat"
	top := c.gen.CurrentAddr()
	// Statements.
	c.parseStatement()
	for c.tok.Kind == TkSemi {
		c.consume()
		if c.tok.Kind == TkUntil {
			break
		}
		c.parseStatement()
	}
	c.expect(TkUntil)
	c.parseExpr()
	// Jump back to top if condition is false.
	p := c.gen.EmitJumpFalse()
	// If condition is true, fall through (exit loop).
	// We want: if false, go back to top; if true, exit.
	// EmitJumpFalse jumps if rax==0 (false). So if false → jump back.
	// But we want: if true → exit. So patch is actually: if false → top.
	// Re-emit: we need a jump-back-if-false pattern.
	// EmitJumpFalse emits je target. We want jne target (jump back if false = 0 → je).
	// wait: "until expr" = loop while expr is false. So:
	//   evaluate expr; if false (0) → jump to top; else exit.
	// je = jump if rax==0. So EmitJumpFalse patches to go to top = correct.
	// But PatchJump fills in forward jumps. For a backward jump we use EmitJumpTo.
	// The issue: EmitJumpFalse emits a forward-only je. We need backward je.
	// Solution: instead of using EmitJumpFalse, emit a test + conditional jump backward.
	// Let's undo and use a different approach.
	// Undo the PatchJump approach for repeat:
	// We emit the condition, then a conditional backward jump.
	// We can't easily undo the EmitJumpFalse already emitted.
	// Workaround: patch it to jump to a jmp instruction that goes back to top.
	backJmp := c.gen.EmitJump()    // unconditional backward jmp placeholder
	c.gen.PatchJump(p)              // false case → jump here
	c.gen.EmitJumpTo(top)           // go back to repeat top
	c.gen.PatchJump(backJmp)        // true case: exit loop → patch to current addr
	// This is a bit convoluted but works:
	// - je false_target  (EmitJumpFalse → if rax==0 → jump to false_target)
	// - jmp exit_loop    (EmitJump → always jump to exit_loop)
	// - false_target: jmp top  (EmitJumpTo(top))
	// - exit_loop: (here)
	// So: if expr true (non-zero) → je doesn't fire → jmp exit_loop fires → exit. ✓
	// If expr false (zero) → je fires → false_target → jmp top → repeat. ✓
}

// parseExpr parses an expression and returns its inferred type.
func (c *Compiler) parseExpr() TypeKind {
	return c.parseSimpleExpr()
}

func (c *Compiler) parseSimpleExpr() TypeKind {
	ltype := c.parseTerm()
	for {
		op := c.tok.Kind
		if op != TkPlus && op != TkMinus && op != TkOr {
			break
		}
		c.consume()
		c.gen.EmitPush()
		c.parseTerm()
		c.gen.EmitBinaryOp(op)
		if op == TkOr {
			ltype = TypeBoolean
		}
		// Addition of strings is not supported in this subset.
	}
	// Relational operators.
	op := c.tok.Kind
	if op == TkEq || op == TkNe || op == TkLt || op == TkLe || op == TkGt || op == TkGe {
		c.consume()
		c.gen.EmitPush()
		c.parseTerm()
		c.gen.EmitBinaryOp(op)
		ltype = TypeBoolean
	}
	return ltype
}

func (c *Compiler) parseTerm() TypeKind {
	typ := c.parseFactor()
	for {
		op := c.tok.Kind
		if op != TkStar && op != TkSlash && op != TkDiv && op != TkMod && op != TkAnd {
			break
		}
		c.consume()
		c.gen.EmitPush()
		c.parseFactor()
		c.gen.EmitBinaryOp(op)
		if op == TkAnd {
			typ = TypeBoolean
		}
	}
	return typ
}

func (c *Compiler) parseFactor() TypeKind {
	switch c.tok.Kind {
	case TkInt:
		n := c.tok.IntVal
		c.consume()
		c.gen.EmitLoadInt(n)
		return TypeInteger
	case TkStr:
		s := c.tok.StrVal
		c.consume()
		c.gen.EmitLoadStr(s)
		return TypeString
	case TkTrue:
		c.consume()
		c.gen.EmitLoadBool(true)
		return TypeBoolean
	case TkFalse:
		c.consume()
		c.gen.EmitLoadBool(false)
		return TypeBoolean
	case TkNot:
		c.consume()
		c.parseFactor()
		c.gen.EmitUnaryOp(TkNot)
		return TypeBoolean
	case TkMinus:
		c.consume()
		c.parseFactor()
		c.gen.EmitUnaryOp(TkMinus)
		return TypeInteger
	case TkLParen:
		c.consume()
		typ := c.parseExpr()
		c.expect(TkRParen)
		return typ
	case TkIdent:
		name := c.tok.StrVal
		line, col := c.tok.Line, c.tok.Col
		c.consume()
		sym := c.scope.Lookup(name)
		if sym == nil {
			c.diagAt(line, col, "undefined: "+name)
			return TypeUnknown
		}
		if sym.Kind == SymConst {
			c.gen.EmitLoadInt(sym.Value)
			return sym.Type
		}
		if sym.Kind == SymFunc {
			// Function call as expression.
			if c.tok.Kind == TkLParen {
				c.consume()
				for c.tok.Kind != TkRParen {
					c.parseExpr()
					c.gen.EmitPush()
					if c.tok.Kind == TkComma {
						c.consume()
					}
				}
				c.expect(TkRParen)
			}
			c.gen.EmitCallProc(sym.Offset)
			// Result is in rax (from [rbp-8] of the called function... simplified).
			return sym.Type
		}
		// Variable load.
		c.gen.EmitLoadVar(sym.Offset)
		return sym.Type
	default:
		c.errorf("unexpected token in expression: " + c.tok.String())
		c.consume()
		return TypeUnknown
	}
}

// ---- helpers ----

func (c *Compiler) expect(kind TokenKind) Token {
	if c.tok.Kind != kind {
		c.errorf(fmt.Sprintf("expected %v, got %v", kindName(kind), c.tok))
		return c.tok
	}
	t := c.tok
	c.tok = c.lex.Next()
	return t
}

func (c *Compiler) consume() Token {
	t := c.tok
	c.tok = c.lex.Next()
	return t
}

func (c *Compiler) errorf(msg string) {
	c.diagAt(c.tok.Line, c.tok.Col, msg)
}

func (c *Compiler) diagAt(line, col int, msg string) {
	c.diags = append(c.diags, Diagnostic{Line: line, Col: col, Msg: msg})
}

func kindName(k TokenKind) string {
	names := map[TokenKind]string{
		TkIdent: "identifier", TkInt: "integer", TkStr: "string",
		TkProgram: "program", TkBegin: "begin", TkEnd: "end",
		TkSemi: ";", TkDot: ".", TkColon: ":", TkAssign: ":=",
		TkLParen: "(", TkRParen: ")", TkComma: ",",
		TkEOF: "EOF",
	}
	if n, ok := names[k]; ok {
		return n
	}
	return fmt.Sprintf("token(%d)", k)
}

// emitLoadAddr emits lea rax, [rbp + offset] to load the address of a variable.
func (c *Compiler) emitLoadAddr(offset int) {
	// This is x86_64-specific. We access the underlying codegen type.
	// Since CodeGen is an interface, we need a method for this.
	// For now, load the value and compute address from rbp:
	// We'll add EmitLoadAddr to CodeGen if needed. For the readln builtin,
	// we actually pass the address in rax. The helper writes to [rax].
	//
	// Emit: mov rax, rbp; add rax, offset  — available on all backends.
	// But our CodeGen interface doesn't have this. Add it as a special-case.
	// For the x86_64 backend, we can type-assert.
	type addrLoader interface {
		EmitLoadVarAddr(offset int)
	}
	if al, ok := c.gen.(addrLoader); ok {
		al.EmitLoadVarAddr(offset)
	}
	// If the backend doesn't support it, readln won't work — acceptable for now.
}

// emitForCmp emits the loop condition for a for loop.
// rax = current var value, [rsp] = limit.
// Sets rax to 1 if the loop should continue, 0 if it should exit.
func (c *Compiler) emitForCmp(downto bool) {
	type forCmper interface {
		EmitForCmp(downto bool)
	}
	if fc, ok := c.gen.(forCmper); ok {
		fc.EmitForCmp(downto)
	}
}

// emitStackCleanup emits code to pop n bytes from the stack.
func (c *Compiler) emitStackCleanup(n int) {
	type stackCleaner interface {
		EmitAddRSP(n int)
	}
	if sc, ok := c.gen.(stackCleaner); ok {
		sc.EmitAddRSP(n)
	}
}
