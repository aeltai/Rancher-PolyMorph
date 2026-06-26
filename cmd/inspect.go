package cmd

import (
	"fmt"
	"strings"

	"github.com/aeltai/rancher-migrate/internal/backup"
	"github.com/spf13/cobra"
)

func inspectCmd() *cobra.Command {
	var input string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a Rancher backup without modifying it",
		Long: `Read-only analysis of a Rancher backup tarball: cluster inventory,
Fleet mappings, fleet-default JSON count, local-cluster references, and
orphan "ghost" cluster IDs referenced in paths but missing from management.`,
		Example: strings.TrimSpace(`
  rancher-migrate inspect --input backups/source-full.tar.gz
  rancher-migrate inspect -i ./backups/source-full.tar.gz`),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := backup.InspectBackup(input)
			if err != nil {
				return err
			}
			fmt.Print(backup.FormatInspectSummary(res))
			fmt.Print(backup.FormatInspectBrief(res))
			return nil
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "Rancher backup .tar.gz (required)")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}
