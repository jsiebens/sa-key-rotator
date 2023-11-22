package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var (
	stdout = os.Stdout
	stderr = os.Stderr
)

var (
	logLevel = os.Getenv("LOG_LEVEL")
)

func Command() *cobra.Command {
	rootCmd := rootCommand()
	rootCmd.AddCommand(serverCommand())
	rootCmd.AddCommand(rotateCommand())
	return rootCmd
}

func Execute() error {
	return Command().Execute()
}

func rootCommand() *cobra.Command {
	return &cobra.Command{
		Use: "sa-key-rotator",
	}
}
