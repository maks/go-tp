//go:build linux

// Package debugger provides ptrace-based debugging for the Pascal IDE.
// All ptrace functions must be called from the same OS thread for a given pid.
// Use Session which manages a locked worker goroutine for this purpose.
package debugger

import (
	"encoding/binary"
	"fmt"
	"os/exec"
	"syscall"
)

// LaunchStopped starts path under ptrace control and returns the child pid.
// The child is stopped before executing its first instruction.
//
// IMPORTANT: The caller must call Wait on the returned pid from the same OS
// thread that will make all subsequent ptrace calls (the session worker goroutine).
func LaunchStopped(path string, args []string) (int, error) {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Ptrace: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("LaunchStopped: start %q: %w", path, err)
	}
	return cmd.Process.Pid, nil
}

// Continue resumes the stopped process without delivering a signal.
func Continue(pid int) error {
	if err := syscall.PtraceCont(pid, 0); err != nil {
		return fmt.Errorf("Continue(%d): %w", pid, err)
	}
	return nil
}

// SingleStep executes exactly one instruction then stops.
func SingleStep(pid int) error {
	if err := syscall.PtraceSingleStep(pid); err != nil {
		return fmt.Errorf("SingleStep(%d): %w", pid, err)
	}
	return nil
}

// Wait blocks until the process stops. Returns the wait status.
// Call after Continue or SingleStep.
func Wait(pid int) (syscall.WaitStatus, error) {
	var ws syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
		return 0, fmt.Errorf("Wait(%d): %w", pid, err)
	}
	return ws, nil
}

// GetRegs reads all general-purpose registers from the stopped child.
func GetRegs(pid int) (syscall.PtraceRegs, error) {
	var regs syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
		return regs, fmt.Errorf("GetRegs(%d): %w", pid, err)
	}
	return regs, nil
}

// SetRegs writes all general-purpose registers to the stopped child.
func SetRegs(pid int, regs syscall.PtraceRegs) error {
	if err := syscall.PtraceSetRegs(pid, &regs); err != nil {
		return fmt.Errorf("SetRegs(%d): %w", pid, err)
	}
	return nil
}

// PeekWord reads one 8-byte word from the child's address space.
// addr must be 8-byte aligned.
func PeekWord(pid int, addr uintptr) (uint64, error) {
	buf := make([]byte, 8)
	n, err := syscall.PtracePeekData(pid, addr, buf)
	if err != nil {
		return 0, fmt.Errorf("PeekWord(%d, %#x): %w", pid, addr, err)
	}
	if n < 8 {
		return 0, fmt.Errorf("PeekWord(%d, %#x): short read %d bytes", pid, addr, n)
	}
	return binary.LittleEndian.Uint64(buf), nil
}

// PokeWord writes one 8-byte word to the child's address space.
// addr must be 8-byte aligned.
func PokeWord(pid int, addr uintptr, val uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, val)
	if _, err := syscall.PtracePokeData(pid, addr, buf); err != nil {
		return fmt.Errorf("PokeWord(%d, %#x): %w", pid, addr, err)
	}
	return nil
}

// PeekByte reads one byte from an arbitrary address in the child's address space.
// It reads the aligned 8-byte word containing the address and extracts the byte.
func PeekByte(pid int, addr uintptr) (byte, error) {
	aligned := addr &^ 7
	offset := addr - aligned
	word, err := PeekWord(pid, aligned)
	if err != nil {
		return 0, fmt.Errorf("PeekByte(%d, %#x): %w", pid, addr, err)
	}
	return byte(word >> (offset * 8)), nil
}

// PokeByte writes one byte to an arbitrary address using read-modify-write.
func PokeByte(pid int, addr uintptr, b byte) error {
	aligned := addr &^ 7
	offset := addr - aligned
	word, err := PeekWord(pid, aligned)
	if err != nil {
		return fmt.Errorf("PokeByte(%d, %#x) read: %w", pid, addr, err)
	}
	mask := ^(uint64(0xFF) << (offset * 8))
	word = (word & mask) | (uint64(b) << (offset * 8))
	if err := PokeWord(pid, aligned, word); err != nil {
		return fmt.Errorf("PokeByte(%d, %#x) write: %w", pid, addr, err)
	}
	return nil
}

// Kill sends SIGKILL to the process.
func Kill(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
