package backup

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

type RemovalMatcher struct {
	patterns   []clusterPattern
	removeIDs  map[string]struct{}
	fleetIndex *FleetIndex
}

type clusterPattern struct {
	id      string
	pattern *regexp.Regexp
}

func NewRemovalMatcher(removeIDs map[string]struct{}, fleetIndex *FleetIndex) *RemovalMatcher {
	ids := sortedKeys(removeIDs)
	patterns := make([]clusterPattern, 0, len(ids))
	for _, cid := range ids {
		escaped := regexp.QuoteMeta(cid)
		re := regexp.MustCompile(
			`(?:/` + escaped + `/|/` + escaped + `\.json|-` + escaped + `-|-` + escaped + `\.json|-` + escaped + `/|#` + escaped + `/|/` + escaped + `-|/` + escaped + `$)`,
		)
		patterns = append(patterns, clusterPattern{id: cid, pattern: re})
	}
	return &RemovalMatcher{
		patterns:   patterns,
		removeIDs:  removeIDs,
		fleetIndex: fleetIndex,
	}
}

func (m *RemovalMatcher) Reason(path string, doc map[string]any) string {
	if _, ok := LocalExactPaths[path]; ok {
		return LocalReason
	}
	if path == "authconfigs.management.cattle.io#v3/local.json" {
		return ""
	}
	for _, sub := range LocalSubstrings {
		if strings.Contains(path, sub) {
			return LocalReason
		}
	}
	if strings.HasSuffix(path, "/local/") || strings.HasSuffix(path, "/fleet-local/") {
		return LocalReason
	}

	for _, cp := range m.patterns {
		if cp.pattern.MatchString(path) {
			return "cluster " + cp.id
		}
	}

	if reason := m.fleetReason(path); reason != "" {
		return reason
	}
	if reason := m.secretReason(path); reason != "" {
		return reason
	}
	if doc != nil {
		if cid := clusterIDFromJSON(doc); cid != "" {
			if _, ok := m.removeIDs[cid]; ok {
				return "fleet cluster " + cid
			}
		}
	}
	return ""
}

func (m *RemovalMatcher) fleetReason(path string) string {
	matched := false
	for _, prefix := range FleetPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			matched = true
			break
		}
	}
	if !matched {
		return ""
	}

	basename := path
	if idx := strings.LastIndex(basename, "/"); idx >= 0 {
		basename = basename[idx+1:]
	}
	basename = strings.TrimSuffix(basename, "/")
	if strings.HasSuffix(basename, ".json") {
		basename = basename[:len(basename)-5]
	}

	for _, name := range m.fleetIndex.BundleNames(basename) {
		if cid := m.fleetIndex.ClusterForName(name); cid != "" {
			if _, ok := m.removeIDs[cid]; ok {
				return "fleet cluster " + cid
			}
		}
	}
	if cid := m.fleetIndex.ClusterForName(basename); cid != "" {
		if _, ok := m.removeIDs[cid]; ok {
			return "fleet cluster " + cid
		}
	}
	return ""
}

func (m *RemovalMatcher) secretReason(path string) string {
	const prefix = "secrets.#v1/cattle-system/c-c-"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, ".json") {
		return ""
	}
	cid := path[len(prefix) : len(path)-5]
	if _, ok := m.removeIDs[cid]; ok {
		return "cluster " + cid
	}
	return ""
}

func parseJSONIfNeeded(data []byte) map[string]any {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	return doc
}

func countReasons(removed []RemovedEntry) map[string]int {
	counts := make(map[string]int)
	for _, e := range removed {
		counts[e.Reason]++
	}
	return counts
}

func sortedReasonCounts(counts map[string]int) []struct {
	Reason string
	Count  int
} {
	type pair struct {
		Reason string
		Count  int
	}
	pairs := make([]pair, 0, len(counts))
	for reason, count := range counts {
		pairs = append(pairs, pair{reason, count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count == pairs[j].Count {
			return pairs[i].Reason < pairs[j].Reason
		}
		return pairs[i].Count > pairs[j].Count
	})
	out := make([]struct {
		Reason string
		Count  int
	}, len(pairs))
	for i, p := range pairs {
		out[i].Reason = p.Reason
		out[i].Count = p.Count
	}
	return out
}
