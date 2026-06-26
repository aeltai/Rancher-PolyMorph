# restore

Copy a sanitized tarball to the rancher-backup operator pod and apply a Restore CR.

## Prerequisites

- Target cluster kubeconfig in config (`restore.kubeconfig`)
- rancher-backup operator running in `operator_namespace`
- Sanitized backup `.tar.gz` on local disk

## Usage

```bash
rancher-polymorph restore run --local ./backups/sanitized.tar.gz
rancher-polymorph restore status
```

## What it does

1. Finds backup operator pod by label
2. `kubectl cp` tarball into `backup_container_path`
3. Applies Restore CR with `prune: false`
4. Watches until Ready (or timeout)

After restore completes, install cert-manager and Rancher Helm manually.
