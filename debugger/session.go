//go:build linux

package debugger

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"go-tp/pascal"
)

// State represents the current state of a debug session.
type State int

const (
	StateStopped State = iota // stopped, ready for commands
	StateRunning              // running freely
	StateStepping             // single-stepping
	StateExited               // process has exited
)

// StopReason describes why the process stopped.
type StopReason int

const (
	StopBreakpoint StopReason = iota
	StopStep
	StopExited
	StopSignal
)

// VarSnapshot is a variable's name and formatted value captured at a stop.
type VarSnapshot struct {
	Name  string
	Value string
}

// StopEvent is sent on Session.Events when execution stops.
type StopEvent struct {
	Reason   StopReason
	Line     int           // 1-based source line (0 if unknown)
	BPIndex  int           // index into Breakpoints slice, −1 if not a BP stop
	ExitCode int
	Vars     []VarSnapshot // variable values at stop time (read by worker goroutine)
}

// Breakpoint is one source-level breakpoint.
type Breakpoint struct {
	ID         int
	SourceLine int
	CodeAddr   uintptr // virtual address of the breakpoint instruction
	OrigByte   byte    // the byte replaced with 0xCC (INT 3)
	Enabled    bool
}

// ELF layout constants — must match pascal/codegen/x86_64/x86_64.go.
const (
	elfTextOffset = 176 // ELF header(64) + 2 program headers(2×56)
	startStubSize = 17  // _start stub: call(5)+xor(3)+mov(7)+syscall(2)
)

// reqKind identifies a worker request.
type reqKind int

const (
	reqRun reqKind = iota
	reqStepInto
	reqStepOver
	reqStop
	reqSetBP
	reqRemoveBP
)

type sessionReq struct {
	kind reqKind
	line int         // for reqSetBP
	bp   *Breakpoint // for reqRemoveBP
	resp chan<- sessionResp
}

type sessionResp struct {
	bp  *Breakpoint
	err error
}

// Session owns a single debug session for one ELF binary.
// All ptrace operations are performed by an internal worker goroutine locked
// to a single OS thread, satisfying Linux's ptrace thread requirement.
type Session struct {
	pid         int
	state       State
	info        *pascal.DebugInfo
	loadBase    uintptr
	Breakpoints []*Breakpoint
	Events      chan StopEvent // IDE reads from this channel

	binaryPath string
	reqCh      chan sessionReq
	readyCh    chan error
}

// NewSession launches the binary under ptrace and waits for the initial stop.
// info is the DebugInfo from the most recent successful compile.
func NewSession(binaryPath string, info *pascal.DebugInfo) (*Session, error) {
	s := &Session{
		info:       info,
		Events:     make(chan StopEvent, 16),
		reqCh:      make(chan sessionReq),
		readyCh:    make(chan error, 1),
		binaryPath: binaryPath,
	}
	go s.worker()
	if err := <-s.readyCh; err != nil {
		return nil, err
	}
	return s, nil
}

// Pid returns the child process ID.
func (s *Session) Pid() int { return s.pid }

// SetBreakpoint installs a breakpoint at sourceLine.
// Blocks until the worker processes the request. Only call when stopped.
func (s *Session) SetBreakpoint(sourceLine int) (*Breakpoint, error) {
	resp := make(chan sessionResp, 1)
	s.reqCh <- sessionReq{kind: reqSetBP, line: sourceLine, resp: resp}
	r := <-resp
	return r.bp, r.err
}

// RemoveBreakpoint removes the breakpoint and restores the original byte.
// Blocks until the worker processes the request. Only call when stopped.
func (s *Session) RemoveBreakpoint(bp *Breakpoint) error {
	resp := make(chan sessionResp, 1)
	s.reqCh <- sessionReq{kind: reqRemoveBP, bp: bp, resp: resp}
	r := <-resp
	return r.err
}

// Run resumes free execution. Returns immediately; StopEvent sent on Events.
func (s *Session) Run() {
	s.state = StateRunning
	s.reqCh <- sessionReq{kind: reqRun}
}

// StepOver advances one source line without descending into calls.
// Returns immediately; StopEvent sent on Events.
func (s *Session) StepOver() {
	s.state = StateStepping
	s.reqCh <- sessionReq{kind: reqStepOver}
}

// StepInto advances one source line, descending into calls.
// Returns immediately; StopEvent sent on Events.
func (s *Session) StepInto() {
	s.state = StateStepping
	s.reqCh <- sessionReq{kind: reqStepInto}
}

