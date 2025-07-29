package tui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
	"github.com/slack-go/slack"
)

type model struct {
	state        string
	textInput    textinput.Model
	spinner      spinner.Model
	errorMessage string
	loggedIn     bool
}

type endMsg struct{}

type Config struct {
	Token string `json:"token"`
}

func InitialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Input the slack token here..."
	ti.CharLimit = 100
	ti.Width = 100

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot

	var loggedIn bool

	baseDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	folderDir := filepath.Join(baseDir, "HackCLI")
	configLocation := filepath.Join(folderDir, "config.json")
	_, err = os.Stat(configLocation)
	if err == nil {
		data, _ := os.ReadFile(configLocation)

		var config Config
		json.Unmarshal(data, &config)

		client := slack.New(config.Token)
		_, err := client.AuthTest()
		loggedIn = err == nil

	} else if os.IsNotExist(err) {
		loggedIn = false
	} else {
		panic(err)
	}

	return model{
		textInput:    ti,
		spinner:      sp,
		state:        "initial",
		errorMessage: "",
		loggedIn:     loggedIn,
	}
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
	case endMsg:
		return m, tea.Quit
	}

	switch m.state {
	case "initial":
		return m.handleInitial(msg)
	case "inputToken":
		return m.handleInputToken(msg)
	case "end":
		return m.handleEnd()
	}
	return m, nil
}

func (m model) handleInitial(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.loggedIn {
	case true:
		return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
			return endMsg{}
		})
	case false:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				if err := openBrowser(); err != nil {
					m.errorMessage = "Couldn't open the browser\n\n"
					return m, nil
				}
				m.state = "inputToken"
				m.textInput.Focus()
				m.errorMessage = ""
				return m, tea.Batch(textinput.Blink, m.spinner.Tick)
			}
		}
	}
	return m, nil
}

func (m model) handleInputToken(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if !strings.HasPrefix(m.textInput.Value(), "xoxp-") || m.textInput.Value() == "" {
				m.errorMessage = "Please enter a valid slack token\n\n"
				return m, nil
			}
			token := m.textInput.Value()
			err := saveTokenToConfig(token)
			if err != nil {
				m.errorMessage = "Couldn't create config file containing the token\n\n"
				return m, nil
			}
			m.state = "end"
			m.errorMessage = ""
			return m, nil
		}
	}
	var textCmd, spinCmd tea.Cmd
	m.textInput, textCmd = m.textInput.Update(msg)
	m.spinner, spinCmd = m.spinner.Update(msg)

	return m, tea.Batch(textCmd, spinCmd)
}

func (m model) handleEnd() (tea.Model, tea.Cmd) {
	return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
		return endMsg{}
	})
}

func (m model) View() string {
	s := "\n"
	switch m.state {
	case "initial":
		if m.loggedIn {
			s += "✨ You are already logged in ✨\n\nGo ahead and use HackCLI 🥳\n\n"
		} else {
			s += lipgloss.NewStyle().Faint(true).Render("Press ") +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#18c39bff")).Bold(true).Render("enter") +
				lipgloss.NewStyle().Faint(true).Render(" to log in with slack\n\n")
		}
	case "inputToken":
		s += fmt.Sprint(m.spinner.View(), " ", "Waiting for authorization\n")
		s += m.textInput.View() + "\n\n"
	case "end":
		s += "✅ SUCCESS ✅\n\nYou are now logged in and can use HackCLI 🥳\n\n"
	}
	s += m.errorMessage
	s += lipgloss.NewStyle().Faint(true).Render("Press ") +
		lipgloss.NewStyle().Bold(true).Render("q") +
		lipgloss.NewStyle().Faint(true).Render(" to quit")

	return s
}

func openBrowser() error {
	params := url.Values{}
	params.Add("client_id", "9218969411171.9249336220343")
	params.Add("user_scope", "users:read")
	params.Add("redirect_uri", "https://hackcli-backend.vercel.app/api/callback")

	oauthURL := "https://slack.com/oauth/v2/authorize?" + params.Encode()

	err := browser.OpenURL(oauthURL)
	if err != nil {
		if err := exec.Command("wslview", oauthURL).Start(); err != nil {
			return err
		}
	}
	return nil
}

func saveTokenToConfig(token string) error {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(baseDir, "HackCLI")
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	file := filepath.Join(dir, "config.json")

	configData := Config{
		Token: token,
	}

	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(file, data, 0600)
	if err != nil {
		return err
	}

	return nil
}
