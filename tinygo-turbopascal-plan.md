# TinyGo Turbo Pascal IDE — Implementation Plan

## Development Strategy: Regular Go First, TinyGo Later

The project is built in two stages:

**Stage 1 — Regular Go on Linux**: All packages (`tv/`, `pascal/`, `ide/`) are written
in standard Go. The full stdlib is available: `golang.org/x/term` for raw terminal mode,
goroutines where convenient. This lets us move fast, validate the architecture, and ship
a working IDE without TinyGo constraints getting in the way.

**Stage 2 — TinyGo port for RP2040**: Once the Linux version is complete and stable, port
to TinyGo targeting the RP2040. The clean `DisplayBackend` interface and the absence of
stdlib-heavy dependencies in core logic means the port is mostly a backend swap, not a
rewrite. The compiler backend is replaced from x86-64 (with `os/exec`) to an in-process
ARM Thumb-2 instruction encoder.

The architectural rules that make the port tractable are established from day one:
- No `reflect` anywhere in `tv/` or `pascal/` — use explicit type tags
- No goroutines in the event loop — single-threaded polling throughout
- No `tcell` or other heavy terminal library — ANSI backend written from scratch
  (so the port to TinyGo only touches the one backend file, not the framework)
- 16-color palette only — maps cleanly to both ANSI and RGB565

---

## Dual-Target Architecture

```
┌──────────────────────────────────────────────────────┐
│                  ide/ (application)                  │
├──────────────────────────────────────────────────────┤
│     tv/ (Turbo Vision framework — target-agnostic)   │
├──────────────────────────────────────────────────────┤
│        pascal/ (compiler — target-agnostic)          │
├──────────────────────────────────────────────────────┤
│  tv/backend/ansi/      │  tv/backend/rp2040/         │
│  Linux raw terminal    │  ST7789 LCD + USB UART      │
│  (ANSI escape codes,   │  (pixel font rendering,     │
│  golang.org/x/term)    │  tinygo.org/x/drivers)      │
├────────────────────────┴─────────────────────────────┤
│  pascal/codegen/x86_64/  │  pascal/codegen/thumb2/   │
│  encode x86-64 bytes +   │  encode Thumb-2 bytes →   │
│  write ELF64 binary      │  load into SRAM buffer →  │
│  → os/exec to run it     │  call via unsafe.Pointer  │
│  (Stage 1, std Go)       │  (Stage 2, TinyGo/RP2040) │
└──────────────────────────┴───────────────────────────┘
```

The `tv/` framework and `pascal/` frontend (lexer, parser, symbol table) contain **zero**
target-specific code. Backend and codegen are selected at build time via build tags
(`//go:build !rp2040` / `//go:build rp2040`).

### RP2040 Hardware (Stage 2 target)

- **Display**: ST7789 SPI LCD (320×240 px). Characters rendered as pixel glyphs using a
  built-in 8×8 bitmap font → 40×30 character cells.
- **Input**: USB CDC (serial UART over USB) — keystrokes sent as VT100 escape sequences,
  parsed by the same shared parser used by the ANSI backend.

---

## Project Structure