// Stop kills the process. Blocks until the worker exits.
func (s *Session) Stop() {
	resp := make(chan sessionResp, 1)
	s.reqCh <- sessionReq{kind: reqStop, resp: resp}
	<-resp
}

// --- address helpers ---

func (s *Session) codeBase() uintptr {
	return s.loadBase + uintptr(elfTextOffset+startStubSize)
}

func (s *Session) codeAddrToVA(codeOff int) uintptr {
	return s.codeBase() + uintptr(codeOff)
}

func (s *Session) ripToLine(rip uint64) int {
	if s.info == nil || len(s.info.Lines) == 0 || s.loadBase == 0 {
		return 0
	}
	cb := s.codeBase()
	if uintptr(rip) < cb {
		return 0
	}
	codeOff := int(uintptr(rip) - cb)
	line := 0
	for _, dl := range s.info.Lines {
		if dl.CodeAddr <= codeOff {
			line = dl.Line
		} else {
			break
		}
	}
	return line
}

func (s *Session) currentLine() int {
	regs, err := GetRegs(s.pid)
	if err != nil {
		return 0
	}
	return s.ripToLine(regs.Rip)
}

func (s *Session) findBPAt(va uintptr) (*Breakpoint, int) {
	for i, bp := range s.Breakpoints {
		if bp.Enabled && bp.CodeAddr == va {
			return bp, i
		}
	}
	return nil, -1
}

func (s *Session) readAllVars() []VarSnapshot {
	if s.info == nil || len(s.info.Vars) == 0 {
		return nil
	}
	regs, err := GetRegs(s.pid)
	if err != nil {
		return nil
	}
	rbp := uintptr(regs.Rbp)
	snaps := make([]VarSnapshot, 0, len(s.info.Vars))
	for _, v := range s.info.Vars {
		val, err := ReadVar(s.pid, rbp, v)
		if err != nil {
			val = "(error)"
		}
		snaps = append(snaps, VarSnapshot{Name: v.Name, Value: val})
	}
	return snaps
}

// --- worker goroutine ---

func (s *Session) worker() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pid, err := LaunchStopped(s.binaryPath, nil)
	if err != nil {
		s.readyCh <- err
		return
	}
	s.pid = pid

	// Wait for the initial exec-SIGTRAP (process stopped before first instruction).
	if _, err := Wait(pid); err != nil {
		s.readyCh <- fmt.Errorf("initial wait: %w", err)
		Kill(pid)
		return
	}

	s.loadBase = readLoadBase(pid, s.binaryPath)
	s.readyCh <- nil

	// pending holds a breakpoint whose original byte has been restored and which
	// needs to be reinstalled (0xCC) before the next free Continue.
	var pending *Breakpoint

	for req := range s.reqCh {
		switch req.kind {
		case reqStop:
			Kill(s.pid)
			Wait(s.pid)
			close(s.Events)
			if req.resp != nil {
				req.resp <- sessionResp{}
			}
			return

		case reqSetBP:
			bp, bpErr := s.doSetBP(req.line)
			if req.resp != nil {
				req.resp <- sessionResp{bp: bp, err: bpErr}
			}

		case reqRemoveBP:
			bpErr := s.doRemoveBP(req.bp)
			if req.resp != nil {
				req.resp <- sessionResp{err: bpErr}
			}

		case reqRun:
			pending = s.execRun(pending)

		case reqStepInto:
			pending = s.execStepInto(pending)

		case reqStepOver:
			pending = s.execStepOver(pending)
		}

		if s.state == StateExited {
			close(s.Events)
			return
		}
	}
}

func (s *Session) doSetBP(sourceLine int) (*Breakpoint, error) {
	if s.info == nil {
		return nil, fmt.Errorf("no debug info")
	}
	codeAddr := -1
	for _, dl := range s.info.Lines {
		if dl.Line == sourceLine {
			codeAddr = dl.CodeAddr
			break
		}
	}
	if codeAddr < 0 {
		return nil, fmt.Errorf("no code at source line %d", sourceLine)
	}
	va := s.codeAddrToVA(codeAddr)
	origByte, err := PeekByte(s.pid, va)
	if err != nil {
		return nil, fmt.Errorf("SetBreakpoint: peek: %w", err)
	}
	if err := PokeByte(s.pid, va, 0xCC); err != nil {
		return nil, fmt.Errorf("SetBreakpoint: poke INT3: %w", err)
	}
	bp := &Breakpoint{
		ID:         len(s.Breakpoints) + 1,
		SourceLine: sourceLine,
		CodeAddr:   va,
		OrigByte:   origByte,
		Enabled:    true,
	}
	s.Breakpoints = append(s.Breakpoints, bp)
	return bp, nil
}

