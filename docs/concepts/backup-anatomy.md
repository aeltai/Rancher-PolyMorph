# Backup anatomy

Rancher backup tarballs are **gzip-compressed tar archives** of Kubernetes object snapshots taken by the rancher-backup operator.

## Member naming

Each tar member name encodes the resource:

```
<group>.<resource>#<version>/<namespace>/<name>.json
```

Examples:

```
clusters.management.cattle.io#v3/c-xxxxx.json
nodes.management.cattle.io#v3/c-xxxxx/m-node.json
clusters.fleet.cattle.io#v1alpha1/fleet-default/c-m-xxxxx.json
```

## Cluster kinds

| Kind | Detection |
|------|-----------|
| **rke1** | `spec.rancherKubernetesEngineConfig` present |
| **imported** | `spec.importedConfig` or `status.driver=imported` |
| **rke2-provisioned** | `spec.rke2Config` or `spec.k3sConfig` |
| **local** | management cluster running Rancher |

## Local cluster

The **local** cluster represents the Rancher management Kubernetes cluster itself.
On restore to a **new** cluster, local objects must be **stripped** — they are recreated by the new installation.

## Ghost IDs

Paths may reference cluster IDs that no longer have a `clusters.management.cattle.io` definition (detached clusters, stale Fleet mappings).
`sanitize` can auto-detect and remove these when `--keep-cluster` is set.
