package cmd

import (
	"fmt"
	"os"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/cmd/profile"
	"github.com/Jan-Kur/HackCLI/tui/channel"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "hackcli",
	Short: "A brief description of your application",
	Long:  `Opens a the main tui. The provided channel or DM will be opened first. First in the list if not specified.`,
	Run:   start,
	Args:  cobra.MaximumNArgs(1),
}

func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func start(cmd *cobra.Command, args []string) {
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

func init() {
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	RootCmd.AddCommand(profile.ProfileCmd)
}
