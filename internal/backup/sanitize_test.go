package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeEndToEnd(t *testing.T) {
	fixture := writeTestFixture(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "out.tar.gz")
	report := filepath.Join(dir, "report.txt")

	res, err := Sanitize(Options{
		Input:       fixture,
		Output:      out,
		KeepCluster: "c-aaaaa",
		Report:      report,
		Quiet:       true,
		Fast:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	if len(res.Kept) == 0 {
		t.Fatal("expected kept objects")
	}
	if len(res.Removed) == 0 {
		t.Fatal("expected removed objects")
	}
	if err := WriteReport(res, report); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatalf("report missing: %v", err)
	}

	inspect, err := InspectBackup(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspect.Clusters) == 0 {
		t.Fatal("sanitized backup should still list clusters")
	}
	if _, ok := inspect.Clusters["c-aaaaa"]; !ok {
		t.Fatal("c-aaaaa not in sanitized inventory")
	}
	if _, ok := inspect.Clusters["c-bbbbb"]; ok {
		t.Fatal("c-bbbbb should have been removed from inventory")
	}
}

func TestSanitizeKeepRKE1Only(t *testing.T) {
	fixture := writeTestFixture(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "rke1-only.tar.gz")

	_, err := Sanitize(Options{
		Input:        fixture,
		Output:       out,
		KeepRKE1Only: true,
		Quiet:        true,
		Fast:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	inspect, err := InspectBackup(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := inspect.Clusters["c-bbbbb"]; !ok {
		t.Fatal("expected rke1 cluster kept")
	}
	if _, ok := inspect.Clusters["c-aaaaa"]; ok {
		t.Fatal("rke2 cluster should be removed in keep-rke1-only mode")
	}
}

func TestSanitizeMissingInput(t *testing.T) {
	_, err := Sanitize(Options{
		Input:       "/nonexistent/backup.tar.gz",
		Output:      t.TempDir() + "/out.tar.gz",
		KeepCluster: "c-aaaaa",
		Quiet:       true,
	})
	if err == nil {
		t.Fatal("expected error for missing input")
	}
}