func (s *Session) doRemoveBP(bp *Breakpoint) error {
	if bp == nil || !bp.Enabled {
		return nil
	}
	if err := PokeByte(s.pid, bp.CodeAddr, bp.OrigByte); err != nil {
		return err
	}
	bp.Enabled = false
	for i, b := range s.Breakpoints {
		if b == bp {
			s.Breakpoints = append(s.Breakpoints[:i], s.Breakpoints[i+1:]...)
			break
		}
	}
	return nil
}

// stepOverPending single-steps once to execute the restored original byte,
// then reinstalls 0xCC. Returns false if the process exited during the step.
func (s *Session) stepOverPending(pending *Breakpoint) bool {
	if pending == nil {
		return true
	}
	if err := SingleStep(s.pid); err != nil {
		return false
	}
	ws, err := Wait(s.pid)
	if err != nil || ws.Exited() || ws.Signaled() {
		s.handleWaitStatus(ws)
		return false
	}
	PokeByte(s.pid, pending.CodeAddr, 0xCC)
	return true
}

func (s *Session) execRun(pending *Breakpoint) *Breakpoint {
	if !s.stepOverPending(pending) {
		return nil
	}
	if err := Continue(s.pid); err != nil {
		s.sendSignalStop()
		return nil
	}
	ws, err := Wait(s.pid)
	if err != nil {
		s.sendSignalStop()
		return nil
	}
	return s.handleWaitStatus(ws)
}

func (s *Session) execStepInto(pending *Breakpoint) *Breakpoint {
	if !s.stepOverPending(pending) {
		return nil
	}
	startLine := s.currentLine()
	const maxSteps = 100_000
	for i := 0; i < maxSteps; i++ {
		if err := SingleStep(s.pid); err != nil {
			s.sendSignalStop()
			return nil
		}
		ws, err := Wait(s.pid)
		if err != nil || ws.Exited() || ws.Signaled() {
			return s.handleWaitStatus(ws)
		}
		if ws.Stopped() && ws.StopSignal() == syscall.SIGTRAP {
			regs, _ := GetRegs(s.pid)
			rip := uintptr(regs.Rip)
			// If a breakpoint was hit mid-step, defer to handleWaitStatus.
			if bp, _ := s.findBPAt(rip - 1); bp != nil {
				return s.handleWaitStatus(ws)
			}
			newLine := s.ripToLine(regs.Rip)
			if newLine != startLine && newLine > 0 {
				vars := s.readAllVars()
				s.state = StateStopped
				s.Events <- StopEvent{
					Reason:  StopStep,
					Line:    newLine,
					BPIndex: -1,
					Vars:    vars,
				}
				return nil
			}
		}
	}
	// Stepped too many times — report current position.
	line := s.currentLine()
	s.state = StateStopped
	s.Events <- StopEvent{Reason: StopStep, Line: line, BPIndex: -1, Vars: s.readAllVars()}
	return nil
}

