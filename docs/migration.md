# Migration guide

This guide covers the **backup → sanitize → restore** path for moving Rancher management to a new cluster while keeping selected downstream clusters.

## Overview

| Step | Tool / action |
|------|----------------|
| 1 | Full backup on **source** Rancher (rancher-backup operator) |
| 2 | `rancher-migrate inspect --tree` — review inventory |
| 3 | `rancher-migrate sanitize --keep-cluster <id>` |
| 4 | `rancher-migrate restore run` on **target** cluster |
| 5 | Install cert-manager + Rancher Helm on target |
| 6 | Reconnect downstream cluster agents to new Rancher URL |

## RKE1 provisioned clusters

Rancher-provisioned **RKE1** clusters cannot be cleanly “detached” like imported clusters.
The supported migration patterns are:

1. **Dual Rancher (backup/restore)** — sanitize backup to keep RKE1 cluster definitions; restore on new mgmt; point agents at new URL.
2. **Per-cluster import** — export projects/RBAC, detach/cleanup, import on target (separate runbook).

`rancher-migrate` focuses on pattern **1**.

!!! note
    After restore, downstream RKE1 nodes still run Kubernetes — you must update
    `CATTLE_SERVER` / re-run registration so agents connect to the **new** Rancher URL.

## S3 workflow

If backups live in S3:

```bash
rancher-migrate s3 pull migrations/source-full.tar.gz -o ./backups/in.tar.gz
rancher-migrate sanitize -i ./backups/in.tar.gz -o ./backups/out.tar.gz --keep-cluster c-xxxxx
rancher-migrate s3 push ./backups/out.tar.gz migrations/sanitized.tar.gz
```

Or use the TUI: **Full migration → Download from S3**.

## Post-restore checklist

- [ ] Restore CR status **Ready**
- [ ] cert-manager installed on target local cluster
- [ ] Rancher Helm installed (matching your target version policy)
- [ ] DNS / ingress points to new Rancher URL
- [ ] Downstream agents reconnected
- [ ] Fleet / GitRepo repos reconciled

## Further reading

- [sanitize-backup-for-restore.md](https://github.com/aeltai/rancher-migrate/blob/main/docs/sanitize-backup-for-restore.md) — detailed sanitize semantics
- [Keep vs drop](concepts/keep-drop.md) — what gets removed
