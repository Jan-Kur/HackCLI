package channel

import (
	"fmt"
	"os"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/tui/channel"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open a slack channel",
	Long: `Opens a tui where you can interact with all of your channels.
The channel you selected or a default one will be opened by default`,
	Run:  open,
	Args: cobra.MaximumNArgs(1),
}

func init() {
	ChannelCmd.AddCommand(openCmd)
}

func open(cmd *cobra.Command, args []string) {
	if !api.IsLoggedIn() {
		fmt.Println("You are not logged in.\n\nLog in with: hackcli login")
		os.Exit(1)
	}

	var initialChannel string

	if len(args) > 0 {
		initialChannel = args[0]
	}

	app := channel.Start(initialChannel)

	program := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())

	go func() {
		for msg := range app.MsgChan {
			program.Send(msg)
		}
	}()

	_, err := program.Run()
	if err != nil {
		fmt.Println("Something went wrong:", err)
		os.Exit(1)
	}
}
