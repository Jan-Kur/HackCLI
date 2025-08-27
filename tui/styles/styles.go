package styles

import (
	"strings"

	lg "github.com/charmbracelet/lipgloss"
)

var (
	Error       = lg.Color("#c92323")
	Green       = lg.Color("#18c39b")
	GreenDim    = lg.Color("#164137")
	StrGreen    = "#18c39b"
	StrGreenDim = "#164137"
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
