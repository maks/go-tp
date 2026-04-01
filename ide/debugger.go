//go:build !rp2040

package ide

import (
	"go-tp/debugger"
	"go-tp/pascal"
)

// IdeDebugger bridges a debugger.Session to the IDE editor and watch window.
// All public methods are safe to call from the IDE event loop goroutine.
type IdeDebugger struct {
	session      *debugger.Session
	editorWin    *IdeEditorWindow
	watchWin     *WatchWindow
	installedBPs map[int]*debugger.Breakpoint // source line → Breakpoint
}

// NewIdeDebugger creates an IdeDebugger wired to the given editor and watch windows.
func NewIdeDebugger(ew *IdeEditorWindow, ww *WatchWindow) *IdeDebugger {
	return &IdeDebugger{
		editorWin:    ew,
		watchWin:     ww,
		installedBPs: make(map[int]*debugger.Breakpoint),
	}
}

// HasSession reports whether a debug session is active.
func (d *IdeDebugger) HasSession() bool { return d.session != nil }

// Start launches a new debug session for the given binary with the provided
// debug info. It installs breakpoints for every line in the editor that has
// been toggled on, then waits at the initial stop (before first instruction).
func (d *IdeDebugger) Start(binaryPath string, info *pascal.DebugInfo) error {
	sess, err := debugger.NewSession(binaryPath, info)
	if err != nil {
		return err
	}
	d.session = sess
	d.installedBPs = make(map[int]*debugger.Breakpoint)

	// Install breakpoints for all marked lines.
	for line := range d.editorWin.Breakpoints() {
		bp, err := sess.SetBreakpoint(line)
		if err == nil {
			d.installedBPs[line] = bp
		}
	}
	return nil
}

// Stop kills the debug session and cleans up IDE state.
func (d *IdeDebugger) Stop() {
	if d.session == nil {
		return
	}
	d.session.Stop()
	d.session = nil
	d.installedBPs = make(map[int]*debugger.Breakpoint)
	d.editorWin.ClearCurrentLine()
	d.watchWin.Clear()
}

// Run resumes free execution (F5).
func (d *IdeDebugger) Run() {
	if d.session == nil {
		return
	}
	d.editorWin.ClearCurrentLine()
	d.session.Run()
}

// StepOver advances one source line without descending into calls (F8).
func (d *IdeDebugger) StepOver() {
	if d.session == nil {
		return
	}
	d.editorWin.ClearCurrentLine()
	d.session.StepOver()
}

// StepInto advances one source line, descending into calls (F7).
func (d *IdeDebugger) StepInto() {
	if d.session == nil {
		return
	}
	d.editorWin.ClearCurrentLine()
	d.session.StepInto()
}

// ToggleBreakpoint adds or removes a breakpoint at the editor cursor line.
// If a session is active the change is applied immediately via ptrace.
func (d *IdeDebugger) ToggleBreakpoint() {
	line := d.editorWin.CursorLine()
	added := d.editorWin.ToggleBreakpoint(line)
	if d.session == nil {
		return
	}
	if added {
		bp, err := d.session.SetBreakpoint(line)
		if err == nil {
			d.installedBPs[line] = bp
		}
	} else {
		if bp, ok := d.installedBPs[line]; ok {
			d.session.RemoveBreakpoint(bp)
			delete(d.installedBPs, line)
		}
	}
}

// Poll checks for a pending stop event (non-blocking) and updates the IDE.
// Call from the application TickHandler.
func (d *IdeDebugger) Poll() {
	if d.session == nil {
		return
	}
	select {
	case ev, ok := <-d.session.Events:
		if !ok {
			// Channel closed — session ended.
			d.session = nil
			d.editorWin.ClearCurrentLine()
			d.watchWin.Clear()
			return
		}
		d.handleEvent(ev)
	default:
		// No event pending.
	}
}

func (d *IdeDebugger) handleEvent(ev debugger.StopEvent) {
	switch ev.Reason {
	case debugger.StopExited, debugger.StopSignal:
		d.session = nil
		d.editorWin.ClearCurrentLine()
		d.watchWin.Clear()
	case debugger.StopBreakpoint, debugger.StopStep:
		if ev.Line > 0 {
			d.editorWin.SetCurrentLine(ev.Line)
			// Scroll the editor so the current line is visible.
			d.editorWin.Editor().GotoLine(ev.Line-1, d.editorWin.Editor().CursorCol())
		}
		d.watchWin.SetValues(ev.Vars)
	}
}
