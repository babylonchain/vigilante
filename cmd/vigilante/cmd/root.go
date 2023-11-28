package cmd

import (
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	log = vlog.Logger.With(zap.String("module", "cmd")).Sugar()
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
