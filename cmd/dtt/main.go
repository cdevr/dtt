package main

import (
	"fmt"
	"os"

	"github.com/example/dtt/cmd/dtt/commands"
)

var version = "0.1.0"

func main() {
	rootCmd := commands.NewRootCommand()
	rootCmd.Version = version

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
