package pascal

// TypeKind identifies the Pascal type of a symbol.
type TypeKind int

const (
	TypeUnknown TypeKind = iota
	TypeInteger
	TypeString
	TypeBoolean
	TypeChar
	TypeVoid  // for procedures
	TypeArray // array[lo..hi] of T
	TypeRecord
)

// SymbolKind identifies how a symbol was declared.
type SymbolKind int

const (
	SymVar SymbolKind = iota
	SymParam
	SymProc
	SymFunc
	SymConst
	SymType // named type alias
)

// ArrayInfo describes an array type.
type ArrayInfo struct {
	ElemType TypeKind
	Low, High int
}

// RecordField is one field of a record type.
// Offset is positive (0 for first field, 8 for second, …).
// Actual rbp-relative address = sym.Offset − field.Offset.
type RecordField struct {
	Name   string
	Type   TypeKind
	Offset int
}

// RecordInfo describes a record type.
type RecordInfo struct {
	Fields []RecordField
	Size   int // total size in bytes
}

// Symbol is one entry in a scope.
type Symbol struct {
	Name     string
	Kind     SymbolKind
	Type     TypeKind
	Offset   int    // stack offset for Var/Param, code address for Proc/Func
	Value    int64  // for integer/boolean constants
	StrConst string // for string constants
	ArrInfo  *ArrayInfo
	RecInfo  *RecordInfo
}

// Scope is a single lexical scope. Scopes are chained via parent.
type Scope struct {
	parent  *Scope
	symbols map[string]*Symbol
	// nextOffset tracks the next available frame slot (grows negatively).
	nextOffset int
}

// NewScope creates a new scope, optionally with a parent.
func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, symbols: make(map[string]*Symbol), nextOffset: -8}
}

// Declare adds sym to the current scope. Returns an error string if already declared.
func (s *Scope) Declare(sym *Symbol) string {
	key := toLower(sym.Name)
	if _, exists := s.symbols[key]; exists {
		return "duplicate declaration: " + sym.Name
	}
	s.symbols[key] = sym
	return ""
}

// DeclareVar declares an 8-byte variable and assigns it the next stack offset.
func (s *Scope) DeclareVar(name string, typ TypeKind) *Symbol {
	return s.DeclareVarSized(name, typ, 8)
}

// DeclareVarSized declares a variable of sizeBytes bytes (rounded up to a
// multiple of 8) and assigns it the next stack offset.
func (s *Scope) DeclareVarSized(name string, typ TypeKind, sizeBytes int) *Symbol {
	aligned := (sizeBytes + 7) &^ 7
	if aligned < 8 {
		aligned = 8
	}
	sym := &Symbol{Name: name, Kind: SymVar, Type: typ, Offset: s.nextOffset}
	s.nextOffset -= aligned
	s.symbols[toLower(name)] = sym
	return sym
}

// DeclareParam declares a parameter (positive offset from rbp for caller args).
// Params are assigned positive offsets starting at 16.
func (s *Scope) DeclareParam(name string, typ TypeKind, offset int) *Symbol {
	sym := &Symbol{Name: name, Kind: SymParam, Type: typ, Offset: offset}
	s.symbols[toLower(name)] = sym
	return sym
}

// LookupLocal finds a symbol by name in the current scope only (no parent search).
// Returns nil if not found.
func (s *Scope) LookupLocal(name string) *Symbol {
	sym, _ := s.symbols[toLower(name)]
	return sym
}

// Lookup finds a symbol by name, searching through parent scopes.
// Returns nil if not found.
func (s *Scope) Lookup(name string) *Symbol {
	key := toLower(name)
	for sc := s; sc != nil; sc = sc.parent {
		if sym, ok := sc.symbols[key]; ok {
			return sym
		}
	}
	return nil
}

// FrameSize returns the total number of bytes needed for local variables.
func (s *Scope) FrameSize() int {
	// nextOffset starts at -8 and decrements by 8 for each var.
	return -s.nextOffset - 8
}

func toLower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}
