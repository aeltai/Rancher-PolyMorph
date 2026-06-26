package cmd

import (
	"fmt"
	"os"

	"github.com/aeltai/rancher-polymorph/internal/tui"
	"github.com/spf13/cobra"
)

func uiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ui",
		Aliases: []string{"interactive", "tui"},
		Short:   "Launch interactive terminal UI",
		Long: `Interactive wizard for inspecting and sanitizing Rancher backups.

Guides you through backup path entry, cluster selection, orphan cleanup,
output paths, and sanitize execution with a live progress bar.

Requires a terminal (TTY). For scripting, use the sanitize and inspect subcommands.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := tui.Run(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return err
			}
			return nil
		},
	}
	return cmd
}
