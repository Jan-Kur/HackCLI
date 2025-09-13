package channel

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	"github.com/Jan-Kur/HackCLI/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

func Start(initialChannel string) *app {
	f, err := os.Create("debug.log")
	if err != nil {
		panic(err)
	}
	log.SetOutput(f)

	cfg, err := api.LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("Couldn't load config: %v", err))
	}

	httpCl := utils.NewCookieHTTP("https://slack.com", utils.ConvertCookies([]http.Cookie{{Name: "d", Value: cfg.Cookie}}))
	client := slack.New(cfg.Token, slack.OptionHTTPClient(httpCl))

	user, _ := client.AuthTest()

	msgChan := make(chan tea.Msg)

	a := &app{
		model: model{
			chat: chat{
				viewport: initializeChat(),
			},
			input:   initializeInput(styles.Themes[cfg.Theme]),
			focused: FocusInput,
			popup: popup{
				theme:     styles.Themes[cfg.Theme],
				isVisible: false,
				input:     initializePopup(styles.Themes[cfg.Theme]),
			},
			theme: styles.Themes[cfg.Theme],
			threadWindow: threadWindow{
				isOpen: false,
				chat: chat{
					viewport: initializeChat(),
				},
				input: initializeInput(styles.Themes[cfg.Theme]),
			},
			latestMarked:  make(map[string]string),
			latestMessage: make(map[string]string),
			userPresence:  make(map[string]string),
		},
		App: core.App{
			User:      user.UserID,
			Config:    cfg,
			Client:    client,
			MsgChan:   msgChan,
			UserCache: make(map[string]string),
		},
	}
	InitializeStyles(a.theme)

	l, initialChannelID := a.initializeSidebar(initialChannel)

	a.sidebar = l
	a.CurrentChannel = initialChannelID

	go api.RunWebsocket(a.Config.Token, a.Config.Cookie, a.MsgChan)

	return a
}
