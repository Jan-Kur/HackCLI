package profile

import (
	"fmt"
	"os"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/tui/profile"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit your slack profile",
	Long: `Change the following profile info:
	- real name
	- display name
	- status text
	- status emoji`,
	Run: edit,
}

func init() {
	ProfileCmd.AddCommand(editCmd)
}

func edit(cmd *cobra.Command, args []string) {
	if !api.IsLoggedIn() {
		fmt.Println("You are not logged in.\n\nLog in with: hackcli login")
		os.Exit(1)
	}

	program := tea.NewProgram(profile.Start(), tea.WithAltScreen())
	_, err := program.Run()
	if err != nil {
		fmt.Println("Something went wrong:", err)
		os.Exit(1)
	}
}