```
go-tp/
├── go.mod
├── tv/                    # Phase 1: Turbo Vision framework
│   ├── core/
│   │   ├── rect.go        # Rect, Point
│   │   ├── cell.go        # Cell, DrawBuffer, Attribute (color encoding)
│   │   ├── event.go       # Event, EventType, KeyCode constants + VT100 parser
│   │   ├── command.go     # CommandId constants (CM_QUIT, CM_OK, …)
│   │   └── palette.go     # 16-color Borland palette
│   ├── views/
│   │   ├── view.go        # View interface + ViewBase (embed in all views)
│   │   ├── group.go       # Group: child container, focus cycling
│   │   ├── frame.go       # Frame: single/double border, title, resize handles
│   │   ├── window.go      # Window: draggable + interior Group + Frame
│   │   ├── dialog.go      # Dialog: modal Window, gray palette
│   │   ├── desktop.go     # Desktop: window manager, z-order, background
│   │   ├── label.go       # Label / StaticText
│   │   ├── button.go      # Button: click → command dispatch
│   │   ├── input_line.go  # InputLine: single-line text input
│   │   ├── scrollbar.go   # ScrollBar (h/v), Indicator
│   │   ├── editor.go      # Editor: multi-line, selection, undo/redo, highlighter
│   │   ├── menu.go        # MenuBar, MenuItem, MenuBox (popup)
│   │   ├── status_line.go # StatusLine + StatusItem
│   │   └── list_box.go    # ListBox (scrollable item list)
│   ├── backend/
│   │   ├── backend.go     # DisplayBackend interface (target-agnostic)
│   │   ├── ansi/
│   │   │   └── ansi.go    # Linux: raw terminal via golang.org/x/term + ANSI output
│   │   └── rp2040/
│   │       └── rp2040.go  # RP2040: ST7789 SPI + USB CDC, 8×8 bitmap font renderer
│   └── app/
│       └── application.go # Application: owns backend+desktop+menu+status, event loop
├── pascal/                # Phase 2: Pascal compiler
│   ├── lexer.go           # Tokenizer
│   ├── compiler.go        # Single-pass recursive descent parser + codegen dispatch
│   ├── symbols.go         # Symbol table / scope
│   ├── codegen/
│   │   ├── codegen.go     # CodeGen interface implemented by both backends
│   │   ├── x86_64/
│   │   │   └── x86_64.go  # Emit GNU x86-64 .s, shell out to as+cc  (Stage 1)
│   │   └── thumb2/
│   │       └── thumb2.go  # ARM Thumb-2 in-process encoder → exec from RAM (Stage 2)
│   └── stdlib.go          # Built-in procedures: write, writeln, readln, halt, …
└── ide/                   # Phase 3: Pascal IDE
    ├── main.go
    ├── commands.go        # CmBuild, CmRun, CmGotoErr, …
    ├── editor_window.go   # IdeEditorWindow: gutter + Editor + scrollbars
    ├── gutter.go          # LineNumberGutter view (3-char wide)
    ├── output_window.go   # OutputWindow: scrollable build/run log
    └── highlighter.go     # PascalHighlighter: keywords, strings, comments
```

---

## Phase 1 — Turbo Vision (`tv/`)

### 1.1 Core Types (`tv/core/`)

| File | What to build |
|------|--------------|
| `rect.go` | `Rect{X,Y,W,H}`, `Point{X,Y}`, `Contains`, `Intersect`, `Grow`, `Offset` |
| `cell.go` | `Attr uint8` (fg 4b + bg 4b; 16 colors), `Cell{Ch rune, Attr}`, `DrawBuffer []Cell` with `MoveStr`, `MoveChar`, `PutAttr` |
| `event.go` | `EventType` enum; `Event{Type, Key KeyCode, Ch rune, MouseX/Y int, Cmd CommandId}`; Borland key constants; **shared VT100 escape sequence parser** used by both ANSI and RP2040 backends |
| `command.go` | `CommandId` uint16 constants: CM_QUIT, CM_CLOSE, CM_OK, CM_CANCEL, CM_ZOOM, CM_NEW, CM_OPEN, CM_SAVE, CM_SAVE_AS, CM_CUT, CM_COPY, CM_PASTE, CM_UNDO, CM_REDO, CM_FIND, CM_REPLACE |
| `palette.go` | 16-color Borland palette mapped to ANSI colors (Linux) or RGB565 (ST7789); `AttrFg(a) uint8`, `AttrBg(a) uint8` helpers |

### 1.2 Display Backend (`tv/backend/backend.go`)

The only target-specific seam in the entire codebase:

```go
type DisplayBackend interface {
    Init() error
    Size() (cols, rows int)
    SetCell(x, y int, ch rune, attr core.Attr)
    Flush()
    PollEvent() *core.Event   // non-blocking; returns nil if no event ready
    Close()
}
```

**`backend/ansi/`** (Linux Stage 1 — standard Go, build tag `!rp2040`):
- `Init()`: raw mode via `golang.org/x/term` (`term.MakeRaw`); enable mouse reporting
  (`\x1b[?1000h`)
- `Size()`: `term.GetSize(fd)`
- `SetCell()`: accumulates dirty cells; `Flush()` emits minimal ANSI sequences
  (`\x1b[row;colH\x1b[FG;BGm` + char) — only changed cells
- `PollEvent()`: non-blocking `os.Stdin.Read` with a zero-timeout `syscall.Select`;
  hands raw bytes to the shared VT100 parser in `tv/core/event.go`
