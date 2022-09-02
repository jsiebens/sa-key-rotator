package main

import (
	"github.com/jsiebens/sa-key-rotator/internal/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
