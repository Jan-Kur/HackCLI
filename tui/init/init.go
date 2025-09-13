package init

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
)

type state int

const (
	initial state = iota
	themeChange
	end
)

var (
	successStyle      lg.Style
	inputStyle        lg.Style
	titleStyle        lg.Style
	headerStyle       lg.Style
	optionalStyle     lg.Style
	itemStyle         lg.Style
	selectedItemStyle lg.Style
	errorStyle        lg.Style
)

type endMsg struct{}

type model struct {
	state    state
	input    textinput.Model
	list     list.Model
	errorMsg string
	cfg      core.Config
	height   int
	width    int
}

type themeItem struct {
	Name   string
	Colors styles.Theme
}

func Start() model {
	cfg, err := api.LoadConfig()
	var theme styles.Theme
	if err == nil && cfg.Theme != "" {
		if loadedTheme, exists := styles.Themes[cfg.Theme]; exists {
			theme = loadedTheme
		} else {
			theme = styles.Themes["Rose Pine"]
		}
	} else {
		theme = styles.Themes["Rose Pine"]
	}

	initializeStyles(theme)

	i := textinput.New()
	i.Focus()
	i.Width = 100
	i.Cursor.SetMode(cursor.CursorBlink)
	i.TextStyle = lg.NewStyle().Foreground(theme.Text)
	i.Cursor.Style = lg.NewStyle().Foreground(theme.Text)

	var items []list.Item

	items = []list.Item{
		themeItem{"Rose Pine", styles.Themes["Rose Pine"]},
		themeItem{"Catppuccin Mocha", styles.Themes["Catppuccin Mocha"]},
		themeItem{"Dracula", styles.Themes["Dracula"]},
	}

	l := list.New(items, itemDelegate{}, 0, 0)
	l.Title = headerStyle.Render("Choose a theme for HackCLI") +
		optionalStyle.Render(" (you can change this anytime in the config)")
	l.Styles.Title = titleStyle
	l.Help.Styles.ShortKey = optionalStyle
	l.Help.Styles.ShortDesc = optionalStyle
	l.Help.Styles.FullKey = optionalStyle
	l.Help.Styles.FullDesc = optionalStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowPagination(false)
	l.SetShowFilter(false)
	l.KeyMap = list.KeyMap{
		Quit:       key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc/ctrl+c", "quit")),
		CursorUp:   key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		CursorDown: key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
	}

	return model{
		state:  0,
		input:  i,
		list:   l,
		height: 0,
		width:  0,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case endMsg:
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.input.Width = msg.Width
		m.list.SetHeight(msg.Height)
		m.list.SetWidth(msg.Width)

	case tea.KeyMsg:
		switch m.state {
		case initial:
			switch msg.String() {
			case "ctrl+c", "esc":
				return m, tea.Quit
			case "enter":
				m.cfg.Cookie = strings.TrimSpace(m.input.Value())

				token, err := getToken(m.cfg.Cookie)
				if err != nil {
					m.errorMsg = "Invalid slack cookie"
				} else {
					m.cfg.Token = token
					m.state = themeChange
					m.errorMsg = ""
				}
				return m, nil
			}
			m.input, cmd = m.input.Update(msg)
			return m, tea.Batch(cmd, textinput.Blink)
		case themeChange:
			switch msg.String() {
			case "ctrl+c", "esc":
				return m, tea.Quit
			case "enter":
				i, ok := m.list.SelectedItem().(themeItem)
				if ok {
					m.cfg.Theme = string(i.Name)
				}
				api.SaveConfig(m.cfg)
				m.state = end
				m.errorMsg = ""
				return m, nil
			}
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		case end:
			switch msg.String() {
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
			return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
				return endMsg{}
			})
		}
	}

	return m, nil
}

func (m model) View() string {
	var s string

	switch m.state {
	case initial:
		s = headerStyle.Render("  Input your slack d cookie")
		s += "\n\n"
		s += inputStyle.Render(m.input.View())
	case themeChange:
		s = m.list.View()
	case end:
		s = successStyle.Render("You can now use HackCLI!")
		s += "\n\n"
		s += successStyle.Render("To update your settings just edit the config file")
	}

	if m.errorMsg != "" {
		s += "\n\n"
		s += errorStyle.Render(m.errorMsg)
	}

	return s
}

func (i themeItem) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 1 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(themeItem)
	if !ok {
		return
	}

	str := fmt.Sprint(i.Name + " " +
		lg.NewStyle().Background(i.Colors.Background).Render("  ") + " " +
		lg.NewStyle().Background(i.Colors.Text).Render("  ") + " " +
		lg.NewStyle().Background(i.Colors.Primary).Render("  ") + " " +
		lg.NewStyle().Background(i.Colors.Secondary).Render("  ") + " " +
		lg.NewStyle().Background(i.Colors.Border).Render("  ") + " " +
		lg.NewStyle().Background(i.Colors.Selected).Render("  ") + " " +
		lg.NewStyle().Background(i.Colors.Subtle).Render("  ") + " " +
		lg.NewStyle().Background(i.Colors.Muted).Render("  "))

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}
	fmt.Fprint(w, fn(str))
}

func getToken(cookie string) (string, error) {
	client := http.Client{}
	req, _ := http.NewRequest("GET", "https://hackclub.slack.com", nil)
	req.Header.Add("Cookie", fmt.Sprintf("d=%v", cookie))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	regex := regexp.MustCompile(`xox[a-zA-Z]-[a-zA-Z0-9-]+`)
	token := regex.FindString(string(body))
	if token == "" {
		return "", fmt.Errorf("Didn't find token in the response")
	}

	return token, nil
}

func initializeStyles(theme styles.Theme) {
	successStyle = titleStyle.Foreground(styles.Green).Bold(true)
	inputStyle = lg.NewStyle().MarginLeft(2).Foreground(theme.Primary)
	titleStyle = lg.NewStyle().MarginLeft(2)
	headerStyle = lg.NewStyle().Bold(true).Foreground(theme.Primary)
	optionalStyle = lg.NewStyle().Foreground(theme.Muted)
	itemStyle = lg.NewStyle().PaddingLeft(4).Foreground(theme.Text)
	selectedItemStyle = lg.NewStyle().PaddingLeft(2).Foreground(theme.Primary)
	errorStyle = lg.NewStyle().PaddingLeft(2).Foreground(styles.Pink)
}
