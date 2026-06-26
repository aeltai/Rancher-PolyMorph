package cmd

import (
	"fmt"
	"strings"

	"github.com/aeltai/rancher-polymorph/internal/version"
	"github.com/spf13/cobra"
)

func Root() *cobra.Command {
	loadAppConfig()

	root := &cobra.Command{
		Use:   "rancher-polymorph",
		Short: "Rancher PolyMorph — backup sanitize and migration to a new cluster",
		Long: `Rancher PolyMorph (rancher-polymorph) — CLI for sanitizing Rancher backup tarballs before restore
on a new management cluster, inspecting backups, and generating Restore CR manifests.

Migration flow:
  1. config init     — write ~/.config/rancher-polymorph/rancher-polymorph.yaml
  2. s3 pull         — download full backup (optional)
  3. inspect/sanitize — single-cluster tarball
  4. restore run     — copy to operator pod + apply Restore CR (kubeconfig)
  5. Install cert-manager + Rancher Helm after restore completes

Run 'rancher-polymorph ui' for the interactive wizard.
Configure defaults via rancher-polymorph.yaml (see: config init).`,
		Version:      version.Version,
		SilenceUsage: true,
	}

	root.SetVersionTemplate("rancher-polymorph version {{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = false

	root.PersistentFlags().BoolVarP(&globalVerbose, "verbose", "v", false, "Verbose stderr/log-file diagnostics")
	root.PersistentFlags().BoolVarP(&globalQuiet, "quiet", "q", false, "Suppress progress and logs")

	root.AddCommand(sanitizeCmd())
	root.AddCommand(inspectCmd())
	root.AddCommand(restoreCmd())
	root.AddCommand(s3Cmd())
	root.AddCommand(configCmd())
	root.AddCommand(manualCmd())
	root.AddCommand(uiCmd())

	setHelpRecursive(root)

	return root
}

func setHelpRecursive(cmd *cobra.Command) {
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		fmt.Fprint(c.OutOrStdout(), formatHelp(c))
	})
	for _, child := range cmd.Commands() {
		setHelpRecursive(child)
	}
}

func formatHelp(cmd *cobra.Command) string {
	var b strings.Builder
	if cmd.Long != "" {
		b.WriteString(cmd.Long)
		b.WriteByte('\n')
	}
	b.WriteString("\nUsage:\n  ")
	b.WriteString(cmd.UseLine())
	b.WriteString("\n")

	if cmd.HasAvailableLocalFlags() {
		b.WriteString("\nFlags:\n")
		b.WriteString(cmd.LocalFlags().FlagUsages())
	}
	if cmd.HasAvailableInheritedFlags() {
		b.WriteString("\nGlobal flags:\n")
		b.WriteString(cmd.InheritedFlags().FlagUsages())
	}
	if cmd.Example != "" {
		b.WriteString("\nExamples:\n")
		b.WriteString(indentBlock(cmd.Example))
	}
	if len(cmd.Commands()) > 0 {
		b.WriteString("\nCommands:\n")
		for _, c := range cmd.Commands() {
			if c.Hidden {
				continue
			}
			b.WriteString(fmt.Sprintf("  %-16s %s\n", c.Name(), c.Short))
		}
	}
	b.WriteString("\nSee 'rancher-polymorph manual' for the full manual.\n")
	return b.String()
}

func indentBlock(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n") + "\n"
}
