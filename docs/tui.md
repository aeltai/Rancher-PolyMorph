# TUI wizard

The interactive UI (`rancher-polymorph ui`) guides you through backup inspection, cluster selection, sanitization, and optional restore.

## Screens

1. **Menu** — pick workflow
2. **Source select** — local file or S3 (full migration)
3. **Inspect view** — inventory tree with local cluster marked `STRIP`
4. **Cluster select** — multi-select clusters to keep
5. **Tree preview** — before/after keep/drop (`tab` to switch)
6. **Sanitize progress** — live progress bar
7. **Post-sanitize** — restore now or finish
8. **Restore running** — kubectl cp → Restore CR → watch Ready

## Keys

| Key | Action |
|-----|--------|
| `j` / `k` | Move selection / tree row |
| `enter` | Confirm / expand tree group |
| `tab` | Before/after tree (done screen) |
| `esc` | Back to menu |
| `q` | Quit |

## S3 pull → sanitize

After **Pull from S3**, press **enter** on the done screen to start sanitize on the downloaded file.

## Requirements

- `s3.bucket` configured for S3 flows
- `restore.kubeconfig` configured for restore flows
