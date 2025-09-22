package channel

import (
	"fmt"
	"net/http"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	"github.com/Jan-Kur/HackCLI/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

func Start(initialChannel string) *app {
	cfg, err := api.LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("Couldn't load config: %v", err))
	}

	firstRun := false
	cache := api.LoadCache()
	if cache == nil {
		cache = &core.Cache{
			Conversations: make(map[string]*core.Conversation),
			Users:         make(map[string]*core.User),
		}
		firstRun = true
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
			errorPopup: errorPopup{
				theme:     styles.Themes[cfg.Theme],
				isVisible: false,
			},
			theme: styles.Themes[cfg.Theme],
			threadWindow: threadWindow{
				isOpen: false,
				chat: chat{
					viewport: initializeChat(),
				},
				input: initializeInput(styles.Themes[cfg.Theme]),
			},
		},
		App: core.App{
			User:           user.UserID,
			Config:         cfg,
			Cache:          cache,
			InitialLoading: firstRun,
			CurrentChannel: initialChannel,
			Client:         client,
			MsgChan:        msgChan,
		},
	}
	InitializeStyles(a.theme)

	a.LoadConversations()

	go api.RunWebsocket(a.Config.Token, a.Config.Cookie, a.MsgChan)

	return a
}
