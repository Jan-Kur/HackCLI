package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hackcli",
	Short: "A brief description of your application",
	Long: `To use HackCLI you first need to log in with slack.
You can do so by running "hackcli login".
After that you will be able to use all HackCLI features by running one of the commands below:`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
