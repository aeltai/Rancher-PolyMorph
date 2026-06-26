# rancher-migrate

CLI for sanitizing Rancher backup tarballs and **migrating Rancher to a new management cluster** (backup → sanitize → restore).

## Features

- **sanitize** — filter full backups to one cluster (or all RKE1); strip local cluster, Fleet ghosts, orphans
- **inspect** — read-only backup analysis
- **ui** — interactive wizard with ASCII splash
- **s3** — pull/push backup tarballs
- **restore** — copy tarball to operator pod, apply Restore CR via kubeconfig
- **config** — defaults in `rancher-migrate.yaml`

## Quick start

```bash
git clone https://github.com/aeltai/rancher-migrate.git
cd rancher-migrate
make build

./bin/rancher-migrate config init
./bin/rancher-migrate ui

# Or non-interactive:
./bin/rancher-migrate inspect -i ./backups/source-full.tar.gz
./bin/rancher-migrate sanitize \
  -i ./backups/source-full.tar.gz \
  -o ./backups/sanitized.tar.gz \
  --keep-cluster c-xxxxx \
  --report ./backups/report.txt
```

## Migration flow

1. Full backup on source Rancher
2. `rancher-migrate sanitize` → single-cluster tarball
3. `rancher-migrate restore run --local ./backups/sanitized.tar.gz` (target kubeconfig in config)
4. Install cert-manager + Rancher Helm after restore completes
5. Reconnect downstream RKE1 agents

See [docs/sanitize-backup-for-restore.md](docs/sanitize-backup-for-restore.md).

## Configuration

```bash
rancher-migrate config init   # ~/.config/rancher-migrate/rancher-migrate.yaml
```

See [config.example.yaml](config.example.yaml) for `defaults`, `restore.kubeconfig`, and `s3` settings.

## S3

```bash
rancher-migrate s3 pull migrations/source-full.tar.gz -o ./backups/in.tar.gz
rancher-migrate s3 push ./backups/sanitized.tar.gz migrations/out.tar.gz
```

## Data hygiene

**Never commit** backup tarballs, sanitize logs, kubeconfigs, or credentials. The `.gitignore` blocks common patterns; keep production backups local only.

## Build

```bash
make build          # → bin/rancher-migrate
make test
make man-view       # man page
```

## License

Apache-2.0 (or your choice — add LICENSE if needed)