func (s *Session) execStepOver(pending *Breakpoint) *Breakpoint {
	if !s.stepOverPending(pending) {
		return nil
	}
	regs, err := GetRegs(s.pid)
	if err != nil {
		s.sendSignalStop()
		return nil
	}
	rip := uintptr(regs.Rip)

	// Check if the current instruction is a relative call (opcode 0xE8).
	b, _ := PeekByte(s.pid, rip)
	if b == 0xE8 {
		retAddr := rip + 5 // instruction after the 5-byte call
		origByte, err := PeekByte(s.pid, retAddr)
		if err == nil {
			PokeByte(s.pid, retAddr, 0xCC)
			if err := Continue(s.pid); err == nil {
				ws, _ := Wait(s.pid)
				PokeByte(s.pid, retAddr, origByte) // always restore
				if ws.Stopped() && ws.StopSignal() == syscall.SIGTRAP {
					checkRegs, _ := GetRegs(s.pid)
					if uintptr(checkRegs.Rip) == retAddr+1 {
						// Hit temp BP: fix rip back and emit step stop.
						checkRegs.Rip = uint64(retAddr)
						SetRegs(s.pid, checkRegs)
						line := s.ripToLine(checkRegs.Rip)
						s.state = StateStopped
						s.Events <- StopEvent{
							Reason:  StopStep,
							Line:    line,
							BPIndex: -1,
							Vars:    s.readAllVars(),
						}
						return nil
					}
				}
				// Otherwise fell through to a real breakpoint or exit.
				return s.handleWaitStatus(ws)
			}
		}
	}

	// Not a call (or temp-BP setup failed): single-step until line changes.
	startLine := s.ripToLine(regs.Rip)
	const maxSteps = 100_000
	for i := 0; i < maxSteps; i++ {
		if err := SingleStep(s.pid); err != nil {
			s.sendSignalStop()
			return nil
		}
		ws, err := Wait(s.pid)
		if err != nil || ws.Exited() || ws.Signaled() {
			return s.handleWaitStatus(ws)
		}
		if ws.Stopped() && ws.StopSignal() == syscall.SIGTRAP {
			checkRegs, _ := GetRegs(s.pid)
			rip := uintptr(checkRegs.Rip)
			if bp, _ := s.findBPAt(rip - 1); bp != nil {
				return s.handleWaitStatus(ws)
			}
			newLine := s.ripToLine(checkRegs.Rip)
			if newLine != startLine && newLine > 0 {
				s.state = StateStopped
				s.Events <- StopEvent{
					Reason:  StopStep,
					Line:    newLine,
					BPIndex: -1,
					Vars:    s.readAllVars(),
				}
				return nil
			}
		}
	}
	line := s.currentLine()
	s.state = StateStopped
	s.Events <- StopEvent{Reason: StopStep, Line: line, BPIndex: -1, Vars: s.readAllVars()}
	return nil
}

// handleWaitStatus analyses ws and sends an appropriate StopEvent.
// Returns the BP that needs reinstallation (non-nil for breakpoint hits only).
func (s *Session) handleWaitStatus(ws syscall.WaitStatus) *Breakpoint {
	if ws.Exited() {
		s.state = StateExited
		s.Events <- StopEvent{Reason: StopExited, ExitCode: ws.ExitStatus(), BPIndex: -1}
		return nil
	}
	if ws.Signaled() {
		s.state = StateExited
		s.Events <- StopEvent{Reason: StopSignal, BPIndex: -1}
		return nil
	}
	if !ws.Stopped() {
		return nil
	}

	if ws.StopSignal() != syscall.SIGTRAP {
		s.state = StateStopped
		s.Events <- StopEvent{Reason: StopSignal, Line: s.currentLine(), BPIndex: -1}
		return nil
	}

	regs, err := GetRegs(s.pid)
	if err != nil {
		s.state = StateStopped
		s.Events <- StopEvent{Reason: StopSignal, BPIndex: -1}
		return nil
	}
	rip := uintptr(regs.Rip)

	// INT3 advances rip by 1; check if rip-1 is a known breakpoint address.
	if bp, idx := s.findBPAt(rip - 1); bp != nil {
		PokeByte(s.pid, bp.CodeAddr, bp.OrigByte)
		regs.Rip = uint64(bp.CodeAddr)
		SetRegs(s.pid, regs)

		line := s.ripToLine(regs.Rip)
		s.state = StateStopped
		s.Events <- StopEvent{
			Reason:  StopBreakpoint,
			Line:    line,
			BPIndex: idx,
			Vars:    s.readAllVars(),
		}
		return bp
	}

	// Pure single-step SIGTRAP (no matching BP).
	s.state = StateStopped
	s.Events <- StopEvent{
		Reason:  StopStep,
		Line:    s.ripToLine(uint64(rip)),
		BPIndex: -1,
		Vars:    s.readAllVars(),
	}
	return nil
}

func (s *Session) sendSignalStop() {
	s.state = StateStopped
	s.Events <- StopEvent{Reason: StopSignal, BPIndex: -1}
}

// readLoadBase reads the ELF load base address from /proc/<pid>/maps.
// Falls back to 0x400000 (the expected base for our static binaries).
func readLoadBase(pid int, binaryPath string) uintptr {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return 0x400000
	}
	// Extract the basename of the binary for matching.
	baseName := binaryPath
	if idx := strings.LastIndexByte(binaryPath, '/'); idx >= 0 {
		baseName = binaryPath[idx+1:]
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, baseName) {
			continue
		}
		var start uint64
		if _, err := fmt.Sscanf(line, "%x-", &start); err == nil && start > 0 {
			return uintptr(start)
		}
	}
	return 0x400000
}
