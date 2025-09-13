package styles

import (
	lg "github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Background lg.Color
	Text       lg.Color
	Primary    lg.Color
	Secondary  lg.Color
	Border     lg.Color
	Selected   lg.Color
	Subtle     lg.Color
	Muted      lg.Color
}

var Themes = map[string]Theme{
	"Rose Pine": {
		Background: lg.Color("#191724"),
		Text:       lg.Color("#e0def4"),
		Primary:    lg.Color("#31748f"),
		Secondary:  lg.Color("#ebbcba"),
		Border:     lg.Color("#393552"),
		Selected:   lg.Color("#eb6f92"),
		Subtle:     lg.Color("#6e6a86"),
		Muted:      lg.Color("#403d52"),
	},
	"Catppuccin Mocha": {
		Background: lg.Color("#1e1e2e"),
		Text:       lg.Color("#cdd6f4"),
		Primary:    lg.Color("#89b4fa"),
		Secondary:  lg.Color("#f5c2e7"),
		Border:     lg.Color("#3e3f55ff"),
		Selected:   lg.Color("#cba6f7"),
		Subtle:     lg.Color("#6c7086"),
		Muted:      lg.Color("#45475a"),
	},
	"Dracula": {
		Background: lg.Color("#282a36"),
		Text:       lg.Color("#f8f8f2"),
		Primary:    lg.Color("#bd93f9"),
		Secondary:  lg.Color("#ff79c6"),
		Border:     lg.Color("#44475a"),
		Selected:   lg.Color("#50fa7b"),
		Subtle:     lg.Color("#6272a4"),
		Muted:      lg.Color("#44475a"),
	},
}
