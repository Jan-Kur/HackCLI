package styles

import (
	lg "github.com/charmbracelet/lipgloss"
)

var (
	Faint = lg.NewStyle().Faint(true)
	Bold  = lg.NewStyle().Bold(true)
	Error = lg.NewStyle().Foreground(lg.Color("#c92323"))
	Green = lg.NewStyle().Foreground(lg.Color("#18c39b"))
)
