package ascii

import (
	"strings"
	"testing"
	"time"
)

func TestGTASplashRenders(t *testing.T) {
	start := time.Now().Add(-1 * time.Second)
	out := SplashFrame(time.Now(), 100, start)
	if len(out) < 80 {
		t.Fatalf("splash too short: %q", out)
	}
	if !strings.Contains(out, "LOADING") {
		t.Fatal("expected loading label")
	}
	if strings.Contains(out, "rockstar") {
		t.Fatal("rockstar tagline should be removed")
	}
	if strings.Contains(out, "moo") {
		t.Fatal("cow ascii should be gone")
	}
}

func TestSplashProgressMonotonic(t *testing.T) {
	start := time.Now()
	p0 := splashProgress(start, start.Add(200*time.Millisecond))
	p1 := splashProgress(start, start.Add(800*time.Millisecond))
	p2 := splashProgress(start, start.Add(1500*time.Millisecond))
	if p1 <= p0 || p2 <= p1 {
		t.Fatalf("splash progress should increase: %v %v %v", p0, p1, p2)
	}
	if splashProgress(start, start.Add(5*time.Second)) != 1 {
		t.Fatal("splash should cap at 100%")
	}
}

func TestProgressLoader(t *testing.T) {
	out := ProgressLoader(0.5, 80, time.Now())
	if !strings.Contains(out, "50%") {
		t.Fatalf("expected 50%% progress, got: %s", out)
	}
}

func TestIndeterminateNoPercentJump(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(3500 * time.Millisecond) // past old 3.2s loop — should not reset harshly
	p0 := indeterminateProgress(t0)
	p1 := indeterminateProgress(t1)
	if p0 < 0.2 || p0 > 0.9 || p1 < 0.2 || p1 > 0.9 {
		t.Fatalf("indeterminate out of range: %v %v", p0, p1)
	}
}
