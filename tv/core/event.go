package core

// EventType discriminates the kind of event.
type EventType uint8

const (
	EvNothing   EventType = 0
	EvKeyboard  EventType = 1
	EvMouseDown EventType = 2
	EvMouseUp   EventType = 3
	EvMouseMove EventType = 4
	EvCommand   EventType = 5
	EvBroadcast EventType = 6
)

// KeyCode constants — Borland-compatible virtual key values.
// Low byte is the ASCII value for printable keys; high byte is a scan code.
type KeyCode uint16

const (
	KbNoKey    KeyCode = 0
	KbEsc      KeyCode = 0x001B
	KbEnter    KeyCode = 0x000D
	KbTab      KeyCode = 0x0009
	KbBackSpace KeyCode = 0x0008
	KbUp       KeyCode = 0x4800
	KbDown     KeyCode = 0x5000
	KbLeft     KeyCode = 0x4B00
	KbRight    KeyCode = 0x4D00
	KbHome     KeyCode = 0x4700
	KbEnd      KeyCode = 0x4F00
	KbPgUp     KeyCode = 0x4900
	KbPgDn     KeyCode = 0x5100
	KbIns      KeyCode = 0x5200
	KbDel      KeyCode = 0x5300
	KbF1       KeyCode = 0x3B00
	KbF2       KeyCode = 0x3C00
	KbF3       KeyCode = 0x3D00
	KbF4       KeyCode = 0x3E00
	KbF5       KeyCode = 0x3F00
	KbF6       KeyCode = 0x4000
	KbF7       KeyCode = 0x4100
	KbF8       KeyCode = 0x4200
	KbF9       KeyCode = 0x4300
	KbF10      KeyCode = 0x4400
	KbF11      KeyCode = 0x5700 // non-Borland extension
	KbF12      KeyCode = 0x5800
	// Ctrl+F9
	KbCtrlF9 KeyCode = 0x2300
	// Shift+Tab
	KbShiftTab KeyCode = 0x0F00
	// Alt combinations (Alt+A = 0x1E00 … etc.)
	KbAltA KeyCode = 0x1E00
	KbAltB KeyCode = 0x3000
	KbAltC KeyCode = 0x2E00
	KbAltD KeyCode = 0x2000
	KbAltE KeyCode = 0x1200
	KbAltF KeyCode = 0x2100
	KbAltG KeyCode = 0x2200
	KbAltH KeyCode = 0x2300
	KbAltI KeyCode = 0x1700
	KbAltJ KeyCode = 0x2400
	KbAltK KeyCode = 0x2500
	KbAltL KeyCode = 0x2600
	KbAltM KeyCode = 0x3200
	KbAltN KeyCode = 0x3100
	KbAltO KeyCode = 0x1800
	KbAltP KeyCode = 0x1900
	KbAltQ KeyCode = 0x1000
	KbAltR KeyCode = 0x1300
	KbAltS KeyCode = 0x1F00
	KbAltT KeyCode = 0x1400
	KbAltU KeyCode = 0x1600
	KbAltV KeyCode = 0x2F00
	KbAltW KeyCode = 0x1100
	KbAltX KeyCode = 0x2D00
	KbAltY KeyCode = 0x1500
	KbAltZ KeyCode = 0x2C00
	KbAltF1 KeyCode = 0x6800
	KbAltF2 KeyCode = 0x6900
	KbAltF3 KeyCode = 0x6A00
	KbAltF4 KeyCode = 0x6B00
)

// IsAlt reports whether k is an Alt+letter combination.
func (k KeyCode) IsAlt() bool { return k != 0 && k&0xFF == 0 }

// AltLetter returns the letter for an Alt+letter key code (lower-case).
// Returns 0 if k is not an Alt+letter key.
func (k KeyCode) AltLetter() rune {
	// Map high byte back to letter via a reverse scan.
	altMap := map[KeyCode]rune{
		KbAltA: 'a', KbAltB: 'b', KbAltC: 'c', KbAltD: 'd',
		KbAltE: 'e', KbAltF: 'f', KbAltG: 'g', KbAltH: 'h',
		KbAltI: 'i', KbAltJ: 'j', KbAltK: 'k', KbAltL: 'l',
		KbAltM: 'm', KbAltN: 'n', KbAltO: 'o', KbAltP: 'p',
		KbAltQ: 'q', KbAltR: 'r', KbAltS: 's', KbAltT: 't',
		KbAltU: 'u', KbAltV: 'v', KbAltW: 'w', KbAltX: 'x',
		KbAltY: 'y', KbAltZ: 'z',
	}
	return altMap[k]
}

