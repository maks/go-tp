package core

// CommandId identifies a UI command dispatched through the event system.
type CommandId uint16

const (
	CmQuit    CommandId = 1
	CmClose   CommandId = 2
	CmOK      CommandId = 3
	CmCancel  CommandId = 4
	CmZoom    CommandId = 5
	CmNew     CommandId = 10
	CmOpen    CommandId = 11
	CmSave    CommandId = 12
	CmSaveAs  CommandId = 13
	CmCut     CommandId = 20
	CmCopy    CommandId = 21
	CmPaste   CommandId = 22
	CmUndo    CommandId = 23
	CmRedo    CommandId = 24
	CmFind    CommandId = 25
	CmReplace CommandId = 26
)
