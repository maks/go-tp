// pascc is the Pascal cross-compiler command-line tool.
// It compiles a single .pas source file to a native x86-64 binary.
//
// Usage:
//
//	pascc [-format elf|macho] [-o output] input.pas
//
// Flags:
//
//	-format   Output binary format: elf (Linux, default) or macho (macOS).
//	-o        Output path. Defaults to input filename without the .pas extension.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"go-tp/pascal"
	x86 "go-tp/pascal/codegen/x86_64"
)

func main() {
	format := flag.String("format", "elf", "output format: elf (Linux) or macho (macOS)")
	output := flag.String("o", "", "output binary path")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: pascc [-format elf|macho] [-o output] input.pas")
		os.Exit(1)
	}

	inputPath := flag.Arg(0)
	src, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pascc: %v\n", err)
		os.Exit(1)
	}

	outPath := *output
	if outPath == "" {
		outPath = strings.TrimSuffix(inputPath, ".pas")
		if outPath == inputPath {
			outPath = inputPath + ".out"
		}
	}

	gen := x86.New(outPath)
	switch strings.ToLower(*format) {
	case "macho", "mach-o":
		gen.Format = x86.FormatMachO
	case "elf":
		gen.Format = x86.FormatELF
	default:
		fmt.Fprintf(os.Stderr, "pascc: unknown format %q (use elf or macho)\n", *format)
		os.Exit(1)
	}

	compiler := pascal.NewCompiler(string(src), gen)
	diags := compiler.Compile()
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "%s:%d:%d: %s\n", inputPath, d.Line, d.Col, d.Msg)
	}
	if len(diags) > 0 {
		os.Exit(1)
	}
}
