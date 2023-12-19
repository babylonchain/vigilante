package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "vigilante",
		Short: "Babylon vigilante",
	}
	rootCmd.AddCommand(
		GetReporterCmd(),
		GetSubmitterCmd(),
		GetMonitorCmd(),
		GetBTCStakingTracker(),
	)

	return rootCmd
}
