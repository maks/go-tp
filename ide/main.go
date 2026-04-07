//go:build !rp2040

package ide

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go-tp/pascal"
	x86codegen "go-tp/pascal/codegen/x86_64"
	"go-tp/tv/app"
	"go-tp/tv/backend/ansi"
	"go-tp/tv/core"
	"go-tp/tv/views"
)

// currentOutputFormat is the user-selected binary output format.
// Defaults to ELF (Linux). Toggled via the Compile > Output Format menu.
var currentOutputFormat = x86codegen.FormatELF

func execCommand(path string) *exec.Cmd {
	return exec.Command(path)
}

// Run is the main entry point for the IDE.
func Run() {
	backend := ansi.New()
	application := app.New(backend)
	if err := application.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "ide: init failed: %v\n", err)
		os.Exit(1)
	}
	defer application.Close()

	cols, rows := backend.Size()
	desktop := application.Desktop()

	// Desktop occupies rows 1..(rows-2); row 0 = menu bar, row rows-1 = status line.
	const desktopTop = 1
	desktopH := rows - 2
	edH := desktopH * 7 / 10
	if edH < 5 {
		edH = 5
	}
	bottomH := desktopH - edH
	if bottomH < 3 {
		bottomH = 3
	}
	// Split bottom pane: output(60%) | watches(40%).
	outW := cols * 6 / 10
	if outW < 20 {
		outW = 20
	}
	watchW := cols - outW
	if watchW < 10 {
		watchW = 10
	}

	editorWin := NewIdeEditorWindow(core.Rect{X: 0, Y: desktopTop, W: cols, H: edH})
	outputWin := NewOutputWindow(core.Rect{X: 0, Y: desktopTop + edH, W: outW, H: bottomH})
	watchWin := NewWatchWindow(core.Rect{X: outW, Y: desktopTop + edH, W: watchW, H: bottomH})

	// Wire goto-error callback.
	outputWin.OnGotoErr = func(srcLine int) {
		editorWin.Editor().GotoLine(srcLine-1, 0)
		desktop.BringToFront(editorWin.Win())
	}

	// Create debugger bridge.
	ideDbg := NewIdeDebugger(editorWin, watchWin)

	// Wire tick handler for non-blocking debug event polling.
	application.TickHandler = func() { ideDbg.Poll() }

	// Load initial file from command line.
	if len(os.Args) > 1 {
		if err := editorWin.loadFile(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "ide: cannot open %s: %v\n", os.Args[1], err)
		}
	} else {
		editorWin.setNewFile()
	}

	// Output window and watch window added first (lower z-order); editor on top.
	desktop.AddWindow(watchWin.Win())
	desktop.AddWindow(outputWin.Win())
	desktop.AddWindow(editorWin.Win())

	application.SetMenuBar(buildMenuBar())
	application.SetStatusLine(buildStatusLine())

	application.CommandHandler = func(cmd core.CommandId) {
		handleCommand(cmd, application, editorWin, outputWin, ideDbg)
	}

	editorWin.Win().SetInitialFocus()
	application.Run()
}

