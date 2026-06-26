# inspect

Read-only analysis of a Rancher backup tarball.

## Usage

```bash
rancher-polymorph inspect -i ./backups/source-full.tar.gz
rancher-polymorph inspect -i ./backups/source-full.tar.gz --tree
rancher-polymorph inspect -i ./backups/source-full.tar.gz --tree --keep-cluster c-xxxxx
```

## Output

- Member count and size
- Cluster inventory (ID, display name, kind: rke1 / imported / rke2)
- Ghost cluster IDs (paths referencing unknown cluster IDs)
- Local cluster artifact count

With `--tree`, prints a grouped keep/drop preview (same logic as sanitize).