- Mouse: parse `\x1b[M` byte reports from the enabled mouse mode

**`backend/rp2040/`** (RP2040 Stage 2 — TinyGo, build tag `rp2040`):
- `Init()`: configure SPI bus, init ST7789 via `tinygo.org/x/drivers/st7789`,
  enable `machine.USBCDC`
- `Size()`: fixed 40×30 (320÷8 × 240÷8)
- `SetCell()`: dirty-cell accumulator; `Flush()` renders each dirty cell as 8×8 px
  glyph from a compiled-in bitmap font, fg/bg → RGB565 color lookup
- `PollEvent()`: poll `machine.USBCDC` for available bytes; feed to shared VT100 parser
- No mouse on initial RP2040 build

### 1.3 View System (`tv/views/view.go`, `group.go`)

**`View` interface** — minimal, modeled directly on the Rust `view.rs`:

```go
type View interface {
    Bounds() core.Rect
    SetBounds(core.Rect)
    Draw(buf *core.DrawBuffer)
    HandleEvent(ev *core.Event)
    CanFocus() bool
    SetFocused(bool)
    IsFocused() bool
    SetOwner(View)
    Owner() View
}
```

**`ViewBase` struct** — embed in all concrete views; provides default implementations for
bounds, owner, and focus flag. Custom views override only what they need.

**`Group`** — stores `[]View` children; dispatches events to focused child first, then
broadcasts; Tab/Shift+Tab cycles focus; `Add(v View, rel Rect)` converts child's relative
`Rect` to absolute on insertion.

### 1.4 Widgets (build in order of dependency)

| Widget | Key behavior | Reference |
|--------|-------------|-----------|
| `Frame` | Draw single/double box-drawing chars; title in top border; resize corners | `views/frame.rs` |
| `Window` | Interior `Group` + `Frame`; drag on title bar (MouseDown+Move); 1-cell shadow | `views/window.rs` |
| `Dialog` | Window + `SF_MODAL`; `Desktop.ExecView(d)` runs nested event loop until close | `views/dialog.rs` |
| `Desktop` | `[]Window` z-stack; routes events to top window first; `░` background | `views/desktop.rs` |
| `Label` | Writes string at fixed position; no focus | |
| `Button` | Focus highlight; Enter/Space/Click → emit `Command(cmd)` event | `views/button.rs` |
| `InputLine` | Cursor, left/right, Home/End, insert/backspace, configurable max length | `views/input_line.rs` |
| `ScrollBar` | H or V; `Value`, `Max`, `PageSize`; proportional thumb; mouse drag; `OnChange func(int)` | `views/scroll_bar.rs` |
| `ListBox` | `[]string` items; arrow key + click selection; delegates scroll to `ScrollBar` | `views/list_box.rs` |
| `MenuBar` | Top row of submenu labels; Alt+letter or click opens `MenuBox` popup | `views/menu_bar.rs` |
| `MenuBox` | Floating item list; keyboard + click navigation; shortcut letter highlighted | `views/menu_box.rs` |
| `StatusLine` | Bottom row of `StatusItem{label, key, cmd}`; pressing the key fires the command | `views/status_line.rs` |
| `Editor` | `[][]rune` lines; cursor, selection, undo stack (`[]EditOp`); pluggable `Highlighter` interface returning `[]Attr` per line | `views/editor.rs` |

### 1.5 Application (`tv/app/application.go`)

```go
type Application struct {
    backend    backend.DisplayBackend
    desktop    *views.Desktop
    menuBar    *views.MenuBar    // nil if unused
    statusLine *views.StatusLine
}

func (a *Application) Run()
func (a *Application) HandleCommand(cmd core.CommandId)
```

Event loop (single-threaded polling throughout — safe for both Linux and RP2040):
1. `backend.PollEvent()` → `*core.Event` (nil if nothing pending)
2. If event: dispatch keyboard/mouse → `desktop.HandleEvent`; command → `HandleCommand`
3. `desktop.Draw(drawbuf)` → diff against previous frame → `backend.SetCell` for changes only
4. `backend.Flush()`

---

## Phase 2 — Pascal Compiler (`pascal/`)

### Design: Single-Pass Recursive Descent + Pluggable Native Code Backend

