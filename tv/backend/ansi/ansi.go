package ansi

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"go-tp/tv/core"
)

// Backend implements backend.DisplayBackend for a Linux raw terminal.
// It uses direct ANSI escape codes for output and non-blocking stdin reads.
type Backend struct {
	fd       int
	oldState [unsafe.Sizeof(syscall.Termios{})]byte
	cols     int
	rows     int
	cells    []core.Cell   // current screen state (what was last flushed)
	dirty    []core.Cell   // pending state to flush
	parser   core.VT100Parser
	mouseOn  bool
	pendingMouse []byte    // raw bytes buffered for X10 mouse report
}

// New creates an ANSI backend targeting the given file descriptor (usually 1 for stdout).
func New() *Backend {
	return &Backend{fd: 1}
}

// Init switches stdin to raw mode and enables mouse reporting.
func (b *Backend) Init() error {
	// Get terminal size.
	cols, rows, err := getTermSize(1)
	if err != nil {
		cols, rows = 80, 24
	}
	b.cols = cols
	b.rows = rows

	// Put stdin (fd 0) into raw mode.
	if err := makeRaw(0, &b.oldState); err != nil {
		return err
	}

	b.cells = make([]core.Cell, cols*rows)
	b.dirty = make([]core.Cell, cols*rows)
	for i := range b.cells {
		b.cells[i] = core.Cell{Ch: ' ', Attr: 0}
		b.dirty[i] = core.Cell{Ch: ' ', Attr: 0}
	}

	// Hide cursor, clear screen.
	fmt.Fprint(os.Stdout, "\x1b[?25l\x1b[2J")
	// Enable mouse reporting (X10 mode).
	fmt.Fprint(os.Stdout, "\x1b[?1000h")
	b.mouseOn = true
	return nil
}

// Size returns the current terminal dimensions.
func (b *Backend) Size() (cols, rows int) { return b.cols, b.rows }

// SetCell marks a cell as dirty with new content.
func (b *Backend) SetCell(x, y int, ch rune, attr core.Attr) {
	if x < 0 || y < 0 || x >= b.cols || y >= b.rows {
		return
	}
	b.dirty[y*b.cols+x] = core.Cell{Ch: ch, Attr: attr}
}

// Flush writes all dirty cells to the terminal using minimal ANSI sequences.
func (b *Backend) Flush() {
	var sb strings.Builder
	lastAttr := core.Attr(0xFF) // impossible sentinel

	for y := 0; y < b.rows; y++ {
		for x := 0; x < b.cols; x++ {
			i := y*b.cols + x
			d := b.dirty[i]
			c := b.cells[i]
			if d == c {
				continue
			}
			b.cells[i] = d

			// Move cursor.
			fmt.Fprintf(&sb, "\x1b[%d;%dH", y+1, x+1)

			// Set colors if changed.
			if d.Attr != lastAttr {
				fg := core.ANSIFg[d.Attr.Fg()]
				bg := core.ANSIBg[d.Attr.Bg()]
				fmt.Fprintf(&sb, "\x1b[0;%d;%dm", fg, bg)
				lastAttr = d.Attr
			}

			// Write the character.
			if d.Ch == 0 {
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(d.Ch)
			}
		}
	}

	// Reset and reposition cursor off-screen.
	sb.WriteString("\x1b[0m")
	os.Stdout.WriteString(sb.String())
}

