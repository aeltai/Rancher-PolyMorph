#!/usr/bin/env bash
# Produce a sanitized Rancher backup tarball for migration restore.
#
# Prefers the Go CLI (rancher-polymorph), then Python, then legacy bash/tar.
#
# Usage:
#   ./scripts/sanitize-backup-for-restore.sh \
#     --input backups/source-full-backup.tar.gz \
#     --output backups/sanitized-single-cluster.tar.gz \
#     --keep-cluster c-aaaaa \
#     --report backups/sanitize-report.txt

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO_BIN="${REPO_ROOT}/bin/rancher-polymorph"
PY_SCRIPT="${SCRIPT_DIR}/sanitize-backup-for-restore.py"

usage() {
  cat <<'EOF'
Sanitize Rancher backup for migration restore.

Required:
  --input PATH           Full Rancher backup .tar.gz
  --output PATH          Sanitized output .tar.gz

Optional:
  --report PATH          Write human-readable removal report
  --keep-cluster ID      Keep only this downstream cluster (+ drops local)
  --keep-rke1-only       Remove imported/RKE2 clusters; keep all RKE1
  --remove-cluster ID    Explicit cluster ID to strip (repeatable; for orphan ghosts)
  --fast                 Faster gzip (level 1)
  --compress-level N     gzip level 1-9 (default: 3 in Python path)
  --no-auto-orphans      Disable auto orphan ID detection with --keep-cluster
  --quiet                Suppress progress on stderr
  --bash-only            Force slow bash/tar implementation (no Go/Python)

Build Go CLI: make build  → bin/rancher-polymorph
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

# Delegate to Go CLI or Python when available (recommended).
if [[ "${SANITIZE_BASH_ONLY:-}" != "1" ]]; then
  bash_only=false
  passthrough=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --bash-only) bash_only=true; shift ;;
      -h|--help) usage; exit 0 ;;
      *) passthrough+=("$1"); shift ;;
    esac
  done

  if [[ "$bash_only" == false ]]; then
    if [[ -x "$GO_BIN" ]]; then
      exec "$GO_BIN" sanitize "${passthrough[@]}"
    fi
    if [[ -f "$PY_SCRIPT" ]] && command -v python3 >/dev/null 2>&1; then
      exec python3 "$PY_SCRIPT" "${passthrough[@]}"
    fi
  fi

  set -- "${passthrough[@]}"
fi

INPUT=""
OUTPUT=""
REPORT=""
KEEP_CLUSTER=""
KEEP_RKE1_ONLY=false
REMOVE_CLUSTERS=()
QUIET=false

cluster_kind() {
  local json="$1"
  local cid="$2"
  if [[ "$cid" == "local" ]]; then
    echo "local"
    return
  fi
  if echo "$json" | jq -e '.spec.rancherKubernetesEngineConfig' >/dev/null 2>&1; then
    echo "rke1"
  elif echo "$json" | jq -e '.spec.importedConfig // .spec.genericEngineConfig' >/dev/null 2>&1; then
    echo "imported"
  elif echo "$json" | jq -e '.spec.rke2Config // .spec.k3sConfig' >/dev/null 2>&1; then
    echo "rke2-provisioned"
  else
    echo "unknown"
  fi
}

cluster_display_name() {
  echo "$1" | jq -r '.spec.displayName // empty'
}

id_in_remove_list() {
  local needle="$1"
  local rid
  for rid in "${REMOVE_IDS[@]}"; do
    [[ "$rid" == "$needle" ]] && return 0
  done
  return 1
}

should_remove_local_cluster_artifact() {
  local path="$1"

  [[ "$path" == authconfigs.management.cattle.io#v3/local.json ]] && return 1

  if echo "$path" | grep -qE '(^clusters\.management\.cattle\.io#v3/local\.json$)|(^namespaces\.#v1/(local|fleet-local|cattle-fleet-local-system)\.json$)|(^fleetworkspaces\.management\.cattle\.io#v3/fleet-local\.json$)|fleet-local(/|$)|#v3/local(/|$)|#v1/local(/|$)|cattle-fleet-local-system(/|$)|cattle-fleet-local-system-fleet-agent|local-fleet-local-owner|local-clusterowner'; then
    echo "local cluster (recreated on restore Rancher)"
    return 0
  fi
  return 1
}

should_remove() {
  local path="$1"
  local cid

  if reason="$(should_remove_local_cluster_artifact "$path")"; then
    echo "$reason"
    return 0
  fi

  for cid in "${REMOVE_IDS[@]}"; do
    if [[ "$path" == *"/${cid}/"* ]] \
      || [[ "$path" == *"/${cid}.json" ]] \
      || [[ "$path" == *"-${cid}-"* ]] \
      || [[ "$path" == *"-${cid}.json" ]] \
      || [[ "$path" == *"-${cid}/"* ]] \
      || [[ "$path" == *"#${cid}/"* ]] \
      || [[ "$path" == *"/${cid}-"* ]] \
      || [[ "$path" == */"$cid" ]]; then
      echo "cluster ${cid}"
      return 0
    fi
  done

  return 1
}

progress_tick() {
  local current="$1"
  local total="$2"
  local label="$3"
  [[ "$QUIET" == true ]] && return 0
  local pct=$((current * 100 / total))
  printf '\r%s %3d%% (%d/%d)' "$label" "$pct" "$current" "$total" >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --input) INPUT="${2:-}"; shift 2 ;;
    --output) OUTPUT="${2:-}"; shift 2 ;;
    --report) REPORT="${2:-}"; shift 2 ;;
    --keep-cluster) KEEP_CLUSTER="${2:-}"; shift 2 ;;
    --keep-rke1-only) KEEP_RKE1_ONLY=true; shift ;;
    --remove-cluster) REMOVE_CLUSTERS+=("${2:-}"); shift 2 ;;
    --quiet) QUIET=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1 (install python3 for full option support)" ;;
  esac
