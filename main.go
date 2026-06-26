package main

import (
	"os"

	"github.com/aeltai/rancher-polymorph/cmd"
	"github.com/aeltai/rancher-polymorph/internal/tui"
	"golang.org/x/term"
)

func main() {
	// Launch interactive UI when invoked with no arguments in a terminal.
	if len(os.Args) == 1 && term.IsTerminal(int(os.Stdout.Fd())) {
		if err := tui.Run(); err != nil {
			os.Exit(1)
		}
		return
	}
	if err := cmd.Root().Execute(); err != nil {
		os.Exit(1)
	}
}