// PollEvent reads available bytes from stdin (non-blocking) and returns an
// Event if one is complete. Returns nil if no event is ready.
func (b *Backend) PollEvent() *core.Event {
	// Non-blocking read: use select with a zero timeout.
	var rfds syscall.FdSet
	fdSet(&rfds, 0)
	tv := syscall.Timeval{Sec: 0, Usec: 0}
	n, err := syscall.Select(1, &rfds, nil, nil, &tv)
	if err != nil || n == 0 {
		// Also flush any pending lone ESC after a short pause.
		return b.checkFlush()
	}

	buf := make([]byte, 64)
	nr, err := syscall.Read(0, buf)
	if err != nil || nr == 0 {
		return b.checkFlush()
	}
	buf = buf[:nr]

	for i := 0; i < len(buf); i++ {
		raw := buf[i]

		// Collect X10 mouse bytes directly if we detected ESC [ M earlier.
		if len(b.pendingMouse) > 0 {
			b.pendingMouse = append(b.pendingMouse, raw)
			if len(b.pendingMouse) == 3 {
				ev := core.FeedMouseReport(b.pendingMouse[0], b.pendingMouse[1], b.pendingMouse[2])
				b.pendingMouse = b.pendingMouse[:0]
				return &ev
			}
			continue
		}

		// Intercept ESC [ M before the VT100 parser: it's an X10 mouse report.
		// The sequence is 6 bytes: ESC [ M b x y.
		if raw == 0x1B && i+5 < len(buf) && buf[i+1] == '[' && buf[i+2] == 'M' {
			ev := core.FeedMouseReport(buf[i+3], buf[i+4], buf[i+5])
			i += 5 // skip remaining 5 bytes (loop adds +1 = 6 total)
			return &ev
		}

		// Intercept Ctrl+F9: ESC [ 2 0 ; 5 ~ (7 bytes).
		if raw == 0x1B && i+6 < len(buf) &&
			buf[i+1] == '[' && buf[i+2] == '2' && buf[i+3] == '0' &&
			buf[i+4] == ';' && buf[i+5] == '5' && buf[i+6] == '~' {
			ev := core.KeyEvent(core.KbCtrlF9, 0)
			i += 6
			return &ev
		}

		ev := b.parser.Feed(raw)
		if ev.Type == core.EvNothing {
			continue
		}
		return &ev
	}

	// Check if a complete lone ESC is sitting in the parser.
	if ev := b.parser.Flush(); ev.Type != core.EvNothing {
		return &ev
	}
	return nil
}

func (b *Backend) checkFlush() *core.Event {
	_ = time.Now() // satisfy import
	ev := b.parser.Flush()
	if ev.Type != core.EvNothing {
		return &ev
	}
	return nil
}

// Close disables mouse, shows cursor, and restores terminal state.
func (b *Backend) Close() {
	if b.mouseOn {
		fmt.Fprint(os.Stdout, "\x1b[?1000l")
	}
	fmt.Fprint(os.Stdout, "\x1b[?25h\x1b[2J\x1b[H\x1b[0m")
	restoreRaw(0, &b.oldState)
}

// -----------------------------------------------------------------------------
// Low-level terminal helpers — syscall-only, no golang.org/x/term dependency.
// -----------------------------------------------------------------------------

// makeRaw puts fd into raw mode and saves the old termios in dst.
func makeRaw(fd int, dst *[unsafe.Sizeof(syscall.Termios{})]byte) error {
	var termios syscall.Termios
	if err := ioctlTermios(fd, syscall.TCGETS, &termios); err != nil {
		return err
	}
	// Save.
	*(*syscall.Termios)(unsafe.Pointer(dst)) = termios

	// Apply raw settings (cfmakeraw equivalent).
	termios.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	termios.Oflag &^= syscall.OPOST
	termios.Cflag |= syscall.CS8
	termios.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	termios.Cc[syscall.VMIN] = 1
	termios.Cc[syscall.VTIME] = 0
	return ioctlTermios(fd, syscall.TCSETS, &termios)
}

// restoreRaw restores fd from the saved termios.
func restoreRaw(fd int, saved *[unsafe.Sizeof(syscall.Termios{})]byte) {
	termios := (*syscall.Termios)(unsafe.Pointer(saved))
	ioctlTermios(fd, syscall.TCSETS, termios) //nolint
}

func ioctlTermios(fd int, req uint, arg *syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(req),
		uintptr(unsafe.Pointer(arg)))
	if errno != 0 {
		return errno
	}
	return nil
}

// getTermSize returns the terminal dimensions using TIOCGWINSZ.
func getTermSize(fd int) (cols, rows int, err error) {
	type winsize struct {
		Row, Col, Xpixel, Ypixel uint16
	}
	var ws winsize
	const TIOCGWINSZ = 0x5413
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), TIOCGWINSZ,
		uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 0, 0, errno
	}
	return int(ws.Col), int(ws.Row), nil
}

// fdSet sets bit fd in an FdSet.
func fdSet(set *syscall.FdSet, fd int) {
	set.Bits[fd/64] |= 1 << uint(fd%64)
}
