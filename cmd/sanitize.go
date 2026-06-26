package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/aeltai/rancher-polymorph/internal/backup"
	"github.com/spf13/cobra"
)

var (
	globalVerbose bool
	globalQuiet   bool
)

func sanitizeCmd() *cobra.Command {
	var (
		input          string
		output         string
		report         string
		keepCluster    string
		keepRKE1Only   bool
		removeClusters []string
		noAutoOrphans  bool
		compressLevel  int
		fast           bool
		logFile        string
		fullStdout     bool
		quiet          bool
	)

	cmd := &cobra.Command{
		Use:   "sanitize",
		Short: "Sanitize a Rancher backup tarball for migration restore",
		Long: `Filter a full Rancher backup to keep only selected downstream cluster(s).

Strips local cluster artifacts, Fleet/provisioning debris for removed clusters,
and auto-detects orphan cluster ghost IDs in paths. Use inspect first on large
backups to review inventory and ghosts before sanitizing.

Progress and phase logs go to stderr (or --log-file). A summary box is printed
to stdout; per-path details are written to --report when set.

See also: rancher-polymorph manual sanitize`,
		Example: strings.TrimSpace(`
  # Keep one RKE1 cluster for migration restore
  rancher-polymorph sanitize \
    --input backups/source-full.tar.gz \
    --output backups/sanitized.tar.gz \
    --keep-cluster c-aaaaa \
    --report backups/sanitize-report.txt

  # Fast pass with verbose logging
  rancher-polymorph sanitize -i in.tar.gz -o out.tar.gz --keep-cluster c-aaaaa \
    --fast --verbose --log-file sanitize.log

  # Strip explicit orphan ghosts found by inspect
  rancher-polymorph sanitize -i in.tar.gz -o out.tar.gz --keep-cluster c-aaaaa \
    --remove-cluster c-ghost1 --remove-cluster c-ghost2`),
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			if keepCluster == "" {
				keepCluster = appConfig.Defaults.KeepCluster
			}
			if !cmd.Flags().Changed("fast") {
				fast = appConfig.Defaults.Fast
			}
			if !cmd.Flags().Changed("compress-level") && appConfig.Defaults.CompressLevel > 0 {
				compressLevel = appConfig.Defaults.CompressLevel
			}
			if !cmd.Flags().Changed("no-auto-orphans") {
				noAutoOrphans = !appConfig.AutoOrphansEnabled()
			}
			if keepCluster != "" && len(removeClusters) > 0 {
				fmt.Fprintln(os.Stderr, "warning: --remove-cluster adds extra IDs on top of --keep-cluster")
			}
			opts := backup.Options{
				Input:          input,
				Output:         output,
				Report:         report,
				KeepCluster:    keepCluster,
				KeepRKE1Only:   keepRKE1Only,
				RemoveClusters: removeClusters,
				NoAutoOrphans:  noAutoOrphans,
				CompressLevel:  compressLevel,
				Fast:           fast,
				Quiet:          globalQuiet || quiet,
				Verbose:        globalVerbose,
				LogFile:        logFile,
			}
			res, err := backup.Sanitize(opts)
			if err != nil {
				return err
			}
			fmt.Print(backup.FormatSanitizeSummary(res))
			if fullStdout || report == "" {
				fmt.Print(backup.FormatReport(res))
			} else {
				fmt.Print(backup.FormatReportBrief(res))
				fmt.Fprintf(os.Stderr, "Full path list written to %s\n", report)
			}
			return backup.WriteReport(res, report)
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "Full Rancher backup .tar.gz (required)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Sanitized output .tar.gz (required)")
	cmd.Flags().StringVarP(&report, "report", "r", "", "Write full removal report (all paths)")
	cmd.Flags().StringVar(&keepCluster, "keep-cluster", "", "Keep only this downstream cluster ID")
	cmd.Flags().BoolVar(&keepRKE1Only, "keep-rke1-only", false, "Keep all RKE1 clusters; remove imported/RKE2")
	cmd.Flags().StringArrayVar(&removeClusters, "remove-cluster", nil, "Cluster ID to strip (repeatable)")
	cmd.Flags().BoolVar(&noAutoOrphans, "no-auto-orphans", false, "Disable orphan cluster auto-detection")
	cmd.Flags().IntVar(&compressLevel, "compress-level", 3, "gzip level 1-9 for output tarball")
	cmd.Flags().BoolVar(&fast, "fast", false, "Shorthand for --compress-level 1")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress progress bar and stderr logs")
	cmd.Flags().StringVar(&logFile, "log-file", "", "Append timestamped log lines to this file")
	cmd.Flags().BoolVar(&fullStdout, "full", false, "Print full report to stdout (default when --report is unset)")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}