// Event carries a single UI event.
type Event struct {
	Type    EventType
	Key     KeyCode
	Ch      rune      // printable character for keyboard events
	MouseX  int
	MouseY  int
	Cmd     CommandId
	Handled bool
}

// KeyEvent creates a keyboard Event.
func KeyEvent(key KeyCode, ch rune) Event {
	return Event{Type: EvKeyboard, Key: key, Ch: ch}
}

// MouseDownEvent creates a mouse button press event.
func MouseDownEvent(x, y int) Event {
	return Event{Type: EvMouseDown, MouseX: x, MouseY: y}
}

// MouseUpEvent creates a mouse button release event.
func MouseUpEvent(x, y int) Event {
	return Event{Type: EvMouseUp, MouseX: x, MouseY: y}
}

// MouseMoveEvent creates a mouse move event.
func MouseMoveEvent(x, y int) Event {
	return Event{Type: EvMouseMove, MouseX: x, MouseY: y}
}

// CommandEvent creates a command dispatch event.
func CommandEvent(cmd CommandId) Event {
	return Event{Type: EvCommand, Cmd: cmd}
}

// BroadcastEvent creates a broadcast event (delivered to all views).
func BroadcastEvent(cmd CommandId) Event {
	return Event{Type: EvBroadcast, Cmd: cmd}
}

// -----------------------------------------------------------------------------
// VT100 / ANSI escape sequence parser
// Used by both the ANSI backend (Linux) and the RP2040 backend (USB CDC input).
// -----------------------------------------------------------------------------

// VT100Parser is a stateful parser for terminal escape sequences.
// Feed raw bytes from stdin via Feed(); it returns complete Events.
type VT100Parser struct {
	buf [32]byte
	n   int
}

// Feed processes a raw byte from the terminal and returns an Event.
// Returns EvNothing if more bytes are needed to complete the sequence.
func (p *VT100Parser) Feed(b byte) Event {
	if p.n == 0 {
		if b == 0x1B {
			p.buf[0] = b
			p.n = 1
			return Event{Type: EvNothing}
		}
		return p.singleByte(b)
	}

	p.buf[p.n] = b
	p.n++

	if p.n >= 32 {
		// Buffer overflow — reset and swallow.
		p.n = 0
		return Event{Type: EvNothing}
	}

	// ESC alone (bare escape, no follow-on).
	if p.n == 1 {
		// Already handled above.
		return Event{Type: EvNothing}
	}

	// ESC [ …  (CSI sequences) or ESC O … (SS3)
	switch p.buf[1] {
	case '[':
		return p.parseCsi()
	case 'O':
		return p.parseSS3()
	default:
		// ESC + single char — treat as Alt+char.
		ch := rune(p.buf[1])
		p.n = 0
		return altChar(ch)
	}
}

// Flush should be called when no more bytes are available to drain any pending
// lone ESC that was waiting for a follow-on byte (treat as bare ESC).
func (p *VT100Parser) Flush() Event {
	if p.n == 1 && p.buf[0] == 0x1B {
		p.n = 0
		return KeyEvent(KbEsc, 0)
	}
	p.n = 0
	return Event{Type: EvNothing}
}

func (p *VT100Parser) singleByte(b byte) Event {
	switch b {
	case 0x0D:
		return KeyEvent(KbEnter, 0)
	case 0x1B:
		return KeyEvent(KbEsc, 0)
	case 0x08, 0x7F:
		return KeyEvent(KbBackSpace, 0)
	case 0x09:
		return KeyEvent(KbTab, 0)
	default:
		if b >= 32 {
			return KeyEvent(0, rune(b))
		}
		// Ctrl+letter (^A = 0x01, ^Z = 0x1A).
		return KeyEvent(KbNoKey, rune(b))
	}
}

