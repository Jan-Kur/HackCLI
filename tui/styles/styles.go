package styles

import (
	"strings"

	lg "github.com/charmbracelet/lipgloss"
)

var (
	Green = lg.Color("#77c3a1")
	Gray  = lg.Color("#6e6a86")
	Pink  = lg.Color("#eb6f92")
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
		topBorderStyler           = lg.NewStyle().Foreground(b.BoxStyle.GetBorderTopForeground()).
				Background(b.BoxStyle.GetBorderTopBackground()).Render
		topLeft       string = topBorderStyler(border.TopLeft)
		topRight      string = topBorderStyler(border.TopRight)
		renderedLabel string = b.LabelStyle.Render(label)
	)

	borderWidth := b.BoxStyle.GetHorizontalBorderSize()

	cellsShort := max(0, width+borderWidth-lg.Width(topLeft+topRight+renderedLabel)-1)

	gap := strings.Repeat(border.Top, cellsShort)

	top := topLeft + topBorderStyler(border.Top) + renderedLabel + topBorderStyler(gap) + topRight

	bottom := b.BoxStyle.BorderTop(false).Width(width).Render(content)

	return top + "\n" + bottom
}
