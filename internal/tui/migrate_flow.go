package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	postActionRestore = iota
	postActionMenu
)

func newSourceList(width, height int) list.Model {
	items := []list.Item{
		menuItem{"Local backup file", "Path to a .tar.gz on disk"},
		menuItem{"Download from S3", "List and pull from configured s3.bucket"},
	}
	w, h := listSize(width, height)
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = "Backup source"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return l
}

func newPostSanitizeList(width, height int) list.Model {
	items := []list.Item{
		menuItem{"Restore to cluster now", "kubectl cp + Restore CR + wait for Ready"},
		menuItem{"Finish (no restore)", "Keep sanitized tarball locally"},
	}
	w, h := listSize(width, height)
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = "Sanitize complete — next step"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return l
}

func listSize(width, height int) (int, int) {
	w, h := 60, 10
	if width > 4 {
		w = width - 4
	}
	if height > 10 {
		h = min(12, height-10)
	}
	return w, h
}

func (m model) updateSourceSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.screen = screenMenu
		return m, nil
	}
	if msg.String() != "enter" {
		var cmd tea.Cmd
		m.sourceList, cmd = m.sourceList.Update(msg)
		return m, cmd
	}
	switch m.sourceList.Index() {
	case 0:
		m.flow = flowMigrate
		m.screen = screenInput
		m.input.SetValue(m.backupPath)
		m.input.Placeholder = "/path/to/rancher-backup.tar.gz"
		m.input.Focus()
		return m, textinput.Blink
	case 1:
		if !s3Configured(m.cfg) {
			m.err = "s3.bucket not set — configure rancher-migrate.yaml (s3 section)"
			return m, nil
		}
		m.screen = screenLoading
		m.afterLoad = loadToS3List
		return m, tea.Batch(m.spinner.Tick, runS3List(m.cfg))
	}
	return m, nil
}

func (m model) updateS3KeyList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.screen = screenMenu
		return m, nil
	}
	if msg.String() != "enter" {
		var cmd tea.Cmd
		m.s3KeyList, cmd = m.s3KeyList.Update(msg)
		return m, cmd
	}
	item, ok := m.s3KeyList.SelectedItem().(s3KeyItem)
	if !ok {
		return m, nil
	}
	m.screen = screenLoading
	m.afterLoad = loadToS3Pull
	return m, tea.Batch(m.spinner.Tick, runS3Pull(m.cfg, item.key))
}

func (m model) updatePostSanitize(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.screen = screenMenu
		return m, nil
	}
	if msg.String() != "enter" {
		var cmd tea.Cmd
		m.postActionList, cmd = m.postActionList.Update(msg)
		return m, cmd
	}
	switch m.postActionList.Index() {
	case postActionRestore:
		return m.startRestoreFromResult()
	case postActionMenu:
		m.screen = screenDone
		return m, nil
	}
	return m, nil
}

func (m model) startRestoreFromResult() (tea.Model, tea.Cmd) {
	path := ""
	if m.result != nil {
		path = m.result.OutputPath
	}
	if path == "" {
		path = strings.TrimSpace(m.outputInput.Value())
	}
	if path == "" {
		m.err = "no sanitized output path"
		return m, nil
	}
	if _, err := os.Stat(path); err != nil {
		m.err = fmt.Sprintf("sanitized file: %v", err)
		return m, nil
	}
	if m.cfg.Restore.Kubeconfig == "" {
		m.err = "restore.kubeconfig not set — run: rancher-migrate config init"
		return m, nil
	}
	m.restoreLocal = path
	m.flow = flowRestore
	m.screen = screenRestoreRunning
	m.restoreStatus = "Starting restore…"
	ch := make(chan tea.Msg, 12)
	m.restoreCh = ch
	go restoreGoroutine(m.cfg, path, ch)
	return m, tea.Batch(m.spinner.Tick, waitSanitizeMsg(ch))
}

func (m model) beginSanitizeAfterPull(localPath string) (tea.Model, tea.Cmd) {
	m.backupPath = localPath
	m.err = ""
	m.afterLoad = loadToInspect
	m.screen = screenLoading
	return m, tea.Batch(m.spinner.Tick, runInspect(localPath))
}
