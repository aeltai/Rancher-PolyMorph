package cmd

import (
	"strings"
	"testing"

	"github.com/aeltai/rancher-migrate/internal/version"
)

func TestRootCommand(t *testing.T) {
	root := Root()
	if root.Use != "rancher-migrate" {
		t.Fatalf("use=%q", root.Use)
	}
	if root.Version != version.Version {
		t.Fatalf("version=%q want %q", root.Version, version.Version)
	}
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"sanitize", "inspect", "restore", "s3", "config", "ui", "manual"} {
		if !names[want] {
			t.Fatalf("missing subcommand %q, have %v", want, names)
		}
	}
}

func TestFormatHelp(t *testing.T) {
	root := Root()
	help := formatHelp(root)
	if !strings.Contains(help, "Migration flow:") {
		t.Fatalf("help missing long text:\n%s", help)
	}
	if !strings.Contains(help, "Usage:") {
		t.Fatal("help missing usage")
	}
}

func TestIndentBlock(t *testing.T) {
	got := indentBlock("line one\nline two")
	if !strings.HasPrefix(got, "  line one") {
		t.Fatalf("got %q", got)
	}
}
