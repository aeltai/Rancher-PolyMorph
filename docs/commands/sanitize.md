# sanitize

Filter a full Rancher backup tarball to keep only selected downstream cluster(s).

## Usage

```bash
rancher-polymorph sanitize \
  --input ./backups/source-full.tar.gz \
  --output ./backups/sanitized.tar.gz \
  --keep-cluster c-xxxxx \
  --report ./backups/report.txt
```

## Flags

| Flag | Description |
|------|-------------|
| `-i, --input` | Full backup `.tar.gz` (required) |
| `-o, --output` | Sanitized output path (required) |
| `--keep-cluster` | Keep this downstream cluster ID |
| `--keep-rke1-only` | Keep all RKE1; remove imported/RKE2 |
| `--remove-cluster` | Explicit cluster ID to strip (repeatable) |
| `--no-auto-orphans` | Disable orphan ghost auto-detection |
| `--fast` | gzip level 1 |
| `--compress-level` | gzip 1–9 (default 3) |
| `-r, --report` | Write full removal report |
| `--log-file` | Append timestamped log |

## What gets removed

- Other downstream clusters (management CRs, nodes, Fleet mappings)
- **Local cluster** artifacts (recreated on restore)
- Auto-detected **orphan** cluster IDs in paths without management definitions
- Fleet debris for removed clusters

See [Keep vs drop](../concepts/keep-drop.md).
