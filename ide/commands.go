package ide

import "go-tp/tv/core"

// IDE-specific command IDs (start at 1000 to avoid conflict with tv/core commands).
const (
	CmBuild      core.CommandId = 1000 // F9: compile current file
	CmRun        core.CommandId = 1001 // Ctrl+F9: compile and run
	CmGotoErr    core.CommandId = 1002 // fired by OutputWindow on error-line activation
	CmDebugRun   core.CommandId = 1003 // F5: start/continue debug
	CmStepOver   core.CommandId = 1004 // F8: step over
	CmStepInto   core.CommandId = 1005 // F7: step into
	CmStopDebug  core.CommandId = 1006 // Ctrl+F2: stop debugger
	CmToggleBP   core.CommandId = 1007 // Ctrl+F5: toggle breakpoint at cursor line
)
