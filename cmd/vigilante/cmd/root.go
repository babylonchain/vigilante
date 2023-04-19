package cmd

import (
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/spf13/cobra"
)

var (
	log = vlog.Logger.WithField("module", "cmd")
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "vigilante",
		Short: "Babylon vigilante",
	}
	rootCmd.AddCommand(
		GetReporterCmd(),
		GetSubmitterCmd(),
		GetMonitorCmd())

	return rootCmd
}
