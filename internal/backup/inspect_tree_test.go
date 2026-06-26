package backup

import (
	"strings"
	"testing"
)

func TestBuildInspectTree(t *testing.T) {
	path := writeTestFixture(t)
	tree, err := BuildInspectTree(path)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Mode != "inventory" {
		t.Fatalf("mode=%q", tree.Mode)
	}
	var hasLocal, hasCluster, hasGlobal bool
	for _, g := range tree.Groups {
		switch {
		case g.Key == "local":
			hasLocal = true
			if g.Disposition != "strip" {
				t.Fatalf("local disposition=%q", g.Disposition)
			}
		case g.Key == "cluster:c-aaaaa":
			hasCluster = true
			if g.Disposition != "present" {
				t.Fatalf("cluster disposition=%q", g.Disposition)
			}
		case g.Key == "global":
			hasGlobal = true
		}
	}
	if !hasLocal || !hasCluster || !hasGlobal {
		t.Fatalf("missing groups: local=%v cluster=%v global=%v groups=%v", hasLocal, hasCluster, hasGlobal, tree.Groups)
	}
	out := strings.Join(FormatTreeLines(tree, nil, 100), "\n")
	if !strings.Contains(out, "STRIP") || !strings.Contains(out, "IN BACKUP") {
		t.Fatalf("expected strip/present tags in:\n%s", out)
	}
}

func TestIsLocalPath(t *testing.T) {
	if !IsLocalPath("clusters.management.cattle.io#v3/local.json") {
		t.Fatal("local cluster json should match")
	}
	if IsLocalPath("authconfigs.management.cattle.io#v3/local.json") {
		t.Fatal("authconfig local should be kept")
	}
}