The compiler is a **single-pass recursive descent** parser that drives a `CodeGen`
interface directly during parsing — no separate AST phase. This matches the architecture
of the reference `turbopascal` JavaScript implementation (`Compiler.js`) and is essential
for the eventual TinyGo port: it avoids allocating a full AST tree and keeps heap
pressure low.

Forward jumps (if/while) are backpatched: the compiler emits a placeholder jump, records
its address, finishes the branch body, then patches the placeholder with the real target —
exactly as done in `Compiler.js` with the `exitInstructions` stack.

The `CodeGen` interface allows the same parser to drive two completely different backends:

```go
type CodeGen interface {
    // Called by the parser as it descends
    EmitLoadInt(n int32)
    EmitLoadStr(s string)
    EmitLoadVar(offset int)
    EmitStoreVar(offset int)
    EmitBinaryOp(op token.Kind)   // +, -, *, div, mod, =, <>, <, >, <=, >=, and, or
    EmitUnaryOp(op token.Kind)    // -, not
    EmitJumpFalse() int           // emits placeholder; returns address for backpatch
    EmitJump() int                // unconditional; returns address for backpatch
    EmitJumpTo(addr int)          // unconditional jump to known address (for loops)
    PatchJump(placeholder int)    // fills in placeholder with current emit position
    EmitCallBuiltin(idx int)      // writeln, readln, halt, …
    EmitCallProc(addr int)
    EmitProcEntry(frameSize int)
    EmitProcReturn()
    Finalize() ([]Diagnostic, error)  // assemble/link or flush encoded bytes
}
```

### 2.1 Lexer (`pascal/lexer.go`)

Integrated into the compiler struct as a peek/next tokenizer, porting `Lexer.js`.

Token types:
- Keywords: `program var begin end if then else while do for to downto repeat until procedure function type const uses`
- Literals: `IntLit(int32)`, `StrLit(string)`, `BoolLit(bool)`
- Operators: `:=  = <> < > <= >=  + - * / div mod and or not`
- Delimiters: `( ) [ ] , ; : . ..`
- Comments: `{ }`, `(* *)`, `//` — skipped by the lexer

### 2.2 Symbol Table (`pascal/symbols.go`)

- `Scope` with `parent *Scope`, `symbols map[string]*Symbol`
- `Symbol`: `Name string`, `Kind (Var/Param/Proc/Func/Const)`, `Type TypeKind`, `Offset int`
- `Allocate()` assigns stack offsets to all locals; returns total frame size

### 2.3 Compiler (`pascal/compiler.go`)

```go
type Compiler struct {
    src     string
    pos     int        // lexer cursor
    tok     Token      // one-token lookahead
    scope   *Scope
    gen     CodeGen    // injected at construction
}

type Diagnostic struct{ Line, Col int; Msg string }

func NewCompiler(source string, gen CodeGen) *Compiler
func (c *Compiler) Compile() []Diagnostic
```

**Grammar** (bruto-pascal subset plus procedures/functions from turbopascal):

```
program      = "program" IDENT ";" [uses_clause] [var_section] {proc_or_func} block "."
var_section  = "var" (ident_list ":" type ";")+
proc_or_func = ("procedure"|"function") IDENT ["(" params ")"] [":" type] ";"
               [var_section] block ";"
block        = "begin" statement {";" statement} "end"
statement    = assignment | if_stmt | while_stmt | for_stmt | repeat_stmt
             | write_stmt | writeln_stmt | readln_stmt | proc_call | block | ""
assignment   = IDENT ":=" expr
if_stmt      = "if" expr "then" statement ["else" statement]
while_stmt   = "while" expr "do" statement
for_stmt     = "for" IDENT ":=" expr ("to"|"downto") expr "do" statement
repeat_stmt  = "repeat" statement {";" statement} "until" expr
expr         = simple_expr [("="|"<>"|"<"|">"|"<="|">=") simple_expr]
simple_expr  = term {("+"|"-"|"or") term}
term         = factor {("*"|"/"|"div"|"mod"|"and") factor}
factor       = INT_LIT | STR_LIT | "true" | "false" | IDENT ["(" args ")"]
             | "not" factor | "(" expr ")" | "-" factor
type         = "integer" | "string" | "boolean"
```

**Single-pass emit examples** (calls on `gen`):

