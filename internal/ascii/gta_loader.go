package ascii

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
)

var (
	titleFiglet    string
	subtitleFiglet string

	gtaTips = []string{
		"Inspect backups before sanitizing large environments",
		"Keep only the clusters you plan to migrate",
		"Local cluster artifacts are always stripped on restore",
		"Use prune: false on the Restore CR",
		"Ghost cluster IDs are auto-detected from tar paths",
		"RKE1, RKE2, and imported clusters are all supported",
		"Review the keep/drop tree before confirming sanitize",
		"Never commit backup tarballs or kubeconfigs to git",
	}

	styleTitle    lipgloss.Style
	styleSubtitle lipgloss.Style
	styleTip      lipgloss.Style
	styleLabel    lipgloss.Style
	styleBarFill  lipgloss.Style
	styleBarShine lipgloss.Style
	styleBarEmpty lipgloss.Style
	stylePct      lipgloss.Style
)

func init() {
	titleFiglet = strings.TrimRight(figure.NewFigure("RANCHER", "slant", true).String(), "\n")
	subtitleFiglet = strings.TrimRight(figure.NewFigure("MIGRATE", "standard", true).String(), "\n")

	gold := lipgloss.Color("#F0A030")
	goldBright := lipgloss.Color("#FFD060")
	dim := lipgloss.Color("#6B6B6B")
	mid := lipgloss.Color("#9A9A9A")

	styleTitle = lipgloss.NewStyle().Foreground(gold).Bold(true)
	styleSubtitle = lipgloss.NewStyle().Foreground(mid)
	styleTip = lipgloss.NewStyle().Foreground(dim).Italic(true)
	styleLabel = lipgloss.NewStyle().Foreground(gold).Bold(true)
	styleBarFill = lipgloss.NewStyle().Foreground(gold)
	styleBarShine = lipgloss.NewStyle().Foreground(goldBright).Bold(true)
	styleBarEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	stylePct = lipgloss.NewStyle().Foreground(mid)
}

// GTASplash returns the loading splash (animated bar + rotating tips).
func GTASplash(now time.Time, width int, started time.Time) string {
	if width < 40 {
		width = 80
	}
	progress := splashProgress(started, now)
	tip := gtaTips[int(now.UnixMilli()/2200)%len(gtaTips)]

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(centerBlock(styleTitle.Render(titleFiglet), width))
	b.WriteString("\n")
	b.WriteString(centerBlock(styleSubtitle.Render(subtitleFiglet), width))
	b.WriteString("\n\n")
	b.WriteString(centerBlock(GTALoadingBar(progress, width, now, false), width))
	b.WriteString("\n\n")
	b.WriteString(centerBlock(styleTip.Render(tip), width))
	return b.String()
}

// IndeterminateLoader animates the bar when real progress is unknown.
func IndeterminateLoader(width int, t time.Time) string {
	return GTALoadingBar(indeterminateProgress(t), width, t, true)
}

// splashProgress ramps 0→1 once over the splash window (no loop reset).
func splashProgress(started, now time.Time) float64 {
	if started.IsZero() {
		return indeterminateProgress(now)
	}
	const duration = 2.8
	p := now.Sub(started).Seconds() / duration
	if p < 0 {
		return 0
	}
	if p >= 1 {
		return 1
	}
	// Ease-out: smooth deceleration, no snap at the end.
	return 1 - math.Pow(1-p, 2.5)
}

// indeterminateProgress uses a sine wave so the bar breathes without looping to 0%.
func indeterminateProgress(t time.Time) float64 {
	ms := float64(t.UnixMilli())
	// 0.25–0.85 range, slow oscillation
	return 0.25 + 0.60*(0.5+0.5*math.Sin(ms/700.0))
}

// GTALoadingBar renders a segmented loading bar (0.0–1.0).
// When indeterminate is true, the shine travels the full bar instead of jumping with fill.
func GTALoadingBar(progress float64, width int, t time.Time, indeterminate bool) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	barWidth := 36
	if width > 20 && width-12 < barWidth {
		barWidth = width - 12
	}
	if barWidth < 12 {
		barWidth = 12
	}

	// Fractional fill for smoother steps (round each frame, not truncated early).
	filledF := progress * float64(barWidth)
	filled := int(math.Round(filledF))
	if progress > 0 && filled == 0 {
		filled = 1
	}
	if filled > barWidth {
		filled = barWidth
	}

	ms := float64(t.UnixMilli())
	var shine int
	if indeterminate {
		shine = int(ms/70) % barWidth
	} else if filled > 0 {
		shine = int(ms/90) % filled
	}

	var bar strings.Builder
	bar.WriteString(styleLabel.Render("LOADING"))
	bar.WriteString("\n")
	bar.WriteString(styleBarEmpty.Render("["))
	for i := 0; i < barWidth; i++ {
		switch {
		case indeterminate && i == shine:
			bar.WriteString(styleBarShine.Render("█"))
		case !indeterminate && i < filled && i == shine:
			bar.WriteString(styleBarShine.Render("█"))
		case !indeterminate && i < filled:
			bar.WriteString(styleBarFill.Render("█"))
		case indeterminate:
			bar.WriteString(styleBarEmpty.Render("░"))
		default:
			bar.WriteString(styleBarEmpty.Render("░"))
		}
	}
	bar.WriteString(styleBarEmpty.Render("]"))
	if indeterminate {
		bar.WriteString(stylePct.Render("  ..."))
	} else {
		bar.WriteString(stylePct.Render(fmt.Sprintf(" %3d%%", int(math.Round(progress*100)))))
	}
	return bar.String()
}

func centerBlock(s string, width int) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			out = append(out, "")
			continue
		}
		visible := lipgloss.Width(line)
		pad := (width - visible) / 2
		if pad < 0 {
			pad = 0
		}
		out = append(out, strings.Repeat(" ", pad)+line)
	}
	return strings.Join(out, "\n")
}
