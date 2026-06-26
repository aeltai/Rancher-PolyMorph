package ascii

import (
	"strings"
	"time"
)

// Logo is the static rancher-migrate banner.
const Logo = `
 ██████╗  █████╗ ███╗   ██╗ ██████╗██╗  ██╗███████╗██████╗       ███╗   ███╗██╗ ██████╗ ██████╗  █████╗ ████████╗███████╗
 ██╔══██╗██╔══██╗████╗  ██║██╔════╝██║  ██║██╔════╝██╔══██╗      ████╗ ████║██║██╔════╝ ██╔══██╗██╔══██╗╚══██╔══╝██╔════╝
 ██████╔╝███████║██╔██╗ ██║██║     ███████║█████╗  ██████╔╝█████╗██╔████╔██║██║██║  ███╗██████╔╝███████║   ██║   █████╗
 ██╔══██╗██╔══██║██║╚██╗██║██║     ██╔══██║██╔══╝  ██╔══██╗╚════╝██║╚██╔╝██║██║██║   ██║██╔══██╗██╔══██║   ██║   ██╔══╝
 ██║  ██║██║  ██║██║ ╚████║╚██████╗██║  ██║███████╗██║  ██║      ██║ ╚═╝ ██║██║╚██████╔╝██║  ██║██║  ██║   ██║   ███████╗
 ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝      ╚═╝     ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   ╚══════╝
`

var taglines = []string{
	"backup → sanitize → restore",
	"migrate Rancher to a new cluster",
	"strip ghosts · keep clusters · ship it",
}

var herdFrames = []string{
	`
    \\___/          \\___/
   (  o o )  --->  (  o o )
   /   V   \\       /   V   \\
`,
	`
    \\___/          \\___/
   (  ^ o )  --->  (  o ^ )
   /   V   \\       /   V   \\
`,
	`
    \\___/          \\___/
   (  o o )  --->  (  o o )
   /   V   \\       /   V   \\
      moo!             migrate!
`,
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SplashFrame returns an animated splash screen for the TUI.
func SplashFrame(t time.Time) string {
	frame := int(t.UnixMilli()/180) % len(herdFrames)
	spin := spinnerFrames[int(t.UnixMilli()/80)%len(spinnerFrames)]
	tag := taglines[int(t.UnixMilli()/900)%len(taglines)]

	var b strings.Builder
	b.WriteString(Logo)
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(spin)
	b.WriteString("  ")
	b.WriteString(tag)
	b.WriteString("\n")
	b.WriteString(herdFrames[frame])
	return b.String()
}

// CompactHeader for in-app title bar.
func CompactHeader() string {
	return "◈ rancher-migrate ◈  backup · sanitize · restore · s3 · k8s"
}

// ProgressCow small inline ASCII animation.
func ProgressCow(t time.Time) string {
	frames := []string{"(o_o)", "(^o^)", ">-moo->", "=-cow=-"}
	return frames[int(t.UnixMilli()/250)%len(frames)]
}
