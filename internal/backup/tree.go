package backup

import (
	"fmt"
	"sort"
	"strings"
)

// ResourceBucket groups tar members by API resource prefix.
type ResourceBucket struct {
	Prefix string
	Count  int
}

// TreeGroup is a cluster (or global/local) bucket in the backup tree.
type TreeGroup struct {
	Key           string
	Label         string
	DisplayName   string
	Kind          string
	Disposition   string // "keep" or "remove"
	Count         int
	ResourceTypes []ResourceBucket
	SamplePaths   []string
}

// PreviewResult is a dry-run classification of backup members.
type PreviewResult struct {
	InputPath    string
	MemberCount  int
	Groups       []TreeGroup
	TotalKept    int
	TotalRemoved int
	RemoveIDs    map[string]struct{}
	AutoOrphans  []string
	Mode         string // "inventory" or "sanitize"
}

// PreviewSanitize classifies every tar member as kept or removed without writing output.
func PreviewSanitize(opts Options) (*PreviewResult, error) {
	headers, err := readAllHeaders(opts.Input)
	if err != nil {
		return nil, err
	}

	allNames := make([]string, len(headers))
	for i, h := range headers {
		allNames[i] = h.Name
	}

	clusters, fleetIndex, err := buildInventory(opts.Input, headers)
	if err != nil {
		return nil, err
	}

	removeIDs, autoOrphans := buildRemovePlan(opts, clusters, fleetIndex, allNames)
	matcher := NewRemovalMatcher(removeIDs, fleetIndex)
	keepSet := keepSetFromOpts(opts)

	buckets := make(map[string]*TreeGroup)

	for _, hdr := range headers {
		path := hdr.Name
		reason := matcher.Reason(path, nil)
		removed := reason != ""
		key, label, display, kind := classifyMember(path, reason, removed, clusters, removeIDs, keepSet)

		g, ok := buckets[key]
		if !ok {
			disp := "keep"
			if removed {
				disp = "remove"
			}
			g = &TreeGroup{
				Key:         key,
				Label:       label,
				DisplayName: display,
				Kind:        kind,
				Disposition: disp,
			}
			buckets[key] = g
		}
		g.Count++
		if len(g.SamplePaths) < 3 {
			g.SamplePaths = append(g.SamplePaths, path)
		}
		prefix := resourcePrefix(path)
		merged := false
		for i := range g.ResourceTypes {
			if g.ResourceTypes[i].Prefix == prefix {
				g.ResourceTypes[i].Count++
				merged = true
				break
			}
		}
		if !merged {
			g.ResourceTypes = append(g.ResourceTypes, ResourceBucket{Prefix: prefix, Count: 1})
		}
	}

	groups := make([]TreeGroup, 0, len(buckets))
	var kept, removed int
	for _, g := range buckets {
		sort.Slice(g.ResourceTypes, func(i, j int) bool {
			if g.ResourceTypes[i].Count == g.ResourceTypes[j].Count {
				return g.ResourceTypes[i].Prefix < g.ResourceTypes[j].Prefix
			}
			return g.ResourceTypes[i].Count > g.ResourceTypes[j].Count
		})
		groups = append(groups, *g)
		if g.Disposition == "keep" {
			kept += g.Count
		} else {
			removed += g.Count
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		return treeGroupOrder(groups[i]) < treeGroupOrder(groups[j])
	})

	return &PreviewResult{
		InputPath:    opts.Input,
		MemberCount:  len(headers),
		Groups:       groups,
		TotalKept:    kept,
		TotalRemoved: removed,
		RemoveIDs:    removeIDs,
		AutoOrphans:  autoOrphans,
		Mode:         "sanitize",
	}, nil
}

