package backup

import (
	"sort"
	"strings"
)

// IsLocalPath reports whether a tar member belongs to the management/local cluster
// and would be stripped during sanitize.
func IsLocalPath(path string) bool {
	if _, ok := LocalExactPaths[path]; ok {
		return true
	}
	if path == "authconfigs.management.cattle.io#v3/local.json" {
		return false
	}
	for _, sub := range LocalSubstrings {
		if strings.Contains(path, sub) {
			return true
		}
	}
	if strings.HasSuffix(path, "/local/") || strings.HasSuffix(path, "/fleet-local/") {
		return true
	}
	return false
}

// BuildInspectTree groups every tar member by cluster / local / global for read-only inspect.
func BuildInspectTree(path string) (*PreviewResult, error) {
	headers, err := readAllHeaders(path)
	if err != nil {
		return nil, err
	}

	clusters, _, err := buildInventory(path, headers)
	if err != nil {
		return nil, err
	}

	buckets := make(map[string]*TreeGroup)
	var localStrip int

	for _, hdr := range headers {
		p := hdr.Name
		key, label, display, kind, disp := classifyInspectPath(p, clusters)
		g := ensureBucket(buckets, key, label, display, kind, disp)
		addToBucket(g, p)
		if disp == "strip" {
			localStrip++
		}
	}

	groups := sortedTreeGroups(buckets)
	sort.Slice(groups, func(i, j int) bool {
		return inspectTreeOrder(groups[i]) < inspectTreeOrder(groups[j])
	})

	var clusterObjs, globalObjs, ghostObjs int
	for _, g := range groups {
		switch {
		case g.Key == "global":
			globalObjs += g.Count
		case strings.HasPrefix(g.Key, "ghost:"):
			ghostObjs += g.Count
		case g.Key == "local":
			// counted in localStrip
		default:
			clusterObjs += g.Count
		}
	}

	return &PreviewResult{
		InputPath:    path,
		MemberCount:  len(headers),
		Groups:       groups,
		TotalKept:    clusterObjs + globalObjs + ghostObjs,
		TotalRemoved: localStrip,
		Mode:         "inventory",
	}, nil
}

func classifyInspectPath(path string, clusters map[string]ClusterMeta) (key, label, display, kind, disposition string) {
	if IsLocalPath(path) {
		return "local", "local", "mgmt cluster · stripped on sanitize", "local", "strip"
	}

	for _, match := range clusterIDRe.FindAllString(path, -1) {
		if match == "local" {
			continue
		}
		if meta, ok := clusters[match]; ok {
			return "cluster:" + match, match, meta.DisplayName, meta.Kind, "present"
		}
		return "ghost:" + match, match, "orphan path refs", "ghost", "present"
	}

	return "global", "global", "Rancher global config", "global", "present"
}

func inspectTreeOrder(g TreeGroup) int {
	switch {
	case g.Key == "global":
		return 10
	case strings.HasPrefix(g.Key, "cluster:"):
		return 20
	case strings.HasPrefix(g.Key, "ghost:"):
		return 30
	case g.Key == "local":
		return 40
	default:
		return 50
	}
}
