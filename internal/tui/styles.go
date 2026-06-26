package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			MarginBottom(1)

	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)

	focusedStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().BorderStyle(lipgloss.HiddenBorder())

	treeRowSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("236")).
				PaddingLeft(1).
				PaddingRight(1)

	clusterRowSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("236"))
)