// PreviewFromResult builds an "after sanitize" tree from kept members only.
func PreviewFromResult(res *Result) *PreviewResult {
	if res == nil {
		return nil
	}

	buckets := make(map[string]*TreeGroup)
	for _, path := range res.Kept {
		key, label, display, kind := classifyMember(path, "", false, res.Clusters, res.RemoveIDs, nil)
		g := ensureBucket(buckets, key, label, display, kind, "keep")
		addToBucket(g, path)
	}

	groups := sortedTreeGroups(buckets)
	total := len(res.Kept)
	return &PreviewResult{
		InputPath:    res.OutputPath,
		MemberCount:  total,
		Groups:       groups,
		TotalKept:    total,
		TotalRemoved: 0,
		RemoveIDs:    res.RemoveIDs,
		AutoOrphans:  res.AutoOrphans,
	}
}

func ensureBucket(buckets map[string]*TreeGroup, key, label, display, kind, disp string) *TreeGroup {
	g, ok := buckets[key]
	if !ok {
		g = &TreeGroup{Key: key, Label: label, DisplayName: display, Kind: kind, Disposition: disp}
		buckets[key] = g
	}
	return g
}

func addToBucket(g *TreeGroup, path string) {
	g.Count++
	if len(g.SamplePaths) < 3 {
		g.SamplePaths = append(g.SamplePaths, path)
	}
	prefix := resourcePrefix(path)
	for i := range g.ResourceTypes {
		if g.ResourceTypes[i].Prefix == prefix {
			g.ResourceTypes[i].Count++
			return
		}
	}
	g.ResourceTypes = append(g.ResourceTypes, ResourceBucket{Prefix: prefix, Count: 1})
}

func sortedTreeGroups(buckets map[string]*TreeGroup) []TreeGroup {
	groups := make([]TreeGroup, 0, len(buckets))
	for _, g := range buckets {
		sort.Slice(g.ResourceTypes, func(i, j int) bool {
			if g.ResourceTypes[i].Count == g.ResourceTypes[j].Count {
				return g.ResourceTypes[i].Prefix < g.ResourceTypes[j].Prefix
			}
			return g.ResourceTypes[i].Count > g.ResourceTypes[j].Count
		})
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return treeGroupOrder(groups[i]) < treeGroupOrder(groups[j])
	})
	return groups
}

func treeGroupOrder(g TreeGroup) int {
	switch {
	case strings.HasPrefix(g.Key, "cluster:"):
		if g.Disposition == "keep" {
			return 10
		}
		return 20
	case g.Key == "global":
		return 5
	case g.Key == "local":
		return 30
	case strings.HasPrefix(g.Key, "ghost:"):
		return 25
	default:
		return 40
	}
}

func classifyMember(path, reason string, removed bool, clusters map[string]ClusterMeta, removeIDs, keepSet map[string]struct{}) (key, label, display, kind string) {
	if removed {
		if reason == LocalReason || strings.Contains(reason, "local") {
			return "local", "local", "local", "local"
		}
		if strings.HasPrefix(reason, "cluster ") {
			cid := strings.TrimPrefix(reason, "cluster ")
			meta := clusters[cid]
			return "cluster:" + cid, cid, meta.DisplayName, meta.Kind
		}
		if strings.HasPrefix(reason, "fleet cluster ") {
			cid := strings.TrimPrefix(reason, "fleet cluster ")
			if _, ok := clusters[cid]; ok {
				meta := clusters[cid]
				return "cluster:" + cid, cid, meta.DisplayName, meta.Kind
			}
			return "ghost:" + cid, cid, "(orphan)", "ghost"
		}
	}

	for _, match := range clusterIDRe.FindAllString(path, -1) {
		if match == "local" {
			continue
		}
		if _, rm := removeIDs[match]; rm {
			meta := clusters[match]
			disp, k := match, meta.Kind
			if meta.DisplayName != "" {
				disp = meta.DisplayName
			}
			if _, ok := clusters[match]; ok {
				return "cluster:" + match, match, disp, k
			}
			return "ghost:" + match, match, "(orphan)", "ghost"
		}
		if len(keepSet) > 0 {
			if _, keep := keepSet[match]; keep {
				meta := clusters[match]
				return "cluster:" + match, match, meta.DisplayName, meta.Kind
			}
		} else if meta, ok := clusters[match]; ok && match != "local" {
			return "cluster:" + match, match, meta.DisplayName, meta.Kind
		}
	}

	return "global", "global", "Rancher global config", "global"
}

