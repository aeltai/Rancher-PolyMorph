# rancher-migrate

CLI for sanitizing Rancher backup tarballs and **migrating Rancher to a new management cluster** (backup → sanitize → restore).

## Features

- **sanitize** — filter full backups to one cluster (or all RKE1); strip local cluster, Fleet ghosts, orphans
- **inspect** — read-only backup analysis; `--tree` previews keep/drop layout
- **ui** — interactive wizard: S3 pull, sanitize, full migration (source → sanitize → restore), restore watch
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

## TUI (`rancher-migrate ui`)

Configure `rancher-migrate.yaml` first (`config init`). The menu covers the full migration path:

| Menu item | What it does |
|-----------|----------------|
| **Full migration** | Pick **local file** or **S3** → inspect → cluster filter → sanitize → **Restore to cluster now** (kubectl cp + Restore CR + wait for Ready) |
| **Sanitize backup** | Local file only — inspect, filter, write sanitized tarball |
| **Pull from S3** | List `.tar.gz` in `s3.bucket` → download to `defaults.output_dir` → **enter** to sanitize |
| **Inspect only** | Read-only inventory tree |
| **Restore to cluster** | Point at an existing sanitized `.tar.gz` → copy to operator pod → apply Restore CR → poll until Ready |

**S3** requires `s3.bucket`, `region`, and optional `prefix` / `profile` in config.

**Restore** requires `restore.kubeconfig`, `operator_namespace`, `backup_pod_label`, and `backup_container_path` (see `config.example.yaml`). The TUI runs the same steps as `rancher-migrate restore run --local …` and watches the Restore CR until Ready.

After restore completes, install cert-manager and Rancher Helm on the target cluster, then reconnect downstream agents.

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
