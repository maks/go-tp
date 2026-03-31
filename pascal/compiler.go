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

// typeDesc carries type information produced by the type-parsing helpers.
type typeDesc struct {
	Kind    TypeKind
	Size    int // bytes (8 for scalars)
	ArrInfo *ArrayInfo
	RecInfo *RecordInfo
}

func (c *Compiler) parseProgram() {
	// program = "program" IDENT ";" [uses] [const] [type] [var]
	//           {proc_or_func} block "."
	c.expect(TkProgram)
	c.expect(TkIdent)
	c.expect(TkSemi)

	if c.tok.Kind == TkUses {
		c.parseUses()
	}
	for {
		switch c.tok.Kind {
		case TkConst:
			c.parseConst()
		case TkType:
			c.parseTypeSection()
		case TkVar:
			c.parseVarSection()
		case TkProcedure, TkFunction:
			c.parseProcOrFunc()
		default:
			goto doneDecls
		}
	}
doneDecls:
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
			val := c.tok.StrVal
			c.consume()
			sym := &Symbol{Name: name, Kind: SymConst, Type: TypeString, StrConst: val}
			c.scope.Declare(sym)
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
			if c.tok.Kind == TkIdent {
				names = append(names, c.tok.StrVal)
				c.consume()
			}
		}
		c.expect(TkColon)
		td := c.parseFullType()
		c.expect(TkSemi)
		for _, name := range names {
			if c.scope.LookupLocal(name) != nil {
				c.errorf("duplicate declaration: " + name)
				continue
			}
			sym := c.scope.DeclareVarSized(name, td.Kind, td.Size)
			sym.ArrInfo = td.ArrInfo
			sym.RecInfo = td.RecInfo
		}
	}
}

func (c *Compiler) parseType() TypeKind {
	return c.parseFullType().Kind
}

// parseFullType parses any type (simple, array, record, or named) and returns
// a typeDesc carrying size and optional ArrayInfo/RecordInfo.
func (c *Compiler) parseFullType() typeDesc {
	switch c.tok.Kind {
	case TkInteger:
		c.consume()
		return typeDesc{Kind: TypeInteger, Size: 8}
	case TkString:
		c.consume()
		return typeDesc{Kind: TypeString, Size: 8}
	case TkBoolean:
		c.consume()
		return typeDesc{Kind: TypeBoolean, Size: 8}
	case TkChar:
		c.consume()
		return typeDesc{Kind: TypeChar, Size: 8}
	case TkArray:
		c.consume() // "array"
		c.expect(TkLBrack)
		lowTok := c.tok
		c.expect(TkInt)
		c.expect(TkDotDot)
		highTok := c.tok
		c.expect(TkInt)
		c.expect(TkRBrack)
		c.expect(TkOf)
		elemTd := c.parseFullType()
		ai := &ArrayInfo{
			ElemType: elemTd.Kind,
			Low:      int(lowTok.IntVal),
			High:     int(highTok.IntVal),
		}
		count := ai.High - ai.Low + 1
		if count < 0 {
			count = 0
		}
		return typeDesc{Kind: TypeArray, Size: count * 8, ArrInfo: ai}
	case TkRecord:
		return c.parseRecordType()
	case TkIdent:
		name := c.tok.StrVal
		c.consume()
		sym := c.scope.Lookup(name)
		if sym == nil || sym.Kind != SymType {
			c.errorf("unknown type: " + name)
			return typeDesc{Kind: TypeUnknown, Size: 8}
		}
		size := 8
		if sym.RecInfo != nil {
			size = sym.RecInfo.Size
		} else if sym.ArrInfo != nil {
			count := sym.ArrInfo.High - sym.ArrInfo.Low + 1
			if count < 0 {
				count = 0
			}
			size = count * 8
		}
		return typeDesc{Kind: sym.Type, Size: size, ArrInfo: sym.ArrInfo, RecInfo: sym.RecInfo}
	default:
		c.errorf("expected type")
		c.consume()
		return typeDesc{Kind: TypeUnknown, Size: 8}
	}
}

// parseTypeSection parses a "type" declaration block.
func (c *Compiler) parseTypeSection() {
	c.consume() // "type"
	for c.tok.Kind == TkIdent {
		name := c.tok.StrVal
		c.consume()
		c.expect(TkEq)
		td := c.parseFullType()
		c.expect(TkSemi)
		sym := &Symbol{
			Name:    name,
			Kind:    SymType,
			Type:    td.Kind,
			ArrInfo: td.ArrInfo,
			RecInfo: td.RecInfo,
		}
		c.scope.Declare(sym)
	}
}

