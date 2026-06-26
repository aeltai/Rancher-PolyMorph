package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aeltai/rancher-polymorph/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var appConfig config.Config
var appConfigPath string

func loadAppConfig() {
	cfg, path, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	appConfig = cfg
	appConfigPath = path
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or initialize rancher-polymorph configuration",
	}

	cmd.AddCommand(configShowCmd())
	cmd.AddCommand(configInitCmd())
	cmd.AddCommand(configPathsCmd())
	return cmd
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print effective configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			loadAppConfig()
			if appConfigPath != "" {
				fmt.Fprintf(os.Stderr, "# loaded: %s\n", appConfigPath)
			} else {
				fmt.Fprintf(os.Stderr, "# no config file found (using defaults)\n")
			}
			b, err := yaml.Marshal(appConfig)
			if err != nil {
				return err
			}
			fmt.Print(string(b))
			return nil
		},
	}
}

func configInitCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write an example config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				path = filepath.Join(home, ".config", "rancher-polymorph", config.FileName)
			}
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists (use --path to choose another file)", path)
			}
			if err := config.WriteDefault(path); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Wrote %s\n", path)
			fmt.Fprintf(os.Stderr, "Edit kubeconfig, s3 bucket, and default keep_cluster.\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Config file path (default ~/.config/rancher-polymorph/rancher-polymorph.yaml)")
	return cmd
}

func configPathsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "paths",
		Short: "List config search paths",
		Run: func(cmd *cobra.Command, args []string) {
			for _, p := range config.SearchPaths() {
				fmt.Println(p)
			}
		},
	}
}
