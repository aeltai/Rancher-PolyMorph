package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aeltai/rancher-polymorph/internal/backup"
	"github.com/aeltai/rancher-polymorph/internal/config"
)

type Client struct {
	kubeconfig string
	context    string
	namespace  string
}

func NewClient(cfg config.Restore) *Client {
	return &Client{
		kubeconfig: cfg.Kubeconfig,
		context:    cfg.Context,
		namespace:  cfg.Namespace,
	}
}

func (c *Client) kubectl(ctx context.Context, args ...string) *exec.Cmd {
	base := []string{}
	if c.kubeconfig != "" {
		base = append(base, "--kubeconfig", c.kubeconfig)
	}
	if c.context != "" {
		base = append(base, "--context", c.context)
	}
	cmd := exec.CommandContext(ctx, "kubectl", append(base, args...)...)
	return cmd
}

func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := c.kubectl(ctx, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// FindBackupPod returns namespace/name of a rancher-backup operator pod.
func (c *Client) FindBackupPod(ctx context.Context, operatorNS, label string) (string, string, error) {
	if operatorNS == "" {
		operatorNS = c.namespace
	}
	out, err := c.run(ctx, "get", "pods", "-n", operatorNS, "-l", label,
		"-o", "jsonpath={.items[0].metadata.namespace}/{.items[0].metadata.name}")
	if err != nil {
		return "", "", fmt.Errorf("find backup pod (label %s in %s): %w", label, operatorNS, err)
	}
	parts := strings.Split(strings.TrimSpace(out), "/")
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("no backup operator pod found with label %s", label)
	}
	return parts[0], parts[1], nil
}

// CopyBackup uploads a local tarball into the operator PVC path.
func (c *Client) CopyBackup(ctx context.Context, localPath, operatorNS, label, containerPath string) (string, error) {
	ns, pod, err := c.FindBackupPod(ctx, operatorNS, label)
	if err != nil {
		return "", err
	}
	dest := strings.TrimSuffix(containerPath, "/") + "/" + filepathBase(localPath)
	args := []string{"cp", localPath, fmt.Sprintf("%s/%s:%s", ns, pod, dest)}
	cmd := c.kubectl(ctx, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("kubectl cp: %w", err)
	}
	return filepathBase(localPath), nil
}

func filepathBase(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// ApplyRestore creates the Restore CR on the cluster.
func (c *Client) ApplyRestore(ctx context.Context, cfg config.Restore, backupFilename string) error {
	manifest := backup.FormatRestoreCR(backup.RestorePlanOptions{
		Name:          firstNonEmpty(cfg.RestoreName, "rancher-restore"),
		BackupFile:    backupFilename,
		Encryption:    cfg.EncryptionSecret,
		StorageSecret: cfg.StorageSecret,
	})
	cmd := c.kubectl(ctx, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply restore: %w: %s", err, stderr.String())
	}
	return nil
}

// WatchRestore waits for restore CR to complete or fail.
func (c *Client) WatchRestore(ctx context.Context, cfg config.Restore) error {
	name := firstNonEmpty(cfg.RestoreName, "rancher-restore")
	ns := firstNonEmpty(cfg.Namespace, "cattle-resources-system")
	timeout, _ := time.ParseDuration(cfg.WatchTimeout)
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
		out, err := c.run(ctx, "get", "restore", name, "-n", ns,
			"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}")
		if err == nil && strings.TrimSpace(out) == "True" {
			return nil
		}
		fail, _ := c.run(ctx, "get", "restore", name, "-n", ns,
			"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].message}")
		if strings.Contains(strings.ToLower(fail), "error") {
			return fmt.Errorf("restore failed: %s", strings.TrimSpace(fail))
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("restore %s/%s not ready within %s", ns, name, timeout)
}

// RestorePhase returns human status, ready, and failure message from the Restore CR.
func (c *Client) RestorePhase(ctx context.Context, cfg config.Restore) (status string, ready bool, failed string, err error) {
	name := firstNonEmpty(cfg.RestoreName, "rancher-restore")
	ns := firstNonEmpty(cfg.Namespace, "cattle-resources-system")
	phase, err := c.run(ctx, "get", "restore", name, "-n", ns, "-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status},{.status.conditions[?(@.type==\"Ready\")].message}")
	if err != nil {
		return "", false, "", err
	}
	parts := strings.SplitN(strings.TrimSpace(phase), ",", 2)
	st := ""
	if len(parts) > 0 {
		st = parts[0]
	}
	msg := ""
	if len(parts) > 1 {
		msg = parts[1]
	}
	if st == "True" {
		return "Ready", true, "", nil
	}
	if strings.Contains(strings.ToLower(msg), "error") || strings.Contains(strings.ToLower(msg), "fail") {
		return msg, false, msg, nil
	}
	if msg != "" {
		return msg, false, "", nil
	}
	return "In progress", false, "", nil
}

// Status prints restore resource status.
func (c *Client) Status(ctx context.Context, cfg config.Restore) (string, error) {
	name := firstNonEmpty(cfg.RestoreName, "rancher-restore")
	ns := firstNonEmpty(cfg.Namespace, "cattle-resources-system")
	return c.run(ctx, "get", "restore", name, "-n", ns, "-o", "yaml")
}

// ClusterInfo returns current context from kubeconfig.
func (c *Client) ClusterInfo(ctx context.Context) (string, error) {
	return c.run(ctx, "config", "current-context")
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
