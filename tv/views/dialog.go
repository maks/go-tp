package views

import "go-tp/tv/core"

// Dialog is a modal Window that closes when it receives CmOK or CmCancel.
// Use Desktop.ExecView to run it.
type Dialog struct {
	*Window
	closed bool
	Result core.CommandId // CmOK or CmCancel when closed
}

// NewDialog creates a Dialog with the given bounds and title.
func NewDialog(bounds core.Rect, title string) *Dialog {
	d := &Dialog{}
	d.Window = NewWindow(bounds, title)
	return d
}

func (d *Dialog) HandleEvent(ev *core.Event) {
	if ev.Handled {
		return
	}
	// ESC → cancel.
	if ev.Type == core.EvKeyboard && ev.Key == core.KbEsc {
		d.closed = true
		d.Result = core.CmCancel
		ev.Handled = true
		return
	}
	d.Window.HandleEvent(ev)
	// Command events that close the dialog.
	if ev.Type == core.EvCommand {
		switch ev.Cmd {
		case core.CmOK, core.CmCancel, core.CmClose:
			d.closed = true
			d.Result = ev.Cmd
			ev.Handled = true
		}
	}
}

// Close programmatically closes the dialog with result.
func (d *Dialog) Close(result core.CommandId) {
	d.closed = true
	d.Result = result
}
