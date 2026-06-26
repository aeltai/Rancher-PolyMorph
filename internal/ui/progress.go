package ui

import (
	"fmt"
	"io"
	"os"
	"time"
)

// ProgressBar renders a single-line progress indicator to stderr.
type ProgressBar struct {
	label      string
	total      int
	current    int
	enabled    bool
	start      time.Time
	lastRender time.Time
	out        io.Writer
	log        io.Writer
}

func NewProgressBar(total int, label string, enabled bool, log io.Writer) *ProgressBar {
	return &ProgressBar{
		label:   label,
		total:   max(1, total),
		enabled: enabled,
		start:   time.Now(),
		out:     os.Stderr,
		log:     log,
	}
}

func (p *ProgressBar) Advance() {
	p.current++
	if p.current > p.total {
		p.current = p.total
	}
	if !p.enabled {
		return
	}
	now := time.Now()
	if now.Sub(p.lastRender) < 80*time.Millisecond && p.current < p.total {
		return
	}
	p.render(now)
	p.lastRender = now
}

func (p *ProgressBar) Current() int { return p.current }
func (p *ProgressBar) Total() int   { return p.total }

func (p *ProgressBar) render(now time.Time) {
	ratio := float64(p.current) / float64(p.total)
	filled := int(ratio * 28)
	bar := repeat('#', filled) + repeat('-', 28-filled)
	elapsed := now.Sub(p.start).Seconds()
	eta := ""
	if p.current > 0 && p.current < p.total {
		rate := float64(p.current) / elapsed
		if rate > 0 {
			eta = fmt.Sprintf(", ETA %.0fs", float64(p.total-p.current)/rate)
		}
	}
	line := fmt.Sprintf("%s [%s] %5.1f%% (%d/%d, %.1fs%s)\n",
		p.label, bar, ratio*100, p.current, p.total, elapsed, eta)
	if p.current < p.total {
		// overwrite line when in progress
		_, _ = fmt.Fprintf(p.out, "\r%s [%s] %5.1f%% (%d/%d, %.1fs%s)",
			p.label, bar, ratio*100, p.current, p.total, elapsed, eta)
	} else {
		_, _ = fmt.Fprintf(p.out, "\r%-72s\n", p.label+" done")
	}
	if p.log != nil {
		_, _ = p.log.Write([]byte(line))
	}
}

func (p *ProgressBar) Finish(msg string) {
	if !p.enabled {
		return
	}
	_, _ = fmt.Fprintf(p.out, "%s\n", msg)
	if p.log != nil {
		_, _ = io.WriteString(p.log, msg+"\n")
	}
}

func repeat(ch byte, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
