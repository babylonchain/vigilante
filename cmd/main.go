package main

import (
	"fmt"
	"github.com/babylonchain/babylon/app/params"
	"github.com/babylonchain/vigilante/cmd/monitor"
	"os"

	"github.com/spf13/cobra"

	"github.com/babylonchain/vigilante/cmd/reporter"
	"github.com/babylonchain/vigilante/cmd/submitter"
)

// TODO: init log

func main() {
	params.SetAddressPrefixes()
	rootCmd := &cobra.Command{
		Use:   "vigilante",
		Short: "Babylon vigilante",
	}
	rootCmd.AddCommand(reporter.GetCmd(), submitter.GetCmd(), monitor.GetCmd())

	if err := rootCmd.Execute(); err != nil {
		switch e := err.(type) {
		// TODO: dedicated error codes for vigilantes
		default:
			fmt.Print(e.Error())
			os.Exit(1)
		}
	}
}
