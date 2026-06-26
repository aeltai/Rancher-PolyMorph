#!/usr/bin/env python3
"""Produce a sanitized Rancher backup for migration to a new management cluster.

Full documentation: docs/sanitize-backup-for-restore.md

Removes imported/provisioned RKE2 cluster ghosts, the local cluster object, and
dependent Fleet/RBAC metadata. Keeps RKE1 custom clusters and global auth/RBAC.

Fleet/provisioning objects keyed by display name are resolved via JSON labels
(management.cattle.io/cluster-name) and stripped with the rest of the cluster.

Usage:
  ./scripts/sanitize-backup-for-restore.py \\
    --input backups/source-full-backup.tar.gz \\
    --output backups/sanitized-single-cluster.tar.gz \\
    --keep-cluster c-aaaaa \\
    --remove-cluster c-ghost1 \\
    --report backups/sanitize-report.txt
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import tarfile
import time
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path


LOCAL_EXACT_PATHS = frozenset(
    {
        "clusters.management.cattle.io#v3/local.json",
        "namespaces.#v1/local.json",
        "namespaces.#v1/fleet-local.json",
        "namespaces.#v1/cattle-fleet-local-system.json",
        "fleetworkspaces.management.cattle.io#v3/fleet-local.json",
        "clusterroles.rbac.authorization.k8s.io#v1/local-clusterowner.json",
        "clusterrolebindings.rbac.authorization.k8s.io#v1/cattle-fleet-local-system-fleet-agent-role-binding.json",
        "clusterroles.rbac.authorization.k8s.io#v1/cattle-fleet-local-system-fleet-agent-role.json",
    }
)

LOCAL_SUBSTRINGS = (
    "fleet-local/",
    "fleet-local",
    "#v3/local/",
    "roles.rbac.authorization.k8s.io#v1/local/",
    "rolebindings.rbac.authorization.k8s.io#v1/local/",
    "roles.rbac.authorization.k8s.io#v1/fleet-local/",
    "rolebindings.rbac.authorization.k8s.io#v1/fleet-local/",
    "configmaps.#v1/cattle-fleet-local-system/",
    "deployments.apps#v1/cattle-fleet-local-system/",
    "configmaps.#v1/fleet-local/",
    "nodes.management.cattle.io#v3/local/",
    "projects.management.cattle.io#v3/local/",
    "clusterroletemplatebindings.management.cattle.io#v3/local/",
    "clusterregistrationtokens.management.cattle.io#v3/local/",
    "clusterregistrationtokens.fleet.cattle.io#v1alpha1/fleet-local/",
    "clusters.fleet.cattle.io#v1alpha1/fleet-local/",
    "clusters.provisioning.cattle.io#v1/fleet-local/",
    "bundles.fleet.cattle.io#v1alpha1/fleet-local/",
    "clustergroups.fleet.cattle.io#v1alpha1/fleet-local/",
    "clusterregistrations.fleet.cattle.io#v1alpha1/fleet-local/",
)

CLUSTER_PREFIX = "clusters.management.cattle.io#v3/"
FLEET_CLUSTER_PREFIX = "clusters.fleet.cattle.io#v1alpha1/fleet-default/"
LOCAL_REASON = "local cluster (recreated on restore Rancher)"

CLUSTER_ID_RE = re.compile(r"\b(c-m-[a-z0-9]+|c-[a-z0-9]{5})\b")

FLEET_PATH_PREFIXES = (
    "clusters.fleet.cattle.io#v1alpha1/fleet-default/",
    "clusters.provisioning.cattle.io#v1/fleet-default/",
    "bundles.fleet.cattle.io#v1alpha1/fleet-default/",
    "clusterregistrationtokens.fleet.cattle.io#v1alpha1/fleet-default/",
)

JSON_CLUSTER_PREFIXES = (
    "clusters.fleet.cattle.io#v1alpha1/",
    "clusters.provisioning.cattle.io#v1/",
    "bundles.fleet.cattle.io#v1alpha1/fleet-default/",
)


def human_size(num_bytes: int) -> str:
    value = float(num_bytes)
    for unit in ("B", "KB", "MB", "GB"):
        if value < 1024 or unit == "GB":
            return f"{value:.1f} {unit}" if unit != "B" else f"{int(value)} B"
        value /= 1024
    return f"{value:.1f} GB"


class ProgressBar:
    """Simple stderr progress bar (stdlib only, no tqdm dependency)."""

    def __init__(self, total: int, label: str, *, enabled: bool = True) -> None:
        self.total = max(total, 1)
        self.label = label
        self.enabled = enabled
        self.current = 0
        self.start = time.monotonic()
        self._last_render = 0.0
        self._width = 28

    def advance(self, step: int = 1) -> None:
        self.current = min(self.current + step, self.total)
        now = time.monotonic()
        if not self.enabled:
            return
        if now - self._last_render < 0.08 and self.current < self.total:
            return
        self._render(now)
        self._last_render = now

    def _render(self, now: float) -> None:
        ratio = self.current / self.total
        filled = int(self._width * ratio)
        bar = "#" * filled + "-" * (self._width - filled)
        elapsed = now - self.start
        rate = self.current / elapsed if elapsed > 0 else 0.0
        eta = (self.total - self.current) / rate if rate > 0 and self.current < self.total else 0.0
        eta_text = f", ETA {eta:.0f}s" if eta > 0 else ""
        print(
            f"\r{self.label} [{bar}] {ratio * 100:5.1f}% "
            f"({self.current}/{self.total}, {elapsed:.1f}s{eta_text})",
            end="",
            file=sys.stderr,
            flush=True,
        )

    def finish(self, message: str) -> None:
        if self.enabled:
            print(f"\r{message:<100}", file=sys.stderr, flush=True)


@dataclass
class FleetIndex:
    """Maps Fleet display names / object names → management cluster ID."""

    name_to_cluster: dict[str, str] = field(default_factory=dict)
    display_to_cluster: dict[str, str] = field(default_factory=dict)

    def cluster_for_name(self, name: str) -> str | None:
        if name in self.name_to_cluster:
            return self.name_to_cluster[name]
        return self.display_to_cluster.get(name)

    def bundle_names(self, basename: str) -> list[str]:
        """Extract candidate fleet/provisioning names from a bundle filename."""
        names = [basename]
        if basename.startswith("fleet-agent-"):
            names.append(basename[len("fleet-agent-") :])
        if basename.endswith("-managed-system-agent"):
            names.append(basename[: -len("-managed-system-agent")])
        if basename.startswith("mcc-") and "-managed-system-" in basename:
            names.append(basename[4:].split("-managed-system-", 1)[0])
        return names


def build_fleet_index(tar: tarfile.TarFile, members: list[tarfile.TarInfo]) -> FleetIndex:
    index = FleetIndex()
    for member in members:
        if not member.name.startswith(FLEET_CLUSTER_PREFIX) or not member.name.endswith(".json"):
            continue
        raw = tar.extractfile(member)
        if not raw:
            continue
        doc = json.load(raw)
        labels = doc.get("metadata", {}).get("labels") or {}
        cluster_id = labels.get("management.cattle.io/cluster-name")
        display = labels.get("management.cattle.io/cluster-display-name")
        fleet_name = doc.get("metadata", {}).get("name")
        if not cluster_id:
            continue
        if fleet_name:
            index.name_to_cluster[fleet_name] = cluster_id
        if display:
            index.display_to_cluster[display] = cluster_id
        index.name_to_cluster[cluster_id] = cluster_id
    return index


def merge_inventory_display_names(index: FleetIndex, clusters: dict[str, dict]) -> None:
    for cid, meta in clusters.items():
        if cid == "local":
            continue
        index.display_to_cluster[meta["displayName"]] = cid
        index.name_to_cluster[cid] = cid


def discover_orphan_cluster_ids(
    names: list[str], keep_cluster: str | None, mgmt_cluster_ids: set[str]
) -> set[str]:
    """Find cluster IDs referenced in paths but missing from management inventory."""
    orphans: set[str] = set()
    for path in names:
        for match in CLUSTER_ID_RE.finditer(path):
            cid = match.group(1)
            if cid == "local":
                continue
            if keep_cluster and cid == keep_cluster:
                continue
            if cid not in mgmt_cluster_ids:
                orphans.add(cid)
    return orphans


def cluster_id_from_json(doc: dict) -> str | None:
    labels = doc.get("metadata", {}).get("labels") or {}
    if cid := labels.get("management.cattle.io/cluster-name"):
        return cid
    return None


@dataclass(frozen=True)
class RemovalMatcher:
    cluster_patterns: tuple[tuple[str, re.Pattern[str]], ...]
    remove_ids: frozenset[str]
    fleet_index: FleetIndex

    @classmethod
    def build(cls, remove_ids: set[str], fleet_index: FleetIndex) -> RemovalMatcher:
        patterns: list[tuple[str, re.Pattern[str]]] = []
        for cid in sorted(remove_ids):
            escaped = re.escape(cid)
            pattern = re.compile(
                rf"(?:/{escaped}/|/{escaped}\.json|-{escaped}-|-{escaped}\.json|"
                rf"-{escaped}/|#{escaped}/|/{escaped}-|/{escaped}$)"
            )
            patterns.append((cid, pattern))
        return cls(
            cluster_patterns=tuple(patterns),
            remove_ids=frozenset(remove_ids),
            fleet_index=fleet_index,
        )

    def _fleet_reason(self, path: str) -> str | None:
        if not any(path.startswith(prefix) for prefix in FLEET_PATH_PREFIXES):
            return None

        basename = path.rstrip("/").split("/")[-1]
        if basename.endswith(".json"):
            basename = basename[:-5]

        candidates = self.fleet_index.bundle_names(basename)
        for name in candidates:
            cid = self.fleet_index.cluster_for_name(name)
            if cid and cid in self.remove_ids:
                return f"fleet cluster {cid}"

        cid = self.fleet_index.cluster_for_name(basename)
        if cid and cid in self.remove_ids:
            return f"fleet cluster {cid}"
        return None

    def _secret_reason(self, path: str) -> str | None:
        # secrets.#v1/cattle-system/c-c-<clusterId>.json
        prefix = "secrets.#v1/cattle-system/c-c-"
        if not path.startswith(prefix) or not path.endswith(".json"):
            return None
        cid = path[len(prefix) : -5]
        if cid in self.remove_ids:
            return f"cluster {cid}"
        return None

    def reason(self, path: str, doc: dict | None = None) -> str | None:
        if path in LOCAL_EXACT_PATHS:
            return LOCAL_REASON
        if path == "authconfigs.management.cattle.io#v3/local.json":
            return None
        if any(sub in path for sub in LOCAL_SUBSTRINGS):
            return LOCAL_REASON
        if path.endswith("/local/") or path.endswith("/fleet-local/"):
            return LOCAL_REASON

        for cid, pattern in self.cluster_patterns:
            if pattern.search(path):
                return f"cluster {cid}"

        fleet_reason = self._fleet_reason(path)
        if fleet_reason:
            return fleet_reason

        secret_reason = self._secret_reason(path)
        if secret_reason:
            return secret_reason

        if doc is not None:
            if cid := cluster_id_from_json(doc):
                if cid in self.remove_ids:
                    return f"fleet cluster {cid}"

        return None


def cluster_ids_from_members(
    tar: tarfile.TarFile, members: list[tarfile.TarInfo]
) -> dict[str, dict]:
    clusters: dict[str, dict] = {}
    for member in members:
        if not member.name.startswith(CLUSTER_PREFIX) or not member.name.endswith(".json"):
            continue
        cid = member.name[len(CLUSTER_PREFIX) : -5]
        raw = tar.extractfile(member)
        if not raw:
            continue
        doc = json.load(raw)
        spec = doc.get("spec", {})
        if spec.get("rancherKubernetesEngineConfig"):
            kind = "rke1"
        elif spec.get("importedConfig") or spec.get("genericEngineConfig"):
            kind = "imported"
        elif spec.get("rke2Config") or spec.get("k3sConfig"):
            kind = "rke2-provisioned"
        elif cid == "local":
            kind = "local"
        else:
            kind = "unknown"
        clusters[cid] = {
            "displayName": spec.get("displayName", cid),
            "kind": kind,
            "internal": bool(spec.get("internal")),
        }
    return clusters


def should_parse_json(path: str) -> bool:
    return path.endswith(".json") and any(path.startswith(p) for p in JSON_CLUSTER_PREFIXES)


def open_gz_tar(path: Path, mode: str, compresslevel: int) -> tarfile.TarFile:
    if "w" in mode:
        return tarfile.open(path, mode, compresslevel=compresslevel)
    return tarfile.open(path, mode)


def main() -> int:
    parser = argparse.ArgumentParser(description="Sanitize Rancher backup for migration restore")
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--report", type=Path, help="Write human-readable removal report")
    parser.add_argument(
        "--remove-cluster",
        action="append",
        default=[],
        help="Cluster ID to strip (repeatable). Use for orphan ghosts not in inventory.",
    )
    parser.add_argument(
        "--keep-cluster",
        metavar="CLUSTER_ID",
        help="Keep only this downstream cluster (removes all other non-local clusters).",
    )
    parser.add_argument(
        "--keep-rke1-only",
        action="store_true",
        default=False,
        help="Remove RKE2/imported clusters; keep all RKE1 clusters.",
    )
    parser.add_argument(
        "--no-auto-orphans",
        action="store_true",
        help="Do not auto-detect orphan cluster IDs in paths when using --keep-cluster.",
    )
    parser.add_argument(
        "--compress-level",
        type=int,
        default=3,
        metavar="1-9",
        help="gzip level for output (default: 3).",
    )
    parser.add_argument(
        "--fast",
        action="store_true",
        help="Shortcut for --compress-level 1 (fastest write, larger output).",
    )
    parser.add_argument(
        "--quiet",
        action="store_true",
        help="Suppress progress output on stderr.",
    )
    args = parser.parse_args()

    if args.keep_cluster and args.remove_cluster:
        print("warning: --remove-cluster adds extra IDs on top of --keep-cluster", file=sys.stderr)

    if not args.input.is_file():
        print(f"Input not found: {args.input}", file=sys.stderr)
        return 1

    compresslevel = 1 if args.fast else max(1, min(9, args.compress_level))
    input_size = args.input.stat().st_size
    t0 = time.monotonic()

    with tarfile.open(args.input, "r:gz") as src:
        members = src.getmembers()
        member_count = len(members)
        all_names = [m.name for m in members]

        if not args.quiet:
            print(
                f"Reading backup index ({member_count} objects, {human_size(input_size)})…",
                file=sys.stderr,
            )

        clusters = cluster_ids_from_members(src, members)
        fleet_index = build_fleet_index(src, members)
        merge_inventory_display_names(fleet_index, clusters)

        if not args.quiet:
            print(
                f"Inventory done — {len(clusters)} cluster(s), "
                f"{len(fleet_index.name_to_cluster)} fleet mapping(s) "
                f"in {time.monotonic() - t0:.1f}s",
                file=sys.stderr,
            )

        remove_ids = set(args.remove_cluster)
        if args.keep_cluster:
            for cid in clusters:
                if cid in ("local", args.keep_cluster):
                    continue
                remove_ids.add(cid)
        elif args.keep_rke1_only and not remove_ids:
            for cid, meta in clusters.items():
                if cid == "local":
                    continue
                if meta["kind"] in ("imported", "rke2-provisioned", "unknown"):
                    remove_ids.add(cid)

        auto_orphans: set[str] = set()
        if args.keep_cluster and not args.no_auto_orphans:
            auto_orphans = discover_orphan_cluster_ids(
                all_names, args.keep_cluster, set(clusters.keys())
            )
            remove_ids.update(auto_orphans)
            # Fleet mappings reference imported clusters by c-m-* IDs not always in inventory.
            for cid in set(fleet_index.name_to_cluster.values()):
                if cid not in ("local", args.keep_cluster):
                    remove_ids.add(cid)

        matcher = RemovalMatcher.build(remove_ids, fleet_index)
        kept: list[str] = []
        removed: list[tuple[str, str]] = []
        kept_bytes = 0

        args.output.parent.mkdir(parents=True, exist_ok=True)
        sanitize_progress = ProgressBar(member_count, "Sanitize", enabled=not args.quiet)

        with open_gz_tar(args.output, "w:gz", compresslevel) as dst:
            for member in members:
                doc: dict | None = None
                if should_parse_json(member.name) and member.isfile():
                    raw = src.extractfile(member)
                    if raw:
                        try:
                            doc = json.load(raw)
                            raw.seek(0)
                        except json.JSONDecodeError:
                            doc = None
                            raw.seek(0)

                reason = matcher.reason(member.name, doc)
                if reason:
                    removed.append((member.name, reason))
                elif member.isdir() or member.issym() or member.islnk():
                    dst.addfile(member)
                    kept.append(member.name)
                else:
                    src_file = src.extractfile(member)
                    if src_file is None:
                        removed.append((member.name, "unreadable member"))
                    else:
                        dst.addfile(member, src_file)
                        kept.append(member.name)
                        kept_bytes += member.size
                sanitize_progress.advance()

        sanitize_progress.finish(
            f"Sanitize done — kept {len(kept)}, removed {len(removed)} "
            f"in {time.monotonic() - t0:.1f}s"
        )

    output_size = args.output.stat().st_size
    elapsed = time.monotonic() - t0
    if not args.quiet:
        print(
            f"Output: {args.output} ({human_size(output_size)}, "
            f"{elapsed:.1f}s total, gzip level {compresslevel})",
            file=sys.stderr,
        )

    counts = Counter(reason for _, reason in removed)
    lines = [
        f"Input:  {args.input} ({human_size(input_size)})",
        f"Output: {args.output} ({human_size(output_size)})",
        f"Elapsed: {elapsed:.1f}s (gzip level {compresslevel})",
        f"Clusters in backup: {len(clusters)}",
        f"Fleet name mappings: {len(fleet_index.name_to_cluster)}",
        "",
        "Cluster inventory:",
    ]
    for cid, meta in sorted(clusters.items()):
        flag = "REMOVE" if cid in remove_ids else "KEEP"
        if cid == "local":
            flag = "REMOVE (always)"
        lines.append(f"  [{flag:14}] {cid:10} {meta['displayName']:25} kind={meta['kind']}")

    if auto_orphans:
        lines.extend(["", "Auto-detected orphan cluster IDs (not in inventory):"])
        for cid in sorted(auto_orphans):
            lines.append(f"  {cid}")

    explicit_orphans = set(args.remove_cluster) - auto_orphans
    if explicit_orphans:
        lines.extend(["", "Explicit --remove-cluster IDs:"])
        for cid in sorted(explicit_orphans):
            lines.append(f"  {cid}")

    lines.extend(
        [
            "",
            f"Kept {len(kept)} objects ({human_size(kept_bytes)} uncompressed), "
            f"removed {len(removed)} objects",
            "",
            "Removal summary:",
        ]
    )
    for reason, n in counts.most_common():
        lines.append(f"  {n:4d}  {reason}")
    lines.append("")
    lines.append("Removed paths:")
    for path, reason in sorted(removed):
        lines.append(f"  [{reason}] {path}")

    report_text = "\n".join(lines) + "\n"
    print(report_text)
    if args.report:
        args.report.write_text(report_text)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
