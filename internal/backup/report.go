package backup

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// FormatReportBrief is the default stdout report (summary, inventory, reason counts).
func FormatReportBrief(res *Result) string {
	var b strings.Builder
	b.WriteString("Cluster inventory:\n")
	cids := sortedClusterIDs(res.Clusters)
	for _, cid := range cids {
		meta := res.Clusters[cid]
		flag := clusterFlag(res, cid)
		b.WriteString(fmt.Sprintf("  [%-14s] %-10s %-25s kind=%s\n", flag, cid, meta.DisplayName, meta.Kind))
	}

	if len(res.AutoOrphans) > 0 {
		b.WriteString("\nAuto-detected orphan cluster IDs (not in inventory):\n")
		for _, cid := range res.AutoOrphans {
			b.WriteString(fmt.Sprintf("  %s\n", cid))
		}
	}
	if len(res.ExplicitRemove) > 0 {
		b.WriteString("\nExplicit --remove-cluster IDs:\n")
		for _, cid := range res.ExplicitRemove {
			b.WriteString(fmt.Sprintf("  %s\n", cid))
		}
	}

	b.WriteString("\nRemoval summary:\n")
	for _, rc := range sortedReasonCounts(countReasons(res.Removed)) {
		b.WriteString(fmt.Sprintf("  %4d  %s\n", rc.Count, rc.Reason))
	}
	return b.String()
}

// FormatReport is the full report written to --report (includes every removed path).
func FormatReport(res *Result) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Input:  %s (%s)\n", res.InputPath, HumanSize(res.InputSize)))
	if res.OutputPath != "" && res.OutputSize > 0 {
		b.WriteString(fmt.Sprintf("Output: %s (%s)\n", res.OutputPath, HumanSize(res.OutputSize)))
	}
	b.WriteString(fmt.Sprintf("Elapsed: %.1fs (gzip level %d)\n", res.Elapsed.Seconds(), res.CompressLevel))
	b.WriteString(fmt.Sprintf("Clusters in backup: %d\n", len(res.Clusters)))
	b.WriteString(fmt.Sprintf("Fleet name mappings: %d\n", res.FleetMappings))
	b.WriteString("\n")
	b.WriteString(FormatReportBrief(res))
	b.WriteString(fmt.Sprintf("\nKept %d objects (%s uncompressed), removed %d objects\n",
		len(res.Kept), HumanSize(res.KeptBytes), len(res.Removed)))
	b.WriteString("\nRemoved paths:\n")
	removed := append([]RemovedEntry(nil), res.Removed...)
	sort.Slice(removed, func(i, j int) bool {
		if removed[i].Reason == removed[j].Reason {
			return removed[i].Path < removed[j].Path
		}
		return removed[i].Reason < removed[j].Reason
	})
	for _, e := range removed {
		b.WriteString(fmt.Sprintf("  [%s] %s\n", e.Reason, e.Path))
	}
	return b.String()
}

func clusterFlag(res *Result, cid string) string {
	if cid == "local" {
		return "REMOVE (always)"
	}
	if _, remove := res.RemoveIDs[cid]; remove {
		return "REMOVE"
	}
	return "KEEP"
}

func sortedClusterIDs(clusters map[string]ClusterMeta) []string {
	cids := make([]string, 0, len(clusters))
	for cid := range clusters {
		cids = append(cids, cid)
	}
	sort.Strings(cids)
	return cids
}

func WriteReport(res *Result, reportPath string) error {
	if reportPath == "" {
		return nil
	}
	return os.WriteFile(reportPath, []byte(FormatReport(res)), 0o644)
}

// FormatInspectBrief is stdout-friendly inspect output.
func FormatInspectBrief(in *InspectResult) string {
	var b strings.Builder
	b.WriteString("Cluster inventory:\n")
	for _, cid := range sortedClusterIDs(in.Clusters) {
		meta := in.Clusters[cid]
		b.WriteString(fmt.Sprintf("  %-10s %-25s kind=%s\n", cid, meta.DisplayName, meta.Kind))
	}
	if len(in.GhostIDs) > 0 {
		b.WriteString("\nGhost cluster IDs (in paths, no management definition):\n")
		keys := make([]string, 0, len(in.GhostIDs))
		for k := range in.GhostIDs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("  %s: %d paths\n", k, in.GhostIDs[k]))
		}
	} else {
		b.WriteString("\nGhost cluster IDs: none\n")
	}
	return b.String()
}

func FormatInspect(in *InspectResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Backup: %s (%s)\n", in.Path, HumanSize(in.InputSize)))
	b.WriteString(fmt.Sprintf("Tar members: %d\n", in.MemberCount))
	b.WriteString(fmt.Sprintf("Fleet name mappings: %d\n", in.FleetMappings))
	b.WriteString(fmt.Sprintf("fleet-default cluster JSON files: %d\n", in.FleetDefault))
	b.WriteString(fmt.Sprintf("local-cluster path references: %d\n", in.LocalArtifacts))
	b.WriteString("\n")
	b.WriteString(FormatInspectBrief(in))
	return b.String()
}
