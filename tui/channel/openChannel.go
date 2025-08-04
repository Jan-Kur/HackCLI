package channel

import (
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	sidebar        sidebarModel
	chat           chatModel
	input          inputModel
	currentChannel string
}

func Start(initialChannel string) model {

	return model{
		sidebar:        initialSidebar(initialChannel),
		chat:           initialChat(initialChannel),
		input:          initialInput(initialChannel),
		currentChannel: initialChannel,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(tea.Msg) (tea.Model, tea.Cmd) {
	panic("unimplemented")
}

func (m model) View() string {
	panic("unimplemented")
}
