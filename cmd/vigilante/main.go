package main

import (
	"fmt"
	"github.com/babylonchain/babylon/app/params"
	"github.com/babylonchain/vigilante/cmd/vigilante/cmd"
	"os"
)

// TODO: init log

func main() {
	params.SetAddressPrefixes()

	rootCmd := cmd.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		switch e := err.(type) {
		// TODO: dedicated error codes for vigilantes
		default:
			fmt.Print(e.Error())
			os.Exit(1)
		}
	}
}
