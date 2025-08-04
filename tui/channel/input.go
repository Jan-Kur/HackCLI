package channel

import (
	tea "github.com/charmbracelet/bubbletea"
)

type inputModel struct {
	currentChannel string
}

func initialInput(initialChannel string) inputModel {

	return inputModel{
		currentChannel: initialChannel,
	}
}

func (m inputModel) Init() tea.Cmd {
	return nil
}

func (m inputModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	panic("unimplemented")
}

func (m inputModel) View() string {
	panic("unimplemented")
}
