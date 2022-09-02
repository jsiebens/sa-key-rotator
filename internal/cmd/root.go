package cmd

import (
	"github.com/muesli/coral"
	"os"
)

var (
	stdout = os.Stdout
	stderr = os.Stderr
)

var (
	logLevel = os.Getenv("LOG_LEVEL")
)

func Command() *coral.Command {
	rootCmd := rootCommand()
	rootCmd.AddCommand(serverCommand())
	rootCmd.AddCommand(checkCommand())
	return rootCmd
}

func Execute() error {
	return Command().Execute()
}

func rootCommand() *coral.Command {
	return &coral.Command{
		Use: "sa-key-rotator",
	}
}
