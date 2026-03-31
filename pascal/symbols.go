package pascal

// TypeKind identifies the Pascal type of a symbol.
type TypeKind int

const (
	TypeUnknown TypeKind = iota
	TypeInteger
	TypeString
	TypeBoolean
	TypeChar
	TypeVoid // for procedures
)

// SymbolKind identifies how a symbol was declared.
type SymbolKind int

const (
	SymVar SymbolKind = iota
	SymParam
	SymProc
	SymFunc
	SymConst
)

// Symbol is one entry in a scope.
type Symbol struct {
	Name   string
	Kind   SymbolKind
	Type   TypeKind
	Offset int  // stack offset for Var/Param, code address for Proc/Func
	Value  int64 // for constants
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

// DeclareVar declares a variable and assigns it the next stack offset.
// Returns the assigned offset.
func (s *Scope) DeclareVar(name string, typ TypeKind) *Symbol {
	sym := &Symbol{Name: name, Kind: SymVar, Type: typ, Offset: s.nextOffset}
	s.nextOffset -= 8
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
