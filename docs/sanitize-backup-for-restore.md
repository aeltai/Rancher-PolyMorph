# Sanitize Rancher backup for restore (migrate to new cluster)

This document describes **`scripts/sanitize-backup-for-restore.py`** and the thin bash wrapper **`scripts/sanitize-backup-for-restore.sh`**.

Use these tools after taking a **full** Rancher backup on the source management cluster and **before** copying the tarball to a new restore target. The output is a smaller backup that contains only the downstream cluster(s) you want to migrate, plus global Rancher configuration (users, auth, settings).

Official Rancher background:

- [Backup, restore, and disaster recovery](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/backup-restore-and-disaster-recovery)
- [Migrate Rancher to a new cluster](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/backup-restore-and-disaster-recovery/migrate-rancher-to-new-cluster)
- [Restore configuration](https://ranchermanager.docs.rancher.com/reference-guides/backup-restore-configuration/restore-configuration)

---

## Why sanitize?

A full Rancher backup (`rancher-resource-set-full`) contains **every** downstream cluster registered on that Rancher server, plus the **local** management cluster object and Fleet metadata for all workspaces.

For **migrating Rancher to a new management cluster**, you typically want to:

1. Restore Rancher on the **new** cluster using the backup operator only (Rancher **not** running during restore).
2. Reconnect **one** downstream RKE1 cluster to the new Rancher URL.
3. Avoid ghost clusters, Fleet debris, and local-cluster conflicts on the destination.

The sanitizer **filters** the backup tarball: it streams each tar member, decides keep vs remove, and writes a new `.tar.gz`. It does **not** modify JSON contents — only inclusion/exclusion.

---

## Where this fits in the migration flow

```
Source Rancher                    Destination (restore target)
─────────────                     ───────────────────────────
1. Full backup (.tar.gz)
2. Sanitize locally      ──────►  3. Copy tarball to backup PVC
   (this script)                  4. Restore CR (prune: false)
                                  5. Install cert-manager + Rancher
                                  6. reconnect-rke1-agents.sh on nodes
```

See [Rancher: migrate to a new cluster](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/backup-restore-and-disaster-recovery/migrate-rancher-to-new-cluster).

---

## Scripts

| Script / binary | Use |
|-----------------|-----|
| **`bin/rancher-migrate`** | **Recommended Go CLI.** `sanitize`, `inspect`, `restore`, `s3`, `ui`. Build: `make build` |
| **`scripts/sanitize-backup-for-restore.py`** | Python implementation (parity with Go). Stdlib only. |
| **`scripts/sanitize-backup-for-restore.sh`** | Wrapper: Go → Python → legacy bash. |

### Requirements

- **Go CLI:** Go 1.22+ to build; no runtime deps after build
- **Python:** 3.9+ (stdlib only)
- **Bash fallback:** `tar`, `jq`, `mktemp`

---

## Quick start

### Build the Go CLI (recommended)

```bash
cd ~/rancher-migrate
make build
./bin/rancher-migrate --help
```

### Keep a single downstream cluster (most common)

```bash
cd ~/rancher-migrate

./bin/rancher-migrate sanitize \
  --input backups/source-full-backup.tar.gz \
  --output backups/sanitized-single-cluster.tar.gz \
  --keep-cluster c-aaaaa \
  --report backups/sanitize-report.txt
```

Or via the shell wrapper (auto-picks Go → Python):

```bash
./scripts/sanitize-backup-for-restore.sh \
  --input backups/source-full-backup.tar.gz \
  --output backups/sanitized-single-cluster.tar.gz \
  --keep-cluster c-aaaaa \
  --report backups/sanitize-report.txt
```

Python equivalent:

```bash
python3 scripts/sanitize-backup-for-restore.py \
  --input backups/source-full-backup.tar.gz \
  --output backups/sanitized-single-cluster.tar.gz \
  --keep-cluster c-aaaaa \
  --report backups/sanitize-report.txt \
  2> backups/sanitize-progress.log
```

Find the cluster ID in the Rancher UI (Cluster → Config) or from the report's **Cluster inventory** section after a dry run on the full backup.

### Inspect a backup (no changes)

```bash
./bin/rancher-migrate inspect --input backups/source-full-backup.tar.gz
```

### Generate Restore CR manifest

```bash
./bin/rancher-migrate restore plan \
  --name rancher-restore \
  --backup-file sanitized-single-cluster.tar.gz \
  --output manifests/restore.yaml
```

Apply on the destination **before** Rancher is installed (`prune: false`).

### Example (single RKE1 cluster)

```bash
python3 scripts/sanitize-backup-for-restore.py \
  --input ./backups/source-full-backup.tar.gz \
  --output ./backups/sanitized-single-cluster.tar.gz \
  --keep-cluster c-xxxxx \
  --report ./backups/sanitize-report.txt \
  2> ./backups/sanitize-progress.log
```

### Keep all RKE1 clusters, drop imported / RKE2-provisioned

```bash
python3 scripts/sanitize-backup-for-restore.py \
  --input backups/source-full-backup.tar.gz \
  --output backups/sanitized-rke1-only.tar.gz \
  --keep-rke1-only \
  --report backups/sanitize-report-rke1-only.txt
```

### Explicitly remove orphan cluster ghosts

Use when a cluster was deleted from Rancher but artifacts remain in the backup (no `clusters.management.cattle.io#v3/<id>.json`):

```bash
python3 scripts/sanitize-backup-for-restore.py \
  --input full-backup.tar.gz \
  --output sanitized.tar.gz \
  --keep-cluster c-xxxxx \
  --remove-cluster c-ghost1 \
  --report report.txt
```

With `--keep-cluster`, orphan IDs are **auto-detected** by default (see below). `--remove-cluster` adds extra IDs on top.

---

## CLI reference

### Required

| Flag | Description |
|------|-------------|
| `--input PATH` | Full Rancher backup `.tar.gz` from source |
| `--output PATH` | Sanitized output `.tar.gz` |

### Cluster selection (pick one strategy)

| Flag | Description |
|------|-------------|
| `--keep-cluster ID` | Keep **only** this downstream cluster. Removes all other clusters from inventory, all Fleet mappings except this ID, and **always** removes `local`. |
| `--keep-rke1-only` | Keep every cluster whose management spec has `rancherKubernetesEngineConfig` (RKE1). Remove imported, RKE2-provisioned, and unknown kinds. |
| `--remove-cluster ID` | Explicit cluster ID to strip. Repeatable. Used alone or **in addition to** `--keep-cluster` for orphan ghosts. |

If none of the above are given and `--remove-cluster` is empty, nothing is removed except **local** cluster artifacts.

### Optional

| Flag | Default | Description |
|------|---------|-------------|
| `--report PATH` | — | Write the full text report to a file (same content as stdout) |
| `--compress-level N` | `3` | gzip level 1–9 for output tarball |
| `--fast` | off | Shortcut for `--compress-level 1` (faster, larger output) |
| `--quiet` | off | Suppress progress bar and timing on stderr |
| `--no-auto-orphans` | off | Disable automatic orphan cluster ID detection when using `--keep-cluster` |

### Shell wrapper only

| Flag | Description |
|------|-------------|
| `--bash-only` | Force legacy bash/tar implementation (slow; no Fleet index) |

---

## How the script works

### Phase 1 — Index the backup

1. Open input `.tar.gz` and read the member list (one pass over tar headers).
2. **Cluster inventory** — parse every `clusters.management.cattle.io#v3/<id>.json`:
   - Detect kind: `rke1`, `imported`, `rke2-provisioned`, `local`, `unknown`
   - Record `displayName`
3. **Fleet index** — parse every `clusters.fleet.cattle.io#v1alpha1/fleet-default/<name>.json`:
   - Read label `management.cattle.io/cluster-name` → management cluster ID
   - Read label `management.cattle.io/cluster-display-name`
   - Map fleet object name and display name → cluster ID
4. Merge management `displayName` → cluster ID into the Fleet index.

### Phase 2 — Build remove list

Depending on flags:

| Mode | `remove_ids` contains |
|------|------------------------|
| `--keep-cluster X` | Every cluster in inventory except `X` and `local` |
| `--keep-rke1-only` | All `imported`, `rke2-provisioned`, `unknown` clusters |
| `--remove-cluster` | Explicit IDs (always added) |

**With `--keep-cluster` (default auto-orphan behaviour):**

1. Scan **all** tar paths for cluster ID patterns (`c-xxxxx`, `c-m-*`).
2. Any ID not in management inventory and not the kept cluster → **orphan** → add to `remove_ids`.
3. Every cluster ID from the Fleet index except the kept cluster → add to `remove_ids` (covers imported clusters referenced only by display name).

### Phase 3 — Stream and filter

For each tar member:

1. Compute removal **reason** (see matching rules below).
2. If removed → skip (file body is not read for path-only matches).
3. If kept → stream into output tar (directories/symlinks copied as-is).

Progress is printed to **stderr**. The detailed report goes to **stdout** and optionally `--report`.

---

## Matching rules (what gets removed)

Each tar member path is checked in order. First match wins.

### 1. Local cluster (always removed)

The **local** downstream cluster on the source Rancher must **not** be restored — the destination creates its own local cluster when Rancher is installed.

Removed by exact path, substring, or suffix, including:

- `clusters.management.cattle.io#v3/local.json`
- `fleet-local/`, `cattle-fleet-local-system/`, `#v3/local/`
- Fleet-local bundles, roles, rolebindings, namespaces, etc.

**Exception (kept):** `authconfigs.management.cattle.io#v3/local.json` — auth provider config, not the local cluster.

### 2. Path-based cluster ID match

For each ID in `remove_ids`, the path is matched if it contains patterns like:

- `/<id>/`, `/<id>.json`
- `-<id>-`, `-<id>.json`, `-<id>/`
- `#<id>/`, `/<id>-`, path ending in `/<id>`

Examples:

- `namespaces.#v1/c-bbbbb.json` → removed when `c-bbbbb` is in `remove_ids`
- `nodes.management.cattle.io#v3/c-xxxxx/m-abc.json` → kept when `c-xxxxx` is the kept cluster

### 3. Fleet / provisioning / bundle paths (display-name aware)

For paths under `fleet-default`:

- `clusters.fleet.cattle.io#v1alpha1/fleet-default/`
- `clusters.provisioning.cattle.io#v1/fleet-default/`
- `bundles.fleet.cattle.io#v1alpha1/fleet-default/`
- `clusterregistrationtokens.fleet.cattle.io#v1alpha1/fleet-default/`

The basename is resolved through the Fleet index:

| Path example | Resolved via |
|--------------|--------------|
| `.../fleet-default/my-cluster.json` | Fleet label → `c-m-example` |
| `.../fleet-default/c-xxxxx.json` | Direct ID |
| `.../fleet-agent-my-cluster.json` | Strip `fleet-agent-` prefix |
| `.../my-cluster-managed-system-agent.json` | Strip `-managed-system-agent` suffix |
| `.../mcc-my-cluster-managed-system-upgrade-controller.json` | Strip `mcc-` + `-managed-system-*` |

If the resolved cluster ID is in `remove_ids` → removed with reason `fleet cluster <id>`.

### 4. JSON content check (Fleet / provisioning)

For selected `.json` files under Fleet/provisioning prefixes, the file is parsed and label `management.cattle.io/cluster-name` is read. If that ID is in `remove_ids` → removed.

### 5. Orphan cattle-system secrets

`secrets.#v1/cattle-system/c-c-<clusterId>.json` → removed when `<clusterId>` is in `remove_ids`.

---

## What is kept

| Category | Kept? |
|----------|-------|
| Target downstream cluster(s) | Yes — nodes, projects, RBAC, etcd backups, registration tokens |
| Global users, tokens, authconfigs | Yes |
| Global roles, settings, CRDs | Yes |
| `fleet-default` namespace shell | Yes (if not cluster-specific) |
| Fleet objects for **kept** cluster only | Yes |
| Other downstream clusters | No |
| Local / fleet-local cluster | No |
| Auth provider `authconfigs.../local.json` | Yes |

---

## Output artifacts

### stderr (progress)

```
Reading backup index (1505 objects, 2.4 MB)…
Inventory done — 4 cluster(s), 3 fleet mapping(s) in 0.0s
Sanitize [############################] 100.0% (1505/1505, 0.2s)
Sanitize done — kept 1316, removed 189 in 0.2s
Output: backups/sanitized.tar.gz (2.7 MB, 0.2s total, gzip level 3)
```

Save with `2> progress.log`.

### stdout / `--report` (audit trail)

```
Input:  ... (size)
Output: ... (size)
Elapsed: 0.2s (gzip level 3)
Clusters in backup: 4
Fleet name mappings: 3

Cluster inventory:
  [REMOVE        ] c-bbbbb    rke2-workload-1           kind=rke2-provisioned
  [KEEP          ] c-aaaaa    rke1-target               kind=rke1
  [REMOVE (always)] local      local                     kind=rke2-provisioned

Auto-detected orphan cluster IDs (not in inventory):
  c-ghost1

Kept 1316 objects (15.3 MB uncompressed), removed 189 objects

Removal summary:
    68  cluster c-bbbbb
    52  local cluster (recreated on restore Rancher)
     1  fleet cluster c-ghost1

Removed paths:
  [cluster c-bbbbb] clusters.management.cattle.io#v3/c-bbbbb.json
  ...
```

**Always review** the report before restore:

- Only your target cluster(s) show **KEEP**
- `local` shows **REMOVE (always)**
- Removal summary has no unexpected cluster IDs
- For large environments, spot-check **Removed paths** for your kept cluster ID (should not appear)

---

## Performance

| Backup size | Objects | Typical time (Python, default gzip) |
|-------------|---------|-------------------------------------|
| ~2–3 MB (small) | ~1,500 | **< 1 s** |
| ~50–100 MB (large) | ~50,000 | **~10–30 s** |

The script is I/O bound. Use `--fast` for demos (gzip level 1). Output may be **larger** than input with `--fast` — that is normal.

---

## Validation checklist (before restore)

```bash
# Cluster definitions — should list only your target (+ nothing if local already stripped)
tar -tzf sanitized.tar.gz | grep '^clusters\.management\.cattle\.io#v3/.*\.json$'

# No local cluster
tar -tzf sanitized.tar.gz | grep -E 'fleet-local|#v3/local\.json' | head

# Fleet-default cluster JSONs — should be 0 or 1 per kept cluster
tar -tzf sanitized.tar.gz | grep 'clusters\.fleet\.cattle\.io#v1alpha1/fleet-default/.*\.json$'

# Ghost cluster IDs in paths (optional Python one-liner)
python3 -c "
import tarfile, re
from collections import Counter
p='sanitized.tar.gz'
with tarfile.open(p,'r:gz') as t:
    names=[m.name for m in t.getmembers()]
mgmt={n.split('/')[-1][:-5] for n in names if n.startswith('clusters.management.cattle.io#v3/') and n.endswith('.json')}
ids=Counter()
for n in names:
    for m in re.finditer(r'\b(c-m-[a-z0-9]+|c-[a-z0-9]{5})\b', n):
        ids[m.group(1)]+=1
ghosts={k:v for k,v in ids.items() if k not in mgmt}
print('mgmt:', sorted(mgmt))
print('ghosts:', ghosts or 'none')
"
```

---

## Post-sanitize restore reminder

After sanitizing:

1. Copy tarball to destination backup storage (e.g. `/var/lib/rancher-backups/`).
2. Ensure **Rancher is NOT running** on the destination.
3. Apply Restore CR with **`prune: false`** ([Rancher migration docs](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/backup-restore-and-disaster-recovery/migrate-rancher-to-new-cluster)).
4. Wait for Restore `Completed`.
5. Install cert-manager, then Rancher via Helm.
6. Run `scripts/reconnect-rke1-agents.sh` on each downstream RKE1 node.

---

## Limitations and gotchas

1. **Does not edit JSON** — only filters tar members. Downstream cluster specs still contain the **old** Rancher URL until agents are reconnected.
2. **Full backup required** — sanitize cannot invent objects missing from the source backup.
3. **Re-sanitizing an already-sanitized backup** works but always start from the **original full backup** when possible.
4. **Bash fallback** does not implement Fleet index or orphan auto-detection — use Python.
5. **Encrypted backups** — if the source backup used Rancher encryption, the same `EncryptionConfiguration` secret must be recreated on the destination ([backup configuration docs](https://ranchermanager.docs.rancher.com/reference-guides/backup-restore-configuration/backup-configuration)).
6. **Kubernetes distribution change** — migrating between K3s/RKE2 may require editing the local cluster object after restore (per Rancher docs); sanitizing removes the old local cluster so the new one is created fresh.
7. **`--keep-cluster` and `--remove-cluster` together** — explicit removes are **added** to the keep-cluster remove set (warning printed).

---

## Internal reference (code map)

| Module / symbol | Role |
|-----------------|------|
| `cluster_ids_from_members()` | Parse management cluster inventory |
| `build_fleet_index()` | Fleet display name → cluster ID map |
| `discover_orphan_cluster_ids()` | Path scan for IDs missing from inventory |
| `RemovalMatcher` | Path, Fleet, secret, and JSON-based removal decisions |
| `ProgressBar` | stderr progress with ETA |
| `LOCAL_EXACT_PATHS` / `LOCAL_SUBSTRINGS` | Local cluster artifact patterns |
| `FLEET_PATH_PREFIXES` | fleet-default path roots |
| `CLUSTER_ID_RE` | Orphan detection regex |

---

## Related files

| Path | Purpose |
|------|---------|
| `bin/rancher-migrate` | Go CLI binary (build with `make build`) |
| `cmd/`, `internal/`, `main.go` | Go CLI source |
| `scripts/sanitize-backup-for-restore.py` | Python implementation |
| `scripts/sanitize-backup-for-restore.sh` | Wrapper / legacy fallback |
| `scripts/reconnect-rke1-agents.sh` | Post-restore agent cutover |
| `backups/sanitize-report-*.txt` | Local sanitize reports (not committed) |
