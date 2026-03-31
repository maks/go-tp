package ide

import "go-tp/tv/core"

// IDE-specific command IDs (start at 1000 to avoid conflict with tv/core commands).
const (
	CmBuild   core.CommandId = 1000 // F9: compile current file
	CmRun     core.CommandId = 1001 // Ctrl+F9: compile and run
	CmGotoErr core.CommandId = 1002 // fired by OutputWindow on error-line activation
)
