package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Defaults.OutputDir != "./backups" {
		t.Fatalf("output_dir=%q", cfg.Defaults.OutputDir)
	}
	if !cfg.AutoOrphansEnabled() {
		t.Fatal("auto orphans should default true")
	}
	if cfg.Restore.OperatorNamespace != "cattle-resources-system" {
		t.Fatalf("operator ns=%q", cfg.Restore.OperatorNamespace)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	content := ExampleYAML()
	content = strings.Replace(content, "c-xxxxx", "c-test1", 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigPath, path)

	cfg, loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != path {
		t.Fatalf("loaded from %q", loaded)
	}
	if cfg.Defaults.KeepCluster != "c-test1" {
		t.Fatalf("keep_cluster=%q", cfg.Defaults.KeepCluster)
	}
	if cfg.S3.Bucket != "my-rancher-backups" {
		t.Fatalf("bucket=%q", cfg.S3.Bucket)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("defaults: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigPath, path)

	_, _, err := Load()
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	got := expandHome("~/kube/config.yaml")
	want := filepath.Join(home, "kube/config.yaml")
	if got != want {
		t.Fatalf("expandHome=%q want %q", got, want)
	}
	if expandHome("/abs/path") != "/abs/path" {
		t.Fatal("absolute path changed")
	}
}

func TestS3URI(t *testing.T) {
	cfg := Default()
	cfg.S3.Bucket = "my-bucket"
	cfg.S3.Prefix = "migrations/"

	if got := cfg.S3URI("backup.tar.gz"); got != "s3://my-bucket/migrations/backup.tar.gz" {
		t.Fatalf("uri=%q", got)
	}
	if got := cfg.S3URI("/nested/key.tgz"); got != "s3://my-bucket/migrations/nested/key.tgz" {
		t.Fatalf("uri=%q", got)
	}
}

func TestWriteDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg", FileName)
	if err := WriteDefault(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "keep_cluster") {
		t.Fatal("expected example yaml content")
	}
}

func TestAutoOrphansDisabled(t *testing.T) {
	cfg := Default()
	f := false
	cfg.Defaults.AutoOrphans = &f
	if cfg.AutoOrphansEnabled() {
		t.Fatal("expected false")
	}
}

func TestSearchPathsIncludesEnv(t *testing.T) {
	t.Setenv(EnvConfigPath, "/tmp/custom.yaml")
	paths := SearchPaths()
	if paths[0] != "/tmp/custom.yaml" {
		t.Fatalf("first path=%q", paths[0])
	}
}
