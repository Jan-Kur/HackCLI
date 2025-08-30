package styles

import (
	"strings"

	lg "github.com/charmbracelet/lipgloss"
)

var (
	Error    = lg.Color("#c92323")
	Green    = lg.Color("#77c3a1")
	Base     = lg.Color("#191724")
	Overlay  = lg.Color("#26233a")
	Contrast = lg.Color("#524f67")
	Muted    = lg.Color("#6e6a86")
	Subtle   = lg.Color("#b7b3d7ff")
	Text     = lg.Color("#e0def4")
	Pink     = lg.Color("#eb6f92")
	Rose     = lg.Color("#ebbcba")
	Pine     = lg.Color("#31748f")
	Gold     = lg.Color("#f6c177")
)

type BoxWithLabel struct {
	BoxStyle   lg.Style
	LabelStyle lg.Style
}

func NewDefaultBoxWithLabel() BoxWithLabel {
	return BoxWithLabel{
		BoxStyle: lg.NewStyle().
			Border(lg.RoundedBorder()).
			Faint(true).
			Padding(1),
		LabelStyle: lg.NewStyle().
			PaddingTop(0).
			PaddingBottom(0).
			PaddingLeft(1).
			PaddingRight(1).
			Faint(true),
	}
}

func (b BoxWithLabel) Render(label, content string, width int) string {
	var (
		border          lg.Border = b.BoxStyle.GetBorderStyle()
		topBorderStyler           = lg.NewStyle().Foreground(b.BoxStyle.GetBorderTopForeground()).Render
		topLeft         string    = topBorderStyler(border.TopLeft)
		topRight        string    = topBorderStyler(border.TopRight)
		renderedLabel   string    = b.LabelStyle.Render(label)
	)

	borderWidth := b.BoxStyle.GetHorizontalBorderSize()

	cellsShort := max(0, width+borderWidth-lg.Width(topLeft+topRight+renderedLabel)-1)

	gap := strings.Repeat(border.Top, cellsShort)

	top := topLeft + topBorderStyler(border.Top) + renderedLabel + topBorderStyler(gap) + topRight

	bottom := b.BoxStyle.BorderTop(false).Width(width).Render(content)

	return top + "\n" + bottom
}
