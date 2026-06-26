package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aeltai/rancher-polymorph/internal/config"
	"github.com/aeltai/rancher-polymorph/internal/s3store"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type s3KeyItem struct {
	key string
}

func (i s3KeyItem) Title() string       { return filepath.Base(i.key) }
func (i s3KeyItem) Description() string { return i.key }
func (i s3KeyItem) FilterValue() string { return i.key }

type s3ListDoneMsg struct {
	keys []string
	err  error
}

type s3PullDoneMsg struct {
	localPath string
	err       error
}

func newS3KeyList(keys []string, width, height int) list.Model {
	items := make([]list.Item, 0, len(keys))
	for _, k := range keys {
		if !strings.HasSuffix(k, ".tar.gz") && !strings.HasSuffix(k, ".tgz") {
			continue
		}
		items = append(items, s3KeyItem{key: k})
	}
	w := 60
	h := 14
	if width > 4 {
		w = width - 4
	}
	if height > 10 {
		h = min(16, height-8)
	}
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = "S3 backups (.tar.gz)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	return l
}

func s3Configured(cfg config.Config) bool {
	return cfg.S3.Bucket != ""
}

func s3DestPath(cfg config.Config, key string) string {
	dir := cfg.Defaults.OutputDir
	if dir == "" {
		dir = "./backups"
	}
	return filepath.Join(dir, filepath.Base(key))
}

func runS3List(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		client, err := s3store.NewFromConfig(cfg.S3)
		if err != nil {
			return s3ListDoneMsg{err: err}
		}
		keys, err := client.ListKeys(context.Background(), "")
		if err != nil {
			return s3ListDoneMsg{err: err}
		}
		return s3ListDoneMsg{keys: keys}
	}
}

func runS3Pull(cfg config.Config, key string) tea.Cmd {
	return func() tea.Msg {
		client, err := s3store.NewFromConfig(cfg.S3)
		if err != nil {
			return s3PullDoneMsg{err: err}
		}
		dest := s3DestPath(cfg, key)
		if err := client.Download(context.Background(), key, dest); err != nil {
			return s3PullDoneMsg{err: err}
		}
		return s3PullDoneMsg{localPath: dest}
	}
}

func renderS3KeyList(cfg config.Config) string {
	var b strings.Builder
	b.WriteString(boxStyle.Render("Pull backup from S3"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  bucket: %s  region: %s\n", cfg.S3.Bucket, cfg.S3.Region))
	if cfg.S3.Prefix != "" {
		b.WriteString(fmt.Sprintf("  prefix: %s\n", cfg.S3.Prefix))
	}
	return b.String()
}
