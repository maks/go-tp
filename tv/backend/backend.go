package backend

import "go-tp/tv/core"

// DisplayBackend is the only target-specific seam in the codebase.
// The ANSI backend implements it for Linux terminals; the RP2040 backend
// implements it for the ST7789 LCD + USB CDC.
type DisplayBackend interface {
	// Init sets up the display and input hardware.
	Init() error
	// Size returns the current display dimensions in character cells.
	Size() (cols, rows int)
	// SetCell queues a cell update. Changes are batched until Flush.
	SetCell(x, y int, ch rune, attr core.Attr)
	// Flush sends all queued cell updates to the display.
	Flush()
	// PollEvent is non-blocking. Returns nil if no event is ready.
	PollEvent() *core.Event
	// Close restores the display/input to its previous state.
	Close()
}
