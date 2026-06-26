package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aeltai/rancher-migrate/internal/ascii"
	"github.com/aeltai/rancher-migrate/internal/backup"
	"github.com/aeltai/rancher-migrate/internal/k8s"
	"github.com/spf13/cobra"
)

func restoreCmd() *cobra.Command {
	restore := &cobra.Command{
		Use:   "restore",
		Short: "Restore helpers for migration to a new cluster",
		Long: `Generate Restore CR manifests, copy backups to the operator PVC,
and apply/watch restore on a target cluster via kubeconfig.`,
	}

	restore.AddCommand(restorePlanCmd())
	restore.AddCommand(restoreCopyCmd())
	restore.AddCommand(restoreApplyCmd())
	restore.AddCommand(restoreStatusCmd())
	restore.AddCommand(restoreRunCmd())
	return restore
}

func restorePlanCmd() *cobra.Command {
	var (
		name          string
		backupFile    string
		output        string
		encryption    string
		storageSecret string
	)

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Print a Restore CR manifest (prune: false)",
		Long: `Generate a Kubernetes Restore custom resource for migrating Rancher
to a new cluster with prune: false (per Rancher documentation).

Apply while Rancher is NOT running; install cert-manager and Rancher Helm
after the restore completes.`,
		Example: strings.TrimSpace(`
  rancher-migrate restore plan \
    --backup-file sanitized-backup.tar.gz \
    --output restore-cr.yaml`),
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			if name == "rancher-restore" && appConfig.Restore.RestoreName != "" {
				name = appConfig.Restore.RestoreName
			}
			if encryption == "" {
				encryption = appConfig.Restore.EncryptionSecret
			}
			if storageSecret == "" {
				storageSecret = appConfig.Restore.StorageSecret
			}
			text := backup.FormatRestoreCR(backup.RestorePlanOptions{
				Name:          name,
				BackupFile:    backupFile,
				Encryption:    encryption,
				StorageSecret: storageSecret,
			})
			if output != "" {
				if err := os.WriteFile(output, []byte(text), 0o644); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Wrote Restore CR to %s\n", output)
				return nil
			}
			fmt.Print(text)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "rancher-restore", "Restore CR metadata.name")
	cmd.Flags().StringVarP(&backupFile, "backup-file", "b", "", "backupFilename in Restore spec (required)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write manifest to file instead of stdout")
	cmd.Flags().StringVar(&encryption, "encryption-secret", "", "encryptionConfigSecretName if backup was encrypted")
	cmd.Flags().StringVar(&storageSecret, "storage-secret", "", "S3 credentialSecretName for storageLocation")
	_ = cmd.MarkFlagRequired("backup-file")
	return cmd
}

