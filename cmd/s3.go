package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aeltai/rancher-migrate/internal/config"
	"github.com/aeltai/rancher-migrate/internal/s3store"
	"github.com/spf13/cobra"
)

func s3Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "s3",
		Short: "Download or upload backup tarballs via S3",
		Long: `S3 helpers for backup/restore workflows. Credentials use the default AWS
chain (env vars, ~/.aws/credentials, IAM role). Configure defaults in
rancher-migrate.yaml under s3:.`,
	}
	cmd.AddCommand(s3PullCmd())
	cmd.AddCommand(s3PushCmd())
	cmd.AddCommand(s3ListCmd())
	return cmd
}

func s3PullCmd() *cobra.Command {
	var (
		output string
		bucket string
		region string
		prefix string
		profile string
	)
	cmd := &cobra.Command{
		Use:   "pull KEY_OR_URI",
		Short: "Download a backup from S3",
		Example: strings.TrimSpace(`
  rancher-migrate s3 pull migrations/source-full.tar.gz -o ./backups/in.tar.gz
  rancher-migrate s3 pull s3://my-bucket/migrations/source-full.tar.gz -o in.tar.gz`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			opts := mergeS3Opts(appConfig.S3, bucket, region, prefix, profile)
			client, err := s3store.New(opts)
			if err != nil {
				return err
			}
			uri := args[0]
			if !strings.HasPrefix(uri, "s3://") && opts.Bucket != "" {
				uri = appConfig.S3URI(uri)
			}
			if output == "" {
				output = filepathBase(uri)
			}
			fmt.Fprintf(os.Stderr, "Downloading %s → %s\n", uri, output)
			return client.Download(context.Background(), uri, output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Local destination path")
	cmd.Flags().StringVar(&bucket, "bucket", "", "S3 bucket (overrides config)")
	cmd.Flags().StringVar(&region, "region", "", "AWS region")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Key prefix")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS shared config profile")
	return cmd
}

func s3PushCmd() *cobra.Command {
	var (
		uri     string
		bucket  string
		region  string
		prefix  string
		profile string
	)
	cmd := &cobra.Command{
		Use:   "push LOCAL_PATH [KEY_OR_URI]",
		Short: "Upload a backup tarball to S3",
		Example: strings.TrimSpace(`
  rancher-migrate s3 push ./backups/sanitized.tar.gz migrations/sanitized.tar.gz
  rancher-migrate s3 push ./sanitized.tar.gz s3://bucket/migrations/sanitized.tar.gz`),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			opts := mergeS3Opts(appConfig.S3, bucket, region, prefix, profile)
			client, err := s3store.New(opts)
			if err != nil {
				return err
			}
			local := args[0]
			dest := uri
			if len(args) == 2 {
				dest = args[1]
			}
			if dest == "" {
				dest = filepathBase(local)
			}
			if !strings.HasPrefix(dest, "s3://") && opts.Bucket != "" {
				dest = appConfig.S3URI(dest)
			}
			fmt.Fprintf(os.Stderr, "Uploading %s → %s\n", local, dest)
			return client.Upload(context.Background(), local, dest)
		},
	}
	cmd.Flags().StringVar(&uri, "uri", "", "Destination key or s3:// URI")
	cmd.Flags().StringVar(&bucket, "bucket", "", "S3 bucket")
	cmd.Flags().StringVar(&region, "region", "", "AWS region")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Key prefix")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS profile")
	return cmd
}

func s3ListCmd() *cobra.Command {
	var prefix, bucket, region, profile string
	cmd := &cobra.Command{
		Use:   "list [PREFIX]",
		Short: "List backup keys under prefix",
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			opts := mergeS3Opts(appConfig.S3, bucket, region, prefix, profile)
			client, err := s3store.New(opts)
			if err != nil {
				return err
			}
			p := ""
			if len(args) > 0 {
				p = args[0]
			}
			keys, err := client.ListKeys(context.Background(), p)
			if err != nil {
				return err
			}
			for _, k := range keys {
				fmt.Println(k)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bucket, "bucket", "", "S3 bucket")
	cmd.Flags().StringVar(&region, "region", "", "AWS region")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Key prefix")
	cmd.Flags().StringVar(&profile, "profile", "", "AWS profile")
	return cmd
}

func mergeS3Opts(cfg config.S3, bucket, region, prefix, profile string) s3store.Options {
	if bucket != "" {
		cfg.Bucket = bucket
	}
	if region != "" {
		cfg.Region = region
	}
	if prefix != "" {
		cfg.Prefix = prefix
	}
	if profile != "" {
		cfg.Profile = profile
	}
	return s3store.Options{
		Region:   cfg.Region,
		Bucket:   cfg.Bucket,
		Prefix:   cfg.Prefix,
		Profile:  cfg.Profile,
		Endpoint: cfg.Endpoint,
	}
}

func filepathBase(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
