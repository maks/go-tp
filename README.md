# go-tp — Turbo Pascal IDE for Linux x86-64

A Turbo Pascal-style IDE written in Go, targeting Linux x86-64. It includes a
terminal UI, a Pascal compiler, and an x86-64 code generator that produces
native ELF binaries — no assembler or linker required.

## Requirements

- Go 1.25 or newer
- Linux x86-64 (the compiler produces native ELF64 executables)
- A terminal that supports ANSI escape codes

## Build

```sh
go build -o pascal-ide ./cmd/pascal-ide
```

## Run

Launch the IDE without a file (blank editor):

```sh
./pascal-ide
```

Launch with an existing Pascal source file:

```sh
./pascal-ide sample/hw.pas
```

The file is loaded into the editor immediately. Any compiled binary is written
alongside the source file (same directory, no extension).

## Key bindings

| Key | Action |
|-----|--------|
| `F9` | Compile the current file |
| `Ctrl+F9` | Compile and run |
| `F10` | Open the menu bar |
| `Alt+F4` | Quit |

## Sample programs

| File | Description |
|------|-------------|
| `sample/hw.pas` | Hello, World |
| `sample/arrec.pas` | Arrays and records smoketest |

## Project layout

```
cmd/pascal-ide/   IDE entry point
ide/              IDE UI (editor, menus, compile/run integration)
pascal/           Pascal lexer, parser, and compiler
pascal/codegen/x86_64/  x86-64 machine-code and ELF64 emitter
tv/               Terminal UI toolkit (windows, widgets, ANSI backend)
sample/           Example Pascal programs
```

## Running the tests

```sh
go test ./...
```

The `pascal/` tests include end-to-end ELF execution tests that compile Pascal
snippets and run the resulting binaries.

## Development

### Architecture overview

The project is split into four packages that form a clean layered stack:

```
cmd/pascal-ide  ← thin main() wrapper
      │
      ▼
    ide/         ← IDE: editor window, output pane, menus, syntax highlight
      │  uses
      ├──────────────────────────────┐
      ▼                              ▼
   pascal/                         tv/
   compiler + symbol table    terminal UI toolkit
      │
      ▼
   pascal/codegen/x86_64/
   machine-code + ELF64 emitter
```

### tv/ — terminal UI toolkit

A self-contained, Borland-style UI framework. Nothing here knows about Pascal.

| Sub-package | Role |
|-------------|------|
| `tv/core` | Primitive types: `Cell`, `DrawBuffer`, `Rect`, `Attr` (8-bit fg/bg colour), `Event`, `KeyCode`, `VT100Parser` |
| `tv/backend` | `DisplayBackend` interface — `Init`, `Size`, `SetCell`, `Flush`, `PollEvent`, `Close` |
| `tv/backend/ansi` | Linux implementation: raw-mode stdin, ANSI escape output, X10 mouse, diff-based rendering (only changed cells are sent) |
| `tv/views` | All widgets: `View` interface, `ViewBase` embed, `Group`, `Window`, `Desktop`, `Editor`, `Button`, `InputLine`, `Label`, `ListBox`, `ScrollBar`, `MenuBar`, `StatusLine`, `Dialog` |
| `tv/app` | `Application` — owns the backend, desktop, menu bar, status line; runs the render + event loop |

**Rendering model.** Every `Draw(buf *DrawBuffer)` call writes into an off-screen cell array. After each event, `Application.render()` walks the whole view tree, then diffs the result against the previous frame and flushes only the changed cells via ANSI sequences.

**Event routing.** `Backend.PollEvent()` produces an `Event`. The application passes it down through `StatusLine → MenuBar → Desktop → focused Window → focused View`. Any handler sets `ev.Handled = true` to stop propagation.

### pascal/ — compiler

A single-pass recursive-descent compiler. There is no AST: the parser calls `CodeGen` methods directly as it recognises each construct.

| File | Role |
|------|------|
| `lexer.go` | `Lexer` — tokenises Pascal source; tracks line/col; handles `{ }`, `(* *)`, `//` comments |
| `symbols.go` | `Scope` (chained, case-insensitive), `Symbol`, `TypeKind`, `ArrayInfo`, `RecordInfo` |
| `compiler.go` | `Compiler` — recursive-descent parser that drives `CodeGen`; resolves forward references via a patch list |
| `codegen.go` | `CodeGen` interface — the only seam between the compiler and any backend |

**Calling convention for user-defined procedures.** Arguments are pushed left-to-right by the caller. The callee's first declared parameter therefore lives at the highest rbp-relative address. At declaration time the compiler assigns `offset = 16 + (nParams-1-i)*8` so that the first parameter maps to `[rbp+16+…]` in source order.

**Stack layout for locals.** `nextOffset` starts at −8 and decrements by 8 (or by the aligned size for arrays/records). `sym.Offset` is the most-positive slot of the allocated block (closest to rbp).

**Array element address.** `&a[i] = rbp + (sym.Offset + low*8) − i*8`. Emitted as a runtime subtraction so the index can be dynamic.

**Record field access.** `field.Offset` is a compile-time constant (0, 8, 16, …). The rbp-relative address of `p.field` is `sym.Offset − field.Offset`, computed at compile time.

**Supported Pascal subset.**

- Scalar types: `integer`, `boolean`, `char`, `string`
- Aggregate types: `array[lo..hi] of T`, `record … end`
- `const`, `type` (named aliases), `var` sections — any number, in any order before the main block
- `procedure` and `function` with parameters and local variables
- `if/then/else`, `while/do`, `for/to/downto/do`, `repeat/until`
- `exit`, `writeln`, `readln`
- Arithmetic: `+ - * / div mod`; relational: `= <> < <= > >=`; logical: `and or not`

### pascal/codegen/x86_64/ — native code emitter

Implements `CodeGen` for Linux x86-64. Encodes machine bytes directly — no assembler.

- User-procedure code accumulates in a `code []byte` slice.
- String literals go into a separate `rodata []byte` slice (null-terminated, deduplicated).
- `Finalize()` patches all RIP-relative string loads and call-site rel32 fields, then serialises an ELF64 binary: ELF header + one `PT_LOAD` segment + `.text` + `.rodata` + builtin helper stubs + `_start`.

Builtin helpers (written as inline byte sequences) cover `writeln`, `write` for int/bool/string, `readln` for int/string, and `halt`.

### ide/ — IDE shell

Glues the compiler and the UI together.

| Component | Description |
|-----------|-------------|
| `IdeEditorWindow` | `views.Window` containing a `LineNumberGutter`, a `views.Editor`, and a `views.ScrollBar`. Tracks the current file path and modified state. |
| `PascalHighlighter` | Implements `views.Highlighter` — per-character colour map: keywords (yellow), strings (green), numbers (cyan), comments (cyan), normal text (white). Stateful across lines for `{ }` and `(* *)` block comments. |
| `OutputWindow` | Scrollable list of compiler diagnostics and run output. Clicking an error line fires `OnGotoErr(lineNo)` which jumps the editor to that line. |
| `commands.go` | Handles `CmBuild` (F9) and `CmRun` (Ctrl+F9): calls `pascal.NewCompiler` → `x86_64.New` → `Compile()`, then either reports diagnostics or `exec`s the binary and captures stdout. |
| `main.go` (`Run()`) | Creates the ANSI backend, lays out editor + output windows on the desktop, wires up menus and the status line, optionally loads the file named in `os.Args[1]`, then starts the event loop. |
