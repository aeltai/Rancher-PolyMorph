package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	EnvConfigPath = "RANCHER_MIGRATE_CONFIG"
	FileName      = "rancher-migrate.yaml"
)

// Config holds user defaults for sanitize, restore, and S3.
type Config struct {
	Defaults Defaults `yaml:"defaults"`
	Restore  Restore  `yaml:"restore"`
	S3       S3       `yaml:"s3"`
	UI       UI       `yaml:"ui"`
}

type Defaults struct {
	KeepCluster   string `yaml:"keep_cluster"`
	KeepRKE1Only  bool   `yaml:"keep_rke1_only"`
	Fast          bool   `yaml:"fast"`
	OutputDir     string `yaml:"output_dir"`
	ReportDir     string `yaml:"report_dir"`
	LogDir        string `yaml:"log_dir"`
	AutoOrphans   *bool  `yaml:"auto_orphans"` // nil = default true
	CompressLevel int    `yaml:"compress_level"`
}

type Restore struct {
	Kubeconfig          string `yaml:"kubeconfig"`
	Context             string `yaml:"context"`
	Namespace           string `yaml:"namespace"`
	OperatorNamespace   string `yaml:"operator_namespace"`
	BackupPodLabel      string `yaml:"backup_pod_label"`
	BackupContainerPath string `yaml:"backup_container_path"`
	RestoreName         string `yaml:"restore_name"`
	EncryptionSecret    string `yaml:"encryption_secret"`
	StorageSecret       string `yaml:"storage_secret"`
	WatchTimeout        string `yaml:"watch_timeout"`
}

type S3 struct {
	Region   string `yaml:"region"`
	Bucket   string `yaml:"bucket"`
	Prefix   string `yaml:"prefix"`
	Profile  string `yaml:"profile"`
	Endpoint string `yaml:"endpoint"` // optional MinIO / custom
}

type UI struct {
	Animations bool   `yaml:"animations"`
	Theme      string `yaml:"theme"`
}

func Default() Config {
	t := true
	return Config{
		Defaults: Defaults{
			Fast:          true,
			OutputDir:     "./backups",
			ReportDir:     "./backups",
			LogDir:        "./backups",
			AutoOrphans:   &t,
			CompressLevel: 3,
		},
		Restore: Restore{
			Namespace:           "cattle-resources-system",
			OperatorNamespace:   "cattle-resources-system",
			BackupPodLabel:      "app.kubernetes.io/name=rancher-backup",
			BackupContainerPath: "/var/lib/rancher-backup",
			RestoreName:         "rancher-restore",
			WatchTimeout:        "30m",
		},
		UI: UI{
			Animations: true,
			Theme:      "default",
		},
	}
}

func ExampleYAML() string {
	c := Default()
	c.Restore.Kubeconfig = "~/.kube/target-rancher.yaml"
	c.S3.Region = "eu-central-1"
	c.S3.Bucket = "my-rancher-backups"
	c.S3.Prefix = "migrations/"
	c.Defaults.KeepCluster = "c-xxxxx"
	b, _ := yaml.Marshal(c)
	return string(b)
}

func SearchPaths() []string {
	var paths []string
	if p := os.Getenv(EnvConfigPath); p != "" {
		paths = append(paths, p)
	}
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(wd, FileName))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "rancher-migrate", FileName),
			filepath.Join(home, "."+FileName),
		)
	}
	return paths
}

func Load() (Config, string, error) {
	cfg := Default()
	for _, path := range SearchPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, path, fmt.Errorf("parse %s: %w", path, err)
		}
		cfg.expandPaths()
		return cfg, path, nil
	}
	cfg.expandPaths()
	return cfg, "", nil
}

func (c *Config) expandPaths() {
	c.Restore.Kubeconfig = expandHome(c.Restore.Kubeconfig)
	c.Defaults.OutputDir = expandHome(c.Defaults.OutputDir)
	c.Defaults.ReportDir = expandHome(c.Defaults.ReportDir)
	c.Defaults.LogDir = expandHome(c.Defaults.LogDir)
}

func expandHome(path string) string {
	if path == "" || !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

func (c Config) AutoOrphansEnabled() bool {
	if c.Defaults.AutoOrphans == nil {
		return true
	}
	return *c.Defaults.AutoOrphans
}

func (c Config) S3URI(key string) string {
	key = strings.TrimPrefix(key, "/")
	prefix := strings.Trim(c.S3.Prefix, "/")
	if prefix != "" {
		key = prefix + "/" + strings.TrimPrefix(key, "/")
	}
	return fmt.Sprintf("s3://%s/%s", c.S3.Bucket, key)
}

func WriteDefault(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(ExampleYAML()), 0o644)
}
