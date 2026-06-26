package backup

import (
	"strings"
)

func (f *FleetIndex) ClusterForName(name string) string {
	if cid, ok := f.NameToCluster[name]; ok {
		return cid
	}
	return f.DisplayToCluster[name]
}

func (f *FleetIndex) BundleNames(basename string) []string {
	names := []string{basename}
	if after, ok := strings.CutPrefix(basename, "fleet-agent-"); ok {
		names = append(names, after)
	}
	if before, ok := strings.CutSuffix(basename, "-managed-system-agent"); ok {
		names = append(names, before)
	}
	if after, ok := strings.CutPrefix(basename, "mcc-"); ok {
		if idx := strings.Index(after, "-managed-system-"); idx > 0 {
			names = append(names, after[:idx])
		}
	}
	return names
}

func MergeInventoryDisplayNames(index *FleetIndex, clusters map[string]ClusterMeta) {
	for cid, meta := range clusters {
		if cid == "local" {
			continue
		}
		index.DisplayToCluster[meta.DisplayName] = cid
		index.NameToCluster[cid] = cid
	}
}
