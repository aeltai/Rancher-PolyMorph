package ascii

import "time"

// SplashFrame returns the animated splash screen for the TUI.
func SplashFrame(now time.Time, width int, started time.Time) string {
	return GTASplash(now, width, started)
}

// CompactHeader for in-app title bar.
func CompactHeader() string {
	return styleLabel.Render("◆ RANCHER") + styleSubtitle.Render(" // ") + styleTitle.Render("MIGRATE")
}

// ProgressLoader returns a loading bar for operation progress (0–1).
func ProgressLoader(progress float64, width int, t time.Time) string {
	return GTALoadingBar(progress, width, t, false)
}
