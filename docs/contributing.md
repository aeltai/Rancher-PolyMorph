# Contributing

Contributions welcome! This project is licensed under **Apache-2.0**.

## Development setup

```bash
git clone https://github.com/aeltai/Rancher-PolyMorph.git
cd Rancher-PolyMorph
make build
make test
make test-cover
```

## Pull requests

1. Fork and create a feature branch
2. Add tests for behavior changes
3. Run `make test` and `make lint`
4. Update docs in `docs/` if user-facing behavior changes
5. Open a PR against `main`

## Docs site

```bash
pip install -r docs/requirements.txt
mkdocs serve
```

Docs deploy automatically to GitHub Pages on push to `main`.

## Code layout

| Path | Purpose |
|------|---------|
| `cmd/` | Cobra CLI commands |
| `internal/backup/` | Sanitize, inspect, tree, inventory |
| `internal/config/` | YAML configuration |
| `internal/k8s/` | kubectl restore helpers |
| `internal/s3store/` | S3 client |
| `internal/tui/` | Bubble Tea wizard |

## Security

Never commit backup tarballs, kubeconfigs, or credentials. See `.gitignore`.
