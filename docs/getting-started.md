# Getting started

## Prerequisites

- Go 1.22+ (to build from source)
- `kubectl` configured for the **target** management cluster
- **rancher-backup** operator installed on the target (the TUI restore flow expects it)
- A full Rancher backup `.tar.gz` from the **source** management cluster

!!! warning "Support bundles are not backups"
    Rancher **support bundles** (collector output) cannot be sanitized or restored.
    Use a backup created via the rancher-backup operator or Rancher UI **Backup** feature.

## Install

=== "From source"

    ```bash
    git clone https://github.com/aeltai/Rancher-PolyMorph.git
    cd Rancher-PolyMorph
    make build
    ./bin/rancher-polymorph --version
    ```

=== "Release binary"

    Download `rancher-polymorph` for your platform from
    [GitHub Releases](https://github.com/aeltai/Rancher-PolyMorph/releases).

## First run

```bash
# Write default config
./bin/rancher-polymorph config init

# Interactive wizard (recommended)
./bin/rancher-polymorph ui

# Or inspect a backup first
./bin/rancher-polymorph inspect -i /path/to/full-backup.tar.gz
```

## Configure

Edit `~/.config/rancher-polymorph/rancher-polymorph.yaml` (see [Configuration](configuration.md)):

- `defaults.keep_cluster` — default cluster ID for sanitize/TUI
- `restore.kubeconfig` — target cluster kubeconfig
- `s3.*` — optional S3 bucket for pull/push

## Next steps

1. [Inspect your backup](commands/inspect.md) and review the tree
2. [Sanitize](commands/sanitize.md) to a single-cluster tarball
3. [Restore](commands/restore.md) on the target cluster
4. Install cert-manager and Rancher Helm on the target after restore completes