// parseCsi tries to parse a complete CSI (ESC [) sequence.
// Returns EvNothing if the sequence is not yet complete.
func (p *VT100Parser) parseCsi() Event {
	// We need at least ESC [ <final> where final is 0x40-0x7E.
	if p.n < 3 {
		return Event{Type: EvNothing}
	}
	final := p.buf[p.n-1]
	if final < 0x40 || final > 0x7E {
		return Event{Type: EvNothing} // still accumulating
	}

	// Parse the numeric parameter (simple: single number).
	param := 0
	for i := 2; i < p.n-1; i++ {
		if p.buf[i] >= '0' && p.buf[i] <= '9' {
			param = param*10 + int(p.buf[i]-'0')
		}
	}

	seq := string(p.buf[2:p.n])
	p.n = 0

	// Mouse report: ESC [ M <b> <x> <y>
	if p.buf[2] == 'M' && p.n == 0 {
		// handled below via raw bytes — but this path is for the final-byte variant
	}

	switch final {
	case 'A':
		return KeyEvent(KbUp, 0)
	case 'B':
		return KeyEvent(KbDown, 0)
	case 'C':
		return KeyEvent(KbRight, 0)
	case 'D':
		return KeyEvent(KbLeft, 0)
	case 'H':
		return KeyEvent(KbHome, 0)
	case 'F':
		return KeyEvent(KbEnd, 0)
	case 'Z':
		return KeyEvent(KbShiftTab, 0)
	case '~':
		switch param {
		case 1:
			return KeyEvent(KbHome, 0)
		case 2:
			return KeyEvent(KbIns, 0)
		case 3:
			return KeyEvent(KbDel, 0)
		case 4:
			return KeyEvent(KbEnd, 0)
		case 5:
			return KeyEvent(KbPgUp, 0)
		case 6:
			return KeyEvent(KbPgDn, 0)
		case 11:
			return KeyEvent(KbF1, 0)
		case 12:
			return KeyEvent(KbF2, 0)
		case 13:
			return KeyEvent(KbF3, 0)
		case 14:
			return KeyEvent(KbF4, 0)
		case 15:
			return KeyEvent(KbF5, 0)
		case 17:
			return KeyEvent(KbF6, 0)
		case 18:
			return KeyEvent(KbF7, 0)
		case 19:
			return KeyEvent(KbF8, 0)
		case 20:
			return KeyEvent(KbF9, 0)
		case 21:
			return KeyEvent(KbF10, 0)
		case 23:
			return KeyEvent(KbF11, 0)
		case 24:
			return KeyEvent(KbF12, 0)
		}
	case 'M':
		// X10 mouse report: ESC [ M ... — handled via FeedMouse
		_ = seq
	}
	return Event{Type: EvNothing}
}

// parseSS3 parses ESC O sequences (used for F1-F4 on some terminals).
func (p *VT100Parser) parseSS3() Event {
	if p.n < 3 {
		return Event{Type: EvNothing}
	}
	ch := p.buf[2]
	p.n = 0
	switch ch {
	case 'P':
		return KeyEvent(KbF1, 0)
	case 'Q':
		return KeyEvent(KbF2, 0)
	case 'R':
		return KeyEvent(KbF3, 0)
	case 'S':
		return KeyEvent(KbF4, 0)
	case 'H':
		return KeyEvent(KbHome, 0)
	case 'F':
		return KeyEvent(KbEnd, 0)
	}
	return Event{Type: EvNothing}
}

// FeedMouseReport processes a 3-byte X10 mouse report payload (the bytes after
// ESC [ M). Button byte b: b&3==0 press, b&3==3 release; x,y are 1-based
// column/row with +32 offset.
func FeedMouseReport(b, x, y byte) Event {
	col := int(x) - 33
	row := int(y) - 33
	btn := b & 3
	switch btn {
	case 3:
		return MouseUpEvent(col, row)
	default:
		return MouseDownEvent(col, row)
	}
}

func altChar(ch rune) Event {
	// Map rune to the corresponding Alt+letter KeyCode.
	altMap := map[rune]KeyCode{
		'a': KbAltA, 'b': KbAltB, 'c': KbAltC, 'd': KbAltD,
		'e': KbAltE, 'f': KbAltF, 'g': KbAltG, 'h': KbAltH,
		'i': KbAltI, 'j': KbAltJ, 'k': KbAltK, 'l': KbAltL,
		'm': KbAltM, 'n': KbAltN, 'o': KbAltO, 'p': KbAltP,
		'q': KbAltQ, 'r': KbAltR, 's': KbAltS, 't': KbAltT,
		'u': KbAltU, 'v': KbAltV, 'w': KbAltW, 'x': KbAltX,
		'y': KbAltY, 'z': KbAltZ,
		'A': KbAltA, 'B': KbAltB, 'C': KbAltC, 'D': KbAltD,
		'E': KbAltE, 'F': KbAltF, 'G': KbAltG, 'H': KbAltH,
		'I': KbAltI, 'J': KbAltJ, 'K': KbAltK, 'L': KbAltL,
		'M': KbAltM, 'N': KbAltN, 'O': KbAltO, 'P': KbAltP,
		'Q': KbAltQ, 'R': KbAltR, 'S': KbAltS, 'T': KbAltT,
		'U': KbAltU, 'V': KbAltV, 'W': KbAltW, 'X': KbAltX,
		'Y': KbAltY, 'Z': KbAltZ,
		'1': KbAltF1, '2': KbAltF2, '3': KbAltF3, '4': KbAltF4,
	}
	if kc, ok := altMap[ch]; ok {
		return KeyEvent(kc, 0)
	}
	return KeyEvent(KbEsc, 0) // fallback: bare ESC
}
