package views

import "go-tp/tv/core"

// Label displays a static text string with no interaction.
type Label struct {
	ViewBase
	Text string
	Attr core.Attr
}

// NewLabel creates a Label at the given relative position (W is set to text length).
func NewLabel(text string, attr core.Attr) *Label {
	l := &Label{Text: text, Attr: attr}
	l.bounds = core.Rect{W: len([]rune(text)), H: 1}
	return l
}

func (l *Label) Draw(buf *core.DrawBuffer) {
	buf.MoveStr(0, 0, l.Text, l.Attr)
}
