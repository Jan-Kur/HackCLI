package profile

import (
	"fmt"
	"os"

	sl "github.com/Jan-Kur/HackCLI/slack"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
)

var docStyle = lg.NewStyle().Margin(1, 2)

type item struct {
	title string
	input textinput.Model
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.input.View() }
func (i item) FilterValue() string { return i.title }

type model struct {
	list          list.Model
	state         string
	selectedIndex int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	newIndex := m.list.Index()
	if selected, ok := m.list.SelectedItem().(item); ok {
		updatedInput, _ := selected.input.Update(msg)

		selected.input = updatedInput

		items := m.list.Items()
		items[m.list.Index()] = selected
		m.list.SetItems(items)

		if m.selectedIndex != newIndex {
			items := m.list.Items()

			oldItem := items[m.selectedIndex].(item)
			newItem := items[newIndex].(item)

			oldItem.input.Blur()
			newItem.input.Focus()

			items[m.selectedIndex] = oldItem
			items[newIndex] = newItem

			m.list.SetItems(items)
		}
	}
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func Start() model {
	client, err := sl.GetClient()
	if err != nil {
		fmt.Printf("Error creating a slack client: %v\n", err)
		os.Exit(1)
	}

	authResp, err := client.AuthTest()
	if err != nil {
		fmt.Printf("Error authorizing with slack: %v\n", err)
		os.Exit(1)
	}

	user, err := client.GetUserInfo(authResp.UserID)
	if err != nil {
		fmt.Printf("Error getting user information: %v\n", err)
		os.Exit(1)
	}

	var favActivitiesValue string
	for _, field := range user.Profile.FieldsMap() {
		if field.Label == "Fav Activities" {
			favActivitiesValue = field.Value
			break
		}
	}

	fullName := textinput.New()
	fullName.Placeholder = user.Profile.RealName
	fullName.Focus()
	fullName.Width = 100

	displayName := textinput.New()
	displayName.Placeholder = user.Profile.DisplayName
	displayName.Blur()
	displayName.Width = 100

	status := textinput.New()
	status.Placeholder = user.Profile.StatusText
	status.Blur()
	status.Width = 100

	statusEmoji := textinput.New()
	statusEmoji.Placeholder = user.Profile.StatusEmoji
	statusEmoji.Blur()
	statusEmoji.Width = 100

	favActivities := textinput.New()
	favActivities.Placeholder = favActivitiesValue
	favActivities.Blur()
	favActivities.Width = 100

	items := []list.Item{
		item{title: "Full name", input: fullName},
		item{title: "Display name", input: displayName},
		item{title: "Status", input: status},
		item{title: "Status emoji", input: statusEmoji},
		item{title: "Fav activities", input: favActivities},
	}

	m := model{list: list.New(items, list.NewDefaultDelegate(), 0, 0), state: "initial", selectedIndex: 0}
	m.list.Title = "Profile info"

	return m
}
