package app

import (
	"go-tp/tv/backend"
	"go-tp/tv/core"
	"go-tp/tv/views"
)

// Application owns the backend, desktop, menu bar, and status line.
// It runs the single-threaded event + render loop.
type Application struct {
	backend    backend.DisplayBackend
	desktop    *views.Desktop
	menuBar    *views.MenuBar
	statusLine *views.StatusLine
	// drawBuf is the full-screen DrawBuffer used for rendering each frame.
	drawBuf *core.DrawBuffer
	// prevBuf holds the last flushed frame for diff-based rendering.
	prevBuf *core.DrawBuffer
	running bool
	// CommandHandler is called for application-level commands.
	CommandHandler func(core.CommandId)
	// TickHandler, if set, is called once per event-loop iteration before
	// polling for input. Use it to inject background events (e.g. debugger).
	TickHandler func()
}

// New creates an Application with the given backend.
func New(b backend.DisplayBackend) *Application {
	return &Application{backend: b}
}

// Init initialises the backend and sets up the layout.
func (a *Application) Init() error {
	if err := a.backend.Init(); err != nil {
		return err
	}
	cols, rows := a.backend.Size()
	// Reserve top row for menu bar and bottom row for status line.
	menuY := 0
	statusY := rows - 1
	desktopY := 1
	desktopH := rows - 2
	if desktopH < 1 {
		desktopY = 0
		desktopH = rows
	}

	a.desktop = views.NewDesktop(core.Rect{X: 0, Y: desktopY, W: cols, H: desktopH})
	a.menuBar = views.NewMenuBar(core.Rect{X: 0, Y: menuY, W: cols, H: 1}, nil)
	a.menuBar.CommandHandler = a.dispatchCommand
	a.statusLine = views.NewStatusLine(core.Rect{X: 0, Y: statusY, W: cols, H: 1}, nil)
	a.statusLine.CommandHandler = a.dispatchCommand

	a.drawBuf = core.NewDrawBuffer(cols, rows, core.AttrNormal)
	a.prevBuf = core.NewDrawBuffer(cols, rows, 0)

	a.running = true
	return nil
}

// SetMenuBar replaces the menu bar items.
func (a *Application) SetMenuBar(items []*views.MenuItem) {
	cols, _ := a.backend.Size()
	a.menuBar = views.NewMenuBar(core.Rect{X: 0, Y: 0, W: cols, H: 1}, items)
	a.menuBar.CommandHandler = a.dispatchCommand
}

// SetStatusLine replaces the status line items.
func (a *Application) SetStatusLine(items []views.StatusItem) {
	cols, rows := a.backend.Size()
	a.statusLine = views.NewStatusLine(core.Rect{X: 0, Y: rows - 1, W: cols, H: 1}, items)
	a.statusLine.CommandHandler = a.dispatchCommand
}

// Desktop returns the desktop view.
func (a *Application) Desktop() *views.Desktop { return a.desktop }

// Run enters the main event loop. It blocks until Stop is called.
func (a *Application) Run() {
	for a.running {
		if a.TickHandler != nil {
			a.TickHandler()
		}
		a.render()
		ev := a.backend.PollEvent()
		if ev != nil {
			a.handleEvent(ev)
		}
	}
}

// Stop signals the event loop to exit after the current iteration.
func (a *Application) Stop() {
	a.running = false
}

// Close shuts down the backend cleanly.
func (a *Application) Close() {
	a.backend.Close()
}

func (a *Application) handleEvent(ev *core.Event) {
	// Status line gets first crack at hotkeys.
	if ev.Type == core.EvKeyboard {
		a.statusLine.HandleEvent(ev)
		if ev.Handled {
			return
		}
		a.menuBar.HandleEvent(ev)
		if ev.Handled {
			return
		}
	}
	// Mouse on menu bar.
	if ev.Type == core.EvMouseDown || ev.Type == core.EvMouseMove || ev.Type == core.EvMouseUp {
		if a.menuBar.Bounds().Contains(core.Point{X: ev.MouseX, Y: ev.MouseY}) {
			a.menuBar.HandleEvent(ev)
			return
		}
	}
	// Commands.
	if ev.Type == core.EvCommand {
		a.dispatchCommand(ev.Cmd)
		return
	}
	// Everything else goes to the desktop.
	a.desktop.HandleEvent(ev)
}

func (a *Application) dispatchCommand(cmd core.CommandId) {
	if cmd == core.CmQuit {
		a.Stop()
		return
	}
	if a.CommandHandler != nil {
		a.CommandHandler(cmd)
	}
}

// render draws the full frame into drawBuf, diffs against prevBuf, and flushes
// only changed cells to the backend.
func (a *Application) render() {
	cols, rows := a.backend.Size()
	// Menu bar.
	menuSub := core.NewDrawBuffer(cols, 1, core.AttrMenuBar)
	a.menuBar.Draw(menuSub)
	a.drawBuf.CopyFrom(menuSub, 0, 0)
	// Desktop.
	desktop := a.desktop
	db := desktop.Bounds()
	deskSub := core.NewDrawBuffer(db.W, db.H, core.AttrDesktop)
	desktop.Draw(deskSub)
	a.drawBuf.CopyFrom(deskSub, db.X, db.Y)
	// Status line.
	statusSub := core.NewDrawBuffer(cols, 1, core.AttrStatusLine)
	a.statusLine.Draw(statusSub)
	a.drawBuf.CopyFrom(statusSub, 0, rows-1)
	// Popup menu overlay.
	a.menuBar.DrawPopup(a.drawBuf, 0, 0)

	// Diff and flush.
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			i := y*cols + x
			d := a.drawBuf.Cells[i]
			p := a.prevBuf.Cells[i]
			if d != p {
				a.backend.SetCell(x, y, d.Ch, d.Attr)
				a.prevBuf.Cells[i] = d
			}
		}
	}
	a.backend.Flush()
}

// PollEvent exposes the backend poll for use by ExecView.
func (a *Application) PollEvent() *core.Event {
	return a.backend.PollEvent()
}
