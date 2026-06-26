# Rancher PolyMorph

[![CI](https://github.com/aeltai/Rancher-PolyMorph/actions/workflows/ci.yml/badge.svg)](https://github.com/aeltai/Rancher-PolyMorph/actions/workflows/ci.yml)
[![Docs](https://github.com/aeltai/Rancher-PolyMorph/actions/workflows/docs.yml/badge.svg)](https://github.com/aeltai/Rancher-PolyMorph/actions/workflows/docs.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Release](https://img.shields.io/github/v/release/aeltai/Rancher-PolyMorph)](https://github.com/aeltai/Rancher-PolyMorph/releases)

**Rancher PolyMorph** is a CLI (`rancher-migrate`) for sanitizing Rancher backup tarballs and **migrating Rancher to a new management cluster** (backup → sanitize → restore).

📖 **Documentation:** [docs/](docs/) · [https://aeltai.github.io/Rancher-PolyMorph/](https://aeltai.github.io/Rancher-PolyMorph/)

## Features

- **sanitize** — filter full backups to one cluster (or all RKE1); strip local cluster, Fleet ghosts, orphans
- **inspect** — read-only backup analysis; `--tree` previews keep/drop layout
- **ui** — interactive wizard: S3 pull, sanitize, full migration (source → sanitize → restore), restore watch
- **s3** — pull/push backup tarballs
- **restore** — copy tarball to operator pod, apply Restore CR via kubeconfig
- **config** — defaults in `rancher-migrate.yaml`

## Quick start

```bash
git clone https://github.com/aeltai/Rancher-PolyMorph.git
cd Rancher-PolyMorph
make build

./bin/rancher-migrate config init
./bin/rancher-migrate ui

# Or non-interactive:
./bin/rancher-migrate inspect -i ./backups/source-full.tar.gz --tree
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

See the [migration guide](https://aeltai.github.io/Rancher-PolyMorph/migration/) and [sanitize reference](docs/sanitize-backup-for-restore.md).

## Configuration

```bash
rancher-migrate config init   # ~/.config/rancher-migrate/rancher-migrate.yaml
```

See [config.example.yaml](config.example.yaml) and [configuration docs](https://aeltai.github.io/Rancher-PolyMorph/configuration/).

## Development

```bash
make build
make test
make test-cover
make lint
make docs-serve   # local docs at http://127.0.0.1:8000
```

## Data hygiene

**Never commit** backup tarballs, sanitize logs, kubeconfigs, or credentials. The `.gitignore` blocks common patterns; keep production backups local only.

## License

Apache-2.0 — see [LICENSE](LICENSE).