| Construct | CodeGen calls |
|-----------|--------------|
| `IntLit n` | `EmitLoadInt(n)` |
| `StrLit s` | `EmitLoadStr(s)` |
| `Var x` | `EmitLoadVar(x.Offset)` |
| `x := e` | compile e; `EmitStoreVar(x.Offset)` |
| `e1 + e2` | compile e1; compile e2; `EmitBinaryOp(ADD)` |
| `if c then t else e` | compile c; `p1=EmitJumpFalse()`; compile t; `p2=EmitJump()`; `PatchJump(p1)`; compile e; `PatchJump(p2)` |
| `while c do b` | `top=currentAddr`; compile c; `p=EmitJumpFalse()`; compile b; `EmitJumpTo(top)`; `PatchJump(p)` |
| procedure entry | `EmitProcEntry(frameSize)` |
| `proc_call(args)` | compile each arg; `EmitCallProc(proc.Addr)` |

### 2.4 x86-64 Code Generator (`pascal/codegen/x86_64/`) — Stage 1

Implements `CodeGen` in the style of TCC: encodes x86-64 machine code bytes directly
in Go and writes a proper **ELF64 executable** entirely in Go — no `as`, no `ld`, no
external tools of any kind for the compilation step. `os/exec` is used only afterwards
to *run* the finished binary, not to produce it.

**What the codegen writes itself (no external tools)**:

```
ELF64 binary layout written by Go code:
  ELF header          (64 bytes)
  Program headers     (LOAD segments: .text rx, .data rw)
  .text section       (encoded x86-64 instruction bytes)
  .rodata section     (string literal bytes)
  .data section       (global/static variable storage)
  .bss size           (zero-initialised variables — just recorded in header)
  Section headers     (for debuggers; optional but good practice)
```

Relocations for string addresses and inter-procedure calls are resolved in-process
during `Finalize()` before writing — the codegen tracks all forward references in a
`[]reloc` list and patches them once all code offsets are known, exactly as TCC does.

**Executable entry point**: a small hand-encoded `_start` stub is prepended to `.text`
that calls `main`, then issues `syscall exit(rax)` directly — no libc, no crt0.

**I/O**: `writeln`, `readln`, etc. are implemented as hand-encoded helper routines
appended to `.text` at the start of `Finalize()`. They use Linux syscalls directly
(`syscall write(1, …)`, `syscall read(0, …)`) — no libc dependency, so the ELF
needs no dynamic linker (`PT_INTERP` is omitted; the binary is statically self-contained).

**Frame layout** (System V AMD64 ABI):
```
[rbp-8*n] … [rbp-8]  : local vars (64-bit slots)
[rbp]                 : saved rbp
[rbp+8]               : return address
[rbp+16] …            : parameters (if any)
```

**Register convention**: expression results in `rax`; binary ops spill lhs to the
stack (`push rax`) then pop into `rcx` after rhs is evaluated (`pop rcx; op rcx, rax`).

**Instruction encoding** (representative subset):
```
push rbp             → 55
mov  rbp, rsp        → 48 89 E5
sub  rsp, imm8       → 48 83 EC <imm>
mov  rax, imm64      → 48 B8 <8 bytes LE>
mov  [rbp-off], rax  → 48 89 45 <-off>
mov  rax, [rbp-off]  → 48 8B 45 <-off>
push rax             → 50
pop  rcx             → 59
add  rax, rcx        → 48 01 C8
imul rax, rcx        → 48 0F AF C1
cqo; idiv rcx        → 48 99; 48 F7 F9
cmp  rax, rcx        → 48 39 C8
sete al; movzx rax,al→ 0F 94 C0; 48 0F B6 C0
jmp  rel32           → E9 <4 bytes LE>   ← placeholder patched by PatchJump
je   rel32           → 0F 84 <4 bytes LE>
call rel32           → E8 <4 bytes LE>
ret                  → C3
```

### 2.5 ARM Thumb-2 Code Generator (`pascal/codegen/thumb2/`) — Stage 2

Implements `CodeGen` for TinyGo/RP2040. Encodes 16-bit and 32-bit Thumb-2 instructions
directly into a `[]byte` buffer. On `Finalize()` the buffer is loaded into a pre-allocated
SRAM region and called via `unsafe.Pointer`.

