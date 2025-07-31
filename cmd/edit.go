package cmd

import (
	"fmt"
	"os"

	"github.com/Jan-Kur/HackCLI/slack"
	"github.com/Jan-Kur/HackCLI/tui/profile"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var validArguments = []string{"profile", "theme"}

var editCmd = &cobra.Command{
	Use:       "edit",
	Short:     "Edit something",
	Long:      `Edit the argument you pass, eg. "hackcli edit profile" let's you edit your slack profile`,
	Run:       edit,
	ValidArgs: validArguments,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func edit(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Printf(`Usage: hackcli edit [argument]
Available arguments: %s`, validArguments)
		os.Exit(1)
	}
	var program *tea.Program

	switch args[0] {
	case "profile":
		program = tea.NewProgram(profile.Start(), tea.WithAltScreen())
	default:
		fmt.Printf(`Usage: hackcli edit [argument]
		Available arguments: %s`, validArguments)
		os.Exit(1)
	}

	if !slack.IsLoggedIn() {
		fmt.Println("You are not logged in.\n\nLog in with: hackcli login")
		os.Exit(1)
	}

	_, err := program.Run()
	if err != nil {
		fmt.Println("Something went wrong:", err)
		os.Exit(1)
	}
}
