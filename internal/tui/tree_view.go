package tui

import (
	"fmt"
	"strings"

	"github.com/aeltai/rancher-polymorph/internal/backup"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

type treeTab int

const (
	treeTabBefore treeTab = iota
	treeTabAfter
)

func newTreeViewport(width, height int) viewport.Model {
	vp := viewport.New(max(20, width-4), max(8, height-14))
	vp.Style = lipgloss.NewStyle()
	return vp
}

func refreshTreeViewport(vp *viewport.Model, preview *backup.PreviewResult, expanded map[string]bool, width, groupCursor int) {
	if preview == nil {
		vp.SetContent("(no preview)")
		return
	}
	vp.SetContent(formatTreeContent(preview, expanded, width, groupCursor))
}

// treeGroupHeaderLine returns the line index of a group's header in FormatTreeLines output.
func treeGroupHeaderLine(preview *backup.PreviewResult, expanded map[string]bool, groupIndex int) int {
	if preview == nil || groupIndex < 0 || groupIndex >= len(preview.Groups) {
		return -1
	}
	line := 3 // title, stats, blank
	for i, g := range preview.Groups {
		if i == groupIndex {
			return line
		}
		line++
		if expanded[g.Key] {
			line += len(g.ResourceTypes) + len(g.SamplePaths)
		}
	}
	return -1
}

func formatTreeContent(preview *backup.PreviewResult, expanded map[string]bool, width, groupCursor int) string {
	lines := backup.FormatTreeLines(preview, expanded, width)
	if preview == nil || len(lines) == 0 {
		return strings.Join(lines, "\n")
	}

	selectedLine := treeGroupHeaderLine(preview, expanded, groupCursor)
	out := make([]string, len(lines))
	for i, line := range lines {
		if i == selectedLine {
			// Visible cursor + inverted row so position is obvious when scrolling.
			marker := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("▸ ")
			body := strings.TrimSpace(strings.TrimPrefix(line, "▸"))
			body = strings.TrimSpace(strings.TrimPrefix(body, "▾"))
			out[i] = treeRowSelectedStyle.Render(marker + body)
			continue
		}
		out[i] = line
	}
	return strings.Join(out, "\n")
}

func renderTreeHeader(preview *backup.PreviewResult, tab treeTab, showTabs bool, groupCursor int) string {
	if preview == nil {
		return boxStyle.Render("Backup tree")
	}
	title := "Backup tree"
	if preview.Mode == "inventory" {
		title = "Backup inventory"
	}
	var b strings.Builder
	b.WriteString(boxStyle.Render(title))
	b.WriteString("\n")
	if showTabs {
		before := "  before "
		after := "  after "
		if tab == treeTabBefore {
			before = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")).Render("▸ before ")
			after = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  after ")
		} else {
			before = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  before ")
			after = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")).Render("▸ after ")
		}
		b.WriteString(before + after + "\n")
	}
	b.WriteString(fmtLegendFor(preview))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  members %d  ·  ", preview.MemberCount))
	b.WriteString(okStyle.Render(fmt.Sprintf("keep %d", preview.TotalKept)))
	b.WriteString("  ·  ")
	b.WriteString(warnStyle.Render(fmt.Sprintf("drop %d", preview.TotalRemoved)))
	if groupCursor >= 0 && groupCursor < len(preview.Groups) {
		g := preview.Groups[groupCursor]
		b.WriteString("  ·  ")
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("selected: %s", g.Label)))
	}
	return b.String()
}

func fmtLegend() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"  📦 in backup  🟠 strip on sanitize  🟢 keep  🔴 drop  ·  enter expand  j/k group  ↑↓ scroll",
	)
}

func fmtLegendFor(preview *backup.PreviewResult) string {
	if preview != nil && preview.Mode == "inventory" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"  📦 in backup  🟠 local stripped on sanitize  ·  enter expand  j/k group  ↑↓ scroll",
		)
	}
	return fmtLegend()
}

func renderClusterPicker(
	clusters map[string]backup.ClusterMeta,
	selected map[string]bool,
	cursor int,
	width int,
) string {
	ids := selectableClusterIDs(clusters)
	var b strings.Builder
	b.WriteString(boxStyle.Render("Select clusters to keep"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("space toggle · enter preview tree · a all · n none · j/k move"))
	b.WriteString("\n\n")

	for i, cid := range ids {
		meta := clusters[cid]
		mark := "[ ]"
		if selected[cid] {
			mark = okStyle.Render("[✓]")
		}
		line := fmt.Sprintf("%s %-10s %-20s %s", mark, cid, meta.DisplayName, meta.Kind)
		if i == cursor {
			line = "▸ " + line
			b.WriteString(truncate(clusterRowSelectedStyle.Render(line), width))
		} else {
			b.WriteString(truncate("  "+line, width))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	visible := lipgloss.Width(s)
	if visible <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	// Trim runes until visible width fits (ANSI-aware).
	for len(s) > 0 && lipgloss.Width(s) > width {
		s = s[:len(s)-1]
	}
	return s + "…"
}

func selectableClusterIDs(clusters map[string]backup.ClusterMeta) []string {
	ids := sortedIDs(clusters)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "local" {
			out = append(out, id)
		}
	}
	return out
}

func toggleGroupExpand(expanded map[string]bool, preview *backup.PreviewResult, cursor int) {
	if preview == nil || cursor < 0 || cursor >= len(preview.Groups) {
		return
	}
	key := preview.Groups[cursor].Key
	expanded[key] = !expanded[key]
}

func scrollTreeToGroup(vp *viewport.Model, preview *backup.PreviewResult, expanded map[string]bool, cursor, width int) {
	if preview == nil {
		return
	}
	line := treeGroupHeaderLine(preview, expanded, cursor)
	if line < 0 {
		return
	}
	// Keep selected row near vertical center of viewport.
	target := line - vp.Height/3
	if target < 0 {
		target = 0
	}
	maxOff := max(0, len(backup.FormatTreeLines(preview, expanded, width))-vp.Height)
	if target > maxOff {
		target = maxOff
	}
	vp.YOffset = target
}
