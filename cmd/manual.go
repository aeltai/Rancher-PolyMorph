package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed manual/rancher-polymorph.1
var manPageROFF string

//go:embed manual/rancher-polymorph.txt
var manPageText string

func manualCmd() *cobra.Command {
	var usePager bool

	cmd := &cobra.Command{
		Use:   "manual [topic]",
		Short: "Show the rancher-polymorph manual (man page)",
		Long: `Display the rancher-polymorph manual. Without a topic, shows the full manual.

Topics: sanitize, inspect, restore, migration`,
		Example: strings.TrimSpace(`
  rancher-polymorph manual
  rancher-polymorph manual sanitize
  man -l cmd/manual/rancher-polymorph.1`),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := ""
			if len(args) > 0 {
				topic = strings.ToLower(args[0])
			}
			if usePager {
				return showManualWithPager(topic)
			}
			fmt.Print(filterManual(topic))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&usePager, "pager", "p", false, "Pipe manual through $PAGER (less)")
	return cmd
}

func showManualWithPager(topic string) error {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}
	parts := strings.Fields(pager)
	if len(parts) == 0 {
		parts = []string{"less"}
	}
	c := exec.Command(parts[0], append(parts[1:], "-")...)
	c.Stdin = strings.NewReader(filterManual(topic))
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func filterManual(topic string) string {
	if topic == "" {
		return manPageText
	}
	marker := "TOPIC: " + topic
	var b strings.Builder
	inSection := false
	for _, line := range strings.Split(manPageText, "\n") {
		if strings.HasPrefix(line, "TOPIC: ") {
			inSection = line == marker
			continue
		}
		if inSection {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if b.Len() == 0 {
		return fmt.Sprintf("No manual section for %q. Run: rancher-polymorph manual\n", topic)
	}
	return b.String()
}
