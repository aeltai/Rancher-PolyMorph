package version

import "testing"

func TestVersionSemver(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
	if Version[0] != '0' && Version[0] != '1' {
		t.Fatalf("unexpected version %q", Version)
	}
}