func restoreCopyCmd() *cobra.Command {
	var (
		kubeconfig string
		local      string
	)
	cmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy local backup tarball into rancher-backup operator pod",
		Long: `Uses kubectl cp to place the tarball on the backup operator PVC path.
Requires kubectl and a valid kubeconfig for the restore target cluster.`,
		Example: strings.TrimSpace(`
  rancher-migrate restore copy --local ./backups/sanitized.tar.gz`),
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			rc := appConfig.Restore
			if kubeconfig != "" {
				rc.Kubeconfig = kubeconfig
			}
			if rc.Kubeconfig == "" {
				return fmt.Errorf("kubeconfig required: set restore.kubeconfig in config or --kubeconfig")
			}
			client := k8s.NewClient(rc)
			ctx := context.Background()
			if info, err := client.ClusterInfo(ctx); err == nil {
				fmt.Fprintf(os.Stderr, "Cluster context: %s\n", strings.TrimSpace(info))
			}
			name, err := client.CopyBackup(ctx, local,
				rc.OperatorNamespace, rc.BackupPodLabel, rc.BackupContainerPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Copied to operator pod as %s\n", name)
			fmt.Fprintf(os.Stdout, "%s\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to target cluster kubeconfig")
	cmd.Flags().StringVar(&local, "local", "", "Local .tar.gz path (required)")
	_ = cmd.MarkFlagRequired("local")
	return cmd
}

func restoreApplyCmd() *cobra.Command {
	var (
		kubeconfig string
		backupFile string
		watch      bool
	)
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply Restore CR on target cluster",
		Example: strings.TrimSpace(`
  rancher-migrate restore apply --backup-file sanitized.tar.gz --watch`),
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			rc := appConfig.Restore
			if kubeconfig != "" {
				rc.Kubeconfig = kubeconfig
			}
			if rc.Kubeconfig == "" {
				return fmt.Errorf("kubeconfig required: set restore.kubeconfig in config or --kubeconfig")
			}
			client := k8s.NewClient(rc)
			ctx := context.Background()
			if info, err := client.ClusterInfo(ctx); err == nil {
				fmt.Fprintf(os.Stderr, "Cluster context: %s\n", strings.TrimSpace(info))
			}
			fmt.Fprintf(os.Stderr, "Applying Restore CR (prune: false)…\n")
			if err := client.ApplyRestore(ctx, rc, backupFile); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Restore CR applied.")
			if watch {
				fmt.Fprintln(os.Stderr, "Watching restore status…")
				return client.WatchRestore(ctx, rc)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Target cluster kubeconfig")
	cmd.Flags().StringVarP(&backupFile, "backup-file", "b", "", "backupFilename in Restore spec (required)")
	cmd.Flags().BoolVar(&watch, "watch", false, "Wait until restore Ready")
	_ = cmd.MarkFlagRequired("backup-file")
	return cmd
}

func restoreStatusCmd() *cobra.Command {
	var kubeconfig string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Restore CR status from target cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			rc := appConfig.Restore
			if kubeconfig != "" {
				rc.Kubeconfig = kubeconfig
			}
			client := k8s.NewClient(rc)
			out, err := client.Status(context.Background(), rc)
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	}
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Target cluster kubeconfig")
	return cmd
}

func restoreRunCmd() *cobra.Command {
	var (
		kubeconfig string
		local      string
		watch      bool
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Copy backup to operator pod and apply Restore CR",
		Long: `End-to-end restore kickoff: kubectl cp → Restore CR apply → optional watch.
Rancher must NOT be running on the target cluster.`,
		Example: strings.TrimSpace(`
  rancher-migrate restore run --local ./backups/sanitized.tar.gz --watch`),
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			rc := appConfig.Restore
			if kubeconfig != "" {
				rc.Kubeconfig = kubeconfig
			}
			if rc.Kubeconfig == "" {
				return fmt.Errorf("kubeconfig required: rancher-migrate config init")
			}
			fmt.Println(ascii.CompactHeader())
			client := k8s.NewClient(rc)
			ctx := context.Background()
			if info, err := client.ClusterInfo(ctx); err == nil {
				fmt.Fprintf(os.Stderr, "Target: %s\n", strings.TrimSpace(info))
			}
			fmt.Fprintln(os.Stderr, "Step 1/2: copy backup to operator pod…")
			name, err := client.CopyBackup(ctx, local, rc.OperatorNamespace, rc.BackupPodLabel, rc.BackupContainerPath)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Step 2/2: apply Restore CR…")
			if err := client.ApplyRestore(ctx, rc, name); err != nil {
				return err
			}
			if watch {
				fmt.Fprintln(os.Stderr, "Watching restore…")
				return client.WatchRestore(ctx, rc)
			}
			fmt.Fprintln(os.Stderr, "Done. Check: rancher-migrate restore status")
			return nil
		},
	}
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Target cluster kubeconfig")
	cmd.Flags().StringVar(&local, "local", "", "Local sanitized .tar.gz (required)")
	cmd.Flags().BoolVar(&watch, "watch", true, "Wait for restore Ready")
	_ = cmd.MarkFlagRequired("local")
	return cmd
}
