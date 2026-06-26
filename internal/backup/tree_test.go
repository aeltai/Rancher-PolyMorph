package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewSanitizeMultiCluster(t *testing.T) {
	fixture := writeTestFixture(t)
	defer os.Remove(fixture)

	keepOne, err := PreviewSanitize(Options{
		Input:       fixture,
		KeepCluster: "c-aaaaa",
	})
	if err != nil {
		t.Fatal(err)
	}
	if keepOne.TotalKept == 0 || keepOne.TotalRemoved == 0 {
		t.Fatalf("expected both kept and removed, got keep=%d remove=%d", keepOne.TotalKept, keepOne.TotalRemoved)
	}

	keepRKE1, err := PreviewSanitize(Options{
		Input:        fixture,
		KeepRKE1Only: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var hasRKE1 bool
	for _, g := range keepRKE1.Groups {
		if g.Label == "c-bbbbb" && g.Disposition == "keep" {
			hasRKE1 = true
		}
	}
	if !hasRKE1 {
		t.Fatal("expected c-bbbbb rke1 cluster kept in keep-rke1-only mode")
	}
}

func writeTestFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.tar.gz")
	// reuse python-less: call a minimal shell or write via test helper
	// use Sanitize with inspect on existing - create via go tar
	if err := writeMinimalFixture(path); err != nil {
		t.Fatal(err)
	}
	return path
}
