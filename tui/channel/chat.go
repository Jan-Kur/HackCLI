package channel

import (
	tea "github.com/charmbracelet/bubbletea"
)

type chatModel struct {
	currentChannel string
}

func initialChat(initialChannel string) chatModel {

	return chatModel{
		currentChannel: initialChannel,
	}
}

func (m chatModel) Init() tea.Cmd {
	return nil
}

func (m chatModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	panic("unimplemented")
}

func (m chatModel) View() string {
	panic("unimplemented")
}