func resourcePrefix(path string) string {
	if idx := strings.Index(path, "#"); idx >= 0 {
		rest := path[idx+1:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return path[:idx+1] + rest[:slash]
		}
		return path[:idx]
	}
	if slash := strings.Index(path, "/"); slash >= 0 {
		return path[:slash]
	}
	return path
}

// FormatTreeLines renders a preview tree for CLI or TUI (plain text).
func FormatTreeLines(p *PreviewResult, expanded map[string]bool, maxWidth int) []string {
	if p == nil {
		return nil
	}
	if maxWidth < 40 {
		maxWidth = 80
	}
	var lines []string
	if p.Mode == "inventory" {
		lines = append(lines, fmt.Sprintf("Backup inventory — %d members", p.MemberCount))
		lines = append(lines, fmt.Sprintf("  clusters %d  ·  global %d  ·  local strip %d",
			p.countClusterObjects(), p.countGlobal(), p.TotalRemoved))
		if g := p.countGhosts(); g > 0 {
			lines[len(lines)-1] += fmt.Sprintf("  ·  ghosts %d", g)
		}
	} else {
		lines = append(lines, fmt.Sprintf("Backup tree — %d members", p.MemberCount))
		lines = append(lines, fmt.Sprintf("  keep %d  ·  remove %d", p.TotalKept, p.TotalRemoved))
	}
	lines = append(lines, "")

	for _, g := range p.Groups {
		lines = append(lines, formatTreeGroup(g, expanded, maxWidth, 0)...)
	}
	return lines
}

func formatTreeGroup(g TreeGroup, expanded map[string]bool, maxWidth, depth int) []string {
	indent := strings.Repeat("  ", depth)
	icon, tag := treeIconTag(g.Disposition)
	exp := expanded[g.Key]
	caret := "▸"
	if exp {
		caret = "▾"
	}
	head := fmt.Sprintf("%s%s %s %s · %s · %s [%d]", indent, caret, icon, g.Label, g.DisplayName, tag, g.Count)
	if g.Kind != "" && g.Kind != "global" {
		head += " · " + g.Kind
	}
	var lines []string
	lines = append(lines, truncateLine(head, maxWidth))
	if !exp {
		return lines
	}
	for _, rt := range g.ResourceTypes {
		lines = append(lines, truncateLine(fmt.Sprintf("%s    ├─ %s (%d)", indent, rt.Prefix, rt.Count), maxWidth))
	}
	for _, sample := range g.SamplePaths {
		lines = append(lines, truncateLine(fmt.Sprintf("%s    └─ %s", indent, sample), maxWidth))
	}
	return lines
}

func truncateLine(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func treeIconTag(disposition string) (icon, tag string) {
	switch disposition {
	case "strip":
		return "🟠", "STRIP"
	case "present":
		return "📦", "IN BACKUP"
	case "remove":
		return "🔴", "DROP"
	default:
		return "🟢", "KEEP"
	}
}

func (p *PreviewResult) countGhosts() int {
	if p == nil {
		return 0
	}
	n := 0
	for _, g := range p.Groups {
		if strings.HasPrefix(g.Key, "ghost:") {
			n += g.Count
		}
	}
	return n
}

func (p *PreviewResult) countGlobal() int {
	if p == nil {
		return 0
	}
	for _, g := range p.Groups {
		if g.Key == "global" {
			return g.Count
		}
	}
	return 0
}

func (p *PreviewResult) countClusterObjects() int {
	if p == nil {
		return 0
	}
	n := 0
	for _, g := range p.Groups {
		if strings.HasPrefix(g.Key, "cluster:") {
			n += g.Count
		}
	}
	return n
}
