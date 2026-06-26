package tui

import (
	"context"
	"strings"
	"time"

	"github.com/aeltai/rancher-polymorph/internal/config"
	"github.com/aeltai/rancher-polymorph/internal/k8s"
	tea "github.com/charmbracelet/bubbletea"
)

type restoreProgressMsg struct {
	status     string
	backupName string
	done       bool
	err        error
}

func restoreGoroutine(cfg config.Config, localPath string, ch chan<- tea.Msg) {
	client := k8s.NewClient(cfg.Restore)
	ctx := context.Background()

	send := func(msg restoreProgressMsg) {
		ch <- msg
	}

	send(restoreProgressMsg{status: "Connecting to target cluster…"})
	if info, err := client.ClusterInfo(ctx); err == nil {
		send(restoreProgressMsg{status: "Target: " + strings.TrimSpace(info)})
	}

	send(restoreProgressMsg{status: "Step 1/3 — copying backup to operator pod…"})
	name, err := client.CopyBackup(ctx, localPath, cfg.Restore.OperatorNamespace,
		cfg.Restore.BackupPodLabel, cfg.Restore.BackupContainerPath)
	if err != nil {
		send(restoreProgressMsg{err: err, done: true})
		return
	}

	send(restoreProgressMsg{status: "Step 2/3 — applying Restore CR (prune: false)…", backupName: name})
	if err := client.ApplyRestore(ctx, cfg.Restore, name); err != nil {
		send(restoreProgressMsg{err: err, done: true})
		return
	}

	send(restoreProgressMsg{status: "Step 3/3 — waiting for restore Ready…", backupName: name})
	if err := pollRestoreReady(ctx, client, cfg, ch); err != nil {
		send(restoreProgressMsg{err: err, done: true, backupName: name})
		return
	}

	send(restoreProgressMsg{
		status:     "Restore Ready. Next: install cert-manager + Rancher Helm on the target cluster.",
		backupName: name,
		done:       true,
	})
}

func pollRestoreReady(ctx context.Context, client *k8s.Client, cfg config.Config, ch chan<- tea.Msg) error {
	timeout, _ := time.ParseDuration(cfg.Restore.WatchTimeout)
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		status, ready, failed, err := client.RestorePhase(ctx, cfg.Restore)
		if err != nil {
			return err
		}
		if status != "" {
			select {
			case ch <- restoreProgressMsg{status: "Restore: " + status}:
			default:
			}
		}
		if ready {
			return nil
		}
		if failed != "" {
			return errRestoreFailed(failed)
		}
		time.Sleep(5 * time.Second)
	}
	return errRestoreFailed("timed out waiting for Ready")
}

type restoreFailedError string

func (e restoreFailedError) Error() string { return string(e) }

func errRestoreFailed(msg string) error {
	return restoreFailedError(msg)
}