On RP2040 there is no OS and therefore no fork/exec — running compiled programs from SRAM
is the only viable option on bare metal. This is not JIT: the program is fully compiled
first, then called once. The SRAM execution region is a fixed-size array allocated at
startup (e.g. 48 KB), separate from the IDE's own stack and heap.

The register convention, stack frame layout, and I/O calls differ from x86-64 (ARM AAPCS,
UART write for output) but the `CodeGen` interface is identical so the parser is unchanged.

### 2.6 Standard Library (`pascal/stdlib.go`)

Built-in procedure table indexed by `EmitCallBuiltin(idx)`:

| Index | Procedure | Stage 1 action | Stage 2 action |
|-------|-----------|---------------|----------------|
| 0 | `write(int)` | hand-coded x86-64 helper using `syscall write` | Thumb-2 helper, UART |
| 1 | `write(str)` | hand-coded x86-64 helper using `syscall write` | Thumb-2 helper, UART |
| 2 | `write(bool)` | hand-coded x86-64 helper using `syscall write` | Thumb-2 helper, UART |
| 3 | `writeln` | above + newline byte | above + CRLF |
| 4 | `readln(int var)` | hand-coded x86-64 helper using `syscall read` | Thumb-2 helper, UART |
| 5 | `readln(str var)` | hand-coded x86-64 helper using `syscall read` | Thumb-2 helper, UART |
| 6 | `halt` | `syscall exit(0)` encoded inline | branch to return address |

---

## Phase 3 — Pascal IDE (`ide/`)

Modeled directly on `bruto-pascal`, adapted to the Go TV framework.

### 3.1 Window Layout

```
┌─ Menu ─────────────────────────────────────────┐
├─ Editor Window (main, ~70% height) ─────────────┤
│ ### │ Pascal source with syntax highlight        │
│ ### │                                            │
├─ Output Window (~30% height) ───────────────────┤
│ Compiler diagnostics / program stdout            │
└─ Status Line ───────────────────────────────────┘
```

On RP2040 (40×30 cells): Editor ~20 rows, Output ~7 rows, MenuBar 1 row, StatusLine 1 row.

### 3.2 Commands (`ide/commands.go`)

```go
const (
    CmBuild   = core.CommandId(1000) // F9  — compile source to native binary
    CmRun     = core.CommandId(1001) // ^F9 — compile + run, stream output
    CmGotoErr = core.CommandId(1002) // click/enter on error line → move editor cursor
)
```

### 3.3 IDE Editor Window (`ide/editor_window.go`)

- Extend `tv.Window` with `LineNumberGutter` (3 chars wide) + `Editor` side by side
  as interior children; `ScrollBar` + `Indicator` as frame children
- Gutter scroll synced from editor vertical scrollbar on every `Draw`
- Tracks `filePath string`, `modified bool` (asterisk in window title)

### 3.4 Output Window (`ide/output_window.go`)

- `tv.Window` containing a scrollable `[]string` view with per-line `Attr`
- `AppendLine(s string, attr core.Attr)` — auto-scrolls to bottom
- Error lines formatted as `file.pas:12: message`; click/Enter → emit `CmGotoErr`

### 3.5 Pascal Highlighter (`ide/highlighter.go`)

Implements `tv.Highlighter`:

```go
type Highlighter interface {
    Highlight(lines []string) [][]core.Attr  // one Attr per rune
}
```

Multiline state machine for `{ }` / `(* *)` block comments; coloring:
- Keywords → bright white on blue
- Strings → bright yellow on blue
- Comments → dark cyan on blue
- Numbers → bright cyan on blue
- Normal → white on blue

### 3.6 Menu & Status

**MenuBar**:
- `File` → New, Open…, Save, Save As…, ─, Quit
- `Edit` → Undo, Redo, ─, Cut, Copy, Paste
- `Compile` → Build (F9), Run (Ctrl+F9)

**StatusLine**: `F1 Help  F9 Build  ^F9 Run  F3 Open  F2 Save`

### 3.7 Build & Run Flow

**Stage 1 (Linux, regular Go)**:
```
F9 pressed (CmBuild)
  → get source text from Editor
  → compiler.Compile(source, x86_64.New(outputPath))
  → codegen encodes x86-64 bytes + writes ELF64 binary to outputPath — no as, no ld
  → on diagnostics: show in OutputWindow (red), highlight error lines in Editor
  → on success: show "Build OK — <outputPath>"

^F9 pressed (CmRun)
  → CmBuild (if source dirty)
  → if success: os/exec.Command(outputPath) — run the ELF binary
    stream stdout+stderr to OutputWindow line by line
  → show "Program exited." in OutputWindow
```

