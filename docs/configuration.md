# Configuration

Configuration file: `rancher-migrate.yaml`

Search order:

1. `$RANCHER_MIGRATE_CONFIG`
2. `./rancher-migrate.yaml` (current directory)
3. `~/.config/rancher-migrate/rancher-migrate.yaml`
4. `~/.rancher-migrate.yaml`

## Initialize

```bash
rancher-migrate config init
```

## Example

```yaml
defaults:
  keep_cluster: "c-xxxxx"
  keep_rke1_only: false
  fast: true
  output_dir: ./backups
  report_dir: ./backups
  log_dir: ./backups
  auto_orphans: true
  compress_level: 3

restore:
  kubeconfig: ~/.kube/target-rancher.yaml
  context: ""
  namespace: cattle-resources-system
  operator_namespace: cattle-resources-system
  backup_pod_label: app.kubernetes.io/name=rancher-backup
  backup_container_path: /var/lib/rancher-backup
  restore_name: rancher-restore
  encryption_secret: ""
  storage_secret: ""
  watch_timeout: 30m

s3:
  region: eu-central-1
  bucket: my-rancher-backups
  prefix: migrations/
  profile: default
  endpoint: ""

ui:
  animations: true
  theme: default
```

## Key fields

| Section | Field | Purpose |
|---------|-------|---------|
| `defaults` | `keep_cluster` | Default `--keep-cluster` for sanitize/TUI |
| `defaults` | `keep_rke1_only` | Keep all RKE1 clusters; drop imported/RKE2 |
| `defaults` | `auto_orphans` | Auto-remove ghost cluster IDs found in paths |
| `restore` | `kubeconfig` | Target cluster for `restore run` |
| `restore` | `backup_pod_label` | Label to find rancher-backup operator pod |
| `s3` | `bucket`, `prefix` | S3 pull/push defaults |

See [config.example.yaml](https://github.com/aeltai/rancher-migrate/blob/main/config.example.yaml) in the repository.
