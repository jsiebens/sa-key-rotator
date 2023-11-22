package main

import (
	"github.com/jsiebens/sa-key-rotator/internal/cmd"
	_ "github.com/jsiebens/sa-key-rotator/internal/log"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
