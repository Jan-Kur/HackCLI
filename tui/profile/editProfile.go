package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Jan-Kur/HackCLI/api"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/slack-go/slack"
)

var (
	docStyle      = lg.NewStyle().Margin(1, 2)
	selectedStyle = lg.NewStyle().Foreground(lg.Color("#f16de6ff"))
	normalStyle   = lg.NewStyle().Foreground(lg.Color("#7c7c7cff"))
)

type endMsg struct {
}

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
	errorMessage  error
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case endMsg:
		return m, tea.Quit
	}

	switch m.state {
	case "initial":
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				list := m.list.Items()
				realName := list[0].(item).input.Value()
				displayName := list[1].(item).input.Value()
				statusText := list[2].(item).input.Value()
				statusEmoji := list[3].(item).input.Value()

				cfg, err := api.LoadConfig()
				if err != nil {
					fmt.Printf("Error creating a slack client: %v\n", err)
					os.Exit(1)
				}

				profile, err := json.Marshal(&struct {
					RealName    string `json:"real_name"`
					DisplayName string `json:"display_name"`
					StatusText  string `json:"status_text"`
					StatusEmoji string `json:"status_emoji"`
				}{
					RealName:    realName,
					DisplayName: displayName,
					StatusText:  statusText,
					StatusEmoji: statusEmoji,
				})
				if err != nil {
					m.errorMessage = err
					m.state = "error"
					return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
						return endMsg{}
					})
				}
				values := url.Values{
					"token":   {cfg.Token},
					"profile": {string(profile)},
				}

				resp, err := http.PostForm("https://slack.com/api/users.profile.set", values)
				if err != nil {
					m.errorMessage = err
					m.state = "error"
					return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
						return endMsg{}
					})
				}

				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					m.errorMessage = err
					m.state = "error"
					return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
						return endMsg{}
					})
				}

				var result struct {
					OK    bool   `json:"ok"`
					Error string `json:"error,omitempty"`
				}

				if err := json.Unmarshal(body, &result); err != nil {
					m.errorMessage = err
					m.state = "error"
					return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
						return endMsg{}
					})
				}

				if !result.OK {
					m.errorMessage = fmt.Errorf("%v", result.Error)
					m.state = "error"
					return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
						return endMsg{}
					})
				}

				m.state = "end"

				return m, nil
			}
		case tea.WindowSizeMsg:
			h, v := docStyle.GetFrameSize()
			m.list.SetSize(msg.Width-h, msg.Height-v)
		}
	case "end":
		return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
			return endMsg{}
		})
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
	switch m.state {
	case "initial":
		return docStyle.Render(m.list.View())
	case "error":
		s := lg.NewStyle().Bold(true).Foreground(lg.Color("rgba(255, 73, 124, 1)")).Render("❌ ERROR ❌\n\n")
		s += m.errorMessage.Error()
		return s
	case "end":
		s := lg.NewStyle().Bold(true).Foreground(lg.Color("#77CCAA")).Render("✅ SUCCESS ✅")
		s += "\n\nYour profile has been updated"
		return s
	}
	return ""
}

func Start() model {
	cfg, err := api.LoadConfig()
	if err != nil {
		fmt.Printf("Error getting token: %v\n", err)
		os.Exit(1)
	}

	api := slack.New(cfg.Token)

	authResp, err := api.AuthTest()
	if err != nil {
		fmt.Printf("Error authorizing with slack: %v\n", err)
		os.Exit(1)
	}

	user, err := api.GetUserProfile(&slack.GetUserProfileParameters{
		UserID:        authResp.UserID,
		IncludeLabels: true,
	})
	if err != nil {
		fmt.Printf("Error getting user info %v\n", err)
		os.Exit(1)
	}

	fullName := textinput.New()
	fullName.SetValue(user.RealName)
	fullName.Focus()
	fullName.Width = 100
	fullName.Cursor.SetMode(cursor.CursorBlink)
	fullName.TextStyle = selectedStyle
	fullName.Cursor.Style = selectedStyle

	displayName := textinput.New()
	displayName.SetValue(user.DisplayName)
	displayName.Blur()
	displayName.Width = 100
	displayName.Cursor.SetMode(cursor.CursorHide)
	displayName.TextStyle = normalStyle
	displayName.Cursor.Style = normalStyle

	status := textinput.New()
	status.SetValue(user.StatusText)
	status.Blur()
	status.Width = 100
	status.Cursor.SetMode(cursor.CursorHide)
	status.TextStyle = normalStyle
	status.Cursor.Style = normalStyle

	statusEmoji := textinput.New()
	statusEmoji.SetValue(user.StatusEmoji)
	statusEmoji.Blur()
	statusEmoji.Width = 100
	statusEmoji.Cursor.SetMode(cursor.CursorHide)
	statusEmoji.TextStyle = normalStyle
	statusEmoji.Cursor.Style = normalStyle

	items := []list.Item{
		item{title: "Full name", input: fullName},
		item{title: "Display name", input: displayName},
		item{title: "Status", input: status},
		item{title: "Status emoji", input: statusEmoji},
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
