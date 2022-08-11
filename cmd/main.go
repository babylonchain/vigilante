package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/babylonchain/vigilante/cmd/reporter"
	"github.com/babylonchain/vigilante/cmd/submitter"
)

// TODO: init log

func main() {
	rootCmd := &cobra.Command{
		Use:   "vigilante",
		Short: "Babylon vigilante",
	}
	rootCmd.AddCommand(reporter.GetCmd(), submitter.GetCmd())

	if err := rootCmd.Execute(); err != nil {
		switch e := err.(type) {
		// TODO: dedicated error codes for vigilantes
		default:
			fmt.Print(e.Error())
			os.Exit(1)
		}
	}
}
