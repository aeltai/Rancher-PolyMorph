package backup

import "testing"

func TestRemovalMatcherLocalPaths(t *testing.T) {
	m := NewRemovalMatcher(map[string]struct{}{}, NewFleetIndex())

	if reason := m.Reason("clusters.management.cattle.io#v3/local.json", nil); reason != LocalReason {
		t.Fatalf("local cluster: %q", reason)
	}
	if reason := m.Reason("authconfigs.management.cattle.io#v3/local.json", nil); reason != "" {
		t.Fatalf("authconfig local should be kept, got %q", reason)
	}
	if reason := m.Reason("nodes.management.cattle.io#v3/local/m-1.json", nil); reason != LocalReason {
		t.Fatalf("local node path: %q", reason)
	}
}

func TestRemovalMatcherClusterPath(t *testing.T) {
	remove := map[string]struct{}{"c-drop1": {}}
	m := NewRemovalMatcher(remove, NewFleetIndex())

	path := "nodes.management.cattle.io#v3/c-drop1/m-node.json"
	if reason := m.Reason(path, nil); reason != "cluster c-drop1" {
		t.Fatalf("reason=%q", reason)
	}
}

func TestRemovalMatcherSecretReason(t *testing.T) {
	remove := map[string]struct{}{"c-secretx": {}}
	m := NewRemovalMatcher(remove, NewFleetIndex())

	path := "secrets.#v1/cattle-system/c-c-c-secretx.json"
	if reason := m.Reason(path, nil); reason != "cluster c-secretx" {
		t.Fatalf("reason=%q", reason)
	}
}

func TestRemovalMatcherFleetReason(t *testing.T) {
	fi := NewFleetIndex()
	fi.NameToCluster["fleet-agent-c-drop2"] = "c-drop2"
	remove := map[string]struct{}{"c-drop2": {}}
	m := NewRemovalMatcher(remove, fi)

	path := "clusters.fleet.cattle.io#v1alpha1/fleet-default/fleet-agent-c-drop2.json"
	if reason := m.Reason(path, nil); reason != "cluster c-drop2" && reason != "fleet cluster c-drop2" {
		t.Fatalf("reason=%q", reason)
	}
}

func TestCountReasons(t *testing.T) {
	removed := []RemovedEntry{
		{Reason: "cluster c-a"},
		{Reason: "cluster c-a"},
		{Reason: LocalReason},
	}
	counts := countReasons(removed)
	if counts["cluster c-a"] != 2 || counts[LocalReason] != 1 {
		t.Fatalf("counts=%v", counts)
	}
	sorted := sortedReasonCounts(counts)
	if sorted[0].Count != 2 {
		t.Fatalf("sorted=%v", sorted)
	}
}

func TestParseJSONIfNeeded(t *testing.T) {
	doc := parseJSONIfNeeded([]byte(`{"metadata":{"name":"x"}}`))
	if doc == nil || doc["metadata"] == nil {
		t.Fatal("expected doc")
	}
	if parseJSONIfNeeded([]byte(`not json`)) != nil {
		t.Fatal("invalid json should return nil")
	}
}
