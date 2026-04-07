package x86_64

// OutputFormat selects the binary output format produced by Finalize.
type OutputFormat int

const (
	FormatELF   OutputFormat = iota // Linux ELF64 (default)
	FormatMachO                     // macOS Mach-O (x86-64)
)

// sysCalls holds OS-specific system call numbers for the target platform.
// On Linux x86-64 the numbers are used directly.
// On macOS x86-64 BSD syscalls carry the 0x2000000 class prefix.
type sysCalls struct {
	read  uint32
	write uint32
	exit  uint32
}

func (f OutputFormat) sc() sysCalls {
	if f == FormatMachO {
		return sysCalls{read: 0x2000003, write: 0x2000004, exit: 0x2000001}
	}
	return sysCalls{read: 0, write: 1, exit: 60}
}
