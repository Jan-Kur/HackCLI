package channel

import (
	tea "github.com/charmbracelet/bubbletea"
)

type sidebarModel struct {
	currentChannel string
}

func initialSidebar(initialChannel string) sidebarModel {

	return sidebarModel{
		currentChannel: initialChannel,
	}
}

func (m sidebarModel) Init() tea.Cmd {
	return nil
}

func (m sidebarModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	panic("unimplemented")
}

func (m sidebarModel) View() string {
	panic("unimplemented")
}
