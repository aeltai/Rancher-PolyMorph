package backup

import (
	"strings"
	"testing"
)

func TestFormatReportBrief(t *testing.T) {
	res := &Result{
		InputPath:  "/in.tar.gz",
		OutputPath: "/out.tar.gz",
		Clusters: map[string]ClusterMeta{
			"c-aaaaa": {DisplayName: "keep-me", Kind: "rke1"},
		},
		Removed: []RemovedEntry{
			{Path: "x", Reason: "cluster c-bbbbb"},
		},
	}
	out := FormatReportBrief(res)
	if !strings.Contains(out, "keep-me") {
		t.Fatalf("report=%q", out)
	}
}

func TestWriteReport(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/report.txt"
	res := &Result{Removed: []RemovedEntry{{Path: "a", Reason: "test"}}}
	if err := WriteReport(res, path); err != nil {
		t.Fatal(err)
	}
	if err := WriteReport(res, ""); err != nil {
		t.Fatal(err)
	}
}
