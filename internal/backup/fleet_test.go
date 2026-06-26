package backup

import "testing"

func TestFleetIndexClusterForName(t *testing.T) {
	fi := NewFleetIndex()
	fi.NameToCluster["c-abc12"] = "c-abc12"
	fi.DisplayToCluster["my-cluster"] = "c-abc12"

	if got := fi.ClusterForName("c-abc12"); got != "c-abc12" {
		t.Fatalf("got %q", got)
	}
	if got := fi.ClusterForName("my-cluster"); got != "c-abc12" {
		t.Fatalf("display lookup got %q", got)
	}
	if got := fi.ClusterForName("missing"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFleetIndexBundleNames(t *testing.T) {
	fi := NewFleetIndex()
	names := fi.BundleNames("fleet-agent-c-m-abc12-managed-system-agent")
	if len(names) < 2 {
		t.Fatalf("bundle names=%v", names)
	}
	foundAgent := false
	for _, n := range names {
		if n == "fleet-agent-c-m-abc12" {
			foundAgent = true
		}
	}
	if !foundAgent {
		t.Fatalf("bundle names=%v", names)
	}

	mcc := fi.BundleNames("mcc-c-m-xyz99-managed-system-agent")
	if len(mcc) < 2 {
		t.Fatalf("mcc names=%v", mcc)
	}
}

func TestMergeInventoryDisplayNames(t *testing.T) {
	fi := NewFleetIndex()
	clusters := map[string]ClusterMeta{
		"c-aaaaa": {DisplayName: "prod-rke1"},
		"local":   {DisplayName: "local"},
	}
	MergeInventoryDisplayNames(fi, clusters)
	if fi.DisplayToCluster["prod-rke1"] != "c-aaaaa" {
		t.Fatalf("display map=%v", fi.DisplayToCluster)
	}
	if _, ok := fi.NameToCluster["local"]; ok {
		t.Fatal("local should not be merged as downstream")
	}
}
