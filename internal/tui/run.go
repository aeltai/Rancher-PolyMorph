package tui

import (
	"fmt"
	"os"

	"github.com/aeltai/rancher-migrate/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// Run starts the interactive TUI. Requires a terminal (stdout is a TTY).
func Run() error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("interactive UI requires a terminal; use subcommands instead (rancher-migrate --help)")
	}
	cfg, _, _ := config.Load()
	m := newModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
