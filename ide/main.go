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
	outH := desktopH - edH
	if outH < 3 {
		outH = 3
	}

	editorWin := NewIdeEditorWindow(core.Rect{X: 0, Y: desktopTop, W: cols, H: edH})
	outputWin := NewOutputWindow(core.Rect{X: 0, Y: desktopTop + edH, W: cols, H: outH})

	// Wire goto-error callback.
	outputWin.OnGotoErr = func(srcLine int) {
		// GotoLine is 0-based; diagnostics are 1-based.
		editorWin.Editor().GotoLine(srcLine-1, 0)
		desktop.BringToFront(editorWin.Win())
	}

	// Load initial file from command line.
	if len(os.Args) > 1 {
		if err := editorWin.loadFile(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "ide: cannot open %s: %v\n", os.Args[1], err)
		}
	} else {
		editorWin.setNewFile()
	}

	// Output window added first (lower z-order); editor on top receives keyboard focus.
	desktop.AddWindow(outputWin.Win())
	desktop.AddWindow(editorWin.Win())

	application.SetMenuBar(buildMenuBar())
	application.SetStatusLine(buildStatusLine())

	application.CommandHandler = func(cmd core.CommandId) {
		handleCommand(cmd, application, editorWin, outputWin)
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
		}},
	}
}

func buildStatusLine() []views.StatusItem {
	return []views.StatusItem{
		{Label: "F2 Save", Key: core.KbF2, Cmd: core.CmSave},
		{Label: "F3 Open", Key: core.KbF3, Cmd: core.CmOpen},
		{Label: "F9 Build", Key: core.KbF9, Cmd: CmBuild},
		{Label: "^F9 Run", Key: core.KbCtrlF9, Cmd: CmRun},
	}
}

func handleCommand(cmd core.CommandId, a *app.Application, ew *IdeEditorWindow, ow *OutputWindow) {
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
	}
}

func doBuild(ew *IdeEditorWindow, ow *OutputWindow) (string, bool) {
	ow.Clear()
	ow.AppendLine("Building...", attrOutputNormal)

	src := ew.editor.GetText()
	outPath := buildOutputPath(ew.filePath)

	gen := x86codegen.New(outPath)
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
		return "", false
	}

	ew.SetErrorLines(nil)
	ow.AppendLine("Build OK — "+outPath, attrOutputOK)
	return outPath, true
}

func doRun(ew *IdeEditorWindow, ow *OutputWindow) {
	outPath, ok := doBuild(ew, ow)
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

	// Stream stdout (blocking — acceptable since the event loop is single-threaded).
	// Use a goroutine + channel so the IDE remains responsive during long programs.
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
