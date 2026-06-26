package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

var clusterIDRe = regexp.MustCompile(`\b(c-m-[a-z0-9]+|c-[a-z0-9]{5})\b`)

func HumanSize(n int64) string {
	value := float64(n)
	units := []string{"B", "KB", "MB", "GB"}
	for i, unit := range units {
		if value < 1024 || unit == "GB" {
			if unit == "B" {
				return fmt.Sprintf("%d B", int(value))
			}
			return fmt.Sprintf("%.1f %s", value, unit)
		}
		value /= 1024
		if i == len(units)-1 {
			break
		}
	}
	return fmt.Sprintf("%.1f GB", value)
}

func openTarGz(path string) (*tar.Reader, io.Closer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	gr, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return tar.NewReader(gr), &gzipReadCloser{f: f, gr: gr}, nil
}

type gzipReadCloser struct {
	f  *os.File
	gr *gzip.Reader
}

func (g *gzipReadCloser) Close() error {
	_ = g.gr.Close()
	return g.f.Close()
}

func readAllHeaders(path string) ([]*tar.Header, error) {
	tr, closer, err := openTarGz(path)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	var headers []*tar.Header
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		headers = append(headers, hdr)
		if hdr.Size > 0 {
			if _, err := io.CopyN(io.Discard, tr, hdr.Size); err != nil {
				return nil, err
			}
		}
	}
	return headers, nil
}

func readMemberJSON(path string, hdr *tar.Header) (map[string]any, error) {
	tr, closer, err := openTarGz(path)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("member not found: %s", hdr.Name)
		}
		if err != nil {
			return nil, err
		}
		if h.Name == hdr.Name {
			if h.Size == 0 {
				return nil, fmt.Errorf("empty member: %s", hdr.Name)
			}
			var doc map[string]any
			dec := json.NewDecoder(tr)
			if err := dec.Decode(&doc); err != nil {
				return nil, err
			}
			return doc, nil
		}
		if h.Size > 0 {
			if _, err := io.CopyN(io.Discard, tr, h.Size); err != nil {
				return nil, err
			}
		}
	}
}

func clusterKindFromSpec(spec map[string]any, cid string) string {
	if spec == nil {
		if cid == "local" {
			return "local"
		}
		return "unknown"
	}
	if _, ok := spec["rancherKubernetesEngineConfig"]; ok {
		return "rke1"
	}
	if _, ok := spec["importedConfig"]; ok {
		return "imported"
	}
	if _, ok := spec["genericEngineConfig"]; ok {
		return "imported"
	}
	if _, ok := spec["rke2Config"]; ok {
		return "rke2-provisioned"
	}
	if _, ok := spec["k3sConfig"]; ok {
		return "rke2-provisioned"
	}
	if cid == "local" {
		return "local"
	}
	return "unknown"
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func boolField(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	b, _ := v.(bool)
	return b
}

func labelsField(doc map[string]any) map[string]string {
	out := make(map[string]string)
	meta, _ := doc["metadata"].(map[string]any)
	if meta == nil {
		return out
	}
	labels, _ := meta["labels"].(map[string]any)
	for k, v := range labels {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func clusterIDFromJSON(doc map[string]any) string {
	labels := labelsField(doc)
	return labels["management.cattle.io/cluster-name"]
}

func buildInventory(path string, headers []*tar.Header) (map[string]ClusterMeta, *FleetIndex, error) {
	clusters := make(map[string]ClusterMeta)
	fleetIndex := NewFleetIndex()

	for _, hdr := range headers {
		name := hdr.Name
		if strings.HasPrefix(name, FleetClusterPrefix) && strings.HasSuffix(name, ".json") {
			doc, err := readMemberJSON(path, hdr)
			if err != nil {
				continue
			}
			labels := labelsField(doc)
			clusterID := labels["management.cattle.io/cluster-name"]
			if clusterID == "" {
				continue
			}
			meta, _ := doc["metadata"].(map[string]any)
			fleetName := stringField(meta, "name")
			display := labels["management.cattle.io/cluster-display-name"]
			if fleetName != "" {
				fleetIndex.NameToCluster[fleetName] = clusterID
			}
			if display != "" {
				fleetIndex.DisplayToCluster[display] = clusterID
			}
			fleetIndex.NameToCluster[clusterID] = clusterID
			continue
		}

		if !strings.HasPrefix(name, ClusterPrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		cid := name[len(ClusterPrefix) : len(name)-5]
		doc, err := readMemberJSON(path, hdr)
		if err != nil {
			continue
		}
		spec, _ := doc["spec"].(map[string]any)
		display := stringField(spec, "displayName")
		if display == "" {
			display = cid
		}
		clusters[cid] = ClusterMeta{
			DisplayName: display,
			Kind:        clusterKindFromSpec(spec, cid),
			Internal:    boolField(spec, "internal"),
		}
	}

	MergeInventoryDisplayNames(fleetIndex, clusters)
	return clusters, fleetIndex, nil
}

func buildRemovePlan(opts Options, clusters map[string]ClusterMeta, fleetIndex *FleetIndex, allNames []string) (map[string]struct{}, []string) {
	remove := make(map[string]struct{})
	for _, cid := range opts.RemoveClusters {
		remove[cid] = struct{}{}
	}

	keepSet := keepSetFromOpts(opts)
	if len(keepSet) > 0 {
		for cid := range clusters {
			if cid == "local" {
				continue
			}
			if _, keep := keepSet[cid]; !keep {
				remove[cid] = struct{}{}
			}
		}
	} else if opts.KeepRKE1Only && len(opts.RemoveClusters) == 0 {
		for cid, meta := range clusters {
			if cid == "local" {
				continue
			}
			switch meta.Kind {
			case "imported", "rke2-provisioned", "unknown":
				remove[cid] = struct{}{}
			}
		}
	}

	var autoOrphans []string
	if len(keepSet) > 0 && !opts.NoAutoOrphans {
		orphanSet := discoverOrphanClusterIDs(allNames, keepSet, clusters)
		for cid := range orphanSet {
			remove[cid] = struct{}{}
		}
		autoOrphans = sortedKeys(orphanSet)
		for _, cid := range fleetIndex.NameToCluster {
			if cid == "local" {
				continue
			}
			if _, keep := keepSet[cid]; !keep {
				remove[cid] = struct{}{}
			}
		}
	}

	return remove, autoOrphans
}

func discoverOrphanClusterIDs(names []string, keepSet map[string]struct{}, clusters map[string]ClusterMeta) map[string]struct{} {
	orphans := make(map[string]struct{})
	for _, path := range names {
		for _, match := range clusterIDRe.FindAllString(path, -1) {
			if match == "local" {
				continue
			}
			if _, keep := keepSet[match]; keep {
				continue
			}
			if _, ok := clusters[match]; !ok {
				orphans[match] = struct{}{}
			}
		}
	}
	return orphans
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func shouldParseJSON(path string) bool {
	if !strings.HasSuffix(path, ".json") {
		return false
	}
	for _, p := range JSONClusterPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func compressLevel(opts Options) int {
	if opts.Fast {
		return 1
	}
	level := opts.CompressLevel
	if level < 1 {
		level = 3
	}
	if level > 9 {
		level = 9
	}
	return level
}
