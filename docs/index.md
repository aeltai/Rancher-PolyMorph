# Rancher PolyMorph

**Sanitize Rancher backup tarballs and migrate Rancher to a new management cluster.**

Rancher PolyMorph (`rancher-migrate`) filters full Rancher backups down to the downstream cluster(s) you want to keep, strips local-cluster artifacts that would conflict on restore, and drives the **rancher-backup** restore flow on a target cluster.

[:octicons-arrow-right-24: Getting started](getting-started.md){ .md-button .md-button--primary }
[:octicons-mark-github-16: GitHub](https://github.com/aeltai/Rancher-PolyMorph){ .md-button }

## Why this tool?

Moving Rancher management to a new cluster (backup → restore) requires a **sanitized** tarball:

- Full backups include **every** registered downstream cluster, Fleet mappings, and the **local** management cluster state.
- Restoring a full backup onto a fresh Rancher instance causes conflicts — local cluster objects, ghost Fleet IDs, and clusters you did not intend to migrate.
- Rancher PolyMorph builds an explicit **keep/drop plan**, previews it as a tree, and writes a restore-ready tarball.

## Features

| Feature | Description |
|---------|-------------|
| **sanitize** | Filter backup to one or more clusters; auto-detect orphan ghost IDs |
| **inspect** | Read-only inventory; `--tree` previews keep/drop layout |
| **restore** | `kubectl cp` + Restore CR + watch Ready |
| **s3** | Pull/push `.tar.gz` backups |
| **ui** | Interactive TUI — full migration wizard with S3 and restore watch |

## Quick example

```bash
git clone https://github.com/aeltai/Rancher-PolyMorph.git
cd Rancher-PolyMorph && make build

rancher-migrate inspect -i ./backups/source-full.tar.gz --tree
rancher-migrate sanitize \
  -i ./backups/source-full.tar.gz \
  -o ./backups/sanitized.tar.gz \
  --keep-cluster c-xxxxx \
  --report ./backups/report.txt

rancher-migrate restore run --local ./backups/sanitized.tar.gz
```

## Migration flow

```mermaid
flowchart LR
  A[Full backup] --> B[inspect / tree preview]
  B --> C[sanitize]
  C --> D[Sanitized tarball]
  D --> E[restore to target cluster]
  E --> F[Helm: cert-manager + Rancher]
  F --> G[Reconnect downstream agents]
```

See the [Migration guide](migration.md) for RKE1-specific notes.

## License

Apache-2.0 — see [LICENSE](https://github.com/aeltai/Rancher-PolyMorph/blob/main/LICENSE).
