# Keep vs drop

When you run `sanitize --keep-cluster c-xxxxx`, `rancher-polymorph` classifies every tar member:

| Disposition | Meaning |
|-------------|---------|
| **KEEP** | Retained in output tarball |
| **DROP** | Removed |
| **STRIP** | Local cluster — always removed on sanitize |
| **PRESENT** | In backup inventory (inspect-only mode) |

## Always stripped

- Local cluster management CR and related objects
- Fleet-local namespace artifacts
- Node/project RBAC under `local/`

## Kept for selected cluster

- Global Rancher settings and auth configs (except local-specific)
- Management CR, nodes, secrets for kept cluster ID
- Fleet mappings for kept cluster

## Dropped

- All other downstream cluster definitions and objects
- Fleet bundles for removed clusters
- Orphan ghost paths (when auto-orphans enabled)

## Preview before running

```bash
rancher-polymorph inspect -i full.tar.gz --tree --keep-cluster c-xxxxx
```

Or use the TUI tree preview (before/after tabs).