func buildMenuBar() []*views.MenuItem {
	return []*views.MenuItem{
		{Label: "File", SubMenu: []*views.MenuItem{
			{Label: "New", Cmd: core.CmNew},
			{Label: "Open...", Cmd: core.CmOpen, HotKey: core.KbF3, HotText: "F3"},
			{Label: "Save", Cmd: core.CmSave, HotKey: core.KbF2, HotText: "F2"},
			{Label: "Save As...", Cmd: core.CmSaveAs},
			views.Sep(),
			{Label: "Quit", Cmd: core.CmQuit},
		}},
		{Label: "Edit", SubMenu: []*views.MenuItem{
			{Label: "Undo", Cmd: core.CmUndo},
			{Label: "Redo", Cmd: core.CmRedo},
			views.Sep(),
			{Label: "Cut", Cmd: core.CmCut},
			{Label: "Copy", Cmd: core.CmCopy},
			{Label: "Paste", Cmd: core.CmPaste},
		}},
		{Label: "Compile", SubMenu: []*views.MenuItem{
			{Label: "Build", Cmd: CmBuild, HotKey: core.KbF9, HotText: "F9"},
			{Label: "Run", Cmd: CmRun, HotKey: core.KbCtrlF9, HotText: "^F9"},
			views.Sep(),
			{Label: "Output Format", SubMenu: []*views.MenuItem{
				{Label: "Linux (ELF64)", Cmd: CmSetFormatELF},
				{Label: "macOS (Mach-O)", Cmd: CmSetFormatMachO},
			}},
		}},
		{Label: "Debug", SubMenu: []*views.MenuItem{
			{Label: "Debug Run", Cmd: CmDebugRun, HotKey: core.KbF5, HotText: "F5"},
			{Label: "Step Over", Cmd: CmStepOver, HotKey: core.KbF8, HotText: "F8"},
			{Label: "Step Into", Cmd: CmStepInto, HotKey: core.KbF7, HotText: "F7"},
			views.Sep(),
			{Label: "Toggle Breakpoint", Cmd: CmToggleBP, HotKey: core.KbCtrlF5, HotText: "^F5"},
			{Label: "Stop Debugger", Cmd: CmStopDebug, HotKey: core.KbCtrlF2, HotText: "^F2"},
		}},
	}
}

func buildStatusLine() []views.StatusItem {
	return []views.StatusItem{
		{Label: "F2 Save", Key: core.KbF2, Cmd: core.CmSave},
		{Label: "F3 Open", Key: core.KbF3, Cmd: core.CmOpen},
		{Label: "F5 Debug", Key: core.KbF5, Cmd: CmDebugRun},
		{Label: "F7 Into", Key: core.KbF7, Cmd: CmStepInto},
		{Label: "F8 Over", Key: core.KbF8, Cmd: CmStepOver},
		{Label: "F9 Build", Key: core.KbF9, Cmd: CmBuild},
		{Label: "^F2 Stop", Key: core.KbCtrlF2, Cmd: CmStopDebug},
		{Label: "^F5 BP", Key: core.KbCtrlF5, Cmd: CmToggleBP},
		{Label: "^F9 Run", Key: core.KbCtrlF9, Cmd: CmRun},
	}
}

func handleCommand(cmd core.CommandId, a *app.Application, ew *IdeEditorWindow, ow *OutputWindow, ideDbg *IdeDebugger) {
	switch cmd {
	case core.CmNew:
		ew.setNewFile()
	case core.CmOpen:
		doOpenDialog(a, ew, ow)
	case core.CmSave:
		if err := ew.saveFile(); err != nil {
			ow.AppendLine("Save failed: "+err.Error(), attrOutputError)
		}
	case core.CmSaveAs:
		doSaveAsDialog(a, ew, ow)
	case CmBuild:
		doBuild(ew, ow)
	case CmRun:
		doRun(ew, ow)
	case CmDebugRun:
		doDebugRun(ew, ow, ideDbg)
	case CmStepOver:
		ideDbg.StepOver()
	case CmStepInto:
		ideDbg.StepInto()
	case CmStopDebug:
		ideDbg.Stop()
		ow.AppendLine("Debugger stopped.", attrOutputNormal)
	case CmToggleBP:
		ideDbg.ToggleBreakpoint()
	case CmSetFormatELF:
		currentOutputFormat = x86codegen.FormatELF
		ow.AppendLine("Output format: Linux (ELF64)", attrOutputNormal)
	case CmSetFormatMachO:
		currentOutputFormat = x86codegen.FormatMachO
		ow.AppendLine("Output format: macOS (Mach-O)", attrOutputNormal)
	}
}

// doBuild compiles the current file. Returns (outputPath, debugInfo, ok).
func doBuild(ew *IdeEditorWindow, ow *OutputWindow) (string, *pascal.DebugInfo, bool) {
	ow.Clear()
	ow.AppendLine("Building...", attrOutputNormal)

	src := ew.editor.GetText()
	outPath := buildOutputPath(ew.filePath)

	gen := x86codegen.New(outPath)
	gen.Format = currentOutputFormat
	compiler := pascal.NewCompiler(src, gen)
	diags := compiler.Compile()

	if len(diags) > 0 {
		errorLines := make(map[int]bool)
		for _, d := range diags {
			ow.AppendError(d)
			if d.Line > 0 {
				errorLines[d.Line] = true
			}
		}
		ew.SetErrorLines(errorLines)
		ow.AppendLine(fmt.Sprintf("Build failed (%d error(s))", len(diags)), attrOutputError)
		return "", nil, false
	}

	ew.SetErrorLines(nil)
	ow.AppendLine("Build OK — "+outPath, attrOutputOK)
	return outPath, gen.DebugInfo(), true
}

