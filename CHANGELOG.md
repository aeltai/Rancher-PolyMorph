# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.2] - 2026-06-26

### Changed

- CLI binary renamed from `rancher-migrate` to `rancher-polymorph`
- Config defaults to `~/.config/rancher-polymorph/rancher-polymorph.yaml` (legacy paths still supported)
- TUI splash and header ASCII updated to **RANCHER / POLYMORPH**
- Go module path: `github.com/aeltai/rancher-polymorph`

## [0.0.1] - 2026-06-26

### Added

- Initial open-source release under Apache-2.0
- `sanitize` — filter Rancher backup tarballs to selected downstream cluster(s)
- `inspect` — read-only backup inventory with optional `--tree` preview
- `restore` — copy tarball to rancher-backup operator pod and apply Restore CR
- `s3` — pull/push backup tarballs from S3
- `ui` — interactive TUI wizard (full migration, S3 pull, sanitize, restore watch)
- Multi-cluster keep support and orphan ghost auto-detection
- Before/after backup tree visualization in TUI and CLI
- GitHub Pages documentation site
- CI workflow (test, lint, build) and release workflow

[0.0.2]: https://github.com/aeltai/Rancher-PolyMorph/releases/tag/v0.0.2
[0.0.1]: https://github.com/aeltai/Rancher-PolyMorph/releases/tag/v0.0.1
