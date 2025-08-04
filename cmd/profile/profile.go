package profile

import (
	"github.com/spf13/cobra"
)

var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Perform actions related to a slack profile",
	Long: `Available actions:
	- edit`,
}
