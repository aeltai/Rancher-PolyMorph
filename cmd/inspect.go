package cmd

import (
	"fmt"
	"strings"

	"github.com/aeltai/rancher-migrate/internal/backup"
	"github.com/spf13/cobra"
)

func inspectCmd() *cobra.Command {
	var (
		input        string
		showTree     bool
		keepCluster  string
		keepClusters []string
		keepRKE1Only bool
	)

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a Rancher backup without modifying it",
		Long: `Read-only analysis of a Rancher backup tarball: cluster inventory,
Fleet mappings, fleet-default JSON count, local-cluster references, and
orphan "ghost" cluster IDs referenced in paths but missing from management.

Use --tree to show the full backup inventory (clusters, local artifacts, global).
Add --keep-cluster to preview keep/drop before sanitizing.`,
		Example: strings.TrimSpace(`
  rancher-migrate inspect --input backups/source-full.tar.gz
  rancher-migrate inspect -i backup.tar.gz --tree
  rancher-migrate inspect -i backup.tar.gz --tree --keep-cluster c-aaaaa`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if showTree {
				if keepCluster != "" || len(keepClusters) > 0 || keepRKE1Only {
					opts := backup.Options{
						Input:        input,
						KeepCluster:  keepCluster,
						KeepClusters: keepClusters,
						KeepRKE1Only: keepRKE1Only,
						InspectOnly:  true,
					}
					preview, err := backup.PreviewSanitize(opts)
					if err != nil {
						return err
					}
					printTree(preview)
					return nil
				}
				tree, err := backup.BuildInspectTree(input)
				if err != nil {
					return err
				}
				printTree(tree)
				return nil
			}
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
	cmd.Flags().BoolVar(&showTree, "tree", false, "Show backup inventory tree (clusters + local strip targets)")
	cmd.Flags().StringVar(&keepCluster, "keep-cluster", "", "Cluster ID to keep (tree preview)")
	cmd.Flags().StringSliceVar(&keepClusters, "keep-clusters", nil, "Additional cluster IDs to keep (repeatable)")
	cmd.Flags().BoolVar(&keepRKE1Only, "keep-rke1-only", false, "Keep all RKE1 clusters (tree preview)")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func printTree(preview *backup.PreviewResult) {
	expanded := make(map[string]bool)
	for _, line := range backup.FormatTreeLines(preview, expanded, 100) {
		fmt.Println(line)
	}
}
