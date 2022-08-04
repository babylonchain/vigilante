package submitter

import (
	"fmt"

	"github.com/spf13/cobra"
)

// GetCmd returns the cli query commands for this module
func GetCmd() *cobra.Command {
	// Group epoching queries under a subcommand
	cmd := &cobra.Command{
		Use:   "submitter",
		Short: "Vigilant submitter",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("Hello world!")
		},
	}

	return cmd
}