done

[[ -n "$INPUT" ]] || die "--input is required"
[[ -n "$OUTPUT" ]] || die "--output is required"
[[ -f "$INPUT" ]] || die "input not found: $INPUT"

need_cmd tar
need_cmd jq
need_cmd mktemp

if [[ -n "$KEEP_CLUSTER" && ${#REMOVE_CLUSTERS[@]} -gt 0 ]]; then
  die "use either --keep-cluster or --remove-cluster, not both"
fi

echo "warning: using legacy bash sanitizer (slow). Install python3 for progress + streaming." >&2

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

INVENTORY="$WORK/inventory.tsv"
KEPT_LIST="$WORK/kept.lst"
REMOVED_LIST="$WORK/removed.lst"
MEMBER_LIST="$WORK/members.lst"
: >"$INVENTORY"
: >"$KEPT_LIST"
: >"$REMOVED_LIST"

tar -tzf "$INPUT" >"$MEMBER_LIST"
total_members="$(wc -l <"$MEMBER_LIST" | tr -d ' ')"

idx=0
while IFS= read -r member; do
  [[ "$member" =~ ^clusters\.management\.cattle\.io#v3/[^/]+\.json$ ]] || continue
  cid="${member#clusters.management.cattle.io#v3/}"
  cid="${cid%.json}"
  json="$(tar -xOzf "$INPUT" "$member")"
  kind="$(cluster_kind "$json" "$cid")"
  display="$(cluster_display_name "$json")"
  [[ -n "$display" ]] || display="$cid"
  printf '%s\t%s\t%s\n' "$cid" "$display" "$kind" >>"$INVENTORY"
  idx=$((idx + 1))
  progress_tick "$idx" "$total_members" "Inventory"
done <"$MEMBER_LIST"
[[ "$QUIET" == false ]] && echo >&2

REMOVE_IDS=()
if [[ ${#REMOVE_CLUSTERS[@]} -gt 0 ]]; then
  REMOVE_IDS=("${REMOVE_CLUSTERS[@]}")
elif [[ -n "$KEEP_CLUSTER" ]]; then
  while IFS=$'\t' read -r cid _display _kind; do
    [[ "$cid" == "local" || "$cid" == "$KEEP_CLUSTER" ]] && continue
    REMOVE_IDS+=("$cid")
  done <"$INVENTORY"
elif [[ "$KEEP_RKE1_ONLY" == true ]]; then
  while IFS=$'\t' read -r cid _display kind; do
    [[ "$cid" == "local" ]] && continue
    case "$kind" in
      imported|rke2-provisioned|unknown) REMOVE_IDS+=("$cid") ;;
    esac
  done <"$INVENTORY"
fi

mkdir -p "$WORK/extract" "$(dirname "$OUTPUT")"

idx=0
while IFS= read -r member; do
  [[ -z "$member" ]] && continue
  idx=$((idx + 1))
  if reason="$(should_remove "$member")"; then
    printf '%s\t%s\n' "$member" "$reason" >>"$REMOVED_LIST"
  else
    printf '%s\n' "$member" >>"$KEPT_LIST"
    tar -xzf "$INPUT" -C "$WORK/extract" "$member"
  fi
  progress_tick "$idx" "$total_members" "Sanitize"
done <"$MEMBER_LIST"
[[ "$QUIET" == false ]] && echo >&2

COPYFILE_DISABLE=1 tar --no-recursion -czf "$OUTPUT" -C "$WORK/extract" -T "$KEPT_LIST"

kept_count="$(wc -l <"$KEPT_LIST" | tr -d ' ')"
removed_count="$(wc -l <"$REMOVED_LIST" | tr -d ' ')"
cluster_count="$(wc -l <"$INVENTORY" | tr -d ' ')"

REPORT_TEXT="$WORK/report.txt"
{
  echo "Input:  $INPUT"
  echo "Output: $OUTPUT"
  echo "Clusters in backup: $cluster_count"
  echo ""
  echo "Cluster inventory:"
  sort -t $'\t' -k1,1 "$INVENTORY" | while IFS=$'\t' read -r cid display kind; do
    flag="KEEP"
    if [[ "$cid" == "local" ]]; then
      flag="REMOVE (always)"
    elif id_in_remove_list "$cid"; then
      flag="REMOVE"
    fi
    printf '  [%-14s] %-10s %-25s kind=%s\n' "$flag" "$cid" "$display" "$kind"
  done
  echo ""
  echo "Kept ${kept_count} objects, removed ${removed_count} objects"
  echo ""
  echo "Removal summary:"
  cut -f2 "$REMOVED_LIST" | sort | uniq -c | sort -rn | while read -r count reason; do
    printf '  %4s  %s\n' "$count" "$reason"
  done
  echo ""
  echo "Removed paths:"
  sort -t $'\t' -k2,2 -k1,1 "$REMOVED_LIST" | while IFS=$'\t' read -r path reason; do
    printf '  [%s] %s\n' "$reason" "$path"
  done
} >"$REPORT_TEXT"

cat "$REPORT_TEXT"
if [[ -n "$REPORT" ]]; then
  mkdir -p "$(dirname "$REPORT")"
  cp "$REPORT_TEXT" "$REPORT"
fi
