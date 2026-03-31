package core

// Attr encodes a foreground/background color pair in a single byte.
// Bits 0-3 are the foreground color index; bits 4-7 are the background index.
// Colors are drawn from the 16-color Borland palette.
type Attr uint8

// MakeAttr constructs an Attr from fg and bg color indices (0-15).
func MakeAttr(fg, bg uint8) Attr { return Attr(fg&0x0F | (bg&0x0F)<<4) }

// Fg returns the foreground color index (0-15).
func (a Attr) Fg() uint8 { return uint8(a) & 0x0F }

// Bg returns the background color index (0-15).
func (a Attr) Bg() uint8 { return uint8(a) >> 4 }

// Swap exchanges foreground and background colors.
func (a Attr) Swap() Attr { return MakeAttr(a.Bg(), a.Fg()) }

// 16-color Borland palette color indices.
const (
	ColorBlack        uint8 = 0
	ColorBlue         uint8 = 1
	ColorGreen        uint8 = 2
	ColorCyan         uint8 = 3
	ColorRed          uint8 = 4
	ColorMagenta      uint8 = 5
	ColorBrown        uint8 = 6
	ColorLightGray    uint8 = 7
	ColorDarkGray     uint8 = 8
	ColorLightBlue    uint8 = 9
	ColorLightGreen   uint8 = 10
	ColorLightCyan    uint8 = 11
	ColorLightRed     uint8 = 12
	ColorLightMagenta uint8 = 13
	ColorYellow       uint8 = 14
	ColorWhite        uint8 = 15
)

// ANSIFg maps a Borland color index to its ANSI foreground escape code number.
var ANSIFg = [16]int{30, 34, 32, 36, 31, 35, 33, 37, 90, 94, 92, 96, 91, 95, 93, 97}

// ANSIBg maps a Borland color index to its ANSI background escape code number.
var ANSIBg = [16]int{40, 44, 42, 46, 41, 45, 43, 47, 100, 104, 102, 106, 101, 105, 103, 107}

// Common attribute presets.
var (
	// Desktop background.
	AttrDesktop = MakeAttr(ColorLightGray, ColorBlue)
	// Normal window content.
	AttrNormal = MakeAttr(ColorBlack, ColorLightGray)
	// Active window frame.
	AttrActive = MakeAttr(ColorBlack, ColorLightGray)
	// Inactive window frame.
	AttrInactive = MakeAttr(ColorDarkGray, ColorLightGray)
	// Dialog background.
	AttrDialog = MakeAttr(ColorBlack, ColorLightGray)
	// Button normal.
	AttrButton = MakeAttr(ColorBlack, ColorLightGreen)
	// Button focused.
	AttrButtonFocused = MakeAttr(ColorBlack, ColorGreen)
	// Input line normal.
	AttrInput = MakeAttr(ColorBlack, ColorCyan)
	// Input line focused.
	AttrInputFocused = MakeAttr(ColorBlack, ColorLightCyan)
	// Menu bar normal.
	AttrMenuBar = MakeAttr(ColorBlack, ColorLightGray)
	// Menu bar selected.
	AttrMenuBarSelected = MakeAttr(ColorBlack, ColorGreen)
	// Menu box normal.
	AttrMenuBox = MakeAttr(ColorBlack, ColorLightGray)
	// Menu box selected.
	AttrMenuBoxSelected = MakeAttr(ColorBlack, ColorGreen)
	// Status line.
	AttrStatusLine = MakeAttr(ColorBlack, ColorLightGray)
	// Status line hotkey.
	AttrStatusKey = MakeAttr(ColorBlack, ColorLightGreen)
	// Editor body.
	AttrEditor = MakeAttr(ColorYellow, ColorBlue)
	// Editor selected text.
	AttrEditorSelected = MakeAttr(ColorBlack, ColorCyan)
	// ScrollBar.
	AttrScrollBar = MakeAttr(ColorBlack, ColorLightGray)
	// List box normal.
	AttrListBox = MakeAttr(ColorBlack, ColorLightGray)
	// List box selected.
	AttrListBoxSelected = MakeAttr(ColorBlack, ColorGreen)
	// Shadow.
	AttrShadow = MakeAttr(ColorDarkGray, ColorBlack)
)