// parseRecordType parses "record field_list end" and returns its typeDesc.
func (c *Compiler) parseRecordType() typeDesc {
	c.consume() // "record"
	ri := &RecordInfo{}
	fieldOffset := 0
	for c.tok.Kind == TkIdent {
		var fieldNames []string
		fieldNames = append(fieldNames, c.tok.StrVal)
		c.consume()
		for c.tok.Kind == TkComma {
			c.consume()
			if c.tok.Kind == TkIdent {
				fieldNames = append(fieldNames, c.tok.StrVal)
				c.consume()
			}
		}
		c.expect(TkColon)
		td := c.parseFullType()
		for _, fn := range fieldNames {
			ri.Fields = append(ri.Fields, RecordField{
				Name:   fn,
				Type:   td.Kind,
				Offset: fieldOffset,
			})
			fieldOffset += 8
		}
		if c.tok.Kind == TkSemi {
			c.consume()
		}
	}
	c.expect(TkEnd)
	ri.Size = fieldOffset
	if ri.Size == 0 {
		ri.Size = 8 // minimum size
	}
	return typeDesc{Kind: TypeRecord, Size: ri.Size, RecInfo: ri}
}

// findField looks up a field by name in RecordInfo (case-insensitive).
func findField(ri *RecordInfo, name string) *RecordField {
	lname := toLower(name)
	for i := range ri.Fields {
		if toLower(ri.Fields[i].Name) == lname {
			return &ri.Fields[i]
		}
	}
	return nil
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

	// Collect parameters first, then assign offsets in reverse so that
	// left-to-right call-site argument pushing matches declaration order.
	// (First arg is pushed first → ends up deepest on stack → highest rbp offset.)
	type paramEntry struct {
		name string
		typ  TypeKind
	}
	var allParams []paramEntry
	if c.tok.Kind == TkLParen {
		c.consume()
		for c.tok.Kind != TkRParen {
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
				allParams = append(allParams, paramEntry{pname, typ})
			}
			if c.tok.Kind == TkSemi {
				c.consume()
			}
		}
		c.expect(TkRParen)
	}
	// Assign offsets: first declared param gets the highest positive rbp offset,
	// last declared param gets rbp+16 (immediately above saved-rbp+ret-addr).
	for i, p := range allParams {
		offset := 16 + (len(allParams)-1-i)*8
		c.scope.DeclareParam(p.name, p.typ, offset)
	}

	// Return type for functions (result stored as first local var at [rbp-8]).
	var retType TypeKind
	resultVarOffset := -8
	if isFunc && c.tok.Kind == TkColon {
		c.consume()
		retType = c.parseType()
		resSym := c.scope.DeclareVar(name, retType)
		resultVarOffset = resSym.Offset
	}
	c.expect(TkSemi)

	for c.tok.Kind == TkConst || c.tok.Kind == TkVar {
		switch c.tok.Kind {
		case TkConst:
			c.parseConst()
		case TkVar:
			c.parseVarSection()
		}
	}

	// Record procedure address.
	addr := c.gen.CurrentAddr()
	c.procAddrs[toLower(name)] = addr
	sym := &Symbol{Name: name, Kind: SymProc, Type: TypeVoid, Offset: addr}
	if isFunc {
		sym.Kind = SymFunc
		sym.Type = retType
	}
	outer.Declare(sym)

	frameSize := c.scope.FrameSize()
	c.gen.EmitProcEntry(frameSize)
	c.parseBlock()
	if isFunc {
		// Ensure the result variable is in rax before returning.
		c.gen.EmitLoadVar(resultVarOffset)
	}
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
			c.emitStackCleanup(len(args) * 8)
		}
		return
	}

	if sym.Kind == SymConst {
		// Constants can't be assigned.
		c.diagAt(line, col, "cannot assign to constant "+name)
		return
	}

	// Array element assignment: a[i] := expr
	if sym.Type == TypeArray && c.tok.Kind == TkLBrack && sym.ArrInfo != nil {
		adjustedBase := sym.Offset + sym.ArrInfo.Low*8
		c.emitLoadAddr(adjustedBase)     // rax = rbp + adjustedBase (base addr)
		c.gen.EmitPush()                 // push base (lhs for final subtraction)
		c.consume()                      // consume '['
		c.parseExpr()                    // index → rax
		c.gen.EmitPush()                 // push index (lhs for multiply)
		c.gen.EmitLoadInt(8)             // rax = 8
		c.gen.EmitBinaryOp(TkStar)      // pop index, rax = index * 8
		c.gen.EmitBinaryOp(TkMinus)     // pop base, rax = base - index*8
		c.expect(TkRBrack)
		// rax = &a[index]
		c.gen.EmitPush() // save element address
		c.expect(TkAssign)
		c.parseExpr() // value → rax
		c.emitPopRcxAndStore()
		return
	}

	// Record field assignment: p.field := expr
	if sym.Type == TypeRecord && c.tok.Kind == TkDot && sym.RecInfo != nil {
		c.consume() // consume '.'
		fieldName := c.tok.StrVal
		fieldLine, fieldCol := c.tok.Line, c.tok.Col
		c.expect(TkIdent)
		field := findField(sym.RecInfo, fieldName)
		if field == nil {
			c.diagAt(fieldLine, fieldCol, "unknown field: "+fieldName)
			return
		}
		c.expect(TkAssign)
		c.parseExpr()
		c.gen.EmitStoreVar(sym.Offset - field.Offset)
		return
	}

	// Simple assignment.
	c.expect(TkAssign)
	c.parseExpr()
	c.gen.EmitStoreVar(sym.Offset)
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
	// Increment or decrement the loop variable.
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
			if sym.Type == TypeString {
				c.gen.EmitLoadStr(sym.StrConst)
			} else {
				c.gen.EmitLoadInt(sym.Value)
			}
			return sym.Type
		}
		if sym.Kind == SymFunc {
			// Function call as expression.
			nArgs := 0
			if c.tok.Kind == TkLParen {
				c.consume()
				for c.tok.Kind != TkRParen {
					c.parseExpr()
					c.gen.EmitPush()
					nArgs++
					if c.tok.Kind == TkComma {
						c.consume()
					}
				}
				c.expect(TkRParen)
			}
			c.gen.EmitCallProc(sym.Offset)
			if nArgs > 0 {
				c.emitStackCleanup(nArgs * 8)
			}
			return sym.Type
		}
		// Array element read: a[i]
		if sym.Type == TypeArray && c.tok.Kind == TkLBrack && sym.ArrInfo != nil {
			ai := sym.ArrInfo
			adjustedBase := sym.Offset + ai.Low*8
			c.emitLoadAddr(adjustedBase) // rax = base
			c.gen.EmitPush()             // push base
			c.consume()                  // '['
			c.parseExpr()                // index → rax
			c.gen.EmitPush()             // push index
			c.gen.EmitLoadInt(8)         // rax = 8
			c.gen.EmitBinaryOp(TkStar)  // rax = index * 8
			c.gen.EmitBinaryOp(TkMinus) // rax = base - index*8 = &a[index]
			c.expect(TkRBrack)
			c.emitLoadFromAddr() // rax = a[index]
			return ai.ElemType
		}
		// Record field read: p.field
		if sym.Type == TypeRecord && c.tok.Kind == TkDot && sym.RecInfo != nil {
			c.consume() // '.'
			fieldName := c.tok.StrVal
			fieldLine, fieldCol := c.tok.Line, c.tok.Col
			c.expect(TkIdent)
			field := findField(sym.RecInfo, fieldName)
			if field == nil {
				c.diagAt(fieldLine, fieldCol, "unknown field: "+fieldName)
				return TypeUnknown
			}
			c.gen.EmitLoadVar(sym.Offset - field.Offset)
			return field.Type
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

// emitLoadFromAddr emits code to load [rax] into rax (indirect load).
func (c *Compiler) emitLoadFromAddr() {
	type loader interface{ EmitLoadFromAddr() }
	if l, ok := c.gen.(loader); ok {
		l.EmitLoadFromAddr()
	}
}

// emitPopRcxAndStore pops the saved address into rcx and stores rax there.
func (c *Compiler) emitPopRcxAndStore() {
	type storer interface{ EmitPopRcxAndStore() }
	if s, ok := c.gen.(storer); ok {
		s.EmitPopRcxAndStore()
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
