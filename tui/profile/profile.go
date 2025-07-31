package profile

import (
	"fmt"
	"os"

	sl "github.com/Jan-Kur/HackCLI/slack"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
)

var (
	docStyle      = lg.NewStyle().Margin(1, 2)
	selectedStyle = lg.NewStyle().Foreground(lg.Color("#f16de6ff"))
	normalStyle   = lg.NewStyle().Foreground(lg.Color("#7c7c7cff"))
)

type item struct {
	title string
	input textinput.Model
}

func (i item) Title() string { return i.title }

func (i item) Description() string {
	if i.input.Focused() {
		return selectedStyle.Render(i.input.View())
	}
	return normalStyle.Render(i.input.Value())
}

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
		case "enter":
			client, err := sl.GetClient()
			if err != nil {
				fmt.Printf("Error creating a slack client: %v\n", err)
				os.Exit(1)
			}

			//authResp, err := client.AuthTest()
			//if err != nil {
			//	fmt.Printf("Error authorizing with slack: %v\n", err)
			//	os.Exit(1)
			//}

			list := m.list.Items()
			client.SetUserCustomStatus(list[2].(item).input.Value(), list[3].(item).input.Value(), 0)
			client.SetUserRealName(list[0].(item).input.Value())
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

			oldItem.input.Cursor.SetMode(cursor.CursorHide)
			oldItem.input.Blur()
			oldItem.input.TextStyle = normalStyle
			oldItem.input.Cursor.Style = normalStyle

			newItem.input.Cursor.SetMode(cursor.CursorBlink)
			newItem.input.Focus()
			newItem.input.TextStyle = selectedStyle
			newItem.input.Cursor.Style = selectedStyle

			items[m.selectedIndex] = oldItem
			items[newIndex] = newItem

			m.list.SetItems(items)
			m.selectedIndex = newIndex
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
	fullName.SetValue(user.Profile.RealName)
	fullName.Focus()
	fullName.Width = 100
	fullName.Cursor.SetMode(cursor.CursorBlink)
	fullName.TextStyle = selectedStyle
	fullName.Cursor.Style = selectedStyle

	displayName := textinput.New()
	displayName.SetValue(user.Profile.DisplayName)
	displayName.Blur()
	displayName.Width = 100
	displayName.Cursor.SetMode(cursor.CursorHide)
	displayName.TextStyle = normalStyle
	displayName.Cursor.Style = normalStyle

	status := textinput.New()
	status.SetValue(user.Profile.StatusText)
	status.Blur()
	status.Width = 100
	status.Cursor.SetMode(cursor.CursorHide)
	status.TextStyle = normalStyle
	status.Cursor.Style = normalStyle

	statusEmoji := textinput.New()
	statusEmoji.SetValue(user.Profile.StatusEmoji)
	statusEmoji.Blur()
	statusEmoji.Width = 100
	statusEmoji.Cursor.SetMode(cursor.CursorHide)
	statusEmoji.TextStyle = normalStyle
	statusEmoji.Cursor.Style = normalStyle

	favActivities := textinput.New()
	favActivities.SetValue(favActivitiesValue)
	favActivities.Blur()
	favActivities.Width = 100
	favActivities.Cursor.SetMode(cursor.CursorHide)
	favActivities.TextStyle = normalStyle
	favActivities.Cursor.Style = normalStyle

	items := []list.Item{
		item{title: "Full name", input: fullName},
		item{title: "Display name", input: displayName},
		item{title: "Status", input: status},
		item{title: "Status emoji", input: statusEmoji},
		item{title: "Fav activities", input: favActivities},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)

	l.KeyMap.NextPage = key.NewBinding()
	l.KeyMap.PrevPage = key.NewBinding()
	l.KeyMap.GoToEnd = key.NewBinding()
	l.KeyMap.ShowFullHelp = key.NewBinding()
	l.KeyMap.CloseFullHelp = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.KeyMap.GoToStart = key.NewBinding()

	l.KeyMap.Filter = key.NewBinding(
		key.WithKeys(""),
		key.WithHelp("enter", "apply changes"),
	)
	l.KeyMap.CursorUp = key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	)
	l.KeyMap.CursorDown = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	)
	l.KeyMap.Quit = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)

	m := model{list: l, state: "initial", selectedIndex: 0}
	m.list.Title = "Profile info"

	return m
}
