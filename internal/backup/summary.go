package backup

import (
	"fmt"
	"strings"
)

func FormatSanitizeSummary(res *Result) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(lineBox("="))
	b.WriteString("  RANCHER BACKUP SANITIZE — SUMMARY\n")
	b.WriteString(lineBox("="))
	b.WriteString(fmt.Sprintf("  Input     %-38s %8s\n", truncPath(res.InputPath, 38), HumanSize(res.InputSize)))
	if res.OutputPath != "" {
		b.WriteString(fmt.Sprintf("  Output    %-38s %8s\n", truncPath(res.OutputPath, 38), HumanSize(res.OutputSize)))
	}
	b.WriteString(fmt.Sprintf("  Elapsed   %.1fs          gzip level %d\n", res.Elapsed.Seconds(), res.CompressLevel))
	b.WriteString(lineBox("-"))
	b.WriteString(fmt.Sprintf("  Kept      %6d objects  (%s uncompressed)\n", len(res.Kept), HumanSize(res.KeptBytes)))
	b.WriteString(fmt.Sprintf("  Removed   %6d objects\n", len(res.Removed)))
	b.WriteString(fmt.Sprintf("  Clusters  %6d in backup, %d fleet mappings\n", len(res.Clusters), res.FleetMappings))
	if len(res.AutoOrphans) > 0 {
		b.WriteString(fmt.Sprintf("  Orphans   auto-detected: %s\n", strings.Join(res.AutoOrphans, ", ")))
	}
	b.WriteString(lineBox("="))
	b.WriteString("\n")
	return b.String()
}

func FormatInspectSummary(in *InspectResult) string {
	ghostCount := len(in.GhostIDs)
	status := "CLEAN"
	if ghostCount > 0 || in.FleetDefault > len(in.Clusters) {
		status = "REVIEW"
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(lineBox("="))
	b.WriteString("  RANCHER BACKUP INSPECT — SUMMARY\n")
	b.WriteString(lineBox("="))
	b.WriteString(fmt.Sprintf("  Backup    %-38s %8s\n", truncPath(in.Path, 38), HumanSize(in.InputSize)))
	b.WriteString(fmt.Sprintf("  Members   %6d tar objects\n", in.MemberCount))
	b.WriteString(fmt.Sprintf("  Clusters  %6d management definitions\n", len(in.Clusters)))
	b.WriteString(fmt.Sprintf("  Fleet     %6d name mappings, %d fleet-default JSON\n", in.FleetMappings, in.FleetDefault))
	b.WriteString(fmt.Sprintf("  Local     %6d local-cluster path refs (unsanitized)\n", in.LocalArtifacts))
	b.WriteString(fmt.Sprintf("  Ghosts    %6d orphan cluster ID(s)     status: %s\n", ghostCount, status))
	b.WriteString(lineBox("="))
	b.WriteString("\n")
	return b.String()
}

func lineBox(ch string) string {
	return strings.Repeat(ch, 72) + "\n"
}

func truncPath(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return "..." + s[len(s)-(max-3):]
}