**Stage 2 (RP2040, TinyGo)**:
```
F9 pressed (CmBuild)
  → compiler.Compile(source, thumb2.New(&codeBuffer))
  → on diagnostics: show in OutputWindow
  → on success: show "Build OK"; codeBuffer holds executable Thumb-2 bytes

^F9 pressed (CmRun)
  → CmBuild (if source dirty)
  → if success: call codeBuffer as function pointer; program I/O routed to OutputWindow
    via the stdlib IO interface; returns when program calls halt or reaches end
```

---

## Dependency Order & Milestones

### Stage 1 — Regular Go on Linux

| Milestone | Deliverable | Can demo |
|-----------|------------|---------|
| **M1** | `tv/core` + ANSI backend + `ViewBase`+`Group`+`Frame`+`Window`+`Desktop`+`Application` | Draggable windows on desktop |
| **M2** | `Label`+`Button`+`InputLine`+`Dialog` | Modal input dialog |
| **M3** | `MenuBar`+`MenuBox`+`StatusLine` | Full menu navigation |
| **M4** | `ScrollBar`+`Editor`+`ListBox` | Text editor window |
| **M5** ✓ | Pascal lexer + compiler + x86-64 codegen | Compile & run hello world from CLI |
| **M6** ✓ | `ide/` integrating all above | Full IDE on Linux with build+run |

### Stage 2 — TinyGo port for RP2040

| Milestone | Deliverable | Runs on RP2040? |
|-----------|------------|-----------------|
| **M7** | Verify all of `tv/` and `pascal/` compile with `tinygo build -target=linux` | Linux only |
| **M8** | `backend/rp2040/` ST7789 + USB CDC backend | Display + keyboard working |
| **M9** | ARM Thumb-2 codegen (`pascal/codegen/thumb2/`) | Pascal programs run on device |
| **M10** | Full IDE on RP2040 hardware | Yes |

---

## Key Technical Decisions

1. **Regular Go first**: Stage 1 uses full standard Go — `os/exec`, `golang.org/x/term`,
   goroutines where helpful. Validate the design before introducing TinyGo constraints.

2. **No tcell even in Stage 1**: The ANSI backend is written from scratch using
   `golang.org/x/term` for raw mode setup and direct ANSI escape writes. This means
   the Stage 2 port touches only one file (`backend/ansi/ansi.go` → `backend/rp2040/rp2040.go`)
   and nothing else.

3. **No reflect anywhere in `tv/` or `pascal/`**: Use explicit type tags/enums instead.
   This is the primary constraint that must be enforced from M1 to keep the TinyGo port
   mechanical rather than a rewrite.

4. **Single-pass compiler, no AST**: The Pascal compiler calls `CodeGen` methods directly
   during parsing with backpatching for forward jumps, exactly like the reference `Compiler.js`.
   No AST tree is allocated; the same parser drives both the x86-64 and Thumb-2 backends.

5. **TCC-style native code generation**: The compiler encodes machine instruction bytes
   directly in Go and handles all output formatting itself — no `as`, no `ld`, no external
   tools for the compilation step. On Linux: x86-64 bytes + ELF64 binary written entirely
   in Go; `os/exec` is used only to *run* the resulting binary. On RP2040: Thumb-2 bytes
   loaded into a SRAM region and called directly (no OS available to exec from).

6. **Coordinate system**: All `View.Bounds()` are absolute screen coords; `Group.Add()`
   takes relative bounds and converts on insertion (same as Rust TV).

7. **Event consumption**: `Event.Handled bool` field — views set it to stop further routing.

8. **Modal dialogs**: `Desktop.ExecView(d *Dialog)` runs a nested polling loop until the
   dialog closes — no goroutines required.

9. **16 colors only**: Borland palette mapped to 16 ANSI codes on Linux and 16 RGB565
   values for the ST7789. No 256-color or true-color complexity.

10. **Pascal scope**: Grammar matches bruto-pascal (integer/string/boolean, if/while/for,
    writeln/readln) plus procedures/functions from turbopascal. Arrays and records are
    stretch goals after M6.