func doRun(ew *IdeEditorWindow, ow *OutputWindow) {
	outPath, _, ok := doBuild(ew, ow)
	if !ok {
		return
	}

	ow.AppendLine("Running "+outPath+"...", attrOutputNormal)

	cmd := execCommand(outPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ow.AppendLine("Run failed: "+err.Error(), attrOutputError)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		ow.AppendLine("Run failed: "+err.Error(), attrOutputError)
		return
	}

	if err := cmd.Start(); err != nil {
		ow.AppendLine("Run failed: "+err.Error(), attrOutputError)
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			ow.AppendLine(sc.Text(), attrOutputNormal)
		}
		sc2 := bufio.NewScanner(stderr)
		for sc2.Scan() {
			ow.AppendLine(sc2.Text(), attrOutputError)
		}
		cmd.Wait()
	}()
	<-done
	ow.AppendLine("Program exited.", attrOutputNormal)
}

func doDebugRun(ew *IdeEditorWindow, ow *OutputWindow, ideDbg *IdeDebugger) {
	// If a session is already running, continue execution.
	if ideDbg.HasSession() {
		ideDbg.Run()
		return
	}

	// Build first.
	outPath, info, ok := doBuild(ew, ow)
	if !ok {
		return
	}
	if info == nil {
		ow.AppendLine("No debug info from build.", attrOutputError)
		return
	}

	ow.AppendLine("Starting debugger: "+outPath, attrOutputNormal)
	if err := ideDbg.Start(outPath, info); err != nil {
		ow.AppendLine("Debugger start failed: "+err.Error(), attrOutputError)
		return
	}
	ow.AppendLine("Debugger ready. Press F5 to run, F8 step over, F7 step into.", attrOutputNormal)
	// The process is paused at the initial stop; user presses F5 again to run.
}

func doOpenDialog(a *app.Application, ew *IdeEditorWindow, ow *OutputWindow) {
	dlg := views.NewDialog(core.Rect{X: 10, Y: 5, W: 50, H: 7}, "Open File")
	input := views.NewInputLine(36, 256)
	btn := views.NewButton("OK", core.CmOK)
	dlg.Add(input, core.Rect{X: 1, Y: 1, W: 36, H: 1})
	dlg.Add(btn, core.Rect{X: 38, Y: 1, W: 8, H: 1})

	a.Desktop().ExecView(dlg, a.PollEvent)
	if dlg.Result == core.CmOK {
		path := strings.TrimSpace(input.Value)
		if path != "" {
			if err := ew.loadFile(path); err != nil {
				ow.AppendLine("Open failed: "+err.Error(), attrOutputError)
			}
		}
	}
}

func doSaveAsDialog(a *app.Application, ew *IdeEditorWindow, ow *OutputWindow) {
	dlg := views.NewDialog(core.Rect{X: 10, Y: 5, W: 50, H: 7}, "Save As")
	input := views.NewInputLine(36, 256)
	input.Value = ew.filePath
	btn := views.NewButton("OK", core.CmOK)
	dlg.Add(input, core.Rect{X: 1, Y: 1, W: 36, H: 1})
	dlg.Add(btn, core.Rect{X: 38, Y: 1, W: 8, H: 1})

	a.Desktop().ExecView(dlg, a.PollEvent)
	if dlg.Result == core.CmOK {
		path := strings.TrimSpace(input.Value)
		if path != "" {
			ew.filePath = path
			if err := ew.saveFile(); err != nil {
				ow.AppendLine("Save failed: "+err.Error(), attrOutputError)
			}
		}
	}
}

func buildOutputPath(filePath string) string {
	if filePath == "" {
		return "/tmp/pascal_out"
	}
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, name)
}
