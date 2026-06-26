package backup

import "testing"

func TestKeepSetFromOpts(t *testing.T) {
	set := keepSetFromOpts(Options{
		KeepCluster:  "c-aaaaa",
		KeepClusters: []string{"c-bbbbb", "", "c-aaaaa"},
	})
	if len(set) != 2 {
		t.Fatalf("len=%d", len(set))
	}
	if _, ok := set["c-aaaaa"]; !ok {
		t.Fatal("missing c-aaaaa")
	}
	if _, ok := set["c-bbbbb"]; !ok {
		t.Fatal("missing c-bbbbb")
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{512, "512 B"},
		{2048, "2.0 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
	}
	for _, tc := range cases {
		if got := HumanSize(tc.n); got != tc.want {
			t.Fatalf("HumanSize(%d)=%q want %q", tc.n, got, tc.want)
		}
	}
}

func TestClusterKindFromSpec(t *testing.T) {
	if got := clusterKindFromSpec(map[string]any{
		"rancherKubernetesEngineConfig": map[string]any{},
	}, "c-abc"); got != "rke1" {
		t.Fatalf("got %q", got)
	}
	if got := clusterKindFromSpec(map[string]any{
		"importedConfig": map[string]any{},
	}, "c-m-xyz"); got != "imported" {
		t.Fatalf("got %q", got)
	}
	if got := clusterKindFromSpec(map[string]any{
		"rke2Config": map[string]any{},
	}, "c-m-xyz"); got != "rke2-provisioned" {
		t.Fatalf("got %q", got)
	}
	if got := clusterKindFromSpec(nil, "local"); got != "local" {
		t.Fatalf("got %q", got)
	}
}

func TestCompressLevel(t *testing.T) {
	if compressLevel(Options{Fast: true}) != 1 {
		t.Fatal("fast should be 1")
	}
	if compressLevel(Options{CompressLevel: 7}) != 7 {
		t.Fatal("explicit level")
	}
	if compressLevel(Options{}) != 3 {
		t.Fatal("default level")
	}
	if compressLevel(Options{CompressLevel: 99}) != 9 {
		t.Fatal("cap at 9")
	}
}

func TestDiscoverOrphanClusterIDs(t *testing.T) {
	names := []string{
		"settings.#v1/c-gh001/foo.json",
		"clusters.management.cattle.io#v3/c-keep1.json",
	}
	keep := map[string]struct{}{"c-keep1": {}}
	clusters := map[string]ClusterMeta{"c-keep1": {DisplayName: "keep"}}
	orphans := discoverOrphanClusterIDs(names, keep, clusters)
	if len(orphans) != 1 {
		t.Fatalf("orphans=%v", orphans)
	}
	if _, ok := orphans["c-gh001"]; !ok {
		t.Fatalf("missing c-gh001 in %v", orphans)
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]struct{}{"b": {}, "a": {}, "c": {}}
	got := sortedKeys(m)
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("sorted=%v", got)
	}
}

func TestShouldParseJSON(t *testing.T) {
	if !shouldParseJSON("clusters.fleet.cattle.io#v1alpha1/fleet-default/x.json") {
		t.Fatal("fleet path should parse")
	}
	if shouldParseJSON("settings.management.cattle.io#v3/server-url.json") {
		t.Fatal("non-cluster json should not parse")
	}
}
