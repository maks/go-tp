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
